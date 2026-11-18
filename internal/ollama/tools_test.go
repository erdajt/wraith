package ollama

import (
	"strings"
	"testing"
)

func TestNativeCallsRemainPrivateUntilSuccessfulCompletion(t *testing.T) {
	t.Parallel()
	call := `{"message":{"tool_calls":[{"function":{"name":"request_command","arguments":{"command":"printf ready","reason":"check"}}}]}}` + "\n"
	request := Request{Tools: []Tool{{Type: "function"}}}
	result, err := fixture(call+`{"done":true,"done_reason":"stop"}`, 200).Chat(t.Context(), request, func(chunk Response) {
		if len(chunk.Calls) > 0 || strings.Contains(chunk.Message.Content, "printf") {
			t.Fatal("arguments emitted before validation")
		}
	})
	if err != nil || len(result.Calls) != 1 || result.Calls[0].Name != "request_command" {
		t.Fatalf("native call lost: %+v, %v", result, err)
	}

	for _, body := range []string{call, call + call + `{"done":true,"done_reason":"stop"}`, call + `{"done":true,"done_reason":"length"}`, call + `{"error":"failed"}`} {
		result, err := fixture(body, 200).Chat(t.Context(), request, nil)
		if err == nil || len(result.Calls) > 0 {
			t.Fatal("failed stream retained actionable calls")
		}
	}
	if _, err := fixture(call+`{"done":true,"done_reason":"stop"}`, 200).Chat(t.Context(), Request{}, nil); err == nil {
		t.Fatal("unadvertised native call accepted")
	}
}
