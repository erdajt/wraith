package ui

import (
	"slices"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
)

func (a *App) outputExcerpt(output string) string {
	output = clean(output)
	budget := min(a.Config.Execution.OutputContext, max(256, (a.contextLimit()-a.Config.Predict)/4))
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
	r.ShareOutput, r.OutputHidden = true, false
	if err := a.history().Save(r); err != nil {
		a.notice = "output sharing: " + err.Error()
		return nil
	}
	evidence := a.commandEvidence(r)
	candidate := a.Session.Clone()
	candidate.SharedRuns = append(candidate.SharedRuns, r.ID)
	if !slices.Contains(candidate.DeliveredRuns, r.ID) {
		candidate.DeliveredRuns = append(candidate.DeliveredRuns, r.ID)
	}
	candidate.Messages = append(candidate.Messages, evidence.Message())
	if err := a.Store.Save(candidate); err != nil {
		a.notice = "output remains in /history; chat save failed: " + err.Error()
		return nil
	}
	a.Session = candidate
	delete(a.pendingRuns, r.ID)
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

func (a *App) commandEvidence(r runner.Record) session.Evidence {
	evidence := session.Evidence{ID: r.ID, Kind: "result", Command: a.Variables.Redact(r.Command), Directory: a.Variables.Redact(r.Directory), Status: r.Status, ExitCode: r.ExitCode, Hidden: r.OutputHidden || !r.ShareOutput}
	if evidence.Hidden {
		return evidence
	}
	page, err := runner.ReadOutput(r, runner.Query{Limit: 40}, a.Variables)
	if err != nil {
		evidence.Error = "output unavailable"
		return evidence
	}
	output := ""
	for _, line := range page.Lines {
		output += line.Text + "\n"
	}
	if r.Truncated && r.TailOffset > 0 {
		tail, err := runner.ReadOutput(r, runner.Query{From: r.TailLine, Limit: 20}, a.Variables)
		if err == nil {
			output += "[capture gap; retained tail]\n"
			for _, line := range tail.Lines {
				output += line.Text + "\n"
			}
		}
	}
	evidence.Error, evidence.Output = a.Variables.Redact(r.Error), a.outputExcerpt(output)
	return evidence
}
