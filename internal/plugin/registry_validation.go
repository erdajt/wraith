package plugin

import "fmt"

// validateRegistry rejects invalid local settings and unresolved mapping targets before publication.
func (e *engine) validateRegistry() error {
	if err := e.manifest.Config.Validate(); err != nil {
		return err
	}
	for _, binding := range e.manifest.Config.Keymaps {
		if binding.Input == "" && !e.actions[binding.Action] && e.functions[binding.Action] == nil {
			return fmt.Errorf("keymap %q: unknown action %s", binding.Keys, binding.Action)
		}
	}
	return nil
}
