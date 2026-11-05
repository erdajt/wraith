package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/provider"
)

func (a *App) contextLimit() int {
	return a.Config.ContextFor(a.Session.Model, a.Info.ContextWindow)
}

func (a *App) runtimeConfig() config.Config {
	c := a.Config
	c.Context = a.contextLimit()
	return c
}

func (a *App) setContext(value string) tea.Cmd {
	value = strings.TrimSpace(value)
	capacity := 0
	if value != "auto" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			a.notice = "context must be a positive integer or auto"
			return nil
		}
		capacity = n
	}
	c := a.Config.WithModelContext(a.Session.Model, capacity)
	if err := config.Save(config.File(a.Paths.Config), c); err != nil {
		a.notice = "could not save context: " + err.Error()
		return nil
	}
	a.Config = c
	a.Client = provider.New(c)
	a.notice = "saved context = " + value + " for " + c.Provider + " / " + a.Session.Model
	return a.probe()
}
