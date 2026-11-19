package provider

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/modeltool"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

func extensionClient(t *testing.T) *Client {
	t.Helper()
	client := New(config.Default())
	catalog, err := modeltool.NewCatalog([]modeltool.Definition{{Name: "inspect", Description: "inspect metadata", Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"additionalProperties":false}`)}})
	if err != nil {
		t.Fatal(err)
	}
	client.Tools = catalog
	return client
}

func TestExtensionCatalogAdvertisementFollowsRequestPurpose(t *testing.T) {
	for _, native := range []bool{false, true} {
		client := extensionClient(t)
		client.Profile.ActionProtocol = "text"
		if native {
			client.Profile.ActionProtocol = "native"
		}
		for _, request := range []ollama.Request{{ExecutionMode: "agent"}, {ExecutionMode: "blocks"}, {ExecutionMode: "agent", CompactionTask: "extract"}, {ExecutionMode: "agent", Format: "json"}} {
			_, prepared, err := client.privateRequest(request)
			if err != nil {
				t.Fatal(err)
			}
			visible := request.ExecutionMode == "agent" && request.CompactionTask == "" && request.Format == nil
			advertised := false
			for _, tool := range prepared.Tools {
				advertised = advertised || tool.Function.Name == "inspect"
			}
			if advertised != (native && visible) {
				t.Fatalf("wrong extension transport for %+v", request)
			}
			if !native && visible && !strings.Contains(prepared.Messages[0].Content, `"name":"inspect"`) {
				t.Fatal("text mode omitted tool schemas")
			}
			if !visible {
				for _, message := range prepared.Messages {
					if strings.Contains(message.Content, "Registered extension tools") {
						t.Fatal("extension policy leaked into another request purpose")
					}
				}
			}
		}
	}
}

func TestExtensionSchemasRejectPrivateMetadataBeforeTransport(t *testing.T) {
	client := extensionClient(t)
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "SECRET", Value: "inspect metadata"}}, nil)
	if _, _, err := client.privateRequest(ollama.Request{ExecutionMode: "agent"}); err == nil {
		t.Fatal("private tool description entered model context")
	}
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "SECRET", Value: "path"}}, nil)
	if _, _, err := client.privateRequest(ollama.Request{ExecutionMode: "agent"}); err == nil {
		t.Fatal("private schema key entered model context")
	}
}

func TestExtensionResponseRepairValidatesFrozenSchemaAndPrivateArguments(t *testing.T) {
	client := extensionClient(t)
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	calls := 0
	client.HTTP.Transport = transport(func(request *http.Request) (*http.Response, error) {
		calls++
		var prepared ollama.Request
		if err := json.NewDecoder(request.Body).Decode(&prepared); err != nil {
			t.Fatal(err)
		}
		if calls == 2 && !strings.Contains(prepared.Messages[len(prepared.Messages)-1].Content, "do not match schema") {
			t.Fatal("repair omitted schema failure")
		}
		arguments := `{"path":false}`
		if calls > 1 {
			arguments = `{"path":"private.example"}`
		}
		content := "```wraith-tool\n{\"tool\":\"extension\",\"name\":\"inspect\",\"arguments\":" + arguments + "}\n```"
		data, err := json.Marshal(ollama.Response{Done: true, Message: session.Message{Role: "assistant", Content: content}})
		if err != nil {
			t.Fatal(err)
		}
		return response(string(data)), nil
	})
	result, err := client.ChatWithRepair(t.Context(), ollama.Request{ExecutionMode: "agent"}, nil)
	if err != nil || calls != 2 || strings.Contains(result.Message.Content, "private.example") {
		t.Fatalf("extension repair failed: %d calls, %v", calls, err)
	}
	tool, err := contextual.RequestedTool(result.Message.Content)
	if err != nil || tool == nil || string(tool.Arguments) != `{"path":"wraith::TARGET"}` {
		t.Fatalf("private arguments lost structure: %+v, %v", tool, err)
	}
}
