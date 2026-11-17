package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/provider"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/store"
	"github.com/erdajt/wraith/internal/vim"
)

func (a *App) submit() tea.Cmd {
	text := strings.TrimSpace(a.Prompt.String())
	if text == "" {
		return nil
	}
	if strings.HasPrefix(text, "/") {
		a.Prompt = vim.New()
		return a.runCommand(strings.TrimPrefix(text, "/"))
	}
	if name, _, assignment := strings.Cut(text, "="); assignment && !strings.ContainsAny(strings.TrimSpace(name), " \t\n") {
		if a.busy || a.demo {
			a.notice = "private variable changes are unavailable during an operation or demo"
			return nil
		}
		a.Prompt = vim.New()
		return a.environmentCommand("set "+text, false)
	}
	if a.loading {
		a.notice = "model is loading · draft retained"
		return nil
	}
	if a.busy {
		a.notice = "wait for the current operation or press " + a.Config.Keys["cancel"]
		return nil
	}
	if a.demo {
		a.notice = "demo mode does not send prompts"
		return nil
	}
	if err := a.refreshVariables(); err != nil {
		a.notice = err.Error()
		return nil
	}
	probe, _ := a.pendingEvidence()
	probe.Messages = append(probe.Messages, session.Message{Role: "user", Content: text})
	if err := contextual.Fits(probe, a.runtimeConfig(), a.chatInstructions()); err != nil {
		a.notice = err.Error()
		a.pendingPrompt = text
		if a.Config.Compact.Mode == "auto" {
			return a.startCompact()
		}
		a.popup = &menu{Kind: "overflow", Title: "compact before sending?", Items: []item{{"yes · compact and keep this draft", "yes", ""}, {"no · keep draft without sending", "no", ""}}}
		return nil
	}
	a.rememberInput(text)
	a.Prompt = vim.New()
	a.agentReads, a.pendingTool = 0, nil
	a.agentSteps = 0
	a.generation, a.agentStopped = session.ID(), false
	a.proposal = nil
	original := a.Session
	a.Session = probe
	if a.Session.Title == "untitled" {
		r := []rune(text)
		a.Session.Title = string(r[:min(len(r), 60)])
	}
	if err := a.Store.Save(a.Session); err != nil {
		a.Session = original
		a.Prompt.Set(text)
		a.notice = err.Error()
		return nil
	}
	return a.startChat()
}

