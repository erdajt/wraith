package ui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/plugin"
)

type pluginReloadMsg struct {
	Runtime      *plugin.Runtime
	Serial       uint64
	Owner, Draft string
	Digest       [32]byte
	Err          error
}

// LoadPlugin prepares and validates a candidate without replacing an active runtime.
func LoadPlugin(ctx context.Context, directory string, c config.Config, locations ...config.Paths) (*plugin.Runtime, error) {
	r, err := plugin.Open(ctx, directory, c, PluginActions(), locations...)
	if err != nil {
		return nil, err
	}
	if err := ValidatePlugin(r.Manifest()); err != nil {
		r.Close()
		return nil, err
	}
	return r, nil
}

// reloadPlugins loads yaml and lua on a worker while the active workspace remains usable.
func (a *App) reloadPlugins() tea.Cmd {
	if a.busy || a.loading || a.editing || !a.actionAllowed("command.reload") {
		a.notice = "finish the current operation before reloading init.lua"
		return nil
	}
	if a.pluginLoadCancel != nil {
		a.pluginLoadCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.pluginLoadCancel = cancel
	a.pluginSerial++
	m := pluginReloadMsg{Serial: a.pluginSerial, Owner: a.keyOwner(), Draft: a.Prompt.String(), Digest: configDigest(a.Config)}
	paths := a.Paths
	a.notice = "loading init.lua"
	return func() tea.Msg {
		c, err := config.LoadDirectory(paths.Config)
		if err == nil {
			m.Runtime, err = LoadPlugin(ctx, paths.Config, c, paths)
		}
		m.Err = err
		select {
		case <-a.done:
			if m.Runtime != nil {
				m.Runtime.Close()
			}
			cancel()
			return nil
		default:
			return m
		}
	}
}

// finishPluginReload atomically installs validated configuration and rejects stale candidates.
func (a *App) finishPluginReload(m pluginReloadMsg) tea.Cmd {
	installed := false
	defer func() {
		if !installed && m.Runtime != nil {
			m.Runtime.Close()
		}
	}()
	if m.Serial != a.pluginSerial {
		return nil
	}
	cancel := a.pluginLoadCancel
	a.pluginLoadCancel = nil
	defer func() {
		if !installed && cancel != nil {
			cancel()
		}
	}()
	if m.Err != nil {
		a.PluginError(m.Err)
		return nil
	}
	if m.Owner != a.keyOwner() || m.Draft != a.Prompt.String() || m.Digest != configDigest(a.Config) {
		a.notice = "lua reload discarded after workspace changed"
		return nil
	}
	manifest := m.Runtime.Manifest()
	nativeWindows := 0
	for _, view := range a.windows {
		if view.surface == nil {
			nativeWindows++
		}
	}
	surfaces, err := preparePluginSurfaces(m.Runtime, max(1, nativeWindows))
	if err != nil {
		a.PluginError(err)
		return nil
	}
	oldActions := a.actions
	a.actions = a.pluginActions(manifest)
	cmd, err := a.applyPluginConfig(manifest.Config, m.Runtime)
	if err != nil {
		a.actions = oldActions
		a.PluginError(err)
		return nil
	}
	old := a.plugins
	a.plugins, a.pluginManifest = m.Runtime, manifest
	installed = true
	if old != nil {
		old.Close()
	}
	a.installPluginSurfaces(surfaces)
	a.notice = "init.lua reloaded · pending key sequences cleared"
	if len(manifest.Diagnostics) > 0 {
		a.notice += " · lua: " + strings.ReplaceAll(clean(strings.Join(manifest.Diagnostics, " · ")), "\n", " ")
	}
	return tea.Batch(cmd, a.callPlugin("", "ready", ""), a.watchPluginNotices(a.plugins), a.startPluginResources(), a.watchPluginInvocations(a.plugins))
}
