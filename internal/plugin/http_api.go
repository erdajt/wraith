package plugin

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// installHTTP reserves validated HTTP requests without starting network work during Lua execution.
func (e *engine) installHTTP(api *lua.LTable) {
	api.RawSetString("http", e.L.SetFuncs(e.L.NewTable(), map[string]lua.LGFunction{
		"request": func(L *lua.LState) int {
			op := parseHTTPRequest(L)
			callback := L.CheckFunction(2)
			id := e.reserveResource(L, "http", callback, func(ctx context.Context, sink *resourceSink) resourceResult {
				data, err := op.run(ctx)
				return resourceResult{Data: data, Err: err}
			})
			L.Push(lua.LString(id))
			return 1
		},
	}))
}

// parseHTTPRequest copies bounded request options and rejects malformed headers before reservation.
func parseHTTPRequest(L *lua.LState) httpOperation {
	op := httpOperation{method: "GET", headers: http.Header{}, timeout: 30 * time.Second, limit: 1 << 20}
	L.CheckTable(1).ForEach(func(key, value lua.LValue) {
		name, ok := key.(lua.LString)
		if !ok {
			L.RaiseError("http.request: option names require strings")
		}
		switch string(name) {
		case "url", "method", "body":
			text, ok := value.(lua.LString)
			if !ok {
				L.RaiseError("http.request: %s requires a string", name)
			}
			switch string(name) {
			case "url":
				if len(text) > 8192 {
					L.RaiseError("http.request: url exceeds 8 kib")
				}
				op.url = string(text)
			case "method":
				if len(text) > 32 || !httpToken(string(text)) {
					L.RaiseError("http.request: invalid method")
				}
				op.method = string(text)
			case "body":
				if len(text) > 4<<20 {
					L.RaiseError("http.request: body exceeds 4 mib")
				}
				op.body = []byte(text)
			}
		case "headers":
			table, ok := value.(*lua.LTable)
			if !ok {
				L.RaiseError("http.request: headers require a table")
			}
			count, size := 0, 0
			table.ForEach(func(key, value lua.LValue) {
				name, nameOK := key.(lua.LString)
				text, textOK := value.(lua.LString)
				if !nameOK || !textOK || !httpToken(string(name)) || strings.EqualFold(string(name), "Host") {
					L.RaiseError("http.request: headers require valid string names and values; host comes from url")
				}
				for _, character := range []byte(text) {
					if character < 32 && character != '\t' || character == 127 {
						L.RaiseError("http.request: invalid header value")
					}
				}
				count++
				size += len(name) + len(text)
				if count > 128 || size > 64<<10 {
					L.RaiseError("http.request: headers exceed count or size limit")
				}
				op.headers.Add(string(name), string(text))
			})
		case "timeout_ms", "max_bytes", "redirects":
			number, ok := value.(lua.LNumber)
			minimum, maximum := 1, 300000
			if name == "max_bytes" {
				maximum = 4 << 20
			} else if name == "redirects" {
				minimum, maximum = 0, 10
			}
			if !ok || number < lua.LNumber(minimum) || number > lua.LNumber(maximum) || math.Trunc(float64(number)) != float64(number) {
				L.RaiseError("http.request: %s requires an integer between %d and %d", name, minimum, maximum)
			}
			switch name {
			case "timeout_ms":
				op.timeout = time.Duration(number) * time.Millisecond
			case "max_bytes":
				op.limit = int(number)
			case "redirects":
				op.redirects = int(number)
			}
		default:
			L.RaiseError("http.request: unknown option %s", name)
		}
	})
	address, err := url.ParseRequestURI(op.url)
	if err != nil || address.Scheme != "http" && address.Scheme != "https" || address.Hostname() == "" {
		L.RaiseError("http.request: url requires an absolute http or https address")
	}
	return op
}

// httpToken validates the shared HTTP token syntax for methods and header names.
func httpToken(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range []byte(value) {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", rune(character)) {
			continue
		}
		return false
	}
	return true
}
