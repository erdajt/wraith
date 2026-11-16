package runner

import (
	"strings"
	"testing"
)

func TestCaptureRetainsActualTailAfterPrefixLimit(t *testing.T) {
	t.Parallel()
	total := 0
	c := capture{emit: func(s string) { total += len(s) }}
	input := strings.Repeat("old line\n", OutputLimit/9+100)
	for offset := 0; offset < len(input); offset += 4096 {
		_, _ = c.Write([]byte(input[offset:min(len(input), offset+4096)]))
	}
	_, _ = c.Write([]byte("latest result\n"))
	if c.text.Len() != OutputLimit || !c.truncated || !strings.HasSuffix(c.tail, "latest result\n") || len(c.tail) != TailLimit || c.observed != int64(total) {
		t.Fatal("capture lost retained prefix or actual tail")
	}
	removed := input[:len(input)+len("latest result\n")-TailLimit]
	if c.tailLine != int64(strings.Count(removed, "\n")) {
		t.Fatal("tail line offset is incorrect")
	}
}

func TestCommandEnvironmentIsExplicitAndCapturedOutputIsDurable(t *testing.T) {
	t.Parallel()
	h := History{Root: t.TempDir()}
	r, err := h.Run(t.Context(), Record{ID: "0123456789abcdef01234567", Shell: "/bin/sh", Command: `printf '%s' "$TARGET"`, Environment: []string{"TARGET=private.example"}}, nil)
	if err != nil || r.Output != "private.example" {
		t.Fatalf("environment not applied: %+v %v", r, err)
	}
	saved, err := h.Load(r.ID)
	if err != nil || saved.Tail != r.Output || saved.Observed != int64(len(r.Output)) || len(saved.Environment) != 0 {
		t.Fatalf("capture metadata not persisted correctly: %+v %v", saved, err)
	}
}
