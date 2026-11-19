package ui

import (
	"slices"
	"strings"
)

// pluginSnapshot detaches bounded workspace state for commands, events and service callbacks.
func (a *App) pluginSnapshot(argument, event string, data any) map[string]any {
	a.syncNativeSurfaces()
	var windows []map[string]any
	ids := make([]uint64, 0, len(a.windows))
	for id := range a.windows {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	places := a.windowPlaces()
	for _, id := range ids {
		view := a.windows[id]
		entry := map[string]any{"id": id, "focused": id == a.windowID, "cursor": view.chatCursor, "column": view.chatColumn, "search": view.search, "offset": view.offset, "visible": false}
		for _, place := range places {
			if place.id == id {
				entry["width"], entry["height"], entry["visible"] = place.width, place.height, true
			}
		}
		windows = append(windows, entry)
	}
	messages := []map[string]string{}
	budget := 128 << 10
	for i := len(a.Session.Messages) - 1; i >= 0 && len(messages) < 64 && budget > 0; i-- {
		message := a.Session.Messages[i]
		content := message.Content[:min(len(message.Content), budget)]
		budget -= len(content)
		messages = append(messages, map[string]string{"role": message.Role, "content": strings.ToValidUTF8(content, "")})
	}
	slices.Reverse(messages)
	snapshot := map[string]any{"argument": argument, "event": event, "prompt": a.Prompt.String(), "input": a.inputSnapshot(), "windows": windows, "session": map[string]any{"id": a.Session.ID, "model": a.Session.Model, "provider": a.Config.Provider, "directory": a.Directory, "messages": messages, "message_count": len(a.Session.Messages)}}
	snapshot["data"] = data
	return snapshot
}
