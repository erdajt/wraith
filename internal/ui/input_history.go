package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) rememberInput(text string) {
	text = strings.TrimSpace(text)
	if a.demo || text == "" {
		return
	}
	a.inputIndex = -1
	n := len(a.inputHistory)
	if n > 0 && a.inputHistory[n-1] == text {
		return
	}
	a.inputHistory = append(a.inputHistory, text)
	total := 0
	start := len(a.inputHistory)
	for start > 0 && len(a.inputHistory)-start < 100 {
		size := len(a.inputHistory[start-1])
		if total+size > 512<<10 {
			break
		}
		total += size
		start--
	}
	a.inputHistory = a.inputHistory[start:]
	if err := a.Store.SaveInputs(a.Directory, a.inputHistory); err != nil {
		a.notice = "input history: " + err.Error()
	}
}

func (a *App) historyKey(k tea.KeyMsg) bool {
	if a.focus != "prompt" || a.Prompt.Mode != "insert" || (k.String() != "up" && k.String() != "down") {
		return false
	}
	if len(a.inputHistory) == 0 {
		return false
	}
	if a.inputIndex < 0 {
		if k.String() == "down" {
			return false
		}
		a.inputDraft, a.inputDraftPosition = a.Prompt.String(), a.Prompt.Pos
		a.inputIndex = len(a.inputHistory)
	}
	if k.String() == "up" {
		a.inputIndex = max(0, a.inputIndex-1)
	} else {
		a.inputIndex++
	}
	if a.inputIndex >= len(a.inputHistory) {
		a.Prompt.Set(a.inputDraft)
		a.Prompt.Pos = a.inputDraftPosition
		a.inputIndex = -1
	} else {
		a.Prompt.Set(a.inputHistory[a.inputIndex])
	}
	return true
}
