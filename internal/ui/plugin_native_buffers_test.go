package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/erdajt/wraith/internal/plugin"
	"github.com/erdajt/wraith/internal/session"
)

// TestNativeBufferEditsUseCurrentPromptRevision prevents stale explicit edits from replacing new input.
func TestNativeBufferEditsUseCurrentPromptRevision(t *testing.T) {
	a := pluginFixture(t, "")
	a.Prompt.Set("original")
	a.Session.Messages = []session.Message{{Role: "user", Content: "question"}, {Role: "assistant", Content: "answer"}}
	a.syncNativeSurfaces()
	var prompt plugin.SurfaceBufferInfo
	for _, buffer := range a.plugins.Surfaces().Index().Buffers {
		if buffer.Projection == "prompt" {
			prompt = buffer
		}
	}
	mutation := plugin.SurfaceMutation{Operation: "buffer.set_lines", ID: prompt.ID, Owner: "config", Lease: a.plugins.Surfaces().OwnerLease("config"), Expected: prompt.Revision, End: 1, Lines: []string{"plugin edit"}}
	if err := a.applySurfaceMutation(a.plugins, mutation); err != nil {
		t.Fatal(err)
	}
	if a.Prompt.String() != "plugin edit" {
		t.Fatal("accepted explicit edit did not reach native prompt")
	}
	a.Prompt.Set("new user input")
	a.syncNativeSurfaces()
	if err := a.applySurfaceMutation(a.plugins, mutation); err == nil || a.Prompt.String() != "new user input" {
		t.Fatal("stale explicit edit replaced a newer prompt")
	}
	first, err := a.plugins.Surfaces().Buffer(prompt.ID)
	if err != nil {
		t.Fatal(err)
	}
	a.syncNativeSurfaces()
	second, err := a.plugins.Surfaces().Buffer(prompt.ID)
	if err != nil || first.Revision != second.Revision {
		t.Fatal("unchanged native prompt advanced its revision")
	}
}

// TestIncompleteNativePromptCannotReplaceFullDraft keeps bounded display mirrors non-destructive.
func TestIncompleteNativePromptCannotReplaceFullDraft(t *testing.T) {
	a := pluginFixture(t, "")
	text := strings.Repeat("界", 24000)
	a.Prompt.Set(text)
	a.syncNativeSurfaces()
	for _, info := range a.plugins.Surfaces().Index().Buffers {
		if info.Projection != "prompt" {
			continue
		}
		if !info.Truncated || info.Editable || a.Prompt.String() != text {
			t.Fatal("partial mirror altered or exposed an editable full draft")
		}
		buffer, err := a.plugins.Surfaces().Buffer(info.ID)
		if err != nil || !utf8.ValidString(buffer.Lines[0]) {
			t.Fatal("projection split a multibyte character")
		}
	}
}

// TestNativeProjectionReportsDiscardedControlsAndRetainsTabs validates lossless editable mirrors.
func TestNativeProjectionReportsDiscardedControlsAndRetainsTabs(t *testing.T) {
	for _, test := range []struct {
		input, want string
		incomplete  bool
	}{
		{"one\t界\ntwo", "one\t界\ntwo", false},
		{"one\x00two", "onetwo", true},
		{string([]byte{'a', 255, 'b'}), "a�b", true},
	} {
		lines, incomplete := nativeSurfaceLines(test.input)
		if strings.Join(lines, "\n") != test.want || incomplete != test.incomplete {
			t.Fatalf("projection %q: %q, incomplete=%v", test.input, lines, incomplete)
		}
	}
}
