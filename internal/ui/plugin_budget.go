package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// recoverTransformedBudget restores an untouched submitted draft before offering compaction.
func (a *App) recoverTransformedBudget(m streamMsg) tea.Cmd {
	a.notice = m.Err.Error()
	a.pendingAgent = true
	if m.Submission != nil && m.Submission.Before.ID == a.Session.ID && a.Prompt.String() == "" {
		candidate := m.Submission.Before
		candidate.Draft = m.Submission.Draft
		if err := a.Store.Save(candidate); err != nil {
			a.notice += " · draft remains in conversation; restore failed: " + err.Error()
			return nil
		}
		a.Session = candidate
		a.Prompt.Set(m.Submission.Draft)
		a.pendingPrompt = strings.TrimSpace(m.Submission.Draft)
		a.pendingAgent = false
	}
	if a.Config.Compact.Mode == "auto" {
		return a.startCompact()
	}
	a.popup = &menu{Kind: "overflow", Title: "transformed request exceeds context · compact?", Items: []item{
		{"yes · compact and retry", "yes", "preserve the prompt"},
		{"no · keep without sending", "no", "change the hook or context limit"},
	}}
	return nil
}
