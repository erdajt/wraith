package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) runCommand(line string) tea.Cmd {
	line = strings.TrimSpace(strings.TrimLeft(line, "/:"))
	name, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)
	if a.busy && name != "help" && name != "context" && name != "cancel" {
		a.notice = "wait for the current operation or use /cancel"
		return nil
	}
	if a.demo {
		switch name {
		case "help", "h", "?", "context", "settings", "theme", "think", "thinking", "pins", "blocks", "quit", "q":
		default:
			a.notice = "this command is disabled in demo mode"
			return nil
		}
	}
	if strings.HasPrefix(name, "settings.") {
		for _, cmd := range subcommands["settings"] {
			if cmd.Name == name {
				a.settingsCategory(strings.TrimPrefix(name, "settings."))
				return nil
			}
		}
		a.notice = "unknown settings category"
		return nil
	}
	switch name {
	case "history":
		return a.openHistory()
	case "model", "models", "think", "settings", "set", "theme", "context", "compact", "cancel", "thinking":
		return a.modelCommand(name, arg)
	case "new", "clear", "chats", "sessions", "resume", "archives", "restore", "export", "retry", "terminal-restart", "quit", "q":
		return a.sessionCommand(name, arg)
	case "help", "h", "?", "instructions", "memory", "remember", "pin", "unpin", "pins", "blocks", "edit", "edit-response":
		return a.memoryCommand(name, arg)
	}
	a.notice = "unknown command · /help"
	return nil
}
func (a *App) persist(message string) {
	if a.demo {
		a.notice = "demo state is not saved"
		return
	}
	if err := a.Store.Save(a.Session); err != nil {
		a.notice = "save failed: " + err.Error()
	} else {
		a.notice = message
	}
}
