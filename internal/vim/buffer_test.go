package vim

import (
	"strconv"
	"testing"
)

func TestBufferUndoRestoresUnicode(t *testing.T) {
	b := New()
	b.Set("界a")
	b.Insert("b")
	b.restore(false)

	if b.String() != "界a" || b.Pos != 2 {
		t.Fatalf("undo: %q at %d", b.String(), b.Pos)
	}

	b.restore(true)

	if b.String() != "界ab" {
		t.Fatal("redo lost inserted text")
	}

	b.Mode = "visual"
	b.Anchor = 0
	b.Pos = 1
	a, z := b.Selection()
	if a != 0 || z != 2 {
		t.Fatal("selection must use rune positions")
	}
}
func keys(b *Buffer, s ...string) {
	for _, k := range s {
		b.Key(k, k)
	}
}
func normal(s string) *Buffer { b := New(); b.Set(s); b.Mode = "normal"; b.Pos = 0; return b }

func TestMotionsOperatorsUndoAndPaste(t *testing.T) {
	b := normal("alpha beta\ngamma delta")
	keys(b, "d", "w")

	if b.String() != "beta\ngamma delta" {
		t.Fatalf("dw: %q", b.String())
	}

	keys(b, "u")

	if b.String() != "alpha beta\ngamma delta" {
		t.Fatal("undo failed")
	}

	keys(b, "w", "c", "i", "w")
	b.Key("runes", "new")
	b.Key("esc", "")

	if b.String() != "alpha new\ngamma delta" {
		t.Fatalf("ciw: %q", b.String())
	}

	keys(b, "g", "g", "y", "y", "G", "p")

	if b.String() != "alpha new\ngamma delta\nalpha new\n" {
		t.Fatalf("line paste: %q", b.String())
	}
}

func TestVisualUnicodeAndSearch(t *testing.T) {
	b := normal("界 alpha beta alpha")
	keys(b, "v", "l", "d")

	if b.String() != "alpha beta alpha" {
		t.Fatalf("visual delete: %q", b.String())
	}

	keys(b, "/", "a", "l", "p", "h", "a")
	b.Key("enter", "")

	if b.Pos != 11 {
		t.Fatalf("search position: %d", b.Pos)
	}

	keys(b, "n")

	if b.Pos != 0 {
		t.Fatal("search did not wrap")
	}
}

func TestMultilineInsertAndCounts(t *testing.T) {
	b := New()
	b.Key("runes", "hello")
	b.Key("enter", "")
	b.Key("runes", "world")
	b.Key("esc", "")
	keys(b, "g", "g", "2", "d", "d")

	if b.String() != "" {
		t.Fatalf("2dd: %q", b.String())
	}

	keys(b, "u")

	if b.String() != "hello\nworld" {
		t.Fatal("undo multiline failed")
	}
}

func FuzzEditing(f *testing.F) {
	f.Add("hello world", "dwiabc\x1bu")
	f.Add("one two three four five six seven", "2d3wu\x122y3w")
	f.Add("α beta\n界 gamma", "iword\nnext\x1b/界\rnvly")
	controls := map[rune]string{'\x1b': "esc", '\r': "enter", '\n': "enter", '\t': "tab", '\b': "backspace", '\x7f': "delete", '\x12': "ctrl+r"}
	f.Fuzz(func(t *testing.T, text, seq string) {
		if len(text)+len(seq) > 4096 {
			t.Skip()
		}

		b := normal(text)

		for _, r := range seq {
			key, text := string(r), string(r)
			if control, ok := controls[r]; ok {
				key, text = control, ""
			}
			if (key == "p" || key == "P") && b.Mode != "insert" && b.Mode != "search" && b.Pending == "" {
				count, _ := strconv.Atoi(b.Count)
				if len(b.Register) > ((64<<10)-len(b.Text))/max(1, min(count, 9999)) {
					t.Skip("counted paste exceeds fuzz allocation budget")
				}
			}
			b.Key(key, text)

			if b.Pos < 0 || b.Pos > len(b.Text) {
				t.Fatal("cursor out of bounds")
			}
			if left, right := b.Selection(); left >= 0 && (left > right || left > len(b.Text) || right > len(b.Text)+1) {
				t.Fatal("selection out of bounds")
			}
			if len(b.Text) > 64<<10 {
				t.Skip("expanded text exceeds fuzz budget")
			}
		}
	})
}

