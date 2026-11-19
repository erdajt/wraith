package plugin

import (
	"encoding/json"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func jsonFixture(t *testing.T) *lua.LState {
	t.Helper()
	L := lua.NewState()
	t.Cleanup(L.Close)
	api := L.NewTable()
	L.SetGlobal("wraith", api)
	(&engine{L: L}).installJSON(api)
	return L
}

func TestJSONPreservesContainerKindsAndNulls(t *testing.T) {
	L := jsonFixture(t)
	source := `local j = wraith.json
local v = j.decode('{"array":[],"object":{},"value":null,"items":[null,"last"]}')
assert(v.value == j.null and v.items[1] == j.null and #v.items == 2)
assert(j.encode(v) == '{"array":[],"items":[null,"last"],"object":{},"value":null}')
assert(j.encode(j.array()) == "[]")
assert(j.encode(j.object()) == "{}")
assert(j.encode({}) == "{}")
assert(j.encode({false, j.null, true}) == "[false,null,true]")
local shared = {value="same"}
assert(j.encode({shared, shared}) == '[{"value":"same"},{"value":"same"}]')`
	if err := L.DoString(source); err != nil {
		t.Fatal(err)
	}
}

func TestJSONRejectsInvalidLuaValues(t *testing.T) {
	for name, expression := range map[string]string{
		"nil":             `nil`,
		"function":        `function() end`,
		"cycle":           `(function() local v={} v.self=v return v end)()`,
		"mixed":           `{[1]="first", name="value"}`,
		"sparse":          `{[2]="second"}`,
		"array object":    `wraith.json.array({name="value"})`,
		"object array":    `wraith.json.object({"value"})`,
		"infinity":        `1/0`,
		"nan":             `0/0`,
		"invalid utf8":    `string.char(255)`,
		"invalid key":     `{[string.char(255)]=true}`,
		"depth":           `(function() local v={} for i=1,34 do v={v} end return v end)()`,
		"items":           `(function() local v={} for i=1,16385 do v[i]=i end return v end)()`,
		"escaped length":  `string.rep(string.char(0), 180000)`,
		"combined length": `{string.rep("a",600000), string.rep("b",600000)}`,
	} {
		t.Run(name, func(t *testing.T) {
			L := jsonFixture(t)
			if err := L.DoString(`wraith.json.encode(` + expression + `)`); err == nil {
				t.Fatal("invalid JSON value accepted")
			}
		})
	}
}

func TestJSONStringsRoundTripExactly(t *testing.T) {
	L := jsonFixture(t)
	for _, text := range []string{"", "é😀", "\x00\r\n\t\\\"", "literal \\ud800"} {
		L.SetGlobal("original", lua.LString(text))
		if err := L.DoString(`assert(wraith.json.decode(wraith.json.encode(original)) == original)`); err != nil {
			t.Fatalf("string %q: %v", text, err)
		}
	}
	if err := L.DoString(`assert(wraith.json.decode('"\\ud83d\\ude00"') == "😀")`); err != nil {
		t.Fatal(err)
	}
}

func TestJSONDecodeRejectsLossyOrExcessiveInput(t *testing.T) {
	for _, input := range []string{`"\ud800"`, `"\udc00"`, `"\ud800\u0061"`, "\"\xff\"", `{} {}`, `1e999`, strings.Repeat(" ", 1<<20) + "0", strings.Repeat("[", 34) + "0" + strings.Repeat("]", 34)} {
		L := jsonFixture(t)
		L.SetGlobal("input", lua.LString(input))
		if err := L.DoString(`wraith.json.decode(input)`); err == nil {
			t.Fatalf("invalid JSON input accepted: %.80q", input)
		}
	}
}

func FuzzJSONSurrogateValidation(f *testing.F) {
	for _, input := range []string{`"\ud800"`, `{"x":"\ud83d\ude00"}`, `"\\ud800"`, `[]`, `"\/"`} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 1<<20 || !json.Valid([]byte(input)) {
			return
		}
		_ = validateJSONEscapes(input)
	})
}
