package contextual

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/erdajt/wraith/internal/modeltool"
	"github.com/erdajt/wraith/internal/runner"
)

type ToolRequest struct {
	Tool      string          `json:"tool"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	runner.Query
}

var commandID = regexp.MustCompile(`^[a-f0-9]{24}$`)

// RequestedTool extracts one complete tool request without granting execution authority.
func RequestedTool(content string) (*ToolRequest, error) {
	var request *ToolRequest
	for _, section := range commandSections(content, "wraith-tool") {
		if !section.complete || section.suggestion {
			continue
		}

		if request != nil {
			return nil, errors.New("request one tool at a time")
		}

		var decoded ToolRequest
		if err := decodeObject(section.text, &decoded); err != nil {
			return nil, err
		}
		if decoded.Tool == "extension" {
			if !modeltool.ValidName(decoded.Name) {
				return nil, errors.New("invalid extension tool name")
			}
			if err := modeltool.ValidateArguments(decoded.Arguments); err != nil {
				return nil, err
			}
		} else if (decoded.Tool != "poll" && decoded.Tool != "output") || !commandID.MatchString(decoded.ID) {
			return nil, errors.New("invalid output tool or command id")
		}
		request = &decoded
	}

	return request, nil
}

// displayTools replaces internal output requests with readable status labels.
func displayTools(content string) string {
	var out strings.Builder
	position := 0

	for _, section := range commandSections(content, "wraith-tool") {
		out.WriteString(content[position:section.start])

		if section.complete {
			var request ToolRequest
			if err := decodeObject(section.text, &request); err == nil {
				if request.Tool == "extension" {
					out.WriteString("[tool · " + request.Name + "]\n")
				} else {
					out.WriteString("[" + request.Tool + " · command " + request.ID + "]\n")
				}
			} else {
				out.WriteString("[invalid output request]\n")
			}
		} else {
			out.WriteString("[output request is incomplete]\n")
		}
		position = section.end
	}

	out.WriteString(content[position:])
	return out.String()
}

// ValidateTool checks mode and catalog membership before a model request reaches approval.
func ValidateTool(request *ToolRequest, mode string, catalog *modeltool.Catalog) error {
	if request == nil {
		return nil
	}
	if mode != "agent" {
		return errors.New("tool requests require agent execution mode")
	}
	if request.Tool == "extension" {
		return catalog.Validate(request.Name, request.Arguments)
	}
	if (request.Tool != "poll" && request.Tool != "output") || !commandID.MatchString(request.ID) {
		return errors.New("invalid output tool or command id")
	}
	if len(request.Pattern) > 4096 || request.From < 0 || request.Before < 0 || request.Before > 10 || request.After < 0 || request.After > 10 || request.Limit < 0 || request.Limit > 200 {
		return errors.New("invalid output query bounds")
	}
	return nil
}
