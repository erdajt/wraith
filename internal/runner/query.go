package runner

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

type Query struct {
	IgnoreCase bool   `json:"ignore_case,omitempty"`
	Pattern    string `json:"pattern,omitempty"`
	Regex      bool   `json:"regex,omitempty"`
	From       int64  `json:"from,omitempty"`
	Before     int    `json:"before,omitempty"`
	After      int    `json:"after,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

type OutputLine = session.OutputLine
type Page = session.OutputPage

// ReadOutput returns a bounded, redacted search window only when output sharing is allowed.
func ReadOutput(record Record, query Query, snapshot variables.Snapshot) (Page, error) {
	page := Page{Lines: []OutputLine{}, Truncated: record.Truncated}

	if !record.ShareOutput || record.OutputHidden {
		return page, errors.New("output is private; user sharing is required")
	}

	if len(query.Pattern) > 4096 || query.From < 0 || query.Before < 0 || query.After < 0 || query.Before > 10 || query.After > 10 || query.Limit < 0 || query.Limit > 200 {
		return page, errors.New("invalid output query bounds")
	}
	if query.Limit == 0 {
		query.Limit = 80
	}

	pattern := query.Pattern
	if query.IgnoreCase {
		pattern = strings.ToLower(pattern)
	}
	match := func(text string) bool {
		if query.IgnoreCase {
			text = strings.ToLower(text)
		}
		return strings.Contains(text, pattern)
	}

	if query.Regex {
		pattern := query.Pattern
		if query.IgnoreCase {
			pattern = "(?i)" + pattern
		}
		expression, err := regexp.Compile(pattern)
		if err != nil {
			return page, errors.New("invalid output search expression")
		}
		match = expression.MatchString
	}

	lines := map[int64]string{}
	parts := []struct {
		text                string
		line                int64
		skipFirst, complete bool
	}{
		{record.Output, 1, false, !record.Truncated && !record.Finished.IsZero()},
	}
	if record.TailOffset > 0 {
		parts = append(parts, struct {
			text                string
			line                int64
			skipFirst, complete bool
		}{record.Tail, record.TailLine, true, !record.Finished.IsZero()})
	}

	for _, part := range parts {
		for part.text != "" {
			line, rest, terminated := strings.Cut(part.text, "\n")
			part.text = rest
			if !terminated && !part.complete {
				break
			}

			if !part.skipFirst {
				if len(line) > 1<<20 {
					line = "[overlong output line omitted]"
				} else {
					line = ansi.Strip(line)
					line = strings.Map(func(r rune) rune {
						if unicode.IsControl(r) && r != '\t' {
							return -1
						}
						return r
					}, line)
					line = snapshot.Redact(line)
					if len(line) > 16<<10 {
						line = "[line exceeds response budget; inspect local history]"
					}
				}
				lines[part.line] = line
			}

			part.skipFirst = false
			part.line++
		}
	}

	page.Stats = session.OutputStats{ObservedBytes: max(record.Observed, int64(len(record.Output))), PrefixBytes: len(record.Output), TailBytes: len(record.Tail), RetainedLines: len(lines)}
	if !record.Started.IsZero() {
		finished := record.Finished
		if finished.IsZero() {
			finished = time.Now()
		}
		page.Stats.ElapsedMS = max(0, finished.Sub(record.Started).Milliseconds())
	}
	if record.TailOffset > int64(len(record.Output)) {
		page.Stats.GapFrom, page.Stats.GapTo = int64(len(record.Output)), record.TailOffset
	}

	var numbers []int64
	for number := range lines {
		numbers = append(numbers, number)
	}
	sort.Slice(numbers, func(i, j int) bool { return numbers[i] < numbers[j] })

	selected := map[int64]bool{}
	for _, number := range numbers {
		if match(lines[number]) {
			page.Stats.MatchedLines++
			for n := number - int64(query.Before); n <= number+int64(query.After); n++ {
				selected[n] = true
			}
		}
	}

	size := 0
	for _, number := range numbers {
		if number < query.From || !selected[number] {
			continue
		}

		text := lines[number]
		if len(page.Lines) >= query.Limit || size+len(text) > 16<<10 {
			page.Next = number
			break
		}
		page.Lines = append(page.Lines, OutputLine{Number: number, Text: text})
		size += len(text)
	}

	return page, nil
}
