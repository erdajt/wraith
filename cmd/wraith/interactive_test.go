package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/terminal"
)

func TestTUIHelperProcess(t *testing.T) {
	root := os.Getenv("WRAITH_TEST_WORKSPACE")
	if root == "" {
		t.Skip("subprocess entry point")
	}
	err := run([]string{"--config-dir", root + "/config", "--data-dir", root + "/data", "--cache-dir", root + "/cache"}, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func TestInteractiveEditorAndPersistentShells(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WRAITH_TEST_WORKSPACE", root)
	t.Setenv("TERM", "xterm-256color")
	edit := filepath.Join(root, "edit.sh")
	if err := os.WriteFile(edit, []byte("printf ' edited' >> \"$1\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c := config.Default()
	c.AutoStart = false
	c.Host = "http://127.0.0.1:1"
	c.Shell = []string{"/bin/sh", "-i"}
	c.Editor = []string{"/bin/sh", edit}
	if err := config.Save(root+"/config/config.json", c); err != nil {
		t.Fatal(err)
	}
	p, err := terminal.Start([]string{os.Args[0], "-test.run=^TestTUIHelperProcess$"}, "", 100, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	screen := terminal.NewScreen(100, 30, 100)
	waitFor := func(want string) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			select {
			case event, ok := <-p.Events:
				if !ok || event.Err != nil {
					t.Fatalf("tui exited before %q: %v", want, event.Err)
				}
				if reply := screen.Feed(event.Data); len(reply) > 0 {
					if err := p.Write(reply); err != nil {
						t.Fatal(err)
					}
				}
				visible := ansi.Strip(strings.Join(screen.Lines(false, "", ""), "\n"))
				if strings.Contains(visible, want) {
					return
				}
			case <-deadline:
				t.Fatalf("timed out waiting for %q", want)
			}
		}
	}
	send := func(s string) {
		t.Helper()
		if err := p.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	waitFor("W R A I T H")
	send("draft with spaces\x05")
	waitFor("draft with spaces edited")
	send("\x1be")
	waitFor("scratch shell")
	send("WRAITH_TEST_STATE=yes; printf '\\n%s:%s\\n' retained \"$WRAITH_TEST_STATE\"\r")
	waitFor("retained:yes")
	send("\x1be")
	waitFor("draft with spaces edited")
	send("\x1be")
	waitFor("scratch shell")
	send("printf '\\n%s:%s\\n' survived \"$WRAITH_TEST_STATE\"\r")
	waitFor("survived:yes")
	send("\x1be\x1bs")
	waitFor("side shell")
	send("printf '\\n%s:%s\\n' independent \"${WRAITH_TEST_STATE-unset}\"\r")
	waitFor("independent:unset")
	send("\x11")
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-p.Events:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("tui did not exit")
		}
	}
}
