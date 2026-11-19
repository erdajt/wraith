package ui

import (
	"strconv"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/vim"
	"github.com/rivo/uniseg"
)

type reader struct {
	Buffer *vim.Buffer
	Offset int
	Frozen bool
}

// newReader creates a read-only text viewer with a normal-mode vim cursor.
func newReader(text string) *reader {
	b := vim.New()
	b.Text = []rune(text)
	b.Mode = "normal"
	return &reader{Buffer: b}
}

// key handles reader motions and returns copied text or a request to leave the viewer.
func (r *reader) key(k tea.KeyMsg, height int) (string, bool) {
	b := r.Buffer
	r.Frozen = true

	if k.Type == tea.KeyRunes && len(k.Runes) > 1 && !k.Paste {
		copied := ""
		for _, ch := range k.Runes {
			text, exit := r.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}}, height)
			if text != "" {
				copied = text
			}
			if exit {
				return copied, true
			}
		}
		return copied, false
	}

	key := k.String()
	if b.Mode == "search" {
		b.Key(key, string(k.Runes))
		return "", false
	}

	if (key == "q" || key == "i" || key == "esc") && b.Mode == "normal" && b.Pending == "" {
		return "", true
	}
	if key == "esc" {
		b.Key("esc", "")
		return "", false
	}

	pending := b.Pending
	switch key {
	case "ctrl+d", "pgdown", "ctrl+f", "ctrl+u", "pgup", "ctrl+b":
		count, _ := strconv.Atoi(b.Count)
		count = max(1, count)
		step := max(1, height/2)
		if key == "ctrl+f" || key == "ctrl+b" {
			step = height
		}
		b.Count = strconv.Itoa(count * step)
		key = "j"
		if k.String() == "ctrl+u" || k.String() == "pgup" || k.String() == "ctrl+b" {
			key = "k"
		}
	default:
		allowed := strings.Contains("hjklwbeWBE0^$gG{}%fFtTvyVY/nN123456789", key) && len(key) == 1
		allowed = allowed || key == "up" || key == "down" || key == "left" || key == "right" || key == "home" || key == "end"
		allowed = allowed || (len(pending) > 0 && strings.Contains("fFtT", pending[len(pending)-1:])) || (pending == "y" && (key == "i" || key == "a"))
		if !allowed || k.Paste {
			return "", false
		}
	}

	b.Key(key, key)
	if b.Yanked && len(b.Register) > 0 {
		return string(b.Register), false
	}
	return "", false
}

// lines renders the cursor's viewport with search and visual-selection highlights.
func (r *reader) lines(width, height int, p palette) []string {
	return r.styledLines(width, height, p, true, nil)
}

// styledLines applies cached line styles beneath the reader's search, selection and cursor.
func (r *reader) styledLines(width, height int, p palette, focused bool, style func(int, string) string) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	b := r.Buffer
	text := strings.Split(string(b.Text), "\n")
	row, lineStart, position := 0, 0, 0
	for i, ch := range b.Text[:min(b.Pos, len(b.Text))] {
		if ch == '\n' {
			row++
			lineStart = i + 1
		}
	}
	column, _ := runeDisplayRange(text[row], b.Pos-lineStart, b.Pos-lineStart)

	if row < r.Offset {
		r.Offset = row
	}
	if row >= r.Offset+height {
		r.Offset = row - height + 1
	}
	r.Offset = max(0, min(r.Offset, max(0, len(text)-height)))
	left, right := b.Selection()
	out := make([]string, 0, height)
	for i, line := range text {
		size := len([]rune(line))
		if i >= r.Offset && i < r.Offset+height {
			styled := ink(p.FG, line)
			if style != nil {
				styled = style(i, line)
			}
			styled = highlightMatches(styled, b.SearchQuery(), p)
			if left >= 0 && right > position && left <= position+size {
				runes := []rune(line)
				start := max(0, left-position)
				end := min(size, right-position)
				if start <= end {
					from, to := runeDisplayRange(string(runes), start, end)
					styled = highlightRange(styled, from, to, p.Second, p)
				}
			}

			if focused && i == row {
				if b.Mode == "insert" && column == ansi.StringWidth(styled) {
					styled += " "
				}
				styled = cursorText(styled, column, p)
			}

			horizontal := 0
			if i == row && column >= width {
				horizontal = column - width + 1
			}
			out = append(out, fit(ansi.Cut(styled, horizontal, horizontal+width), width))
		}
		position += size + 1
	}
	for len(out) < height {
		out = append(out, strings.Repeat(" ", width))
	}
	return out
}

// runeDisplayRange expands rune bounds to complete grapheme cells for cursor and selection rendering.
func runeDisplayRange(text string, start, end int) (int, int) {
	left, right, position, cells := -1, -1, 0, 0
	clusters := uniseg.NewGraphemes(text)
	for clusters.Next() {
		next := position + utf8.RuneCountInString(clusters.Str())
		if left < 0 && start < next {
			left = cells
		}
		if right < 0 && end <= next {
			right = cells
			if end > position {
				right += clusters.Width()
			}
		}
		position, cells = next, cells+clusters.Width()
	}
	if left < 0 {
		left = cells
	}
	if right < 0 {
		right = cells
	}
	return left, right
}
