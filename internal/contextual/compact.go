package contextual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

type Chatter interface {
	Chat(context.Context, ollama.Request, func(ollama.Response)) (ollama.Response, error)
}
type Memory struct {
	Summary string   `json:"summary"`
	Facts   []string `json:"facts"`
}

var literal = regexp.MustCompile("`[^`\\n]{1,1000}`|https?://[^\\s<>]+|(?:[0-9]{1,3}\\.){3}[0-9]{1,3}(?::[0-9]+)?|(?:~?/|[A-Za-z]:\\\\)[A-Za-z0-9_./\\\\:@%+-]+|CVE-[0-9]{4}-[0-9]+")

func Compact(ctx context.Context, client Chatter, s session.Session, c config.Config, instructions string, progress func(string)) (session.Session, error) {
	cut := len(s.Messages) - c.Compact.KeepTurns*2
	for cut > 0 && cut < len(s.Messages) && s.Messages[cut].Role != "user" {
		cut--
	}
	if cut <= 0 {
		return s, errors.New("not enough older turns to compact; pin key facts and start a new chat, or reduce keep_turns")
	}
	before := Used(s, instructions)
	candidate := s.Clone()
	candidate.Messages = append([]session.Message(nil), s.Messages[cut:]...)
	chunkBytes := (c.Context - c.Compact.Budget - Estimate([]session.Message{{Content: CompactPrompt + AuditPrompt}}) - 512) * 3 / 2
	if chunkBytes < 256 {
		return s, errors.New("compaction budget leaves no room for source fragments")
	}
	chunks := Fragments(s.Messages[:cut], chunkBytes)
	facts := append([]string(nil), s.Facts...)
	if s.Summary != "" {
		facts = append(facts, "previous working state: "+s.Summary)
	}
	var summaries []string
	for i, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return s, err
		}
		if progress != nil {
			progress(fmt.Sprintf("extracting fragment %d/%d", i+1, len(chunks)))
		}
		var memory Memory
		if err := extract(ctx, client, c, CompactPrompt, chunk, &memory); err != nil {
			return s, fmt.Errorf("fragment %d: %w", i+1, err)
		}
		if strings.TrimSpace(memory.Summary) == "" || len(memory.Facts) == 0 {
			return s, errors.New("compactor returned an empty memory; original context retained")
		}
		for _, f := range memory.Facts {
			if strings.TrimSpace(f) == "" {
				return s, errors.New("compactor returned an empty fact")
			}
		}
		encoded, _ := json.Marshal(memory)
		auditInput := "SOURCE FRAGMENT:\n" + chunk + "\nCANDIDATE MEMORY:\n" + string(encoded)
		if Estimate([]session.Message{{Content: AuditPrompt}, {Content: auditInput}})+c.Compact.Budget+128 > c.Context {
			return s, errors.New("memory audit exceeds context; lower compact.budget or increase context")
		}
		if progress != nil {
			progress(fmt.Sprintf("auditing fragment %d/%d", i+1, len(chunks)))
		}
		var audit struct {
			Missing []string `json:"missing"`
		}
		if err := extract(ctx, client, c, AuditPrompt, auditInput, &audit); err != nil {
			return s, fmt.Errorf("audit %d: %w", i+1, err)
		}
		if audit.Missing == nil {
			return s, errors.New("auditor omitted its missing-facts field")
		}
		facts = append(facts, memory.Facts...)
		facts = append(facts, audit.Missing...)
		for _, value := range literal.FindAllString(chunk, -1) {
			facts = append(facts, "source literal (refer to archive for surrounding evidence): "+value)
		}
		summaries = append(summaries, memory.Summary)
	}
	candidate.Summary = strings.Join(summaries, "\n")
	candidate.Facts = unique(facts)
	candidate.Declines = 0
	candidate.Usage = session.Usage{}
	after := Used(candidate, instructions)
	if after >= before {
		return s, errors.New("compaction did not reduce context without dropping protected facts; original context retained")
	}
	if err := Fits(candidate, c, instructions); err != nil {
		return s, fmt.Errorf("protected facts and recent turns still exceed the context window: %w", err)
	}
	return candidate, nil
}

func extract(ctx context.Context, client Chatter, c config.Config, system, input string, out any) error {
	r, err := client.Chat(ctx, ollama.Request{Model: c.Model, Messages: []session.Message{{Role: "system", Content: system}, {Role: "user", Content: input}}, Format: "json", KeepAlive: c.KeepAlive, Options: ollama.Options{Context: c.Context, Predict: c.Compact.Budget, Temperature: 0.1}}, nil)
	if err != nil {
		return err
	}
	if !r.Done || r.Reason == "length" {
		return errors.New("compaction response was truncated")
	}
	if err = json.Unmarshal([]byte(r.Message.Content), out); err != nil {
		return fmt.Errorf("invalid compaction json: %w", err)
	}
	return nil
}

func Fragments(messages []session.Message, limit int) []string {
	if limit < 128 {
		limit = 128
	}
	var out []string
	var buf strings.Builder
	for index, m := range messages {
		text := m.Content
		for part := 1; len(text) > 0; part++ {
			header := fmt.Sprintf("\nmessage %d, role=%s, part=%d:\n", index+1, m.Role, part)
			room := limit - buf.Len() - len(header)
			if room < 64 {
				out = append(out, buf.String())
				buf.Reset()
				room = limit - len(header)
			}
			n := min(len(text), room)
			for n > 0 && n < len(text) && !utf8.RuneStart(text[n]) {
				n--
			}
			buf.WriteString(header)
			buf.WriteString(text[:n])
			text = text[n:]
			if len(text) > 0 {
				out = append(out, buf.String())
				buf.Reset()
			}
		}
	}
	if buf.Len() > 0 {
		out = append(out, buf.String())
	}
	return out
}

func unique(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
