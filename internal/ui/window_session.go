package ui

import "github.com/erdajt/wraith/internal/session"

// remapConversation carries retained message anchors across compaction and forks.
func (a *App) remapConversation(old, next []session.Message) {
	mapping := make(map[int]int, len(next))
	before := len(old) - 1
	if len(next) <= len(old) {
		prefix := true
		for i, message := range next {
			if message.Role != old[i].Role || message.Content != old[i].Content || message.Thinking != old[i].Thinking {
				prefix = false
				break
			}
		}
		if prefix {
			before = len(next) - 1
		}
	}
	for after := len(next) - 1; after >= 0; after-- {
		for before >= 0 {
			message := old[before]
			index := before
			before--
			if message.Role == next[after].Role && message.Content == next[after].Content && message.Thinking == next[after].Thinking {
				mapping[index] = after
				break
			}
		}
	}

	for _, view := range a.windows {
		view.rows = append([]rendered(nil), view.rows...)
		if view.visualMode != "" {
			first, _, last, _ := view.selectionBounds()
			for i := max(0, first); i <= last && i < len(view.rows); i++ {
				if _, kept := mapping[view.rows[i].Message]; !kept {
					view.visualMode = ""
					break
				}
			}
		}
		for i := range view.rows {
			row := &view.rows[i]
			if row.Message < 0 {
				continue
			}
			if index, kept := mapping[row.Message]; kept {
				row.Message = index
				continue
			}
			nearest, distance := max(0, len(next)-1), len(old)+1
			for before, after := range mapping {
				d := before - row.Message
				if d < 0 {
					d = -d
				}
				if d < distance || d == distance && after < nearest {
					nearest, distance = after, d
				}
			}
			row.Message, row.Section, row.Source, row.Cell = nearest, "header", 0, 0
		}
		view.chatMotionPending, view.chatMotionCount, view.chatCount = "", "", ""
		view.chatG, view.chatZ = false, false
	}
	a.keyPending = nil
}
