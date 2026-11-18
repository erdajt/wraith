package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/editor"
)

func TestNestedEditorCompletionOwnsItsCaller(t *testing.T) {
	for _, name := range []string{"saved", "cancelled", "failed", "stale", "reload", "unrelated"} {
		t.Run(name, func(t *testing.T) {
			a := pluginFixture(t, `
				wraith.commands.create("inner", function() wraith.action("editor.open") end)
				wraith.commands.create("outer", function() wraith.action("command.inner"); wraith.prompt.set("after") end)
			`)
			a.Prompt.Set("before")
			_, child := a.Update(a.runCommand("outer")())
			_, handoff := a.Update(child())
			if handoff == nil || !a.editing || a.editorFile == nil || len(a.editorDeferred) != 1 {
				t.Fatal("last child effect did not hold its caller for the editor")
			}
			f := a.editorFile
			if name != "cancelled" {
				if err := os.WriteFile(f.Path, []byte("saved"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			result := editedMsg{Kind: "prompt", File: f}
			switch name {
			case "failed":
				result.Err = errors.New("editor failed")
			case "stale":
				a.Prompt.Set("new input")
			case "reload":
				a.plugins = nil
			case "unrelated":
				other, err := editor.Prepare(a.Paths.Cache, ".md", "unrelated")
				if err != nil {
					t.Fatal(err)
				}
				_, next := a.Update(editedMsg{Kind: "prompt", File: other})
				if next != nil || !a.editing || a.editorFile != f || len(a.editorDeferred) != 1 || a.Prompt.String() != "before" {
					t.Fatal("unrelated editor completion consumed the active handoff")
				}
			}

			_, next := a.Update(result)
			for i := 0; next != nil && i < 4; i++ {
				_, next = a.Update(next())
			}
			want := "before"
			if name == "saved" || name == "unrelated" {
				want = "after"
			} else if name == "stale" {
				want = "new input"
			}
			if next != nil || a.editing || a.Prompt.String() != want || len(a.editorDeferred) != 0 {
				t.Fatalf("draft=%q want=%q notice=%q", a.Prompt.String(), want, a.notice)
			}
			if name == "stale" || name == "reload" || name == "failed" {
				if _, err := os.Stat(f.Path); err != nil {
					t.Fatalf("failed edit was not retained: %v", err)
				}
			}
			a.Prompt.Set("later input")
			a.Update(result)
			if a.Prompt.String() != "later input" {
				t.Fatal("duplicate editor completion reapplied an old edit")
			}
		})
	}
}

func TestNestedEditorStartupFailureStopsCaller(t *testing.T) {
	for _, name := range []string{"busy", "prepare"} {
		t.Run(name, func(t *testing.T) {
			a := pluginFixture(t, `
				wraith.commands.create("inner", function() wraith.action("editor.open") end)
				wraith.commands.create("outer", function() wraith.action("command.inner"); wraith.prompt.set("after") end)
			`)
			a.Prompt.Set("before")
			_, child := a.Update(a.runCommand("outer")())
			if name == "busy" {
				a.busy = true
			} else {
				if err := os.MkdirAll(a.Paths.Cache, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(a.Paths.Cache, "edits"), []byte("occupied"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, next := a.Update(child())
			if next != nil || a.editing || a.Prompt.String() != "before" || len(a.editorDeferred) != 0 {
				t.Fatalf("failed startup resumed caller: %s", a.notice)
			}
			if name == "busy" && !strings.Contains(a.notice, "current operation") {
				t.Fatalf("missing startup failure: %s", a.notice)
			}
		})
	}
}
