package ui

import "github.com/charmbracelet/x/ansi"

func (a *App) expandStyled(text string) string {
	expanded, spans := a.Variables.Expand(text)
	runes := []rune(expanded)
	for i := len(spans) - 1; i >= 0; i-- {
		span := spans[i]
		start := ansi.StringWidth(string(runes[:span.Start]))
		end := ansi.StringWidth(string(runes[:span.End]))
		expanded = ansi.Cut(expanded, 0, start) + ink(a.Config.Palette().Variable, string(runes[span.Start:span.End])) + ansi.Cut(expanded, end, ansi.StringWidth(expanded))
	}
	return expanded
}
