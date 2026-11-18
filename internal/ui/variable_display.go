package ui

import (
	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/variables"
)

// expandStyled expands private references for local display and highlights resolved values.
func (a *App) expandStyled(text string) string {
	expanded, spans := a.Variables.Expand(text)
	return a.styleVariableSpans(expanded, spans)
}

// styleVariableSpans colors resolved variable spans without shifting their display positions.
func (a *App) styleVariableSpans(expanded string, spans []variables.Span) string {
	runes := []rune(expanded)
	for i := len(spans) - 1; i >= 0; i-- {
		span := spans[i]
		start := ansi.StringWidth(string(runes[:span.Start]))
		end := ansi.StringWidth(string(runes[:span.End]))
		expanded = ansi.Cut(expanded, 0, start) + ink(a.Config.Palette().Variable, string(runes[span.Start:span.End])) + ansi.Cut(expanded, end, ansi.StringWidth(expanded))
	}
	return expanded
}
