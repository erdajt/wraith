package distribution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSeedGitIgnoresInheritedConfiguration(t *testing.T) {
	directory := t.TempDir()
	config := filepath.Join(directory, "hostile.config")
	if err := os.WriteFile(config, []byte("[alias]\n injected = !touch marker\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "alias.injected")
	t.Setenv("GIT_CONFIG_VALUE_0", "!touch marker")
	t.Setenv("GIT_DIR", filepath.Join(directory, "other"))
	if _, err := runGit(t.Context(), directory, "init", "--quiet", "--template=", "."); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(t.Context(), directory, "injected"); err == nil {
		t.Fatal("inherited git alias executed")
	}
	if _, err := os.Stat(filepath.Join(directory, "marker")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("injected alias left marker: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, ".git")); err != nil {
		t.Fatalf("inherited GIT_DIR redirected initialization: %v", err)
	}
}

func TestSeedGitDoesNotDiscoverParentRepository(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	if _, err := runGit(t.Context(), parent, "init", "--quiet", "--template=", "."); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(t.Context(), parent, "config", "--local", "seedreview.marker", "parent"); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(parent, "staging")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	output, err := runGit(t.Context(), child, "config", "--local", "--get", "seedreview.marker")
	if err == nil || output != "" {
		t.Fatalf("discovered parent configuration: %q, %v", output, err)
	}
}

func TestSeedGitReapsProcessGroupsOnCancellationAndPipeTimeout(t *testing.T) {
	for _, running := range []bool{false, true} {
		t.Run(strconv.FormatBool(running), func(t *testing.T) {
			directory := t.TempDir()
			script := "#!/bin/sh\n/bin/sleep 30 &\necho $! > child.pid\n"
			if running {
				script += "wait\n"
			}
			if err := os.WriteFile(filepath.Join(directory, "git"), []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", directory)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			finished := make(chan error, 1)
			go func() {
				_, err := runGit(ctx, directory, "version")
				finished <- err
			}()
			var pid int
			for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
				data, _ := os.ReadFile(filepath.Join(directory, "child.pid"))
				pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
				if pid > 0 {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			if pid <= 0 {
				t.Fatal("child did not start")
			}
			defer syscall.Kill(pid, syscall.SIGKILL)
			if running {
				cancel()
			}
			select {
			case err := <-finished:
				if err == nil || (running && !errors.Is(err, context.Canceled)) {
					t.Fatalf("unexpected completion: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("git did not stop")
			}
			for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
				status, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
				if errors.Is(err, os.ErrNotExist) || strings.Contains(string(status), ") Z ") {
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
			t.Fatal("descendant still running after git stopped")
		})
	}
}

func TestSeedGitCancellationAndBoundedOutput(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := runGit(ctx, t.TempDir(), "version"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled git: %v", err)
	}
	var output gitOutput
	input := []byte(strings.Repeat("x", 128<<10))
	for range 2 {
		n, err := output.Write(input)
		if n != len(input) || err != nil || output.Len() != 64<<10 || !output.truncated {
			t.Fatalf("bounded output: written=%d retained=%d truncated=%v err=%v", n, output.Len(), output.truncated, err)
		}
	}
}
