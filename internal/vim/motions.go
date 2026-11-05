package vim

import (
	"strings"
	"unicode"
)

func category(r rune) int {
	if unicode.IsSpace(r) {
		return 0
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
		return 1
	}
	return 2
}
func (b *Buffer) wordBounds() (int, int) {
	if len(b.Text) == 0 {
		return 0, 0
	}
	a, z := min(b.Pos, len(b.Text)-1), min(b.Pos, len(b.Text)-1)
	kind := category(b.Text[a])
	for a > 0 && category(b.Text[a-1]) == kind {
		a--
	}
	for z < len(b.Text) && category(b.Text[z]) == kind {
		z++
	}
	return a, z
}

func (b *Buffer) motion(key string, n int) {
	classify := category
	if key == "W" || key == "B" || key == "E" {
		classify = func(r rune) int {
			if unicode.IsSpace(r) {
				return 0
			}
			return 1
		}
	}

	for range n {
		switch key {
		case "h", "left":
			b.Pos = max(b.lineStart(b.Pos), b.Pos-1)
		case "l", "right":
			b.Pos = min(max(b.lineStart(b.Pos), b.lineEnd(b.Pos)-1), b.Pos+1)
		case "j", "down":
			b.moveVertical(1)
		case "k", "up":
			b.moveVertical(-1)
		case "0", "home":
			b.Pos = b.lineStart(b.Pos)
		case "^":
			b.Pos = b.lineStart(b.Pos)
			for b.Pos < b.lineEnd(b.Pos) && unicode.IsSpace(b.Text[b.Pos]) {
				b.Pos++
			}
		case "$", "end":
			b.Pos = max(b.lineStart(b.Pos), b.lineEnd(b.Pos)-1)
		case "G":
			b.Pos = b.lineStart(len(b.Text))
		case "w", "W":
			if b.Pos < len(b.Text) {
				kind := classify(b.Text[b.Pos])
				for b.Pos < len(b.Text) && classify(b.Text[b.Pos]) == kind {
					b.Pos++
				}
				for b.Pos < len(b.Text) && unicode.IsSpace(b.Text[b.Pos]) {
					b.Pos++
				}
			}
		case "b", "B":
			b.Pos = max(0, b.Pos-1)
			for b.Pos > 0 && unicode.IsSpace(b.Text[b.Pos]) {
				b.Pos--
			}
			if b.Pos < len(b.Text) {
				kind := classify(b.Text[b.Pos])
				for b.Pos > 0 && classify(b.Text[b.Pos-1]) == kind {
					b.Pos--
				}
			}
		case "e", "E":
			b.Pos = min(len(b.Text), b.Pos+1)
			for b.Pos < len(b.Text) && unicode.IsSpace(b.Text[b.Pos]) {
				b.Pos++
			}
			if b.Pos < len(b.Text) {
				kind := classify(b.Text[b.Pos])
				for b.Pos+1 < len(b.Text) && classify(b.Text[b.Pos+1]) == kind {
					b.Pos++
				}
			}
		case "%":
			b.matchPair()
		}
	}
	b.clamp()
}

func (b *Buffer) moveVertical(delta int) {
	start := b.lineStart(b.Pos)
	col := b.Pos - start
	if delta > 0 {
		e := b.lineEnd(b.Pos)
		if e < len(b.Text) {
			b.Pos = min(b.lineEnd(e+1), e+1+col)
		}
	} else if start > 0 {
		s := b.lineStart(start - 1)
		b.Pos = min(start-1, s+col)
	}
	b.clamp()
}

func (b *Buffer) find(reverse bool) {
	needle := []rune(b.Search)
	if len(needle) == 0 {
		return
	}
	step := 1
	if reverse {
		step = -1
	}
	for n := 1; n <= len(b.Text); n++ {
		p := (b.Pos + step*n + len(b.Text)) % len(b.Text)
		if p+len(needle) <= len(b.Text) && string(b.Text[p:p+len(needle)]) == b.Search {
			b.Pos = p
			return
		}
	}
}
func (b *Buffer) matchPair() {
	if b.Pos >= len(b.Text) {
		return
	}
	opens, closes := "([{", ")]}"
	r := b.Text[b.Pos]
	i := strings.IndexRune(opens, r)
	step := 1
	var other rune
	if i >= 0 {
		other = []rune(closes)[i]
	} else {
		i = strings.IndexRune(closes, r)
		if i < 0 {
			return
		}
		step = -1
		other = []rune(opens)[i]
	}
	depth := 1
	for p := b.Pos + step; p >= 0 && p < len(b.Text); p += step {
		if b.Text[p] == r {
			depth++
		}
		if b.Text[p] == other {
			depth--
			if depth == 0 {
				b.Pos = p
				return
			}
		}
	}
}
