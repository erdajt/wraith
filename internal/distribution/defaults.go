package distribution

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
)

//go:embed assets/manifest.json assets/seeds/*.bundle
var assets embed.FS

type Package struct {
	Entry Entry
	Path  string
}

// Defaults returns the release's pinned packages after validating host compatibility.
func Defaults(api int) (Manifest, error) {
	data, err := assets.ReadFile("assets/manifest.json")
	if err != nil {
		return Manifest{}, err
	}
	return Parse(data, api)
}

// Prepare installs a release pin through the same verified git path used for imported candidates.
func Prepare(ctx context.Context, name, directory string, api int) (Package, error) {
	manifest, err := Defaults(api)
	if err != nil {
		return Package{}, err
	}
	seeds, err := fs.Sub(assets, "assets")
	if err != nil {
		return Package{}, err
	}
	for _, entry := range manifest.Packages {
		if entry.Name != name {
			continue
		}
		path, err := Install(ctx, seeds, entry, directory, api)
		if err != nil {
			return Package{}, fmt.Errorf("prepare %s: %w", name, err)
		}
		return Package{Entry: entry, Path: path}, nil
	}
	return Package{}, fmt.Errorf("package %q is absent from this distribution", name)
}
