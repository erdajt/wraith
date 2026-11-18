package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
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

func TestModelMetadataKeepsCapabilitiesAndValidContextLimits(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		context    int
		failure    bool
	}{
		{"limits", `{"capabilities":["thinking","tools"],"model_info":{"a.context_length":32768,"b.context_length":8192,"bad.context_length":"large","negative.context_length":-1}}`, 8192, false},
		{"unknown", `{"capabilities":["tools"],"model_info":{"a.context_length":0}}`, 0, false},
		{"malformed", `{`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := New("http://localhost:11434")
			c.HTTP.Transport = loadTransport(func(r *http.Request) (*http.Response, error) {
				var payload map[string]string
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if r.Method != http.MethodPost || r.URL.Path != "/api/show" || payload["model"] != "selected-model" {
					t.Fatal("metadata request did not identify selected model")
				}
				return loadResponse(tc.body), nil
			})
			info, err := c.Show(t.Context(), "selected-model")
			if (err != nil) != tc.failure || info.ContextWindow != tc.context {
				t.Fatalf("metadata context=%d err=%v", info.ContextWindow, err)
			}
			if tc.name == "limits" {
				if value, err := info.Think("on"); err != nil || value != true {
					t.Fatal("advertised thinking capability lost")
				}
			}
		})
	}
}

func TestDetachedLauncherUsesSelectedHostAndPrivateLog(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "ollama")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf '%s\\n' \"$1\" \"$OLLAMA_HOST\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("OLLAMA_HOST", "unrelated-host")
	logPath := filepath.Join(directory, "logs", "ollama.log")
	if err := launchServer("127.0.0.1:12345", logPath); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("fake launcher did not finish")
		case <-poll.C:
			data, err := os.ReadFile(logPath)
			if err == nil && string(data) == "serve\n127.0.0.1:12345\n" {
				info, err := os.Stat(logPath)
				if err != nil || info.Mode().Perm() != 0600 {
					t.Fatal("server log is not private")
				}
				return
			}
		}
	}
}

func TestServerProbeAndLauncherFailuresStayLocal(t *testing.T) {
	c := New("http://localhost:11434")
	c.HTTP.Transport = loadTransport(func(*http.Request) (*http.Response, error) {
		return loadResponse(`{"models":[]}`), nil
	})
	t.Setenv("PATH", t.TempDir())
	logPath := filepath.Join(t.TempDir(), "unused.log")
	if err := c.EnsureServer(t.Context(), true, logPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatal("healthy server probe attempted to launch a process")
	}
	if err := launchServer("localhost:1", filepath.Join(t.TempDir(), "log")); err == nil {
		t.Fatal("missing executable reported startup success")
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := launchServer("localhost:1", filepath.Join(blocked, "log")); err == nil {
		t.Fatal("failed log creation reported startup success")
	}
}
