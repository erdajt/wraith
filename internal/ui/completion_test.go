package ui

import "testing"

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
