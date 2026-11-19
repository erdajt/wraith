package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type httpOperation struct {
	url, method string
	headers     http.Header
	body        []byte
	timeout     time.Duration
	limit       int
	redirects   int
	transport   *http.Transport
}

// run keeps HTTP status separate from transport errors and retains bounded partial responses.
func (op httpOperation) run(parent context.Context) (result map[string]any, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("http.request: %w", err)
		}
	}()
	ctx, cancel := context.WithTimeout(parent, op.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, op.method, op.url, bytes.NewReader(op.body))
	if err != nil {
		return nil, err
	}
	request.Header = op.headers.Clone()
	transport := op.transport
	if transport == nil {
		transport = http.DefaultTransport.(*http.Transport)
	}
	transport = transport.Clone()
	transport.MaxResponseHeaderBytes = 64 << 10
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(next *http.Request, via []*http.Request) error {
		if op.redirects == 0 {
			return http.ErrUseLastResponse
		}
		if len(via) > op.redirects {
			return errors.New("redirect limit reached")
		}
		if next.URL.Scheme != "http" && next.URL.Scheme != "https" || via[len(via)-1].URL.Scheme == "https" && next.URL.Scheme != "https" {
			return errors.New("redirect scheme is not allowed")
		}
		return nil
	}}
	response, err := client.Do(request)
	if response == nil {
		return nil, err
	}
	defer response.Body.Close()
	result = map[string]any{"status": response.StatusCode, "url": response.Request.URL.String(), "complete": false}
	headers := make(map[string]any, len(response.Header))
	for key, values := range response.Header {
		headers[key] = values
	}
	result["headers"] = headers
	if err != nil {
		return result, err
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, int64(op.limit)+1))
	result["body"] = data[:min(len(data), op.limit)]
	result["truncated"] = len(data) > op.limit
	result["complete"] = readErr == nil && len(data) <= op.limit
	if readErr != nil {
		return result, readErr
	}
	return result, ctx.Err()
}
