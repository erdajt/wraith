package ui

import (
	"fmt"

	"github.com/charmbracelet/x/ansi"
)

func (a *App) completionFrame() []string {
	choices, index, visible := a.completions()
	if !visible {
		return nil
	}
	p := palettes[a.Config.Theme]
	w := min(a.mainWidth()-4, 90)
	count := max(1, min(6, a.bodyHeight()-4))
	start := max(0, min(index-count/2, len(choices)-count))
	title := "commands"
	if len(choices) > 0 {
		title += fmt.Sprintf(" · %d/%d", index+1, len(choices))
	}
	content := []string{bold(p.Accent, title)}
	if len(choices) == 0 {
		content = append(content, ink(p.Muted, "no matching commands"))
	}
	for i := start; i < len(choices) && i < start+count; i++ {
		choice := choices[i]
		label := "/" + choice.Name + "  ·  " + choice.Description
		label = ansi.Truncate(label, w-6, "…")
		if i == index {
			content = append(content, bold(p.Accent, "› "+label))
		} else {
			content = append(content, ink(p.FG, "  "+label))
		}
	}
	content = append(content, ink(p.Muted, "↑/↓ move · enter choose · esc close"))
	return frame(content, w, p, false, "", "")
}
