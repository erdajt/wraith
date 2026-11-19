package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// approvalFrame renders command details, reasons and approval actions within the available screen.
func (a *App) approvalFrame() []string {
	p, m, request := a.Config.Palette(), a.popup, a.proposal
	w, h := min(a.width-6, 100), min(a.height-2, 30)
	inner := w - 4
	roomy := h >= 24
	mode := "foreground"
	if request.Background {
		mode = "background"
	}
	content := []string{
		bold(p.Warn, "◆ APPROVAL REQUIRED"),
		ink(p.FG, "not executed · "+mode),
		ink(p.Muted, ansi.Truncate("cwd: "+clean(request.launch.Directory), inner, "…")),
	}
	if len(request.policyChanges) > 0 {
		content = append(content, ink(p.Warn, ansi.Truncate("policy changed: "+strings.Join(request.policyChanges, ", ")+" · view full command", inner, "…")))
	}
	if roomy {
		content = append(content, "")
	}
	content = append(content, bold(p.Second, "COMMAND"))

	reason, _ := request.Variables.Expand(request.Reason)
	if reason == "" {
		reason = "no reason supplied"
	}
	reasons := strings.Split(ansi.Hardwrap(clean("reason: "+reason), inner, true), "\n")
	reasonLimit := 1
	if roomy {
		reasonLimit = 3
	}
	if len(reasons) > reasonLimit {
		reasons = reasons[:reasonLimit]
		reasons[len(reasons)-1] = ansi.Truncate(reasons[len(reasons)-1], inner-1, "") + "…"
	}

	slots := max(1, min(len(m.Items), 6))
	spacing := 0
	if roomy {
		spacing = 1
	}
	commandLimit := max(1, min(8, h-2-len(content)-len(reasons)-slots-2-spacing))

	command := clean(request.Resolved)
	command = a.styleVariableSpans(command, request.Variables.LiteralSpans(command))
	commands := strings.Split(ansi.Hardwrap(command, inner-2, true), "\n")
	clipped := len(commands) > commandLimit
	for _, line := range commands[:min(len(commands), commandLimit)] {
		content = append(content, ink(p.Accent, "▎ ")+ink(p.FG, line))
	}
	if roomy {
		content = append(content, "")
	}
	for _, line := range reasons {
		content = append(content, ink(p.Muted, line))
	}

	divider := strings.Repeat("─", inner)
	if clipped {
		divider = "… view full command for remaining lines"
	}
	content = append(content, ink(p.Border, ansi.Truncate(divider, inner, "…")))

	start := max(0, min(m.Index-slots/2, len(m.Items)-slots))
	if len(m.Items) == 0 {
		content = append(content, ink(p.Muted, "no matching actions"))
	}
	for i := start; i < min(len(m.Items), start+slots); i++ {
		action := m.Items[i]
		label := action.Label
		if inner >= 64 && action.Detail != "" {
			label += " · " + action.Detail
		}
		label = ansi.Truncate(clean(label), inner-2, "…")
		if i == m.Index {
			content = append(content, "\x1b[1;7;"+color(p.Accent, false)+"m"+fit("› "+label, inner)+reset)
		} else {
			content = append(content, ink(p.FG, "  "+label))
		}
	}

	hint := keyHints("j/k or ↑/↓ · enter selects · esc rejects", p)
	if m.Searching || m.Query != "" {
		hint = ink(p.Second, "/"+clean(m.Query)+"▏ · ") + keyHints("enter selects · esc resets", p)
	}
	content = append(content, ansi.Truncate(hint, inner, "…"))

	for i := range content {
		content[i] = " " + fit(content[i], inner) + " "
	}
	p.Border = p.Accent
	return frame(content, w, p, false, "", "")
}
