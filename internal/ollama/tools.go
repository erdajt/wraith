package ollama

import "encoding/json"

type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type Call struct {
	Name      string
	Arguments json.RawMessage
}

type ActionError struct{ Problem string }

// Error describes a rejected native request without exposing its arguments.
func (e *ActionError) Error() string { return e.Problem }
