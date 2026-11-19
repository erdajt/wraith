package ui

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

type commandPolicyMsg struct {
	Runtime  *plugin.Runtime
	Serial   uint64
	Hooks    [32]byte
	Phase    string
	Execute  bool
	Before   plugin.CommandData
	Proposal proposal
	Result   plugin.HookResult
	Err      error
}

// commandTuple detaches every execution-affecting field for validation and approval comparison.
func commandTuple(p proposal) plugin.CommandData {
	return plugin.CommandData{
		Command: p.Resolved, Directory: p.launch.Directory, Shell: p.launch.Shell,
		Environment: slices.Clone(p.launch.Environment), Timeout: p.timeout.String(), Background: p.Background,
	}
}

// checkCommandPolicy validates on a worker while keeping the proposal cancellable in the ui.
func (a *App) checkCommandPolicy(p proposal, phase string, execute bool) tea.Cmd {
	if a.plugins == nil || !slices.ContainsFunc(a.pluginManifest.Hooks, func(hook plugin.HookDefinition) bool { return hook.Kind == "before_command" }) {
		if execute {
			return a.launchApproved(p)
		}
		return a.showApproval(p)
	}
	if a.policyCancel != nil {
		a.policyCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.policyCancel = cancel
	a.policySerial++
	hooks, _ := json.Marshal(a.pluginManifest.Hooks)
	m := commandPolicyMsg{Runtime: a.plugins, Serial: a.policySerial, Hooks: sha256.Sum256(hooks), Phase: phase, Execute: execute, Before: commandTuple(p), Proposal: p}
	a.proposal = &p
	a.popup = &menu{Kind: "command-policy", Title: "checking command policy", Hints: "esc rejects this command"}
	input := plugin.HookRequest{Kind: "before_command", Phase: phase, ID: p.launch.ID, SessionID: p.launch.Session, Directory: a.Directory, Command: &m.Before}
	return func() tea.Msg {
		defer cancel()
		m.Result, m.Err = m.Runtime.Transform(ctx, input)
		return m
	}
}

// finishCommandPolicy rejects stale checks and requests approval again when a hook changes the tuple.
func (a *App) finishCommandPolicy(m commandPolicyMsg) tea.Cmd {
	if m.Runtime != a.plugins || m.Serial != a.policySerial || a.proposal == nil || a.proposal.launch.ID != m.Proposal.launch.ID || m.Proposal.launch.Session != a.Session.ID || m.Proposal.launch.Generation != a.generation || a.popup == nil || a.popup.Kind != "command-policy" {
		return nil
	}
	a.policyCancel = nil
	hooks, _ := json.Marshal(a.pluginManifest.Hooks)
	if sha256.Sum256(hooks) != m.Hooks {
		return a.checkCommandPolicy(m.Proposal, m.Phase, m.Execute)
	}
	if m.Err == nil && m.Result.Decision == "deny" {
		m.Err = errors.New("denied: " + m.Result.Reason)
	}
	if m.Err == nil && m.Result.Command == nil {
		m.Err = errors.New("policy returned no command")
	}
	if m.Err == nil {
		m.Err = validateCommandTuple(*m.Result.Command)
	}
	if m.Err != nil {
		a.declineCommand()
		a.popup = nil
		a.notice = "command policy: " + m.Err.Error()
		return nil
	}

	p, tuple := m.Proposal, *m.Result.Command
	changed := !reflect.DeepEqual(tuple, m.Before)
	if changed {
		for _, field := range []struct {
			name          string
			before, after any
		}{
			{"command", m.Before.Command, tuple.Command}, {"directory", m.Before.Directory, tuple.Directory},
			{"shell", m.Before.Shell, tuple.Shell}, {"environment", m.Before.Environment, tuple.Environment},
			{"timeout", m.Before.Timeout, tuple.Timeout}, {"background", m.Before.Background, tuple.Background},
		} {
			if !reflect.DeepEqual(field.before, field.after) && !slices.Contains(p.policyChanges, field.name) {
				p.policyChanges = append(p.policyChanges, field.name)
			}
		}
		p.Resolved, p.launch.Command = tuple.Command, tuple.Command
		p.launch.Directory, p.launch.Shell, p.launch.Environment = tuple.Directory, tuple.Shell, slices.Clone(tuple.Environment)
		p.Background, p.launch.Background = tuple.Background, tuple.Background
		p.timeout, _ = time.ParseDuration(tuple.Timeout)
	}
	if !m.Execute || changed {
		a.notice = "command policy checked"
		if changed {
			a.notice = "policy changed " + strings.Join(p.policyChanges, ", ") + " · review before running"
		}
		return a.showApproval(p)
	}
	if m.Phase == "preview" {
		return a.checkCommandPolicy(p, "execute", true)
	}
	return a.launchApproved(p)
}

// validateCommandTuple rejects invalid process arguments before an approval can be presented.
func validateCommandTuple(tuple plugin.CommandData) error {
	if strings.TrimSpace(tuple.Command) == "" || strings.ContainsAny(tuple.Command, "\x00\x1b\r") || strings.TrimSpace(tuple.Shell) == "" || strings.ContainsRune(tuple.Shell, 0) || !filepath.IsAbs(tuple.Directory) || strings.ContainsRune(tuple.Directory, 0) {
		return errors.New("policy returned an invalid command, shell or directory")
	}
	if timeout, err := time.ParseDuration(tuple.Timeout); err != nil || timeout <= 0 {
		return errors.New("policy timeout is outside the supported duration range")
	}
	for _, entry := range tuple.Environment {
		name, _, found := strings.Cut(entry, "=")
		if !found || name == "" || strings.ContainsRune(entry, 0) {
			return errors.New("policy returned an invalid environment entry")
		}
	}
	return nil
}
