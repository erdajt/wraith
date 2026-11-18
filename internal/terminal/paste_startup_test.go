package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestEmptyZdotdirDoesNotSourceWorkspace(t *testing.T) {
	shell, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "unexpected-startup")
	if err := os.WriteFile(filepath.Join(dir, ".zshenv"), []byte("print workspace-loaded > "+shellQuote(marker)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	plain := exec.Command(shell, "-i", "-c", "exit 0")
	plain.Dir = dir
	plain.Env = append(os.Environ(), "HOME="+t.TempDir(), "ZDOTDIR=")
	if out, err := plain.CombinedOutput(); err != nil {
		t.Fatalf("ordinary shell: %v %q", err, out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("control shell loaded workspace; fixture invalid")
	}
	p, err := Start([]string{shell}, dir, 80, 20, "HOME="+t.TempDir(), "ZDOTDIR=")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	timeout := time.After(3 * time.Second)
	for {
		select {
		case e := <-p.Pastes:
			if e.ID == "ready" {
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatal("bridge sourced workspace .zshenv with ZDOTDIR empty")
				}
				return
			}
		case <-p.Events:
		case <-timeout:
			t.Fatal("shell failed to become ready")
		}
	}
}
