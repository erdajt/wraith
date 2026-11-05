package contextual

import (
	"context"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

type retryChat struct {
	calls    int
	repair   bool
	feedback bool
}

func (f *retryChat) Chat(_ context.Context, r ollama.Request, _ func(ollama.Response)) (ollama.Response, error) {
	f.calls++
	text := "broken json"
	if f.calls > 1 {
		f.feedback = strings.Contains(r.Messages[0].Content, "invalid compaction json") && strings.Contains(r.Messages[0].Content, "broken json")
		if f.repair {
			text = `{"summary":"lab review","facts":["scope is the isolated lab"]}`
			if strings.HasPrefix(r.Messages[0].Content, AuditPrompt) {
				text = `{"missing":[]}`
			}
		}
	}
	return ollama.Response{Done: true, Message: session.Message{Content: text}}, nil
}

func TestCompactionRetriesWithFailureFeedback(t *testing.T) {
	f := &retryChat{repair: true}
	s := longSession()
	result, err := Compact(context.Background(), f, s, config.Default(), "", nil, nil)
	if err != nil || !f.feedback || len(result.Messages) >= len(s.Messages) {
		t.Fatalf("retry: %v, feedback %v", err, f.feedback)
	}
}

func TestCompactionStopsAfterThreeRetriesWithoutMutation(t *testing.T) {
	f := &retryChat{}
	s := longSession()
	result, err := Compact(context.Background(), f, s, config.Default(), "", nil, nil)
	if err == nil || f.calls != 4 || result.Summary != s.Summary || len(result.Messages) != len(s.Messages) {
		t.Fatalf("calls %d error %v", f.calls, err)
	}
}
