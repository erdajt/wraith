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

func TestSmallApprovalKeepsCommandAndSelectedActionVisible(t *testing.T) {
	a := fixture(t)
	a.width, a.height = 50, 18
	a.approve(proposal{Command: "printf evidence", Reason: "inspect output"})
	for i := range a.popup.Items {
		a.popup.Index = i
		frame := a.menuFrame()
		view := ansi.Strip(strings.Join(frame, "\n"))
		if len(frame) > a.height || !strings.Contains(view, "printf evidence") || !strings.Contains(view, "reason: inspect output") || !strings.Contains(view, "› "+a.popup.Items[i].Label) {
			t.Fatalf("approval hid command, reason or action %d: %s", i, view)
		}
	}
	a.proposal.Command = strings.Repeat("long command argument ", 20)
	a.popup.Index = len(a.popup.Items) - 1
	view := ansi.Strip(strings.Join(a.menuFrame(), "\n"))
	if !strings.Contains(view, "› reject") || !strings.Contains(view, "view full command") {
		t.Fatal("long preview hid selected action or full preview access")
	}
}
