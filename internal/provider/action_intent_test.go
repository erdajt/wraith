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

func TestAmbiguousCommandRepairRequiresModelToChooseIntent(t *testing.T) {
	t.Parallel()
	object := `{"command":"printf ready","reason":"check output"}`
	for _, malformed := range []string{object, "```json\n" + object + "\n```", "```text\nwraith-run " + object + "\n```"} {
		for _, execute := range []bool{false, true} {
			t.Run(malformed+"/"+map[bool]string{false: "example", true: "execution"}[execute], func(t *testing.T) {
				t.Parallel()
				client := New(config.Default())
				calls, retries := 0, 0
				client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
					calls++
					var req ollama.Request
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Fatal(err)
					}
					content := malformed
					if calls > 1 {
						feedback := req.Messages[len(req.Messages)-1].Content
						if !strings.Contains(feedback, "Only when requesting execution") || !strings.Contains(feedback, "For an example") {
							t.Fatal("repair must preserve both possible intents")
						}
						content = "```sh\nprintf ready\n```"
						if execute {
							content = "```wraith-run\n" + object + "\n```"
						}
					}
					data, _ := json.Marshal(ollama.Response{Done: true, Message: session.Message{Role: "assistant", Content: content}})
					return response(string(data)), nil
				})
				result, err := client.ChatWithRepair(t.Context(), ollama.Request{ExecutionMode: "agent"}, func(update StreamUpdate) {
					if update.Retry > 0 {
						retries++
						if request, _ := contextual.RequestedCommand(update.Rejected.Content); request != nil {
							t.Fatal("rejected answer became actionable")
						}
					}
				})
				request, decodeErr := contextual.RequestedCommand(result.Message.Content)
				if err != nil || decodeErr != nil || calls != 2 || retries != 1 || (request != nil) != execute {
					t.Fatalf("intent repair failed: calls=%d retries=%d request=%+v err=%v decode=%v", calls, retries, request, err, decodeErr)
				}
			})
		}
	}
}

func TestExamplesAndNonAgentRequestsDoNotTriggerIntentRepair(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, content string
		request       ollama.Request
	}{
		{"shell", "```sh\nprintf ready\n```", ollama.Request{ExecutionMode: "agent"}},
		{"placeholder", "```json\n{\"command\":\"curl <TARGET>\",\"reason\":\"example\"}\n```", ollama.Request{ExecutionMode: "agent"}},
		{"blocks", `{"command":"printf ready","reason":"example"}`, ollama.Request{ExecutionMode: "blocks"}},
		{"compaction", `{"command":"printf ready","reason":"example"}`, ollama.Request{ExecutionMode: "agent", CompactionTask: "extract"}},
		{"format", `{"command":"printf ready","reason":"example"}`, ollama.Request{ExecutionMode: "agent", Format: "json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := New(config.Default())
			calls := 0
			client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
				calls++
				data, _ := json.Marshal(ollama.Response{Done: true, Message: session.Message{Role: "assistant", Content: tc.content}})
				return response(string(data)), nil
			})
			result, err := client.ChatWithRepair(t.Context(), tc.request, nil)
			if err != nil || calls != 1 || result.Message.Content != tc.content {
				t.Fatalf("unexpected repair: calls=%d err=%v", calls, err)
			}
		})
	}
}
