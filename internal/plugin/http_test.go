package plugin

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHTTPPreservesStatusBytesAndBoundedBodies(t *testing.T) {
	url, transport := httpFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/status", http.StatusFound)
			return
		}
		w.Header().Add("X-Value", "first")
		w.Header().Add("X-Value", "second")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte{255, 0, 'a', 'b', 'c'})
	}), false)
	op := httpOperation{url: url + "/status", method: "GET", timeout: time.Second, limit: 10, transport: transport}
	got, err := op.run(t.Context())
	if err != nil || got["status"] != http.StatusTeapot || got["complete"] != true || !bytes.Equal(got["body"].([]byte), []byte{255, 0, 'a', 'b', 'c'}) {
		t.Fatalf("non-success response: %+v, %v", got, err)
	}
	if values := got["headers"].(map[string]any)["X-Value"].([]string); len(values) != 2 {
		t.Fatalf("repeated headers lost: %v", values)
	}
	op.limit = 2
	got, err = op.run(t.Context())
	if err != nil || got["truncated"] != true || got["complete"] != false || !bytes.Equal(got["body"].([]byte), []byte{255, 0}) {
		t.Fatalf("bounded response: %+v, %v", got, err)
	}
	op.url, op.limit = url+"/redirect", 1024
	got, err = op.run(t.Context())
	if err != nil || got["status"] != http.StatusFound {
		t.Fatalf("redirect followed by default: %+v, %v", got, err)
	}
	op.redirects = 1
	got, err = op.run(t.Context())
	if err != nil || got["status"] != http.StatusTeapot {
		t.Fatalf("explicit redirect failed: %+v, %v", got, err)
	}
}

func TestHTTPCancellationPreservesPartialResponse(t *testing.T) {
	started := make(chan struct{})
	url, transport := httpFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("prefix"))
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}), false)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	op := httpOperation{url: url, method: "GET", timeout: time.Second, limit: 1024, transport: transport}
	type outcome struct {
		data map[string]any
		err  error
	}
	finished := make(chan outcome, 1)
	go func() {
		data, err := op.run(ctx)
		finished <- outcome{data, err}
	}()
	<-started
	cancel()
	select {
	case got := <-finished:
		if !errors.Is(got.err, context.Canceled) || got.data != nil && got.data["complete"] != false {
			t.Fatalf("cancelled response: %+v, %v", got.data, got.err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled HTTP request did not finish")
	}
}

func TestHTTPRejectsUntrustedTLSAndReportsTimeout(t *testing.T) {
	url, transport := httpFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), true)
	op := httpOperation{url: url, method: "GET", timeout: time.Second, limit: 10, transport: transport}
	if _, err := op.run(t.Context()); err == nil {
		t.Fatal("untrusted TLS certificate accepted")
	}
	op.timeout = time.Nanosecond
	if _, err := op.run(t.Context()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("request deadline was not preserved: %v", err)
	}
}

func TestHTTPReportsPartialBodiesAndRedirectLoops(t *testing.T) {
	url, transport := httpFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/loop" {
			http.Redirect(w, r, "/loop", http.StatusFound)
			return
		}
		w.Header().Set("Content-Length", "20")
		_, _ = w.Write([]byte("prefix"))
	}), false)
	op := httpOperation{url: url, method: "GET", timeout: time.Second, limit: 100, transport: transport}
	got, err := op.run(t.Context())
	if !errors.Is(err, io.ErrUnexpectedEOF) || got["complete"] != false || string(got["body"].([]byte)) != "prefix" {
		t.Fatalf("partial body discarded: %+v, %v", got, err)
	}
	op.url, op.redirects = url+"/loop", 2
	got, err = op.run(t.Context())
	if err == nil || !strings.Contains(err.Error(), "redirect limit") || got["status"] != http.StatusFound {
		t.Fatalf("redirect loop result: %+v, %v", got, err)
	}
}
