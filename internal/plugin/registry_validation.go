package plugin

import (
	"fmt"
	"github.com/erdajt/wraith/internal/modeltool"
)

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
	if e.catalogDirty {
		definitions := make([]modeltool.Definition, 0, len(e.manifest.Tools))
		for _, tool := range e.manifest.Tools {
			definitions = append(definitions, modeltool.Definition{Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters})
		}
		catalog, err := modeltool.NewCatalog(definitions)
		if err != nil {
			return err
		}
		e.manifest.ToolCatalog = catalog
		e.catalogDirty = false
	}
	return nil
}
