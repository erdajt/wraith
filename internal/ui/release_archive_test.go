package ui

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/distribution"
)

// TestReleaseArchiveContainsExecutableExamples exercises the workflow's archive builder and extracted packages.
func TestReleaseArchiveContainsExecutableExamples(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "built binary")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "release files")
			build := exec.CommandContext(t.Context(), filepath.Join(root, "tools", "package-release.sh"), "v0.0.0-test", arch, binary, output)
			if data, err := build.CombinedOutput(); err != nil {
				t.Fatalf("package archive: %s, %v", data, err)
			}
			name := "wraith_v0.0.0-test_linux_" + arch
			extracted := t.TempDir()
			extract := exec.CommandContext(t.Context(), "tar", "-xzf", filepath.Join(output, name+".tar.gz"), "-C", extracted)
			if data, err := extract.CombinedOutput(); err != nil {
				t.Fatalf("extract archive: %s, %v", data, err)
			}
			archive := filepath.Join(extracted, name)
			compareReleaseFile(t, binary, filepath.Join(archive, "wraith"))
			info, err := os.Stat(filepath.Join(archive, "wraith"))
			if err != nil || info.Mode().Perm()&0111 == 0 {
				t.Fatalf("binary lost executable mode: %v, %v", info, err)
			}

			for _, path := range []string{"README.md", "LICENSE", "NOTICE", "third_party/README.md", "third_party/gopher-lua/LICENSE", "docs", "examples/plugins", "internal/distribution/assets/manifest.json", "internal/distribution/assets/seeds"} {
				err := filepath.WalkDir(filepath.Join(root, path), func(source string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					relative, err := filepath.Rel(root, source)
					if err != nil {
						return err
					}
					compareReleaseFile(t, source, filepath.Join(archive, relative))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			compareReleaseFile(t, filepath.Join(root, "third_party/gopher-lua/LICENSE"), filepath.Join(archive, "licenses/gopher-lua.txt"))
			for _, guide := range []string{"docs/lua/examples.md", "docs/distribution.md", "third_party/README.md"} {
				path := filepath.Join(archive, guide)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				for _, match := range regexp.MustCompile(`\]\(([^)]+)\)`).FindAllStringSubmatch(string(data), -1) {
					link, _, _ := strings.Cut(match[1], "#")
					if link == "" || strings.Contains(link, "://") {
						continue
					}
					if _, err := os.Stat(filepath.Join(filepath.Dir(path), link)); err != nil {
						t.Errorf("archive link %s -> %s: %v", guide, link, err)
					}
				}
			}

			assets := filepath.Join(archive, "internal/distribution/assets")
			data, err := os.ReadFile(filepath.Join(assets, "manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			manifest, err := distribution.Parse(data, 2)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range manifest.Packages {
				if _, err := distribution.Install(t.Context(), os.DirFS(assets), entry, t.TempDir(), 2); err != nil {
					t.Fatalf("install archived package %s: %v", entry.Name, err)
				}
			}

			names := []string{
				"TestTaskPackageTransformsRequestsAndUnloadsOwnedUI",
				"TestHistoryPackagePagesNativeRecordsAndReadsHiddenLocalOutput",
				"TestFilesPackageReadsRealGitStateAndReleasesSurfaces",
				"TestScopePackageChecksApprovedEditsAndUnloadsPolicy",
			}
			check := exec.CommandContext(t.Context(), testBinary, "-test.run=^("+strings.Join(names, "|")+")$", "-test.count=1", "-test.timeout=30s", "-test.v")
			check.Env = append(os.Environ(), "WRAITH_TEST_EXAMPLES="+filepath.Join(archive, "examples", "plugins"))
			data, err = check.CombinedOutput()
			if err != nil {
				t.Fatalf("extracted packages: %s, %v", data, err)
			}
			for _, name := range names {
				if !strings.Contains(string(data), "--- PASS: "+name+" ") {
					t.Errorf("extracted package test did not pass: %s\n%s", name, data)
				}
			}
		})
	}
}

// compareReleaseFile checks that packaging preserves source bytes.
func compareReleaseFile(t *testing.T, source, target string) {
	t.Helper()
	want, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("archive changed %s", target)
	}
}

// TestReleaseArchiveRejectsInvalidArguments rejects unsupported inputs before staging an archive.
func TestReleaseArchiveRejectsInvalidArguments(t *testing.T) {
	output := filepath.Join(t.TempDir(), "absent")
	binary := filepath.Join(t.TempDir(), "binary")
	if err := os.WriteFile(binary, nil, 0755); err != nil {
		t.Fatal(err)
	}
	nonExecutable := filepath.Join(filepath.Dir(binary), "non-executable")
	if err := os.WriteFile(nonExecutable, nil, 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{},
		{"../escape", "amd64", binary, output},
		{"v1.0.0", "../escape", binary, output},
		{"v1.0.0", "amd64", binary + "-missing", output},
		{"v1.0.0", "amd64", nonExecutable, output},
		{"v1.0.0", "amd64", filepath.Dir(binary), output},
		{"v1.0.0", "amd64", binary, ""},
	} {
		command := exec.CommandContext(t.Context(), "../../tools/package-release.sh", args...)
		if data, err := command.CombinedOutput(); err == nil {
			t.Fatalf("invalid arguments accepted: %q: %s", args, data)
		}
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("invalid arguments created output: %v", err)
	}
}
