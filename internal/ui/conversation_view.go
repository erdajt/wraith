package ui

import "github.com/charmbracelet/x/ansi"

type conversationView struct {
	chatCursor, chatColumn             int
	offset, selected                   int
	follow                             bool
	search, visualMode                 string
	visualRow, visualColumn            int
	chatMotionPending, chatMotionCount string
	chatCount                          string
	chatG, chatZ                       bool
	rows                               []rendered
}

type textAnchor struct {
	message, source, cell int
	section               string
	field, part           int
}

// anchorAt identifies a display position by message, source line and unwrapped cell.
func anchorAt(rows []rendered, row, column int) textAnchor {
	if len(rows) == 0 {
		return textAnchor{message: -1}
	}
	line := rows[max(0, min(row, len(rows)-1))]
	anchor := textAnchor{line.Message, line.Source, line.Cell + column, line.Section, -1, line.Part}
	for _, span := range line.Spans {
		if column >= span.Column && column < span.Column+span.Width {
			anchor.field, anchor.cell, anchor.part = span.Field, span.Cell+column-span.Column, 0
			break
		}
	}
	return anchor
}

// locate resolves a source anchor, falling back to the nearest retained message or section.
func (anchor textAnchor) locate(rows []rendered) (int, int, bool) {
	if anchor.field >= 0 {
		best, column, distance := -1, 0, int(^uint(0)>>1)
		for i, row := range rows {
			if row.Message != anchor.message || row.Source != anchor.source || row.Section != anchor.section {
				continue
			}
			for _, span := range row.Spans {
				if span.Field != anchor.field {
					continue
				}
				cell := max(span.Cell, min(anchor.cell, span.Cell+span.Width-1))
				d := cell - anchor.cell
				if d < 0 {
					d = -d
				}
				if d < distance {
					best, column, distance = i, span.Column+cell-span.Cell, d
				}
			}
		}
		if best >= 0 {
			return best, column, true
		}
	}
	best, distance, found := 0, int(^uint(0)>>1), false
	for i, row := range rows {
		if row.Message == anchor.message && row.Section == anchor.section && row.Source == anchor.source && row.Part == anchor.part {
			if !found || row.Cell <= anchor.cell {
				best = i
			}
			found = true
			continue
		}
		if found {
			continue
		}
		d := row.Message - anchor.message
		if d < 0 {
			d = -d
		}
		d *= 1000000
		if row.Section != anchor.section {
			d += 100000
		}
		source := row.Source - anchor.source
		if source < 0 {
			source = -source
		}
		d += source
		if d < distance {
			best, distance = i, d
		}
	}
	column := 0
	if found {
		column = max(0, min(anchor.cell-rows[best].Cell, ansi.StringWidth(rows[best].Text)-1))
	}
	return best, column, found
}

// reflow retains cursor, viewport and selection anchors when rendered rows change.
func (v *conversationView) reflow(rows []rendered) {
	old := v.rows
	v.rows = rows
	if len(old) == 0 {
		return
	}
	if v.chatCursor >= 0 {
		var found bool
		v.chatCursor, v.chatColumn, found = anchorAt(old, v.chatCursor, v.chatColumn).locate(rows)
		if !found {
			v.visualMode = ""
		}
		if len(rows) > 0 {
			v.selected = rows[v.chatCursor].Message
		}
	}
	if !v.follow {
		v.offset, _, _ = anchorAt(old, v.offset, 0).locate(rows)
	}
	if v.visualMode != "" {
		var found bool
		v.visualRow, v.visualColumn, found = anchorAt(old, v.visualRow, v.visualColumn).locate(rows)
		if !found {
			v.visualMode = ""
		}
	}
}

// viewportStart keeps an anchored cursor visible after the available height changes.
func (v *conversationView) viewportStart(height int) int {
	end := max(0, len(v.rows)-height)
	if v.follow {
		return end
	}
	start := max(0, min(v.offset, end))
	if v.chatCursor >= 0 {
		if v.chatCursor < start {
			start = v.chatCursor
		}
		if v.chatCursor >= start+height {
			start = v.chatCursor - height + 1
		}
	}
	v.offset = max(0, min(start, end))
	return v.offset
}
