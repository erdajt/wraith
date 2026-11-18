package variables

import (
	"strings"
	"testing"
)

func TestStreamingMarkersWaitForCompleteNames(t *testing.T) {
	t.Parallel()
	s := NewSnapshot([]Entry{{"TARGET", "example.test", ""}}, nil)
	text := "scan wraith::TARGET/path now"

	for split := range len(text) + 1 {
		prefix := text[:split]
		end, err := s.Validate(prefix, false)
		if err != nil {
			t.Fatalf("split %d: %v", split, err)
		}

		if strings.Contains(prefix[end:], "/") {
			t.Fatalf("complete marker remained buffered at %d", split)
		}

		if end, err := s.Validate(text, true); err != nil || end != len(text) {
			t.Fatalf("final: %d, %v", end, err)
		}
	}

	if end, err := s.Validate("wraith::TARGET_UNKNOWN ", false); err == nil || end != 0 {
		t.Fatal("valid prefix accepted as a different name")
	}
}

func TestMarkersRejectMalformedAndUnknownReferences(t *testing.T) {
	t.Parallel()
	s := NewSnapshot([]Entry{{"TARGET", "example.test", ""}}, nil)

	for _, text := range []string{"wraith::MISSING", "wraith:::TARGET", "wraith::TARGETsuffix", "wraith::TARGETé", "wraith:", "wraith::", "wraith::TARGET::MORE", "wraith::" + strings.Repeat("X", 65)} {
		if _, err := s.Validate(text, true); err == nil {
			t.Errorf("accepted %q", text)
		}
	}

	if _, err := s.Validate("wraith::TARGET:443", true); err != nil {
		t.Fatal(err)
	}
}

func TestExpansionTracksDisplayedUnicodeAndLeavesUnknownReferences(t *testing.T) {
	t.Parallel()
	s := NewSnapshot([]Entry{{"TARGET", "café.test", ""}}, nil)
	got, spans := s.Expand("→ wraith::TARGET wraith::MISSING wraith::TARGETsuffix")
	if got != "→ café.test wraith::MISSING wraith::TARGETsuffix" || len(spans) != 1 || spans[0] != (Span{2, 11}) {
		t.Fatalf("display %q, spans %+v", got, spans)
	}
}
