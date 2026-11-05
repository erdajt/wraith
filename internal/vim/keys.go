package vim

import (
	"strconv"
	"strings"
	"unicode"
)

func (b *Buffer) Key(key, text string) {
	if key == "esc" {
		if b.Mode == "insert" && b.Pos > b.lineStart(b.Pos) {
			b.Pos--
		}
		b.Mode = "normal"
		b.Pending = ""
		b.Count = ""
		b.clamp()
		return
	}
	if b.Mode == "search" {
		switch key {
		case "enter":
			b.Search = b.query
			b.Mode = "normal"
			b.find(false)
		case "backspace":
			r := []rune(b.query)
			if len(r) > 0 {
				b.query = string(r[:len(r)-1])
			}
		default:
			b.query += text
		}
		return
	}
	if b.Mode == "insert" {
		switch key {
		case "backspace":
			if b.Pos > 0 {
				b.remove(b.Pos-1, b.Pos)
			}
		case "delete":
			if b.Pos < len(b.Text) {
				b.remove(b.Pos, b.Pos+1)
			}
		case "enter":
			b.Insert("\n")
		case "tab":
			b.Insert("    ")
		case "left":
			b.Pos = max(0, b.Pos-1)
		case "right":
			b.Pos = min(len(b.Text), b.Pos+1)
		case "up":
			b.moveVertical(-1)
		case "down":
			b.moveVertical(1)
		case "home":
			b.Pos = b.lineStart(b.Pos)
		case "end":
			b.Pos = b.lineEnd(b.Pos)
		default:
			if text != "" {
				b.Insert(text)
			}
		}
		return
	}
	if len(text) > 1 {
		b.Insert(text)
		b.clamp()
		return
	}
	if key == "ctrl+r" {
		b.restore(true)
		return
	}
	if key >= "0" && key <= "9" && (key != "0" || b.Count != "") {
		if len(b.Count) < 4 {
			b.Count += key
		}
		return
	}
	n := 1
	if b.Count != "" {
		n, _ = strconv.Atoi(b.Count)
		n = max(1, min(n, 9999))
	}
	if b.pendingKey(key, text, n) {
		return
	}
	b.Count = ""
	if b.Mode == "visual" || b.Mode == "visual-line" {
		if key == "d" || key == "c" || key == "y" {
			a, z := min(b.Anchor, b.Pos), max(b.Anchor, b.Pos)+1
			line := b.Mode == "visual-line"
			if line {
				a = b.lineStart(a)
				z = min(len(b.Text), b.lineEnd(z-1)+1)
			}
			b.apply(key, a, z, line)
			return
		}
	}
	switch key {
	case "i":
		b.Mode = "insert"
	case "a":
		b.Pos = min(b.lineEnd(b.Pos), b.Pos+1)
		b.Mode = "insert"
	case "I":
		b.Pos = b.lineStart(b.Pos)
		b.Mode = "insert"
	case "A":
		b.Pos = b.lineEnd(b.Pos)
		b.Mode = "insert"
	case "o":
		b.Pos = b.lineEnd(b.Pos)
		b.Insert("\n")
		b.Mode = "insert"
	case "O":
		b.Pos = b.lineStart(b.Pos)
		b.Insert("\n")
		b.Pos--
		b.Mode = "insert"
	case "v":
		if b.Mode == "visual" {
			b.Mode = "normal"
		} else {
			b.Mode = "visual"
			b.Anchor = b.Pos
		}
	case "V":
		b.Mode = "visual-line"
		b.Anchor = b.Pos
	case "d", "c", "y", "g", "r", "f", "F", "t", "T":
		b.Pending = key
		if n > 1 {
			b.Count = strconv.Itoa(n)
		}
	case "x":
		b.Linewise = false
		b.remove(b.Pos, min(b.lineEnd(b.Pos), b.Pos+n))
	case "X":
		b.Linewise = false
		b.remove(max(b.lineStart(b.Pos), b.Pos-n), b.Pos)
	case "s":
		b.remove(b.Pos, min(b.lineEnd(b.Pos), b.Pos+n))
		b.Mode = "insert"
	case "D", "C":
		b.apply("d", b.Pos, b.lineEnd(b.Pos), false)
		if key == "C" {
			b.Mode = "insert"
		}
	case "S":
		b.apply("c", b.lineStart(b.Pos), b.lineEnd(b.Pos), false)
	case "p", "P":
		if len(b.Register) > 0 {
			r := append([]rune(nil), b.Register...)
			if b.Linewise {
				if key == "p" {
					b.Pos = b.lineEnd(b.Pos)
					if b.Pos < len(b.Text) {
						b.Pos++
					} else {
						b.Insert("\n")
					}
				} else {
					b.Pos = b.lineStart(b.Pos)
				}
			} else if key == "p" {
				b.Pos = min(len(b.Text), b.Pos+1)
			}
			b.Insert(strings.Repeat(string(r), n))
			b.clamp()
		}
	case "u":
		for range n {
			b.restore(false)
		}
	case "J":
		p := b.lineEnd(b.Pos)
		if p < len(b.Text) {
			b.save()
			b.Text[p] = ' '
		}
	case "~":
		if b.Pos < len(b.Text) {
			b.save()
			r := b.Text[b.Pos]
			if unicode.IsUpper(r) {
				b.Text[b.Pos] = unicode.ToLower(r)
			} else {
				b.Text[b.Pos] = unicode.ToUpper(r)
			}
			b.Pos++
			b.clamp()
		}
	case "/":
		b.Mode = "search"
		b.query = ""
	case "n":
		b.find(false)
	case "N":
		b.find(true)
	default:
		b.motion(key, n)
	}
}

func (b *Buffer) apply(op string, a, z int, line bool) {
	b.Linewise = line
	if op == "y" {
		b.Register = append([]rune(nil), b.Text[max(0, a):min(z, len(b.Text))]...)
		b.Pos = a
		b.Mode = "normal"
		b.clamp()
		return
	}
	b.remove(a, z)
	b.Mode = "normal"
	if op == "c" {
		b.Mode = "insert"
	}
	b.clamp()
}
