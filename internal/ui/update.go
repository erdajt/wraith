package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// update dispatches ui, provider and terminal events to their state handlers.
func (a *App) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case pluginResourceMsg:
		return a, a.callPluginResource(m)
	case pluginResourceResultMsg:
		cmd := a.applyPluginResult(m.Result)
		if m.Result.Runtime != a.plugins {
			return a, cmd
		}
		return a, tea.Batch(cmd, a.watchPluginResources(m.Result.Runtime))
	case pluginScheduledMsg:
		return a, a.runPluginSchedule(m)
	case pluginNoticeMsg:
		return a, a.applyPluginNotice(m)
	case pluginReloadMsg:
		return a, a.finishPluginReload(m)
	case pluginResultMsg:
		return a, a.applyPluginResult(m)
	case queuedKeysMsg:
		return a, a.dispatchKeys(m)
	case keymapTimeoutMsg:
		return a, a.keymapTimeout(uint64(m))
	case tea.WindowSizeMsg:
		a.width = m.Width
		a.height = m.Height
		a.resizePanes()
	case tick:
		a.frame++
		jobs := a.syncJobs()
		if a.Config.Animations {
			return a, tea.Batch(a.ticker(), jobs)
		}
		return a, tea.Batch(jobs, tea.Tick(time.Second, func(t time.Time) tea.Msg { return tick(t) }))
	case infoMsg:
		if m.Model == a.Session.Model && m.Serial == a.loadSerial {
			a.loading = false
			a.loadCancel = nil
			a.Info = m.Info
			a.connected = m.Err == nil
			if m.Err != nil {
				a.notice = m.Err.Error()
			} else {
				a.notice = "model loaded · ready"
			}
		}
	case copiedMsg:
		if m.Err != nil {
			a.notice = "copied to wraith register; system clipboard: " + m.Err.Error()
		} else {
			a.notice = "copied to system clipboard and wraith register"
		}
	case tea.MouseMsg:
		return a, a.mouse(m)
	case tea.KeyMsg:
		return a, a.key(m)
	case restartTerminalMsg:
		return a, a.restartExitedTerminal(m)
	case pasteMsg:
		return a, a.pasted(m)
	case pasteTimeoutMsg:
		p := a.panes[m.Index]
		if p != nil && p.PTY == m.PTY && p.PasteID == m.ID {
			p.PTY.CancelPaste(p.PasteID)
			p.PasteCancelled = true
			a.notice = "shell did not acknowledge insertion · command retained; ctrl+c interrupts, /terminal-restart resets"
		}
		return a, nil
	case terminalMsg:
		p := a.panes[m.Index]
		if p == nil || p.PTY != m.PTY {
			break
		}
		if !m.Open || m.Event.Err != nil {
			return a, a.terminalExited(m)
		}
		if reply := p.Screen.Feed(m.Event.Data); len(reply) > 0 {
			_ = p.PTY.Write(reply)
		}
		return a, a.readTerminal(m.Index)

	case streamMsg:
		return a, a.receive(m)
	case progressMsg:
		a.phase = string(m)
		return a, a.next()
	case compactMsg:
		return a, a.finishCompact(m)
	case editedMsg:
		if !a.editing || a.editorFile != m.File {
			return a, nil
		}
		a.editorFile = nil
		pending := a.editorDeferred
		a.editorDeferred = nil
		for _, message := range pending {
			if effect, ok := message.(pluginResultMsg); ok {
				if !a.pluginOwns(effect) {
					var resumes []tea.Cmd
					for _, queued := range pending {
						if callback, ok := queued.(pluginResultMsg); ok {
							discardPluginSchedules(callback)
							discardPluginResources(callback)
							if next := remainingPluginSchedules(callback); len(next.IDs) > 0 {
								resumes = append(resumes, func() tea.Msg { return next })
							}
						}
					}
					a.editing = false
					a.notice = "editor result discarded after workspace changed · edit retained at " + m.File.Path
					return a, tea.Batch(resumes...)
				}
			}
		}
		cmd := a.finishEdit(m)
		cmds := []tea.Cmd{cmd}
		for _, message := range pending {
			if effect, ok := message.(pluginResultMsg); ok {
				if !a.editorSucceeded {
					discardPluginSchedules(effect)
					discardPluginResources(effect)
					if next := remainingPluginSchedules(effect); len(next.IDs) > 0 {
						cmds = append(cmds, func() tea.Msg { return next })
					}
					continue
				}
				effect.Owner, effect.Draft, effect.Digest, effect.Target = a.keyOwner(), a.Prompt.String(), configDigest(a.Config), a.pluginTarget()
				message = effect
			}
			cmds = append(cmds, func() tea.Msg { return message })
		}
		return a, tea.Sequence(cmds...)
	case providerModelsMsg:
		a.providerModels(m)
	case modelsMsg:
		if m.Serial != a.catalogSerial || a.popup == nil || a.popup.Kind != "models-loading" {
			break
		}
		a.popup = nil
		if a.busy {
			a.notice = "model list dismissed while a response is running"
			break
		}
		if m.Err != nil {
			a.notice = m.Err.Error()
			break
		}

		items := make([]item, 0, len(m.Models))
		for _, model := range m.Models {
			items = append(items, item{model.Name, model.Name, modelSize(model.Size)})
		}
		a.popup = &menu{Kind: "models", Title: a.Config.Provider + " models", Items: items}
		if len(items) == 0 {
			a.notice = "no models found; configure a model catalog or choose a model id"
		}
	}
	return a, nil
}

// modelSize formats a known model size in decimal gigabytes, leaving unknown sizes blank.
func modelSize(size int64) string {
	if size == 0 {
		return ""
	}
	return fmt.Sprintf("%.1f gb", float64(size)/1e9)
}
