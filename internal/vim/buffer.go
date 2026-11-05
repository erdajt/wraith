package vim

import "strings"

type snapshot struct {
	Text []rune
	Pos  int
}
type Buffer struct {
	YankStart, YankEnd int
	Text               []rune
	Pos                int
	Mode               string
	Anchor             int
	Register           []rune
	Linewise           bool
	Pending            string
	Count              string
	Search             string
	query              string
	undo, redo         []snapshot
}

func New() *Buffer               { return &Buffer{Mode: "insert"} }
func (b *Buffer) String() string { return string(b.Text) }
func (b *Buffer) Set(s string) {
	b.save()
	b.Text = []rune(s)
	b.Pos = len(b.Text)
	b.Mode = "insert"
	b.Pending = ""
}
func (b *Buffer) save() {
	b.undo = append(b.undo, snapshot{append([]rune(nil), b.Text...), b.Pos})
	if len(b.undo) > 100 {
		b.undo = b.undo[1:]
	}
	b.redo = nil
}
func (b *Buffer) clamp() {
	b.Pos = max(0, min(b.Pos, len(b.Text)))
	if b.Mode != "insert" && b.Pos == len(b.Text) && b.Pos > 0 && b.Text[b.Pos-1] != '\n' {
		b.Pos--
	}
}
func (b *Buffer) lineStart(p int) int {
	p = min(p, len(b.Text))
	for p > 0 && b.Text[p-1] != '\n' {
		p--
	}
	return p
}
func (b *Buffer) lineEnd(p int) int {
	p = min(p, len(b.Text))
	for p < len(b.Text) && b.Text[p] != '\n' {
		p++
	}
	return p
}
func (b *Buffer) Insert(s string) {
	b.save()
	r := []rune(s)
	b.Text = append(b.Text[:b.Pos], append(r, append([]rune(nil), b.Text[b.Pos:]...)...)...)
	b.Pos += len(r)
}
func (b *Buffer) remove(a, z int) {
	a = max(0, min(a, len(b.Text)))
	z = max(a, min(z, len(b.Text)))
	b.save()
	b.Register = append([]rune(nil), b.Text[a:z]...)
	b.Text = append(b.Text[:a], b.Text[z:]...)
	b.Pos = a
	b.clamp()
}

func (b *Buffer) restore(redo bool) {
	source, dest := &b.undo, &b.redo
	if redo {
		source, dest = &b.redo, &b.undo
	}
	if len(*source) == 0 {
		return
	}
	*dest = append(*dest, snapshot{append([]rune(nil), b.Text...), b.Pos})
	v := (*source)[len(*source)-1]
	*source = (*source)[:len(*source)-1]
	b.Text = v.Text
	b.Pos = v.Pos
	b.clamp()
}
func (b *Buffer) Selection() (int, int) {
	if b.Mode == "visual" {
		return min(b.Anchor, b.Pos), max(b.Anchor, b.Pos) + 1
	}
	if b.Mode == "visual-line" {
		return b.lineStart(min(b.Anchor, b.Pos)), min(len(b.Text), b.lineEnd(max(b.Anchor, b.Pos))+1)
	}
	return -1, -1
}
func (b *Buffer) Status() string {
	if b.Mode == "search" {
		return "/" + b.query
	}
	return strings.ToUpper(b.Mode) + " " + b.Count + b.Pending
}
