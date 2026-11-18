package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

// PluginActions lists host actions available to configuration without executing them.
func PluginActions() []string {
	catalog := (&App{}).builtinActions()
	names := make([]string, 0, len(catalog))
	for name := range catalog {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// ValidatePlugin checks a complete candidate against the same registry used by the workspace.
func ValidatePlugin(m plugin.Manifest) error {
	if err := m.Config.Validate(); err != nil {
		return err
	}
	a := &App{}
	a.actions = a.pluginActions(m)
	_, err := a.buildKeymaps(m.Config)
	return err
}

// pluginActions adds generation-local callbacks and commands to the host action table.
func (a *App) pluginActions(m plugin.Manifest) map[string]action {
	actions := a.builtinActions()
	for _, id := range m.Callbacks {
		actions[id] = action{"lua callback", func(arg string) tea.Cmd { return a.callPlugin(id, "", arg) }}
	}
	for _, command := range m.Commands {
		actions["command."+command.Name] = action{command.Description, func(arg string) tea.Cmd { return a.callPlugin(command.Callback, "", arg) }}
	}
	return actions
}

// pluginCommands merges user commands while preserving dotted-child completion rules.
func (a *App) pluginCommands(query string) []commandSpec {
	matches := matchingCommands(query)
	parent, _, dotted := strings.Cut(query, ".")
	for _, command := range a.pluginManifest.Commands {
		name := command.Name
		if !dotted {
			name, _, _ = strings.Cut(name, ".")
		} else if !strings.HasPrefix(name, parent+".") {
			continue
		}
		if strings.HasPrefix(name, query) {
			index := slices.IndexFunc(matches, func(spec commandSpec) bool { return spec.Name == name })
			choice := commandSpec{name, command.Description, command.NeedsArgument}
			if index < 0 {
				matches = append(matches, choice)
			} else if name == command.Name {
				matches[index] = choice
			}
		}
	}
	slices.SortStableFunc(matches, func(a, b commandSpec) int {
		if a.Name == query {
			return -1
		}
		if b.Name == query {
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return matches
}

// PluginError reports a failed startup candidate while keeping yaml configuration active.
func (a *App) PluginError(err error) { a.notice = fmt.Sprintf("init.lua unchanged: %v", err) }
