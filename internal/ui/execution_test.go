package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/editor"
)

func TestBlockNeedsApprovalAndSavesOutput(t *testing.T) {
	a := fixture(t)
	marker := filepath.Join(t.TempDir(), "approved")
	command := "printf evidence; touch " + editor.Quote(marker)
	a.blockActions(editor.Block{Language: "sh", Text: command})
	a.blockAction("run")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("block executed before approval")
	}
	if !strings.Contains(a.View(), "printf evidence") {
		t.Fatal("approval did not show actual command")
	}
	a.approvalAction("run")
	drain(t, a)
	records, err := a.history().List()
	if err != nil || len(records) != 1 {
		t.Fatalf("history: %v", err)
	}
	record, err := a.history().Load(records[0].ID)
	if err != nil || record.Output != "evidence" || record.Command != command || record.ExitCode != 0 {
		t.Fatalf("record: %+v %v", record, err)
	}
	a.openHistory()
	a.menuKey(tea.KeyMsg{Type: tea.KeyEnter})
	if a.popup.Kind != "run-output" || !strings.Contains(a.View(), "evidence") {
		t.Fatal("saved output not visible")
	}
}

func TestPreviewReturnsToApprovalAndEscapeDoesNotExecute(t *testing.T) {
	a := fixture(t)
	a.approve(proposal{Command: "printf hello", Text: "printf hello", Language: "sh"})
	a.approvalAction("view")
	a.menuKey(tea.KeyMsg{Type: tea.KeyEsc})
	if a.popup.Kind != "approval" {
		t.Fatal("preview lost approval")
	}
	a.menuKey(tea.KeyMsg{Type: tea.KeyEsc})
	if a.proposal != nil || a.busy {
		t.Fatal("escape did not discard approval")
	}
	records, err := a.history().List()
	if err != nil || len(records) != 0 {
		t.Fatal("declined command entered execution history")
	}
}
