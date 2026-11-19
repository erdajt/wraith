package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/distribution"
)

// managerOptions installs the pinned source through the production distribution path.
func managerOptions(t *testing.T) Options {
	t.Helper()
	paths := config.Paths{Data: t.TempDir(), Cache: t.TempDir()}
	selected, err := distribution.Prepare(t.Context(), "shroud", filepath.Join(paths.Data, "bootstrap"), 2)
	if err != nil {
		t.Fatal(err)
	}
	return Options{Paths: paths, Packages: []PackageRoot{{Directory: selected.Path, Namespaces: []string{"shroud", "wraith.manager"}}}}
}

// managerModule locates source-level fault injection fixtures in the installed package.
func managerModule(t *testing.T, name string) string {
	t.Helper()
	options := managerOptions(t)
	return filepath.Join(options.Packages[0].Directory, "lua", "wraith", "manager", name)
}

// openManagerRuntime loads user configuration against an installed package in isolated storage.
func openManagerRuntime(t *testing.T, source string, prepared ...Options) *Runtime {
	t.Helper()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "init.lua"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	var options Options
	if len(prepared) == 0 {
		options = managerOptions(t)
	} else if len(prepared) == 1 {
		options = prepared[0]
	} else {
		t.Fatal("manager fixture accepts one prepared package set")
	}
	runtime, err := OpenWithOptions(t.Context(), directory, config.Default(), nil, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	return runtime
}
