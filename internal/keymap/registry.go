package keymap

import (
	"fmt"
	"slices"
	"strings"
)

type Binding struct {
	Mode        string `json:"mode" yaml:"mode"`
	Keys        string `json:"keys" yaml:"keys"`
	Action      string `json:"action" yaml:"action"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type Match struct {
	Action, Description string
	Exact, Pending      bool
	Hints               []Binding
}

type Registry struct {
	entries  map[string]map[string]Binding
	defaults *Registry
}

// New validates complete mappings before returning a registry with later overrides applied.
func New(bindings []Binding) (*Registry, error) {
	r := &Registry{entries: map[string]map[string]Binding{}}
	for _, binding := range bindings {
		if !slices.Contains([]string{"*", "prompt.normal", "prompt.insert", "prompt.visual", "prompt.operator", "chat.normal", "chat.visual", "chat.operator", "command", "search", "picker", "output", "terminal", "terminal.copy"}, binding.Mode) {
			return nil, fmt.Errorf("keymap %q: unknown mode %q", binding.Keys, binding.Mode)
		}
		keys, err := Parse(binding.Keys)
		if err != nil {
			return nil, fmt.Errorf("keymap %q: %w", binding.Keys, err)
		}
		if binding.Action == "" || len(binding.Action) > 256 {
			return nil, fmt.Errorf("keymap %q: action must contain 1-256 bytes", binding.Keys)
		}
		if r.entries[binding.Mode] == nil {
			r.entries[binding.Mode] = map[string]Binding{}
		}
		r.entries[binding.Mode][strings.Join(keys, "\x00")] = binding
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
	var result Match
	if r == nil {
		return result
	}
	sequence := strings.Join(keys, "\x00")
	entries := make(map[string]Binding, len(r.entries["*"])+len(r.entries[mode]))
	for key, binding := range r.entries["*"] {
		entries[key] = binding
	}
	for key, binding := range r.entries[mode] {
		entries[key] = binding
	}
	for key, binding := range entries {
		if key == sequence {
			result.Action, result.Description, result.Exact = binding.Action, binding.Description, true
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
	slices.SortFunc(out, func(a, b Binding) int { return strings.Compare(a.Mode+"\x00"+a.Keys, b.Mode+"\x00"+b.Keys) })
	return out
}
