package contextual

import (
	"strings"
	"testing"
)

func TestCommandDisplayPreservesReasonAndHidesTransport(t *testing.T) {
	raw := `{"command":"printf evidence","reason":"inspect local output"}`
	for _, content := range []string{"wraith-run " + raw, "```wraith-run\n" + raw + "\n```", "~~~wraith-run\n" + raw + "\n~~~"} {
		request, err := RequestedCommand(content)
		if err != nil || request == nil || request.Command != "printf evidence" {
			t.Fatalf("request: %+v %v", request, err)
		}
		visible := DisplayContent(content)
		if strings.Contains(visible, "wraith-run") || !strings.Contains(visible, "reason: inspect local output") || !strings.Contains(visible, "```sh\nprintf evidence") {
			t.Fatalf("display: %s", visible)
		}
	}
}

func TestPlaceholderRequestsStaySuggestions(t *testing.T) {
	for _, command := range []string{"nmap <TARGET_IP_OR_HOST>", "curl https://YOUR_HOST", "ping {{target}}"} {
		content := `wraith-run {"command":"` + command + `","reason":"inspect the supplied target"}`
		request, err := RequestedCommand(content)
		if err != nil || request != nil {
			t.Fatalf("placeholder became actionable: %s", command)
		}
		if !strings.Contains(DisplayContent(content), command) {
			t.Fatal("suggestion disappeared")
		}
	}
}

func TestOrdinaryFencesCannotSmuggleRequests(t *testing.T) {
	content := "```text\nwraith-run {\"command\":\"printf x\",\"reason\":\"example\"}\n```"
	request, err := RequestedCommand(content)
	if err != nil || request != nil {
		t.Fatal("example became a request")
	}
	if strings.Contains(DisplayContent("```wraith-run\n{\"command\":\"unfinished"), "wraith-run") {
		t.Fatal("streaming wrapper leaked")
	}
}
