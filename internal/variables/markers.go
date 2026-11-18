package variables

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Span struct{ Start, End int }
type reference struct {
	start, end int
	name       string
}

// references parses references and withholds incomplete suffixes during streaming.
func references(text string, final bool) ([]reference, int, error) {
	var refs []reference
	const prefix = "wraith::"
	position := 0

	for {
		at := strings.Index(text[position:], prefix)
		if at < 0 {
			break
		}

		start := position + at
		end := start + len(prefix)

		for end < len(text) {
			r, size := utf8.DecodeRuneInString(text[end:])
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				break
			}
			end += size
		}

		if end == len(text) && !final && end-start <= len(prefix)+64 {
			return refs, start, nil
		}

		name := text[start+len(prefix) : end]
		if !namePattern.MatchString(name) || end < len(text) && (name == "" && text[end] == ':' || strings.HasPrefix(text[end:], "::")) {
			return refs, start, fmt.Errorf("invalid variable reference at byte %d", start+1)
		}
		refs = append(refs, reference{start, end, name})
		position = end
	}

	for size := min(len(prefix)-1, len(text)-position); size > 0; size-- {
		if strings.HasSuffix(text, prefix[:size]) {
			if !final {
				return refs, len(text) - size, nil
			}
			if size >= len("wraith:") {
				return refs, len(text) - size, fmt.Errorf("incomplete variable reference")
			}
			break
		}
	}

	return refs, len(text), nil
}

// Validate checks reference syntax and names, returning the safe streamed prefix length.
func (s Snapshot) Validate(text string, final bool) (int, error) {
	refs, end, err := references(text, final)
	for _, ref := range refs {
		if _, ok := s.values[ref.name]; !ok {
			return ref.start, fmt.Errorf("unknown variable %s", ref.name)
		}
	}
	return end, err
}

// Expand resolves known references and returns rune spans for highlighting.
func (s Snapshot) Expand(text string) (string, []Span) {
	var out strings.Builder
	var spans []Span
	position, runes := 0, 0

	for _, at := range markerPattern.FindAllStringIndex(text, -1) {
		end := at[1]
		if end < len(text) {
			r, _ := utf8.DecodeRuneInString(text[end:])
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || strings.HasPrefix(text[end:], "::") {
				continue
			}
		}

		value, ok := s.values[text[at[0]+8:end]]
		if !ok {
			continue
		}

		plain := text[position:at[0]]
		out.WriteString(plain)
		runes += utf8.RuneCountInString(plain)
		out.WriteString(value)
		spans = append(spans, Span{runes, runes + utf8.RuneCountInString(value)})
		runes += utf8.RuneCountInString(value)
		position = end
	}

	out.WriteString(text[position:])
	return out.String(), spans
}
