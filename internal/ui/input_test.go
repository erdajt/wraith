package ui

import (
	"testing"

	"github.com/erdajt/wraith/internal/keymap"
)

func TestNativeMappingsRetainCountsOperatorsAndLiteralTargets(t *testing.T) {
	a := fixture(t)
	a.Config.Keymaps = []keymap.Binding{
		{Mode: "prompt.insert", Keys: "jk", Input: "<Esc>"},
		{Mode: "prompt.normal", Keys: "H", Input: "w"},
		{Mode: "prompt.operator", Keys: "H", Input: "e"},
	}
	var err error
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	a.Prompt.Set("one two H three")
	a.key(key("jk"))
	if a.Prompt.Mode != "normal" || a.Prompt.String() != "one two H three" {
		t.Fatal("insert chord did not leave insert mode without inserting text")
	}
	a.Prompt.Pos = 0
	a.key(key("2H"))
	if a.Prompt.Pos != 8 {
		t.Fatalf("native mapping lost count: %d", a.Prompt.Pos)
	}
	a.Prompt.Pos = 0
	a.key(key("fH"))
	if a.Prompt.Pos != 8 {
		t.Fatal("normal mapping intercepted literal find target")
	}
	a.Prompt.Pos = 0
	a.key(key("dH"))
	if a.Prompt.String() != " two H three" {
		t.Fatal("operator mapping did not apply native motion")
	}
}

func TestNativeInputCannotExecuteOrEscapeReadOnlyBuffers(t *testing.T) {
	for _, surface := range []string{"output", "terminal.copy", "terminal", "approval", "chat"} {
		t.Run(surface, func(t *testing.T) {
			a := fixture(t)
			r := newReader("read only evidence")
			switch surface {
			case "output":
				a.popup = &menu{Kind: "output", Reader: r}
			case "terminal.copy", "terminal":
				a.focus = "terminal0"
				a.panes[0] = &pane{}
				if surface == "terminal.copy" {
					a.panes[0].Reader = r
				}
			case "approval":
				a.popup = &menu{Kind: "approval"}
			case "chat":
				a.focus = "chat"
			}
			owner := a.keyOwner()
			a.feedInput("i:q!<CR>dd")
			a.setInputMode("insert")
			if a.keyOwner() != owner || r.Buffer.String() != "read only evidence" || a.busy || a.editing {
				t.Fatal("native input invoked workspace behavior or changed read-only text")
			}
			select {
			case <-a.done:
				t.Fatal("native input quit the application")
			default:
			}
		})
	}
}

func TestNativeInputEditsUnicodeAndYanksWithoutSending(t *testing.T) {
	a := fixture(t)
	a.feedInput("α 界<CR>:q")
	a.setInputMode("normal")
	a.feedInput("ggf界")
	if a.Prompt.Pos != 2 || a.Prompt.String() != "α 界\n:q" {
		t.Fatal("native input corrupted unicode or interpreted command text")
	}
	a.feedInput("0vey")
	if string(a.Prompt.Register) != "α 界" || len(a.Session.Messages) != 0 {
		t.Fatalf("native yank failed: %q", string(a.Prompt.Register))
	}
}

func TestNativeModeResetAndInvalidSequenceAreAtomic(t *testing.T) {
	a := fixture(t)
	a.Prompt.Set("retain this")
	a.Prompt.Mode, a.Prompt.Pos = "normal", 0
	a.feedInput("dd<F1>")
	if a.Prompt.String() != "retain this" || a.Prompt.Pending != "" {
		t.Fatal("invalid sequence partially changed the buffer")
	}
	a.focus, a.visualMode, a.searchMode, a.chatCount = "chat", "visual", true, "3"
	a.setInputMode("normal")
	if a.inputMode() != "chat.normal" || a.visualMode != "" || a.chatCount != "" {
		t.Fatal("normal mode retained search, selection or count")
	}
	a.popup = &menu{Kind: "output", Reader: newReader("retained")}
	a.popup.Reader.Buffer.Count = "3"
	a.setInputMode("normal")
	if a.popup.Reader.Buffer.Count != "" || a.popup.Reader.Buffer.Mode != "normal" {
		t.Fatal("reader mode reset retained count")
	}
}

func TestNativeCallbackTargetTracksReaderAndPendingIntent(t *testing.T) {
	a := fixture(t)
	a.popup = &menu{Kind: "output", Reader: newReader("same output")}
	before := a.pluginTarget()
	a.popup.Reader = newReader("same output")
	if a.pluginTarget() == before {
		t.Fatal("replaced reader retained callback ownership")
	}
	before = a.pluginTarget()
	a.popup.Reader.Buffer.Key("/", "")
	a.popup.Reader.Buffer.Key("x", "x")
	if a.pluginTarget() == before {
		t.Fatal("reader search change retained callback ownership")
	}
	a.popup, a.focus, a.chatMotionPending, a.chatMotionCount = nil, "chat", "y", "2"
	before = a.pluginTarget()
	a.chatMotionCount = "23"
	if a.pluginTarget() == before {
		t.Fatal("changed operator count retained callback ownership")
	}
}

func TestReaderModeChangePreservesUnderlyingConversationSelection(t *testing.T) {
	a := fixture(t)
	a.focus, a.visualMode, a.chatCount = "chat", "visual", "2"
	a.popup = &menu{Kind: "output", Reader: newReader("output")}
	a.setInputMode("visual")
	if a.visualMode != "visual" || a.chatCount != "2" || a.popup.Reader.Buffer.Mode != "visual" {
		t.Fatal("reader mode change altered underlying conversation")
	}
}
