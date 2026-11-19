package distribution

import (
	"encoding/json"
	"strings"
	"testing"
)

// fixtureEntry describes a pinned package without depending on a sibling repository.
func fixtureEntry() Entry {
	return Entry{Name: "shroud", Source: "https://github.com/erdajt/shroud.wraith.git", Commit: strings.Repeat("a", 40), ObjectFormat: "sha1", Artifact: "seeds/shroud.bundle", SHA256: strings.Repeat("b", 64), Size: 100, Module: "shroud", APIMin: 2, APIMax: 2, License: "LICENSE"}
}

func TestManifestRejectsInvalidIdentityBeforeReadingSources(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		edit func(*Entry)
	}{
		{"option name", func(e *Entry) { e.Name = "--config" }},
		{"credentials", func(e *Entry) { e.Source = "https://token@github.com/erdajt/shroud.wraith.git" }},
		{"query", func(e *Entry) { e.Source += "?token=secret" }},
		{"shell transport", func(e *Entry) { e.Source = "ext::sh command" }},
		{"short commit", func(e *Entry) { e.Commit = "abc123" }},
		{"format mismatch", func(e *Entry) { e.ObjectFormat = "sha256" }},
		{"unknown format", func(e *Entry) { e.ObjectFormat = "unknown" }},
		{"uppercase commit", func(e *Entry) { e.Commit = strings.Repeat("A", 40) }},
		{"traversal", func(e *Entry) { e.Artifact = "../seed.bundle" }},
		{"absolute", func(e *Entry) { e.Artifact = "/tmp/seed.bundle" }},
		{"license escape", func(e *Entry) { e.License = "../LICENSE" }},
		{"bad digest", func(e *Entry) { e.SHA256 = strings.Repeat("g", 64) }},
		{"empty seed", func(e *Entry) { e.Size = 0 }},
		{"oversize seed", func(e *Entry) { e.Size = MaxSeedSize + 1 }},
		{"future api", func(e *Entry) { e.APIMin, e.APIMax = 3, 4 }},
		{"old api", func(e *Entry) { e.APIMin, e.APIMax = 1, 1 }},
		{"module escape", func(e *Entry) { e.Module = "../shroud" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := fixtureEntry()
			tc.edit(&entry)
			data, err := json.Marshal(Manifest{Version: 1, Packages: []Entry{entry}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Parse(data, 2); err == nil {
				t.Fatal("accepted invalid distribution")
			}
		})
	}
}

func TestManifestVersionBoundsAndDuplicateSources(t *testing.T) {
	t.Parallel()
	valid, err := json.Marshal(Manifest{Version: 1, Packages: []Entry{fixtureEntry()}})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(valid, 2)
	if err != nil || parsed.Packages[0] != fixtureEntry() {
		t.Fatalf("valid manifest: %+v, %v", parsed, err)
	}
	for _, input := range []string{
		string(valid) + "{}", strings.Replace(string(valid), `"version":1`, `"version":2`, 1),
		strings.Replace(string(valid), `"version":1`, `"extra":true,"version":1`, 1),
		strings.Replace(string(valid), `"name":"shroud"`, `"name":"other","name":"shroud"`, 1),
		strings.Replace(string(valid), `"name":"shroud"`, `"Name":"shroud"`, 1),
		`{"version":1,"packages":[]}`, strings.Repeat(" ", 64<<10+1),
	} {
		if _, err := Parse([]byte(input), 2); err == nil {
			t.Fatal("accepted malformed manifest")
		}
	}
	other := fixtureEntry()
	other.Name, other.Artifact, other.Module = "other", "seeds/other.bundle", "other"
	duplicate, _ := json.Marshal(Manifest{Version: 1, Packages: []Entry{fixtureEntry(), other}})
	if _, err := Parse(duplicate, 2); err == nil {
		t.Fatal("accepted duplicate source under another name")
	}
	other.Source = "https://github.com/Erdajt/Shroud.Wraith.git"
	duplicate, _ = json.Marshal(Manifest{Version: 1, Packages: []Entry{fixtureEntry(), other}})
	if _, err := Parse(duplicate, 2); err == nil {
		t.Fatal("accepted case alias of github source")
	}
}
