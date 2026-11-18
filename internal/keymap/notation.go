package keymap

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Parse translates a bounded neovim-style sequence into terminal key names.
func Parse(sequence string) ([]string, error) {
	if len(sequence) == 0 || len(sequence) > 256 || !utf8.ValidString(sequence) {
		return nil, fmt.Errorf("invalid key sequence: %q", sequence)
	}
	var keys []string
	for len(sequence) > 0 {
		r, size := utf8.DecodeRuneInString(sequence)
		sequence = sequence[size:]
		if r != '<' {
			if unicode.IsControl(r) {
				return nil, fmt.Errorf("control characters require key notation")
			}
			keys = append(keys, string(r))
			continue
		}
		end := strings.IndexByte(sequence, '>')
		if end < 0 {
			return nil, fmt.Errorf("unterminated key notation")
		}
		key, err := namedKey(sequence[:end])
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
		sequence = sequence[end+1:]
	}
	if len(keys) > 16 {
		return nil, fmt.Errorf("key sequence exceeds 16 keys")
	}
	return keys, nil
}

// namedKey normalizes named keys and the control aliases terminals cannot distinguish.
func namedKey(name string) (string, error) {
	name = strings.ToLower(name)
	aliases := map[string]string{"space": " ", "lt": "<", "cr": "enter", "return": "enter", "esc": "esc", "bs": "backspace", "del": "delete", "c-h": "backspace", "c-i": "tab", "c-m": "enter", "c-[": "esc", "c-@": "ctrl+@", "c-space": "ctrl+@"}
	if key, ok := aliases[name]; ok {
		return key, nil
	}
	if strings.HasPrefix(name, "c-") && len(name) == 3 && strings.ContainsRune("abcdefghijklmnopqrstuvwxyz\\]^_", rune(name[2])) {
		return "ctrl+" + name[2:], nil
	}
	if strings.HasPrefix(name, "a-") || strings.HasPrefix(name, "m-") {
		key, err := namedKey(name[2:])
		if utf8.RuneCountInString(name[2:]) == 1 {
			key, err = name[2:], nil
		}
		return "alt+" + key, err
	}
	for _, key := range []string{"enter", "tab", "backspace", "delete", "up", "down", "left", "right", "home", "end", "pgup", "pgdown", "insert", "s-tab", "s-up", "s-down", "s-left", "s-right", "s-home", "s-end", "s-pgup", "s-pgdown"} {
		if key == name {
			return strings.Replace(name, "s-", "shift+", 1), nil
		}
	}
	for i := 1; i <= 20; i++ {
		if name == fmt.Sprintf("f%d", i) {
			return name, nil
		}
	}
	return "", fmt.Errorf("unknown key notation: <%s>", name)
}
