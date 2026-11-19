package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// managerGit runs fixture commands without user git configuration.
func managerGit(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "LC_ALL=C"}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git fixture: %s, %v", output, err)
	}
	return strings.TrimSpace(string(output))
}

// managerGitFixture creates an offline repository whose path exercises literal process arguments.
func managerGitFixture(t *testing.T) (string, string) {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "source 'literal' $value")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	managerGit(t, directory, "init", "--quiet", "--template=", "--initial-branch=main")
	managerGit(t, directory, "config", "user.name", "fixture")
	managerGit(t, directory, "config", "user.email", "fixture@example.invalid")

	if err := os.WriteFile(filepath.Join(directory, "plugin.lua"), []byte("return { answer = 42 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	managerGit(t, directory, "add", "plugin.lua")
	managerGit(t, directory, "commit", "--quiet", "-m", "fixture")
	managerGit(t, directory, "tag", "-a", "v1", "-m", "version one")
	return directory, managerGit(t, directory, "rev-parse", "HEAD")
}

// TestManagerGitStagesExactOfflineRevisions fetches real branches, annotated tags and locked commits.
func TestManagerGitStagesExactOfflineRevisions(t *testing.T) {
	source, commit := managerGitFixture(t)
	for _, selector := range []string{`selector={kind="head"}`, `selector={kind="branch",value="main"}`, `selector={kind="tag",value="v1"}`, `selector={kind="head"},commit="` + commit + `"`} {
		t.Run(selector, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "checkout")
			r := openTestRuntime(t, fmt.Sprintf(`local git=require("wraith.manager.store_git")
git.prepare({source={kind="git",url=%q},%s},%q,{},function(result)
 assert(result.state=="completed",result.error)
 assert(result.commit==%q and result.path==%q)
 wraith.notify("complete")
end)`, source, selector, target, commit, target))
			runManagerResources(t, r)
			data, err := os.ReadFile(filepath.Join(target, "plugin.lua"))
			if err != nil || string(data) != "return { answer = 42 }\n" {
				t.Fatalf("checkout content: %q, %v", data, err)
			}
		})
	}
}

// TestManagerGitRejectsMissingRevisions keeps failed preparation outside the active object tree.
func TestManagerGitRejectsMissingRevisions(t *testing.T) {
	source, _ := managerGitFixture(t)
	for _, selector := range []string{`selector={kind="branch",value="absent"}`, `selector={kind="head"},commit="` + strings.Repeat("1", 40) + `"`} {
		t.Run(selector, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "checkout")
			r := openTestRuntime(t, fmt.Sprintf(`local git=require("wraith.manager.store_git")
git.prepare({source={kind="git",url=%q},%s},%q,{},function(result)
 assert(result.state=="failed" and (result.error or result.stderr))
 wraith.notify("complete")
end)`, source, selector, target))
			runManagerResources(t, r)
			if _, err := os.Stat(filepath.Join(target, "plugin.lua")); !os.IsNotExist(err) {
				t.Fatalf("failed checkout populated files: %v", err)
			}
		})
	}
}

// TestManagerGitSupportsSHA256 preserves the repository object format while resolving tags.
func TestManagerGitSupportsSHA256(t *testing.T) {
	source := t.TempDir()
	managerGit(t, source, "init", "--quiet", "--template=", "--object-format=sha256", "--initial-branch=main")
	managerGit(t, source, "config", "user.name", "fixture")
	managerGit(t, source, "config", "user.email", "fixture@example.invalid")
	managerGit(t, source, "commit", "--quiet", "--allow-empty", "-m", "fixture")
	commit := managerGit(t, source, "rev-parse", "HEAD")
	target := filepath.Join(t.TempDir(), "checkout")
	r := openTestRuntime(t, fmt.Sprintf(`local git=require("wraith.manager.store_git")
git.prepare({source={kind="git",url=%q},selector={kind="head"}},%q,{},function(result)
 assert(result.state=="completed",result.error)
 assert(result.commit==%q and #result.commit==64)
 wraith.notify("complete")
end)`, source, target, commit))
	runManagerResources(t, r)
}

// TestManagerGitCancellationDoesNotInitializeCheckout cancels before the first child starts.
func TestManagerGitCancellationDoesNotInitializeCheckout(t *testing.T) {
	source, _ := managerGitFixture(t)
	target := filepath.Join(t.TempDir(), "checkout")
	r := openTestRuntime(t, fmt.Sprintf(`local git=require("wraith.manager.store_git")
local operation=git.prepare({source={kind="git",url=%q},selector={kind="head"}},%q,{},function(result)
 assert(result.state=="cancelled",result.error)
 wraith.notify("complete")
end)
operation.cancel()`, source, target))
	runManagerResources(t, r)
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("cancelled checkout created: %v", err)
	}
}

// TestManagerGitRejectsTruncatedProtocolOutput refuses a successful child with incomplete ref data.
func TestManagerGitRejectsTruncatedProtocolOutput(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nprintf '%070000d' 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	target := filepath.Join(t.TempDir(), "checkout")
	r := openTestRuntime(t, fmt.Sprintf(`local git=require("wraith.manager.store_git")
git.prepare({source={kind="git",url="/unused"},selector={kind="head"}},%q,{},function(result)
 assert(result.state=="failed" and result.error=="git output exceeded its capture limit")
 wraith.notify("complete")
end)`, target))
	runManagerResources(t, r)
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("truncated ref initialized checkout: %v", err)
	}
}

// TestManagerGitReportsCompletionCallbackFailure keeps consumer errors visible to the runtime.
func TestManagerGitReportsCompletionCallbackFailure(t *testing.T) {
	source, _ := managerGitFixture(t)
	target := filepath.Join(t.TempDir(), "checkout")
	r := openTestRuntime(t, fmt.Sprintf(`require("wraith.manager.store_git").prepare(
 {source={kind="git",url=%q},selector={kind="head"}},%q,{},function() error("completion failed") end)`, source, target))
	manifest := r.Manifest()
	if err := r.Activate(manifest.Resources); err != nil {
		t.Fatal(err)
	}
	for {
		event := receiveResource(t, r)
		if !r.ResourceValid(event) {
			continue
		}
		pending, err := r.SubmitResource(event, nil, manifest.Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if result.Err != nil {
			if !strings.Contains(result.Err.Error(), "completion failed") {
				t.Fatal(result.Err)
			}
			return
		}
		for _, effect := range result.Effects {
			switch effect.Name {
			case "resource.activate":
				if err := r.Activate([]string{effect.Argument}); err != nil {
					t.Fatal(err)
				}
			case "resource.release":
				r.DiscardResources([]string{effect.Argument})
			default:
				t.Fatalf("unexpected effect: %+v", effect)
			}
		}
	}
}
