package contextual

import (
	"errors"
	"strings"
	"testing"
)

func TestWrongActionChannelExplainsCorrectionWithoutExposingProtocol(t *testing.T) {
	t.Parallel()
	query := `{"tool":"output","id":"0123456789abcdef01234567","pattern":"open","regex":true,"ignore_case":false,"from":1,"before":2,"after":2,"limit":40}`
	_, err := RequestedCommand("```wraith-run\n" + query + "\n```")
	var protocol *ActionProtocolError
	if !errors.As(err, &protocol) || !strings.Contains(protocol.Correction, "fenced wraith-tool") || !strings.Contains(protocol.Correction, "one-based output line") {
		t.Fatalf("missing targeted correction: %v", err)
	}
	if strings.Contains(err.Error(), "wraith-") {
		t.Fatal("user-facing error exposed protocol")
	}
	tool, err := RequestedTool("```wraith-tool\n" + query + "\n```")
	if err != nil || tool == nil || tool.Tool != "output" || tool.From != 1 {
		t.Fatalf("corrected query rejected: %+v, %v", tool, err)
	}
	command, err := RequestedCommand("```wraith-tool\n" + query + "\n```")
	if err != nil || command != nil {
		t.Fatal("output query became execution request")
	}
	_, err = RequestedTool("```wraith-tool\n{\"command\":\"printf test\",\"reason\":\"inspect\"}\n```")
	if !errors.As(err, &protocol) || !strings.Contains(protocol.Correction, "fenced wraith-run") || strings.Contains(err.Error(), "wraith-") {
		t.Fatalf("wrong command channel not explained: %v", err)
	}
}
