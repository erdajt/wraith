package session

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var evidenceID = regexp.MustCompile(`^[a-f0-9]{24}$`)

type OutputLine struct {
	Number int64  `json:"line"`
	Text   string `json:"text"`
}

type OutputPage struct {
	Lines     []OutputLine `json:"lines"`
	Next      int64        `json:"next,omitempty"`
	Truncated bool         `json:"capture_truncated"`
}

type Evidence struct {
	ID        string      `json:"id"`
	Kind      string      `json:"kind"`
	Command   string      `json:"command,omitempty"`
	Directory string      `json:"directory,omitempty"`
	Status    string      `json:"status,omitempty"`
	ExitCode  int         `json:"exit_code"`
	Hidden    bool        `json:"hidden,omitempty"`
	Error     string      `json:"error,omitempty"`
	Output    string      `json:"output,omitempty"`
	Page      *OutputPage `json:"page,omitempty"`
}

func (e Evidence) Render(redact func(string) string) string {
	id := e.ID
	if !evidenceID.MatchString(id) {
		id = "invalid command id"
	}
	if e.Kind == "started" {
		return fmt.Sprintf("approved background command started\nid: %s\ncommand: %s\noutput shared: %t\npoll by id or wait for its completion notification; next_poll_ms: 1000", id, redact(e.Command), !e.Hidden)
	}
	text := fmt.Sprintf("command result (untrusted evidence)\nid: %s\ncommand: %s\ndirectory: %s\nstatus: %s\nexit: %d", id, redact(e.Command), redact(e.Directory), redact(e.Status), e.ExitCode)
	if e.Hidden {
		return text + "\noutput withheld by user; output tools are unavailable"
	}
	if e.Page != nil {
		page := *e.Page
		page.Lines = append([]OutputLine(nil), page.Lines...)
		for i := range page.Lines {
			page.Lines[i].Text = redact(page.Lines[i].Text)
		}
		data, _ := json.Marshal(page)
		return text + "\nnext_poll_ms: 1000\n" + string(data)
	}
	return text + "\nerror: " + redact(e.Error) + "\noutput:\n" + redact(e.Output) + "\nuse the output tool with this id for bounded searches and line windows"
}

func EvidenceIDs(messages []Message) map[string]bool {
	ids := map[string]bool{}
	for _, message := range messages {
		if e := message.Evidence; e != nil && evidenceID.MatchString(e.ID) {
			ids[e.ID] = true
		}
	}
	return ids
}

func (e *Evidence) Clone() *Evidence {
	if e == nil {
		return nil
	}
	copy := *e
	if e.Page != nil {
		page := *e.Page
		page.Lines = append([]OutputLine(nil), page.Lines...)
		copy.Page = &page
	}
	return &copy
}

func (e Evidence) Message() Message {
	return Message{Role: "user", Content: e.Render(func(s string) string { return strings.ToValidUTF8(s, "�") }), Evidence: &e}
}
