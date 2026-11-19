package plugin

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// TestDialogOptions keeps stable identities and rejects ambiguous or unsafe display values.
func TestDialogOptions(t *testing.T) {
	for _, test := range []struct {
		name, kind, source, want string
	}{
		{"select", "select", `return {"first", {id="target", label="界", detail="scope"}}, {title="choose"}`, ""},
		{"input", "input", `return {title="draft", initial="first\nsecond"}`, ""},
		{"duplicate", "select", `return {{id="x", label="one"}, {id="x", label="two"}}, {}`, "duplicate"},
		{"sparse", "select", `return {[1]="one", [3]="three"}, {}`, "requires"},
		{"named", "select", `return {name="one"}, {}`, "consecutive"},
		{"control", "select", `return {"\027[31m"}, {}`, "control"},
		{"initial control", "input", `return {initial="\000"}`, "control"},
		{"unknown", "input", `return {mystery=true}`, "unknown"},
		{"empty item field", "select", `return {{label="first", [""]="ignored"}}, {}`, `unknown item field: ""`},
	} {
		t.Run(test.name, func(t *testing.T) {
			L := lua.NewState()
			defer L.Close()
			if err := L.DoString(test.source); err != nil {
				t.Fatal(err)
			}
			dialog, err := parseDialog(L, test.kind)
			if test.want != "" {
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("error = %v, want %q", err, test.want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.kind == "select" && (dialog.Items[1].ID != "target" || dialog.Items[1].Label != "界") {
				t.Fatalf("items = %#v", dialog.Items)
			}
			if test.kind == "input" && dialog.Initial != "first\nsecond" {
				t.Fatalf("initial = %q", dialog.Initial)
			}
		})
	}
}
