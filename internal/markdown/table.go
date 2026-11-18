package markdown

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/config"
)

// cells splits a pipe-delimited table row and trims its cells.
func cells(line string) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// renderTable renders a table or narrow-screen field list and returns the consumed source lines.
func renderTable(lines []string, width int, p config.Palette, transform func(string) string) ([]Row, int) {
	rows := [][]string{cells(lines[0])}
	consumed := 2
	for consumed < len(lines) && strings.Contains(lines[consumed], "|") && strings.TrimSpace(lines[consumed]) != "" {
		row := cells(lines[consumed])
		if len(row) != len(rows[0]) {
			break
		}
		rows = append(rows, row)
		consumed++
	}

	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], ansi.StringWidth(transform(inline(cell, p, "", 0))))
		}
	}

	total := 1
	for _, w := range widths {
		total += w + 3
	}

	var out []Row
	if total > width {
		for n, row := range rows[1:] {
			cellOffset := 0
			for i, cell := range row {
				text := transform(inline(rows[0][i], p, ";1", 0) + paint(p.Border, ": ") + inline(cell, p, "", 0))
				for _, part := range strings.Split(ansi.Wrap(text, width, ""), "\n") {
					out = append(out, Row{Text: part, Source: n + 2, Cell: cellOffset})
					cellOffset += ansi.StringWidth(part)
				}
			}
			out = append(out, Row{Source: n + 2, Cell: cellOffset})
		}

		if len(rows) == 1 {
			for _, text := range strings.Split(ansi.Hardwrap(inline(strings.Join(rows[0], " · "), p, ";1", 0), width, true), "\n") {
				out = append(out, Row{Text: text})
			}
		}
		return out, consumed
	}

	border := func(left, middle, right string) string {
		parts := make([]string, len(widths))
		for i, w := range widths {
			parts[i] = strings.Repeat("─", w+2)
		}
		return paint(p.Border, left+strings.Join(parts, middle)+right)
	}
	out = append(out, Row{Text: border("┌", "┬", "┐")})

	for n, row := range rows {
		var line strings.Builder
		line.WriteString(paint(p.Border, "│"))
		for i, cell := range row {
			flag := ""
			if n == 0 {
				flag = ";1"
			}
			rendered := transform(inline(cell, p, flag, 0))
			line.WriteString(" " + rendered + strings.Repeat(" ", widths[i]-ansi.StringWidth(rendered)+1) + paint(p.Border, "│"))
		}

		sourceLine := n + 1
		if n == 0 {
			sourceLine = 0
		}
		out = append(out, Row{Text: line.String(), Source: sourceLine})
		if n == 0 {
			out = append(out, Row{Text: border("├", "┼", "┤"), Source: 1})
		}
	}

	out = append(out, Row{Text: border("└", "┴", "┘"), Source: consumed})
	return out, consumed
}
