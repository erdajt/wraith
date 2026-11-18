package keymap

import (
	"reflect"
	"testing"
)

func TestNotationPreservesSequencesAndTerminalAliases(t *testing.T) {
	for _, test := range []struct {
		input string
		want  []string
	}{
		{"<C-w>v", []string{"ctrl+w", "v"}},
		{"<C-h><C-j><C-k><C-l>", []string{"ctrl+h", "ctrl+j", "ctrl+k", "ctrl+l"}},
		{"<Space>ca", []string{" ", "c", "a"}},
		{"<S-Tab><A-e><CR><C-i><C-[>", []string{"shift+tab", "alt+e", "enter", "tab", "esc"}},
		{"<A-A>", []string{"alt+A"}},
		{"Y界<lt><F12>", []string{"Y", "界", "<", "f12"}},
	} {
		t.Run(test.input, func(t *testing.T) {
			got, err := Parse(test.input)
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("parse: %q, %v", got, err)
			}
		})
	}
}

func TestNotationRejectsAmbiguousOrUnboundedInput(t *testing.T) {
	for _, input := range []string{"", "<wat>", "<C-w", "<C-Enter>", "line\nfeed", "abcdefghijklmnopq", "<F21>", "\xff"} {
		if _, err := Parse(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}
