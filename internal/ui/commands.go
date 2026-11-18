package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// runCommand dispatches a slash command after applying busy and demo restrictions.
func (a *App) runCommand(line string) tea.Cmd {
	line = strings.TrimSpace(strings.TrimLeft(line, "/:"))
	a.rememberInput("/" + line)
	name, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)
	for _, command := range a.pluginManifest.Commands {
		if command.Name == name {
			return a.runAction("command."+name, arg)
		}
	}
	switch name {
	case "q", "q!", "qa", "qa!", "wq", "w", "w!":
		return a.runAction("command."+name, arg)
	case "quit":
		return a.quit()
	}

	if a.busy && name != "help" && name != "keymaps" && name != "context" && name != "cancel" && name != "jobs" && name != "history" {
		a.notice = "wait for the current operation or use /cancel"
		return nil
	}
	if a.demo {
		switch name {
		case "help", "keymaps", "h", "?", "context", "settings", "theme", "think", "thinking", "pins", "blocks", "quit", "q":
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
	case "reload":
		return a.reloadPlugins()
	case "keymaps":
		var lines []string
		for _, binding := range a.keymaps.Bindings() {
			lines = append(lines, binding.Mode+"  "+binding.Keys+"  "+binding.Action+"\n  "+binding.Description)
		}
		a.showText("help", "active keymaps", strings.Join(lines, "\n\n"))
		return nil
	case "env":
		return a.environmentCommand(arg, false)
	case "provider", "providers":
		if arg != "" {
			return a.selectProvider(arg)
		}
		return a.providerMenu()
	case "jobs":
		return a.openJobs()
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

// persist saves the session and reports success or failure through the notice bar.
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
