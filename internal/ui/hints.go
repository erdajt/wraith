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
		c := hintColor(action, p)
		parts[i] = bold(c, key)
		if action != "" {
			parts[i] += ink(c, " "+action)
		}
	}
	return strings.Join(parts, ink(p.Muted, " · "))
}
