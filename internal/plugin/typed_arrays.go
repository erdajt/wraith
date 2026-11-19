package plugin

import (
	"encoding/json"
	"reflect"
	"strings"
)

// normalizeTypedArrays resolves empty Lua tables using the destination's nested sequence types.
func normalizeTypedArrays(value any, target reflect.Type) any {
	for target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	if reflect.PointerTo(target).Implements(reflect.TypeFor[json.Unmarshaler]()) {
		return value
	}
	switch target.Kind() {
	case reflect.Slice, reflect.Array:
		if object, ok := value.(map[string]any); ok && len(object) == 0 {
			return []any{}
		}
		if array, ok := value.([]any); ok {
			for index, child := range array {
				array[index] = normalizeTypedArrays(child, target.Elem())
			}
		}
	case reflect.Map:
		if object, ok := value.(map[string]any); ok {
			for key, child := range object {
				object[key] = normalizeTypedArrays(child, target.Elem())
			}
		}
	case reflect.Struct:
		if object, ok := value.(map[string]any); ok {
			for index := 0; index < target.NumField(); index++ {
				field := target.Field(index)
				name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
				if name == "-" || !field.IsExported() {
					continue
				}
				if name == "" {
					name = field.Name
				}
				for key, child := range object {
					if strings.EqualFold(key, name) {
						object[key] = normalizeTypedArrays(child, field.Type)
					}
				}
			}
		}
	}
	return value
}
