package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestZshPreservesTrailingCommandNewlines(t *testing.T) {
	shell, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh absent")
	}
	home := t.TempDir()
	captured := filepath.Join(home, "captured")
	rc := `PS1='fixture> '
function inspect_wraith_buffer() { print -rn -- "$BUFFER" > ` + shellQuote(captured) + `; print -r -- "capture captured" >&4; }
zle -N inspect_wraith_buffer
bindkey '^G' inspect_wraith_buffer
`
	if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte(rc), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Start([]string{shell}, home, 80, 20, "HOME="+home, "ZDOTDIR="+home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	command := "printf x\\\n"
	id, err := p.QueuePaste(command)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(3 * time.Second)
	inserted := false
	for !inserted {
		select {
		case ev := <-p.Pastes:
			inserted = ev.ID == id && ev.Status == "inserted"
		case ev, ok := <-p.Events:
			if !ok || ev.Err != nil {
				t.Fatalf("shell stopped: %v", ev.Err)
			}
		case <-deadline:
			t.Fatal("no insertion")
		}
	}

	if err := p.Write([]byte{7}); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case event := <-p.Pastes:
			if event.ID != "capture" {
				continue
			}
			data, err := os.ReadFile(captured)
			if err != nil || string(data) != command {
				t.Fatalf("saved command altered: want %q, got %q, %v", command, data, err)
			}
			return
		case <-p.Events:
		case <-deadline:
			t.Fatal("capture timed out")
		}
	}
}
