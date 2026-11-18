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
	Parent       *pluginResultMsg
	Runtime      *plugin.Runtime
	Owner, Draft string
	Digest       [32]byte
	Target       [32]byte
	Depth        int
	Result       plugin.Result
}

// configDigest invalidates pending callback edits after settings change.
func configDigest(c config.Config) [32]byte {
	data, _ := json.Marshal(c)
	return sha256.Sum256(data)
}

// callPlugin captures bounded workspace state and queues work without blocking rendering.
func (a *App) callPlugin(callback, event, argument string) tea.Cmd {
	if a.plugins == nil || event != "" && !slices.Contains(a.pluginManifest.Events, event) {
		return nil
	}
	if a.pluginDepth >= 8 {
		a.notice = "plugin callback chain limit reached"
		return nil
	}
	var windows []map[string]any
	ids := make([]uint64, 0, len(a.windows))
	for id := range a.windows {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	places := a.windowPlaces()
	for _, id := range ids {
		view := a.windows[id]
		entry := map[string]any{"id": id, "focused": id == a.windowID, "cursor": view.chatCursor, "column": view.chatColumn, "search": view.search, "offset": view.offset, "visible": false}
		for _, place := range places {
			if place.id == id {
				entry["width"], entry["height"], entry["visible"] = place.width, place.height, true
			}
		}
		windows = append(windows, entry)
	}
	messages := []map[string]string{}
	budget := 128 << 10
	for i := len(a.Session.Messages) - 1; i >= 0 && len(messages) < 64 && budget > 0; i-- {
		message := a.Session.Messages[i]
		content := message.Content[:min(len(message.Content), budget)]
		budget -= len(content)
		messages = append(messages, map[string]string{"role": message.Role, "content": strings.ToValidUTF8(content, "")})
	}
	slices.Reverse(messages)
	snapshot := map[string]any{"argument": argument, "event": event, "prompt": a.Prompt.String(), "input": a.inputSnapshot(), "windows": windows, "session": map[string]any{"id": a.Session.ID, "model": a.Session.Model, "provider": a.Config.Provider, "directory": a.Directory, "messages": messages, "message_count": len(a.Session.Messages)}}
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
func (a *App) applyPluginResult(m pluginResultMsg) tea.Cmd {
	if m.Runtime != a.plugins || m.Parent == nil && m.Result.Err == nil && len(m.Result.Effects) == 0 {
		return nil
	}
	if !a.pluginOwns(m) {
		a.notice = "lua callback discarded after workspace changed"
		return nil
	}
	if m.Result.Err != nil {
		a.notice = fmt.Sprintf("lua: %v", m.Result.Err)
		return nil
	}
	for i, effect := range m.Result.Effects {
		if !a.actionAllowed(effect.Name) {
			return nil
		}
		var cmd tea.Cmd
		switch effect.Name {
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
		if effect.Name == "editor.open" && !a.editing {
			return nil
		}
		callback := slices.Contains(a.pluginManifest.Callbacks, effect.Name)
		for _, command := range a.pluginManifest.Commands {
			callback = callback || effect.Name == "command."+command.Name
		}
		if callback {
			if cmd == nil {
				return nil
			}
			m.Result.Effects = m.Result.Effects[i+1:]
			return func() tea.Msg {
				result := cmd()
				if child, ok := result.(pluginResultMsg); ok {
					child.Parent = &m
					return child
				}
				return result
			}
		}
		if cmd != nil {
			if i+1 < len(m.Result.Effects) || m.Parent != nil {
				m.Result.Effects = m.Result.Effects[i+1:]
				m.Owner, m.Draft, m.Digest, m.Target = a.keyOwner(), a.Prompt.String(), configDigest(a.Config), a.pluginTarget()
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
		next.Owner, next.Draft, next.Digest, next.Target = a.keyOwner(), a.Prompt.String(), configDigest(a.Config), a.pluginTarget()
		return func() tea.Msg { return next }
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
