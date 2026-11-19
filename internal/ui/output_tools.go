package ui

import (
	"context"
	"errors"
	"reflect"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/provider"
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
		a.cancelOutputRead()
		return nil
	}
	a.reconcileOutputRead()
	if a.pendingTool == nil || !a.canContinue() {
		return nil
	}
	if a.outputRead != nil {
		if a.outputRead.result != nil {
			return a.finishOutputRead(*a.outputRead.result)
		}
		return nil
	}
	if a.pendingTool.Tool == "extension" {
		request := *a.pendingTool
		a.pendingTool = nil
		return a.proposeExtension(request.Name, request.Arguments)
	}
	if a.agentReads >= a.Config.Execution.MaxReads {
		a.pendingTool, a.agentStopped = nil, true
		a.notice = "output read limit reached · send a prompt to continue"
		return nil
	}

	request := *a.pendingTool
	var record runner.Record
	for _, job := range a.jobs.Snapshot() {
		if job.Record.ID == request.ID {
			record = job.Record
		}
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	state := &outputReadState{
		pending: a.pendingTool, request: request, runtime: a.plugins,
		session: a.Session.ID, generation: a.generation, mode: a.Config.Execution.Mode,
		variables: a.Variables, cancel: cancel,
	}
	a.outputRead = state
	if a.workspaceScans == nil {
		a.workspaceScans = &workspaceScanStore{}
	}
	scans, history := a.workspaceScans, a.history()
	nextLine, lastPoll := a.pollNext[request.ID], a.lastPoll[request.ID]
	a.workers.Add(1)
	go func() {
		defer a.workers.Done()
		defer cancel()
		a.post(readToolOutput(ctx, state, scans, history, record, budget, nextLine, lastPoll))
	}()
	return a.next()
}

// finishOutputRead saves a current read before advancing its cursor or starting a response.
func (a *App) finishOutputRead(result outputReadMsg) tea.Cmd {
	a.reconcileOutputRead()
	if a.outputRead != result.state {
		return nil
	}
	if !a.canContinue() {
		a.outputRead.result = &result
		return nil
	}
	request := result.state.request
	for _, job := range a.jobs.Snapshot() {
		if job.Record.ID != request.ID {
			continue
		}
		if job.Err != nil {
			a.cancelOutputRead()
			a.pendingTool, a.agentStopped = nil, true
			a.notice = "output result is not saved; inspect /jobs before continuing"
			return nil
		}
		if !job.Record.ShareOutput || job.Record.OutputHidden {
			result.text, result.evidence, result.waiting = "output tool denied: output withheld by user", nil, false
			result.nextLine = a.pollNext[request.ID]
		}
	}
	if record, ok := a.pendingRuns[request.ID]; ok && (!record.ShareOutput || record.OutputHidden) {
		result.text, result.evidence, result.waiting = "output tool denied: output withheld by user", nil, false
		result.nextLine = a.pollNext[request.ID]
	}
	a.cancelOutputRead()
	candidate := a.Session.Clone()
	message := session.Message{Role: "user", Content: a.Variables.Redact(result.text)}
	if result.evidence != nil {
		message = result.evidence.Message()
	}
	candidate.Messages = append(candidate.Messages, message)
	if _, err := a.chatBudget(candidate); err != nil {
		var overflow *provider.ContextError
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
		a.pollNext[request.ID] = result.nextLine
		a.lastPoll[request.ID] = time.Now()
	}
	a.Session = candidate

	if result.waiting {
		a.notice = "waiting for background job " + request.ID + " · /jobs shows live output"
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

// reconcileOutputRead cancels work whose privacy snapshot or conversation owner changed.
func (a *App) reconcileOutputRead() {
	state := a.outputRead
	if state == nil {
		return
	}
	ownerChanged := state.session != a.Session.ID || state.generation != a.generation || state.runtime != a.plugins || state.mode != a.Config.Execution.Mode
	if ownerChanged && a.pendingTool == state.pending {
		a.pendingTool = nil
	}
	if ownerChanged || state.pending != a.pendingTool || a.pendingTool == nil || !reflect.DeepEqual(state.request, *a.pendingTool) || a.agentStopped || !state.variables.Equal(a.Variables) {
		a.cancelOutputRead()
	}
}

// cancelOutputRead releases an outstanding read without consuming its pending request.
func (a *App) cancelOutputRead() {
	if a.outputRead != nil {
		a.outputRead.cancel()
		a.outputRead = nil
	}
}
