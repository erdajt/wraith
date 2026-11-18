package provider

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
)

func TestOpenAINativeArgumentsStreamIntoOneApprovalRequest(t *testing.T) {
	t.Parallel()
	c := config.Default()
	c.Provider = "openai"
	profile := c.Providers[c.Provider]
	profile.ActionProtocol = "native"
	c.Providers[c.Provider] = profile
	client := New(c)
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body["tools"]) == 0 || string(body["parallel_tool_calls"]) != "false" {
			t.Fatal("missing serialized tool contract")
		}
		return response("data: " + `{"choices":[{"index":0,"delta":{"content":"checking\n","tool_calls":[{"index":0,"id":"call_one","type":"function","function":{"name":"request_command","arguments":"{\"command\":\"printf "}}]}}]}` + "\n\n" +
			"data: " + `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ready\",\"reason\":\"check\"}"}}]},"finish_reason":"tool_calls"}]}` + "\n\ndata: [DONE]\n\n"), nil
	})
	var streamed strings.Builder
	result, err := client.ChatWithRepair(t.Context(), ollama.Request{ExecutionMode: "agent"}, func(update StreamUpdate) { streamed.WriteString(update.Chunk.Message.Content) })
	request, decodeErr := contextual.RequestedCommand(result.Message.Content)
	if err != nil || decodeErr != nil || request == nil || request.Command != "printf ready" || streamed.String() != result.Message.Content {
		t.Fatalf("streamed call failed: %+v, %v, %v", result, err, decodeErr)
	}
	if !strings.HasPrefix(streamed.String(), "checking\n\n\n") {
		t.Fatal("normalization changed an already streamed prefix")
	}
}

func TestOpenAINativeFailuresNeverReturnCalls(t *testing.T) {
	t.Parallel()
	first := "data: " + `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"one","function":{"name":"request_command","arguments":"{}"}}]}}]}` + "\n\n"
	for _, tail := range []string{
		"",
		"data: " + `{"choices":[{"finish_reason":"length"}]}` + "\n\ndata: [DONE]\n\n",
		"data: " + `{"choices":[{"delta":{"tool_calls":[{"index":1,"id":"two"}]}}]}` + "\n\n",
		"data: " + `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"two"}]}}]}` + "\n\n",
		"data: " + `{"error":{"message":"failed"}}` + "\n\n",
		"data: " + `{"choices":[{"finish_reason":"tool_calls"}]}` + "\n\ndata: [DONE]\n\ndata: " + `{"choices":[{"delta":{"content":"late"}}]}` + "\n\n",
	} {
		api := &API{Config: config.DefaultProviders()["openai"], HTTP: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) { return response(first + tail), nil })}}
		result, err := api.OpenAI(t.Context(), ollama.Request{Tools: contextual.ActionTools("agent")}, func(chunk ollama.Response) {
			if len(chunk.Calls) != 0 {
				t.Fatal("unvalidated call was emitted")
			}
		})
		if err == nil || len(result.Calls) > 0 {
			t.Fatal("failed stream returned a call")
		}
	}
}
