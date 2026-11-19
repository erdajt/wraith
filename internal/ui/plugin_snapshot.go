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
		if view.surface != nil {
			continue
		}
		entry := map[string]any{"id": id, "focused": id == a.windowID, "cursor": view.chatCursor, "column": view.chatColumn, "search": view.search, "offset": view.offset, "visible": false}
		for _, place := range places {
			if place.id == id {
				entry["width"], entry["height"], entry["visible"] = place.width, place.height, true
			}
		}
		windows = append(windows, entry)
	}
	messages := []map[string]string{}
	budget := 64 << 10
	for i := len(a.Session.Messages) - 1; i >= 0 && len(messages) < 64 && budget > 0; i-- {
		message := a.Session.Messages[i]
		content := message.Content[:min(len(message.Content), budget)]
		budget -= len(content)
		messages = append(messages, map[string]string{"role": message.Role, "content": strings.ToValidUTF8(content, "")})
	}
	slices.Reverse(messages)
	snapshot := map[string]any{"argument": snapshotText(argument, 8<<10), "event": event, "prompt": snapshotText(a.Prompt.String(), 32<<10), "input": a.inputSnapshot(), "windows": windows, "session": map[string]any{"id": a.Session.ID, "model": a.Session.Model, "provider": a.Config.Provider, "directory": a.Directory, "messages": messages, "message_count": len(a.Session.Messages)}}
	if a.plugins != nil {
		snapshot["surfaces"] = a.plugins.Surfaces().Index()
	}
	snapshot["prompt_truncated"] = len(a.Prompt.String()) > 32<<10
	snapshot["argument_truncated"] = len(argument) > 8<<10
	snapshot["data"] = data
	return snapshot
}

// snapshotText bounds compatibility mirrors without splitting a utf8 character.
func snapshotText(text string, limit int) string {
	return strings.ToValidUTF8(text[:min(len(text), limit)], "")
}
