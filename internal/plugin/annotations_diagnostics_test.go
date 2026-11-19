package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLuaAnnotationDiagnostics checks declarations, examples and rejected calls with the language server.
func TestLuaAnnotationDiagnostics(t *testing.T) {
	server := os.Getenv("WRAITH_LUALS")
	if server == "" {
		var err error
		server, err = exec.LookPath("lua-language-server")
		if err != nil {
			t.Skip("set WRAITH_LUALS or install lua-language-server to validate editor diagnostics")
		}
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "docs", "lua", "types"))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	workspace := filepath.Join(directory, "workspace")
	if err := os.CopyFS(workspace, os.DirFS(filepath.Join("testdata", "luals"))); err != nil {
		t.Fatal(err)
	}
	for _, example := range []struct{ destination, source string }{
		{"examples", filepath.Join("..", "..", "examples", "plugins")},
		{"documentation", filepath.Join("..", "..", "docs", "examples", "lua")},
	} {
		if err := os.CopyFS(filepath.Join(workspace, example.destination), os.DirFS(example.source)); err != nil {
			t.Fatal(err)
		}
	}
	library := filepath.Join(workspace, "declarations")
	if err := os.CopyFS(library, os.DirFS(root)); err != nil {
		t.Fatal(err)
	}
	settings, err := json.Marshal(map[string]any{
		"runtime.version": "Lua 5.1", "workspace.library": []string{library},
		"workspace.checkThirdParty": false, "diagnostics.libraryFiles": "Enable",
	})
	if err != nil {
		t.Fatal(err)
	}
	configuration := filepath.Join(directory, "config.json")
	if err := os.WriteFile(configuration, settings, 0600); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(directory, "diagnostics.json")
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, server,
		"--check="+workspace, "--check_format=json", "--check_out_path="+report,
		"--configpath="+configuration, "--logpath="+filepath.Join(directory, "log"),
		"--metapath="+filepath.Join(directory, "meta"))
	command.WaitDelay = 5 * time.Second
	output, err := command.CombinedOutput()
	var exit *exec.ExitError
	if ctx.Err() != nil || err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
		t.Fatalf("language server: %v, context: %v\n%s", err, ctx.Err(), output)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("read diagnostics: %v\n%s", err, output)
	}
	var diagnostics map[string][]struct {
		Code, Message string
		Range         struct{ Start struct{ Line int } }
	}
	if err := json.Unmarshal(data, &diagnostics); err != nil {
		t.Fatalf("decode diagnostics: %v\n%s", err, data)
	}

	negative, err := os.ReadFile(filepath.Join(workspace, "invalid.lua"))
	if err != nil {
		t.Fatal(err)
	}
	expected := map[int]string{}
	for line, text := range strings.Split(string(negative), "\n") {
		if _, code, ok := strings.Cut(text, "-- expect: "); ok {
			expected[line] = strings.TrimSpace(code)
		}
	}
	if len(expected) == 0 {
		t.Fatal("negative fixture has no expected diagnostics")
	}
	for source, entries := range diagnostics {
		uri, err := url.Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			line := entry.Range.Start.Line
			if filepath.Base(uri.Path) != "invalid.lua" || expected[line] != entry.Code {
				t.Errorf("unexpected %s:%d %s: %s", source, line+1, entry.Code, entry.Message)
				continue
			}
			delete(expected, line)
		}
	}
	for line, code := range expected {
		t.Errorf("invalid.lua:%d did not report %s", line+1, code)
	}
}
