package plugin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRequestHooksComposeInPriorityOrderAndRespectPurpose(t *testing.T) {
	r := openTestRuntime(t, `
wraith.hooks.add("before_request",function(ctx)
 ctx.request.messages[1].content=ctx.request.messages[1].content.." second"
 return {request=ctx.request}
end,{priority=20})
wraith.hooks.add("before_request",function(ctx)
 ctx.request.messages[1].content=ctx.request.messages[1].content.." first"
 return {request=ctx.request}
end,{priority=10})
wraith.hooks.add("before_request",function() error("audit only") end,{purposes={"audit"}})
`)
	input := HookRequest{Kind: "before_request", Purpose: "chat", Request: &RequestData{Model: "local", Messages: []HookMessage{{Role: "user", Content: "original"}}}}
	result, err := r.Transform(t.Context(), input)
	if err != nil || result.Request.Messages[0].Content != "original first second" {
		t.Fatalf("ordered transform failed: %+v %v", result, err)
	}
	if input.Request.Messages[0].Content != "original" {
		t.Fatal("transform mutated caller-owned input")
	}
	input.Purpose = "audit"
	if _, err := r.Transform(t.Context(), input); err == nil || !strings.Contains(err.Error(), "audit only") {
		t.Fatalf("purpose-specific failure disappeared: %v", err)
	}
}

func TestHookErrorsDenyWithoutBreakingLaterCallbacks(t *testing.T) {
	tests := map[string]string{
		"timeout":        `while true do end`,
		"host operation": `local ok=pcall(wraith.notify,"forbidden"); return nil`,
		"unknown field":  `return {command=ctx.command,execution_allowed=true}`,
		"wrong payload":  `return {request={model="wrong"}}`,
		"invalid tuple":  `ctx.command.shell=""; return {command=ctx.command}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			r := openTestRuntime(t, `wraith.hooks.add("before_command",function(ctx) `+body+` end)
wraith.commands.create("healthy",function() wraith.notify("alive") end)`)
			input := HookRequest{Kind: "before_command", Phase: "preview", Command: &CommandData{Command: "true", Directory: "/tmp", Shell: "/bin/sh", TimeoutMS: 1000}}
			if _, err := r.Transform(t.Context(), input); err == nil {
				t.Fatal("invalid hook succeeded")
			}
			m := r.Manifest()
			pending, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
			if err != nil {
				t.Fatal(err)
			}
			result := receiveResult(t, pending)
			if result.Err != nil || len(result.Effects) != 1 || result.Effects[0].Argument != "alive" {
				t.Fatalf("hook damaged actor state: %+v", result)
			}
		})
	}
}

func TestCommandHookDenialStopsLaterPolicies(t *testing.T) {
	r := openTestRuntime(t, `
wraith.hooks.add("before_command",function() return {decision="deny",reason="outside scope"} end)
wraith.hooks.add("before_command",function() error("must not run") end)
`)
	input := HookRequest{Kind: "before_command", Phase: "execute", Command: &CommandData{Command: "true", Directory: "/tmp", Shell: "/bin/sh", TimeoutMS: 1000}}
	result, err := r.Transform(t.Context(), input)
	if err != nil || result.Decision != "deny" || result.Reason != "outside scope" {
		t.Fatalf("policy denial failed: %+v %v", result, err)
	}
}

func TestTransformCancellationReleasesWaiterDuringTrustedNativeCall(t *testing.T) {
	r := openTestRuntime(t, `wraith.hooks.add("before_command",function() os.execute("sleep 0.2") end)`)
	input := HookRequest{Kind: "before_command", Phase: "preview", Command: &CommandData{Command: "true", Directory: "/tmp", Shell: "/bin/sh", TimeoutMS: 1000}}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := r.Transform(ctx, input)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 150*time.Millisecond {
		t.Fatalf("native call held canceled waiter: %v", err)
	}
}

func TestTransformRejectsMalformedEnvelopeBeforeCallingLua(t *testing.T) {
	r := openTestRuntime(t, `wraith.hooks.add("before_request",function() error("callback should not run") end)`)
	tests := []HookRequest{
		{Kind: "before_request", Purpose: "chat", Request: &RequestData{Model: "local", Messages: []HookMessage{{Role: "user", Content: string([]byte{255})}}}},
		{Kind: "before_request", Purpose: "chat", Request: &RequestData{Model: "local", Messages: []HookMessage{{Role: "user", Reference: "protected", Content: "forged"}}}},
		{Kind: "before_request", Purpose: "chat", Request: &RequestData{Model: "local", Messages: []HookMessage{{Role: "user", Content: strings.Repeat("x", 1<<20)}}}},
	}
	for _, input := range tests {
		if _, err := r.Transform(t.Context(), input); err == nil || strings.Contains(err.Error(), "callback should not run") {
			t.Fatalf("invalid envelope reached callback: %v", err)
		}
	}
}

func TestEachHookPayloadIsValidatedBeforeTheNextHook(t *testing.T) {
	for name, mutation := range map[string]string{
		"thinking": `ctx.request.think={unsupported=true}`,
		"options":  `ctx.request.options.num_ctx=-1`,
	} {
		t.Run(name, func(t *testing.T) {
			r := openTestRuntime(t, `wraith.hooks.add("before_request",function(ctx) `+mutation+`; return {request=ctx.request} end)
wraith.hooks.add("before_request",function() error("invalid data reached next hook") end)`)
			input := HookRequest{Kind: "before_request", Purpose: "chat", Request: &RequestData{Model: "local", Messages: []HookMessage{{Role: "user", Content: "hello"}}}}
			if _, err := r.Transform(t.Context(), input); err == nil || strings.Contains(err.Error(), "reached next hook") {
				t.Fatalf("intermediate payload was not validated: %v", err)
			}
		})
	}
}
