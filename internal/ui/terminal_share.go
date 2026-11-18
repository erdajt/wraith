package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/session"
)

// terminalEvidence formats a bounded, redacted terminal snapshot as untrusted model evidence.
func (a *App) terminalEvidence(index int, text string) session.Message {
	return session.Message{Role: "user", Content: fmt.Sprintf("terminal pane %d snapshot (untrusted evidence; command boundaries and exit status are unknown):\n%s", index+1, a.outputExcerpt(a.Variables.Redact(clean(text))))}
}

// shareTerminal persists selected terminal text as evidence and schedules model continuation.
func (a *App) shareTerminal(index int) tea.Cmd {
	if a.busy {
		a.notice = "finish the current operation before sharing"
		return nil
	}

	if err := a.refreshVariables(); err != nil {
		a.notice = err.Error()
		return nil
	}

	p := a.panes[index]
	text := p.Reader.Buffer.String()
	start, end := p.Reader.Buffer.Selection()
	if start >= 0 {
		text = string(p.Reader.Buffer.Text[start:min(end, len(p.Reader.Buffer.Text))])
	}
	if strings.TrimSpace(text) == "" {
		a.notice = "no terminal output selected"
		return nil
	}

	candidate := a.Session.Clone()
	candidate.Messages = append(candidate.Messages, a.terminalEvidence(index, text))
	if err := a.Store.Save(candidate); err != nil {
		a.notice = err.Error()
		return nil
	}
	a.Session = candidate
	p.SharedText = text
	a.notice = "terminal snapshot shared"
	a.focus = "prompt"
	return a.continueEvidence()
}

// syncTerminalEvidence persists changed terminal snapshots before marking them as shared.
func (a *App) syncTerminalEvidence() error {
	candidate := a.Session.Clone()
	changed := map[int]string{}
	for i, p := range a.panes {
		if p == nil || p.Screen == nil {
			continue
		}
		text := terminalText(p.Screen)
		if strings.TrimSpace(text) == "" || text == p.SharedText {
			continue
		}
		changed[i] = text
		candidate.Messages = append(candidate.Messages, a.terminalEvidence(i, text))
	}
	if len(changed) == 0 {
		return nil
	}
	if err := a.Store.Save(candidate); err != nil {
		return err
	}
	a.Session = candidate
	for i, text := range changed {
		a.panes[i].SharedText = text
	}
	return nil
}
