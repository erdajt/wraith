package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

type proposal struct {
	Command, Language, Text, Source, Reason string
	Continue, Background, HideOutput        bool
	Resolved                                string
	Environment                             []string
	Variables                               variables.Snapshot
	prepared                                bool
}

func (a *App) history() runner.History {
	return runner.History{Root: filepath.Join(a.Paths.Data, "commands")}
}

func (a *App) approve(p proposal) tea.Cmd {
	if a.demo || a.busy {
		a.notice = "finish the current operation before running commands"
		return nil
	}
	if err := a.prepareProposal(&p); err != nil {
		a.notice = "command cannot run: " + err.Error()
		return nil
	}
	a.proposal = &p
	a.popup = &menu{Kind: "approval", Title: "approve command · " + a.runDirectory(), Items: []item{{"run", "run", "execute and save output"}, {"run · hide output from agent", "run-hidden", "execute without sharing output"}, {"edit", "edit", "save to execute once"}, {"view full command", "view", "scroll before deciding"}, {"ask assistant to modify", "modify", "describe the change"}, {"reject", "cancel", "do not execute"}}}
	if p.Background {
		a.popup.Title = "approve background command · " + a.runDirectory()
	}
	return nil
}

func (a *App) runDirectory() string {
	dir := a.Config.Terminal.Directory
	if strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[2:])
	}
	if dir == "" {
		dir, _ = os.Getwd()
	}
	return dir
}

func (a *App) runApproved() tea.Cmd {
	if a.proposal == nil || a.busy || a.demo {
		return nil
	}
	p := *a.proposal
	if err := a.prepareProposal(&p); err != nil {
		a.notice = err.Error()
		return nil
	}
	if p.Continue && a.generation == "" {
		a.generation = session.ID()
	}

	a.proposal = nil
	record := runner.Record{CaptureLimit: a.Config.Execution.CaptureBytes, TailLimit: a.Config.Execution.TailBytes, ID: session.ID(), Session: a.Session.ID, Command: p.Resolved, Environment: p.Environment, ShareOutput: !p.HideOutput && (a.Config.Execution.Mode == "agent" && p.Continue || a.Config.Execution.AutoShare), OutputHidden: p.HideOutput, Background: p.Background, Directory: a.runDirectory(), Shell: a.Config.Shell[0], Source: p.Source, Agent: p.Continue && a.Config.Execution.Mode == "agent", Generation: a.generation}
	if !p.Continue {
		record.Generation = ""
	}
	timeout, _ := time.ParseDuration(a.Config.Execution.CommandTimeout)
	a.jobs.SetLimit(a.Config.Execution.MaxJobs)
	if err := a.jobs.Start(context.Background(), record, timeout); err != nil {
		a.proposal = &p
		a.notice = "command launch: " + err.Error()
		return nil
	}
	a.agentStopped = false
	if p.Continue {
		a.agentSteps++
	}
	a.cancel = func() { a.jobs.Cancel(record.ID) }
	a.busy, a.executing = true, true
	a.phase = "running command"
	record.Status = "starting"
	a.activeRun = record
	a.showRun(record, "")
	if p.Background {
		a.busy, a.executing, a.cancel = false, false, nil
		if record.Agent {
			candidate := a.Session.Clone()
			candidate.Messages = append(candidate.Messages, session.Message{Role: "user", Content: a.Variables.Redact(fmt.Sprintf("approved background command started\nid: %s\ncommand: %s\noutput shared: %t\npoll by id or wait for its completion notification; next_poll_ms: 1000", record.ID, record.Command, record.ShareOutput))})
			if err := a.Store.Save(candidate); err != nil {
				a.notice = "job started; receipt save failed: " + err.Error()
				return nil
			}
			a.Session = candidate
			if a.Prompt.String() == "" {
				return a.continueEvidence()
			}
		}
	}
	return nil
}

func (a *App) syncJobs() tea.Cmd {
	if a.jobs == nil {
		return nil
	}
	select {
	case <-a.jobs.Updates:
		for _, job := range a.jobs.Snapshot() {
			r := job.Record
			if a.popup != nil && a.popup.Kind == "run-output" && a.popup.Record.ID == r.ID {
				a.showRun(r, r.Output)
			}
			if r.ID == a.activeRun.ID && a.executing {
				a.activeRun = r
				if !r.Finished.IsZero() {
					a.busy, a.executing, a.cancel = false, false, nil
					a.notice = fmt.Sprintf("command %s · exit %d · /history", r.Status, r.ExitCode)
				}
			}
			if !r.Finished.IsZero() {
				if job.Err != nil {
					a.notice = "command history: " + job.Err.Error()
					continue
				}
				a.pendingRuns[r.ID] = r
				a.jobs.Forget(r.ID)
			}
		}
	default:
	}
	if a.pendingTool != nil {
		return a.runPendingTool()
	}
	return a.deliverRuns()
}

