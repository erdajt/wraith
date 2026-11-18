package keymap

import "testing"

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
