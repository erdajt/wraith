package ui

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/plugin"
)

type pluginResultMsg struct {
	Parent           *pluginResultMsg
	Scheduled        []string
	PendingSchedules []string
	Runtime          *plugin.Runtime
	Resource         *plugin.ResourceEvent
	ResumeOwned      bool
	Keys             [32]byte
	Owner, Draft     string
	Digest           [32]byte
	Target           [32]byte
	Depth            int
	Result           plugin.Result
}

// configDigest invalidates pending callback edits after settings change.
func configDigest(c config.Config) [32]byte {
	data, _ := json.Marshal(c)
	return sha256.Sum256(data)
}

// keymapDigest isolates registry conflicts from unrelated settings and input changes.
func keymapDigest(c config.Config) [32]byte {
	data, _ := json.Marshal(c.Keymaps)
	return sha256.Sum256(data)
}

// callPlugin captures bounded workspace state and queues work without blocking rendering.
func (a *App) callPlugin(callback, event, argument string) tea.Cmd {
	return a.callPluginData(callback, event, argument, nil)
}

// callPluginData includes detached event data while retaining the callback's input ownership.
func (a *App) callPluginData(callback, event, argument string, data any) tea.Cmd {
	if a.plugins == nil || event != "" && !slices.Contains(a.pluginManifest.Events, event) {
		return nil
	}
	if a.pluginDepth >= 8 {
		a.notice = "plugin callback chain limit reached"
		return nil
	}
	snapshot := a.pluginSnapshot(argument, event, data)
	m := pluginResultMsg{Runtime: a.plugins, Owner: a.keyOwner(), Draft: a.Prompt.String(), Digest: configDigest(a.Config), Target: a.pluginTarget(), Depth: a.pluginDepth + 1}
	result, err := a.plugins.Submit(callback, event, snapshot, a.Config)
	if err != nil {
		a.notice = "lua: " + err.Error()
		return nil
	}
	return func() tea.Msg {
		select {
		case m.Result = <-result:
			return m
		case <-m.Runtime.Done():
			return nil
		case <-a.done:
			return nil
		}
	}
}

