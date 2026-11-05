package markdown

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/config"
)

func cells(line string) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func renderTable(lines []string, width int, p config.Palette) ([]string, int) {
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
			widths[i] = max(widths[i], ansi.StringWidth(inline(cell, p, "", 0)))
		}
	}
	total := 1
	for _, w := range widths {
		total += w + 3
	}
	var out []string
	if total > width {
		for _, row := range rows[1:] {
			for i, cell := range row {
				out = append(out, strings.Split(ansi.Wrap(inline(rows[0][i], p, ";1", 0)+paint(p.Border, ": ")+inline(cell, p, "", 0), width, ""), "\n")...)
			}
			out = append(out, "")
		}
		if len(rows) == 1 {
			out = append(out, inline(strings.Join(rows[0], " · "), p, ";1", 0))
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
	out = append(out, border("┌", "┬", "┐"))
	for n, row := range rows {
		var line strings.Builder
		line.WriteString(paint(p.Border, "│"))
		for i, cell := range row {
			flag := ""
			if n == 0 {
				flag = ";1"
			}
			rendered := inline(cell, p, flag, 0)
			line.WriteString(" " + rendered + strings.Repeat(" ", widths[i]-ansi.StringWidth(rendered)+1) + paint(p.Border, "│"))
		}
		out = append(out, line.String())
		if n == 0 {
			out = append(out, border("├", "┼", "┤"))
		}
	}
	out = append(out, border("└", "┴", "┘"))
	return out, consumed
}
