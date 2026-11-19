package distribution

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"
)

const MaxSeedSize = 32 << 20

var (
	packageName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	moduleName  = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)
	repository  = regexp.MustCompile(`^https://github\.com/[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9][A-Za-z0-9._-]*\.git$`)
	hexDigest   = regexp.MustCompile(`^[0-9a-f]+$`)
)

type Manifest struct {
	Version  int     `json:"version"`
	Packages []Entry `json:"packages"`
}

type Entry struct {
	Name         string `json:"name"`
	Source       string `json:"source"`
	Commit       string `json:"commit"`
	ObjectFormat string `json:"object_format"`
	Artifact     string `json:"artifact"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
	Module       string `json:"module"`
	APIMin       int    `json:"api_min"`
	APIMax       int    `json:"api_max"`
	License      string `json:"license"`
}

// Parse validates the complete distribution before any package source is opened.
func Parse(data []byte, api int) (Manifest, error) {
	var manifest Manifest
	if len(data) > 64<<10 {
		return manifest, errors.New("distribution manifest exceeds 64 kib")
	}
	if err := uniqueFields(json.NewDecoder(bytes.NewReader(data)), 0); err != nil {
		return manifest, fmt.Errorf("distribution manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("distribution manifest: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Manifest{}, errors.New("distribution manifest has trailing data")
	}
	if manifest.Version != 1 || len(manifest.Packages) == 0 || len(manifest.Packages) > 32 {
		return Manifest{}, errors.New("distribution requires version 1 and between 1 and 32 packages")
	}

	names, sources, artifacts, modules := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, entry := range manifest.Packages {
		if err := entry.Validate(api); err != nil {
			return Manifest{}, fmt.Errorf("distribution package %q: %w", entry.Name, err)
		}
		source := strings.ToLower(entry.Source)
		if names[entry.Name] || sources[source] || artifacts[entry.Artifact] || modules[entry.Module] {
			return Manifest{}, errors.New("distribution has duplicate package identities")
		}
		names[entry.Name], sources[source], artifacts[entry.Artifact], modules[entry.Module] = true, true, true, true
	}
	return manifest, nil
}

// uniqueFields rejects ambiguous json keys before decoding into case-insensitive go fields.
func uniqueFields(decoder *json.Decoder, depth int) error {
	if depth > 8 {
		return errors.New("manifest nesting exceeds 8 levels")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, container := token.(json.Delim)
	if !container {
		return nil
	}
	fields := map[string]bool{}
	for decoder.More() {
		if delimiter == '{' {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || fields[name] || name != strings.ToLower(name) {
				return errors.New("manifest keys must be unique and lowercase")
			}
			fields[name] = true
		}
		if err := uniqueFields(decoder, depth+1); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}

// Validate checks identity, artifact bounds and host compatibility without accessing disk.
func (e Entry) Validate(api int) error {
	if !packageName.MatchString(e.Name) || !moduleName.MatchString(e.Module) || len(e.Module) > 128 {
		return errors.New("invalid package name or entry module")
	}
	if !repository.MatchString(e.Source) || len(e.Source) > 512 {
		return errors.New("source must be a canonical https github repository ending in .git")
	}
	length := map[string]int{"sha1": 40, "sha256": 64}[e.ObjectFormat]
	if length == 0 || len(e.Commit) != length || !hexDigest.MatchString(e.Commit) {
		return errors.New("commit must be a full lowercase object id for the declared format")
	}
	if len(e.SHA256) != 64 || !hexDigest.MatchString(e.SHA256) || e.Size <= 0 || e.Size > MaxSeedSize {
		return errors.New("invalid seed checksum or size")
	}
	for _, path := range []string{e.Artifact, e.License} {
		if !fs.ValidPath(path) || path == "." || strings.ContainsAny(path, "\\\x00\r\n") || len(path) > 256 {
			return errors.New("artifact and license must be relative file paths")
		}
	}
	if e.APIMin < 1 || e.APIMax < e.APIMin || api < e.APIMin || api > e.APIMax {
		return fmt.Errorf("package requires host api %d..%d; running %d", e.APIMin, e.APIMax, api)
	}
	return nil
}
