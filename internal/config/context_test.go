package config

import "testing"

func TestContextResolutionKeepsProviderAndModelOverridesSeparate(t *testing.T) {
	c := Default()
	if c.ContextFor("local", 262144) != 8192 || c.ContextFor("small", 4096) != 4096 {
		t.Fatal("local automatic context ignored memory budget or model capacity")
	}
	c.Provider = "codex"
	if c.ContextFor("large", 500000) != 500000 || c.ContextFor("small", 272000) != 272000 {
		t.Fatal("model context metadata was ignored")
	}
	p := c.Providers["codex"]
	p.Context = 64000
	c.Providers["codex"] = p
	updated := c.WithModelContext("large", 1000000)
	if updated.ContextFor("large", 500000) != 1000000 || updated.ContextFor("small", 272000) != 64000 {
		t.Fatal("model override leaked into provider default")
	}
	if c.ContextFor("large", 500000) != 64000 {
		t.Fatal("copy mutated original provider map")
	}
	updated = updated.WithModelContext("large", 0)
	if updated.ContextFor("large", 500000) != 64000 {
		t.Fatal("automatic mode retained model override")
	}
	updated.Provider = "ollama"
	if updated.ContextFor("large", 0) != 8192 {
		t.Fatal("provider switch leaked remote capacity into local inference")
	}
}

func TestModelContextsPersistAndRejectInvalidValues(t *testing.T) {
	c := Default().WithModelContext("model.with/dots:tag", 1000000)
	path := File(t.TempDir())
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.ContextFor("model.with/dots:tag", 0) != 1000000 {
		t.Fatalf("model capacity did not round trip: %v", err)
	}
	if err := c.WithModelContext("model", -1).Validate(); err == nil {
		t.Fatal("negative model capacity accepted")
	}
	if err := c.WithModelContext(" ", 1).Validate(); err == nil {
		t.Fatal("empty model id accepted")
	}
}
