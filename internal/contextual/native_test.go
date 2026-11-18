package contextual

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/ollama"
)

func TestNativeRequestsUseCanonicalActionValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, mode, tool, arguments string
		valid                       bool
	}{
		{"command", "agent", "request_command", `{"command":"printf ready\nprintf done\t","reason":"check"}`, true},
		{"block request", "blocks", "request_command", `{"command":"true","reason":"check"}`, true},
		{"private reference", "agent", "request_command", `{"command":"printf wraith::TARGET","reason":"check"}`, true},
		{"placeholder", "agent", "request_command", `{"command":"curl <TARGET>","reason":"check"}`, false},
		{"duplicate", "agent", "request_command", `{"command":"true","command":"false","reason":"check"}`, false},
		{"unknown field", "agent", "request_command", `{"command":"true","reason":"check","execute":true}`, false},
		{"control", "agent", "request_command", `{"command":"printf \u001b","reason":"check"}`, false},
		{"poll", "agent", "read_output", `{"tool":"poll","id":"0123456789abcdef01234567"}`, true},
		{"query", "agent", "read_output", `{"tool":"output","id":"0123456789abcdef01234567","pattern":"ready","limit":20}`, true},
		{"bounds", "agent", "read_output", `{"tool":"output","id":"0123456789abcdef01234567","limit":201}`, false},
		{"blocks output", "blocks", "read_output", `{"tool":"poll","id":"0123456789abcdef01234567"}`, false},
		{"unknown function", "agent", "exec", `{}`, false},
		{"no mode", "", "request_command", `{"command":"true","reason":"check"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			content, err := NormalizeAction(tc.mode, "checking", []ollama.Call{{Name: tc.tool, Arguments: json.RawMessage(tc.arguments)}})
			if (err == nil) != tc.valid {
				t.Fatalf("unexpected validation: %v", err)
			}
			if !tc.valid {
				if content != "" {
					t.Fatal("rejected request retained actionable content")
				}
				return
			}
			if tc.tool == "request_command" {
				request, err := RequestedCommand(content)
				if err != nil || request == nil || !strings.HasPrefix(DisplayContent(content), "checking") {
					t.Fatal("native command did not enter the existing approval format")
				}
			} else if request, err := RequestedTool(content); err != nil || request == nil {
				t.Fatal("native output request did not enter the existing evidence format")
			}
		})
	}
}

func TestMixedOrMultipleNativeRequestsAreRejected(t *testing.T) {
	t.Parallel()
	call := ollama.Call{Name: "request_command", Arguments: json.RawMessage(`{"command":"true","reason":"check"}`)}
	for _, calls := range [][]ollama.Call{nil, {call, call}} {
		if _, err := NormalizeAction("agent", "", calls); err == nil {
			t.Fatal("accepted wrong call count")
		}
	}
	for _, content := range []string{"```wraith-run\n{}\n```", "```text\nwraith-run {}\n```", string(call.Arguments)} {
		if _, err := NormalizeAction("agent", content, []ollama.Call{call}); err == nil {
			t.Fatal("accepted mixed actions")
		}
	}
}
