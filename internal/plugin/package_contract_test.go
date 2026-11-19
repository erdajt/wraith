package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/distribution"
)

// TestShroudPackageContracts runs package-owned assertions from the installed distribution pin.
func TestShroudPackageContracts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	manifest, err := distribution.Defaults(2)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range manifest.Packages {
		if entry.Name == "shroud" {
			t.Logf("candidate %s %s %s", entry.Name, entry.Source, entry.Commit)
		}
	}
	options := managerOptions(t)
	path := filepath.Join(options.Packages[0].Directory, "tests")
	files, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 || len(files) > 64 {
		t.Fatal("package contracts require between 1 and 64 entries")
	}
	count := 0
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".lua") {
			continue
		}
		if !file.Type().IsRegular() {
			t.Fatalf("contract must be a regular file: %s", file.Name())
		}
		count++
		t.Run(file.Name(), func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			options := managerOptions(t)
			script := filepath.Join(options.Packages[0].Directory, "tests", file.Name())
			openManagerRuntime(t, fmt.Sprintf("dofile(%q)", script), options)
		})
	}
	if count == 0 {
		t.Fatal("selected package contains no lua contracts")
	}
}
