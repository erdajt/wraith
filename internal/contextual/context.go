package contextual

import (
	"fmt"
	"math"
	"strings"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/session"
)

func Messages(s session.Session, instructions string) []session.Message {
	system := System
	if strings.TrimSpace(instructions) != "" {
		system += "\n\nPersistent user instructions:\n" + instructions
	}
	if len(s.Pins) > 0 {
		system += "\n\nExact user-pinned context:\n- " + strings.Join(s.Pins, "\n- ")
	}
	if s.Summary != "" {
		system += "\n\nCompacted working state:\n" + s.Summary
	}
	if len(s.Facts) > 0 {
		system += "\n\nHistorical facts (retain attribution and corrections):\n- " + strings.Join(s.Facts, "\n- ")
	}
	out := []session.Message{{Role: "system", Content: system}}
	for _, m := range s.Messages {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		out = append(out, session.Message{Role: m.Role, Content: m.Content})
	}
	return out
}

func Estimate(messages []session.Message) int {
	n := 16
	for _, m := range messages {
		n += 12 + (len(m.Content)+2)/3
	}
	return n
}

func Used(s session.Session, instructions string) int {
	n := Estimate(Messages(s, instructions))
	if s.Usage.InputEstimate > 0 && s.Usage.Prompt > s.Usage.InputEstimate {
		n = int(math.Ceil(float64(n) * float64(s.Usage.Prompt) / float64(s.Usage.InputEstimate)))
	}
	return n
}

func NeedsCompact(s session.Session, c config.Config, instructions string) bool {
	return ChatUsed(s, c, instructions) >= int(float64(c.Context)*c.Compact.Threshold)
}

func Fits(s session.Session, c config.Config, instructions string) error {
	n := ChatUsed(s, c, instructions)
	if n+c.Predict+128 > c.Context {
		return fmt.Errorf("context budget: about %d input + %d output tokens exceeds %d; compact, trim the prompt, or increase context", n, c.Predict, c.Context)
	}
	return nil
}

func Decision(mode string, declines int, over bool) string {
	if !over || mode == "manual" {
		return "none"
	}
	if mode == "auto" || declines >= 3 {
		return "compact"
	}
	return "ask"
}
