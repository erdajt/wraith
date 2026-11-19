package vim

import "slices"

// CloneView copies navigation state while sharing the document's undo history.
func (b *Buffer) CloneView() *Buffer {
	if b.history == nil {
		b.history = &history{}
	}
	view := *b
	view.Text = slices.Clone(b.Text)
	view.Register = slices.Clone(b.Register)
	view.Pending, view.Count, view.OperatorCount = "", "", ""
	view.Yanked = false
	return &view
}

// SyncText rebases rune anchors and optionally discards history after an external replacement.
func (b *Buffer) SyncText(text string, resetHistory bool) {
	next := []rune(text)
	if slices.Equal(b.Text, next) {
		return
	}
	if resetHistory && b.history != nil {
		*b.history = history{}
	}
	start := 0
	for start < min(len(b.Text), len(next)) && b.Text[start] == next[start] {
		start++
	}
	oldEnd, nextEnd := len(b.Text), len(next)
	for oldEnd > start && nextEnd > start && b.Text[oldEnd-1] == next[nextEnd-1] {
		oldEnd--
		nextEnd--
	}
	left, right := b.Selection()
	if left >= 0 && start < right && oldEnd > left {
		b.Mode = "normal"
	}
	for _, anchor := range []*int{&b.Pos, &b.Anchor} {
		if *anchor > start {
			if *anchor < oldEnd {
				*anchor = start
			} else {
				*anchor += nextEnd - oldEnd
			}
		}
	}
	b.Text = next
	b.Pending, b.Count, b.OperatorCount = "", "", ""
	b.vertical, b.Yanked = false, false
	b.clamp()
	b.Anchor = max(0, min(b.Anchor, len(b.Text)))
}