func TestVisualDeletesAndUndoClearSelection(t *testing.T) {
	for _, tt := range []struct {
		sequence       []string
		text, register string
	}{
		{[]string{"v", "l", "x"}, "pha beta", "al"},
		{[]string{"v", "l", "X"}, "", "alpha beta"},
		{[]string{"A", "runes", "esc", "v", "u"}, "alpha beta", ""},
	} {
		b := normal("alpha beta")
		keys(b, tt.sequence...)
		if b.String() != tt.text || string(b.Register) != tt.register {
			t.Fatalf("%v: text %q, register %q", tt.sequence, b.String(), string(b.Register))
		}
		if b.Mode != "normal" {
			t.Fatalf("%v retained visual mode", tt.sequence)
		}
		if left, right := b.Selection(); left != -1 || right != -1 {
			t.Fatalf("%v retained stale selection: %d:%d", tt.sequence, left, right)
		}
	}
}

func TestLiteralReplaceAndBigWordMotions(t *testing.T) {
	b := normal("abc")
	keys(b, "r", "界")

	if b.String() != "界bc" {
		t.Fatalf("unicode replacement: %q", b.String())
	}

	keys(b, "r", "1")

	if b.String() != "1bc" {
		t.Fatal("numeric replacement became a count")
	}

	b = normal("host.name next")
	keys(b, "W")

	if b.Pos != 10 {
		t.Fatalf("big word split at punctuation: %d", b.Pos)
	}

	b = normal("first next")
	keys(b, "c", "w")
	keys(b, "x", "esc")

	if b.String() != "x next" {
		t.Fatalf("cw consumed trailing space: %q", b.String())
	}
}

func TestVerticalMotionsRetainColumnsAcrossShortLines(t *testing.T) {
	for _, tc := range []struct {
		name, text, mode string
		start, want      int
		input            []string
	}{
		{"short line", "abcd\nx\nwxyz", "normal", 2, 5, []string{"j"}},
		{"down", "abcd\nx\nwxyz", "normal", 2, 9, []string{"j", "j"}},
		{"up", "abcd\nx\nwxyz", "normal", 9, 2, []string{"k", "k"}},
		{"count", "abcd\nx\nwxyz", "normal", 2, 9, []string{"2", "j"}},
		{"empty", "abcd\n\nwxyz", "normal", 2, 8, []string{"j", "j"}},
		{"insert endpoint", "abcd\nx\nwxyz", "insert", 2, 6, []string{"down"}},
		{"horizontal resets", "abcd\nx\nwxyz", "normal", 2, 7, []string{"j", "0", "j"}},
		{"unicode", "界abc\nx\n界xyz", "normal", 2, 9, []string{"j", "j"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := normal(tc.text)
			b.Mode, b.Pos = tc.mode, tc.start
			keys(b, tc.input...)
			if b.Pos != tc.want || b.Mode != tc.mode || b.String() != tc.text {
				t.Fatalf("movement changed state: pos=%d mode=%s text=%q", b.Pos, b.Mode, b.String())
			}
		})
	}
}

func TestVerticalColumnResetsAfterEditsAndLiteralTargets(t *testing.T) {
	for _, edit := range []string{"insert", "undo", "replace", "find", "set"} {
		t.Run(edit, func(t *testing.T) {
			b := normal("abcd\nxk\nwxyz")
			b.Pos = 3
			keys(b, "j")
			switch edit {
			case "insert":
				b.Insert("z")
			case "undo":
				b.Insert("z")
				keys(b, "u")
			case "replace":
				keys(b, "r", "j")
			case "find":
				keys(b, "F", "x")
			case "set":
				b.Set("abcdef\nx\nwxyz")
				b.Mode, b.Pos = "normal", 7
			}
			column := b.Pos - b.lineStart(b.Pos)
			keys(b, "j")
			if b.Pos-b.lineStart(b.Pos) != column {
				t.Fatal("edit or literal argument retained stale desired column")
			}
		})
	}
}

