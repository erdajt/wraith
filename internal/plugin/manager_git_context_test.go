package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManagerGitDoesNotExecuteCurrentRepositoryConfiguration isolates discovery before contacting a configured remote.
func TestManagerGitDoesNotExecuteCurrentRepositoryConfiguration(t *testing.T) {
	repository, _ := managerGitFixture(t)
	directory := t.TempDir()
	marker := filepath.Join(directory, "project-config-ran")
	ordinary := filepath.Join(directory, "ordinary-ssh-ran")
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	scripts := map[string]string{
		"project-ssh": "#!/bin/sh\nprintf inherited > " + quote(marker) + "\nexit 1\n",
		"ssh":         "#!/bin/sh\nprintf isolated > " + quote(ordinary) + "\nexit 1\n",
	}
	for name, script := range scripts {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	managerGit(t, repository, "config", "core.sshCommand", quote(filepath.Join(directory, "project-ssh")))
	t.Chdir(repository)
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	r := openManagerRuntime(t, `require("wraith.manager.store_git_command").run(
 {"ls-remote","--exit-code","--","ssh://example.invalid/repository","HEAD"},nil,{},function(result)
  assert(result.state=="failed")
  wraith.notify("complete")
 end)`)
	runManagerResources(t, r)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("current repository config executed: %v", err)
	}
	if _, err := os.Stat(ordinary); err != nil {
		t.Fatalf("isolated ssh fixture was not reached: %v", err)
	}
}

// TestManagerGitInitializesExplicitCheckoutOutsideCurrentRepository preserves normal staging after discovery isolation.
func TestManagerGitInitializesExplicitCheckoutOutsideCurrentRepository(t *testing.T) {
	source, commit := managerGitFixture(t)
	repository, _ := managerGitFixture(t)
	managerGit(t, repository, "config", "core.bare", "true")
	managerGit(t, repository, "config", "init.defaultBranch", "unrelated")
	t.Chdir(repository)
	target := filepath.Join(t.TempDir(), "checkout")
	r := openManagerRuntime(t, fmt.Sprintf(`require("wraith.manager.store_git").prepare(
 {source={kind="git",url=%q},selector={kind="head"}},%q,{},function(result)
  assert(result.state=="completed" and result.commit==%q,result.error)
  wraith.notify("complete")
 end)`, source, target, commit))
	runManagerResources(t, r)
	if got := managerGit(t, target, "rev-parse", "--is-bare-repository"); got != "false" {
		t.Fatalf("staging inherited bare repository: %s", got)
	}
	if got := managerGit(t, target, "rev-parse", "HEAD"); got != commit {
		t.Fatalf("wrong checkout: %s", got)
	}
}
