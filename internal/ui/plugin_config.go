package ui

import (
	"errors"
	"reflect"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/plugin"
)

// applyPluginConfig validates live changes before updating session identity and runtime services.
func (a *App) applyPluginConfig(c config.Config, runtimes ...*plugin.Runtime) (tea.Cmd, error) {
	if a.busy || a.loading || a.editing || a.popup != nil || a.proposal != nil || a.pluginDialog != nil {
		return nil, errors.New("finish the current operation before applying lua configuration")
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	registry, err := a.buildKeymaps(c)
	if err != nil {
		return nil, err
	}
	before := a.Config
	identityChanged := c.Provider != a.Session.Provider || c.Model != a.Session.Model
	if identityChanged {
		next := a.Session.Clone()
		next.Provider, next.Model = c.Provider, c.Model
		if !a.demo {
			if err := a.Store.Save(next); err != nil {
				return nil, err
			}
		}
		a.Session = next
	}
	a.Config, a.keymaps, a.keyPending = c, registry, nil
	a.Client = a.providerClient(c, runtimes...)
	a.Client.Variables = a.Variables
	a.jobs.SetLimit(c.Execution.MaxJobs)
	a.showThinking = c.Appearance.Thinking
	if before.Appearance.Thinking != c.Appearance.Thinking {
		a.thinkingExpanded = map[string]bool{}
	}
	a.resizePanes()
	var cmds []tea.Cmd
	if before.Appearance.Mouse != c.Appearance.Mouse {
		if c.Appearance.Mouse {
			cmds = append(cmds, tea.EnableMouseCellMotion)
		} else {
			cmds = append(cmds, tea.DisableMouse)
		}
	}
	if !a.demo && (identityChanged || before.Host != c.Host || before.Context != c.Context || before.Threads != c.Threads || !reflect.DeepEqual(before.Providers, c.Providers)) {
		cmds = append(cmds, a.probe())
	}
	return tea.Batch(cmds...), nil
}
