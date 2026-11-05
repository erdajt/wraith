package terminal

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTerminalKeysPreserveSpacesAndBracketedPaste(t *testing.T) {
	tests := []struct {
		key       tea.KeyMsg
		bracketed bool
		want      string
	}{
		{tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}, false, " "},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("echo test\n"), Paste: true}, true, "\x1b[200~echo test\n\x1b[201~"},
		{tea.KeyMsg{Type: tea.KeyCtrlC}, false, "\x03"},
		{tea.KeyMsg{Type: tea.KeyEnter}, false, "\r"},
		{tea.KeyMsg{Type: tea.KeyEsc, Alt: true}, false, "\x1b\x1b"},
	}
	for _, tt := range tests {
		if got := string(Key(tt.key, false, tt.bracketed)); got != tt.want {
			t.Fatalf("key %s: got %q, want %q", tt.key, got, tt.want)
		}
	}
}
