package ui

import "strings"

func hintColor(action string, p palette) string {
	words := strings.Fields(strings.ToLower(action))
	if len(words) == 0 {
		return p.Accent
	}
	switch strings.TrimPrefix(words[0], "/") {
	case "quit", "q", "exit", "close", "closes":
		return p.HintQuit
	case "refresh", "reload", "retry", "terminal-restart":
		return p.HintRefresh
	case "cancel", "cancels", "reject", "rejects", "reset", "resets", "dismisses", "clear", "clears", "remove", "unpin", "normal/cancel":
		return p.HintCancel
	default:
		return p.Accent
	}
}

func keyHints(text string, p palette) string {
	parts := strings.Split(clean(text), " · ")
	for i, part := range parts {
		key, action, _ := strings.Cut(part, " ")
		if action == "" {
			parts[i] = bold(hintColor(key, p), key)
			continue
		}
		c := hintColor(action, p)
		parts[i] = bold(c, key)
		parts[i] += ink(c, " "+action)
	}
	return strings.Join(parts, ink(p.Muted, " · "))
}

func (a *App) noticeText(text string, p palette) string {
	phrases := [][2]string{
		{a.Config.Keys["send"] + " sends", "send"},
		{a.Config.Keys["send"] + " send", "send"},
		{a.Config.Keys["editor"] + " editor", "edit"},
		{"press " + a.Config.Keys["cancel"], "cancel"},
		{a.Config.Keys["terminal_unfocus"] + " returns focus", "focus"},
		{"shift+pgup scrolls", "scroll"}, {"enter to execute", "run"},
		{"enter runs the command", "run"}, {"enter inserts the command", "run"},
		{"enter executes", "run"}, {"esc returns to normal mode", "return"},
		{"press esc", "return"}, {":w", "send"},
		{"/retry", "retry"}, {"/terminal-restart", "terminal-restart"},
		{"/cancel", "cancel"}, {"/help", "help"}, {"/resume", "resume"}, {"/think", "think"}, {"/jobs", "jobs"},
	}
	replacements := make([]string, 0, len(phrases)*2)
	for _, phrase := range phrases {
		replacements = append(replacements, phrase[0], bold(hintColor(phrase[1], p), phrase[0]))
	}
	return strings.NewReplacer(replacements...).Replace(clean(text))
}
