package ui

import "slices"

// helpItems builds the command reference with bindings from the active configuration.
func (a *App) helpItems() []item {
	items := make([]item, 0, len(commandSpecs)+16)
	for _, command := range commandSpecs {
		items = append(items, item{Label: "/" + command.Name, Value: command.Name, Detail: command.Description})
	}

	for _, command := range a.pluginManifest.Commands {
		entry := item{Label: "/" + command.Name, Value: command.Name, Detail: command.Description}
		if index := slices.IndexFunc(items, func(entry item) bool { return entry.Value == command.Name }); index >= 0 {
			items[index] = entry
		} else {
			items = append(items, entry)
		}
	}

	entries := [][3]string{
		{a.Config.Keys["send"] + " · :w", "send the prompt", "send"},
		{a.Config.Keys["editor"], "edit the focused prompt or message; user-message edits fork", "edit"},
		{a.Config.Keys["focus"], "switch prompt and conversation focus", "focus"},
		{a.Config.Leader, "open command binding hints", "commands"},
		{"/settings.appearance", "open categories directly; ? explains settings", "settings"},
		{"insert: up/down", "recall submitted input and restore the draft", "recall"},
		{"normal: / n N", "search the focused buffer and highlight matches", "search"},
		{"h j k l · w b e · 0 ^ $ · { }", "cursor motions; numeric prefixes repeat", "move"},
		{"v V · y Y · ye · yf,", "visual selection and clipboard yanks", "yank"},
		{"chat: [ ] · enter", "select a message or open its command", "select"},
		{a.Config.Keys["thinking_toggle"], "expand or collapse selected thinking", "toggle"},
		{a.Config.Keys["terminal_overlay"] + " · " + a.Config.Keys["terminal_split"], "scratch shell and side shell", "shell"},
		{"terminal: esc esc esc", "vim copy mode; i returns to live input", "copy"},
		{a.Config.Keys["output_share"], "share output explicitly when automatic sharing is off", "share"},
		{"normal: q · " + a.Config.Keys["quit"], "close a window; quit key exits all; unsent drafts require a decision", "quit"},
	}
	for _, entry := range entries {
		items = append(items, item{Label: entry[0], Value: entry[2], Detail: entry[1]})
	}
	return items
}
