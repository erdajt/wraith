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
	change := ChangedText(b.Text, next)
	left, right := b.Selection()
	if left >= 0 && change.Start < right && change.OldEnd > left {
		b.Mode = "normal"
	}
	b.Pos, b.Anchor = change.Rebase(b.Pos), change.Rebase(b.Anchor)
	b.Text = next
	b.Pending, b.Count, b.OperatorCount = "", "", ""
	b.vertical, b.Yanked = false, false
	b.clamp()
	b.Anchor = max(0, min(b.Anchor, len(b.Text)))
}
