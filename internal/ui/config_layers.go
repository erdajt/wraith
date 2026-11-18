package ui

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/erdajt/wraith/internal/config"
)

// saveConfig persists explicit setting changes without copying lua-only values into yaml.
func (a *App) saveConfig(next config.Config) error {
	if a.plugins == nil {
		return config.Save(config.File(a.Paths.Config), next)
	}
	base, err := config.LoadDirectory(a.Paths.Config)
	if err != nil {
		return err
	}
	objects := make([]map[string]any, 3)
	for i, c := range []config.Config{base, a.Config, next} {
		data, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &objects[i]); err != nil {
			return err
		}
	}
	mergeSettings(objects[0], objects[1], objects[2])
	data, err := json.Marshal(objects[0])
	if err != nil {
		return err
	}
	var saved config.Config
	if err := json.Unmarshal(data, &saved); err != nil {
		return err
	}
	for _, binding := range saved.Keymaps {
		if strings.HasPrefix(binding.Action, "lua.callback.") {
			return fmt.Errorf("edit init.lua to save callback bindings")
		}
	}
	if err := saved.Validate(); err != nil {
		return fmt.Errorf("yaml change depends on lua-only settings; edit init.lua: %w", err)
	}
	return config.Save(config.File(a.Paths.Config), saved)
}

// mergeSettings applies changed leaves while retaining unrelated persisted values.
func mergeSettings(base, before, after map[string]any) {
	for key := range before {
		if _, ok := after[key]; !ok {
			delete(base, key)
		}
	}
	for key, value := range after {
		if reflect.DeepEqual(before[key], value) {
			continue
		}
		old, oldMap := before[key].(map[string]any)
		next, nextMap := value.(map[string]any)
		if oldMap && nextMap {
			current, ok := base[key].(map[string]any)
			if !ok {
				current = map[string]any{}
				base[key] = current
			}
			mergeSettings(current, old, next)
		} else {
			base[key] = value
		}
	}
}
