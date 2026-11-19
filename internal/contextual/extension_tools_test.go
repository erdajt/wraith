package contextual

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/modeltool"
	"github.com/erdajt/wraith/internal/ollama"
)

func TestExtensionCallsNormalizeAgainstFrozenSchemas(t *testing.T) {
	catalog, err := modeltool.NewCatalog([]modeltool.Definition{{Name: "inspect", Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"additionalProperties":false}`)}})
	if err != nil {
		t.Fatal(err)
	}
	call := ollama.Call{Name: "inspect", Arguments: json.RawMessage(`{"path":"wraith::TARGET"}`)}
	content, err := NormalizeAction("agent", "inspect metadata", []ollama.Call{call}, catalog)
	if err != nil {
		t.Fatal(err)
	}
	request, err := RequestedTool(content)
	if err != nil || request.Tool != "extension" || request.Name != "inspect" || string(request.Arguments) != string(call.Arguments) {
		t.Fatalf("extension normalization: %+v, %v", request, err)
	}
	if command, err := RequestedCommand(content); err != nil || command != nil {
		t.Fatal("extension request became a shell command")
	}
	if display := DisplayContent(content); strings.Contains(display, "wraith-tool") || !strings.Contains(display, "[tool · inspect]") {
		t.Fatalf("extension protocol leaked into display: %s", display)
	}
	for _, mode := range []string{"blocks", ""} {
		if _, err := NormalizeAction(mode, "", []ollama.Call{call}, catalog); err == nil {
			t.Fatalf("extension available in mode %q", mode)
		}
	}
	if _, err := NormalizeAction("agent", "", []ollama.Call{call}); err == nil {
		t.Fatal("unadvertised extension accepted")
	}
	call.Arguments = json.RawMessage(`{"path":false}`)
	if _, err := NormalizeAction("agent", "", []ollama.Call{call}, catalog); err == nil {
		t.Fatal("schema-invalid native call accepted")
	}
}

func TestExtensionProtocolRejectsAmbiguousArgumentsAndMixedFields(t *testing.T) {
	for _, body := range []string{
		`{"tool":"extension","name":"inspect","arguments":{},"id":""}`,
		`{"tool":"extension","name":"inspect","arguments":{},"pattern":"x"}`,
		`{"tool":"poll","id":"0123456789abcdef01234567","name":"inspect"}`,
		`{"tool":"output","id":"0123456789abcdef01234567","arguments":{}}`,
		`{"tool":"extension","name":"request_command","arguments":{}}`,
		`{"tool":"extension","name":"inspect","arguments":[]}`,
		`{"tool":"extension","name":"inspect","arguments":{"x":1,"x":2}}`,
		`{"tool":"extension","name":"inspect","arguments":{"x":9007199254740992}}`,
		`{"tool":"extension","name":"inspect","arguments":{"x":"\ud800"}}`,
	} {
		if _, err := RequestedTool("```wraith-tool\n" + body + "\n```"); err == nil {
			t.Fatalf("ambiguous tool request accepted: %s", body)
		}
	}
	body := `{"tool":"extension","name":"inspect","arguments":{"array":[],"object":{},"value":null}}`
	request, err := RequestedTool("```wraith-tool\n" + body + "\n```")
	if err != nil || string(request.Arguments) != `{"array":[],"object":{},"value":null}` {
		t.Fatalf("explicit JSON types changed: %+v, %v", request, err)
	}
}
