package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (a *App) menuFrame() []string {
	p := a.Config.Palette()
	m := a.popup
	w := min(a.width-6, 90)
	height := min(a.height-8, 18)
	if m.Reader != nil {
		content := []string{bold(p.Accent, clean(m.Title)), ink(p.Muted, m.Reader.Buffer.Status()+" · / search · v select · y yank · r refresh · q close")}
		content = append(content, m.Reader.lines(w-2, height-3, p)...)
		return frame(content, w, p, false, "", "")
	}
	content := []string{bold(p.Accent, clean(m.Title)), ink(p.Muted, "j/k move · / search · enter select · esc close"), ""}
	if m.Kind == "settings" || m.Kind == "value" {
		content[1] = ink(p.Muted, "j/k move · / search · "+a.Config.Keys["setting_help"]+" explain · enter select")
	}
	if m.Searching || m.Query != "" {
		content[1] = ink(p.Second, "/"+clean(m.Query)+"▏ · enter keeps filter · esc resets")
	}

	preview := ""
	if (m.Kind == "approval" || m.Kind == "modify-command") && a.proposal != nil {
		preview = a.proposal.Command
		if a.proposal.Reason != "" {
			preview = "reason: " + a.proposal.Reason + "\n\n" + preview
		}
	}
	if m.Kind == "block-actions" {
		preview = a.selectedBlock.Text
	}
	if preview != "" {
		lines := strings.Split(ansi.Hardwrap(clean(preview), w-6, true), "\n")
		for _, line := range lines[:min(len(lines), max(1, height-10))] {
			content = append(content, ink(p.Accent, "$ "+line))
		}
		if len(lines) > max(1, height-10) {
			content = append(content, ink(p.Muted, "… use view full command"))
		}
	}
	if m.Input != nil {
		content = append(content, ink(p.Second, m.Input.Status()), cursorText(clean(m.Input.String()), m.Input.Pos, p))
		return frame(content, w, p, false, "", "")
	}
	start := max(0, m.Index-(height-5)/2)
	start = min(start, max(0, len(m.Items)-(height-5)))
	if len(m.Items) == 0 {
		content = append(content, ink(p.Muted, "nothing here yet"))
	}
	for i := start; i < len(m.Items) && len(content) < height-1; i++ {
		it := m.Items[i]
		label := clean(it.Label)
		if it.Detail != "" {
			label += "  ·  " + clean(it.Detail)
		}
		label = ansi.Truncate(label, w-6, "…")
		if i == m.Index {
			content = append(content, bold(p.Accent, "› "+label))
		} else {
			content = append(content, ink(p.FG, "  "+label))
		}
	}
	return frame(content, w, p, false, "", "")
}

func frame(content []string, w int, p palette, inherit bool, fg, bg string) []string {
	wrap := func(s string) string {
		if inherit {
			return reset + fit(s, w) + reset
		}
		return base(p, s, w)
	}
	rows := []string{wrap(ink(p.Border, "╭"+strings.Repeat("─", w-2)+"╮"))}
	for _, s := range content {
		rows = append(rows, wrap(ink(p.Border, "│")+"\x1b[0"+fg+bg+"m"+fit(s, w-2)+reset+ink(p.Border, "│")))
	}
	return append(rows, wrap(ink(p.Border, "╰"+strings.Repeat("─", w-2)+"╯")))
}

func (a *App) terminalFrame(index, w, h int) []string {
	p := a.Config.Palette()
	pane := a.panes[index]
	cfg := a.Config.Terminal
	fg, bg := "", ""
	if cfg.Foreground == "theme" {
		fg = color(p.FG, false)
	} else if cfg.Foreground != "inherit" {
		fg = color(cfg.Foreground, false)
	}
	if cfg.Background == "theme" {
		bg = color(p.BG, true)
	} else if cfg.Background != "inherit" {
		bg = color(cfg.Background, true)
	}
	lines := pane.Screen.Lines(a.focus == fmt.Sprintf("terminal%d", index) && pane.Pending == "", fg, bg)
	if pane.Reader != nil {
		lines = pane.Reader.lines(w-2, h-3, p)
		lines = append(lines, ink(p.Muted, pane.Reader.Buffer.Status()+" · / search · v select · y yank · r refresh · i shell"))
	}
	if pane.Pending != "" {
		lines[len(lines)-2] = ink(p.Warn, " enter inserts · r tracked run · e edits · esc dismisses")
		lines[len(lines)-1] = ink(p.Accent, ansi.Truncate(pane.Pending, w-2, "…"))
	}
	if pane.Err != "" {
		lines[len(lines)-1] = ink(p.Warn, pane.Err)
	}
	fgArg, bgArg := "", ""
	if fg != "" {
		fgArg = ";" + fg
	}
	if bg != "" {
		bgArg = ";" + bg
	}
	box := frame(lines, w, p, cfg.Background == "inherit", fgArg, bgArg)
	label := " scratch shell · " + a.Config.Keys["terminal_overlay"] + " "
	if index == 1 {
		label = " side shell · " + a.Config.Keys["terminal_split"] + " "
	}
	label = ansi.Truncate(label, w-3, "…")
	box[0] = reset + ink(p.Border, "╭─") + ink(p.Accent, label) + ink(p.Border, strings.Repeat("─", max(0, w-ansi.StringWidth(label)-3))+"╮") + reset
	return box[:min(h, len(box))]
}

func (a *App) overlay(rows, box []string, x, y int) {
	for i, line := range box {
		if y+i < 0 || y+i >= len(rows) {
			continue
		}
		w := ansi.StringWidth(line)
		left := fit(ansi.Cut(rows[y+i], 0, max(0, x)), max(0, x))
		right := fit(ansi.Cut(rows[y+i], x+w, a.width), max(0, a.width-x-w))
		rows[y+i] = left + reset + line + reset + right
	}
}
