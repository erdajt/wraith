package plugin

import (
	"math"

	lua "github.com/yuin/gopher-lua"
)

// installProcess exposes trusted subprocesses through owned, cancellable resource handles.
func (e *engine) installProcess(api *lua.LTable) {
	L := e.L
	api.RawSetString("process", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"start": func(L *lua.LState) int {
			options := parseProcessOptions(L, L.CheckTable(1))
			callback := L.CheckFunction(2)
			process := newManagedProcess(options)
			id := e.reserveResource(L, "process", callback, process.run)
			e.services.mu.Lock()
			record := e.services.records[id]
			if record != nil {
				record.control = process
			}
			e.services.mu.Unlock()
			if record == nil {
				L.RaiseError("process runtime closed during reservation")
			}
			L.Push(lua.LString(id))
			return 1
		},
		"write": func(L *lua.LState) int {
			process := e.processHandle(L, L.CheckString(1))
			data := L.CheckString(2)
			if err := process.enqueue([]byte(data), false); err != nil {
				L.RaiseError("process.write: %v", err)
			}
			L.Push(lua.LNumber(len(data)))
			return 1
		},
		"close_stdin": func(L *lua.LState) int {
			process := e.processHandle(L, L.CheckString(1))
			if err := process.enqueue(nil, true); err != nil {
				L.RaiseError("process.close_stdin: %v", err)
			}
			return 0
		},
		"output": func(L *lua.LState) int {
			process := e.processHandle(L, L.CheckString(1))
			stream, offset, limit := "stdout", int64(0), 64<<10
			if options := L.OptTable(2, nil); options != nil {
				options.ForEach(func(key, value lua.LValue) {
					name, ok := key.(lua.LString)
					if !ok {
						L.RaiseError("process output options require named fields")
					}
					switch string(name) {
					case "stream":
						text, ok := value.(lua.LString)
						if !ok || text != "stdout" && text != "stderr" {
							L.RaiseError("process output stream must be stdout or stderr")
						}
						stream = string(text)
					case "offset", "max_bytes":
						number, ok := value.(lua.LNumber)
						maximum, minimum := float64(1<<53), float64(0)
						if name == "max_bytes" {
							maximum, minimum = 1<<20, 1
						}
						if !ok || math.IsNaN(float64(number)) || math.Trunc(float64(number)) != float64(number) || float64(number) < minimum || float64(number) > maximum {
							L.RaiseError("process output %s requires an integer between %.0f and %.0f", name, minimum, maximum)
						}
						if name == "offset" {
							offset = int64(number)
						} else {
							limit = int(number)
						}
					default:
						L.RaiseError("unknown process output option: %s", name)
					}
				})
			}
			data, err := process.output(stream, offset, limit)
			if err != nil {
				L.RaiseError("process.output: %v", err)
			}
			value, err := encodeResourceValue(L, data)
			if err != nil {
				L.RaiseError("process.output: %v", err)
			}
			L.Push(value)
			return 1
		},
	}))
}

// processHandle checks runtime ownership before exposing a process controller to a Lua callback.
func (e *engine) processHandle(L *lua.LState, id string) *managedProcess {
	e.services.mu.Lock()
	record := e.services.records[id]
	var process *managedProcess
	if record != nil && record.owner == e.owner && record.kind == "process" {
		process, _ = record.control.(*managedProcess)
	}
	e.services.mu.Unlock()
	if process == nil {
		L.RaiseError("process is missing or belongs to another owner")
	}
	return process
}
