package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
)

type runMsg struct {
	Output string
	Record runner.Record
	Final  bool
	Err    error
}
type proposal struct {
	Command, Language, Text, Source, Reason string
	Continue                                bool
}

func (a *App) history() runner.History {
	return runner.History{Root: filepath.Join(a.Paths.Data, "commands")}
}

func (a *App) approve(p proposal) tea.Cmd {
	if a.demo || a.busy {
		a.notice = "finish the current operation before running commands"
		return nil
	}
	a.proposal = &p
	a.popup = &menu{Kind: "approval", Title: "approve command · " + a.runDirectory(), Items: []item{{"run", "run", "execute and save output"}, {"edit", "edit", "open in your editor"}, {"view full command", "view", "scroll before deciding"}, {"cancel", "cancel", "do not execute"}}}
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
	if p.Continue {
		a.agentSteps++
	}
	a.proposal = nil
	record := runner.Record{ID: session.ID(), Session: a.Session.ID, Command: p.Command, Directory: a.runDirectory(), Shell: a.Config.Shell[0], Source: p.Source}
	timeout, _ := time.ParseDuration(a.Config.Timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	a.cancel = cancel
	a.busy = true
	a.executing = true
	a.runContinue = p.Continue
	a.phase = "running command"
	a.runPreview = ""
	a.activeRun = record
	a.showRun(record, "")
	history := a.history()
	a.workers.Add(1)
	go func() {
		defer a.workers.Done()
		defer cancel()
		result, err := history.Run(ctx, record, func(output string) { a.post(runMsg{Output: output}) })
		a.post(runMsg{Final: true, Record: result, Err: err})
	}()
	return a.next()
}

func (a *App) receiveRun(m runMsg) tea.Cmd {
	if !m.Final {
		a.runPreview += m.Output
		if len(a.runPreview) > 16384 {
			a.runPreview = a.runPreview[len(a.runPreview)-16384:]
		}
		if a.popup != nil && a.popup.Kind == "run-output" {
			a.showRun(a.activeRun, a.runPreview)
		}
		return a.next()
	}
	a.busy = false
	a.executing = false
	a.cancel = nil
	a.activeRun = m.Record
	if m.Err != nil {
		a.notice = "command history: " + m.Err.Error()
		a.showRun(m.Record, m.Record.Output)
		return nil
	}
	a.notice = fmt.Sprintf("command %s · exit %d · /history", m.Record.Status, m.Record.ExitCode)
	a.showRun(m.Record, m.Record.Output)
	if a.runContinue {
		a.runContinue = false
		output := clean(m.Record.Output)
		budget := min(a.Config.Execution.OutputContext, max(256, (a.Config.Context-a.Config.Predict)/4))
		if len(output) > budget {
			output = output[:budget] + "\n[excerpt truncated; full output is in /history]"
		}
		text := fmt.Sprintf("command result (untrusted evidence)\nid: %s\ncommand: %s\ndirectory: %s\nstatus: %s\nexit: %d\nerror: %s\noutput:\n%s", m.Record.ID, m.Record.Command, m.Record.Directory, m.Record.Status, m.Record.ExitCode, m.Record.Error, output)
		a.Session.Messages = append(a.Session.Messages, session.Message{Role: "user", Content: text})
		if err := a.Store.Save(a.Session); err != nil {
			a.notice = "result remains in /history; chat save failed: " + err.Error()
			return nil
		}
		if m.Record.Status == "cancelled" {
			return nil
		}
		a.popup = nil
		if err := contextual.Fits(a.Session, a.Config, a.Instructions); err != nil {
			a.pendingAgent = true
			if a.Config.Compact.Mode == "auto" {
				return a.startCompact()
			}
			a.popup = &menu{Kind: "overflow", Title: "compact before continuing with command output?", Items: []item{{"yes", "yes", "archive and compact"}, {"no", "no", "stop here; output remains saved"}}}
			return nil
		}
		return a.startChat()
	}
	return nil
}

func (a *App) showRun(r runner.Record, output string) {
	text := fmt.Sprintf("$ %s\n\ndirectory: %s\nstatus: %s · exit: %d\n%s\n\n%s", r.Command, r.Directory, r.Status, r.ExitCode, r.Error, output)
	if r.Truncated {
		text += "\n[output truncated at 8 mib]"
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
	records, err := a.history().List()
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
