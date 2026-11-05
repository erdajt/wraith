package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

func (a *App) runCommand(line string) tea.Cmd {
	line = strings.TrimSpace(strings.TrimLeft(line, "/:"))
	name, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)
	if a.busy && name != "help" && name != "context" && name != "cancel" {
		a.notice = "wait for the current operation or use /cancel"
		return nil
	}
	handlers := map[string]func(string, string) tea.Cmd{"model": a.modelCommand, "models": a.modelCommand, "think": a.modelCommand, "settings": a.modelCommand, "set": a.modelCommand, "theme": a.modelCommand, "context": a.modelCommand, "compact": a.modelCommand, "cancel": a.modelCommand, "thinking": a.modelCommand}
	if handler, ok := handlers[name]; ok {
		return handler(name, arg)
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
