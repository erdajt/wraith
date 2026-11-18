package contextual

import (
	"errors"
	"regexp"
	"strings"

	"github.com/erdajt/wraith/internal/runner"
)

type ToolRequest struct {
	Tool string `json:"tool"`
	ID   string `json:"id"`
	runner.Query
}

var commandID = regexp.MustCompile(`^[a-f0-9]{24}$`)

// RequestedTool extracts one complete output-tool request and validates its tool and command id.
func RequestedTool(content string) (*ToolRequest, error) {
	var request *ToolRequest
	for _, section := range commandSections(content, "wraith-tool") {
		if !section.complete || section.suggestion {
			continue
		}

		if request != nil {
			return nil, errors.New("request one output tool at a time")
		}

		var decoded ToolRequest
		if err := decodeObject(section.text, &decoded); err != nil {
			return nil, err
		}
		if (decoded.Tool != "poll" && decoded.Tool != "output") || !commandID.MatchString(decoded.ID) {
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
				out.WriteString("[" + request.Tool + " · command " + request.ID + "]\n")
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
