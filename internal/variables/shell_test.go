package variables

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellExpansionPreservesArgumentBytesWithoutEvaluatingValues(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ name, source, value, want string }{
		{"bare", "wraith::TARGET", "host.test", "host.test"},
		{"single quoted", "'wraith::TARGET'", "one' two", "one' two"},
		{"double quoted", `"wraith::TARGET"`, "one\" $HOME", "one\" $HOME"},
		{"url", "https://wraith::TARGET/path", "host.test", "https://host.test/path"},
		{"metacharacters", "wraith::TARGET", "$(touch marker); * | cat", "$(touch marker); * | cat"},
		{"assignment", "X=wraith::TARGET; printf '%s' \"$X\"", "one two", "one two"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			snapshot := NewSnapshot([]Entry{{Name: "TARGET", Value: test.value}}, nil)
			input := "printf '%s' " + test.source
			if test.name == "assignment" {
				input = test.source
			}

			command, err := snapshot.Shell(input)
			if err != nil {
				t.Fatal(err)
			}

			cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c", command)
			cmd.Dir = t.TempDir()
			output, err := cmd.CombinedOutput()
			if err != nil || string(output) != test.want {
				t.Fatalf("argument changed: %q %v (command %q)", output, err, command)
			}

			if _, err = os.Stat(filepath.Join(cmd.Dir, "marker")); !os.IsNotExist(err) {
				t.Fatal("private value executed as shell syntax")
			}
		})
	}
}

func TestShellRejectsUnsupportedReferencePositions(t *testing.T) {
	t.Parallel()
	s := NewSnapshot([]Entry{{Name: "TARGET", Value: "value"}}, nil)

	for _, command := range []string{
		"wraith::TARGET --version",
		"printf '%s' wraith::UNKNOWN",
		"cat <<END\nwraith::TARGET\nEND",
		"cat <<wraith::TARGET\ntext\nwraith::TARGET",
		"printf '%s' ${wraith::TARGET}",
		"printf '%s' $((wraith::TARGET))",
		`printf '%s' "$(echo wraith::TARGET)"`,
		"printf '%s' ok # wraith::TARGET",
	} {
		if got, err := s.Shell(command); err == nil {
			t.Errorf("accepted unsupported reference: %s -> %s", command, got)
		}
	}
}

func TestShellRedirectAndUnrelatedSyntaxRemainIntact(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "output file")
	s := NewSnapshot([]Entry{{Name: "OUTPUT", Value: path}}, nil)
	command, err := s.Shell("printf saved >wraith::OUTPUT && printf done")
	if err != nil {
		t.Fatal(err)
	}

	out, err := exec.CommandContext(t.Context(), "/bin/sh", "-c", command).CombinedOutput()
	if err != nil || string(out) != "done" {
		t.Fatalf("redirect failed: %q %v", out, err)
	}

	data, err := os.ReadFile(path)
	if err != nil || string(data) != "saved" || strings.Contains(command, "wraith::") {
		t.Fatal("redirect did not use literal path")
	}
}
