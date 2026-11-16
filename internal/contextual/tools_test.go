package contextual

import (
	"strings"
	"testing"
)

func TestOutputToolRequestsRemainSeparateFromCommandRequests(t *testing.T) {
	t.Parallel()
	text := "```wraith-tool\n{\"tool\":\"output\",\"id\":\"0123456789abcdef01234567\",\"pattern\":\"wraith::TARGET\",\"before\":1}\n```"
	tool, err := RequestedTool(text)
	if err != nil || tool == nil || tool.Before != 1 || tool.Pattern != "wraith::TARGET" {
		t.Fatalf("invalid tool parse: %+v %v", tool, err)
	}
	if command, err := RequestedCommand(text); err != nil || command != nil {
		t.Fatal("read tool became executable")
	}
	display := DisplayContent(text)
	if strings.Contains(display, "wraith-tool") || !strings.Contains(display, "output · command") {
		t.Fatal("protocol marker exposed")
	}
}

func TestActionDecodingRejectsDuplicatesUnknownFieldsAndWrongTypes(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`{"command":"true","command":"false","reason":"test"}`,
		`{"command":"true","reason":"test","background":"yes"}`,
		`{"command":"true","reason":"test","unknown":1}`,
		`{"command":"true","reason":"test"} {}`,
	} {
		if _, err := RequestedCommand("```wraith-run\n" + body + "\n```"); err == nil {
			t.Errorf("accepted invalid command %s", body)
		}
	}
	for _, body := range []string{
		`{"tool":"output","id":"../file"}`,
		`{"tool":"execute","id":"0123456789abcdef01234567"}`,
		`{"tool":"poll","id":"0123456789abcdef01234567","id":"other"}`,
	} {
		if _, err := RequestedTool("```wraith-tool\n" + body + "\n```"); err == nil {
			t.Errorf("accepted invalid tool %s", body)
		}
	}
	background, err := RequestedCommand("```wraith-run\n{\"command\":\"sleep 1\",\"reason\":\"observe\",\"background\":true}\n```")
	if err != nil || background == nil || !background.Background {
		t.Fatalf("background request lost: %v", err)
	}
}
