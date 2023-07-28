package utils

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

// Interceptor
type HttpInterceptor struct {
	core         http.RoundTripper
	enabled      bool
	bodyReplacer Replacer
}
type Replacer func(body []byte) []byte

func NewHttpInterceptor(bodyReplacer Replacer) *HttpInterceptor {
	interceptor := &HttpInterceptor{
		core:         http.DefaultTransport,
		enabled:      false,
		bodyReplacer: bodyReplacer,
	}
	return interceptor
}

func (i *HttpInterceptor) Enable() {
	i.enabled = true
}
func (i *HttpInterceptor) Disable() {
	i.enabled = false
}

func (i *HttpInterceptor) RoundTrip(req *http.Request) (*http.Response, error) {
	defer func() {
		if req != nil && req.Body != nil {
			_ = req.Body.Close()
		}
	}()

	res, err := i.core.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if i.enabled {
		defer func() {
			if req != nil && req.Body != nil {
				_ = res.Body.Close()
			}
		}()
		body, _ := ioutil.ReadAll(res.Body)

		newBody := i.bodyReplacer(body)

		res.Body = io.NopCloser(bytes.NewReader(newBody))
		res.ContentLength = int64(len(newBody))
		res.Header.Set("Content-Length", fmt.Sprintf("%d", res.ContentLength))
	}
	newRes := res

	return newRes, nil
}
