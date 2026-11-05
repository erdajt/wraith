package ui

func (a *App) helpItems() []item {
	items := make([]item, 0, len(commandSpecs)+16)
	for _, command := range commandSpecs {
		items = append(items, item{Label: "/" + command.Name, Detail: command.Description})
	}
	entries := [][2]string{
		{a.Config.Keys["send"] + " · :w", "send the prompt"},
		{a.Config.Keys["editor"], "edit the focused prompt or message; user-message edits fork"},
		{a.Config.Keys["focus"], "switch prompt and conversation focus"},
		{a.Config.Leader, "open command binding hints"},
		{"/settings.appearance", "open categories directly; ? explains settings"},
		{"insert: up/down", "recall submitted input and restore the draft"},
		{"normal: / n N", "search the focused buffer and highlight matches"},
		{"h j k l · w b e · 0 ^ $ · { }", "cursor motions; numeric prefixes repeat"},
		{"v V · y Y · ye · yf,", "visual selection and clipboard yanks"},
		{"chat: [ ] · enter", "select a message or open its command"},
		{a.Config.Keys["thinking_toggle"], "expand or collapse selected thinking"},
		{a.Config.Keys["terminal_overlay"] + " · " + a.Config.Keys["terminal_split"], "scratch shell and side shell"},
		{"terminal: esc esc esc", "vim copy mode; i returns to live input"},
		{a.Config.Keys["output_share"], "share output explicitly when automatic sharing is off"},
		{"normal: q · " + a.Config.Keys["quit"], "save and exit; closes child shells"},
	}
	for _, entry := range entries {
		items = append(items, item{Label: entry[0], Detail: entry[1]})
	}
	return items
}
