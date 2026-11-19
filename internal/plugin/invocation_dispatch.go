package plugin

import (
	"bytes"
	"context"
	"errors"

	"github.com/erdajt/wraith/internal/config"
)

// BeginInvocation binds new work to the host's accepted provider or tool definition.
func (r *Runtime) BeginInvocation(ctx context.Context, request InvocationRequest) (*Invocation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	binding := request.Binding
	matched := false
	if binding.Kind == "provider" {
		for _, provider := range r.manifest.Providers {
			callback := map[string]string{"models": provider.Models, "show": provider.Show, "chat": provider.Chat}[binding.Method]
			matched = matched || callback != "" && binding == (InvocationBinding{"provider", provider.Name, binding.Method, provider.Owner, provider.Lease, callback, provider.Revision})
		}
	} else if binding.Kind == "tool" && binding.Method == "run" {
		for _, tool := range r.manifest.Tools {
			matched = matched || binding == (InvocationBinding{"tool", tool.Name, "run", tool.Owner, tool.Lease, tool.Callback, tool.Revision})
		}
	}
	if !matched || r.ctx.Err() != nil || !r.surfaces.OwnerValid(binding.Owner, binding.Lease) {
		return nil, errors.New("invocation binding is stale or unavailable")
	}
	return r.invocations.begin(ctx, request)
}

// InvocationStarts exposes detached dispatch notices without entering the Lua actor.
func (r *Runtime) InvocationStarts() <-chan InvocationStart { return r.invocations.starts }

// InvocationValid rejects effects after completion, cancellation or owner retirement.
func (r *Runtime) InvocationValid(id string) bool { return r.invocations.valid(id) }

// SubmitInvocation claims a start once and supplies the current host workspace state.
func (r *Runtime) SubmitInvocation(start InvocationStart, snapshot any, c config.Config) (<-chan Result, error) {
	claimed, err := r.invocations.claim(start.ID)
	if err != nil {
		return nil, err
	}
	reply, err := r.submit(claimed.Binding.Callback, "", snapshot, c, nil, &claimed)
	if err != nil {
		r.invocations.mu.Lock()
		if record := r.invocations.records[start.ID]; record != nil {
			record.started = false
		}
		r.invocations.mu.Unlock()
	}
	return reply, err
}

// CommitInvocation delivers an ordered reply after preceding host effects succeed.
func (r *Runtime) CommitInvocation(effect InvocationEffect) error {
	return r.invocations.commit(effect)
}

// AbortInvocation settles a failed host chain and cancels its managed children.
func (r *Runtime) AbortInvocation(id string, err error) {
	if err == nil {
		err = context.Canceled
	}
	r.invocations.finish(id, nil, err)
}

// claim uses retained input rather than trusting a mutable dispatch notification.
func (m *invocationManager) claim(id string) (InvocationStart, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record := m.records[id]
	if m.closed || record == nil || record.started || record.terminal != nil || record.context.Err() != nil {
		return InvocationStart{}, errors.New("invocation is canceled or already dispatched")
	}
	record.started = true
	return InvocationStart{ID: id, Binding: record.binding, Data: bytes.Clone(record.data)}, nil
}

// references retains frozen callbacks while their accepted invocations can still start.
func (m *invocationManager) references() map[string]bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	retained := map[string]bool{}
	for _, record := range m.records {
		if record.terminal == nil && record.context.Err() == nil {
			retained[record.binding.Callback] = true
		}
	}
	return retained
}

// revokeOwner ends bound work before arbitrary package teardown can block.
func (m *invocationManager) revokeOwner(owner string) {
	m.mu.Lock()
	var ids []string
	for id, record := range m.records {
		if record.binding.Owner == owner {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.finish(id, nil, errors.New("invocation owner was unloaded"))
	}
}
