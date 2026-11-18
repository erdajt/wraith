package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestZshPasteChangesBufferWithoutExecutingAndRetainsStartup(t *testing.T) {
	shell, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	home := t.TempDir()
	config := filepath.Join(home, "shell")
	if err := os.Mkdir(config, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".zshenv"), []byte("export ZDOTDIR="+shellQuote(config)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, ".zshrc"), []byte("PS1='fixture> '\nprint startup-preserved\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Start([]string{shell}, home, 80, 20, "HOME="+home, "ZDOTDIR="+home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	marker := filepath.Join(home, "executed")
	id, err := p.QueuePaste("printf '%s' first > " + shellQuote(marker) + "\nprintf '%s' second >> " + shellQuote(marker))
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	wait := func(wantID, status string) {
		t.Helper()
		deadline := time.After(3 * time.Second)
		for {
			select {
			case event, ok := <-p.Events:
				if !ok || event.Err != nil {
					t.Fatalf("shell stopped: %v, output=%q", event.Err, output.String())
				}
				output.Write(event.Data)
			case event := <-p.Pastes:
				if event.ID == wantID {
					if event.Status != status {
						t.Fatalf("unexpected insertion: %+v, output=%q", event, output.String())
					}
					return
				}
			case <-deadline:
				t.Fatalf("insertion timed out: %q", output.String())
			}
		}
	}
	wait(id, "inserted")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("saving executed the command")
	}
	second, err := p.QueuePaste("printf wrong")
	if err != nil {
		t.Fatal(err)
	}
	wait(second, "occupied")
	if err := p.Write([]byte("\r")); err != nil {
		t.Fatal(err)
	}
	third, err := p.QueuePaste("printf next")
	if err != nil {
		t.Fatal(err)
	}
	wait(third, "inserted")
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "firstsecond" {
		t.Fatalf("explicit enter did not execute the complete multiline command: %q, %v", data, err)
	}
	if !strings.Contains(output.String(), "startup-preserved") {
		t.Fatal("user startup configuration was skipped")
	}
}

func TestUnsupportedShellRetainsManualInsertion(t *testing.T) {
	p, err := Start([]string{"/bin/sh", "-c", "read value"}, "", 40, 10)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	if _, err := p.QueuePaste("printf never"); err == nil {
		t.Fatal("unsupported shell claimed automatic insertion")
	}
	for _, value := range []string{"a'b", "path with space", "$(touch marker)"} {
		quoted := shellQuote(value)
		out, err := exec.Command("/bin/sh", "-c", "printf '%s' "+quoted).Output()
		if err != nil || string(out) != value {
			t.Fatalf("startup path was interpreted: %q %v", out, err)
		}
	}
}

func TestZshCancelledPasteCannotReachStartupInput(t *testing.T) {
	shell, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte("PS1='fixture> '\nprint waiting\nIFS= read -r value\nprint -r -- received:$value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Start([]string{shell}, home, 80, 20, "HOME="+home, "ZDOTDIR="+home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	var output strings.Builder
	deadline := time.After(3 * time.Second)
	id := ""
	for {
		select {
		case event, ok := <-p.Events:
			if !ok || event.Err != nil {
				t.Fatalf("shell stopped: %v, output=%q", event.Err, output.String())
			}
			output.Write(event.Data)
			if id == "" && strings.Contains(output.String(), "waiting") {
				id, err = p.QueuePaste("printf never")
				if err != nil {
					t.Fatal(err)
				}
				p.CancelPaste(id)
				if err := p.Write([]byte("release\r")); err != nil {
					t.Fatal(err)
				}
			}
		case event := <-p.Pastes:
			if event.ID != id {
				continue
			}
			if event.Status != "cancelled" || strings.Contains(output.String(), "printf never") || !strings.Contains(output.String(), "received:release") {
				t.Fatalf("cancelled paste escaped into startup input: %+v %q", event, output.String())
			}
			return
		case <-deadline:
			t.Fatalf("startup cancellation timed out: %q", output.String())
		}
	}
}
