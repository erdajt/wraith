package plugin

import (
	"encoding/json"
	"regexp"
	"slices"

	"github.com/erdajt/wraith/internal/modeltool"
	lua "github.com/yuin/gopher-lua"
)

var extensionName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

// installExtensions registers transport and model tool handlers through owned definition layers.
func (e *engine) installExtensions(api *lua.LTable) {
	e.providerLayers = map[string][]ProviderDefinition{}
	e.toolLayers = map[string][]ToolDefinition{}
	for _, kind := range []string{"providers", "tools"} {
		api.RawSetString(kind, e.L.SetFuncs(e.L.NewTable(), map[string]lua.LGFunction{
			"register": func(L *lua.LState) int {
				e.registration(L)
				name, options := L.CheckString(1), L.CheckTable(2)
				if !extensionName.MatchString(name) {
					L.RaiseError("invalid extension name")
				}
				var description string
				var replace bool
				approval := "required"
				var parameters json.RawMessage
				callbacks := map[string]*lua.LFunction{}
				options.ForEach(func(key, value lua.LValue) {
					switch key.String() {
					case "description":
						decodeTypedValue(L, value, &description)
					case "replace":
						decodeTypedValue(L, value, &replace)
					case "approval":
						if kind != "tools" {
							L.RaiseError("provider does not accept approval")
						}
						decodeTypedValue(L, value, &approval)
					case "parameters":
						if kind != "tools" {
							L.RaiseError("provider does not accept parameters")
						}
						decoded, err := e.jsonCodec.toGo(value, 0, &jsonBudget{active: map[*lua.LTable]bool{}})
						if err != nil {
							L.RaiseError("tool parameters: %v", err)
						}
						parameters, err = json.Marshal(decoded)
						if err != nil {
							L.RaiseError("tool parameters: %v", err)
						}
					case "models", "show", "chat", "run":
						fn, ok := value.(*lua.LFunction)
						if !ok || kind == "providers" && key.String() == "run" || kind == "tools" && key.String() != "run" {
							L.RaiseError("invalid extension callback")
						}
						callbacks[key.String()] = fn
					default:
						L.RaiseError("unknown extension option: %s", key)
					}
				})
				if len(description) > 4096 {
					L.RaiseError("extension description exceeds 4096 bytes")
				}
				owner, lease := e.owner, e.surfaceLease()
				if kind == "providers" {
					layers := e.providerLayers[name]
					if len(layers) > 0 && !replace || len(layers) >= 32 || len(layers) == 0 && len(e.providerLayers) >= 32 {
						L.RaiseError("provider exists or registration limit reached")
					}
					if callbacks["models"] == nil || callbacks["show"] == nil || callbacks["chat"] == nil {
						L.RaiseError("provider requires models show and chat callbacks")
					}
					e.pruneCallbacks()
					if len(e.functions)+3 > 512 {
						L.RaiseError("provider callbacks exceed runtime limit")
					}
					definition := ProviderDefinition{Name: name, Description: description, Owner: owner, Lease: lease}
					definition.Models = e.callback(L, callbacks["models"])
					definition.Show = e.callback(L, callbacks["show"])
					definition.Chat = e.callback(L, callbacks["chat"])
					definition.Revision = e.serial
					layers = slices.DeleteFunc(layers, func(old ProviderDefinition) bool { return old.Owner == owner })
					e.providerLayers[name] = append(layers, definition)
				} else {
					layers := e.toolLayers[name]
					if slices.Contains([]string{"run", "command", "output", "poll", "request_command", "read_output", "wraith-run", "wraith-tool"}, name) || len(layers) > 0 && !replace || len(layers) >= 32 || len(layers) == 0 && len(e.toolLayers) >= 64 {
						L.RaiseError("tool is reserved or registration limit reached")
					}
					if callbacks["run"] == nil || !slices.Contains([]string{"required", "read_only"}, approval) {
						L.RaiseError("tool requires a callback and valid approval policy")
					}
					if _, err := modeltool.Compile(parameters); err != nil {
						L.RaiseError("%v", err)
					}
					callback := e.callback(L, callbacks["run"])
					definition := ToolDefinition{Name: name, Description: description, Owner: owner, Lease: lease, Callback: callback, Approval: approval, Parameters: parameters, Revision: e.serial}
					layers = slices.DeleteFunc(layers, func(old ToolDefinition) bool { return old.Owner == owner })
					e.toolLayers[name] = append(layers, definition)
				}
				e.refreshExtensions()
				return 0
			},
			"unregister": func(L *lua.LState) int {
				e.registration(L)
				name := L.CheckString(1)
				if kind == "providers" {
					e.providerLayers[name] = slices.DeleteFunc(e.providerLayers[name], func(definition ProviderDefinition) bool { return definition.Owner == e.owner })
					if len(e.providerLayers[name]) == 0 {
						delete(e.providerLayers, name)
					}
				} else {
					e.toolLayers[name] = slices.DeleteFunc(e.toolLayers[name], func(definition ToolDefinition) bool { return definition.Owner == e.owner })
					if len(e.toolLayers[name]) == 0 {
						delete(e.toolLayers, name)
					}
				}
				e.refreshExtensions()
				return 0
			},
		}))
	}
}
