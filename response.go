package requests4go

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/bitly/go-simplejson"
)

// Response represents the response from the server.
type Response struct {
	RawResponse   *http.Response
	Status        string
	StatusCode    int
	Header        http.Header
	content       *bytes.Buffer
	internalError error
}

// NewResponse returns new Response
func NewResponse(rawResp *http.Response, err error) (*Response, error) {
	if err != nil {
		return &Response{internalError: err}, fmt.Errorf("NewResponse error: %w", err)
	}

	return &Response{
		RawResponse:   rawResp,
		Status:        rawResp.Status,
		StatusCode:    rawResp.StatusCode,
		Header:        rawResp.Header,
		content:       bytes.NewBuffer([]byte{}),
		internalError: nil,
	}, nil
}

// Ok returns true if the status code is less than 400.
func (r *Response) Ok() bool {
	if r.internalError != nil {
		return false
	}
	return r.StatusCode < 400 && r.StatusCode >= 200
}

// Close is to support io.ReadCloser.
func (r *Response) Close() error {
	if r.internalError != nil {
		return r.internalError
	}

	io.Copy(ioutil.Discard, r)
	return r.RawResponse.Body.Close()
}

// Read is to support io.ReadClose.
func (r *Response) Read(p []byte) (n int, err error) {
	if r.internalError != nil {
		return -1, r.internalError
	}
	return r.RawResponse.Body.Read(p)
}

func (r *Response) loadContent() error {
	if r.internalError != nil {
		return r.internalError
	}

	if r.content.Len() != 0 || r.RawResponse.ContentLength == 0 {
		return nil
	}

	if r.RawResponse.ContentLength > 0 {
		r.content.Grow(int(r.RawResponse.ContentLength))
	}

	var reader io.ReadCloser

	defer func() {
		r.Close()
		reader.Close()
	}()

	var err error
	switch r.Header.Get("Content-Encoding") {
	case "gzip":
		if reader, err = gzip.NewReader(r); err != nil {
			return err
		}
	case "deflate":
		if reader, err = zlib.NewReader(r); err != nil {
			return err
		}
	default:
		reader = r
	}

	if _, err := io.Copy(r.content, reader); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func (r *Response) getContent() io.Reader {
	if r.content.Len() == 0 {
		return r
	}
	return r.content
}

// Text returns content of response in string.
func (r *Response) Text() (string, error) {
	if err := r.loadContent(); err != nil {
		return "", err
	}
	return r.content.String(), nil
}

// Content returns content of response in bytes.
func (r *Response) Content() ([]byte, error) {
	if err := r.loadContent(); err != nil {
		return nil, err
	}
	if r.content.Len() == 0 {
		return nil, nil
	}

	return r.content.Bytes(), nil
}

// Json returns simplejson.Json.
// See the usage of simplejson on https://godoc.org/github.com/bitly/go-simplejson.
func (r *Response) Json() (*simplejson.Json, error) {
	if r.internalError != nil {
		return nil, r.internalError
	}
	cnt, err := r.Content()
	if err != nil {
		r.internalError = err
		return nil, fmt.Errorf("Json error: %w", err)
	}
	return simplejson.NewJson(cnt)
}

// SaveContent saves response body to file.
func (r *Response) SaveContent(filename string) error {
	if r.internalError != nil {
		return r.internalError
	}

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("SaveContent error: %w", err)
	}

	defer r.Close()
	defer f.Close()

	_, err = io.Copy(f, r.getContent())

	if err != nil && err != io.EOF {
		return fmt.Errorf("SaveContent error: %w", err)
	}

	return nil
}
