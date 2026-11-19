package plugin

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/erdajt/wraith/internal/vim"
)

// applyWindow commits validated layout metadata; the UI checks available terminal space first.
func (s *SurfaceStore) applyWindow(m SurfaceMutation) (SurfaceDelta, error) {
	delta := SurfaceDelta{Operation: m.Operation}
	current, exists := s.windows[m.ID]
	if m.Operation != "window.create" {
		if !exists {
			return delta, fmt.Errorf("window is no longer available: %s", m.ID)
		}
		if current.Revision != m.Expected {
			return delta, fmt.Errorf("window %s changed: expected revision %d, current %d", m.ID, m.Expected, current.Revision)
		}
	}
	if m.Operation == "window.delete" {
		delete(s.windows, m.ID)
		delta.RemovedWindows = []string{m.ID}
		return delta, nil
	}
	if m.Operation == "window.focus" {
		current.Focus = true
		delta.Window = &current
		return delta, nil
	}
	if m.Operation != "window.create" && m.Operation != "window.configure" {
		return delta, fmt.Errorf("unknown window operation: %s", m.Operation)
	}
	if m.Window == nil {
		return delta, errors.New("window options are required")
	}
	window := *m.Window
	focus := window.Focus
	window.Focus = false
	if m.Operation == "window.create" {
		reservation, ok := s.reserved[m.ID]
		if !ok || reservation.kind != "window" || m.Expected != 0 || exists {
			return delta, errors.New("window creation requires a live reservation and revision zero")
		}
		window.ID, window.Owner, window.Revision = m.ID, reservation.owner, 1
	} else {
		window.ID, window.Owner, window.Revision = current.ID, current.Owner, current.Revision
	}
	buffer := s.buffers[window.BufferID]
	if buffer == nil {
		return delta, errors.New("window buffer is no longer available")
	}
	if window.Mode == "" {
		window.Mode = "normal"
	}
	if !slices.Contains([]string{"normal", "insert"}, window.Mode) {
		return delta, errors.New("window mode must be normal or insert")
	}
	if window.Mode == "insert" && !buffer.Editable {
		return delta, errors.New("read-only buffer cannot enter insert mode")
	}
	if window.Kind == "" {
		window.Kind = "split"
	}
	if window.Position == "" {
		window.Position = "right"
	}
	if !slices.Contains([]string{"split", "float"}, window.Kind) || !slices.Contains([]string{"left", "right", "above", "below"}, window.Position) {
		return delta, errors.New("invalid window kind or split position")
	}
	if window.Width < 0 || window.Height < 0 || window.Width > 10000 || window.Height > 10000 || window.Row < 0 || window.Column < 0 || window.Row > 10000 || window.Column > 10000 {
		return delta, errors.New("window dimensions require integers between 0 and 10000")
	}
	if window.Kind == "float" && (window.Width == 0 || window.Height == 0) {
		return delta, errors.New("floating windows require positive width and height")
	}
	if len(window.Title) > 256 || len(window.RelativeTo) > 256 {
		return delta, errors.New("window title or relative target exceeds 256 bytes")
	}
	if _, err := validateSurfaceLines([]string{window.Title, window.RelativeTo}, 514); err != nil {
		return delta, err
	}
	if window.Cursor.Line < 0 || window.Cursor.Line >= len(buffer.Lines) || window.Cursor.Column < 0 || window.Cursor.Column > utf8.RuneCountInString(buffer.Lines[window.Cursor.Line]) {
		return delta, errors.New("window cursor is outside its buffer")
	}
	if exists && window != current {
		window.Revision++
	}
	s.windows[m.ID] = window
	delete(s.reserved, m.ID)
	window.Focus = focus
	delta.Window = &window
	return delta, nil
}

// rebaseBufferWindows applies the same rune replacement as native editor views.
func (s *SurfaceStore) rebaseBufferWindows(buffer *surfaceBufferRecord, previous []string) []SurfaceWindow {
	var views []SurfaceWindow
	for _, window := range s.windows {
		if window.BufferID == buffer.ID {
			views = append(views, window)
		}
	}
	if len(views) == 0 {
		return nil
	}
	before, after := []rune(strings.Join(previous, "\n")), []rune(strings.Join(buffer.Lines, "\n"))
	change := vim.ChangedText(before, after)
	var changed []SurfaceWindow
	for _, window := range views {
		id := window.ID
		offset := window.Cursor.Column
		for _, line := range previous[:min(window.Cursor.Line, len(previous))] {
			offset += utf8.RuneCountInString(line) + 1
		}
		mode := window.Mode
		if !buffer.Editable {
			mode = "normal"
		}
		offset = max(0, min(change.Rebase(offset), len(after)))
		if mode != "insert" && offset == len(after) && offset > 0 && after[offset-1] != '\n' {
			offset--
		}
		cursor := SurfacePosition{}
		for _, r := range after[:offset] {
			if r == '\n' {
				cursor.Line++
				cursor.Column = 0
			} else {
				cursor.Column++
			}
		}
		if cursor != window.Cursor || mode != window.Mode {
			window.Mode = mode
			window.Cursor = cursor
			window.Revision++
			s.windows[id] = window
			changed = append(changed, window)
		}
	}
	slices.SortFunc(changed, func(a, b SurfaceWindow) int { return strings.Compare(a.ID, b.ID) })
	return changed
}
