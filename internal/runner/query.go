package runner

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/regexutil"
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
	return ReadOutputContext(context.Background(), record, query, snapshot)
}

// ReadOutputContext checks sharing before a cancellable, redacted output search.
func ReadOutputContext(ctx context.Context, record Record, query Query, snapshot variables.Snapshot) (Page, error) {
	if !record.ShareOutput || record.OutputHidden {
		return Page{Lines: []OutputLine{}, Truncated: record.Truncated}, errors.New("output is private; user sharing is required")
	}
	return readOutput(ctx, record, query, snapshot)
}

// ReadLocalOutput inspects trusted local output without changing model-sharing policy.
func ReadLocalOutput(record Record, query Query) (Page, error) {
	return ReadLocalOutputContext(context.Background(), record, query)
}

// ReadLocalOutputContext supports cancellation while inspecting local command output.
func ReadLocalOutputContext(ctx context.Context, record Record, query Query) (Page, error) {
	return readOutput(ctx, record, query, variables.Snapshot{})
}

// readOutput applies the shared bounded search and display normalization rules.
func readOutput(ctx context.Context, record Record, query Query, snapshot variables.Snapshot) (Page, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	page := Page{Lines: []OutputLine{}, Truncated: record.Truncated}

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

	regexWeight := 0
	if query.Regex {
		pattern := query.Pattern
		if query.IgnoreCase {
			pattern = "(?i)" + pattern
		}
		size, err := regexutil.Size(pattern)
		if err != nil {
			return page, err
		}
		regexWeight = size
		expression, err := regexp.Compile(pattern)
		if err != nil {
			return page, errors.New("invalid output search expression")
		}
		match = expression.MatchString
	}

	cursors := [2]outputCursor{
		{text: record.Output, line: 1, complete: !record.Truncated && !record.Finished.IsZero()},
		{line: 1},
	}
	if record.TailOffset > 0 {
		cursors[1] = outputCursor{text: record.Tail, line: record.TailLine, skipFirst: true, complete: !record.Finished.IsZero()}
	}

	page.Stats = session.OutputStats{ObservedBytes: max(record.Observed, int64(len(record.Output))), PrefixBytes: len(record.Output), TailBytes: len(record.Tail)}
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

	var heads [2]OutputLine
	var available [2]bool
	for index := range cursors {
		var err error
		heads[index], available[index], err = cursors[index].next(ctx)
		if err != nil {
			return Page{}, err
		}
	}
	before := make([]OutputLine, 0, query.Before)
	after, selected, size, work := int64(0), int64(0), 0, 0
	appendLine := func(line OutputLine) {
		if line.Number < query.From || line.Number <= selected {
			return
		}
		selected = line.Number
		if page.Next != 0 {
			return
		}
		if len(page.Lines) >= query.Limit || size+len(line.Text) > 16<<10 {
			page.Next = line.Number
			return
		}
		line.Text = strings.Clone(line.Text)
		page.Lines = append(page.Lines, line)
		size += len(line.Text)
	}
	for available[0] || available[1] {
		if err := ctx.Err(); err != nil {
			return Page{}, err
		}
		index := 0
		if !available[0] || available[1] && heads[1].Number <= heads[0].Number {
			index = 1
		}
		line := heads[index]
		for candidate := range heads {
			if available[candidate] && heads[candidate].Number == line.Number {
				var err error
				heads[candidate], available[candidate], err = cursors[candidate].next(ctx)
				if err != nil {
					return Page{}, err
				}
			}
		}
		if len(line.Text) > 1<<20 {
			line.Text = "[overlong output line omitted]"
		} else {
			line.Text = ansi.Strip(line.Text)
			line.Text = strings.Map(func(r rune) rune {
				if unicode.IsControl(r) && r != '\t' {
					return -1
				}
				return r
			}, line.Text)
			line.Text = snapshot.Redact(line.Text)
			if len(line.Text) > 16<<10 {
				line.Text = "[line exceeds response budget; inspect local history]"
			}
		}
		page.Stats.RetainedLines++
		if regexWeight > 0 {
			if len(line.Text)+1 > ((64<<20)-work)/regexWeight {
				return Page{}, errors.New("output search exceeds regex work limit; use a literal pattern")
			}
			work += (len(line.Text) + 1) * regexWeight
		}
		if match(line.Text) {
			page.Stats.MatchedLines++
			for _, previous := range before {
				if previous.Number >= line.Number-int64(query.Before) {
					appendLine(previous)
				}
			}
			appendLine(line)
			after = line.Number + int64(query.After)
		} else if line.Number <= after {
			appendLine(line)
		}
		if query.Before > 0 {
			if len(before) == query.Before {
				copy(before, before[1:])
				before = before[:len(before)-1]
			}
			before = append(before, line)
		}
	}
	return page, ctx.Err()
}
