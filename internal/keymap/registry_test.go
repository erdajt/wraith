package keymap

import "testing"

func TestBufferBindingsInheritNavigationAndOverrideByScope(t *testing.T) {
	r, err := New([]Binding{
		{Mode: "chat.normal", Keys: "<C-h>", Action: "focus.left"},
		{Mode: "chat.normal", Keys: "j", Input: "j"},
		{Mode: "buffer", Keys: "j", Input: "2j"},
		{Mode: "buffer.normal", Keys: "j", Input: "3j"},
		{Mode: "buffer.insert", Keys: "<C-l>", Action: "focus.right"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Lookup("buffer.normal", []string{"ctrl+h"}); got.Action != "focus.left" {
		t.Fatalf("conversation navigation was not inherited: %+v", got)
	}
	if got := r.Lookup("buffer.normal", []string{"j"}); got.Input != "3j" {
		t.Fatalf("specific buffer override lost precedence: %+v", got)
	}
	if got := r.Lookup("buffer.insert", []string{"ctrl+h"}); got.Exact {
		t.Fatal("normal-mode navigation leaked into insert mode")
	}
	if got := r.Lookup("buffer.insert", []string{"ctrl+l"}); got.Action != "focus.right" {
		t.Fatalf("insert mapping missing: %+v", got)
	}
}

func TestBufferLocalMappingsStayWithinTheirTarget(t *testing.T) {
	r, err := New([]Binding{
		{Mode: "buffer.normal", Keys: "x", Action: "global"},
		{Buffer: "first", Mode: "buffer.normal", Keys: "x", Action: "first"},
		{Buffer: "second", Mode: "buffer.normal", Keys: "x", Action: "second"},
		{Buffer: "first", Mode: "buffer.visual", Keys: "y", Input: "y"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{"first": "first", "second": "second", "third": "global", "": "global"} {
		if got := r.LookupBuffer(id, "buffer.normal", []string{"x"}); got.Action != want {
			t.Fatalf("buffer %q mapping = %q, want %q", id, got.Action, want)
		}
	}
	if got := r.LookupBuffer("second", "buffer.visual", []string{"y"}); got.Exact {
		t.Fatal("local visual mapping leaked to another buffer")
	}
	if len(r.Bindings()) != 4 {
		t.Fatal("buffer-local identity overwrote an unrelated binding")
	}
}

func TestRegistryOverridesWithoutSharingContextOrMutatingDefaults(t *testing.T) {
	bindings := []Binding{{Mode: "chat.normal", Keys: "<C-w>v", Action: "window.vertical"}, {Mode: "chat.normal", Keys: "<C-w>", Action: "window.help"}, {Mode: "chat.normal", Keys: "<C-w>v", Action: "window.horizontal"}, {Mode: "*", Keys: "<F1>", Action: "help"}}
	r, err := New(bindings)
	if err != nil {
		t.Fatal(err)
	}
	bindings[2].Action = "mutated"
	if got := r.Lookup("chat.normal", []string{"ctrl+w"}); !got.Exact || !got.Pending || len(got.Hints) != 1 || got.Hints[0].Action != "window.horizontal" {
		t.Fatalf("ambiguous prefix lost: %+v", got)
	}
	if got := r.Lookup("chat.normal", []string{"ctrl+w", "v"}); !got.Exact || got.Pending || got.Action != "window.horizontal" {
		t.Fatalf("override failed: %+v", got)
	}
	for _, mode := range []string{"prompt.insert", "terminal", "picker"} {
		if got := r.Lookup(mode, []string{"ctrl+w"}); got.Exact || got.Pending {
			t.Fatalf("chat binding leaked into %s", mode)
		}
		if got := r.Lookup(mode, []string{"f1"}); got.Action != "help" {
			t.Fatalf("explicit global binding missing from %s", mode)
		}
	}
}

func TestRegistryRejectsInvalidCandidates(t *testing.T) {
	for _, binding := range []Binding{{Mode: "wrong", Keys: "x", Action: "help"}, {Mode: "chat.normal", Keys: "<invalid>", Action: "help"}, {Mode: "chat.normal", Keys: "x"}} {
		if _, err := New([]Binding{binding}); err == nil {
			t.Fatalf("invalid mapping accepted: %+v", binding)
		}
	}
}

func TestNativeMappingsPreserveSpecificityAndLegacyReaders(t *testing.T) {
	r, err := New([]Binding{
		{Mode: "*", Keys: "H", Input: "0"},
		{Mode: "output", Keys: "H", Input: "^"},
		{Mode: "output.visual", Keys: "H", Input: "gg"},
		{Mode: "terminal.copy", Keys: "H", Input: "G"},
		{Mode: "search", Keys: "<C-h>", Input: "<BS>"},
		{Mode: "prompt.search", Keys: "<C-h>", Input: "<Esc>"},
		{Mode: "output.search", Keys: "H", Input: "<Esc>"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ mode, key, want string }{
		{"output.normal", "H", "^"}, {"output.visual", "H", "gg"},
		{"terminal.copy.normal", "H", "G"}, {"prompt.normal", "H", "0"},
		{"output.search", "H", "<Esc>"}, {"terminal.copy.search", "H", "G"},
		{"chat.search", "ctrl+h", "<BS>"}, {"prompt.search", "ctrl+h", "<Esc>"},
	} {
		if got := r.Lookup(tc.mode, []string{tc.key}); got.Input != tc.want || !got.Exact {
			t.Fatalf("%s: %+v, want %s", tc.mode, got, tc.want)
		}
	}
}

func TestNativeMappingsRejectAmbiguousOrUnboundedInput(t *testing.T) {
	for _, binding := range []Binding{
		{Mode: "prompt.normal", Keys: "x", Action: "noop", Input: "i"},
		{Mode: "prompt.normal", Keys: "x", Input: "<invalid>"},
		{Mode: "prompt.normal", Keys: "x", Input: "abcdefghijklmnopq"},
	} {
		if _, err := New([]Binding{binding}); err == nil {
			t.Fatalf("invalid native mapping accepted: %+v", binding)
		}
	}
}
