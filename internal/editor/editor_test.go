package editor

import (
	"os"
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
