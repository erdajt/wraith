package contextual

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/erdajt/wraith/internal/ollama"
)

// ActionTools returns the native request schemas available in the selected execution mode.
func ActionTools(mode string) []ollama.Tool {
	if mode != "agent" && mode != "blocks" {
		return nil
	}
	tools := []ollama.Tool{{Type: "function", Function: ollama.Function{
		Name: "request_command", Description: "request user approval for one concrete shell command; this does not execute it",
		Parameters: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["command","reason"],"properties":{"command":{"type":"string"},"reason":{"type":"string"},"background":{"type":"boolean"}}}`),
	}}}
	if mode == "agent" {
		tools = append(tools, ollama.Tool{Type: "function", Function: ollama.Function{
			Name: "read_output", Description: "poll or search authorized job output by its durable id; from is a one-based line number",
			Parameters: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["tool","id"],"properties":{"tool":{"type":"string","enum":["poll","output"]},"id":{"type":"string","pattern":"^[a-f0-9]{24}$"},"pattern":{"type":"string","maxLength":4096},"regex":{"type":"boolean"},"ignore_case":{"type":"boolean"},"from":{"type":"integer","minimum":0},"before":{"type":"integer","minimum":0,"maximum":10},"after":{"type":"integer","minimum":0,"maximum":10},"limit":{"type":"integer","minimum":0,"maximum":200}}}`),
		}})
	}
	return tools
}

// NormalizeAction converts one complete native request into the validated conversation format.
func NormalizeAction(mode, content string, calls []ollama.Call) (string, error) {
	if len(calls) != 1 {
		return "", errors.New("request exactly one native action per response")
	}
	if len(commandSections(content, "wraith-run"))+len(commandSections(content, "wraith-tool")) > 0 || UnclearCommandIntent(content) != nil {
		return "", errors.New("do not mix native actions with text action requests")
	}

	call := calls[0]
	marker := ""
	var value any
	switch call.Name {
	case "request_command":
		if mode != "agent" && mode != "blocks" {
			return "", errors.New("command requests are unavailable in this request")
		}
		request, err := decodeRequest(string(call.Arguments))
		if err != nil {
			return "", err
		}
		if placeholder.MatchString(request.Command) {
			return "", errors.New("native command contains unresolved placeholders; ask for missing values instead")
		}
		marker, value = "wraith-run", request
	case "read_output":
		if mode != "agent" {
			return "", errors.New("output requests require agent execution mode")
		}
		var request ToolRequest
		if err := decodeObject(string(call.Arguments), &request); err != nil {
			return "", err
		}
		if (request.Tool != "poll" && request.Tool != "output") || !commandID.MatchString(request.ID) {
			return "", errors.New("invalid native output request")
		}
		if len(request.Pattern) > 4096 || request.From < 0 || request.Before < 0 || request.Before > 10 || request.After < 0 || request.After > 10 || request.Limit < 0 || request.Limit > 200 {
			return "", errors.New("invalid output query bounds")
		}
		marker, value = "wraith-tool", request
	default:
		return "", errors.New("unknown native tool; use an advertised function")
	}

	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(content) + "\n\n```" + marker + "\n" + string(data) + "\n```", nil
}
