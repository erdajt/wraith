package provider

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

func TestCompactionRetainsTrustedRunReferencesAcrossRequests(t *testing.T) {
	c := config.Default()
	c.Host, c.Model, c.Context = "http://local.invalid", "test", 32768
	client := New(c)
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TOKEN", Value: "ab"}, {Name: "FIELD", Value: "facts"}}, nil)
	id, unknown := strings.Repeat("ab", 12), strings.Repeat("ab", 11)+"cd"
	s := session.New(c.Model)
	s.Messages = append(s.Messages, (session.Evidence{ID: id, Kind: "result", Output: "ab facts"}).Message())
	for range 10 {
		s.Messages = append(s.Messages, session.Message{Role: "user", Content: strings.Repeat("repeat observations from the isolated test service. ", 100)}, session.Message{Role: "assistant", Content: "id: " + unknown})
	}
	seen := map[string]bool{}
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		var request ollama.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		text := request.Messages[len(request.Messages)-1].Content
		if strings.Contains(text, unknown) {
			t.Fatal("unknown id bypassed redaction")
		}
		task := "extract"
		result := `{"summary":"service reviewed","facts":["tracked command id: ` + id + `"]}`
		if strings.HasPrefix(request.Messages[0].Content, contextual.AuditPrompt) {
			task, result = "audit", `{"missing":[]}`
		} else if !strings.HasPrefix(request.Messages[0].Content, contextual.CompactPrompt) {
			t.Fatal("compaction schema corrupted")
		}
		if strings.Contains(text, "id: "+id) {
			seen[task] = true
		}
		data, _ := json.Marshal(ollama.Response{Done: true, Message: session.Message{Role: "assistant", Content: result}})
		return response(string(data)), nil
	})
	compacted, err := contextual.Compact(t.Context(), client, s, c, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !seen["extract"] || !seen["audit"] || !compacted.RunIDs()[id] {
		t.Fatalf("references lost: %v %+v", seen, compacted.RunReferences)
	}
	for _, m := range compacted.Messages {
		if m.Evidence != nil {
			t.Fatal("test did not compact evidence away")
		}
	}
	request := ollama.Request{RunIDs: compacted.RunIDs(), ExecutionMode: c.Execution.Mode, Messages: contextual.ForChat(compacted, c, "")}
	_, prepared, err := client.privateRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prepared.Messages[0].Content, "id: "+id) {
		t.Fatal("compacted reference lost in next chat")
	}
}

func TestKnownRunReferenceDoesNotExemptUnrelatedPrivateProse(t *testing.T) {
	id := strings.Repeat("ab", 12)
	client := New(config.Default())
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "SECRET", Value: id}}, nil)
	request := ollama.Request{RunIDs: map[string]bool{id: true}, Messages: []session.Message{{Role: "user", Content: "secret is " + id + "\ntracked command id: " + id + "\nlonger id: " + id + "a"}}}
	_, got, err := client.privateRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	want := "secret is wraith::SECRET\ntracked command id: " + id + "\nlonger id: wraith::SECRETa"
	if got.Messages[0].Content != want {
		t.Fatalf("reference boundary: %q", got.Messages[0].Content)
	}
}