// applyPluginResult rejects stale targets and routes effects through ordinary workspace guards.
func (a *App) applyPluginResult(m pluginResultMsg) (command tea.Cmd) {
	transferred := false
	m.Result.Effects = slices.Clone(m.Result.Effects)
	defer func() {
		if m.Result.Revision != 0 {
			_ = m.Runtime.Acknowledge(m.Result.Revision, false)
		}
		if !transferred {
			discardPluginSchedules(m)
			discardPluginResources(m)
			if next := remainingPluginSchedules(m); len(next.IDs) > 0 {
				command = tea.Sequence(command, func() tea.Msg { return next })
			}
		}
	}()
	if m.Runtime != a.plugins || m.Parent == nil && m.Result.Err == nil && len(m.Result.Effects) == 0 && len(m.Scheduled) == 0 && len(m.PendingSchedules) == 0 && m.Result.Manifest == nil {
		return nil
	}
	if m.Resource == nil && !a.pluginOwns(m) || m.Resource != nil && !m.Runtime.ResourceValid(*m.Resource) {
		a.notice = "lua callback discarded after workspace changed"
		return nil
	}
	if m.Result.Err != nil {
		a.notice = fmt.Sprintf("lua: %v", m.Result.Err)
		return nil
	}
	if m.Result.Manifest != nil {
		if m.Resource != nil && m.Keys != keymapDigest(a.Config) {
			a.notice = "lua registrations discarded after keymaps changed"
			return nil
		}
		owned := a.pluginOwns(m)
		if err := a.installPluginRegistry(m); err != nil {
			a.notice = "lua registrations unchanged: " + err.Error()
			return nil
		}
		m.Result.Manifest, m.Result.Revision = nil, 0
		if owned {
			m.Digest = configDigest(a.Config)
		}
		m.Keys = keymapDigest(a.Config)
	}
	for i, effect := range m.Result.Effects {
		owned := a.pluginOwns(m)
		if m.Resource != nil && !owned {
			switch effect.Name {
			case "notify", "event.emit", "schedule.callback", "resource.activate", "resource.release":
			default:
				a.notice = "lua input effect discarded after workspace changed"
				continue
			}
		}
		if effect.Name == "resource.activate" {
			if err := m.Runtime.Activate([]string{effect.Argument}); err != nil {
				a.notice = "lua service: " + err.Error()
				return nil
			}
			m.Result.Effects[i].Name = ""
			continue
		}
		if effect.Name == "resource.release" {
			m.Runtime.DiscardResources([]string{effect.Argument})
			continue
		}
		if effect.Name == "schedule.callback" {
			m.Scheduled = append(m.Scheduled, effect.Argument)
			continue
		}
		if !a.actionAllowed(effect.Name) {
			return nil
		}
		var cmd tea.Cmd
		switch effect.Name {
		case "event.emit":
			cmd = a.callPluginData("", effect.Argument, "", effect.Data)
		case "notify":
			a.notice = strings.Map(func(r rune) rune {
				if r < 32 || r == 127 {
					return ' '
				}
				return r
			}, ansi.Strip(effect.Argument))
		case "config":
			var err error
			cmd, err = a.applyPluginConfig(*effect.Config)
			if err != nil {
				a.notice = "lua config unchanged: " + err.Error()
				return nil
			}
		default:
			cmd = a.runAction(effect.Name, effect.Argument)
		}
		if m.Resource != nil && owned {
			m.Owner, m.Draft, m.Digest, m.Target = a.keyOwner(), a.Prompt.String(), configDigest(a.Config), a.pluginTarget()
		}
		if effect.Name == "editor.open" && !a.editing {
			return nil
		}
		callback := effect.Name == "event.emit" && cmd != nil || slices.Contains(a.pluginManifest.Callbacks, effect.Name)
		for _, command := range a.pluginManifest.Commands {
			callback = callback || effect.Name == "command."+command.Name
		}
		if callback {
			if cmd == nil {
				return nil
			}
			m.Result.Effects = m.Result.Effects[i+1:]
			m.ResumeOwned = owned
			keys := keymapDigest(a.Config)
			transferred = true
			return func() tea.Msg {
				result := cmd()
				if child, ok := result.(pluginResultMsg); ok {
					child.Parent = &m
					if m.Resource != nil {
						child.Resource, child.Keys = m.Resource, keys
					}
					return child
				}
				discardPluginSchedules(m)
				discardPluginResources(m)
				if result == nil {
					return remainingPluginSchedules(m)
				}
				return result
			}
		}
		if cmd != nil {
			if i+1 < len(m.Result.Effects) || m.Parent != nil || len(m.Scheduled) > 0 || len(m.PendingSchedules) > 0 {
				m.Result.Effects = m.Result.Effects[i+1:]
				if m.Resource == nil || owned {
					m.Owner, m.Draft, m.Digest, m.Target = a.keyOwner(), a.Prompt.String(), configDigest(a.Config), a.pluginTarget()
				}
				transferred = true
				if a.editing {
					a.editorDeferred = append(a.editorDeferred, m)
					return cmd
				}
				return tea.Sequence(cmd, func() tea.Msg { return m })
			}
			return cmd
		}
	}
	if m.Parent != nil {
		next := *m.Parent
		next.Scheduled = append(slices.Clone(next.Scheduled), m.Scheduled...)
		if next.Resource == nil || next.ResumeOwned && a.pluginOwns(m) {
			next.Owner, next.Draft, next.Digest, next.Target = a.keyOwner(), a.Prompt.String(), configDigest(a.Config), a.pluginTarget()
		}
		transferred = true
		return func() tea.Msg { return next }
	}
	transferred = true
	scheduled := append(slices.Clone(m.PendingSchedules), m.Scheduled...)
	if len(scheduled) > 0 {
		return func() tea.Msg { return pluginScheduledMsg{Runtime: m.Runtime, IDs: scheduled} }
	}
	return nil
}

// pluginOwns checks whether a callback still refers to the current workspace state.
func (a *App) pluginOwns(m pluginResultMsg) bool {
	return m.Runtime == a.plugins && m.Owner == a.keyOwner() && m.Draft == a.Prompt.String() && m.Digest == configDigest(a.Config) && m.Target == a.pluginTarget()
}

// pluginTarget binds deferred operations to the selected message and navigation state.
func (a *App) pluginTarget() [32]byte {
	var message any
	if a.selected >= 0 && a.selected < len(a.Session.Messages) {
		message = a.Session.Messages[a.selected]
	}
	var readerDigest [32]byte
	if reader := a.inputReader(); reader != nil {
		readerDigest = sha256.Sum256([]byte(reader.Buffer.String()))
	}
	data, _ := json.Marshal([]any{fmt.Sprintf("%p", a.inputReader()), readerDigest, a.selected, a.chatCursor, a.chatColumn, a.visualMode, a.inputMode(), a.inputSnapshot(), message})
	return sha256.Sum256(data)
}
