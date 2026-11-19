package plugin

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// guardHookAPIs checks captured host functions as well as calls through the public table.
func (e *engine) guardHookAPIs(api *lua.LTable) {
	visited := map[*lua.LTable]bool{}
	var wrap func(*lua.LTable, string)
	wrap = func(table *lua.LTable, path string) {
		if visited[table] {
			return
		}
		visited[table] = true
		table.ForEach(func(key, value lua.LValue) {
			name := path + "." + key.String()
			switch value := value.(type) {
			case *lua.LTable:
				wrap(value, name)
			case *lua.LFunction:
				if !value.IsG {
					return
				}
				function := value.GFunction
				table.RawSet(key, e.L.NewFunction(func(L *lua.LState) int {
					if e.pureHook && !strings.HasPrefix(name, "wraith.json.") {
						e.hookViolation = fmt.Errorf("%s is unavailable during pure hooks", name)
						L.RaiseError("%v", e.hookViolation)
					}
					return function(L)
				}))
			}
		})
	}
	wrap(api, "wraith")
}
