package variables

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const FileLimit = 256 << 10

var namePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,63}$`)

type Entry struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

func Parse(text string) ([]Entry, error) {
	if len(text) > FileLimit || !utf8.ValidString(text) || strings.IndexFunc(text, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' }) >= 0 {
		return nil, errors.New("environment file exceeds 256 kib or contains invalid text")
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if strings.ContainsRune(text, '\r') {
		return nil, errors.New("environment file contains a carriage return")
	}
	var entries []Entry
	var pending []string
	seen := map[string]bool{}
	block := false
	inlineBlock := -1
	for number, line := range strings.Split(text, "\n") {
		var code strings.Builder
		var comments []string
		var quote byte
		trailing := inlineBlock >= 0
		for i := 0; i < len(line); {
			if block {
				end := strings.Index(line[i:], "*/")
				if nested := strings.Index(line[i:], "/*"); nested >= 0 && (end < 0 || nested < end) {
					return nil, fmt.Errorf("environment line %d: nested block comment", number+1)
				}
				if end < 0 {
					comments = append(comments, strings.TrimSpace(line[i:]))
					break
				}
				comments = append(comments, strings.TrimSpace(line[i:i+end]))
				i += end + 2
				block = false
				continue
			}
			ch := line[i]
			if quote != 0 {
				code.WriteByte(ch)
				i++
				if ch == '\\' && quote == '"' && i < len(line) {
					code.WriteByte(line[i])
					i++
				} else if ch == quote {
					quote = 0
				}
				continue
			}
			separated := i == 0 || line[i-1] == ' ' || line[i-1] == '\t'
			if separated && (ch == '#' || strings.HasPrefix(line[i:], "//")) {
				start := i + 1
				if ch == '/' {
					start++
				}
				comments = append(comments, strings.TrimSpace(line[start:]))
				break
			}
			if separated && strings.HasPrefix(line[i:], "/*") {
				block = true
				trailing = trailing || strings.TrimSpace(code.String()) != ""
				i += 2
				continue
			}
			if trailing && ch != ' ' && ch != '\t' {
				return nil, fmt.Errorf("environment line %d: syntax after trailing comment", number+1)
			}
			if ch == '\'' || ch == '"' {
				quote = ch
			}
			code.WriteByte(ch)
			i++
		}
		if quote != 0 {
			return nil, fmt.Errorf("environment line %d: unterminated quote", number+1)
		}
		if inlineBlock >= 0 {
			description := strings.TrimSpace(entries[inlineBlock].Description + "\n" + strings.Join(comments, "\n"))
			if len(description) > 4<<10 {
				return nil, fmt.Errorf("environment line %d: description exceeds 4 kib", number+1)
			}
			entries[inlineBlock].Description = description
			comments = nil
			if !block {
				inlineBlock = -1
			}
		}
		body := strings.TrimSpace(code.String())
		if body == "" {
			if len(comments) > 0 || block {
				pending = append(pending, comments...)
			} else {
				pending = nil
			}
			continue
		}
		name, value, ok := strings.Cut(body, "=")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if !ok || !namePattern.MatchString(name) || seen[name] {
			return nil, fmt.Errorf("environment line %d: invalid or duplicate assignment", number+1)
		}
		if len(value) > 0 && (value[0] == '\'' || value[0] == '"') {
			q := value[0]
			var decoded strings.Builder
			closed := false
			for i := 1; i < len(value); i++ {
				if value[i] == q {
					closed = i == len(value)-1
					break
				}
				if q == '"' && value[i] == '\\' {
					i++
					if i >= len(value) || (value[i] != '\\' && value[i] != '"') {
						return nil, fmt.Errorf("environment line %d: unsupported escape", number+1)
					}
				}
				decoded.WriteByte(value[i])
			}
			if !closed {
				return nil, fmt.Errorf("environment line %d: trailing quoted value syntax", number+1)
			}
			value = decoded.String()
		}
		description := strings.TrimSpace(strings.Join(append(pending, comments...), "\n"))
		pending = nil
		if value == "" || len(value) > 16<<10 || strings.Contains(value, "wraith::") || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return nil, fmt.Errorf("environment line %d: invalid value", number+1)
		}
		if len(description) > 4<<10 || strings.IndexFunc(description, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }) >= 0 {
			return nil, fmt.Errorf("environment line %d: invalid description", number+1)
		}
		seen[name] = true
		entries = append(entries, Entry{Name: name, Value: value, Description: description})
		if block {
			inlineBlock = len(entries) - 1
		}
		if len(entries) > 128 {
			return nil, errors.New("environment scope exceeds 128 entries")
		}
	}
	if block {
		return nil, errors.New("environment file: unterminated block comment")
	}
	return entries, nil
}
