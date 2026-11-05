package contextual

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

type fakeChat struct {
	calls     int
	fail      bool
	truncated bool
	audit     bool
}

func (f *fakeChat) Chat(_ context.Context, r ollama.Request, _ func(ollama.Response)) (ollama.Response, error) {
	f.calls++
	if f.fail {
		return ollama.Response{}, errors.New("offline")
	}
	text := `{"summary":"lab assessment in progress","facts":["user owns the isolated lab","service listens on 10.0.0.7:8443"]}`
	if r.Messages[0].Content == AuditPrompt {
		f.audit = true
		text = `{"missing":["the first connection attempt failed with timeout"]}`
	}
	reason := "stop"
	if f.truncated {
		reason = "length"
	}
	return ollama.Response{Done: true, Reason: reason, Message: session.Message{Content: text}}, nil
}

func longSession() session.Session {
	s := session.New("local")
	s.Facts = []string{"do not modify /srv/production"}
	s.Pins = []string{"scope is 10.0.0.7 only"}
	for i := 0; i < 12; i++ {
		s.Messages = append(s.Messages, session.Message{Role: "user", Content: strings.Repeat("lab evidence for 10.0.0.7:8443 and `curl --head` remains unchanged. ", 10)}, session.Message{Role: "assistant", Content: strings.Repeat("check the response and preserve the exact service configuration. ", 10)})
	}
	return s
}

func TestCompactionPreservesLedgerPinsRecentTurnsAndAudit(t *testing.T) {
	s := longSession()
	c := config.Default()
	f := &fakeChat{}
	got, err := Compact(context.Background(), f, s, c, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !f.audit || f.calls < 2 {
		t.Fatal("audit was not performed")
	}
	if len(got.Messages) != 4 || got.Messages[0] != s.Messages[len(s.Messages)-4] {
		t.Fatal("recent turns changed")
	}
	if got.Pins[0] != s.Pins[0] || got.Facts[0] != s.Facts[0] {
		t.Fatal("protected context changed")
	}
	facts := strings.Join(got.Facts, "\n")
	for _, want := range []string{"10.0.0.7:8443", "`curl --head`", "timeout"} {
		if !strings.Contains(facts, want) {
			t.Fatalf("lost %q", want)
		}
	}
	if len(s.Messages) != 24 {
		t.Fatal("input was mutated")
	}
}

func TestFailedCompactionLeavesOriginal(t *testing.T) {
	for _, f := range []*fakeChat{{fail: true}, {truncated: true}} {
		s := longSession()
		got, err := Compact(context.Background(), f, s, config.Default(), "", nil, nil)
		if err == nil || len(got.Messages) != len(s.Messages) || got.Summary != "" {
			t.Fatal("failed compaction replaced context")
		}
	}
}

func TestFragmentsBoundedAndUnicodeSafe(t *testing.T) {
	text := strings.Repeat("🜏界 evidence\n", 200)
	chunks := Fragments([]session.Message{{Role: "user", Content: text}}, 200)
	if len(chunks) < 2 {
		t.Fatal("expected chunks")
	}
	var recovered strings.Builder
	for _, chunk := range chunks {
		if len(chunk) > 200 || !utf8.ValidString(chunk) {
			t.Fatal("invalid fragment")
		}
		_, content, _ := strings.Cut(strings.TrimPrefix(chunk, "\n"), "\n")
		recovered.WriteString(content)
	}
	if recovered.String() != text {
		t.Fatal("fragmentation lost source bytes")
	}
}

func TestCompactionPolicy(t *testing.T) {
	for n := 0; n < 3; n++ {
		if Decision("ask", n, true) != "ask" {
			t.Fatal("asked policy skipped confirmation")
		}
	}
	if Decision("ask", 3, true) != "compact" || Decision("auto", 0, true) != "compact" || Decision("manual", 3, true) != "none" || Decision("ask", 3, false) != "none" {
		t.Fatal("wrong compaction policy")
	}
}

func TestContextReservesOutputAndKeepsInstructions(t *testing.T) {
	s := session.New("local")
	s.Pins = []string{"retain this exact pin"}
	m := Messages(s, "always use metric units")
	if !strings.Contains(m[0].Content, "always use metric units") || !strings.Contains(m[0].Content, s.Pins[0]) {
		t.Fatal("instructions missing")
	}
	s.Messages = []session.Message{{Role: "user", Content: strings.Repeat("a", 30000)}}
	if Fits(s, config.Default(), "") == nil {
		t.Fatal("overflow was accepted")
	}
}
