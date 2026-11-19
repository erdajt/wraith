package plugin

import (
	"errors"
	"strconv"
)

// validateJSONEscapes rejects surrogate escapes that encoding/json would replace silently.
func validateJSONEscapes(input string) error {
	inside := false
	for i := 0; i < len(input); i++ {
		if input[i] == '"' {
			inside = !inside
			continue
		}
		if !inside || input[i] != '\\' {
			continue
		}
		i++
		if input[i] != 'u' {
			continue
		}
		unit, _ := strconv.ParseUint(input[i+1:i+5], 16, 16)
		i += 4
		if unit < 0xd800 || unit > 0xdfff {
			continue
		}
		if unit > 0xdbff || i+6 >= len(input) || input[i+1:i+3] != `\u` {
			return errors.New("unpaired unicode surrogate")
		}
		low, err := strconv.ParseUint(input[i+3:i+7], 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return errors.New("unpaired unicode surrogate")
		}
		i += 6
	}
	return nil
}
