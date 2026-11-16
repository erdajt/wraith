package ui

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/session"
)

func TestBlockNeedsApprovalAndSavesOutput(t *testing.T) {
	a := fixture(t)
	marker := filepath.Join(t.TempDir(), "approved")
	command := "printf evidence; touch " + editor.Quote(marker)
	a.blockActions(editor.Block{Language: "sh", Text: command})
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("block executed before approval")
	}
	a.blockAction("run")
	if !a.executing {
		t.Fatal("run asked for a second approval")
	}
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

func TestApprovalFreezesPrivateArgumentsAndEnvironment(t *testing.T) {
	a := fixture(t)
	a.environmentCommand("set TARGET=private.example", false)
	a.approve(proposal{Command: `printf '%s|%s' wraith::TARGET "$TARGET"`, Language: "sh"})
	if a.proposal == nil || !strings.Contains(a.View(), "private.example") {
		t.Fatal("resolved preview missing")
	}
	a.environmentCommand("set TARGET=changed.example", false)
	a.runApproved()
	drain(t, a)
	if a.activeRun.Output != "private.example|private.example" {
		t.Fatalf("approval changed before execution: %q", a.activeRun.Output)
	}
}

func TestSavedApprovalEditRunsWithoutSecondPrompt(t *testing.T) {
	a := fixture(t)
	a.approve(proposal{Command: "printf original", Language: "sh", Source: "model"})
	f, err := editor.Prepare(a.Paths.Cache, ".sh", a.proposal.Resolved)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.Path, []byte("printf edited"), 0600); err != nil {
		t.Fatal(err)
	}
	a.finishEdit(editedMsg{Kind: "run", Language: "sh", File: f})
	if !a.executing {
		t.Fatalf("saved approval did not execute: %s", a.notice)
	}
	drain(t, a)
	if a.activeRun.Output != "edited" || a.activeRun.Source != "model-edited" {
		t.Fatalf("edit lost: %+v", a.activeRun)
	}
}

func TestInvalidPrivateCommandNeverReachesApproval(t *testing.T) {
	a := fixture(t)
	a.approve(proposal{Command: "printf wraith::MISSING", Language: "sh"})
	if a.proposal != nil || a.executing {
		t.Fatal("invalid reference executable")
	}
}

func TestApprovalRetainsDirectoryShellAndSharingPolicy(t *testing.T) {
	a := fixture(t)
	directory := t.TempDir()
	a.Config.Execution.Mode = "blocks"
	a.Config.Terminal.Directory = directory
	a.approve(proposal{Command: "pwd"})
	a.Config.Terminal.Directory = "/missing-directory"
	a.Config.Shell = []string{"/missing-shell"}
	a.Config.Execution.Mode = "agent"
	a.runApproved()
	drain(t, a)
	if strings.TrimSpace(a.activeRun.Output) != directory || a.activeRun.Agent || a.activeRun.ShareOutput {
		t.Fatalf("approval changed: %+v", a.activeRun)
	}
}

func TestApprovalCannotRunAfterSessionChanges(t *testing.T) {
	a := fixture(t)
	a.approve(proposal{Command: "printf stale", Continue: true})
	a.Session.ID = "another-session"
	a.runApproved()
	if a.executing || a.proposal != nil || len(a.jobs.Snapshot()) != 0 {
		t.Fatal("stale approval executed")
	}
}

func TestBlockEditorReceivesResolvedSourceWithoutExecuting(t *testing.T) {
	for _, language := range []string{"sh", "python"} {
		t.Run(language, func(t *testing.T) {
			a := fixture(t)
			a.environmentCommand("set TARGET=private.example", false)
			cmd := a.edit("block", language, "printf wraith::TARGET", -1)
			if cmd == nil || !a.editing {
				t.Fatalf("editor did not open: %s", a.notice)
			}
			paths, err := filepath.Glob(filepath.Join(a.Paths.Cache, "edits", "*"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("editor file missing: %v", err)
			}
			data, err := os.ReadFile(paths[0])
			if err != nil || strings.Contains(string(data), "wraith::") || !strings.Contains(string(data), "private.example") {
				t.Fatalf("unresolved editor text: %q %v", data, err)
			}
			if len(a.jobs.Snapshot()) != 0 {
				t.Fatal("opening editor executed code")
			}
		})
	}
}

func TestManualRunContinuesExistingConversation(t *testing.T) {
	a := fixture(t)
	a.generation = session.ID()
	calls := 0
	a.Client.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
		calls++
		return response(`{"done":true,"message":{"role":"assistant","content":"observed"}}`), nil
	})
	a.approve(proposal{Command: "printf result", Source: "block"})
	a.runApproved()
	drain(t, a)
	if calls != 1 {
		t.Fatalf("continuations: %d", calls)
	}
}

func TestManualApprovalCannotCrossTurns(t *testing.T) {
	a := fixture(t)
	a.approve(proposal{Command: "printf stale", Source: "block"})
	a.generation = session.ID()
	a.runApproved()
	if a.executing || len(a.jobs.Snapshot()) != 0 {
		t.Fatal("stale manual approval executed")
	}
}

func TestPreparedJobsSeparateCancellationOwnershipFromContinuation(t *testing.T) {
	for _, requested := range []bool{false, true} {
		a := fixture(t)
		p := proposal{Command: "printf result", Continue: requested, Background: true}
		if err := a.prepareProposal(&p); err != nil {
			t.Fatal(err)
		}
		if p.launch.AgentRequested != requested {
			t.Fatal("manual launch inherited agent cancellation ownership")
		}
		if p.launch.Generation == "" || p.launch.Generation != a.generation {
			t.Fatal("manual job lost continuation generation")
		}
	}
}
