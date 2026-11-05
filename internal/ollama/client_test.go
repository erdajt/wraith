package ollama

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fixture(body string, status int) *Client {
	return &Client{Base: "http://localhost", HTTP: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
}

func TestStreamingKeepsThinkingAndMetrics(t *testing.T) {
	c := fixture(`{"message":{"thinking":"plan"}}
{"message":{"content":"hello "}}
{"message":{"content":"world"},"done":true,"prompt_eval_count":50,"eval_count":2,"eval_duration":1000000000}
`, 200)
	count := 0
	r, err := c.Chat(context.Background(), Request{}, func(Response) { count++ })
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 || r.Message.Content != "hello world" || r.Message.Thinking != "plan" || r.Usage(20).TokensPerSecond != 2 {
		t.Fatalf("bad stream: %+v", r)
	}
}

func TestStreamFailures(t *testing.T) {
	for _, body := range []string{`{"message":{"content":"partial"}}`, `not json`, `{"error":"out of memory"}`} {
		if _, err := fixture(body, 200).Chat(context.Background(), Request{}, nil); err == nil {
			t.Fatalf("accepted broken stream: %s", body)
		}
	}
	if _, err := fixture("missing model", 404).Chat(context.Background(), Request{}, nil); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("bad http error: %v", err)
	}
}

func TestThinkingCapabilities(t *testing.T) {
	var info Info
	info.Thinking.Values = []any{true, false}
	if _, err := info.Think("high"); err == nil {
		t.Fatal("invented a native effort level")
	}
	v, err := info.Think("off")
	if err != nil || v != false {
		t.Fatal("boolean thinking failed")
	}
	info.Thinking.Values = []any{"low", "medium", "high"}
	if v, err = info.Think("high"); err != nil || v != "high" {
		t.Fatal("native effort rejected")
	}
}
