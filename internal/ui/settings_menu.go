package ui

import (
	"encoding/json"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) settingsMenu() tea.Cmd {
	a.popup = &menu{Kind: "settings", Title: "settings · e edits config.yaml", Items: []item{
		{"model", "category.model", "model, thinking, context, startup"},
		{"execution", "category.execution", "agent or blocks, step budget, output"},
		{"context", "category.context", "compaction policy and threshold"},
		{"appearance", "category.appearance", "themes, transparency, thinking, mouse"},
		{"terminals", "category.terminals", "pane dimensions, colors, shell"},
		{"keys and editor", "category.keys", "leader, command bindings, editor"},
		{"all settings", "edit", "config.yaml"},
	}}
	return nil
}

func (a *App) settingsCategory(category string) {
	groups := map[string][]string{
		"model":      {"model", "thinking", "context", "predict", "threads", "auto_start"},
		"execution":  {"execution.mode", "execution.max_steps", "execution.output_context"},
		"context":    {"compact.mode", "compact.threshold", "compact.keep_turns", "compact.budget"},
		"appearance": {"theme", "appearance.background", "appearance.thinking", "appearance.mouse", "animations", "themes"},
		"terminals":  {"terminal.overlay_width", "terminal.overlay_height", "terminal.split_width", "terminal.background", "terminal.foreground", "terminal.scrollback", "terminal.directory", "shell"},
		"keys":       {"leader", "bindings", "keys", "editor", "clipboard"},
	}
	encoded, _ := json.Marshal(a.Config)
	var data map[string]any
	_ = json.Unmarshal(encoded, &data)
	items := []item{}
	for _, key := range groups[category] {
		var value any = data
		for _, part := range strings.Split(key, ".") {
			value = value.(map[string]any)[part]
		}
		b, _ := json.Marshal(value)
		items = append(items, item{Label: key, Value: key, Detail: string(b)})
	}
	items = append(items, item{"edit config.yaml", "edit", "all values, custom palettes and key bindings"})
	a.popup = &menu{Kind: "settings", Title: "settings · " + category, Items: items}
}
