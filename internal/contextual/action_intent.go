package contextual

import (
	"encoding/json"
	"strings"

	"github.com/erdajt/wraith/internal/editor"
)

// UnclearCommandIntent identifies command objects that need clarification without making them actionable.
func UnclearCommandIntent(content string) error {
	for _, section := range commandSections(content, "wraith-run") {
		if section.complete && section.suggestion && !placeholder.MatchString(section.text) {
			return &ActionProtocolError{Target: "intent"}
		}
	}

	candidates := strings.Split(content, "\n")
	for _, block := range editor.Blocks(content) {
		if block.Language == "" || block.Language == "json" || block.Language == "text" {
			candidates = append(candidates, block.Text)
		}
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if len(candidate) > 32768 || !strings.HasPrefix(candidate, "{") || placeholder.MatchString(candidate) {
			continue
		}

		var object map[string]json.RawMessage
		if json.Unmarshal([]byte(candidate), &object) != nil {
			continue
		}
		if object["command"] != nil && object["reason"] != nil {
			return &ActionProtocolError{Target: "intent"}
		}
	}
	return nil
}
