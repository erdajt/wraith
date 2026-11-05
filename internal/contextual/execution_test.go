package contextual

import (
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/session"
	"strings"
	"testing"
)

func TestOnlyExplicitWellFormedRequestsAreActionable(t *testing.T) {
	valid := "```wraith-run\n{\"command\":\"printf hello\",\"reason\":\"check output\"}\n```"
	request, err := RequestedCommand(valid)
	if err != nil || request.Command != "printf hello" {
		t.Fatalf("valid request: %v", err)
	}
	for _, text := range []string{valid + "\n" + valid, "```wraith-run\n{}\n```", "```wraith-run\n{\"command\":\"x\",\"reason\":\"y\",\"extra\":1}\n```"} {
		if _, err := RequestedCommand(text); err == nil {
			t.Fatalf("accepted malformed request %q", text)
		}
	}
	for _, text := range []string{"```sh\nprintf hello\n```", "explain commands", "```wraith-run\n{unfinished"} {
		if request, err := RequestedCommand(text); err != nil || request != nil {
			t.Fatal("ordinary text became a request")
		}
	}
}

func TestExecutionPolicyIsIsolatedFromCompaction(t *testing.T) {
	c := config.Default()
	s := session.New("test")
	if !strings.Contains(ForChat(s, c, "")[0].Content, AgentPrompt) {
		t.Fatal("agent policy missing")
	}
	c.Execution.Mode = "blocks"
	if !strings.Contains(ForChat(s, c, "")[0].Content, BlocksPrompt) {
		t.Fatal("blocks policy missing")
	}
	if strings.Contains(CompactPrompt, "wraith-run") || strings.Contains(AuditPrompt, "wraith-run") {
		t.Fatal("execution leaked into compaction")
	}
	if ChatUsed(s, c, "") <= Used(s, "") {
		t.Fatal("execution policy not budgeted")
	}
}
