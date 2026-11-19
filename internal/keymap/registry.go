package keymap

import (
	"fmt"
	"slices"
	"strings"
)

type Binding struct {
	Buffer      string `json:"buffer,omitempty" yaml:"buffer,omitempty"`
	Mode        string `json:"mode" yaml:"mode"`
	Keys        string `json:"keys" yaml:"keys"`
	Action      string `json:"action,omitempty" yaml:"action,omitempty"`
	Input       string `json:"input,omitempty" yaml:"input,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type Match struct {
	Action, Input, Description string
	Exact, Pending             bool
	Hints                      []Binding
}

type Registry struct {
	entries  map[string]map[string]Binding
	defaults *Registry
}

// New validates complete mappings before returning a registry with later overrides applied.
func New(bindings []Binding) (*Registry, error) {
	r := &Registry{entries: map[string]map[string]Binding{}}
	for _, binding := range bindings {
		if len(binding.Buffer) > 256 || strings.ContainsRune(binding.Buffer, 0) {
			return nil, fmt.Errorf("keymap %q: invalid buffer id", binding.Keys)
		}
		valid := slices.Contains([]string{"*", "command", "search", "picker", "output", "buffer", "terminal", "terminal.copy"}, binding.Mode)
		for _, surface := range []string{"prompt", "chat", "output", "buffer", "terminal.copy"} {
			mode := strings.TrimPrefix(binding.Mode, surface+".")
			valid = valid || mode != binding.Mode && slices.Contains([]string{"normal", "visual", "operator", "argument", "search"}, mode)
		}
		if !valid && binding.Mode != "prompt.insert" && binding.Mode != "buffer.insert" {
			return nil, fmt.Errorf("keymap %q: unknown mode %q", binding.Keys, binding.Mode)
		}
		keys, err := Parse(binding.Keys)
		if err != nil {
			return nil, fmt.Errorf("keymap %q: %w", binding.Keys, err)
		}
		if (binding.Action == "") == (binding.Input == "") || len(binding.Action) > 256 {
			return nil, fmt.Errorf("keymap %q: choose one action or native input sequence", binding.Keys)
		}
		if binding.Input != "" {
			if _, err := ParseInput(binding.Input); err != nil {
				return nil, fmt.Errorf("keymap %q input: %w", binding.Keys, err)
			}
		}
		scope := binding.Mode
		if binding.Buffer != "" {
			scope = binding.Buffer + "\x00" + scope
		}
		if r.entries[scope] == nil {
			r.entries[scope] = map[string]Binding{}
		}
		r.entries[scope][strings.Join(keys, "\x00")] = binding
	}
	return r, nil
}

// WithDefaults retains the configured base mappings separately from user overrides.
func WithDefaults(defaults, overrides []Binding) (*Registry, error) {
	base, err := New(defaults)
	if err != nil {
		return nil, err
	}
	all := append(append([]Binding(nil), defaults...), overrides...)
	r, err := New(all)
	if err != nil {
		return nil, err
	}
	r.defaults = base
	return r, nil
}

// LookupDefault resolves fallback input without re-entering user mappings.
func (r *Registry) LookupDefault(mode string, keys []string) Match {
	if r == nil {
		return Match{}
	}
	return r.defaults.Lookup(mode, keys)
}

// Lookup resolves an exact sequence and its continuations for a single input context.
func (r *Registry) Lookup(mode string, keys []string) Match {
	return r.LookupBuffer("", mode, keys)
}

// LookupBuffer overlays one buffer's bindings after inherited workspace and editor mappings.
func (r *Registry) LookupBuffer(bufferID, mode string, keys []string) Match {
	var result Match
	if r == nil {
		return result
	}
	sequence := strings.Join(keys, "\x00")
	entries := make(map[string]Binding)
	layers := []string{"*"}
	if strings.HasSuffix(mode, ".search") {
		layers = append(layers, "search")
	}
	if strings.HasPrefix(mode, "terminal.copy.") {
		layers = append(layers, "terminal.copy")
	} else if strings.HasPrefix(mode, "output.") {
		layers = append(layers, "output")
	} else if strings.HasPrefix(mode, "buffer.") {
		if mode != "buffer.insert" {
			layers = append(layers, "chat."+strings.TrimPrefix(mode, "buffer."))
		}
		layers = append(layers, "buffer")
	}
	layers = append(layers, mode)
	if bufferID != "" {
		for _, layer := range slices.Clone(layers) {
			layers = append(layers, bufferID+"\x00"+layer)
		}
	}
	for _, layer := range layers {
		for key, binding := range r.entries[layer] {
			entries[key] = binding
		}
	}
	for key, binding := range entries {
		if key == sequence {
			result.Action, result.Input, result.Description, result.Exact = binding.Action, binding.Input, binding.Description, true
		} else if strings.HasPrefix(key, sequence+"\x00") {
			result.Pending = true
			result.Hints = append(result.Hints, binding)
		}
	}
	slices.SortFunc(result.Hints, func(a, b Binding) int { return strings.Compare(a.Keys, b.Keys) })
	return result
}

// Bindings returns a sorted copy for help and runtime inspection.
func (r *Registry) Bindings() []Binding {
	var out []Binding
	for _, entries := range r.entries {
		for _, binding := range entries {
			out = append(out, binding)
		}
	}
	slices.SortFunc(out, func(a, b Binding) int {
		return strings.Compare(a.Buffer+"\x00"+a.Mode+"\x00"+a.Keys, b.Buffer+"\x00"+b.Mode+"\x00"+b.Keys)
	})
	return out
}
