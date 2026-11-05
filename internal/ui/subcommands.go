package ui

import "strings"

var subcommands = map[string][]commandSpec{
	"settings": {
		{"settings.model", "model, thinking, context and startup", false},
		{"settings.execution", "agent or blocks mode and command budgets", false},
		{"settings.context", "compaction policy and limits", false},
		{"settings.appearance", "themes, transparency, thinking and mouse", false},
		{"settings.terminals", "pane geometry, colors and shell", false},
		{"settings.keys", "leader, bindings and editor", false},
	},
}

func commandChoices(query string) []commandSpec {
	if parent, _, ok := strings.Cut(query, "."); ok {
		return subcommands[parent]
	}
	return commandSpecs
}
