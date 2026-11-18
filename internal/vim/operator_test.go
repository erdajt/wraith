package vim

import "testing"

func TestOperatorCharacterFindsAndLineYank(t *testing.T) {
	for _, tt := range []struct{ keys, text, register string }{
		{"Y", "call(a, bee) end\nnext", "call(a, bee) end\n"},
		{"ye", "call(a, bee) end\nnext", "call"},
		{"yf,", "call(a, bee) end\nnext", "call(a,"},
		{"yf(", "call(a, bee) end\nnext", "call("},
		{"df(", "a, bee) end\nnext", "call("},
		{"dfe", "e) end\nnext", "call(a, be"},
		{"de", "(a, bee) end\nnext", "call"},
		{"D", "\nnext", "call(a, bee) end"},
		{"dd", "next", "call(a, bee) end\n"},
		{"dfz", "call(a, bee) end\nnext", ""},
		{"2yfe", "call(a, bee) end\nnext", "call(a, bee"},
	} {
		t.Run(tt.keys, func(t *testing.T) {
			b := New()
			b.Text = []rune("call(a, bee) end\nnext")
			b.Mode = "normal"

			for _, r := range tt.keys {
				b.Key(string(r), string(r))
			}

			if b.String() != tt.text || string(b.Register) != tt.register {
				t.Fatalf("text %q register %q", b.String(), string(b.Register))
			}
		})
	}
}

func TestCompoundOperatorCountsMultiplyAndReset(t *testing.T) {
	for _, tt := range []struct{ sequence, text, register string }{
		{"2d3w", "seven eight", "one two three four five six "},
		{"2y3w", "one two three four five six seven eight", "one two three four five six "},
		{"2c3w", " seven eight", "one two three four five six"},
		{"2y2fe", "one two three four five six seven eight", "one two three four five"},
	} {
		t.Run(tt.sequence, func(t *testing.T) {
			b := normal("one two three four five six seven eight")
			for _, r := range tt.sequence {
				b.Key(string(r), string(r))
			}
			if b.String() != tt.text || string(b.Register) != tt.register {
				t.Fatalf("text %q, register %q", b.String(), string(b.Register))
			}
			if b.Count != "" || b.OperatorCount != "" || b.Pending != "" {
				t.Fatal("completed operator retained count state")
			}
		})
	}

	b := normal("one two three")
	keys(b, "2", "d", "3", "esc", "d", "w")
	if b.String() != "two three" {
		t.Fatal("cancelled operator count affected the next edit")
	}
	b.Set("one two three")
	b.Mode, b.Pos = "normal", 0
	keys(b, "2", "d", "3")
	b.Set("replacement")
	if b.Count != "" || b.OperatorCount != "" || b.Pending != "" {
		t.Fatal("replacement retained a pending operator")
	}
}

func TestWordOperatorsIncludeTheEndOfBuffer(t *testing.T) {
	for _, tt := range []struct{ sequence, text, register string }{
		{"dw", "", "last"}, {"dW", "", "last"},
		{"yw", "last", "last"}, {"2d3w", "", "last"},
	} {
		t.Run(tt.sequence, func(t *testing.T) {
			b := normal("last")
			for _, r := range tt.sequence {
				b.Key(string(r), string(r))
			}
			if b.String() != tt.text || string(b.Register) != tt.register {
				t.Fatalf("text %q, register %q", b.String(), string(b.Register))
			}
		})
	}
}
