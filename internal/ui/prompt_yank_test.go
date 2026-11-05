package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPromptYanksReachClipboardAndRegister(t *testing.T) {
	for _, sequence := range []string{"Y", "yy", "ye", "yf(", "v3ly"} {
		t.Run(sequence, func(t *testing.T) {
			a := fixture(t)
			path := filepath.Join(t.TempDir(), "clipboard")
			a.Config.Clipboard = []string{"/bin/sh", "-c", "cat > \"$1\"", "copy", path}
			a.Prompt.Set("call(args)")
			a.Prompt.Mode, a.Prompt.Pos = "normal", 0
			var copied bool
			for _, r := range sequence {
				if cmd := a.key(key(string(r))); cmd != nil {
					a.Update(cmd())
					copied = true
				}
			}
			if !copied {
				t.Fatal("yank did not invoke clipboard")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != string(a.Prompt.Register) {
				t.Fatalf("clipboard %q: %v", got, err)
			}
			if a.Prompt.String() != "call(args)" {
				t.Fatal("yank changed prompt")
			}
		})
	}
}

func TestFindColonAndCancelledYanksPreserveClipboard(t *testing.T) {
	a := fixture(t)
	a.Config.Clipboard = nil
	a.Prompt.Set("abc:def")
	a.Prompt.Mode, a.Prompt.Pos = "normal", 0
	for _, r := range "yf:" {
		a.key(key(string(r)))
	}
	if a.commandMode || string(a.Prompt.Register) != "abc:" {
		t.Fatal("colon opened command mode during yank")
	}
	for _, end := range []string{"esc", "z"} {
		a.key(key("y"))
		if cmd := a.key(key(end)); cmd != nil {
			t.Fatal("cancelled or unsupported yank copied old register")
		}
	}
	r := newReader("abc:def")
	r.Buffer.Register = []rune("previous")
	r.key(key("y"), 5)
	if copied, _ := r.key(key("z"), 5); copied != "" {
		t.Fatal("failed reader yank copied stale register")
	}
}
