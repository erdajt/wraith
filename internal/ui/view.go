package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/contextual"
)

func (a *App) View() string {
	p := palettes[a.Config.Theme]
	if a.width < 50 || a.height < 18 {
		return "wraith needs a terminal at least 50 columns × 18 rows.\nresize the terminal, or press ctrl+q to exit."
	}
	w := a.width
	mainW := a.mainWidth()
	bodyH := a.bodyHeight()
	rows := make([]string, a.height)
	for i := range rows {
		rows[i] = base(p, "", w)
	}
	state := "offline"
	if a.connected {
		state = "local"
	}
	if a.demo {
		state = "demo"
	}
	logo := bold(p.Accent, "  ◈ W R A I T H") + ink(p.Muted, "  /  "+state)
	right := ink(p.Second, a.Prompt.Status()) + ink(p.Muted, "  "+a.focus+"  ")
	rows[0] = base(p, logo+strings.Repeat(" ", max(1, w-ansi.StringWidth(logo)-ansi.StringWidth(right)))+right, w)
	used := contextual.Used(a.Session, a.Instructions)
	ratio := float64(used) / float64(a.Config.Context)
	bar := int(ratio * 12)
	bar = max(0, min(12, bar))
	barColor := p.Accent
	if ratio >= a.Config.Compact.Threshold {
		barColor = p.Warn
	}
	usage := ink(barColor, strings.Repeat("━", bar)) + ink(p.Border, strings.Repeat("─", 12-bar)) + ink(p.Muted, fmt.Sprintf(" ~%d/%dk", used, a.Config.Context/1024))
	model := ink(p.Muted, "  ") + ink(p.FG, ansi.Truncate(clean(a.Session.Model), max(15, w-ansi.StringWidth(usage)-23), "…")) + ink(p.Second, "  think:"+a.Config.Thinking)
	rows[1] = base(p, model+strings.Repeat(" ", max(1, w-ansi.StringWidth(model)-ansi.StringWidth(usage)-2))+usage, w)
	rows[2] = base(p, ink(p.Border, strings.Repeat("─", w)), w)
	transcript := a.transcript(max(10, mainW-5))
	start := min(a.offset, max(0, len(transcript)-bodyH))
	if a.follow {
		start = max(0, len(transcript)-bodyH)
	}
	for y := 0; y < bodyH; y++ {
		content := ""
		if start+y < len(transcript) {
			content = transcript[start+y].Text
		}
		rows[3+y] = base(p, "  "+content, mainW) + base(p, "", w-mainW)
	}
	if len(a.Session.Messages) == 0 && !a.busy {
		welcome := []string{bold(p.Accent, "W R A I T H"), ink(p.Muted, "local intelligence · deliberate control"), "", ink(p.FG, "write a prompt, or bring your own evidence."), ink(p.Muted, a.Config.Keys["send"]+" send     "+a.Config.Keys["editor"]+" editor     /models switch"), ink(p.Muted, a.Config.Keys["terminal_overlay"]+" scratch shell     "+a.Config.Keys["terminal_split"]+" side shell"), "", ink(p.Second, "/remember preferences     /pin exact facts")}
		y := 3 + max(0, (bodyH-len(welcome))/2)
		for _, line := range welcome {
			if y >= 3+bodyH {
				break
			}
			pad := max(0, (mainW-ansi.StringWidth(line))/2)
			rows[y] = base(p, strings.Repeat(" ", pad)+line, mainW) + base(p, "", w-mainW)
			y++
		}
	}
	inputY := 3 + bodyH
	label := " prompt "
	if a.focus == "chat" {
		label = " prompt · tab to focus "
	}
	rows[inputY] = base(p, ink(p.Border, "╭─"+label+strings.Repeat("─", max(0, mainW-ansi.StringWidth(label)-3))+"╮"), mainW) + base(p, "", w-mainW)
	lines := a.promptLines(mainW - 4)
	for y := 0; y < 4; y++ {
		text := ""
		if y < len(lines) {
			text = lines[y]
		}
		rows[inputY+1+y] = base(p, ink(p.Border, "│")+" "+fit(text, mainW-4)+" "+ink(p.Border, "│"), mainW) + base(p, "", w-mainW)
	}
	rows[inputY+5] = base(p, ink(p.Border, "╰"+strings.Repeat("─", mainW-2)+"╯"), mainW) + base(p, "", w-mainW)
	status := clean(a.notice)
	if a.loading {
		status = "loading model · " + a.Session.Model
	}
	if a.busy {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		status = frames[a.frame%len(frames)] + " " + a.phase + " · " + a.Config.Keys["cancel"] + " cancels"
	}
	rows[a.height-3] = base(p, "  "+ink(p.Accent, ansi.Truncate(status, w-4, "…")), w)
	footer := "  " + a.Config.Keys["send"] + " send  ·  " + a.Config.Keys["focus"] + " focus  ·  : commands  ·  /help  ·  " + a.Config.Keys["quit"] + " quit"
	if a.commandMode {
		footer = "  :" + clean(a.command) + "▏"
	} else if a.searchMode {
		footer = "  /" + clean(a.command) + "▏"
	}
	rows[a.height-2] = base(p, ink(p.Muted, fit(footer, w)), w)
	rows[a.height-1] = base(p, ink(p.Border, "  "+clean(a.Session.Title)+"  ·  "+a.Session.ID[:8]), w)
	if pane := a.panes[1]; pane != nil && pane.Visible {
		pw, ph := a.paneSize(1)
		a.overlay(rows, a.terminalFrame(1, pw, ph), w-pw, 3)
	}
	if pane := a.panes[0]; pane != nil && pane.Visible {
		pw, ph := a.paneSize(0)
		a.overlay(rows, a.terminalFrame(0, pw, ph), (w-pw)/2, (a.height-ph)/2)
	}
	if a.popup == nil {
		if box := a.completionFrame(); len(box) > 0 {
			a.overlay(rows, box, 2, inputY-len(box))
		}
	}
	if a.popup != nil {
		box := a.menuFrame()
		a.overlay(rows, box, (w-ansi.StringWidth(box[0]))/2, (a.height-len(box))/2)
	}
	return strings.Join(rows, "\n")
}
