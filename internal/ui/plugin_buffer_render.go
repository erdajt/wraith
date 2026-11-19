package ui

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/erdajt/wraith/internal/plugin"
	"github.com/rivo/uniseg"
)

// renderSurfaceBuffer shares reader highlighting with cached plugin text and typed styles.
func renderSurfaceBuffer(buffer plugin.SurfaceBuffer, view *reader, width, height int, p palette, focused bool) []string {
	return view.styledLines(width, height, p, focused, func(row int, line string) string {
		return styledSurfaceLine(line, row, buffer.Spans, p)
	})
}

// styledSurfaceLine resolves overlapping typed spans without interpreting raw terminal controls.
func styledSurfaceLine(line string, row int, spans []plugin.SurfaceSpan, p palette) string {
	type boundary struct {
		column, index int
		start         bool
	}
	runes := []rune(line)
	starts, ends := make([]int, len(runes)+1), make([]int, len(runes)+1)
	position := 0
	clusters := uniseg.NewGraphemes(line)
	for clusters.Next() {
		next := position + utf8.RuneCountInString(clusters.Str())
		for i := position; i < next; i++ {
			starts[i], ends[i+1] = position, next
		}
		position = next
	}
	starts[len(runes)] = len(runes)
	var boundaries []boundary
	for i, span := range spans {
		if span.Start.Line > row || span.End.Line < row {
			continue
		}
		start, end := 0, len(runes)
		if span.Start.Line == row {
			start = span.Start.Column
		}
		if span.End.Line == row {
			end = span.End.Column
		}
		start, end = min(max(0, start), len(runes)), min(max(0, end), len(runes))
		if start < end {
			start, end = starts[start], ends[end]
			boundaries = append(boundaries, boundary{start, i, true}, boundary{end, i, false})
		}
	}
	slices.SortFunc(boundaries, func(a, b boundary) int { return a.column - b.column })
	active := make([]bool, len(spans))
	var out strings.Builder
	position, top := 0, -1
	for _, edge := range boundaries {
		if edge.column > position {
			style := plugin.SurfaceStyle{}
			if top >= 0 {
				style = spans[top].Style
			}
			out.WriteString(surfaceStyle(string(runes[position:edge.column]), style, p))
			position = edge.column
		}
		active[edge.index] = edge.start
		if edge.start {
			top = max(top, edge.index)
		} else if edge.index == top {
			for top >= 0 && !active[top] {
				top--
			}
		}
	}
	out.WriteString(ink(p.FG, string(runes[position:])))
	return out.String()
}

// surfaceStyle converts a validated typed style into renderer-owned escape sequences.
func surfaceStyle(text string, style plugin.SurfaceStyle, p palette) string {
	colors := map[string]string{
		"bg": p.BG, "fg": p.FG, "muted": p.Muted, "accent": p.Accent,
		"second": p.Second, "border": p.Border, "warn": p.Warn, "variable": p.Variable,
		"hint_quit": p.HintQuit, "hint_refresh": p.HintRefresh, "hint_cancel": p.HintCancel,
	}
	foreground := p.FG
	if style.Foreground != "" {
		foreground = style.Foreground
		if resolved, ok := colors[foreground]; ok {
			foreground = resolved
		}
	}
	codes := []string{"0", color(foreground, false)}
	if style.Background != "" {
		background := style.Background
		if resolved, ok := colors[background]; ok {
			background = resolved
		}
		codes = append(codes, color(background, true))
	}
	if style.Bold {
		codes = append(codes, "1")
	}
	if style.Italic {
		codes = append(codes, "3")
	}
	if style.Underline {
		codes = append(codes, "4")
	}
	return "\x1b[" + strings.Join(codes, ";") + "m" + text + reset
}
