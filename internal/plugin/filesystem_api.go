package plugin

import (
	"context"
	"math"
	"path/filepath"
	"slices"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// installFilesystem exposes immediate filesystem calls with copied options and bounded results.
func (e *engine) installFilesystem(api *lua.LTable) {
	L := e.L
	methods := L.NewTable()
	for _, kind := range []string{"read", "write", "stat", "list", "mkdir", "remove", "rename"} {
		methods.RawSetString(kind, L.NewFunction(func(L *lua.LState) int {
			op, options := parseFileOperation(L, kind)
			if L.Get(options).Type() == lua.LTFunction {
				L.RaiseError("fs.%s: use %s_async for callbacks", kind, kind)
			}
			ctx := L.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			data, err := op.run(ctx)
			if err != nil {
				L.RaiseError("%v", err)
			}
			value, err := encodeResourceValue(L, data)
			if err != nil {
				L.RaiseError("fs.%s: %v", kind, err)
			}
			L.Push(value)
			return 1
		}))
		methods.RawSetString(kind+"_async", L.NewFunction(func(L *lua.LState) int {
			op, options := parseFileOperation(L, kind)
			index := options + 1
			if L.Get(options).Type() == lua.LTFunction {
				index = options
			}
			callback := L.CheckFunction(index)
			id := e.reserveResource(L, "fs."+kind, callback, func(ctx context.Context, sink *resourceSink) resourceResult {
				data, err := op.run(ctx)
				return resourceResult{Data: data, Err: err}
			})
			L.Push(lua.LString(id))
			return 1
		}))
	}
	api.RawSetString("fs", methods)
}

// parseFileOperation validates paths and operation-specific options before any filesystem access.
func parseFileOperation(L *lua.LState, kind string) (fileOperation, int) {
	op := fileOperation{kind: kind, path: L.CheckString(1), limit: 1 << 20, mode: 0600}
	options := 2
	allowed := map[string]string{
		"read": " max_bytes offset ", "write": " overwrite mode ", "stat": " follow ",
		"list": " limit offset ", "mkdir": " parents mode ", "remove": " recursive ", "rename": " overwrite ",
	}
	if kind == "write" {
		data := L.CheckString(2)
		if len(data) > 4<<20 {
			L.RaiseError("fs.write: data exceeds 4 mib")
		}
		op.data = []byte(data)
		options = 3
	}
	if kind == "rename" {
		op.destination = L.CheckString(2)
		options = 3
	}
	if kind == "mkdir" {
		op.mode = 0700
	}
	if kind == "list" {
		op.limit = 1024
	}
	for i, path := range []*string{&op.path, &op.destination} {
		if i == 1 && kind != "rename" {
			continue
		}
		if *path == "" || len(*path) > 4096 || strings.ContainsRune(*path, 0) {
			L.RaiseError("fs.%s: path is empty, too long or contains nul", kind)
		}
		absolute, err := filepath.Abs(*path)
		if err != nil {
			L.RaiseError("fs.%s: %v", kind, err)
		}
		*path = absolute
	}
	if table, ok := L.Get(options).(*lua.LTable); ok {
		table.ForEach(func(key, value lua.LValue) {
			name, ok := key.(lua.LString)
			if !ok || !slices.Contains(strings.Fields(allowed[kind]), string(name)) {
				L.RaiseError("fs.%s: unknown option %s", kind, key.String())
			}
			switch string(name) {
			case "overwrite", "recursive", "parents", "follow":
				flag, ok := value.(lua.LBool)
				if !ok {
					L.RaiseError("fs.%s: %s requires a boolean", kind, name)
				}
				switch string(name) {
				case "overwrite":
					op.overwrite = bool(flag)
				case "recursive":
					op.recursive = bool(flag)
				case "parents":
					op.parents = bool(flag)
				case "follow":
					op.follow = bool(flag)
				}
			default:
				number, ok := value.(lua.LNumber)
				if !ok || math.IsNaN(float64(number)) || number < 0 || number > (1<<53)-(4<<20) || math.Trunc(float64(number)) != float64(number) {
					L.RaiseError("fs.%s: %s requires a nonnegative exact integer", kind, name)
				}
				switch string(name) {
				case "offset":
					op.offset = int64(number)
				case "mode":
					if number > 0777 {
						L.RaiseError("fs.%s: mode requires permission bits only", kind)
					}
					op.mode = int(number)
				case "max_bytes", "limit":
					maximum := 4 << 20
					if kind == "list" {
						maximum = 1024
					}
					if number < 1 || number > lua.LNumber(maximum) {
						L.RaiseError("fs.%s: %s must be between 1 and %d", kind, name, maximum)
					}
					op.limit = int(number)
				}
			}
		})
	} else if L.Get(options) != lua.LNil && L.Get(options).Type() != lua.LTFunction {
		L.RaiseError("fs.%s: options require a table", kind)
	}
	return op, options
}
