package config

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func DefaultBindings() map[string]string {
	return map[string]string{"v": "thinking", "b": "blocks", "m": "models", "s": "settings", "e": "edit", "h": "help", "c e": "edit-response", "c c": "compact", "c n": "clear", "c s": "sessions", "c a": "archives", "i": "instructions", "p": "pins", "t": "think", "r": "history"}
}

func (c Config) validateBindings() error {
	if c.Leader != "space" && utf8.RuneCountInString(c.Leader) != 1 {
		return fmt.Errorf("leader must be space or one character")
	}
	for sequence, command := range c.Bindings {
		keys := strings.Fields(sequence)
		if len(keys) == 0 || strings.Join(keys, " ") != sequence || strings.TrimSpace(command) == "" {
			return fmt.Errorf("invalid command binding: %q", sequence)
		}
		for _, key := range keys {
			if utf8.RuneCountInString(key) != 1 || key == "/" {
				return fmt.Errorf("binding keys must be single characters other than /: %q", sequence)
			}
		}
		for other := range c.Bindings {
			if strings.HasPrefix(other, sequence+" ") {
				return fmt.Errorf("binding %q conflicts with prefix of %q", sequence, other)
			}
		}
	}
	return nil
}
