package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// moduleRoot validates an explicit directory without consulting runtime search paths.
func moduleRoot(ctx context.Context, root string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !filepath.IsAbs(root) || len(root) > 4096 || strings.ContainsAny(root, ";?\x00") {
		return "", errors.New("module root must be an absolute path within 4096 bytes without semicolon, question mark or nul")
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("module root is not a directory: %s", root)
	}
	return canonical, ctx.Err()
}

// moduleFile distinguishes absent sources from invalid types and child symlinks.
func moduleFile(root, path string) (string, error) {
	canonical, err := modulePath(root, path)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(canonical)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("module source is not a regular file: %s", canonical)
	}
	return canonical, nil
}

// catalogModules lists executable top-level lua sources from one explicit root.
func catalogModules(ctx context.Context, root string) ([]moduleSource, error) {
	root, err := moduleRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	sources, _, err := moduleDirectory(ctx, root, "lua", "")
	if err != nil {
		return nil, err
	}
	if len(sources) > 256 {
		return nil, errors.New("module catalog exceeds 256 sources")
	}
	slices.SortFunc(sources, func(a, b moduleSource) int { return strings.Compare(a.name, b.name) })
	for index, source := range sources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(source.name) > 128 || !namespaceName.MatchString(source.name) {
			return nil, fmt.Errorf("invalid discovered module %q", source.name)
		}
		if index > 0 && sources[index-1].name == source.name {
			return nil, fmt.Errorf("ambiguous module %s", source.name)
		}
		path, err := moduleFile(root, source.path)
		if err != nil {
			return nil, err
		}
		sources[index].path = path
	}
	return sources, ctx.Err()
}

// resolveModuleSource selects an exact lua or legacy source without global fallbacks.
func resolveModuleSource(ctx context.Context, root, name string) (*moduleSource, error) {
	if len(name) > 128 || !namespaceName.MatchString(name) {
		return nil, errors.New("invalid module name")
	}
	root, err := moduleRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	relative := filepath.FromSlash(strings.ReplaceAll(name, ".", "/"))
	for _, prefix := range []string{"lua", ""} {
		var selected *moduleSource
		base := filepath.Join(root, prefix, relative)
		for _, path := range []string{base + ".lua", filepath.Join(base, "init.lua")} {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			canonical, err := moduleFile(root, path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if selected != nil {
				return nil, fmt.Errorf("ambiguous module %s", name)
			}
			selected = &moduleSource{name: name, path: canonical, root: root}
		}
		if selected != nil {
			return selected, ctx.Err()
		}
	}
	return nil, ctx.Err()
}
