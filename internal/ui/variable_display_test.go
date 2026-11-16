package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

func TestPrivateValuesRenderAfterMarkdownParsing(t *testing.T) {
	a := fixture(t)
	a.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "**literal** `text`"}}, nil)
	a.Session.Messages = []session.Message{{Role: "assistant", Content: "**inspect wraith::TARGET**\n\n```sh\necho wraith::TARGET\n```"}}
	var plain, styled strings.Builder
	blocks := 0
	for _, row := range a.transcript(100) {
		plain.WriteString(ansi.Strip(row.Text))
		styled.WriteString(row.Text)
		if row.Block >= 0 {
			blocks++
		}
	}
	if !strings.Contains(plain.String(), "inspect **literal** `text`") || !strings.Contains(plain.String(), "echo **literal** `text`") || strings.Contains(plain.String(), "wraith::TARGET") {
		t.Fatal("value was parsed as markdown or left unresolved")
	}
	if blocks != 3 || !strings.Contains(styled.String(), color(a.Config.Palette().Variable, false)) {
		t.Fatal("substitution styling or block identity missing")
	}
	if a.Session.Messages[0].Content != "**inspect wraith::TARGET**\n\n```sh\necho wraith::TARGET\n```" {
		t.Fatal("local rendering changed stored context")
	}
}

func TestPrivateMarkerExpandsBeforeWrapping(t *testing.T) {
	a := fixture(t)
	a.Variables = variables.NewSnapshot([]variables.Entry{{Name: "VERY_LONG_TARGET_NAME", Value: "host"}}, nil)
	a.Session.Messages = []session.Message{{Role: "assistant", Content: "wraith::VERY_LONG_TARGET_NAME"}}
	var output strings.Builder
	for _, row := range a.transcript(10) {
		output.WriteString(ansi.Strip(row.Text))
	}
	if !strings.Contains(output.String(), "host") || strings.Contains(output.String(), "wraith::") {
		t.Fatal("marker wrapped before substitution")
	}
}
