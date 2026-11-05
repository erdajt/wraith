package ui

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
)

func (a *App) jumpSelected() {
	for i, line := range a.transcript(max(10, a.mainWidth()-5)) {
		if line.Message == a.selected {
			a.offset = i
			a.follow = false
			return
		}
	}
}
func (a *App) findNext(reverse bool) {
	if a.search == "" {
		return
	}
	lines := a.transcript(max(10, a.mainWidth()-5))
	if len(lines) == 0 {
		return
	}
	step := 1
	if reverse {
		step = -1
	}
	start := a.offset
	if a.follow {
		start = max(0, len(lines)-a.bodyHeight())
	}
	for n := 1; n <= len(lines); n++ {
		i := (start + step*n + len(lines)*2) % len(lines)
		if strings.Contains(strings.ToLower(ansi.Strip(lines[i].Text)), strings.ToLower(a.search)) {
			a.offset = i
			a.follow = false
			a.notice = "match: " + a.search
			return
		}
	}
	a.notice = "no match: " + a.search
}