func (a *App) startChat() tea.Cmd {
	if a.loading {
		a.notice = "model is loading"
		return nil
	}
	if a.demo {
		return nil
	}
	if err := a.refreshVariables(); err != nil {
		a.notice = err.Error()
		return nil
	}
	if a.Config.Execution.AutoShare {
		if err := a.syncTerminalEvidence(); err != nil {
			a.notice = err.Error()
			return nil
		}
	}
	think, err := a.Info.Think(a.Config.Thinking)
	if err != nil {
		a.notice = err.Error() + "; /retry after changing /think"
		return nil
	}
	if err = contextual.Fits(a.Session, a.runtimeConfig(), a.chatInstructions()); err != nil {
		a.notice = err.Error()
		a.pendingAgent = true
		if a.Config.Compact.Mode == "auto" {
			return a.startCompact()
		}
		a.popup = &menu{Kind: "overflow", Title: "compact before continuing?", Items: []item{{"yes", "yes", "archive and compact"}, {"no", "no", "keep context without sending"}}}
		return nil
	}
	timeout, _ := time.ParseDuration(a.Config.Timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	a.cancel = cancel
	a.busy = true
	a.compacting = false
	a.follow = true
	a.partial = session.Message{Role: "assistant"}
	a.phase = "connecting"
	req := ollama.Request{RunIDs: a.Session.RunIDs(), ExecutionMode: a.Config.Execution.Mode, PrivateInventory: true, Model: a.Session.Model, Messages: contextual.ForChat(a.Session, a.Config, a.Instructions), Think: think, KeepAlive: a.Config.KeepAlive, Options: ollama.Options{Context: a.contextLimit(), Predict: a.Config.Predict, Temperature: a.Config.Temperature, Threads: a.Config.Threads}}
	client := a.Client
	a.streamStopped = false
	a.streamSerial++
	serial := a.streamSerial
	go func() {
		defer cancel()
		r, e := client.ChatWithRepair(ctx, req, func(update provider.StreamUpdate) {
			select {
			case a.events <- streamMsg{Serial: serial, Chunk: update.Chunk, Retry: update.Retry, Problem: update.Problem, Rejected: update.Rejected}:
			case <-ctx.Done():
			case <-a.done:
			}
		})
		a.post(streamMsg{Serial: serial, Chunk: r, Final: true, Err: e})
	}()
	return a.next()
}

func (a *App) receive(m streamMsg) tea.Cmd {
	if m.Serial != a.streamSerial {
		return nil
	}
	if a.streamStopped {
		if !m.Final {
			return a.next()
		}
		m.Err = context.Canceled
	}
	if m.Retry > 0 {
		a.partial = session.Message{Role: "assistant"}
		a.phase = fmt.Sprintf("retrying %d/3 · %s", m.Retry, m.Problem)
		a.notice = a.phase
		if m.Rejected.Content != "" || m.Rejected.Thinking != "" {
			rejected := m.Rejected
			rejected.Interrupted = true
			attempt := session.Session{Version: 1, ID: a.Session.ID, Model: a.Session.Model, Provider: a.Session.Provider, Directory: a.Directory, Title: fmt.Sprintf("rejected response · retry %d · %s", m.Retry, m.Problem), Messages: []session.Message{rejected}}
			if _, err := (store.Store{Root: filepath.Join(a.Paths.Cache, "repairs")}).Archive(attempt); err != nil {
				a.notice += " · attempt cache: " + err.Error()
			}
		}
		return a.next()
	}

	if !m.Final {
		a.partial.Content += m.Chunk.Message.Content
		a.partial.Thinking += m.Chunk.Message.Thinking
		a.phase = "responding"
		if m.Chunk.Message.Thinking != "" {
			a.phase = "thinking"
		}
		return a.next()
	}
	a.busy = false
	a.cancel = nil
	if m.Err != nil {
		if m.Chunk.Message.Content != "" || m.Chunk.Message.Thinking != "" {
			a.partial = m.Chunk.Message
		}
		a.partial.Interrupted = true
		a.notice = m.Err.Error() + " · /retry"
	} else {
		a.partial = m.Chunk.Message
		a.notice = fmt.Sprintf("%.1f tokens/s · %d output tokens", m.Chunk.Usage(0).TokensPerSecond, m.Chunk.EvalCount)
		if m.Chunk.Reason == "length" {
			a.notice += " · output limit reached"
		}
	}
	a.partial.Content = contextual.RedactActions(a.partial.Content, a.Variables.Redact, a.Session.RunIDs())
	a.partial.Thinking = a.Variables.Redact(a.partial.Thinking)
	if a.partial.Content != "" || a.partial.Thinking != "" {
		a.Session.Messages = append(a.Session.Messages, a.partial)
		a.selected = len(a.Session.Messages) - 1
	}
	a.Session.Usage = m.Chunk.Usage(contextual.Estimate(contextual.ForChat(a.SessionWithoutLastAssistant(), a.Config, a.chatInstructions())))
	a.partial = session.Message{}
	if err := a.Store.Save(a.Session); err != nil {
		a.notice = "save failed: " + err.Error()
		return nil
	}
	if m.Err != nil {
		return nil
	}
	if a.Config.Execution.Mode == "agent" {
		tool, err := contextual.RequestedTool(m.Chunk.Message.Content)
		if err != nil {
			a.notice = "output request rejected: " + err.Error()
			return nil
		}
		if tool != nil {
			a.pendingTool = tool
			return a.runPendingTool()
		}
	}

	{
		request, err := contextual.RequestedCommand(m.Chunk.Message.Content)
		if err != nil {
			a.notice = "command request rejected: " + err.Error()
			return nil
		}
		if request != nil {
			if a.agentSteps >= a.Config.Execution.MaxSteps {
				a.agentStopped = true
				a.notice = "command step limit reached · send a prompt to continue"
				return nil
			}
			a.proposal = &proposal{Command: request.Command, Reason: request.Reason, Text: request.Command, Language: "sh", Source: "model", Continue: true, Background: request.Background}
		}
	}
	switch contextual.Decision(a.Config.Compact.Mode, a.Session.Declines, contextual.NeedsCompact(a.Session, a.runtimeConfig(), a.chatInstructions())) {
	case "compact":
		return a.startCompact()
	case "ask":
		a.popup = &menu{Kind: "compact", Title: fmt.Sprintf("context threshold reached · %d/3 declines", a.Session.Declines), Items: []item{{"yes · archive and compact", "yes", ""}, {"no · ask after the next response", "no", ""}}}
	}
	if a.popup == nil && a.proposal != nil {
		return a.approve(*a.proposal)
	}
	return nil
}

func (a *App) SessionWithoutLastAssistant() session.Session {
	s := a.Session.Clone()
	if n := len(s.Messages); n > 0 && s.Messages[n-1].Role == "assistant" {
		s.Messages = s.Messages[:n-1]
	}
	return s
}

func (a *App) next() tea.Cmd {
	return func() tea.Msg {
		select {
		case m := <-a.events:
			return m
		case <-a.done:
			return nil
		}
	}
}
func (a *App) post(m tea.Msg) {
	select {
	case a.events <- m:
	case <-a.done:
	}
}
