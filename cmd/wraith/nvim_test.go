package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/terminal"
)

func TestNeovimTerminalEditorAndCopyMode(t *testing.T) {
	nvim, err := exec.LookPath("nvim")
	if err != nil {
		t.Skip("neovim is not installed")
	}
	root := t.TempDir()
	t.Setenv("WRAITH_TEST_WORKSPACE", root)
	t.Setenv("TERM", "xterm-256color")
	c := config.Default()
	c.AutoStart = false
	c.Host = "http://127.0.0.1:1"
	c.Clipboard = nil
	c.Editor = []string{nvim, "--clean", "-n"}
	c.Shell = []string{"/bin/sh", "-i"}
	if err := config.Save(root+"/config/config.yaml", c); err != nil {
		t.Fatal(err)
	}
	command := "terminal '" + strings.ReplaceAll(os.Args[0], "'", "'\\''") + "' -test.run=^TestTUIHelperProcess$"
	p, err := terminal.Start([]string{nvim, "--clean", "-n", "-c", command, "-c", "startinsert"}, "", 110, 36)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	screen := terminal.NewScreen(110, 36, 100)
	wait := func(want string) {
		t.Helper()
		timeout := time.After(8 * time.Second)
		for {
			select {
			case e, ok := <-p.Events:
				if !ok || e.Err != nil {
					t.Fatalf("terminal exited before %q: %v", want, e.Err)
				}
				if reply := screen.Feed(e.Data); len(reply) > 0 {
					if err := p.Write(reply); err != nil {
						t.Fatal(err)
					}
				}
				if strings.Contains(ansi.Strip(strings.Join(screen.Lines(false, "", ""), "\n")), want) {
					return
				}
			case <-timeout:
				t.Fatalf("missing %q:\n%s", want, ansi.Strip(strings.Join(screen.Lines(false, "", ""), "\n")))
			}
		}
	}
	send := func(s string) {
		t.Helper()
		if err := p.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	wait("write a prompt")
	send("/settings.appearance\r")
	wait("appearance.background")
	send("\x1b")
	time.Sleep(80 * time.Millisecond)
	send("draft evidence\x05")
	wait("draft evidence")
	time.Sleep(150 * time.Millisecond)
	send("\x1bA edited\x1b:wq\r")
	wait("prompt updated")
	wait("draft evidence edited")
	send("\x1bs")
	wait("side shell")
	send("printf '%s%s\\n' shell- evidence\r")
	wait("shell-evidence")
	send("exit\r")
	time.Sleep(350 * time.Millisecond)
	send("printf '%s%s\\n' restarted- shell\r")
	wait("restarted-shell")
	for range 3 {
		send("\x1b")
		time.Sleep(100 * time.Millisecond)
	}
	wait("share to agent")
	send("/restarted-shell\r")
	time.Sleep(100 * time.Millisecond)
	send("Y")
	wait("copied to wraith register")
	send("i\x1bs")
	time.Sleep(100 * time.Millisecond)
	send("\x1b")
	time.Sleep(100 * time.Millisecond)
	send("p")
	wait("restarted-shell")
	send("\x11")
	wait("Process exited")
}
