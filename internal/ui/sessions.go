package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
	"strings"
	"time"
)

func (a *App) resume(id string) tea.Cmd {
	s, err := a.Store.Load(id)
	if err != nil {
		a.notice = err.Error()
		return nil
	}
	a.Session = s
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
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", m.Role, m.Content)
		if m.Edited {
			b.WriteString("\n*edited locally*\n")
		}
		if m.Interrupted {
			b.WriteString("\n*interrupted*\n")
		}
	}
	return b.String()
}
