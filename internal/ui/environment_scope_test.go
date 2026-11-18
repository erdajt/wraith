package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

func TestConversationVariablesFollowResumeAndClear(t *testing.T) {
	a := fixture(t)
	a.set("environment.scope", "conversation")
	original := a.Session.ID
	a.environmentCommand("set TARGET=private.example // this chat", false)

	if _, err := a.Store.Load(original); err != nil {
		t.Fatalf("variable-only conversation not saved: %v", err)
	}

	a.sessionCommand("clear", "")

	if _, err := a.Variables.Validate("wraith::TARGET", true); err == nil {
		t.Fatal("clear inherited conversation variable")
	}

	a.resume(original)

	if text, _ := a.Variables.Expand("wraith::TARGET"); text != "private.example" {
		t.Fatal("resume lost conversation binding")
	}

	if !strings.Contains(a.chatInstructions(), "this chat") || strings.Contains(a.chatInstructions(), "private.example") {
		t.Fatal("annotation privacy changed")
	}

	a.Session.Messages = []session.Message{{Role: "user", Content: "check target"}}
	a.forkIndex = 0
	a.forkPrompt()

	if _, err := a.Variables.Validate("wraith::TARGET", true); err == nil {
		t.Fatal("fork retained old conversation snapshot")
	}
}

func TestDefaultScopeDoesNotMigrateBindingsAndGlobalStillWarns(t *testing.T) {
	a := fixture(t)
	a.environmentCommand("set TARGET=directory.example", false)
	a.set("environment.scope", "global")
	a.environmentCommand("set GLOBAL=global.example", false)

	if a.popup.Kind != "env-global" {
		t.Fatal("global default bypassed warning")
	}

	a.popup.Index = 1
	a.menuKey(tea.KeyMsg{Type: tea.KeyEnter})
	state, err := a.variableStore().Load(a.Directory, variables.Global)
	if err != nil || len(state.Entries) != 1 || state.Entries[0].Name != "GLOBAL" {
		t.Fatalf("default change migrated values: %+v, %v", state.Entries, err)
	}

	a.environmentCommand("", false)
	a.environmentKey(key(a.Config.Keys["env_scope"]))

	if a.popup.Kind != "env-scope" || len(a.popup.Items) != 3 {
		t.Fatal("scope picker missing")
	}

	a.popup.Index = 0
	a.menuKey(tea.KeyMsg{Type: tea.KeyEnter})

	if a.envScope != variables.Conversation || a.Config.Environment.Scope != "global" {
		t.Fatal("managing scope changed saved default")
	}
}

func TestConversationSaveRequiresDurableSession(t *testing.T) {
	a := fixture(t)
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, nil, 0600); err != nil {
		t.Fatal(err)
	}

	a.Store.Root = blocked
	a.environmentCommand("conversation set TARGET=private.example", false)

	if _, err := os.Stat(a.variableStore().Path(a.Directory, variables.Conversation)); !os.IsNotExist(err) {
		t.Fatal("orphan conversation variables created")
	}
}

func TestVariableEditorKeepsOriginalConversationAndScope(t *testing.T) {
	a := fixture(t)
	a.environmentCommand("conversation edit", false)
	original := a.envEdit.Store
	a.Session = session.New(a.Config.Model)
	a.Config.Environment.Scope = "global"
	f, err := editor.Prepare(a.Paths.Cache, ".env", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.Path, []byte("TARGET=edited.example"), 0600); err != nil {
		t.Fatal(err)
	}

	a.finishEdit(editedMsg{Kind: "environment", File: f})
	state, err := original.Load(a.Directory, variables.Conversation)
	if err != nil || len(state.Entries) != 1 {
		t.Fatalf("original edit target lost: %+v, %v", state.Entries, err)
	}

	if _, err := a.Variables.Validate("wraith::TARGET", true); err == nil {
		t.Fatal("edited value leaked into different conversation")
	}
}
