package plugin

import (
	"errors"
	"fmt"
	"strconv"
	"unicode"
	"unicode/utf8"

	lua "github.com/yuin/gopher-lua"
)

// DialogItem retains selection identity independently of its filtered display position.
type DialogItem struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Detail string `json:"detail,omitempty"`
}

// Dialog describes an owned select or input request without retaining Lua values in the UI.
type Dialog struct {
	ID, Owner, Lease string
	Kind, Title      string
	Initial          string
	Items            []DialogItem
}

// dialogText rejects terminal controls before values enter an interactive frame.
func dialogText(value lua.LValue, name string, limit int, multiline bool) (string, error) {
	text, ok := value.(lua.LString)
	if !ok || len(text) > limit || !utf8.ValidString(string(text)) {
		return "", fmt.Errorf("%s requires utf-8 text within %d bytes", name, limit)
	}
	for _, r := range string(text) {
		if unicode.IsControl(r) && !(multiline && r == '\n') {
			return "", fmt.Errorf("%s contains a control character", name)
		}
	}
	return string(text), nil
}

// parseDialog detaches bounded options and stable select items from the calling VM.
func parseDialog(L *lua.LState, kind string) (Dialog, error) {
	dialog := Dialog{Kind: kind, Title: "select"}
	index := 1
	if kind == "select" {
		items := L.CheckTable(1)
		if items.Len() > 1024 {
			return dialog, errors.New("ui.select accepts at most 1024 items")
		}
		seen := map[string]bool{}
		bytes := 0
		for i := 1; i <= items.Len(); i++ {
			item := DialogItem{ID: strconv.Itoa(i)}
			value := items.RawGetInt(i)
			if fields, ok := value.(*lua.LTable); ok {
				value = fields.RawGetString("label")
				if id := fields.RawGetString("id"); id != lua.LNil {
					var err error
					item.ID, err = dialogText(id, "item id", 256, false)
					if err != nil || item.ID == "" {
						return dialog, errors.New("item id requires nonempty text within 256 bytes")
					}
				}
				if detail := fields.RawGetString("detail"); detail != lua.LNil {
					var err error
					item.Detail, err = dialogText(detail, "item detail", 4096, false)
					if err != nil {
						return dialog, err
					}
				}
			}
			var err error
			item.Label, err = dialogText(value, "item label", 4096, false)
			if err != nil {
				return dialog, err
			}
			if seen[item.ID] {
				return dialog, fmt.Errorf("duplicate item id: %s", item.ID)
			}
			seen[item.ID] = true
			bytes += len(item.ID) + len(item.Label) + len(item.Detail)
			if bytes > 256<<10 {
				return dialog, errors.New("ui.select items exceed 256 kib")
			}
			dialog.Items = append(dialog.Items, item)
		}
		count := 0
		items.ForEach(func(_, _ lua.LValue) { count++ })
		if count != len(dialog.Items) {
			return dialog, errors.New("ui.select items must be a consecutive array")
		}
		index = 2
	} else {
		dialog.Title = "input"
	}

	opts := L.CheckTable(index)
	var failure error
	opts.ForEach(func(key, value lua.LValue) {
		if failure != nil {
			return
		}
		switch key.String() {
		case "title":
			dialog.Title, failure = dialogText(value, "dialog title", 256, false)
		case "initial":
			if kind != "input" {
				failure = errors.New("initial is only valid for ui.input")
				return
			}
			dialog.Initial, failure = dialogText(value, "initial input", 64<<10, true)
		default:
			failure = fmt.Errorf("unknown dialog option: %s", key.String())
		}
	})
	return dialog, failure
}
