package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

func TestCommandRequestKeepsReasonAcrossChatApprovalAndBlocks(t *testing.T) {
	a := fixture(t)
	a.Config.Clipboard = nil
	content := `wraith-run {"command":"printf evidence","reason":"check captured output"}`
	a.receive(streamMsg{Final: true, Chunk: ollama.Response{Done: true, Message: session.Message{Role: "assistant", Content: content}}})
	if a.proposal == nil || a.proposal.Reason != "check captured output" {
		t.Fatal("reason was discarded")
	}
	view := ansi.Strip(a.View())
	if strings.Contains(view, "wraith-run") || !strings.Contains(view, "reason: check captured output") {
		t.Fatal("approval lost reason or leaked wrapper")
	}
	a.popup = nil
	a.follow = false
	view = ansi.Strip(a.View())
	if strings.Contains(view, "wraith-run") || !strings.Contains(view, "reason: check captured output") {
		t.Fatal("chat lost reason or leaked wrapper")
	}
	a.selected = 0
	a.selectedBlocks()
	if a.selectedBlock.Language != "sh" || strings.TrimSpace(a.selectedBlock.Text) != "printf evidence" {
		t.Fatal("block selection exposed protocol")
	}
	a.yank()
	if strings.Contains(string(a.Prompt.Register), "wraith-run") {
		t.Fatal("copy exposed protocol")
	}
	if strings.Contains(Export(a.Session), "wraith-run") {
		t.Fatal("export exposed protocol")
	}
}

func TestPlaceholderRequestNeverOpensApproval(t *testing.T) {
	a := fixture(t)
	a.receive(streamMsg{Final: true, Chunk: ollama.Response{Done: true, Message: session.Message{Role: "assistant", Content: `wraith-run {"command":"nmap <TARGET_IP_OR_HOST>","reason":"inspect supplied host"}`}}})
	if a.proposal != nil || a.popup != nil {
		t.Fatal("placeholder opened an execution request")
	}
	if _, _, err := editor.Stage(t.TempDir(), "sh", `wraith-run {"command":"printf x"}`); err == nil {
		t.Fatal("wrapper was accepted as a shell command")
	}
}
