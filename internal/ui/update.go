package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = m.Width
		a.height = m.Height
		a.resizePanes()
	case tick:
		a.frame++
		if a.Config.Animations {
			return a, a.ticker()
		}
		return a, tea.Tick(time.Second, func(t time.Time) tea.Msg { return tick(t) })
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

	case runMsg:
		return a, a.receiveRun(m)
	case streamMsg:
		return a, a.receive(m)
	case progressMsg:
		a.phase = string(m)
		return a, a.next()
	case compactMsg:
		return a, a.finishCompact(m)
	case editedMsg:
		return a, a.finishEdit(m)
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

func modelSize(size int64) string {
	if size == 0 {
		return ""
	}
	return fmt.Sprintf("%.1f gb", float64(size)/1e9)
}
