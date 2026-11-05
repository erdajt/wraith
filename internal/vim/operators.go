package vim

import (
	"strings"
	"unicode"
)

func (b *Buffer) pendingKey(key, text string, n int) bool {
	if b.Pending == "r" {
		if text != "" && b.Pos < len(b.Text) {
			b.save()
			b.Text[b.Pos] = []rune(text)[0]
		}
		b.Pending = ""
		return true
	}
	if b.findPending() && text != "" {
		pending := b.Pending
		motion := pending[len(pending)-1:]
		start := b.Pos
		found := b.findCharacter(motion, []rune(text)[0], n)
		b.Pending, b.Count = "", ""
		if len(pending) == 2 && found {
			end := b.Pos
			b.Pos = start
			b.apply(pending[:1], min(start, end), min(len(b.Text), max(start, end)+1), false)
		}
		return true
	}
	if strings.HasSuffix(b.Pending, "i") || strings.HasSuffix(b.Pending, "a") {
		if key == "w" {
			a, z := b.wordBounds()
			if strings.HasSuffix(b.Pending, "a") {
				for z < len(b.Text) && unicode.IsSpace(b.Text[z]) {
					z++
				}
			}
			op := b.Pending[:1]
			b.Pending = ""
			b.apply(op, a, z, false)
		} else {
			b.Pending = ""
		}
		b.Count = ""
		return true
	}
	if b.Pending == "d" || b.Pending == "c" || b.Pending == "y" {
		op := b.Pending
		if len(key) == 1 && strings.Contains("fFtT", key) {
			b.Pending += key
			return true
		}
		if key == "i" || key == "a" {
			b.Pending += key
			return true
		}
		b.Pending = ""
		b.Count = ""
		if key == op {
			a, z := b.lineStart(b.Pos), b.lineEnd(b.Pos)
			for range n - 1 {
				if z < len(b.Text) {
					z = b.lineEnd(z + 1)
				}
			}
			if z < len(b.Text) {
				z++
			}
			b.apply(op, a, z, true)
			return true
		}
		start := b.Pos
		if op == "c" && (key == "w" || key == "W") && start < len(b.Text) && !unicode.IsSpace(b.Text[start]) {
			if key == "w" {
				key = "e"
			} else {
				key = "E"
			}
		}
		b.motion(key, n)
		end := b.Pos
		b.Pos = start
		if end < start {
			start, end = end, start
		} else if key == "e" || key == "E" || key == "$" {
			end = min(len(b.Text), end+1)
		}
		b.apply(op, start, end, false)
		return true
	}
	if b.Pending == "g" {
		b.Pending = ""
		if key == "g" {
			b.Pos = 0
			for range n - 1 {
				if e := b.lineEnd(b.Pos); e < len(b.Text) {
					b.Pos = e + 1
				}
			}
		}
		b.Count = ""
		return true
	}
	return false
}
