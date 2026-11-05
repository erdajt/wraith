package ui

import (
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/session"
	"os"
	"path/filepath"
	"testing"
)

func TestVisualCopyAndNormalLineYank(t *testing.T) {
	a := fixture(t)
	a.Config.Clipboard = nil
	a.focus = "chat"
	a.follow = false
	a.Session.Messages = []session.Message{{Role: "assistant", Content: "alpha\nbeta"}}
	a.key(key("gg"))
	a.key(key("j"))
	a.key(key("Vj"))
	a.key(key("y"))
	if string(a.Prompt.Register) != "alpha\nbeta" || a.visualMode != "" {
		t.Fatalf("visual copy: %q", a.Prompt.Register)
	}
	a.key(key("Y"))
	if string(a.Prompt.Register) != "beta" {
		t.Fatalf("line copy: %q", a.Prompt.Register)
	}
}

func TestClipboardPassesSelectedTextAsInput(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(t.TempDir(), "clipboard")
	a.Config.Clipboard = []string{"/bin/sh", "-c", "cat > \"$1\"", "copy", path}
	a.Session.Messages = []session.Message{{Role: "assistant", Content: "literal $(not executed)"}}
	cmd := a.yank()
	if cmd == nil {
		t.Fatal("missing clipboard command")
	}
	a.Update(cmd())
	b, err := os.ReadFile(path)
	if err != nil || string(b) != a.Session.Messages[0].Content {
		t.Fatalf("copy: %q %v", b, err)
	}
}

func TestEditingUserMessageArchivesOriginal(t *testing.T) {
	a := fixture(t)
	a.Session.Messages = []session.Message{{Role: "user", Content: "old prompt"}}
	f, err := editor.Prepare(a.Paths.Cache, ".md", "old prompt")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.Path, []byte("revised prompt"), 0600); err != nil {
		t.Fatal(err)
	}
	a.finishEdit(editedMsg{Kind: "response", Index: 0, File: f})
	if a.Session.Messages[0].Role != "user" || a.Session.Messages[0].Content != "revised prompt" {
		t.Fatal("user message edit failed")
	}
	original, err := a.Store.ReadArchive(a.Session.Archives[0])
	if err != nil || original.Messages[0].Content != "old prompt" {
		t.Fatal("original user message lost")
	}
}

func TestVisualCommandSelectionYanksEntireRawBlock(t *testing.T) {
	a := fixture(t)
	a.Config.Clipboard = nil
	a.focus = "chat"
	a.follow = false
	command := "printf '%s\\n' first\nprintf second"
	a.Session.Messages = []session.Message{{Role: "assistant", Content: "before\n```sh\n" + command + "\n```\nafter"}}
	lines := a.transcript(a.mainWidth() - 5)
	row := -1
	for i, line := range lines {
		if line.Block == 0 {
			row = i + 1
			break
		}
	}
	if row < 0 {
		t.Fatal("command metadata missing")
	}
	a.chatCursor = row
	a.chatColumn = 5
	a.key(key("vl"))
	a.key(key("y"))
	if string(a.Prompt.Register) != command {
		t.Fatalf("partial command copied: %q", a.Prompt.Register)
	}
	a.visualMode = "visual"
	a.visualRow = row
	a.visualColumn = 5
	if !visualBlockRow(lines, row-1, row, row) || !visualBlockRow(lines, row+1, row, row) {
		t.Fatal("command frame not selected as a unit")
	}
}

func TestVisualProseKeepsPartialSelection(t *testing.T) {
	a := fixture(t)
	a.Config.Clipboard = nil
	a.focus = "chat"
	a.follow = false
	a.Session.Messages = []session.Message{{Role: "assistant", Content: "alpha beta"}}
	a.chatCursor = 1
	a.chatColumn = 0
	a.key(key("v4ly"))
	if string(a.Prompt.Register) != "alpha" {
		t.Fatalf("prose selection: %q", a.Prompt.Register)
	}
}
