package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
)

func (a *App) jobRecord(id string) (runner.Record, error) {
	for _, job := range a.jobs.Snapshot() {
		if job.Record.ID == id {
			return job.Record, nil
		}
	}
	return a.history().Load(id)
}

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
	a.agentReads++
	nextLine := a.pollNext[request.ID]
	r, err := a.jobRecord(request.ID)
	text := "output tool denied: command is not in this session"
	var evidence *session.Evidence
	if err == nil && r.Session == a.Session.ID {
		if !r.ShareOutput || r.OutputHidden {
			text = "output tool denied: output withheld by user"
		} else if request.Tool == "poll" && time.Since(a.lastPoll[r.ID]) < time.Second {
			text = "output tool: not ready; next_poll_ms: 1000"
		} else {
			if request.Tool == "poll" {
				a.lastPoll[r.ID] = time.Now()
				request.Limit = 20
				request.From = max(1, nextLine)
			}
			page, queryErr := runner.ReadOutput(r, request.Query, a.Variables)
			if queryErr != nil {
				text = "output tool rejected: " + queryErr.Error()
			} else {
				size := 0
				budget := min(a.Config.Execution.OutputContext, max(256, (a.contextLimit()-a.Config.Predict)/4))
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
			}
		}
	}
	candidate := a.Session.Clone()
	message := session.Message{Role: "user", Content: a.Variables.Redact(text)}
	if evidence != nil {
		message = evidence.Message()
	}
	candidate.Messages = append(candidate.Messages, message)
	if err := a.Store.Save(candidate); err != nil {
		a.notice = "output tool result not saved: " + err.Error()
		a.pendingTool, a.agentStopped = nil, true
		return nil
	}
	a.pendingTool = nil
	if request.Tool == "poll" {
		a.pollNext[request.ID] = nextLine
	}
	a.Session = candidate
	return a.continueEvidence()
}