func TestEditingTracesPreserveTextCursorAndMode(t *testing.T) {
	for _, tc := range []struct {
		name, before, after string
		input               []string
		position            int
	}{
		{"append", "abc", "abc!", []string{"A", "!", "esc"}, 3},
		{"prepend", "abc", "!abc", []string{"I", "!", "esc"}, 0},
		{"open below", "abc", "abc\nx", []string{"o", "x", "esc"}, 4},
		{"open above", "abc", "x\nabc", []string{"O", "x", "esc"}, 0},
		{"substitute", "abc", "xbc", []string{"s", "x", "esc"}, 0},
		{"substitute last", "abc", "abx", []string{"2", "l", "s", "x", "esc"}, 2},
		{"change last word", "a bc", "a x", []string{"w", "c", "w", "x", "esc"}, 2},
		{"substitute line", "abc", "x", []string{"S", "x", "esc"}, 0},
		{"change rest", "abc", "ax", []string{"l", "C", "x", "esc"}, 1},
		{"delete rest", "abc", "a", []string{"l", "D"}, 0},
		{"delete before", "abc", "bc", []string{"l", "X"}, 0},
		{"join", "a\nb", "a b", []string{"J"}, 0},
		{"case", "a", "A", []string{"~"}, 0},
		{"undo replace", "abc", "abc", []string{"r", "界", "u"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := normal(tc.before)
			keys(b, tc.input...)
			if b.String() != tc.after || b.Pos != tc.position || b.Mode != "normal" {
				t.Fatalf("text=%q cursor=%d mode=%s", b.String(), b.Pos, b.Mode)
			}
		})
	}
}

func TestReverseLineSelectionAndNestedPairMotions(t *testing.T) {
	b := normal("one\ntwo\nthree")
	keys(b, "G", "V", "k")
	left, right := b.Selection()
	if left != 4 || right != len(b.Text) {
		t.Fatalf("reverse line selection lost complete lines: %d:%d", left, right)
	}
	keys(b, "y")
	if string(b.Register) != "two\nthree" || !b.Linewise || b.Pos != 4 || b.Mode != "normal" {
		t.Fatal("line yank lost selection or register type")
	}

	b = normal("((界))")
	keys(b, "%")
	if b.Pos != 4 {
		t.Fatal("nested pair did not reach matching close")
	}
	keys(b, "%")
	if b.Pos != 0 {
		t.Fatal("reverse nested pair did not return to open")
	}
	b.Set("(unclosed")
	b.Mode, b.Pos = "normal", 0
	keys(b, "%")
	if b.Pos != 0 {
		t.Fatal("unmatched pair moved the cursor")
	}
}

func TestInsertControlsAndSearchCancellation(t *testing.T) {
	b := New()
	b.Set("ab\ncd")
	b.Pos = 3
	keys(b, "backspace", "delete", "home", "end", "left", "right", "enter", "tab")
	if b.String() != "abd\n    " || b.Pos != 8 || b.Mode != "insert" {
		t.Fatalf("insert controls: %q at %d", b.String(), b.Pos)
	}
	b.Key("esc", "")
	keys(b, "/", "a", "b", "backspace")
	if b.SearchQuery() != "a" || b.Status() != "/a" {
		t.Fatal("search editing did not update visible query")
	}
	b.Key("esc", "")
	if b.Mode != "normal" || b.Search != "" || b.Pos != 7 {
		t.Fatal("cancelled search changed cursor or committed query")
	}
}

func TestCharacterEditsReplaceLinewiseRegisterType(t *testing.T) {
	for _, tc := range []struct {
		input []string
		want  string
	}{
		{[]string{"Y", "s", "x", "esc", "p"}, "xabc"},
		{[]string{"Y", "l", "X", "p"}, "bac"},
	} {
		b := normal("abc")
		keys(b, tc.input...)
		if b.Linewise || string(b.Register) != "a" || b.String() != tc.want {
			t.Fatalf("character edit retained line register: text=%q register=%q linewise=%t", b.String(), string(b.Register), b.Linewise)
		}
	}
}
