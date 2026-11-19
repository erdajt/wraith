package plugin

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

type WorkspaceQuery struct {
	Invocation             string
	ID, Owner, Lease, Kind string
	Options                WorkspaceOptions
}

type WorkspaceOptions struct {
	ID         string `json:"id"`
	Session    string `json:"session"`
	Directory  string `json:"directory"`
	Cursor     string `json:"cursor"`
	Revision   string `json:"revision"`
	Field      string `json:"field"`
	Pattern    string `json:"pattern"`
	From       int64  `json:"from"`
	Limit      int    `json:"limit"`
	Index      int    `json:"index"`
	Before     int    `json:"before"`
	After      int    `json:"after"`
	Regex      bool   `json:"regex"`
	IgnoreCase bool   `json:"ignore_case"`
}

var workspaceFields = map[string]string{
	"sessions.list":     "cursor limit directory",
	"sessions.get":      "id directory",
	"sessions.messages": "id revision from limit",
	"sessions.message":  "id revision index field from limit",
	"jobs.list":         "session directory cursor limit",
	"jobs.get":          "id",
	"jobs.output":       "id revision from pattern regex ignore_case before after limit",
	"environment.list":  "",
	"terminals.list":    "",
}

// Validate checks query bounds independently of the Lua caller before host work starts.
func (q WorkspaceQuery) Validate() error {
	if _, ok := workspaceFields[q.Kind]; !ok {
		return errors.New("unknown workspace query")
	}
	o := q.Options
	for _, id := range []string{o.ID, o.Session} {
		if id != "" {
			if _, err := hex.DecodeString(id); err != nil || len(id) != 24 || strings.ToLower(id) != id {
				return errors.New("invalid session or command id")
			}
		}
	}
	if (q.Kind == "jobs.get" || q.Kind == "jobs.output") && o.ID == "" {
		return errors.New("command id is required")
	}
	if o.Session != "" && o.Directory != "" {
		return errors.New("choose session or directory scope")
	}
	if len(o.Directory) > 4096 || strings.ContainsRune(o.Directory, 0) || len(o.Cursor) > 256 || len(o.Revision) > 128 || len(o.Pattern) > 4096 {
		return errors.New("workspace query field exceeds its limit")
	}
	if o.From < 0 || o.From > 1<<53 || o.Index < 0 || o.Index > 1<<20 || o.Before < 0 || o.Before > 10 || o.After < 0 || o.After > 10 {
		return errors.New("invalid workspace query range")
	}
	maximum := 100
	if q.Kind == "jobs.output" {
		maximum = 200
	} else if q.Kind == "sessions.message" {
		maximum = 64 << 10
		if o.Revision == "" || o.Field != "" && o.Field != "content" && o.Field != "thinking" {
			return errors.New("message reads require a revision and content or thinking field")
		}
	}
	if o.Limit < 0 || o.Limit > maximum {
		return fmt.Errorf("query limit must be between 0 and %d", maximum)
	}
	return nil
}
