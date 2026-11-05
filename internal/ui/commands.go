package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

func (a *App) runCommand(line string) tea.Cmd {
	switch strings.Trim(line, " /:") {
	case "compact":
		return a.startCompact()
	case "cancel":
		if a.cancel != nil {
			a.cancel()
		}
	}
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
