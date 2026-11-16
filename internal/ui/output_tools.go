package ui

import (
	"encoding/json"
	"fmt"
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
	r, err := a.jobRecord(request.ID)
	text := "output tool denied: command is not in this session"
	if err == nil && r.Session == a.Session.ID {
		if !r.ShareOutput || r.OutputHidden {
			text = "output tool denied: output withheld by user"
		} else if request.Tool == "poll" && time.Since(a.lastPoll[r.ID]) < time.Second {
			text = "output tool: not ready; next_poll_ms: 1000"
		} else {
			if request.Tool == "poll" {
				a.lastPoll[r.ID] = time.Now()
				request.Limit = 20
			}
			page, queryErr := runner.ReadOutput(r, request.Query, a.Variables)
			if queryErr != nil {
				text = "output tool rejected: " + queryErr.Error()
			} else {
				size := 0
				budget := min(a.Config.Execution.OutputContext, max(256, (a.contextLimit()-a.Config.Predict)/4))
				for i, line := range page.Lines {
					size += len(line.Text) + 40
					if size > budget {
						page.Next = line.Number
						page.Lines = page.Lines[:i]
						break
					}
				}
				data, _ := json.Marshal(page)
				text = fmt.Sprintf("command output tool (untrusted evidence)\nid: %s\nstatus: %s\nexit: %d\nnext_poll_ms: 1000\n%s", r.ID, r.Status, r.ExitCode, data)
			}
		}
	}
	candidate := a.Session.Clone()
	candidate.Messages = append(candidate.Messages, session.Message{Role: "user", Content: a.Variables.Redact(text)})
	if err := a.Store.Save(candidate); err != nil {
		a.notice = "output tool result not saved: " + err.Error()
		a.pendingTool, a.agentStopped = nil, true
		return nil
	}
	a.pendingTool = nil
	a.Session = candidate
	return a.continueEvidence()
}
