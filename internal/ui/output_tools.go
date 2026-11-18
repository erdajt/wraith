package ui

import (
	"errors"
	"github.com/erdajt/wraith/internal/provider"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
)

// jobRecord finds a live job snapshot or loads its saved command record.
func (a *App) jobRecord(id string) (runner.Record, error) {
	for _, job := range a.jobs.Snapshot() {
		if job.Record.ID == id {
			return job.Record, nil
		}
	}
	return a.history().Load(id)
}

// runPendingTool checks output access and budgets, persists bounded tool evidence and resumes the agent when ready.
func (a *App) runPendingTool() tea.Cmd {
	if a.agentStopped {
		a.pendingTool = nil
		return nil
	}
	if a.pendingTool == nil || !a.canContinue() {
		return nil
	}
	if a.agentReads >= a.Config.Execution.MaxReads {
		a.pendingTool, a.agentStopped = nil, true
		a.notice = "output read limit reached · send a prompt to continue"
		return nil
	}

	request := *a.pendingTool
	for _, job := range a.jobs.Snapshot() {
		if job.Record.ID == request.ID && job.Err != nil {
			a.pendingTool, a.agentStopped = nil, true
			a.notice = "output result is not saved; inspect /jobs before continuing"
			return nil
		}
	}

	used, budgetErr := a.chatBudget(a.Session)
	var overflow *provider.ContextError
	if budgetErr != nil && !errors.As(budgetErr, &overflow) {
		a.notice = budgetErr.Error()
		return nil
	}
	remaining := a.contextLimit() - a.Config.Predict - 128 - used
	available := min(a.Config.Execution.OutputContext, max(0, remaining-256))
	budget := min(a.Config.Execution.OutputContext, available*3)
	if budget < 256 {
		return a.deferOutputRead()
	}

	nextLine := a.pollNext[request.ID]
	r, err := a.jobRecord(request.ID)
	text := "output tool denied: command is not in this session"
	var evidence *session.Evidence
	waiting := false
	if err == nil && r.Session == a.Session.ID {
		if !r.ShareOutput || r.OutputHidden {
			text = "output tool denied: output withheld by user"
		} else if request.Tool == "poll" && r.Finished.IsZero() && time.Since(a.lastPoll[r.ID]) < time.Second {
			waiting = true
			evidence = &session.Evidence{ID: r.ID, Kind: "waiting", Status: r.Status}
		} else {
			if request.Tool == "poll" {
				request.Limit = 20
				request.From = max(1, nextLine)
			}
			page, queryErr := runner.ReadOutput(r, request.Query, a.Variables)
			if queryErr != nil {
				text = "output tool rejected: " + queryErr.Error()
			} else {
				size := 0
				for i, line := range page.Lines {
					if len(line.Text)+40 > budget {
						page.Lines[i].Text = "[line exceeds output budget; inspect local history]"
						line = page.Lines[i]
						page.Truncated = true
					}
					size += len(line.Text) + 40
					if size > budget {
						page.Next = line.Number
						page.Truncated = true
						page.Lines = page.Lines[:i]
						break
					}
				}

				if request.Tool == "poll" {
					if page.Next > 0 {
						nextLine = page.Next
					} else if len(page.Lines) > 0 {
						nextLine = page.Lines[len(page.Lines)-1].Number + 1
					}
				}

				evidence = &session.Evidence{ID: r.ID, Kind: "output", Status: r.Status, ExitCode: r.ExitCode, Page: &page}
				if request.Tool == "poll" && r.Finished.IsZero() && len(page.Lines) == 0 {
					waiting, evidence.Kind = true, "waiting"
				}
			}
		}
	}

	candidate := a.Session.Clone()
	message := session.Message{Role: "user", Content: a.Variables.Redact(text)}
	if evidence != nil {
		message = evidence.Message()
	}
	candidate.Messages = append(candidate.Messages, message)
	if _, err := a.chatBudget(candidate); err != nil {
		if errors.As(err, &overflow) {
			return a.deferOutputRead()
		}
		a.notice = err.Error()
		return nil
	}

	if err := a.Store.Save(candidate); err != nil {
		a.notice = "output tool result not saved: " + err.Error()
		a.pendingTool, a.agentStopped = nil, true
		return nil
	}
	a.pendingTool = nil
	a.agentReads++
	if request.Tool == "poll" {
		a.pollNext[request.ID] = nextLine
		a.lastPoll[request.ID] = time.Now()
	}
	a.Session = candidate

	if waiting {
		a.notice = "waiting for background job " + r.ID + " · /jobs shows live output"
		return nil
	}
	return a.continueEvidence()
}

// deferOutputRead requests compaction while retaining a pending read or pauses it if compaction cannot start.
func (a *App) deferOutputRead() tea.Cmd {
	a.notice = "output read pending; compact context before adding evidence"

	if a.Config.Compact.Mode == "auto" {
		cmd := a.startCompact()
		if !a.busy {
			a.pendingTool, a.agentStopped = nil, true
		}
		return cmd
	}

	a.popup = &menu{Kind: "overflow", Title: "compact before reading command output?", Items: []item{{"yes", "yes", "archive and compact"}, {"no", "no", "pause without consuming output"}}}
	return nil
}
