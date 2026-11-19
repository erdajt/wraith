package ui

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/erdajt/wraith/internal/plugin"
	"github.com/erdajt/wraith/internal/session"
)

type nativeProjection struct {
	runtime  *plugin.Runtime
	session  string
	prompt   string
	partial  session.Message
	messages []session.Message
	ready    bool
}

// nativeSurfaceLines bounds display mirrors without silently presenting partial text as complete.
func nativeSurfaceLines(text string) ([]string, bool) {
	lossy := !utf8.ValidString(text)
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			lossy = true
			return -1
		}
		return r
	}, text)
	if len(text) > 1<<20 {
		text = text[len(text)-(1<<20):]
		for len(text) > 0 && !utf8.RuneStart(text[0]) {
			text = text[1:]
		}
		lossy = true
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 100000 {
		lines, lossy = lines[len(lines)-100000:], true
	}
	for i, line := range lines {
		if len(line) <= 64<<10 {
			continue
		}
		end := 64 << 10
		for !utf8.RuneStart(line[end]) {
			end--
		}
		lines[i], lossy = line[:end], true
	}
	return lines, lossy
}

// syncNativeSurfaces publishes changed native text before plugins inspect explicit buffer revisions.
func (a *App) syncNativeSurfaces() {
	if a.plugins == nil || a.plugins.Surfaces().OwnerLease("core") == "" {
		return
	}
	if a.nativeProjection == nil || a.nativeProjection.runtime != a.plugins {
		a.nativeProjection = &nativeProjection{runtime: a.plugins}
	}
	state := a.nativeProjection
	prompt := a.Prompt.String()
	if !state.ready || state.prompt != prompt {
		lines, truncated := nativeSurfaceLines(prompt)
		delta, err := a.plugins.Surfaces().HostUpsert("prompt", lines, truncated)
		if err != nil {
			a.notice = "lua prompt projection: " + err.Error()
			return
		}
		a.applySurfaceDelta(delta, true)
		state.prompt = prompt
	}
	if state.ready && state.session == a.Session.ID && state.partial == a.partial && slices.Equal(state.messages, a.Session.Messages) {
		return
	}

	parts, size := []string{}, 0
	if a.partial.Content != "" {
		part := a.partial.Role + "\n" + a.partial.Content
		parts, size = append(parts, part), len(part)
	}
	start := len(a.Session.Messages) - 1
	for ; start >= 0 && size < 1<<20; start-- {
		message := a.Session.Messages[start]
		part := message.Role + "\n" + message.Content
		parts, size = append(parts, part), size+len(part)+2
	}
	slices.Reverse(parts)
	lines, truncated := nativeSurfaceLines(strings.Join(parts, "\n\n"))
	delta, err := a.plugins.Surfaces().HostUpsert("conversation", lines, truncated || start >= 0)
	if err != nil {
		a.notice = "lua conversation projection: " + err.Error()
		return
	}
	a.applySurfaceDelta(delta, true)
	state.session, state.partial, state.ready = a.Session.ID, a.partial, true
	state.messages = slices.Clone(a.Session.Messages)
}

// adoptNativeSurface applies an explicit revision-checked prompt edit to the native editor.
func (a *App) adoptNativeSurface(buffer plugin.SurfaceBuffer) {
	if buffer.Projection != "prompt" || buffer.Truncated {
		return
	}
	text := strings.Join(buffer.Lines, "\n")
	if text != a.Prompt.String() {
		a.Prompt.SyncText(text, true)
		a.inputIndex, a.completionIndex = -1, 0
		a.completionQuery = strings.TrimPrefix(text, "/")
	}
	if a.nativeProjection != nil {
		a.nativeProjection.prompt = text
	}
}
