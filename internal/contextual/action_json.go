package contextual

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type ActionProtocolError struct{ Target string }

// Error describes a request sent through the wrong action channel.
func (e *ActionProtocolError) Error() string {
	switch e.Target {
	case "output":
		return "output query sent as a command request"
	case "command":
		return "command sent as an output query"
	case "intent":
		return "command object needs an explicit execution or example format"
	default:
		return "action request used the wrong channel"
	}
}

// Correction returns model feedback for correcting the action channel.
func (e *ActionProtocolError) Correction() string {
	switch e.Target {
	case "output":
		return "This is an output query, not a shell command. Keep the query object and place it in a fenced wraith-tool block instead of wraith-run. Do not wrap the query in a command or request shell execution. The from field is a one-based output line number, not a byte offset; use 1 to begin at the first line."
	case "command":
		return "A shell command request belongs in a fenced wraith-run block with command and reason strings, not wraith-tool. It still requires user approval; nothing from the rejected response was executed."
	case "intent":
		return "Clarify whether you intend to request execution or only illustrate a command. Nothing from the rejected answer was executed. For an example, use an ordinary sh fence with shell text only. Only when requesting execution, use this exact block structure with your actual command and reason:\n```wraith-run\n{\"command\":\"printf '%s\\n' ready\",\"reason\":\"check shell output\"}\n```\nDo not wrap the request marker in a text, json or shell fence. Do not request execution for hypothetical or incomplete examples."
	default:
		return ""
	}
}

// decodeObject decodes a bounded action object, rejecting duplicate, unknown and mistyped fields.
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
		if string(value) == "null" {
			return errors.New("action fields cannot be null")
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
			return &ActionProtocolError{Target: "output"}
		}
	case *ToolRequest:
		if seen["command"] && !seen["tool"] {
			return &ActionProtocolError{Target: "command"}
		}
	}

	d = json.NewDecoder(strings.NewReader(text))
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return errors.New("invalid action fields or value types")
	}
	return nil
}
