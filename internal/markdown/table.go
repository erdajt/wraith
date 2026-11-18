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
			for i, cell := range row {
				label := transform(inline(rows[0][i], p, ";1", 0) + paint(p.Border, ": "))
				value := transform(inline(cell, p, "", 0))
				entry := Row{Text: label + value, Source: n + 2, Region: "table", Part: i + 1,
					Spans: []Span{{Column: ansi.StringWidth(label), Width: ansi.StringWidth(value), Field: i}}}
				out = append(out, wrapTableRow(entry, width)...)

			}
			out = append(out, Row{Source: n + 2, Region: "table-gap"})
		}

		if len(rows) == 1 {
			entry := Row{Region: "table"}
			for i, cell := range rows[0] {
				if i > 0 {
					entry.Text += " · "
				}
				value := transform(inline(cell, p, ";1", 0))
				entry.Spans = append(entry.Spans, Span{Column: ansi.StringWidth(entry.Text), Width: ansi.StringWidth(value), Field: i})
				entry.Text += value
			}
			out = append(out, wrapTableRow(entry, width)...)
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
	out = append(out, Row{Text: border("┌", "┬", "┐"), Region: "table-top"})

	for n, row := range rows {
		var line strings.Builder
		var spans []Span
		column := 2
		line.WriteString(paint(p.Border, "│"))
		for i, cell := range row {
			flag := ""
			if n == 0 {
				flag = ";1"
			}
			rendered := transform(inline(cell, p, flag, 0))
			spans = append(spans, Span{Column: column, Width: ansi.StringWidth(rendered), Field: i})
			column += widths[i] + 3
			line.WriteString(" " + rendered + strings.Repeat(" ", widths[i]-ansi.StringWidth(rendered)+1) + paint(p.Border, "│"))
		}

		sourceLine := n + 1
		if n == 0 {
			sourceLine = 0
		}
		out = append(out, Row{Text: line.String(), Source: sourceLine, Region: "table", Spans: spans})
		if n == 0 {
			out = append(out, Row{Text: border("├", "┼", "┤"), Source: 1, Region: "table-separator"})
		}
	}

	out = append(out, Row{Text: border("└", "┴", "┘"), Source: consumed - 1, Region: "table-bottom"})
	return out, consumed
}
