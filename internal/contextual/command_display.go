package contextual

import (
	"encoding/json"
	"strings"
)

type commandSection struct {
	start, end int
	text       string
	complete   bool
}

func commandSections(content string) []commandSection {
	lines := strings.SplitAfter(content, "\n")
	var sections []commandSection
	offset := 0
	fence := ""
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if fence != "" {
			if line == fence {
				fence = ""
			}
			offset += len(lines[i])
			continue
		}
		mark := ""
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			n := 0
			for n < len(line) && line[n] == line[0] {
				n++
			}
			mark = line[:n]
			if strings.TrimSpace(strings.TrimPrefix(line, mark)) != "wraith-run" {
				fence = mark
				offset += len(lines[i])
				continue
			}
		} else if !strings.HasPrefix(line, "wraith-run") {
			offset += len(lines[i])
			continue
		}
		section := commandSection{start: offset}
		if mark != "" {
			offset += len(lines[i])
			for i++; i < len(lines); i++ {
				offset += len(lines[i])
				if strings.TrimSpace(lines[i]) == mark {
					section.complete = true
					break
				}
				section.text += lines[i]
			}
		} else {
			section.text = strings.TrimSpace(strings.TrimPrefix(line, "wraith-run"))
			offset += len(lines[i])
			for {
				var raw json.RawMessage
				decoder := json.NewDecoder(strings.NewReader(section.text))
				if decoder.Decode(&raw) == nil {
					section.complete = true
					break
				}
				if i+1 >= len(lines) {
					break
				}
				i++
				section.text += "\n" + lines[i]
				offset += len(lines[i])
			}
		}
		section.end = offset
		sections = append(sections, section)
	}
	return sections
}

func DisplayContent(content string) string {
	var out strings.Builder
	position := 0
	for _, section := range commandSections(content) {
		out.WriteString(content[position:section.start])
		request, err := decodeRequest(section.text)
		if section.complete && err == nil {
			fence := "```"
			for strings.Contains(request.Command, fence) {
				fence += "`"
			}
			out.WriteString(fence + "sh\n" + request.Command + "\n" + fence + "\n\nreason: " + request.Reason + "\n")
		} else if section.complete {
			out.WriteString("[invalid command request; nothing was run]\n")
		} else {
			out.WriteString("[command request is incomplete]\n")
		}
		position = section.end
	}
	out.WriteString(content[position:])
	return out.String()
}
