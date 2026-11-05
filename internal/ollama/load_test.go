package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"syscall"
	"testing"
)

type loadTransport func(*http.Request) (*http.Response, error)

func (f loadTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func loadResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}
}

func TestPreloadUsesEmptyPromptAndConfiguredMemory(t *testing.T) {
	c := New("http://localhost:11434")
	c.HTTP.Transport = loadTransport(func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if r.URL.Path != "/api/generate" || payload["prompt"] != "" || payload["stream"] != false || payload["model"] != "chosen" || payload["keep_alive"] != "2m" {
			t.Fatalf("unexpected preload: %v", payload)
		}
		if payload["options"].(map[string]any)["num_ctx"] != float64(8192) {
			t.Fatal("context not applied")
		}
		return loadResponse(`{"done":true,"done_reason":"load"}`), nil
	})
	if err := c.Load(context.Background(), "chosen", "2m", Options{Context: 8192}); err != nil {
		t.Fatal(err)
	}
}

func TestServerStartupOnlyForRefusedLoopbackConnections(t *testing.T) {
	for _, tt := range []struct {
		host    string
		failure error
		start   bool
	}{{"http://127.0.0.1:11434", syscall.ECONNREFUSED, true}, {"http://example.com", syscall.ECONNREFUSED, false}, {"http://127.0.0.1", syscall.EPERM, false}, {"https://localhost", syscall.ECONNREFUSED, false}} {
		t.Run(tt.host+tt.failure.Error(), func(t *testing.T) {
			c := New(tt.host)
			started := false
			c.HTTP.Transport = loadTransport(func(*http.Request) (*http.Response, error) {
				if started {
					return loadResponse(`{"models":[]}`), nil
				}
				return nil, tt.failure
			})
			err := c.ensureServer(context.Background(), true, func(string) error { started = true; return nil })
			if started != tt.start || (err == nil) != tt.start {
				t.Fatalf("started=%t err=%v", started, err)
			}
		})
	}
}
