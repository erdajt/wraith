package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/terminal"
)

func TestSavedZshBlockIsInsertedWithoutAnExtraEnter(t *testing.T) {
	shell, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	t.Setenv("ZDOTDIR", t.TempDir())
	a := fixture(t)
	a.Config.Shell = []string{shell}
	marker := filepath.Join(t.TempDir(), "executed")
	command := "printf saved > " + editor.Quote(marker)
	file, err := editor.Prepare(a.Paths.Cache, ".sh", "printf original")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file.Path, []byte(command), 0600); err != nil {
		t.Fatal(err)
	}
	a.finishEdit(editedMsg{Kind: "block", Language: "sh", File: file})
	p := a.panes[0]
	if p == nil || p.PasteID == "" || p.Pending != command {
		t.Fatalf("save did not queue insertion: %s", a.notice)
	}
	deadline := time.After(3 * time.Second)
	for p.PasteID != "" {
		select {
		case event, ok := <-p.PTY.Pastes:
			a.Update(pasteMsg{Index: 0, PTY: p.PTY, Event: event, Open: ok})
		case event, ok := <-p.PTY.Events:
			a.Update(terminalMsg{Index: 0, PTY: p.PTY, Event: event, Open: ok})
		case <-deadline:
			t.Fatalf("saved command was not inserted: %s", a.notice)
		}
	}
	if p.Pending != "" || !strings.Contains(a.notice, "command inserted") {
		t.Fatalf("automatic insertion failed: %s", a.notice)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("saving the block executed it")
	}
	a.terminalKey(tea.KeyMsg{Type: tea.KeyEnter})
	if _, err := p.PTY.QueuePaste("printf next"); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case event := <-p.PTY.Pastes:
			if event.Status == "inserted" {
				data, err := os.ReadFile(marker)
				if err != nil || string(data) != "saved" {
					t.Fatalf("one enter did not execute: %q, %v", data, err)
				}
				return
			}
		case <-p.PTY.Events:
		case <-deadline:
			t.Fatal("shell did not return to its prompt")
		}
	}
}

func TestUncertainInsertionCannotBeRepeatedOrExecuted(t *testing.T) {
	a := fixture(t)
	a.openTerminal(0)
	p := a.panes[0]
	p.Pending, p.PasteID = "printf retained", "pending"
	a.Update(pasteTimeoutMsg{Index: 0, PTY: p.PTY, ID: p.PasteID})
	for _, key := range []tea.KeyMsg{{Type: tea.KeyEnter}, {Type: tea.KeyEsc}, {Type: tea.KeyRunes, Runes: []rune("r")}} {
		a.terminalKey(key)
		if p.Pending != "printf retained" || p.PasteID != "pending" || a.proposal != nil || len(a.jobs.Snapshot()) != 0 {
			t.Fatal("uncertain insertion was discarded, repeated or executed")
		}
	}
	a.pasted(pasteMsg{Index: 0, PTY: p.PTY, Open: false})
	if p.PasteID == "" {
		t.Fatal("lost acknowledgement was treated as cancellation")
	}
	a.pasted(pasteMsg{Index: 0, PTY: p.PTY, Open: true, Event: terminal.PasteEvent{ID: "pending", Status: "inserted"}})
	if p.Pending != "" || p.PasteID != "" || !strings.Contains(a.notice, "before cancellation") {
		t.Fatal("late insertion acknowledgement lost")
	}
}

func TestStalePasteEventsAndRestartPreserveSavedCommands(t *testing.T) {
	a := fixture(t)
	a.openTerminal(0)
	p := a.panes[0]
	p.Pending, p.Path, p.Language, p.PasteID = "printf retained", "saved.sh", "sh", "old"
	a.restartExitedTerminal(restartTerminalMsg{Index: 0, PTY: p.PTY})
	replacement := a.panes[0]
	a.pasted(pasteMsg{Index: 0, PTY: p.PTY, Open: true, Event: terminal.PasteEvent{ID: "old", Status: "inserted"}})
	a.Update(pasteTimeoutMsg{Index: 0, PTY: p.PTY, ID: "old"})
	if replacement.Pending != "printf retained" || replacement.Path != "saved.sh" || replacement.PasteID != "" {
		t.Fatal("stale event changed the replacement shell")
	}
}

func TestManualRestartRetainsSnippetWhenStartupFails(t *testing.T) {
	a := fixture(t)
	a.openTerminal(0)
	p := a.panes[0]
	p.Pending, p.Path, p.Language, p.PasteID = "printf retained", "saved.sh", "sh", "uncertain"
	a.Config.Shell = []string{filepath.Join(t.TempDir(), "missing-shell")}
	a.runCommand("terminal-restart 1")
	if a.panes[0] == nil || a.panes[0].Pending != "printf retained" || a.panes[0].Path != "saved.sh" || a.panes[0].PasteID != "" {
		t.Fatal("failed manual restart discarded the saved snippet")
	}
	a.Config.Shell = []string{"/bin/sh", "-i"}
	a.runCommand("terminal-restart 1")
	if a.panes[0].PTY == p.PTY || a.panes[0].Pending != "printf retained" || a.panes[0].Path != "saved.sh" {
		t.Fatal("manual recovery lost the saved command")
	}
}

func TestSecondSavedSnippetCannotOverwritePendingInsertion(t *testing.T) {
	a := fixture(t)
	a.openTerminal(0)
	p := a.panes[0]
	p.Pending, p.Path, p.Language = "printf first", "first.sh", "sh"
	file, err := editor.Prepare(a.Paths.Cache, ".sh", "printf original")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file.Path, []byte("printf second"), 0600); err != nil {
		t.Fatal(err)
	}
	a.finishEdit(editedMsg{Kind: "block", Language: "sh", File: file})
	if p.Pending != "printf first" || p.Path != "first.sh" || !strings.Contains(a.notice, "new snippet saved at") {
		t.Fatal("second save replaced the first pending command")
	}
	paths, err := filepath.Glob(filepath.Join(a.Paths.Data, "snippets", "*"))
	if err != nil || len(paths) != 1 {
		t.Fatal("second snippet was not retained")
	}
	data, err := os.ReadFile(paths[0])
	if err != nil || string(data) != "printf second" {
		t.Fatal("retained snippet contents changed")
	}
}

func TestOccupiedIntegratedShellRetriesThroughTheEditor(t *testing.T) {
	shell, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	t.Setenv("ZDOTDIR", t.TempDir())
	a := fixture(t)
	a.Config.Shell = []string{shell}
	a.openTerminal(0)
	p := a.panes[0]
	deadline := time.After(3 * time.Second)
	ready := false
	for !ready {
		select {
		case event := <-p.PTY.Pastes:
			ready = event.ID == "ready"
		case <-p.PTY.Events:
		case <-deadline:
			t.Fatal("shell integration did not start")
		}
	}
	p.Pending = "printf retained"
	a.terminalKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if p.Pending != "printf retained" || !strings.Contains(a.notice, "input interrupted") {
		t.Fatal("clearing shell input discarded the snippet")
	}
	a.terminalKey(tea.KeyMsg{Type: tea.KeyEnter})
	if p.PasteID == "" || p.Pending != "printf retained" {
		t.Fatalf("retry bypassed the shell's empty-buffer check: %s", a.notice)
	}
}
