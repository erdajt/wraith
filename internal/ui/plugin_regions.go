package ui

import (
	"slices"

	"github.com/erdajt/wraith/internal/plugin"
)

// surfaceRegion selects the newest live owner for a replacement slot.
func (a *App) surfaceRegion(slot string) (plugin.SurfaceRegion, bool) {
	var current plugin.SurfaceRegion
	for _, region := range a.surfaceRegions {
		if region.Slot == slot && region.Order > current.Order {
			current = region
		}
	}
	return current, current.ID != ""
}

// conversationWidth reserves a sidebar without shrinking the shared prompt or native side shell.
func (a *App) conversationWidth() int {
	width := a.mainWidth()
	if region, ok := a.surfaceRegion("sidebar"); ok {
		width -= min(region.Width, max(0, width-25)) + 1
	}
	return max(1, width)
}

// regionLines clips cached typed text without evaluating extension code during rendering.
func regionLines(region plugin.SurfaceRegion, width, height int, p palette) []string {
	lines := make([]string, max(0, height))
	for i := range lines {
		text := ""
		if i < len(region.Lines) {
			text = styledSurfaceLine(region.Lines[i], i, region.Spans, p)
		}
		lines[i] = base(p, text, max(0, width))
	}
	return lines
}

// drawSurfaceRegions composes passive extension frames below dialogs and terminal overlays.
func (a *App) drawSurfaceRegions(rows []string, p palette) {
	if region, ok := a.surfaceRegion("header"); ok {
		rows[0] = regionLines(region, a.width, 1, p)[0]
	}
	if region, ok := a.surfaceRegion("footer"); ok && !a.commandMode && !a.searchMode {
		rows[a.height-2] = regionLines(region, a.width, 1, p)[0]
	}
	if region, ok := a.surfaceRegion("sidebar"); ok {
		start := a.conversationWidth()
		a.overlay(rows, regionLines(region, max(0, a.mainWidth()-start-1), a.bodyHeight(), p), start+1, 3)
		for y := 3; y < 3+a.bodyHeight(); y++ {
			a.overlay(rows, []string{ink(p.Border, "│")}, start, y)
		}
	}
	var overlays []plugin.SurfaceRegion
	for _, region := range a.surfaceRegions {
		if region.Slot == "body" {
			overlays = append(overlays, region)
		}
	}
	slices.SortFunc(overlays, func(a, b plugin.SurfaceRegion) int {
		if a.Order < b.Order {
			return -1
		}
		if a.Order > b.Order {
			return 1
		}
		return 0
	})
	for _, region := range overlays {
		width := min(region.Width, max(0, a.conversationWidth()-region.Column))
		height := min(region.Height, max(0, a.bodyHeight()-region.Row))
		if width > 0 && height > 0 {
			a.overlay(rows, regionLines(region, width, height, p), region.Column, 3+region.Row)
		}
	}
}
