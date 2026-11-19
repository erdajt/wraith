package plugin

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestDocumentedExtensionSetupLoadsAsOneConfiguration(t *testing.T) {
	data, err := os.ReadFile("../../docs/lua/extensions.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```lua\\n(.*?)\\n```").FindAllSubmatch(data, -1)
	if len(blocks) != 5 {
		t.Fatalf("expected five runnable setup examples, found %d", len(blocks))
	}
	var source strings.Builder
	for _, block := range blocks {
		source.Write(block[1])
		source.WriteByte('\n')
	}
	runtime := openTestRuntime(t, source.String())
	manifest := runtime.Manifest()
	if len(manifest.Providers) != 1 || len(manifest.Tools) != 1 || len(manifest.Hooks) != 1 {
		t.Fatalf("documentation omitted extension registrations: %+v", manifest)
	}
	if manifest.Config.Provider != "local_example" || manifest.Config.Model != "example-model" || manifest.Config.Providers["local_example"].Kind != "plugin" {
		t.Fatal("documented provider profile did not become active")
	}
}
