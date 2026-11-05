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
