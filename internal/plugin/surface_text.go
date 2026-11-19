package plugin

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// validateSurfaceLines rejects terminal controls and bounds complete lines before display storage.
func validateSurfaceLines(lines []string, maximum int) (int, error) {
	if len(lines) > 100000 {
		return 0, errors.New("surface text exceeds 100000 lines")
	}
	size := 0
	for i, line := range lines {
		if !utf8.ValidString(line) || len(line) > 64<<10 {
			return 0, fmt.Errorf("surface line %d is invalid utf-8 or exceeds 64 kib", i)
		}
		for _, r := range line {
			if unicode.IsControl(r) && r != '\t' {
				return 0, fmt.Errorf("surface line %d contains a terminal control", i)
			}
		}
		size += len(line) + 1
		if size > maximum {
			return 0, fmt.Errorf("surface text exceeds %d bytes", maximum)
		}
	}
	return size, nil
}

// validateSurfaceSpans validates half-open rune ranges and typed styles against their actual text.
func validateSurfaceSpans(lines []string, spans []SurfaceSpan) error {
	if len(spans) > 4096 {
		return errors.New("surface frame exceeds 4096 spans")
	}
	for i, span := range spans {
		for _, position := range []SurfacePosition{span.Start, span.End} {
			if position.Line < 0 || position.Line > len(lines) || position.Column < 0 {
				return fmt.Errorf("surface span %d has an invalid position", i)
			}
			if position.Line == len(lines) {
				if position.Column != 0 {
					return fmt.Errorf("surface span %d extends beyond text", i)
				}
			} else if position.Column > utf8.RuneCountInString(lines[position.Line]) {
				return fmt.Errorf("surface span %d extends beyond its line", i)
			}
		}
		if span.Start.Line > span.End.Line || span.Start.Line == span.End.Line && span.Start.Column >= span.End.Column {
			return fmt.Errorf("surface span %d requires a nonempty forward range", i)
		}
		for _, color := range []string{span.Style.Foreground, span.Style.Background} {
			if slices.Contains([]string{"", "bg", "fg", "muted", "accent", "second", "border", "warn", "variable", "hint_quit", "hint_refresh", "hint_cancel"}, color) {
				continue
			}
			if len(color) != 7 || !strings.HasPrefix(color, "#") {
				return fmt.Errorf("surface span %d has an invalid color", i)
			}
			if _, err := strconv.ParseUint(color[1:], 16, 24); err != nil {
				return fmt.Errorf("surface span %d has an invalid color", i)
			}
		}
	}
	return nil
}
