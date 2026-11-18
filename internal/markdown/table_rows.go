package markdown

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// wrapTableRow maps field spans onto wrapped rows while accounting for skipped whitespace.
func wrapTableRow(row Row, width int) []Row {
	plain := ansi.Strip(row.Text)
	used := 0
	var rows []Row
	for _, part := range strings.Split(ansi.Wrap(row.Text, width, ""), "\n") {
		text := ansi.Strip(part)
		if at := strings.Index(plain[used:], text); at >= 0 {
			used += at
		}
		start := ansi.StringWidth(plain[:used])
		end := start + ansi.StringWidth(text)
		entry := Row{Text: part, Source: row.Source, Region: row.Region, Cell: start, Part: row.Part}
		for _, span := range row.Spans {
			left, right := max(start, span.Column), min(end, span.Column+span.Width)
			if left < right {
				entry.Spans = append(entry.Spans, Span{Column: left - start, Width: right - left, Field: span.Field, Cell: span.Cell + left - span.Column})
			}
		}
		rows = append(rows, entry)
		used += len(text)
	}
	return rows
}
