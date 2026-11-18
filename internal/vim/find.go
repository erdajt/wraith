package vim

import "strings"

// findPending reports whether a character-search motion is waiting for its target.
func (b *Buffer) findPending() bool {
	p := b.Pending
	return len(p) > 0 && len(p) <= 2 && strings.Contains("fFtT", p[len(p)-1:])
}

// findCharacter moves to the counted target on the current line, applying inclusive or till motion.
func (b *Buffer) findCharacter(motion string, target rune, count int) bool {
	step := 1
	if motion == "F" || motion == "T" {
		step = -1
	}

	for i := b.Pos + step; i >= b.lineStart(b.Pos) && i < b.lineEnd(b.Pos); i += step {
		if b.Text[i] != target {
			continue
		}

		count--
		if count != 0 {
			continue
		}

		b.Pos = i
		if motion == "t" || motion == "T" {
			b.Pos -= step
		}
		return true
	}

	return false
}
