package ui

import (
	"net/http"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCommandMatchesPreferExactNamesAndKeepAliases(t *testing.T) {
	for _, query := range []string{"model", "models", "think", "thinking", "edit", "edit-response"} {
		matches := matchingCommands(query)
		if len(matches) == 0 || matches[0].Name != query {
			t.Fatalf("exact command %q is not first: %v", query, matches)
		}
	}
	matches := matchingCommands("MO")
	if len(matches) != 2 {
		t.Fatalf("model aliases missing: %v", matches)
	}
	if len(matchingCommands("missing")) != 0 {
		t.Fatal("unknown command matched")
	}
	if len(matchingCommands("")) != len(commandSpecs) {
		t.Fatal("slash did not list all commands")
	}
}

func TestCompletionDoesNotInterceptTextSearchOrArguments(t *testing.T) {
	a := fixture(t)
	for _, text := range []string{"ordinary prompt", "/remember keep this", "/path\nexplain this path"} {
		a.Prompt.Set(text)
		if _, _, active := a.completions(); active {
			t.Fatalf("unexpected completion for %q", text)
		}
	}
	a.Prompt.Set("/mod")
	if choices, _, active := a.completions(); !active || len(choices) != 2 {
		t.Fatal("partial command did not complete")
	}
	a.focus = "chat"
	if _, _, active := a.completions(); active {
		t.Fatal("transcript search opened command completion")
	}
	a.focus = "prompt"
	a.Prompt.Mode = "normal"
	if _, _, active := a.completions(); active {
		t.Fatal("normal-mode motions intercepted")
	}
}

func TestSlashPickerRunsWithoutSendingChat(t *testing.T) {
	a := fixture(t)
	a.key(key("/"))
	a.key(key("thi"))
	if !strings.Contains(a.View(), "/thinking") {
		t.Fatal("matching command list was not rendered")
	}
	a.key(tea.KeyMsg{Type: tea.KeyDown})
	a.key(tea.KeyMsg{Type: tea.KeyUp})
	a.key(tea.KeyMsg{Type: tea.KeyDown})
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.showThinking || a.Prompt.String() != "" || len(a.Session.Messages) != 0 {
		t.Fatal("enter did not run the selected command locally")
	}
}

func TestCommandArgumentSelectionRetainsEditableInput(t *testing.T) {
	a := fixture(t)
	a.key(key("/rem"))
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if a.Prompt.String() != "/remember " || a.Instructions != "" {
		t.Fatal("required argument was not offered for editing")
	}
	a.key(key("keep exact hostnames"))
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(a.Instructions, "keep exact hostnames") || a.Prompt.String() != "" {
		t.Fatal("argument command did not run on enter")
	}
	a.key(key("/unknown"))
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if a.Prompt.String() != "/unknown" {
		t.Fatal("unknown command lost its draft")
	}
	a.key(tea.KeyMsg{Type: tea.KeyEsc})
	if _, _, active := a.completions(); active {
		t.Fatal("escape did not dismiss suggestions")
	}
}

func TestWriteCommandSendsPrompt(t *testing.T) {
	a := fixture(t)
	a.Client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		return response(`{"done":true,"message":{"content":"received"}}`), nil
	})
	a.key(key("prompt sent through vim"))
	a.key(tea.KeyMsg{Type: tea.KeyEsc})
	a.key(key(":"))
	a.key(key("w"))
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.busy {
		t.Fatal(":w did not submit")
	}
	drain(t, a)
	if len(a.Session.Messages) != 2 || a.Session.Messages[0].Content != "prompt sent through vim" {
		t.Fatal(":w lost the prompt")
	}
}
