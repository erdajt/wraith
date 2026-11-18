package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/provider"
)

// contextLimit resolves the selected model's effective context capacity.
func (a *App) contextLimit() int {
	return a.Config.ContextFor(a.Session.Model, a.Info.ContextWindow)
}

// runtimeConfig copies configuration with the selected model's effective context capacity.
func (a *App) runtimeConfig() config.Config {
	c := a.Config
	c.Context = a.contextLimit()
	return c
}

// setContext validates and saves a model-specific capacity before reloading the model.
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
	if err := a.saveConfig(c); err != nil {
		a.notice = "could not save context: " + err.Error()
		return nil
	}
	a.Config = c
	a.Client = provider.New(c)
	a.notice = "saved context = " + value + " for " + c.Provider + " / " + a.Session.Model
	return a.probe()
}
