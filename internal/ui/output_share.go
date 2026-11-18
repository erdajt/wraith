package ui

import (
	"slices"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
)

// outputExcerpt bounds output to the current evidence budget without splitting utf-8 characters.
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

// shareRun persists explicit output-sharing permission and adds command evidence to the session.
func (a *App) shareRun(r runner.Record) tea.Cmd {
	if a.busy || r.Finished.IsZero() {
		a.notice = "wait for the command to finish before sharing"
		return nil
	}

	if err := a.refreshVariables(); err != nil {
		a.notice = err.Error()
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

// continueEvidence clears receipt state and starts a model response to newly available evidence.
func (a *App) continueEvidence() tea.Cmd {
	a.receiptSession, a.receiptGeneration = "", ""
	a.popup = nil
	return a.startChat()
}

// commandEvidence builds redacted command evidence with bounded head and tail output when sharing permits it.
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
	evidence.Stats = &page.Stats
	output := ""
	for _, line := range page.Lines {
		output += line.Text + "\n"
	}

	if r.Truncated && r.TailOffset > 0 {
		tail, err := runner.ReadOutput(r, runner.Query{From: max(r.TailLine+1, r.TailLine+int64(strings.Count(r.Tail, "\n"))-19), Limit: 20}, a.Variables)
		if err == nil {
			recent := ""
			for _, line := range tail.Lines {
				recent += line.Text + "\n"
			}

			gap := "\n[capture gap; retained tail]\n"
			budget := min(a.Config.Execution.OutputContext, max(256, (a.contextLimit()-a.Config.Predict)/4)) - len(gap)
			headLimit := min(len(output), budget/2)
			for headLimit > 0 && headLimit < len(output) && !utf8.RuneStart(output[headLimit]) {
				headLimit--
			}

			tailStart := max(0, len(recent)-(budget-headLimit))
			for tailStart < len(recent) && !utf8.RuneStart(recent[tailStart]) {
				tailStart++
			}
			output = output[:headLimit] + gap + recent[tailStart:]
		}
	}

	evidence.Error, evidence.Output = a.Variables.Redact(r.Error), a.outputExcerpt(output)
	return evidence
}
