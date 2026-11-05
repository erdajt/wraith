package ui

import (
	"context"
	"maps"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/provider"
)

type providerModelsMsg struct {
	Serial uint64
	Name   string
	Models []ollama.Model
	Err    error
}

func (a *App) providerMenu() tea.Cmd {
	var items []item
	for name, p := range a.Config.Providers {
		items = append(items, item{Label: name, Value: name, Detail: p.Kind})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	a.popup = &menu{Kind: "providers", Title: "choose provider", Items: items}
	return nil
}

func (a *App) selectProvider(name string) tea.Cmd {
	p, ok := a.Config.Providers[name]
	if !ok {
		a.notice = "unknown provider: " + name
		return nil
	}
	if p.Model != "" {
		return a.activateProvider(name, p.Model)
	}
	c := a.Config
	c.Provider = name
	client := provider.New(c)
	a.catalogSerial++
	serial := a.catalogSerial
	a.popup = &menu{Kind: "provider-loading", Title: "loading " + name + " models · esc closes"}
	a.notice = "loading models for " + name
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		models, err := client.Models(ctx)
		return providerModelsMsg{Serial: serial, Name: name, Models: models, Err: err}
	}
}

func (a *App) activateProvider(name, model string) tea.Cmd {
	if a.busy || a.demo {
		a.notice = "finish the current operation before switching provider"
		return nil
	}
	c := a.Config
	c.Provider, c.Model, c.Thinking = name, model, "auto"
	c.Providers = maps.Clone(c.Providers)
	p := c.Providers[name]
	p.Model = model
	c.Providers[name] = p
	if err := c.Validate(); err != nil {
		a.notice = err.Error()
		return nil
	}
	if err := config.Save(config.File(a.Paths.Config), c); err != nil {
		a.notice = err.Error()
		return nil
	}
	a.Config = c
	a.Session.Provider, a.Session.Model = name, model
	a.Client = provider.New(c)
	a.Info = ollama.Info{}
	a.persist("provider selected · " + name)
	return a.probe()
}

func (a *App) providerModels(m providerModelsMsg) {
	if a.busy || m.Serial != a.catalogSerial || a.popup == nil || a.popup.Kind != "provider-loading" {
		return
	}
	a.popup = nil
	if m.Err != nil {
		a.notice = "provider models: " + m.Err.Error()
		return
	}
	var items []item
	for _, model := range m.Models {
		items = append(items, item{Label: model.Name, Value: model.Name})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	a.popup = &menu{Kind: "provider-models", Prefix: m.Name, Title: m.Name + " · choose model", Items: items}
}
