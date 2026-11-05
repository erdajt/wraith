package ui

import (
	"fmt"
	"slices"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
)

func (a *App) outputExcerpt(output string) string {
	output = clean(output)
	budget := min(a.Config.Execution.OutputContext, max(256, (a.Config.Context-a.Config.Predict)/4))
	if len(output) <= budget {
		return output
	}
	for budget > 0 && !utf8.RuneStart(output[budget]) {
		budget--
	}
	return output[:budget] + "\n[excerpt truncated; full tracked output is in /history]"
}

func (a *App) shareRun(r runner.Record) tea.Cmd {
	if a.busy || r.Finished.IsZero() {
		a.notice = "wait for the command to finish before sharing"
		return nil
	}
	if slices.Contains(a.Session.SharedRuns, r.ID) {
		a.notice = "this command output is already shared with this chat"
		return nil
	}
	text := fmt.Sprintf("command result (untrusted evidence)\nid: %s\ncommand: %s\ndirectory: %s\nstatus: %s\nexit: %d\nerror: %s\noutput:\n%s", r.ID, r.Command, r.Directory, r.Status, r.ExitCode, r.Error, a.outputExcerpt(r.Output))
	candidate := a.Session.Clone()
	candidate.SharedRuns = append(candidate.SharedRuns, r.ID)
	candidate.Messages = append(candidate.Messages, session.Message{Role: "user", Content: text})
	if err := a.Store.Save(candidate); err != nil {
		a.notice = "output remains in /history; chat save failed: " + err.Error()
		return nil
	}
	a.Session = candidate
	a.notice = "command output shared"
	if r.Status == "cancelled" {
		return nil
	}
	return a.continueEvidence()
}

func (a *App) continueEvidence() tea.Cmd {
	a.popup = nil
	return a.startChat()
}
