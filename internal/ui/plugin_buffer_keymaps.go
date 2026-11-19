package ui

import "github.com/erdajt/wraith/internal/keymap"

type surfaceKeymapCache struct {
	base     *keymap.Registry
	revision uint64
	registry *keymap.Registry
}

// lookupSurfaceKeys overlays cached local mappings for native and plugin-owned buffers.
func (a *App) lookupSurfaceKeys(keys []string) keymap.Match {
	id := ""
	if a.pluginDialog != nil {
		id = a.pluginDialog.buffer.ID
	} else if view := a.focusedSurfaceView(); view != nil {
		id = view.window.BufferID
	} else if a.popup == nil && !a.commandMode && (a.focus == "prompt" || a.focus == "chat") {
		projection := "prompt"
		if a.focus == "chat" {
			projection = "conversation"
		}
		for _, buffer := range a.surfaceBuffers {
			if buffer.Projection == projection {
				id = buffer.ID
				break
			}
		}
	}
	buffer, ok := a.surfaceBuffers[id]
	if !ok || len(buffer.Keymaps) == 0 {
		return a.keymaps.Lookup(a.inputMode(), keys)
	}
	cached := a.surfaceKeymaps[id]
	if cached.base != a.keymaps || cached.revision != buffer.Revision {
		bindings := append(a.keymaps.Bindings(), buffer.Keymaps...)
		registry, err := keymap.New(bindings)
		if err != nil {
			a.notice = "buffer keymap: " + err.Error()
			return a.keymaps.Lookup(a.inputMode(), keys)
		}
		cached = surfaceKeymapCache{a.keymaps, buffer.Revision, registry}
		if a.surfaceKeymaps == nil {
			a.surfaceKeymaps = map[string]surfaceKeymapCache{}
		}
		a.surfaceKeymaps[id] = cached
	}
	return cached.registry.LookupBuffer(id, a.inputMode(), keys)
}
