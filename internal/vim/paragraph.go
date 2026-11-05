package vim

import "strings"

func (b *Buffer) paragraph(direction int) {
	blank := func(p int) bool { return strings.TrimSpace(string(b.Text[b.lineStart(p):b.lineEnd(p)])) == "" }
	p := b.lineStart(b.Pos)
	if direction > 0 {
		for p < len(b.Text) && blank(p) {
			p = min(len(b.Text), b.lineEnd(p)+1)
		}
		for p < len(b.Text) && !blank(p) {
			p = min(len(b.Text), b.lineEnd(p)+1)
		}
	} else {
		if p > 0 {
			p = b.lineStart(p - 1)
		}
		for p > 0 && blank(p) {
			p = b.lineStart(p - 1)
		}
		for p > 0 && !blank(p) {
			p = b.lineStart(p - 1)
		}
	}
	b.Pos = p
}