func (a *App) deliverRuns() tea.Cmd {
	if !a.canContinue() {
		return nil
	}
	candidate, continueAgent := a.pendingEvidence()
	if len(candidate.DeliveredRuns) == len(a.Session.DeliveredRuns) {
		return nil
	}
	if err := a.Store.Save(candidate); err != nil {
		a.notice = "command evidence pending: " + err.Error()
		return nil
	}
	a.Session = candidate
	for _, id := range candidate.DeliveredRuns {
		delete(a.pendingRuns, id)
	}
	if continueAgent {
		return a.continueEvidence()
	}
	return nil
}

func (a *App) showRun(r runner.Record, output string) {
	text := fmt.Sprintf("$ %s\n\ndirectory: %s\nstatus: %s · exit: %d\n%s\n\n%s", r.Command, r.Directory, r.Status, r.ExitCode, r.Error, output)
	if r.Truncated {
		text += "\n[capture limit reached; retained tail follows]\n" + r.Tail
	}
	previous := a.popup
	a.showText("run-output", "command output", text)
	a.popup.Record = r
	a.popup.Reader = newReader(clean(text))
	if previous != nil && previous.Reader != nil && previous.Record.ID == r.ID {
		a.popup.Reader = previous.Reader
		if !previous.Reader.Frozen {
			a.popup.Reader.Buffer.Text = []rune(clean(text))
		}
	}
}

func (a *App) showText(kind, title, text string) {
	var items []item
	for _, line := range strings.Split(ansi.Hardwrap(clean(text), max(20, min(a.width-12, 84)), true), "\n") {
		items = append(items, item{Label: line})
	}
	a.popup = &menu{Kind: kind, Title: title, Items: items}
}

func (a *App) openHistory() tea.Cmd {
	records, err := a.jobList()
	if err != nil {
		a.notice = err.Error()
		return nil
	}
	items := []item{}
	for _, r := range records {
		items = append(items, item{Label: r.Command, Value: r.ID, Detail: fmt.Sprintf("%s · %s · exit %d", r.Started.Local().Format("jan 02 15:04"), r.Status, r.ExitCode)})
	}
	a.popup = &menu{Kind: "history", Title: "approved commands · enter opens saved output", Items: items}
	return nil
}

func (a *App) prepareProposal(p *proposal) error {
	if p.prepared {
		return nil
	}
	if err := a.refreshVariables(); err != nil {
		return err
	}
	resolved, err := a.Variables.Shell(p.Command)
	if err != nil {
		return err
	}
	p.Resolved, p.Variables = resolved, a.Variables
	p.Environment = append(os.Environ(), a.Variables.Environment()...)
	p.prepared = true
	return nil
}

func (a *App) canContinue() bool {
	if a.busy || a.loading || a.editing || a.Prompt.String() != "" || a.visualMode != "" || a.focus == "terminal0" || a.focus == "terminal1" {
		return false
	}
	return a.popup == nil || a.popup.Kind == "run-output" && (a.popup.Reader == nil || !a.popup.Reader.Frozen)
}

func (a *App) pendingEvidence() (session.Session, bool) {
	candidate := a.Session.Clone()
	continueAgent := false
	records := make([]runner.Record, 0, len(a.pendingRuns))
	for _, r := range a.pendingRuns {
		records = append(records, r)
	}
	slices.SortFunc(records, func(a, b runner.Record) int { return a.Started.Compare(b.Started) })
	for _, r := range records {
		id := r.ID
		if r.Session != candidate.ID {
			continue
		}
		if slices.Contains(candidate.DeliveredRuns, id) {
			delete(a.pendingRuns, id)
			continue
		}
		candidate.DeliveredRuns = append(candidate.DeliveredRuns, id)
		if !slices.Contains(candidate.SharedRuns, id) && (r.Agent || r.ShareOutput && !r.OutputHidden) {
			candidate.Messages = append(candidate.Messages, session.Message{Role: "user", Content: a.commandEvidence(r)})
			if r.ShareOutput && !r.OutputHidden {
				candidate.SharedRuns = append(candidate.SharedRuns, id)
			}
			continueAgent = continueAgent || r.Generation == a.generation && !a.agentStopped && r.Status != "cancelled"
		}
	}
	return candidate, continueAgent
}
