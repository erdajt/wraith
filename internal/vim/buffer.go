package vim

import "strings"

type snapshot struct {
	Text []rune
	Pos  int
}

type Buffer struct {
	Yanked             bool
	YankStart, YankEnd int
	Text               []rune
	Pos                int
	Mode               string
	Anchor             int
	Register           []rune
	Linewise           bool
	Pending            string
	Count              string
	OperatorCount      string
	Search             string
	query              string
	undo, redo         []snapshot
	vertical           bool
	preferredColumn    int
}

// New creates an empty buffer in insert mode.
func New() *Buffer { return &Buffer{Mode: "insert"} }

// String returns the buffer's text without editor state.
func (b *Buffer) String() string { return string(b.Text) }

// Set replaces text, saving undo state and entering insert mode at the end.
func (b *Buffer) Set(s string) {
	b.vertical = false
	b.save()
	b.Text = []rune(s)
	b.Pos = len(b.Text)
	b.Mode = "insert"
	b.Pending = ""
	b.Count, b.OperatorCount = "", ""
}

// save records a bounded undo snapshot and clears redo history.
func (b *Buffer) save() {
	b.undo = append(b.undo, snapshot{append([]rune(nil), b.Text...), b.Pos})
	if len(b.undo) > 100 {
		b.undo = b.undo[1:]
	}

	b.redo = nil
}

// clamp keeps the cursor within the positions allowed by the current mode.
func (b *Buffer) clamp() {
	b.Pos = max(0, min(b.Pos, len(b.Text)))
	if b.Mode != "insert" && b.Pos == len(b.Text) && b.Pos > 0 && b.Text[b.Pos-1] != '\n' {
		b.Pos--
	}
}

// lineStart finds the first rune position on the line containing p.
func (b *Buffer) lineStart(p int) int {
	p = min(p, len(b.Text))
	for p > 0 && b.Text[p-1] != '\n' {
		p--
	}
	return p
}

// lineEnd finds the newline or end-of-buffer position after p.
func (b *Buffer) lineEnd(p int) int {
	p = min(p, len(b.Text))
	for p < len(b.Text) && b.Text[p] != '\n' {
		p++
	}
	return p
}

// Insert inserts text at the cursor and records undo state.
func (b *Buffer) Insert(s string) {
	b.vertical = false
	b.save()
	r := []rune(s)
	b.Text = append(b.Text[:b.Pos], append(r, append([]rune(nil), b.Text[b.Pos:]...)...)...)
	b.Pos += len(r)
}

// remove deletes a bounded rune range and saves it in the yank register.
func (b *Buffer) remove(a, z int) {
	b.vertical = false
	a = max(0, min(a, len(b.Text)))
	z = max(a, min(z, len(b.Text)))
	b.save()
	b.Register = append([]rune(nil), b.Text[a:z]...)
	b.Text = append(b.Text[:a], b.Text[z:]...)
	b.Pos = a
	b.clamp()
}

// restore moves one snapshot between undo and redo history and restores its cursor.
func (b *Buffer) restore(redo bool) {
	b.vertical = false
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

// Selection returns a half-open rune range, or negative bounds when visual mode is inactive.
func (b *Buffer) Selection() (int, int) {
	if b.Mode == "visual" {
		return min(b.Anchor, b.Pos), max(b.Anchor, b.Pos) + 1
	}

	if b.Mode == "visual-line" {
		return b.lineStart(min(b.Anchor, b.Pos)), min(len(b.Text), b.lineEnd(max(b.Anchor, b.Pos))+1)
	}

	return -1, -1
}

// Status formats the current mode, pending keys or active search for display.
func (b *Buffer) Status() string {
	if b.Mode == "search" {
		return "/" + b.query
	}
	if b.OperatorCount != "" {
		return strings.ToUpper(b.Mode) + " " + b.OperatorCount + b.Pending + b.Count
	}
	return strings.ToUpper(b.Mode) + " " + b.Count + b.Pending
}

// SearchQuery returns the active search draft or the last committed query.
func (b *Buffer) SearchQuery() string {
	if b.Mode == "search" {
		return b.query
	}
	return b.Search
}
