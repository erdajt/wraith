package provider

import (
	"net/http"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
)

func TestNativeCompletionCannotBeReplacedOrExtended(t *testing.T) {
	for _, kind := range []string{"openai", "anthropic"} {
		for _, test := range []string{"truncated-then-tool", "tool-then-truncated", "duplicate", "late-text", "usage"} {
			t.Run(kind+"/"+test, func(t *testing.T) {
				first, finish, truncated, late, usage, end := "", "", "", "", "", ""
				if kind == "openai" {
					first = "data: " + `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"one","type":"function","function":{"name":"request_command","arguments":"{\"command\":\"printf ready\",\"reason\":\"test\"}"}}]}}]}` + "\n\n"
					finish = "data: " + `{"choices":[{"finish_reason":"tool_calls"}]}` + "\n\n"
					truncated = "data: " + `{"choices":[{"finish_reason":"length"}]}` + "\n\n"
					late = "data: " + `{"choices":[{"delta":{"content":"late"}}]}` + "\n\n"
					usage = "data: " + `{"choices":[],"usage":{"completion_tokens":12}}` + "\n\n"
					end = "data: [DONE]\n\n"
				} else {
					for _, event := range []string{
						`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"one","name":"request_command","input":{}}}`,
						`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"command\":\"printf ready\",\"reason\":\"test\"}"}}`,
						`{"type":"content_block_stop","index":0}`,
					} {
						first += "data: " + event + "\n\n"
					}
					finish = "data: " + `{"type":"message_delta","delta":{"stop_reason":"tool_use"}}` + "\n\n"
					truncated = "data: " + `{"type":"message_delta","delta":{"stop_reason":"max_tokens"}}` + "\n\n"
					late = "data: " + `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"late"}}` + "\n\n"
					usage = "data: " + `{"type":"message_delta","usage":{"output_tokens":12}}` + "\n\n"
					end = "data: " + `{"type":"message_stop"}` + "\n\n"
				}
				tail := map[string]string{
					"truncated-then-tool": truncated + finish,
					"tool-then-truncated": finish + truncated,
					"duplicate":           finish + finish,
					"late-text":           finish + late,
					"usage":               finish + usage,
				}[test]
				api := &API{Config: config.DefaultProviders()[kind], HTTP: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) { return response(first + tail + end), nil })}}
				req := ollama.Request{Tools: contextual.ActionTools("agent")}
				var result ollama.Response
				var err error
				if kind == "openai" {
					result, err = api.OpenAI(t.Context(), req, nil)
				} else {
					result, err = api.Anthropic(t.Context(), req, nil)
				}
				if test == "usage" {
					if err != nil || len(result.Calls) != 1 || result.EvalCount != 12 {
						t.Fatalf("usage frame lost valid completion: %+v %v", result, err)
					}
				} else if err == nil || len(result.Calls) != 0 {
					t.Fatalf("invalid terminal state became actionable: %+v %v", result, err)
				}
			})
		}
	}
}
