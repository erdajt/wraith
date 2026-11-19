package plugin

import (
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// parseProcessOptions copies a validated launch tuple before reserving external work.
func parseProcessOptions(L *lua.LState, table *lua.LTable) processOptions {
	options := processOptions{limit: 256 << 10}
	var shell, shellPath string
	var environment *lua.LTable
	clearEnvironment := false
	table.ForEach(func(key, value lua.LValue) {
		name, ok := key.(lua.LString)
		if !ok {
			L.RaiseError("process options require named fields")
		}
		switch string(name) {
		case "argv":
			argv, ok := value.(*lua.LTable)
			if !ok || argv.Len() == 0 || argv.Len() > 256 {
				L.RaiseError("process argv requires between 1 and 256 strings")
			}
			options.argv = make([]string, argv.Len())
			count, size := 0, 0
			argv.ForEach(func(key, value lua.LValue) {
				index, ok := key.(lua.LNumber)
				text, textOK := value.(lua.LString)
				if !ok || index < 1 || index > lua.LNumber(len(options.argv)) || math.Trunc(float64(index)) != float64(index) || !textOK || strings.ContainsRune(string(text), 0) {
					L.RaiseError("process argv must be a consecutive string array without nul bytes")
				}
				options.argv[int(index)-1] = string(text)
				count++
				size += len(text)
			})
			if count != len(options.argv) || size > 64<<10 || options.argv[0] == "" {
				L.RaiseError("process argv is incomplete, empty or exceeds 64 kib")
			}
		case "cwd", "shell", "shell_path", "mode":
			text, ok := value.(lua.LString)
			if !ok || strings.ContainsRune(string(text), 0) || len(text) > 64<<10 {
				L.RaiseError("process %s requires a string without nul bytes within 64 kib", name)
			}
			switch string(name) {
			case "cwd":
				if text == "" {
					L.RaiseError("process cwd cannot be empty")
				}
				options.directory = string(text)
			case "shell":
				shell = string(text)
			case "shell_path":
				shellPath = string(text)
			case "mode":
				if text != "capture" && text != "protocol" {
					L.RaiseError("process mode must be capture or protocol")
				}
				options.protocol = text == "protocol"
			}
		case "stdin", "clear_env":
			flag, ok := value.(lua.LBool)
			if !ok {
				L.RaiseError("process %s requires a boolean", name)
			}
			if name == "stdin" {
				options.stdin = bool(flag)
			} else {
				clearEnvironment = bool(flag)
			}
		case "env":
			var ok bool
			environment, ok = value.(*lua.LTable)
			if !ok {
				L.RaiseError("process env requires a table")
			}
		case "timeout_ms", "max_output_bytes":
			number, ok := value.(lua.LNumber)
			maximum, minimum := 86400000, 0
			if name == "max_output_bytes" {
				minimum, maximum = 1024, 4<<20
			}
			if !ok || math.IsNaN(float64(number)) || math.Trunc(float64(number)) != float64(number) || number < lua.LNumber(minimum) || number > lua.LNumber(maximum) {
				L.RaiseError("process %s requires an integer between %d and %d", name, minimum, maximum)
			}
			if name == "timeout_ms" {
				options.timeout = time.Duration(number) * time.Millisecond
			} else {
				options.limit = int(number)
			}
		default:
			L.RaiseError("unknown process option: %s", name)
		}
	})
	if table.RawGetString("shell") != lua.LNil {
		if shell == "" {
			L.RaiseError("process shell command cannot be empty")
		}
		if len(options.argv) != 0 {
			L.RaiseError("process requires argv or shell, not both")
		}
		if shellPath == "" {
			shellPath = "/bin/sh"
		}
		if !filepath.IsAbs(shellPath) {
			L.RaiseError("process shell_path must be absolute")
		}
		options.argv = []string{shellPath, "-c", shell}
	} else if table.RawGetString("shell_path") != lua.LNil || len(options.argv) == 0 {
		L.RaiseError("process requires argv or a nonempty shell command")
	}
	if options.directory != "" {
		absolute, err := filepath.Abs(options.directory)
		if err != nil {
			L.RaiseError("process cwd: %v", err)
		}
		options.directory = absolute
	}
	values := map[string]string{}
	if !clearEnvironment {
		for _, entry := range os.Environ() {
			name, value, _ := strings.Cut(entry, "=")
			values[name] = value
		}
	}
	if environment != nil {
		count := 0
		environment.ForEach(func(key, value lua.LValue) {
			name, ok := key.(lua.LString)
			count++
			if !ok || name == "" || strings.ContainsAny(string(name), "=\x00") || count > 128 {
				L.RaiseError("process env requires at most 128 valid variable names")
			}
			if value == lua.LFalse {
				delete(values, string(name))
				return
			}
			text, ok := value.(lua.LString)
			if !ok || strings.ContainsRune(string(text), 0) {
				L.RaiseError("process env values require strings without nul bytes or false to remove")
			}
			values[string(name)] = string(text)
		})
	}
	options.environment = make([]string, 0, len(values))
	size := 0
	for name, value := range values {
		entry := name + "=" + value
		size += len(entry)
		options.environment = append(options.environment, entry)
	}
	if size > 1<<20 {
		L.RaiseError("process environment exceeds 1 mib")
	}
	slices.Sort(options.environment)
	return options
}
