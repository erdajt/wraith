package editor

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/erdajt/wraith/internal/fsutil"
)

type File struct {
	Path     string
	original []byte
	stamp    time.Time
}
type Block struct {
	Language, Text string
	Message        int
}

func Prepare(dir, extension, text string) (*File, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(dir, "edit-*"+extension)
	if err != nil {
		return nil, err
	}
	path := f.Name()
	if _, err = f.WriteString(text); err != nil {
		f.Close()
		os.Remove(path)
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	stamp := time.Unix(946684800, 0)
	if err = os.Chtimes(path, stamp, stamp); err != nil {
		return nil, err
	}
	return &File{Path: path, original: []byte(text), stamp: stamp}, nil
}

func (f *File) Command(argv []string) (*exec.Cmd, error) {
	if len(argv) == 0 {
		return nil, errors.New("editor is empty")
	}
	args := append(append([]string(nil), argv[1:]...), f.Path)
	return exec.Command(argv[0], args...), nil
}

func (f *File) Result(exit error) (string, bool, error) {
	if exit != nil {
		return "", false, exit
	}
	b, err := fsutil.ReadLimit(f.Path, 4<<20)
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(f.Path)
	if err != nil {
		return "", false, err
	}
	return string(b), !bytes.Equal(b, f.original) || !info.ModTime().Equal(f.stamp), nil
}
func (f *File) Remove() { _ = os.Remove(f.Path) }

func Blocks(text string) []Block {
	var out []Block
	var current *Block
	var lines []string
	fence := ""
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimLeft(line, " ")
		indent := len(line) - len(trim)
		if current == nil {
			if indent <= 3 && (strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~")) {
				ch := trim[0]
				n := 0
				for n < len(trim) && trim[n] == ch {
					n++
				}
				fence = trim[:n]
				lang := strings.Fields(strings.TrimSpace(trim[n:]))
				name := "text"
				if len(lang) > 0 {
					name = strings.ToLower(lang[0])
				}
				current = &Block{Language: name}
				lines = nil
			}
		} else if indent <= 3 && strings.HasPrefix(trim, fence) && strings.Trim(trim, string(fence[0])+" \t") == "" {
			current.Text = strings.Join(lines, "\n") + "\n"
			out = append(out, *current)
			current = nil
		} else {
			lines = append(lines, line)
		}
	}
	return out
}

func Extension(lang string) string {
	switch lang {
	case "sh", "bash", "shell", "zsh", "console":
		return ".sh"
	case "python", "py":
		return ".py"
	case "javascript", "js":
		return ".js"
	case "go":
		return ".go"
	case "ruby", "rb":
		return ".rb"
	case "json":
		return ".json"
	case "yaml", "yml":
		return ".yaml"
	case "sql":
		return ".sql"
	default:
		return ".txt"
	}
}

func Stage(dir, lang, text string) (string, string, error) {
	if strings.ContainsRune(text, 0) {
		return "", "", errors.New("code contains a null byte")
	}
	f, err := Prepare(dir, Extension(lang), text)
	if err != nil {
		return "", "", err
	}
	var argv []string
	switch lang {
	case "sh", "bash", "shell", "console":
		argv = []string{"bash", f.Path}
	case "zsh":
		argv = []string{"zsh", f.Path}
	case "python", "py":
		argv = []string{"python3", f.Path}
	case "javascript", "js":
		argv = []string{"node", f.Path}
	case "ruby", "rb":
		argv = []string{"ruby", f.Path}
	case "go":
		argv = []string{"go", "run", f.Path}
	default:
		return f.Path, "", nil
	}
	for i, a := range argv {
		argv[i] = Quote(a)
	}
	return f.Path, strings.Join(argv, " "), nil
}

func Quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
