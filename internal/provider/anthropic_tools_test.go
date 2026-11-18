package provider

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

func TestAnthropicNativeCallUsesCanonicalContinuationWithoutThinkingEnvelope(t *testing.T) {
	t.Parallel()
	c := config.Default()
	c.Provider = "anthropic"
	profile := c.Providers[c.Provider]
	profile.ActionProtocol = "native"
	c.Providers[c.Provider] = profile
	client := New(c)
	calls := 0
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body["tools"]), "input_schema") || !strings.Contains(string(body["tool_choice"]), "disable_parallel_tool_use") {
			t.Fatal("native schema missing")
		}
		if calls == 2 {
			for _, forbidden := range []string{"tool_use", "tool_result", "signature", "private thinking", "provider_id"} {
				if strings.Contains(string(body["messages"]), forbidden) {
					t.Fatalf("native history retained %q", forbidden)
				}
			}
			return response("data: " + `{"type":"content_block_delta","delta":{"type":"text_delta","text":"finished"}}` + "\n\ndata: " + `{"type":"message_stop"}` + "\n\n"), nil
		}
		var stream strings.Builder
		for _, event := range []string{
			`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"private thinking"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"signed"}}`,
			`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"provider_id","name":"request_command","input":{}}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"command\":\"printf "}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"ready\",\"reason\":\"check\"}"}}`,
			`{"type":"content_block_stop","index":1}`,
			`{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
			`{"type":"message_stop"}`,
		} {
			stream.WriteString("data: " + event + "\n\n")
		}
		return response(stream.String()), nil
	})
	request := ollama.Request{ExecutionMode: "agent", Options: ollama.Options{Predict: 2048}, Think: true}
	result, err := client.ChatWithRepair(t.Context(), request, nil)
	command, decodeErr := contextual.RequestedCommand(result.Message.Content)
	if err != nil || decodeErr != nil || command == nil || command.Command != "printf ready" {
		t.Fatalf("native call failed: %+v, %v, %v", result, err, decodeErr)
	}
	request.Messages = []session.Message{result.Message, {Role: "user", Content: "command completed"}}
	if _, err := client.ChatWithRepair(t.Context(), request, nil); err != nil || calls != 2 {
		t.Fatalf("canonical continuation failed: %v", err)
	}
}

func TestAnthropicRejectsIncompleteAndConflictingToolBlocks(t *testing.T) {
	t.Parallel()
	start := "data: " + `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"one","name":"request_command","input":{}}}` + "\n\n"
	for _, tail := range []string{
		"",
		start,
		"data: " + `{"type":"message_delta","delta":{"stop_reason":"max_tokens"}}` + "\n\ndata: " + `{"type":"message_stop"}` + "\n\n",
		"data: " + `{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{}"}}` + "\n\n",
		"data: " + `{"type":"message_delta","delta":{"stop_reason":"tool_use"}}` + "\n\ndata: " + `{"type":"message_stop"}` + "\n\n",
	} {
		api := &API{Config: config.DefaultProviders()["anthropic"], HTTP: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) { return response(start + tail), nil })}}
		result, err := api.Anthropic(t.Context(), ollama.Request{Tools: contextual.ActionTools("agent")}, nil)
		if err == nil || len(result.Calls) > 0 {
			t.Fatal("failed stream retained actionable call")
		}
	}
}
