package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/variables"
)

func TestPrivateAssignmentsStayOutOfChatAndHistory(t *testing.T) {
	a := fixture(t)
	a.Prompt.Set("TARGET=private.example // target domain")

	if cmd := a.submit(); cmd != nil {
		t.Fatal("assignment started a model request")
	}

	if len(a.Session.Messages) != 0 || len(a.inputHistory) != 0 {
		t.Fatal("assignment added to conversation or history")
	}

	a.runCommand("env")

	if a.popup == nil || len(a.popup.Items) != 1 || a.popup.Items[0].Detail != "target domain" {
		t.Fatalf("missing annotation: %+v", a.popup)
	}

	if !strings.Contains(a.chatInstructions(), "target domain") || strings.Contains(a.chatInstructions(), "private.example") {
		t.Fatal("inventory exposed value or lost description")
	}

	a.runCommand("env set TARGET=other.example")

	if a.Variables.Redact("private.example other.example") != "wraith::TARGET wraith::TARGET" {
		t.Fatal("rotation lost redaction")
	}

	a.runCommand("env unset TARGET")
	a.runCommand("env")

	if len(a.popup.Items) != 0 {
		t.Fatal("removed binding still listed")
	}
}

func TestGlobalMutationRequiresVisibleWarning(t *testing.T) {
	a := fixture(t)
	a.runCommand("env global set TARGET=private.example")
	state, err := a.variableStore().Load(a.Directory, variables.Global)
	if err != nil || len(state.Entries) != 0 || a.popup.Kind != "env-global" {
		t.Fatalf("global assignment applied before warning: %v", err)
	}

	a.popup.Index = 1
	a.menuKey(tea.KeyMsg{Type: tea.KeyEnter})
	state, err = a.variableStore().Load(a.Directory, variables.Global)
	if err != nil || len(state.Entries) != 1 {
		t.Fatalf("confirmed global assignment missing: %v", err)
	}

	if len(a.inputHistory) != 0 {
		t.Fatal("private command persisted in prompt history")
	}
}

func TestMalformedAssignmentNeverBecomesPrompt(t *testing.T) {
	a := fixture(t)
	a.Prompt.Set("bad-name=private.example")

	if cmd := a.submit(); cmd != nil || len(a.Session.Messages) != 0 {
		t.Fatal("invalid assignment dispatched")
	}

	if strings.Contains(a.notice, "private.example") {
		t.Fatal("diagnostic echoed private value")
	}
}

func TestRemoveRefreshesPickerAndRevealsGlobalBinding(t *testing.T) {
	a := fixture(t)
	a.environmentCommand("global set TARGET=global.example", true)
	a.environmentCommand("set TARGET=local.example", false)
	a.environmentCommand("", false)
	a.environmentKey(key("d"))

	if a.popup == nil || len(a.popup.Items) != 0 || !strings.Contains(a.notice, "another scoped binding remains active") {
		t.Fatalf("stale variable picker: %s", a.notice)
	}
}

func TestDemoAssignmentsDoNotMutateBindings(t *testing.T) {
	a := fixture(t)
	a.demo = true
	a.Prompt.Set("TARGET=private.example")
	a.submit()
	state, err := a.variableStore().Load(a.Directory, variables.Directory)
	if err != nil || len(state.Entries) != 0 {
		t.Fatalf("demo changed bindings: %v", err)
	}
}

func TestVariablePickerUsesConfiguredBindings(t *testing.T) {
	a := fixture(t)
	a.Config.Keys["env_new"] = "a"
	a.Config.Keys["env_edit"] = "o"
	a.Config.Keys["env_scope"] = "s"
	a.Config.Keys["env_remove"] = "x"
	a.environmentCommand("set TARGET=local.example", false)
	a.environmentCommand("", false)

	if !strings.Contains(a.popup.Hints, "a/o editor · s scope · x remove") {
		t.Fatalf("stale binding hints: %s", a.popup.Hints)
	}

	if _, handled := a.environmentKey(key("x")); !handled || len(a.popup.Items) != 0 {
		t.Fatal("configured remove binding did not remove variable")
	}
}

func TestRemovingDirectoryBindingKeepsConversationOverride(t *testing.T) {
	a := fixture(t)
	a.environmentCommand("global set TARGET=global.example", true)
	a.environmentCommand("directory set TARGET=directory.example", false)
	a.environmentCommand("conversation set TARGET=conversation.example", false)
	a.environmentCommand("directory unset TARGET", false)

	if got, _ := a.Variables.Expand("wraith::TARGET"); got != "conversation.example" {
		t.Fatal("conversation override lost")
	}

	if strings.Contains(a.notice, "global binding") || !strings.Contains(a.notice, "another scoped binding remains active") {
		t.Fatalf("incorrect remaining scope: %s", a.notice)
	}
}
