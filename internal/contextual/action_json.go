package contextual

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type ActionProtocolError struct {
	Problem    string
	Correction string
}

func (e *ActionProtocolError) Error() string { return e.Problem }

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
	switch out.(type) {
	case *CommandRequest:
		if seen["tool"] && !seen["command"] {
			return &ActionProtocolError{
				Problem:    "output query sent as a command request",
				Correction: "This is an output query, not a shell command. Keep the query object and place it in a fenced wraith-tool block instead of wraith-run. Do not wrap the query in a command or request shell execution. The from field is a one-based output line number, not a byte offset; use 1 to begin at the first line.",
			}
		}
	case *ToolRequest:
		if seen["command"] && !seen["tool"] {
			return &ActionProtocolError{
				Problem:    "command sent as an output query",
				Correction: "A shell command request belongs in a fenced wraith-run block with command and reason strings, not wraith-tool. It still requires user approval; nothing from the rejected response was executed.",
			}
		}
	}
	d = json.NewDecoder(strings.NewReader(text))
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return errors.New("invalid action fields or value types")
	}
	return nil
}
