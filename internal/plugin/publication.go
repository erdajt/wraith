package plugin

import (
	"encoding/json"
	"errors"
	"maps"
	"slices"

	lua "github.com/yuin/gopher-lua"
)

type publicationDecision struct {
	revision uint64
	accepted bool
	done     chan struct{}
}

type registryState struct {
	manifest        Manifest
	functions       map[string]*lua.LFunction
	events          map[string][]string
	actions, loaded map[string]bool
	cleanup         []*lua.LFunction
}

// cloneManifest detaches nested configuration and registration values across runtime ownership.
func cloneManifest(m Manifest) Manifest {
	data, _ := json.Marshal(m)
	var copy Manifest
	_ = json.Unmarshal(data, &copy)
	return copy
}

// captureRegistry preserves the committed host surface before evaluating a callback.
func (e *engine) captureRegistry() registryState {
	events := make(map[string][]string, len(e.events))
	for name, ids := range e.events {
		events[name] = slices.Clone(ids)
	}
	return registryState{cloneManifest(e.manifest), maps.Clone(e.functions), events, maps.Clone(e.actions), maps.Clone(e.loaded), slices.Clone(e.cleanup)}
}

// restoreRegistry drops staged registrations without pretending to roll back arbitrary Lua state.
func (e *engine) restoreRegistry(s registryState) {
	e.manifest, e.functions, e.events = s.manifest, s.functions, s.events
	e.actions, e.loaded, e.cleanup = s.actions, s.loaded, s.cleanup
	e.rollback = nil
}

// refreshManifest rebuilds event subscriptions after a successful structural change.
func (e *engine) refreshManifest() {
	e.pruneCallbacks()
	e.manifest.Events = nil
	for event, ids := range e.events {
		if len(ids) > 0 {
			e.manifest.Events = append(e.manifest.Events, event)
		}
	}
	slices.Sort(e.manifest.Events)
}

// Acknowledge accepts or rejects a pending registry without entering the callback queue.
func (r *Runtime) Acknowledge(revision uint64, accepted bool) error {
	r.mu.Lock()
	if err := r.ctx.Err(); err != nil {
		r.mu.Unlock()
		return err
	}
	if revision == 0 || revision != r.pendingRevision {
		r.mu.Unlock()
		return errors.New("plugin publication is stale or already acknowledged")
	}
	decision := publicationDecision{revision: revision, accepted: accepted, done: make(chan struct{})}
	select {
	case r.ack <- decision:
		r.pendingRevision = 0
		r.mu.Unlock()
	case <-r.ctx.Done():
		r.mu.Unlock()
		return r.ctx.Err()
	default:
		r.mu.Unlock()
		return errors.New("plugin publication acknowledgement is pending")
	}
	select {
	case <-decision.done:
		return nil
	case <-r.ctx.Done():
		return r.ctx.Err()
	}
}

// publish waits for the host decision before dispatching another callback.
func (r *Runtime) publish(e *engine, revision uint64) bool {
	select {
	case <-r.ctx.Done():
		e.restoreRegistry(*e.rollback)
		return false
	case decision := <-r.ack:
		defer close(decision.done)
		if decision.revision != revision || !decision.accepted {
			e.restoreRegistry(*e.rollback)
			return true
		}
		e.rollback = nil
		r.mu.Lock()
		r.manifest, r.revision = cloneManifest(e.manifest), revision
		r.mu.Unlock()
		return true
	}
}

// pruneCallbacks releases functions once no registration or pending effect can invoke them.
func (e *engine) pruneCallbacks() {
	retained := map[string]bool{}
	for _, command := range e.manifest.Commands {
		retained[command.Callback] = true
	}
	for _, binding := range e.manifest.Config.Keymaps {
		retained[binding.Action] = true
	}
	for _, ids := range e.events {
		for _, id := range ids {
			retained[id] = true
		}
	}
	for _, effect := range e.effects {
		retained[effect.Name] = true
	}
	e.manifest.Callbacks = nil
	for id := range e.functions {
		if retained[id] {
			e.manifest.Callbacks = append(e.manifest.Callbacks, id)
		} else {
			delete(e.functions, id)
		}
	}
	slices.Sort(e.manifest.Callbacks)
}
