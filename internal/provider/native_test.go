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
	"github.com/erdajt/wraith/internal/variables"
)

func TestOllamaNativeActionContinuesWithPrivateCanonicalHistory(t *testing.T) {
	t.Parallel()
	client := New(config.Default())
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	request := ollama.Request{Capabilities: []string{"tools"}, ExecutionMode: "agent", Messages: []session.Message{{Role: "user", Content: "inspect private.example"}}}
	calls := 0
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body["tools"]) == 0 || strings.Contains(string(body["messages"]), "private.example") {
			t.Fatal("missing tools or private value leaked")
		}
		if calls == 2 {
			for _, forbidden := range []string{"tool_calls", `\"role\":\"tool\"`, "internal thinking"} {
				if strings.Contains(string(body["messages"]), forbidden) {
					t.Fatalf("native history retained %q", forbidden)
				}
			}
			if !strings.Contains(string(body["messages"]), "wraith-run") || !strings.Contains(string(body["messages"]), "completed") {
				t.Fatal("canonical action and evidence missing")
			}
			return response(`{"done":true,"done_reason":"stop","message":{"content":"finished"}}`), nil
		}
		return response(`{"message":{"thinking":"internal thinking"}}
{"message":{"tool_calls":[{"function":{"name":"request_command","arguments":{"command":"printf wraith::TARGET","reason":"check output"}}}]}}
{"done":true,"done_reason":"stop"}`), nil
	})
	var streamed strings.Builder
	result, err := client.ChatWithRepair(t.Context(), request, func(update StreamUpdate) { streamed.WriteString(update.Chunk.Message.Content) })
	command, decodeErr := contextual.RequestedCommand(result.Message.Content)
	if err != nil || decodeErr != nil || command == nil || command.Command != "printf wraith::TARGET" || streamed.String() != result.Message.Content || len(result.Calls) > 0 {
		t.Fatalf("native approval request failed: %+v, %v, %v", result, err, decodeErr)
	}
	request.Messages = append(request.Messages, result.Message, session.Message{Role: "user", Content: "command completed: private.example"})
	if _, err := client.ChatWithRepair(t.Context(), request, nil); err != nil || calls != 2 {
		t.Fatalf("canonical continuation failed: %v", err)
	}
}

func TestNativeActionFailuresUseBoundedRepair(t *testing.T) {
	t.Parallel()
	for _, arguments := range []string{`{"command":"printf wraith::MISSING","reason":"check"}`, `{"command":"true","reason":"check","extra":true}`} {
		client := New(config.Default())
		calls, retries := 0, 0
		client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
			calls++
			return response(`{"message":{"tool_calls":[{"function":{"name":"request_command","arguments":` + arguments + `}}]}}` + "\n" + `{"done":true,"done_reason":"stop"}`), nil
		})
		result, err := client.ChatWithRepair(t.Context(), ollama.Request{Capabilities: []string{"tools"}, ExecutionMode: "agent"}, func(update StreamUpdate) {
			if update.Retry > 0 {
				retries++
			}
		})
		if err == nil || calls != 4 || retries != 3 || !result.Message.Interrupted || len(result.Calls) > 0 {
			t.Fatalf("invalid native request escaped repair bounds: calls=%d retries=%d result=%+v err=%v", calls, retries, result, err)
		}
	}
}
