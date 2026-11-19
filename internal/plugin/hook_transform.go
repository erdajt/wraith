package plugin

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	lua "github.com/yuin/gopher-lua"
)

type hookCall struct {
	context context.Context
	input   HookRequest
	reply   chan hookReply
}

type hookReply struct {
	result HookResult
	err    error
}

// Transform applies a bounded hook chain on the VM owner without blocking the UI owner.
func (r *Runtime) Transform(parent context.Context, input HookRequest) (HookResult, error) {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	stop := context.AfterFunc(r.ctx, cancel)
	defer stop()
	if err := validateHookRequest(input); err != nil {
		return HookResult{}, err
	}
	data, err := json.Marshal(input)
	if err != nil || len(data) > 1<<20 {
		return HookResult{}, errors.New("hook request exceeds value limits")
	}
	var detached HookRequest
	if err := json.Unmarshal(data, &detached); err != nil {
		return HookResult{}, err
	}
	if err := validateHookRequest(detached); err != nil {
		return HookResult{}, err
	}
	call := hookCall{context: ctx, input: detached, reply: make(chan hookReply, 1)}
	select {
	case <-ctx.Done():
		return HookResult{}, ctx.Err()
	case r.hooks <- call:
	default:
		return HookResult{}, ErrQueueFull
	}
	select {
	case <-ctx.Done():
		return HookResult{}, ctx.Err()
	case reply := <-call.reply:
		return reply.result, reply.err
	}
}

// transform runs only pure callbacks and restores the previous VM context after the chain.
func (e *engine) transform(ctx context.Context, input HookRequest) (HookResult, error) {
	if err := ctx.Err(); err != nil {
		return HookResult{}, err
	}
	previousContext, previousOwner := e.L.Context(), e.owner
	defer func() {
		e.L.SetContext(previousContext)
		e.owner = previousOwner
		e.pureHook = false
		e.hookViolation = nil
	}()
	e.pureHook = true
	hooks := slices.Clone(e.manifest.Hooks)
	slices.SortFunc(hooks, func(a, b HookDefinition) int {
		if order := cmp.Compare(a.Priority, b.Priority); order != 0 {
			return order
		}
		return cmp.Compare(a.Sequence, b.Sequence)
	})
	result := HookResult{Request: input.Request, Command: input.Command, Decision: "allow"}
	for _, hook := range hooks {
		if hook.Kind != input.Kind || input.Kind == "before_request" && !slices.Contains(hook.Purposes, input.Purpose) {
			continue
		}
		if !e.surfaces.OwnerValid(hook.Owner, hook.Lease) {
			return HookResult{}, fmt.Errorf("hook %s owner is no longer active", hook.ID)
		}
		input.Request, input.Command = result.Request, result.Command
		next, err := e.callHook(ctx, hook, input)
		if err != nil {
			return HookResult{}, fmt.Errorf("hook %s (%s): %w", hook.ID, hook.Owner, err)
		}
		if next.Decision == "deny" {
			return next, nil
		}
		if next.Request != nil {
			result.Request = next.Request
		}
		if next.Command != nil {
			result.Command = next.Command
		}
		input.Request, input.Command = result.Request, result.Command
		if err := validateHookRequest(input); err != nil {
			return HookResult{}, fmt.Errorf("hook %s: %w", hook.ID, err)
		}
	}
	return result, ctx.Err()
}

// callHook checks one detached return inside a protected Lua call and its own execution budget.
func (e *engine) callHook(parent context.Context, hook HookDefinition, input HookRequest) (result HookResult, err error) {
	ctx, cancel := context.WithTimeout(parent, 100*time.Millisecond)
	defer cancel()
	e.L.SetContext(ctx)
	e.owner, e.hookViolation = hook.Owner, nil
	encoded, err := json.Marshal(input)
	if err != nil {
		return result, err
	}
	var data any
	if err := json.Unmarshal(encoded, &data); err != nil {
		return result, err
	}
	fn := e.functions[hook.Callback]
	if fn == nil {
		return result, errors.New("hook callback no longer exists")
	}
	top := e.L.GetTop()
	defer e.L.SetTop(top)
	if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, encodeValue(e.L, data)); err != nil {
		return result, err
	}
	if e.hookViolation != nil {
		return result, e.hookViolation
	}
	value := e.L.Get(-1)
	if value == lua.LNil {
		return result, ctx.Err()
	}
	decoder := e.L.NewFunction(func(L *lua.LState) int {
		decodeTypedValue(L, value, &result)
		return 0
	})
	if err := e.L.CallByParam(lua.P{Fn: decoder, NRet: 0, Protect: true}); err != nil {
		return result, err
	}
	if result.Decision != "" && result.Decision != "allow" && result.Decision != "deny" {
		return result, errors.New("hook decision must be allow or deny")
	}
	if len(result.Reason) > 4096 {
		return result, errors.New("hook reason exceeds 4096 bytes")
	}
	if input.Kind == "before_request" && result.Command != nil || input.Kind == "before_command" && result.Request != nil {
		return result, errors.New("hook returned the wrong payload kind")
	}
	return result, ctx.Err()
}

// validateHookRequest checks typed envelope invariants before and after every transformation.
func validateHookRequest(input HookRequest) error {
	text := []string{input.Kind, input.Phase, input.Purpose, input.ID, input.SessionID, input.Directory}
	if input.Kind == "before_request" {
		if input.Request == nil || input.Command != nil || !slices.Contains([]string{"chat", "extract", "audit"}, input.Purpose) {
			return errors.New("invalid request hook envelope")
		}
		if strings.TrimSpace(input.Request.Model) == "" || len(input.Request.Messages) > 4096 {
			return errors.New("invalid transformed request model or messages")
		}
		options := input.Request.Options
		if options.Context < 0 || options.Predict < 0 || options.Threads < 0 || math.IsNaN(options.Temperature) || math.IsInf(options.Temperature, 0) || options.Temperature < 0 || options.Temperature > 2 {
			return errors.New("invalid transformed model options")
		}
		switch input.Request.Think.(type) {
		case nil, bool, string:
		default:
			return errors.New("hook thinking must be a boolean or string")
		}
		text = append(text, input.Request.Model)
		if think, ok := input.Request.Think.(string); ok {
			text = append(text, think)
		}
		for _, message := range input.Request.Messages {
			text = append(text, message.Role, message.Content, message.Reference)
			if !slices.Contains([]string{"system", "user", "assistant"}, message.Role) || message.Reference != "" && message.Content != "" {
				return errors.New("invalid hook message role or protected content")
			}
		}
	} else if input.Kind == "before_command" {
		if input.Command == nil || input.Request != nil || !slices.Contains([]string{"preview", "execute"}, input.Phase) {
			return errors.New("invalid command hook envelope")
		}
		command := input.Command
		text = append(text, command.Command, command.Directory, command.Shell)
		text = append(text, command.Environment...)
		timeout, err := time.ParseDuration(command.Timeout)
		if err != nil {
			return errors.New("invalid command timeout")
		}
		if command.Command == "" || command.Shell == "" || command.Directory == "" || timeout <= 0 || len(command.Environment) > 4096 {
			return errors.New("invalid transformed command tuple")
		}
	} else {
		return errors.New("unknown hook kind")
	}
	for _, value := range text {
		if !utf8.ValidString(value) {
			return errors.New("hook input must contain valid utf8")
		}
	}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > 1<<20 {
		return errors.New("hook request exceeds value limits")
	}
	return nil
}
