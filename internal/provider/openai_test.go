package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

type transport func(*http.Request) (*http.Response, error)

func (t transport) RoundTrip(r *http.Request) (*http.Response, error) { return t(r) }
func response(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

func TestCompatibleStreamPreservesReasoningUsageAndRequest(t *testing.T) {
	a := API{Config: config.Provider{BaseURL: "https://provider.invalid/v1", APIKey: "private", TokenField: "max_completion_tokens"}, HTTP: &http.Client{}}
	a.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer private" {
			t.Error("request routing or authentication missing")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["max_completion_tokens"] != float64(1234) || body["num_ctx"] != nil {
			t.Error("wrong generation options")
		}
		return response("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"check\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"answer\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":3}}\n\ndata: [DONE]\n\n"), nil
	})
	var streamed string
	result, err := a.OpenAI(context.Background(), ollama.Request{Model: "model", Messages: []session.Message{{Role: "user", Content: "question"}}, Options: ollama.Options{Predict: 1234}}, func(r ollama.Response) { streamed += r.Message.Content })
	if err != nil || !result.Done || result.Message.Content != "answer" || result.Message.Thinking != "check" || streamed != "answer" || result.PromptCount != 10 {
		t.Fatalf("result: %+v %v", result, err)
	}
}

func TestCompatibleStreamRejectsTruncationAndDoesNotExposeErrorBodies(t *testing.T) {
	a := API{Config: config.Provider{BaseURL: "https://provider.invalid"}, HTTP: &http.Client{}}
	a.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
		return response("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"), nil
	})
	result, err := a.OpenAI(context.Background(), ollama.Request{}, nil)
	if err == nil || result.Message.Content != "partial" {
		t.Fatal("truncated response accepted or lost")
	}
	a.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
		r := response("secret echoed token")
		r.StatusCode = 401
		return r, nil
	})
	_, err = a.Models(context.Background())
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("provider error leaked response body")
	}
}
