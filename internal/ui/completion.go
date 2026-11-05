package ui

import "strings"

type commandSpec struct {
	Name, Description string
	NeedsArgument     bool
}

var commandSpecs = []commandSpec{
	{"clear", "start a clean chat and retain saved history", false},
	{"help", "show commands and key bindings", false},
	{"models", "choose an installed ollama model", false},
	{"model", "choose an installed ollama model", false},
	{"settings", "configure the workspace", false},
	{"theme", "choose a color theme", false},
	{"think", "set native model thinking", false},
	{"thinking", "toggle reasoning display", false},
	{"context", "inspect context usage", false},
	{"compact", "archive and compact older turns", false},
	{"cancel", "cancel model work", false},
	{"instructions", "edit global user instructions", false},
	{"memory", "edit global user instructions", false},
	{"remember", "append a global instruction", true},
	{"pin", "preserve an exact fact in this chat", true},
	{"pins", "list pinned facts", false},
	{"unpin", "remove a numbered pin", true},
	{"blocks", "edit and stage a suggested code block", false},
	{"edit", "edit the draft externally", false},
	{"edit-response", "edit the selected assistant response", false},
	{"new", "start a new chat", false},
	{"chats", "choose a saved chat", false},
	{"sessions", "choose a saved chat", false},
	{"resume", "resume a session id", true},
	{"archives", "browse original context snapshots", false},
	{"restore", "restore an archive filename", true},
	{"export", "export the active context", false},
	{"retry", "retry an interrupted request", false},
	{"set", "set a configuration key and value", true},
	{"terminal-restart", "restart shell 1 or 2", true},
	{"quit", "save and exit", false},
}

func matchingCommands(query string) []commandSpec {
	query = strings.ToLower(query)
	var exact, matches []commandSpec
	for _, cmd := range commandSpecs {
		if cmd.Name == query {
			exact = append(exact, cmd)
		} else if strings.HasPrefix(cmd.Name, query) {
			matches = append(matches, cmd)
		}
	}
	return append(exact, matches...)
}

func (a *App) slashDraft() (string, bool) {
	text := a.Prompt.String()
	active := a.focus == "prompt" && a.Prompt.Mode == "insert" && !a.commandMode && !a.searchMode && strings.HasPrefix(text, "/") && !strings.ContainsRune(text, '\n')
	return strings.TrimPrefix(text, "/"), active
}

func (a *App) completions() ([]commandSpec, int, bool) {
	query, active := a.slashDraft()
	if !active || strings.ContainsAny(query, " \t") {
		return nil, 0, false
	}
	matches := matchingCommands(query)
	index := 0
	if query == a.completionQuery {
		index = min(a.completionIndex, max(0, len(matches)-1))
	}
	return matches, index, true
}
