package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

func TestAnthropicSeparatesSystemAndStreamsThinking(t *testing.T) {
	a := API{Config: config.Provider{Kind: "anthropic", BaseURL: "https://api.invalid/v1", APIKey: "private", ThinkingMode: "adaptive"}, HTTP: &http.Client{}}
	a.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("x-api-key") != "private" || r.Header.Get("anthropic-version") == "" {
			t.Error("anthropic headers missing")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["system"] != "instructions" || len(body["messages"].([]any)) != 1 {
			t.Error("system prompt was not separated")
		}
		return response("data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":42}}}\n\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"thinking\":\"reasoning\"}}\n\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"answer\"}}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"max_tokens\"},\"usage\":{\"output_tokens\":11}}\n\ndata: {\"type\":\"message_stop\"}\n\n"), nil
	})
	result, err := a.Anthropic(context.Background(), ollama.Request{Model: "model", Messages: []session.Message{{Role: "system", Content: "instructions"}, {Role: "user", Content: "question"}}, Think: "high", Options: ollama.Options{Predict: 2048}}, nil)
	if err != nil || !result.Done || result.Reason != "length" || result.Message.Thinking != "reasoning" || result.Message.Content != "answer" || result.PromptCount != 42 {
		t.Fatalf("result %+v %v", result, err)
	}
}
