package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/editor"
)

func TestStagedEditReplacesOnlyItsOriginalPendingCommand(t *testing.T) {
	for _, name := range []string{"saved", "cancelled", "changed", "inserted", "unacknowledged", "restarted"} {
		t.Run(name, func(t *testing.T) {
			a := fixture(t)
			a.openTerminal(1)
			p := a.panes[1]
			path, command, err := editor.Stage(filepath.Join(a.Paths.Data, "snippets"), "sh", "printf original")
			if err != nil {
				t.Fatal(err)
			}
			p.Path, p.Pending, p.Language = path, command, "sh"
			origin := &stagedEdit{PTY: p.PTY, Path: path, Pending: command}
			f, err := editor.Prepare(a.Paths.Cache, ".sh", command)
			if err != nil {
				t.Fatal(err)
			}
			if name != "cancelled" {
				if err := os.WriteFile(f.Path, []byte("printf edited"), 0600); err != nil {
					t.Fatal(err)
				}
			}

			switch name {
			case "changed":
				p.Pending = "printf newer"
			case "inserted":
				p.Pending = ""
			case "unacknowledged":
				p.PasteID = "in-flight"
			case "restarted":
				a.restartExitedTerminal(restartTerminalMsg{Index: 1, PTY: p.PTY})
				p = a.panes[1]
			}
			pending := p.Pending
			a.finishEdit(editedMsg{Kind: "staged", Language: "sh", File: f, Index: 1, Staged: origin})
			if name == "saved" {
				if p.Pending != "printf edited" || a.panes[0] != nil {
					t.Fatalf("edit staged in wrong place: %q %s", p.Pending, a.notice)
				}
				data, err := os.ReadFile(p.Path)
				if err != nil || string(data) != "printf edited" {
					t.Fatalf("edited source not staged: %q %v", data, err)
				}
			} else if p.Pending != pending || p.Path != path {
				t.Fatalf("stale or cancelled edit changed pending payload: %q %s", p.Pending, a.notice)
			}
			if name != "saved" && name != "cancelled" {
				if _, err := os.Stat(f.Path); err != nil {
					t.Fatalf("rejected edit not retained: %v", err)
				}
			}
		})
	}
}

func TestStagedEditWaitsForPasteAcknowledgement(t *testing.T) {
	a := fixture(t)
	a.openTerminal(0)
	p := a.panes[0]
	path, _, err := editor.Stage(a.Paths.Cache, "sh", "printf original")
	if err != nil {
		t.Fatal(err)
	}
	p.Path, p.Pending, p.Language, p.PasteID = path, "printf original", "sh", "in-flight"
	if cmd := a.editStaged(0); cmd != nil || a.editing || !strings.Contains(a.notice, "acknowledgement") {
		t.Fatalf("unacknowledged insertion entered editor: %s", a.notice)
	}
	if p.PasteID != "in-flight" {
		t.Fatal("edit cleared pending acknowledgement")
	}
}
