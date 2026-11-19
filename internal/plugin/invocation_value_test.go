package plugin

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInvocationPayloadBoundsAndDetachment(t *testing.T) {
	input := json.RawMessage(`{"message":"retained"}`)
	detached, err := invocationPayload(input)
	if err != nil {
		t.Fatal(err)
	}
	input[2] = 'x'
	if string(detached) != `{"message":"retained"}` {
		t.Fatal("invocation retained caller bytes")
	}
	for _, value := range []string{
		`{"invalid":`,
		`"` + string([]byte{255}) + `"`,
		strings.Repeat("[", 34) + "0" + strings.Repeat("]", 34),
		"[" + strings.Repeat("0,", 16384) + "0]",
		`"` + strings.Repeat("x", 1<<20) + `"`,
	} {
		if _, err := invocationPayload(json.RawMessage(value)); err == nil {
			t.Fatal("invalid invocation payload accepted")
		}
	}
}
