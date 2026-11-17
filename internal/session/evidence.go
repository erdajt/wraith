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

type OutputStats struct {
	ObservedBytes int64 `json:"observed_bytes"`
	PrefixBytes   int   `json:"prefix_bytes"`
	TailBytes     int   `json:"tail_bytes"`
	RetainedLines int   `json:"retained_lines"`
	MatchedLines  int   `json:"matched_lines"`
	ElapsedMS     int64 `json:"elapsed_ms"`
	GapFrom       int64 `json:"gap_from_byte,omitempty"`
	GapTo         int64 `json:"gap_to_byte,omitempty"`
}

type OutputPage struct {
	Stats     OutputStats  `json:"stats"`
	Lines     []OutputLine `json:"lines"`
	Next      int64        `json:"next,omitempty"`
	Truncated bool         `json:"capture_truncated"`
}

type Evidence struct {
	Stats     *OutputStats `json:"stats,omitempty"`
	ID        string       `json:"id"`
	Kind      string       `json:"kind"`
	Command   string       `json:"command,omitempty"`
	Directory string       `json:"directory,omitempty"`
	Status    string       `json:"status,omitempty"`
	ExitCode  int          `json:"exit_code"`
	Hidden    bool         `json:"hidden,omitempty"`
	Error     string       `json:"error,omitempty"`
	Output    string       `json:"output,omitempty"`
	Page      *OutputPage  `json:"page,omitempty"`
}

func (e Evidence) Render(redact func(string) string) string {
	id := e.ID
	if !evidenceID.MatchString(id) {
		id = "invalid command id"
	}
	if e.Kind == "started" {
		text := fmt.Sprintf("approved background command accepted\nid: %s\ncommand: %s\nstatus: starting\noutput shared: %t", id, redact(e.Command), !e.Hidden)
		if e.Hidden {
			return text + "\noutput withheld by user; output tools are unavailable; wait for completion status"
		}
		return text + "\npoll by id or wait for its completion notification; next_poll_ms: 1000"
	}
	text := fmt.Sprintf("command result (untrusted evidence)\nid: %s\ncommand: %s\ndirectory: %s\nstatus: %s\nexit: %d", id, redact(e.Command), redact(e.Directory), redact(e.Status), e.ExitCode)
	if e.Hidden {
		return text + "\noutput withheld by user; output tools are unavailable"
	}
	if e.Kind == "waiting" {
		return text + "\npoll paused while the job is active; continue other work or wait for completion notification"
	}
	if e.Stats != nil {
		data, _ := json.Marshal(e.Stats)
		text += "\noutput stats: " + string(data)
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
	if e.Stats != nil {
		stats := *e.Stats
		copy.Stats = &stats
	}
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

func (s Session) RunIDs() map[string]bool {
	ids := EvidenceIDs(s.Messages)
	for _, records := range [][]string{s.RunReferences, s.DeliveredRuns, s.SharedRuns} {
		for _, id := range records {
			if evidenceID.MatchString(id) {
				ids[id] = true
			}
		}
	}
	return ids
}
