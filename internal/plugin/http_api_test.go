package plugin

import (
	"context"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestHTTPOptionsPreserveRequestBytesAndRejectInvalidFields(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	var got httpOperation
	L.SetGlobal("parse", L.NewFunction(func(L *lua.LState) int {
		got = parseHTTPRequest(L)
		return 0
	}))
	if err := L.DoString(`parse({url="https://example.invalid/api",method="POST",body=string.char(0,255),headers={["X-Test"]="value"},timeout_ms=100,max_bytes=50,redirects=1})`); err != nil {
		t.Fatal(err)
	}
	if got.method != "POST" || string(got.body) != "\x00\xff" || got.headers.Get("X-Test") != "value" || got.limit != 50 || got.redirects != 1 {
		t.Fatalf("request options changed: %+v", got)
	}
	for _, options := range []string{
		`{url="file:///tmp/input"}`, `{url="http://"}`, `{url="/relative"}`,
		`{url="https://example.invalid",method="GET\r\n"}`,
		`{url="https://example.invalid",headers={Host="elsewhere.invalid"}}`,
		`{url="https://example.invalid",headers={["X-Test"]="value\r\nInjected: yes"}}`,
		`{url="https://example.invalid",headers={["bad name"]="value"}}`,
		`{url="https://example.invalid",headers={test=true}}`,
		`{url="https://example.invalid",timeout_ms=0}`,
		`{url="https://example.invalid",max_bytes=5000000}`,
		`{url="https://example.invalid",redirects=0/0}`,
		`{url="https://example.invalid",insecure=true}`,
	} {
		if err := L.DoString("parse(" + options + ")"); err == nil {
			t.Errorf("invalid request accepted: %s", options)
		}
	}
}

func TestHTTPRequestIsReservedWithoutStartingNetworkWork(t *testing.T) {
	r := openTestRuntime(t, `wraith.http.request({url="http://unreachable.invalid"}, function() end)`)
	m := r.Manifest()
	if len(m.Resources) != 1 {
		t.Fatalf("request reservation missing: %+v", m.Resources)
	}
	status, err := r.services.Status(m.Resources[0], "config")
	if err != nil || status["state"] != "reserved" {
		t.Fatalf("request ran before acceptance: %v, %v", status, err)
	}
	if err := r.services.Cancel(m.Resources[0], "config"); err != nil {
		t.Fatal(err)
	}
	if err := r.Activate(m.Resources); err != nil {
		t.Fatal(err)
	}
	event := receiveResource(t, r)
	if !event.Final || event.Data["state"] != "cancelled" || event.Data["error"] != context.Canceled.Error() {
		t.Fatalf("cancelled request performed work: %+v", event)
	}
}
