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
	if strings.Contains("fFtT", b.Pending) && len(b.Pending) == 1 && text != "" {
		forward := b.Pending == "f" || b.Pending == "t"
		step := 1
		if !forward {
			step = -1
		}
		target := []rune(text)[0]
		for i := b.Pos + step; i >= b.lineStart(b.Pos) && i < b.lineEnd(b.Pos); i += step {
			if b.Text[i] == target {
				b.Pos = i
				if b.Pending == "t" || b.Pending == "T" {
					b.Pos -= step
				}
				break
			}
		}
		b.Pending = ""
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
