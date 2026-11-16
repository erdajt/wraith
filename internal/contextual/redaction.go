package contextual

import (
	"encoding/json"
	"sort"
	"strings"
)

func RedactActions(content string, redact func(string) string, ids map[string]bool) string {
	type replacement struct {
		start, end int
		text       string
	}
	var replacements []replacement
	for _, marker := range []string{"wraith-run", "wraith-tool"} {
		for _, section := range commandSections(content, marker) {
			if !section.complete {
				continue
			}
			var data []byte
			if marker == "wraith-run" {
				request, err := decodeRequest(section.text)
				if err != nil {
					continue
				}
				request.Command, request.Reason = redact(request.Command), redact(request.Reason)
				data, _ = json.Marshal(request)
			} else {
				var request ToolRequest
				if err := decodeObject(section.text, &request); err != nil {
					continue
				}
				if !ids[request.ID] {
					request.ID = redact(request.ID)
				}
				request.Pattern = redact(request.Pattern)
				data, _ = json.Marshal(request)
			}
			text := "```" + marker + "\n" + string(data) + "\n```\n"
			if section.suggestion {
				text = "```text\n" + marker + " " + string(data) + "\n```\n"
			}
			replacements = append(replacements, replacement{section.start, section.end, text})
		}
	}
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start < replacements[j].start })
	var out strings.Builder
	position := 0
	for _, replacement := range replacements {
		if replacement.start < position {
			continue
		}
		out.WriteString(redact(content[position:replacement.start]))
		out.WriteString(replacement.text)
		position = replacement.end
	}
	out.WriteString(redact(content[position:]))
	return out.String()
}
