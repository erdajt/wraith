package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/provider"
	"github.com/erdajt/wraith/internal/session"
)

func (a *App) resume(id string) tea.Cmd {
	s, err := a.Store.LoadDirectory(id, a.Directory)
	if err != nil {
		cmd := a.sessionCommand("new", "")
		if err.Error() == "invalid session id" {
			a.InvalidSession(id)
		} else {
			a.SessionError(err)
		}
		return cmd
	}
	if s.Provider == "" {
		s.Provider = "ollama"
	}
	if _, ok := a.Config.Providers[s.Provider]; !ok {
		a.notice = "session provider is not configured: " + s.Provider
		return nil
	}
	a.Config.Provider = s.Provider
	a.Client = provider.New(a.Config)
	a.Session = s
	a.resetChatNavigation()
	a.agentSteps = 0
	a.pendingAgent = false
	a.proposal = nil
	a.Config.Model = s.Model
	a.Info = ollama.Info{}
	a.Config.Thinking = "auto"
	a.follow = true
	a.selected = max(0, len(s.Messages)-1)
	a.pendingPrompt = ""
	a.notice = "resumed " + s.Title
	return a.probe()
}

func (a *App) restore(name string) tea.Cmd {
	s, err := a.Store.ReadArchive(name)
	if err != nil {
		a.notice = err.Error()
		return nil
	}
	s.ID = session.ID()
	s.Title = "restored · " + s.Title
	s.Created = time.Now().UTC()
	s.Archives = append(s.Archives, name)
	if err = a.Store.Save(s); err != nil {
		a.notice = err.Error()
		return nil
	}
	return a.resume(s.ID)
}

func Export(s session.Session) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\nmodel: %s\n\nsession: %s\n", s.Title, s.Model, s.ID)
	if s.Summary != "" {
		fmt.Fprintf(&b, "\n## compacted state\n\n%s\n", s.Summary)
	}
	for _, group := range []struct {
		name   string
		values []string
	}{{"pins", s.Pins}, {"facts", s.Facts}, {"archives", s.Archives}} {
		if len(group.values) > 0 {
			fmt.Fprintf(&b, "\n## %s\n\n", group.name)
			for _, v := range group.values {
				fmt.Fprintf(&b, "- %s\n", v)
			}
		}
	}
	for _, m := range s.Messages {
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", m.Role, contextual.DisplayContent(m.Content))
		if m.Edited {
			b.WriteString("\n*edited locally*\n")
		}
		if m.Interrupted {
			b.WriteString("\n*interrupted*\n")
		}
	}
	return b.String()
}

func (a *App) OpenSessions() { a.sessionCommand("sessions", "") }

func (a *App) InvalidSession(id string) {
	a.notice = "invalid session id · started a new chat"
	a.showText("help", "invalid session id", id+"\n\nNo saved session could be loaded for this id. A new chat is open. Use /resume to choose from saved sessions.")
}

func (a *App) SessionError(err error) {
	a.notice = err.Error() + " · started a new chat"
	a.showText("help", "cannot resume session", err.Error()+"\n\nA new chat is open in the current directory.")
}

func (a *App) resetChatNavigation() {
	a.chatCursor, a.chatColumn = -1, 0
	a.visualMode, a.chatMotionPending, a.chatMotionCount, a.search = "", "", "", ""
	a.thinkingExpanded = map[string]bool{}
	a.follow = true
}
