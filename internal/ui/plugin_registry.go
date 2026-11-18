package ui

import "slices"

// installPluginRegistry validates live bindings before acknowledging the VM's pending revision.
func (a *App) installPluginRegistry(m pluginResultMsg) error {
	manifest := *m.Result.Manifest
	if err := ValidatePlugin(manifest); err != nil {
		return err
	}

	previousActions := a.actions
	a.actions = a.pluginActions(manifest)
	candidate := a.Config
	candidate.Keymaps = slices.Clone(manifest.Config.Keymaps)
	registry, err := a.buildKeymaps(candidate)
	if err != nil {
		a.actions = previousActions
		return err
	}

	previousConfig, previousManifest, previousRegistry := a.Config, a.pluginManifest, a.keymaps
	a.Config, a.pluginManifest, a.keymaps = candidate, manifest, registry
	if err := m.Runtime.Acknowledge(m.Result.Revision, true); err != nil {
		a.Config, a.pluginManifest, a.keymaps, a.actions = previousConfig, previousManifest, previousRegistry, previousActions
		return err
	}
	a.keyPending = nil
	return nil
}
