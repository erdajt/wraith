package plugin

import (
	"slices"

	"github.com/erdajt/wraith/internal/ollama"
	lua "github.com/yuin/gopher-lua"
)

// HookMessage exposes editable text or an opaque reference to host-protected evidence.
type HookMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Reference string `json:"reference,omitempty"`
}

// RequestData contains request fields that transformations may replace.
type RequestData struct {
	Model    string         `json:"model"`
	Messages []HookMessage  `json:"messages"`
	Think    any            `json:"think,omitempty"`
	Options  ollama.Options `json:"options"`
}

// CommandData preserves the exact execution tuple presented for approval.
type CommandData struct {
	Command     string   `json:"command"`
	Directory   string   `json:"directory"`
	Shell       string   `json:"shell"`
	Environment []string `json:"environment"`
	TimeoutMS   int64    `json:"timeout_ms"`
	Background  bool     `json:"background"`
}

// HookRequest separates immutable host identity from the replaceable typed payload.
type HookRequest struct {
	Kind      string       `json:"kind"`
	Phase     string       `json:"phase"`
	Purpose   string       `json:"purpose"`
	ID        string       `json:"id"`
	SessionID string       `json:"session_id"`
	Directory string       `json:"directory"`
	Request   *RequestData `json:"request,omitempty"`
	Command   *CommandData `json:"command,omitempty"`
}

// HookResult returns a transformation or an explicit denial without host operations.
type HookResult struct {
	Request  *RequestData `json:"request,omitempty"`
	Command  *CommandData `json:"command,omitempty"`
	Decision string       `json:"decision,omitempty"`
	Reason   string       `json:"reason,omitempty"`
}

// HookDefinition identifies one ordered callback and its package lifetime.
type HookDefinition struct {
	ID, Kind, Callback, Owner, Lease string
	Priority                         int
	Sequence                         uint64
	Purposes                         []string
}

// installHooks registers pure callbacks without exposing model or execution authority.
func (e *engine) installHooks(api *lua.LTable) {
	api.RawSetString("hooks", e.L.SetFuncs(e.L.NewTable(), map[string]lua.LGFunction{
		"add": func(L *lua.LState) int {
			e.registration(L)
			kind := L.CheckString(1)
			if kind != "before_request" && kind != "before_command" {
				L.RaiseError("unknown hook kind: %s", kind)
			}
			definition := HookDefinition{Kind: kind, Owner: e.owner, Lease: e.surfaceLease(), Purposes: []string{"chat"}}
			if options := L.OptTable(3, nil); options != nil {
				var decoded struct {
					Priority int      `json:"priority"`
					Purposes []string `json:"purposes"`
				}
				decodeTypedValue(L, options, &decoded)
				definition.Priority = decoded.Priority
				if decoded.Purposes != nil {
					definition.Purposes = decoded.Purposes
				}
			}
			if definition.Priority < -10000 || definition.Priority > 10000 || len(definition.Purposes) == 0 || len(definition.Purposes) > 3 {
				L.RaiseError("invalid hook priority or purposes")
			}
			for _, purpose := range definition.Purposes {
				if !slices.Contains([]string{"chat", "extract", "audit"}, purpose) {
					L.RaiseError("invalid hook purpose: %s", purpose)
				}
			}
			count := 0
			for _, hook := range e.manifest.Hooks {
				if hook.Kind == kind {
					count++
				}
			}
			if count >= 32 {
				L.RaiseError("hook kind exceeds 32 callbacks")
			}
			definition.Callback = e.callback(L, L.Get(2))
			definition.ID, definition.Sequence = definition.Callback, e.serial
			e.manifest.Hooks = append(e.manifest.Hooks, definition)
			L.Push(lua.LString(definition.ID))
			return 1
		},
		"remove": func(L *lua.LState) int {
			e.registration(L)
			id := L.CheckString(1)
			for _, hook := range e.manifest.Hooks {
				if hook.ID == id && hook.Owner != e.owner {
					L.RaiseError("hook belongs to another package")
				}
			}
			e.manifest.Hooks = slices.DeleteFunc(e.manifest.Hooks, func(hook HookDefinition) bool { return hook.ID == id })
			return 0
		},
	}))
}
