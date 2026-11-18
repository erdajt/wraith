package ui

import tea "github.com/charmbracelet/bubbletea"

type closeRequest struct {
	All, Force, Save bool
	Window           uint64
}

// quit requests an application exit while retaining unsent input until the user decides.
func (a *App) quit() tea.Cmd { return a.closeWorkspace(closeRequest{All: true}) }

// closeWorkspace saves a consistent session before closing a view or shutting down.
func (a *App) closeWorkspace(request closeRequest) tea.Cmd {
	if a.Prompt.String() != "" && !request.Force && !request.Save {
		a.notice = "unsent draft · :w sends · :wq saves and quits · :q! closes and discards · :qa! quits and discards"
		return nil
	}

	exiting := request.All || len(a.windows) == 1
	candidate := a.Session.Clone()
	candidate.Draft = ""
	if request.Save {
		candidate.Draft = a.Prompt.String()
	}
	if exiting && a.busy && !a.compacting && (a.partial.Content != "" || a.partial.Thinking != "") {
		partial := a.partial
		partial.Interrupted = true
		candidate.Messages = append(candidate.Messages, partial)
	}
	if !a.demo {
		if err := a.Store.Save(candidate); err != nil {
			a.notice = "save failed; session remains open: " + err.Error()
			return nil
		}
	}
	a.Session = candidate
	if request.Force {
		a.Prompt.Set("")
	}
	if !exiting {
		if request.Window != 0 {
			a.focusWindow(request.Window)
		}
		a.windowAction("c")
		return nil
	}
	a.Close()
	return tea.Quit
}
