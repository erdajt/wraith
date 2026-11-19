package ui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
)

// workspaceSessionRevision identifies transcript changes without copying message bodies into JSON.
func workspaceSessionRevision(value session.Session) string {
	hash := sha256.New()
	for _, message := range value.Messages {
		fmt.Fprintf(hash, "%s:%d:%d:%t:%t:", message.Role, len(message.Content), len(message.Thinking), message.Edited, message.Interrupted)
		_, _ = io.WriteString(hash, message.Content)
		_, _ = io.WriteString(hash, message.Thinking)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// workspaceSessionMetadata exposes navigation fields without sending full transcripts to callbacks.
func workspaceSessionMetadata(value session.Session) map[string]any {
	return map[string]any{
		"id": value.ID, "title": value.Title, "directory": value.Directory,
		"model": value.Model, "provider": value.Provider, "created": value.Created,
		"updated": value.Updated, "message_count": len(value.Messages), "draft_bytes": len(value.Draft),
		"revision": workspaceSessionRevision(value),
	}
}

// workspaceJobMetadata excludes captures and private process environments from catalog pages.
func workspaceJobMetadata(value runner.Record) map[string]any {
	data := map[string]any{
		"id": value.ID, "session": value.Session, "directory": value.Directory,
		"command": value.Command, "shell": value.Shell, "source": value.Source,
		"started": value.Started, "finished": value.Finished, "status": value.Status,
		"exit_code": value.ExitCode, "error": value.Error, "background": value.Background,
		"agent": value.Agent, "share_output": value.ShareOutput, "output_hidden": value.OutputHidden,
		"observed_bytes": value.Observed, "captured_bytes": value.Captured, "truncated": value.Truncated,
	}
	encoded, _ := json.Marshal(data)
	revision := sha256.Sum256(encoded)
	data["revision"] = hex.EncodeToString(revision[:])
	return data
}

// workspacePreview clips valid UTF-8 without retaining unbounded message text in a page.
func workspacePreview(text string, limit int) string {
	if len(text) <= limit {
		return strings.Clone(strings.ToValidUTF8(text, "�"))
	}
	for limit > 0 && !utf8.RuneStart(text[limit]) {
		limit--
	}
	return strings.Clone(strings.ToValidUTF8(text[:limit], "�"))
}
