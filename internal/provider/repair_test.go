package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

func TestRepairStreamsReplacementsAndExplainsRejection(t *testing.T) {
	t.Parallel()
	client := New(config.Default())
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	calls, retries := 0, 0
	var streamed strings.Builder
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		if strings.Contains(string(body), "private.example") {
			t.Fatal("private value transmitted")
		}

		if calls > 1 && !strings.Contains(string(body), "unknown variable MISSING") {
			t.Fatal("repair omitted failure")
		}

		chunks := []string{"checking ", "wraith::MIS", "SING "}
		if calls > 1 {
			chunks = []string{"inspect ", "wraith:", ":TARGET", " now"}
		}

		var out strings.Builder
		for _, text := range chunks {
			data, _ := json.Marshal(ollama.Response{Message: session.Message{Role: "assistant", Content: text}})
			fmt.Fprintln(&out, string(data))
		}

		fmt.Fprintln(&out, `{"done":true,"message":{"role":"assistant"}}`)
		return response(out.String()), nil
	})
	result, err := client.ChatWithRepair(t.Context(), ollama.Request{PrivateInventory: true}, func(update StreamUpdate) {
		if update.Retry > 0 {
			retries++
			streamed.Reset()
			return
		}

		streamed.WriteString(update.Chunk.Message.Content)

		if strings.Contains(streamed.String(), "wraith::MIS") {
			t.Fatal("incomplete marker emitted")
		}
	})
	if err != nil || calls != 2 || retries != 1 || result.Message.Content != "inspect wraith::TARGET now" || streamed.String() != result.Message.Content {
		t.Fatalf("repair failed: calls=%d retries=%d result=%q streamed=%q err=%v", calls, retries, result.Message.Content, streamed.String(), err)
	}
}

func TestRepairStopsAfterThreeRetriesAndRetainsInvalidAnswer(t *testing.T) {
	t.Parallel()
	client := New(config.Default())
	calls := 0
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		return response("{\"message\":{\"role\":\"assistant\",\"content\":\"wraith::MISSING \"}}\n{\"done\":true}\n"), nil
	})
	result, err := client.ChatWithRepair(t.Context(), ollama.Request{}, nil)
	if calls != 4 || err == nil || !strings.Contains(err.Error(), "3 retries") || !strings.Contains(result.Message.Content, "wraith::MISSING") || !result.Message.Interrupted {
		t.Fatalf("unexpected exhaustion: calls=%d result=%+v err=%v", calls, result, err)
	}
}

func TestRepairDoesNotRetryTransportFailuresOrOverflow(t *testing.T) {
	t.Parallel()

	for _, transportFailure := range []bool{true, false} {
		client := New(config.Default())
		calls := 0
		client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
			calls++

			if transportFailure {
				return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("unauthorized"))}, nil
			}
			return response("{\"message\":{\"content\":\"wraith::MISSING \"}}\n{\"done\":true}\n"), nil
		})
		_, err := client.ChatWithRepair(t.Context(), ollama.Request{Options: ollama.Options{Context: 240, Predict: 64}}, nil)
		if err == nil || calls != 1 {
			t.Fatalf("unexpected retry: calls=%d err=%v", calls, err)
		}
	}
}

func TestRepairCorrectsOutputQueryChannelOnNextAttempt(t *testing.T) {
	t.Parallel()
	client := New(config.Default())
	id := "0123456789abcdef01234567"
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "REFERENCE", Value: "wraith-tool"}, {Name: "TARGET", Value: "private.example"}}, nil)
	query := `{"tool":"output","id":"` + id + `","pattern":"open","regex":true,"from":1,"limit":40}`
	calls, retries := 0, 0
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		var req ollama.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}

		channel := "wraith-run"
		if calls > 1 {
			feedback := req.Messages[len(req.Messages)-1].Content
			if strings.Contains(feedback, "private.example") {
				t.Fatal("repair leaked private user content")
			}

			for _, want := range []string{"output query sent as a command request", "fenced wraith-tool block instead of wraith-run", "one-based output line number"} {
				if !strings.Contains(feedback, want) {
					t.Fatalf("repair omitted %q", want)
				}
			}
			channel = "wraith-tool"
		}

		data, err := json.Marshal(ollama.Response{Done: true, Message: session.Message{Role: "assistant", Content: "```" + channel + "\n" + query + "\n```"}})
		if err != nil {
			t.Fatal(err)
		}
		return response(string(data)), nil
	})
	result, err := client.ChatWithRepair(t.Context(), ollama.Request{RunIDs: map[string]bool{id: true}, Messages: []session.Message{{Role: "user", Content: "inspect private.example"}}}, func(update StreamUpdate) {
		if update.Retry > 0 {
			retries++

			if strings.Contains(update.Problem, "wraith-") {
				t.Fatal("retry status exposed internal protocol")
			}
		}
	})
	if err != nil || calls != 2 || retries != 1 || result.Message.Interrupted {
		t.Fatalf("query did not recover: calls=%d retries=%d err=%v", calls, retries, err)
	}

	if request, err := contextual.RequestedTool(result.Message.Content); err != nil || request == nil || request.From != 1 {
		t.Fatalf("corrected output request lost: %+v, %v", request, err)
	}
}

func TestRepairCorrectsBarePrivateNamesInSuggestionsAndRequests(t *testing.T) {
	t.Parallel()

	for _, actionable := range []bool{false, true} {
		t.Run(fmt.Sprint(actionable), func(t *testing.T) {
			t.Parallel()
			client := New(config.Default())
			client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET_DOMAIN", Value: "private.example"}}, nil)
			calls := 0
			client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
				calls++
				var req ollama.Request
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatal(err)
				}

				reference := "TARGET_DOMAIN"
				if calls > 1 {
					feedback := req.Messages[len(req.Messages)-1].Content
					if !strings.Contains(feedback, "bare private variable TARGET_DOMAIN") || strings.Contains(feedback, "private.example") {
						t.Fatal("repair omitted correction or leaked the value")
					}
					reference = "wraith::TARGET_DOMAIN"
				}

				command := "dig +short " + reference
				content := "```bash\n" + command + "\n```"
				if actionable {
					data, _ := json.Marshal(contextual.CommandRequest{Command: command, Reason: "inspect the configured target"})
					content = "```wraith-run\n" + string(data) + "\n```"
				}

				data, _ := json.Marshal(ollama.Response{Done: true, Message: session.Message{Role: "assistant", Content: content}})
				return response(string(data)), nil
			})
			result, err := client.ChatWithRepair(t.Context(), ollama.Request{ExecutionMode: "agent", PrivateInventory: true}, nil)
			if err != nil || calls != 2 || !strings.Contains(result.Message.Content, "wraith::TARGET_DOMAIN") {
				t.Fatalf("bare name did not recover: calls=%d result=%q err=%v", calls, result.Message.Content, err)
			}

			request, err := contextual.RequestedCommand(result.Message.Content)
			if err != nil || (request != nil) != actionable {
				t.Fatal("repair changed a suggestion into an execution request")
			}

			expanded, _ := client.Variables.Expand(result.Message.Content)
			if !strings.Contains(expanded, "private.example") {
				t.Fatal("corrected reference did not resolve locally")
			}
		})
	}
}
