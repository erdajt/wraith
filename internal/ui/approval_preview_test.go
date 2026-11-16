package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/variables"
)

func TestApprovalPreviewUsesFrozenShellArguments(t *testing.T) {
	a := fixture(t)
	a.width, a.height = 110, 40
	a.environmentCommand("set TARGET=\"old's.example\"", false)
	a.approve(proposal{Command: "printf wraith::TARGET", Reason: "inspect wraith::TARGET", Continue: true})
	if a.proposal == nil {
		t.Fatal(a.notice)
	}
	resolved := a.proposal.Resolved
	a.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "new.example"}}, nil)
	frame := strings.Join(a.menuFrame(), "\n")
	text := ansi.Strip(frame)
	if !strings.Contains(text, resolved) || !strings.Contains(text, "reason: inspect old's.example") || strings.Contains(text, "new.example") {
		t.Fatalf("preview differs from launch %q:\n%s", resolved, text)
	}
	if !strings.Contains(frame, "\x1b["+color(a.Config.Palette().Variable, false)+"mold's.example") {
		t.Fatal("resolved shell argument is not highlighted")
	}
	a.approvalAction("view")
	var full strings.Builder
	for _, item := range a.popup.Items {
		full.WriteString(item.Label)
		full.WriteByte('\n')
	}
	if !strings.Contains(full.String(), resolved) || !strings.Contains(full.String(), "reason: inspect old's.example") {
		t.Fatalf("full preview differs from frozen approval: %s", full.String())
	}
}
