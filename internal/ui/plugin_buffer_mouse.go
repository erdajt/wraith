package ui

import (
	"unicode/utf8"

	"github.com/erdajt/wraith/internal/plugin"
	"github.com/rivo/uniseg"
)

// surfaceMouse places a view's rune cursor using the same grapheme cells as its renderer.
func (a *App) surfaceMouse(place windowPlace, x, y int) {
	view := a.windows[place.id].surface
	buffer := a.surfaceBuffers[view.window.BufferID]
	row := view.reader.Offset + y - place.y - 1
	if x <= place.x || x >= place.x+place.width-1 || y <= place.y || row < 0 || row >= len(buffer.Lines) || y >= place.y+place.height-1 {
		return
	}
	start := surfaceCursorOffset(buffer.Lines, view.window.Cursor)
	lineStart := surfaceCursorOffset(buffer.Lines, plugin.SurfacePosition{Line: row})
	column := x - place.x - 1
	if row == view.window.Cursor.Line {
		cursor, _ := runeDisplayRange(buffer.Lines[row], start-lineStart, start-lineStart)
		column += max(0, cursor-max(1, place.width-2)+1)
	}
	position, cells, last := 0, 0, 0
	clusters := uniseg.NewGraphemes(buffer.Lines[row])
	for clusters.Next() {
		width := clusters.Width()
		if clusters.Str() == "\t" {
			width = 4 - cells%4
		}
		if cells+width > column {
			break
		}
		last = position
		cells += width
		position += utf8.RuneCountInString(clusters.Str())
	}
	if view.reader.Buffer.Mode != "insert" && position == len([]rune(buffer.Lines[row])) {
		position = last
	}
	view.reader.Buffer.Pos = lineStart + position
	a.syncSurfaceCursor(view)
}
