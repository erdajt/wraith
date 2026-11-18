package contextual

import (
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/variables"
)

func TestActionRedactionPreservesKeysAndKnownIds(t *testing.T) {
	snapshot := variables.NewSnapshot([]variables.Entry{{Name: "TOKEN", Value: "ab"}, {Name: "FIELD", Value: "command"}}, nil)
	id := strings.Repeat("ab", 12)
	content := "check ab\n```wraith-tool\n{\"tool\":\"output\",\"id\":\"" + id + "\",\"pattern\":\"ab\"}\n```"
	redacted := RedactActions(content, snapshot.Redact, map[string]bool{id: true})
	request, err := RequestedTool(redacted)
	if err != nil || request == nil || request.ID != id || request.Pattern != "wraith::TOKEN" || !strings.HasPrefix(redacted, "check wraith::TOKEN") {
		t.Fatalf("tool redaction: %s %v", redacted, err)
	}

	command := "```wraith-run\n{\"command\":\"printf ab\",\"reason\":\"inspect command\"}\n```"
	requestCommand, err := RequestedCommand(RedactActions(command, snapshot.Redact, nil))
	if err != nil || requestCommand == nil || requestCommand.Command != "printf wraith::TOKEN" || requestCommand.Reason != "inspect wraith::FIELD" {
		t.Fatalf("command redaction: %+v %v", requestCommand, err)
	}
}

func TestRedactionDoesNotTurnSuggestionsIntoActions(t *testing.T) {
	snapshot := variables.NewSnapshot([]variables.Entry{{Name: "TOKEN", Value: "ab"}}, nil)
	content := "```sh\nwraith-run {\"command\":\"printf ab\",\"reason\":\"example\"}\n```"
	redacted := RedactActions(content, snapshot.Redact, nil)
	request, err := RequestedCommand(redacted)
	if err != nil || request != nil || !strings.Contains(redacted, "wraith::TOKEN") {
		t.Fatalf("suggestion became actionable: %s", redacted)
	}
}
