package editor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestEditorSaveAndQuit(t *testing.T) {
	f, err := Prepare(t.TempDir(), ".sh", "echo original\n")
	if err != nil {
		t.Fatal(err)
	}

	if _, saved, err := f.Result(nil); err != nil || saved {
		t.Fatal("quit without save was treated as save")
	}

	if err = os.Chtimes(f.Path, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}

	if _, saved, err := f.Result(nil); err != nil || !saved {
		t.Fatal("explicit unchanged write was missed")
	}

	if err = os.WriteFile(f.Path, []byte("echo edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if content, saved, err := f.Result(nil); err != nil || !saved || content != "echo edited\n" {
		t.Fatal("saved edit lost")
	}
}

func TestBlocksAndStagingNeverExecute(t *testing.T) {
	blocks := Blocks("text\n```bash\nprintf '%s' 'hello'\n```\n~~~python\nprint(1)\n~~~\n```sh\nunclosed")
	if len(blocks) != 2 || blocks[1].Language != "python" {
		t.Fatalf("bad blocks: %+v", blocks)
	}

	dir := t.TempDir()
	marker := dir + "/must-not-exist"
	_, cmd, err := Stage(dir, "bash", "touch "+Quote(marker))
	if err != nil {
		t.Fatal(err)
	}

	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("staging executed code")
	}

	if cmd != "touch "+Quote(marker) {
		t.Fatalf("invalid stage command: %q", cmd)
	}

	if _, cmd, err = Stage(dir, "sql", "select 1;"); err != nil || cmd != "" {
		t.Fatal("unknown code was treated as shell")
	}
}

func TestEditorArgumentsAreLiteral(t *testing.T) {
	f, err := Prepare(t.TempDir(), ".txt", "hello")
	if err != nil {
		t.Fatal(err)
	}

	c, err := f.Command([]string{"nvim", "-u", "space and $dollar"})
	if err != nil {
		t.Fatal(err)
	}

	if c.Args[2] != "space and $dollar" || c.Args[3] != f.Path {
		t.Fatalf("arguments changed: %q", c.Args)
	}
}

func TestFailedEditorResultsCannotBecomeSavedCommands(t *testing.T) {
	for _, failure := range []string{"exit", "missing", "oversized"} {
		t.Run(failure, func(t *testing.T) {
			f, err := Prepare(t.TempDir(), ".sh", "original")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Remove()
			var exit error
			switch failure {
			case "exit":
				exit = errors.New("editor failed")
				err = os.WriteFile(f.Path, []byte("edited but failed"), 0600)
			case "missing":
				err = os.Remove(f.Path)
			case "oversized":
				err = os.WriteFile(f.Path, []byte(strings.Repeat("x", (4<<20)+1)), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if text, saved, err := f.Result(exit); err == nil || saved || text != "" {
				t.Fatal("failed editor result became executable content")
			}
		})
	}
}

func TestLanguageStagingUsesPrivateFilesAndLiteralPaths(t *testing.T) {
	for _, tc := range []struct{ language, extension, interpreter string }{
		{"python", ".py", "python3"}, {"js", ".js", "node"}, {"rb", ".rb", "ruby"}, {"go", ".go", "go run"},
		{"yaml", ".yaml", ""}, {"json", ".json", ""}, {"unknown", ".txt", ""},
	} {
		t.Run(tc.language, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "space 'quote")
			path, command, err := Stage(dir, tc.language, "inert sample")
			if err != nil {
				t.Fatal(err)
			}
			if tc.interpreter != "" {
				output, err := exec.Command("/bin/sh", "-c", "set -- "+command+"; printf '%s\\n' \"$@\"").Output()
				want := append(strings.Fields(tc.interpreter), path)
				got := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
				if err != nil || !slices.Equal(got, want) {
					t.Fatalf("staged invocation changed literal arguments: %q, %v", got, err)
				}
			} else if command != "" {
				t.Fatal("non-executable source produced a shell command")
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "inert sample" || filepath.Ext(path) != tc.extension {
				t.Fatalf("staging changed file or invocation: %q %q %v", path, command, err)
			}
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatal("staged source is not private")
			}
		})
	}
}

func TestStagingRejectsInternalProtocolAndNullBytes(t *testing.T) {
	for _, tc := range []struct{ language, text string }{{"wraith-run", "{}"}, {"sh", "wraith-run {}"}, {"sh", "echo\x00bad"}} {
		if _, _, err := Stage(t.TempDir(), tc.language, tc.text); err == nil {
			t.Fatalf("unsafe staging accepted: %+v", tc)
		}
	}
	f, err := Prepare(t.TempDir(), ".txt", "draft")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Command(nil); err == nil {
		t.Fatal("empty editor accepted")
	}
	f.Remove()
	if _, err := os.Stat(f.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("temporary edit file remained after removal")
	}
}
