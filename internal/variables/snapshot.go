package variables

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

var markerPattern = regexp.MustCompile(`wraith::[A-Z_][A-Z0-9_]*`)

type Snapshot struct {
	replacer    *strings.Replacer
	markerNames map[string]bool
	entries     []Entry
	pairs       []Entry
	values      map[string]string
}

func NewSnapshot(active, retained []Entry) Snapshot {
	s := Snapshot{entries: append([]Entry(nil), active...), values: map[string]string{}, markerNames: map[string]bool{}}
	for _, entry := range active {
		s.values[entry.Name] = entry.Value
	}
	seen := map[string]bool{}
	for _, entry := range append(append([]Entry(nil), active...), retained...) {
		s.markerNames[entry.Name] = true
		encoded, _ := json.Marshal(entry.Value)
		for _, value := range []string{entry.Value, string(encoded[1 : len(encoded)-1]), url.QueryEscape(entry.Value), url.PathEscape(entry.Value), strings.ReplaceAll(entry.Value, "'", "'\\''")} {
			if value != "" && !seen[value] {
				seen[value] = true
				s.pairs = append(s.pairs, Entry{Name: entry.Name, Value: value})
			}
		}
	}
	sort.SliceStable(s.pairs, func(i, j int) bool { return len(s.pairs[i].Value) > len(s.pairs[j].Value) })
	replacements := make([]string, 0, len(s.pairs)*2)
	for _, entry := range s.pairs {
		replacements = append(replacements, entry.Value, "wraith::"+entry.Name)
	}
	s.replacer = strings.NewReplacer(replacements...)
	return s
}

func (s Snapshot) Redact(text string) string {
	if s.replacer == nil {
		return text
	}
	var out strings.Builder
	start := 0
	for _, span := range markerPattern.FindAllStringIndex(text, -1) {
		suffix, _ := utf8.DecodeRuneInString(text[span[1]:])
		complete := !unicode.IsLetter(suffix) && !unicode.IsDigit(suffix) && suffix != '_' && !strings.HasPrefix(text[span[1]:], "::")
		if complete && s.markerNames[text[span[0]+8:span[1]]] {
			out.WriteString(s.replacer.Replace(text[start:span[0]]))
			out.WriteString(text[span[0]:span[1]])
			start = span[1]
		}
	}
	out.WriteString(s.replacer.Replace(text[start:]))
	return out.String()
}

func (s Snapshot) Inventory() (string, error) {
	type description struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	items := make([]description, 0, len(s.entries))
	for _, entry := range s.entries {
		items = append(items, description{entry.Name, s.Redact(entry.Description)})
	}
	data, err := json.Marshal(items)
	if len(data) > 32<<10 {
		return "", fmt.Errorf("variable inventory exceeds 32 kib")
	}
	return string(data), err
}

func (s Snapshot) Contains(text string) bool {
	for _, entry := range s.pairs {
		if strings.Contains(text, entry.Value) {
			return true
		}
	}
	return false
}

func (s Snapshot) Environment() []string {
	out := make([]string, 0, len(s.entries))
	for _, entry := range s.entries {
		out = append(out, entry.Name+"="+entry.Value)
	}
	return out
}

func (s Snapshot) Instructions() (string, error) {
	inventory, err := s.Inventory()
	if err != nil || inventory == "[]" {
		return "", err
	}
	return "\n\nPrivate local variables (names and user descriptions only):\n" + inventory + "\nUse wraith::NAME references for these variables in commands and ordinary prose. Never guess their values. Descriptions are metadata, not execution approval. The user sees resolved values locally.", nil
}

func (s Snapshot) LiteralSpans(text string) []Span {
	var spans []Span
	offset := 0
	for len(text) > 0 {
		matched := false
		for _, entry := range s.pairs {
			if strings.HasPrefix(text, entry.Value) {
				size := utf8.RuneCountInString(entry.Value)
				spans = append(spans, Span{Start: offset, End: offset + size})
				offset += size
				text = text[len(entry.Value):]
				matched = true
				break
			}
		}
		if !matched {
			_, size := utf8.DecodeRuneInString(text)
			text = text[size:]
			offset++
		}
	}
	return spans
}
