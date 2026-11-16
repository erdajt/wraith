package contextual

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

func decodeObject(text string, out any) error {
	if len(text) > 32768 {
		return errors.New("action request exceeds 32 kib")
	}
	d := json.NewDecoder(strings.NewReader(text))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return errors.New("action request must be an object")
	}
	seen := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return errors.New("invalid action field")
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return errors.New("duplicate action field")
		}
		seen[name] = true
		var value json.RawMessage
		if err = d.Decode(&value); err != nil {
			return errors.New("invalid action value")
		}
	}
	if _, err = d.Token(); err != nil {
		return errors.New("unterminated action request")
	}
	if _, err = d.Token(); err != io.EOF {
		return errors.New("action request has trailing data")
	}
	d = json.NewDecoder(strings.NewReader(text))
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return errors.New("invalid action fields or value types")
	}
	return nil
}
