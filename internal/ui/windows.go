package ui

import (
	"fmt"
	"slices"
)

// windowPlaces measures conversation leaves inside the shared workspace body.
func (a *App) windowPlaces() []windowPlace {
	places := a.windowTree.layout(windowRect{0, 3, a.conversationWidth(), a.bodyHeight()}, a.windowID)
	var floating []uint64
	for id, view := range a.windows {
		if view.surface != nil && view.surface.window.Kind == "float" {
			floating = append(floating, id)
		}
	}
	slices.Sort(floating)
	for _, id := range floating {
		window := a.windows[id].surface.window
		x, y := min(max(0, window.Column), max(0, a.conversationWidth()-3)), min(max(0, window.Row), max(0, a.bodyHeight()-3))
		width, height := min(max(3, window.Width), a.conversationWidth()-x), min(max(3, window.Height), a.bodyHeight()-y)
		places = append(places, windowPlace{id, windowRect{x, y + 3, width, height}})
	}
	return places
}

// chatSize returns the focused conversation leaf's current dimensions.
func (a *App) chatSize() (int, int) {
	for _, place := range a.windowPlaces() {
		if place.id == a.windowID {
			return place.width, place.height
		}
	}
	return a.mainWidth(), a.bodyHeight()
}

// chatWidth reserves the focused leaf's transcript padding.
func (a *App) chatWidth() int {
	w, _ := a.chatSize()
	return max(10, w-5)
}

// chatHeight returns the focused leaf's viewport height.
func (a *App) chatHeight() int {
	_, h := a.chatSize()
	return h
}

// focusWindow switches views and cancels incomplete motions before accepting new input.
func (a *App) focusWindow(id uint64) {
	view, ok := a.windows[id]
	if !ok {
		a.notice = "conversation window is no longer available"
		return
	}
	if current := a.focusedSurfaceView(); current != nil {
		current.reader.Buffer.Pending, current.reader.Buffer.Count, current.reader.Buffer.OperatorCount = "", "", ""
	}
	a.Prompt.Pending, a.Prompt.Count, a.Prompt.OperatorCount = "", "", ""
	a.chatCount, a.chatMotionPending, a.chatMotionCount = "", "", ""
	a.chatG, a.chatZ = false, false
	a.keyPending = nil
	a.searchMode, a.commandMode = false, false
	a.windowID, a.conversationView = id, view
	a.focus = "chat"
	a.notice = fmt.Sprintf("conversation window %d · %d open", id, len(a.windows))
}

// windowAction applies a split, focus, close or proportion change to conversation windows.
func (a *App) windowAction(action string) {
	if a.surfaceWindowAction(action) {
		return
	}
	a.chatCount, a.chatMotionPending, a.chatMotionCount = "", "", ""
	a.chatG, a.chatZ = false, false
	places := a.windowPlaces()
	switch action {
	case "s", "v":
		w, h := a.chatSize()
		if action == "v" && w < 49 || action == "s" && h < 9 {
			a.notice = "not enough space for another conversation window"
			return
		}
		if len(a.windows) >= 32 {
			a.notice = "conversation window limit reached"
			return
		}
		a.transcript(a.chatWidth())
		copy := *a.conversationView
		copy.visualMode, copy.chatCount, copy.chatMotionPending, copy.chatMotionCount = "", "", "", ""
		copy.chatG, copy.chatZ = false, false
		a.nextWindowID++
		a.windows[a.nextWindowID] = &copy
		a.windowTree.split(a.windowID, a.nextWindowID, action == "v")
		a.focusWindow(a.nextWindowID)
	case "h", "j", "k", "l":
		a.focusWindow(neighbor(places, a.windowID, action))
	case "w":
		for i, place := range places {
			if place.id == a.windowID {
				a.focusWindow(places[(i+1)%len(places)].id)
				break
			}
		}
	case "c":
		if len(a.windows) == 1 {
			a.notice = "last conversation window · :q quits wraith"
			return
		}
		closed := a.windowID
		a.detachSurfaceWindow(closed)
		delete(a.windows, closed)
		a.ensureSurfaceFallback()
		a.focusWindow(a.windowPlaces()[0].id)
	case "o":
		a.windowTree = &windowNode{id: a.windowID}
		a.windows = map[uint64]*conversationView{a.windowID: a.conversationView}
	case "=":
		a.windowTree.equalize()
	case "+", "-", ">", "<":
		delta := 0.05
		if action == "-" || action == "<" {
			delta = -delta
		}
		if !a.windowTree.resize(a.windowID, action == ">" || action == "<", delta) {
			a.notice = "no divider in that direction"
			return
		}
	}
	if action == "+" || action == "-" || action == ">" || action == "<" || action == "=" {
		w, h := a.chatSize()
		a.notice = fmt.Sprintf("conversation window %d · %d×%d", a.windowID, w, h)
	}
	for _, place := range a.windowPlaces() {
		a.windowTranscript(a.windows[place.id], max(10, place.width-5), place.id == a.windowID && a.focus == "chat")
	}
}

// focusDirection navigates conversation panes and the shared prompt without changing draft mode.
func (a *App) focusDirection(direction string) {
	if a.focus == "prompt" {
		if direction == "k" {
			a.focusWindow(a.windowID)
		}
		return
	}
	if a.focus != "chat" {
		return
	}

	places := append(a.windowPlaces(), windowPlace{0, windowRect{0, 3 + a.bodyHeight(), a.mainWidth(), 6}})
	id := neighbor(places, a.windowID, direction)
	if id != 0 {
		a.focusWindow(id)
		return
	}
	a.chatCount, a.chatMotionPending, a.chatMotionCount = "", "", ""
	a.chatG, a.chatZ = false, false
	a.keyPending = nil
	a.commandMode, a.searchMode = false, false
	a.focus = "prompt"
	a.notice = "prompt focused"
}
