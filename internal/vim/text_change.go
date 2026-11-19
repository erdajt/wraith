package vim

// TextChange describes the smallest contiguous replacement between two rune sequences.
type TextChange struct {
	Start, OldEnd, NewEnd int
}

// ChangedText finds a replacement while preserving the unchanged prefix and suffix.
func ChangedText(before, after []rune) TextChange {
	change := TextChange{OldEnd: len(before), NewEnd: len(after)}
	for change.Start < min(len(before), len(after)) && before[change.Start] == after[change.Start] {
		change.Start++
	}
	for change.OldEnd > change.Start && change.NewEnd > change.Start && before[change.OldEnd-1] == after[change.NewEnd-1] {
		change.OldEnd--
		change.NewEnd--
	}
	return change
}

// Rebase leaves insertion anchors before new text and collapses deleted anchors to its start.
func (change TextChange) Rebase(position int) int {
	if position <= change.Start {
		return position
	}
	if position < change.OldEnd {
		return change.Start
	}
	return position + change.NewEnd - change.OldEnd
}
