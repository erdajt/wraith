package plugin

import (
	"bytes"
	"math"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestResourceValuePreservesDetachedBinaryPayloads(t *testing.T) {
	original := []byte{0xe2, 0x82, 0, 0xff, '\r', '\n'}
	payload := map[string]any{"bytes": original, "offset": int64(42)}
	detached, err := cloneResourceValue(payload)
	if err != nil {
		t.Fatal(err)
	}
	original[0] = 0
	payload["offset"] = int64(99)
	L := lua.NewState()
	defer L.Close()
	value, err := encodeResourceValue(L, detached)
	if err != nil {
		t.Fatal(err)
	}
	table := value.(*lua.LTable)
	if !bytes.Equal([]byte(table.RawGetString("bytes").String()), []byte{0xe2, 0x82, 0, 0xff, '\r', '\n'}) || table.RawGetString("offset") != lua.LNumber(42) {
		t.Fatalf("resource payload changed: %v", table.RawGetString("bytes"))
	}
}

func TestResourceValueRejectsCyclesAndUnsupportedNumbers(t *testing.T) {
	cycle := map[string]any{}
	cycle["self"] = cycle
	for _, value := range []any{cycle, math.NaN(), math.Inf(1), uint64(1 << 54), make([]byte, (8<<20)+1), func() {}} {
		if _, err := cloneResourceValue(value); err == nil {
			t.Fatalf("accepted unsupported value %T", value)
		}
	}
}
