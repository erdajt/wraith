package plugin

import (
	"maps"
	"slices"
)

type surfaceState struct {
	surfaceOwners       map[string]string
	surfaceReleased     []string
	surfaceReserved     []string
	surfacePreview      *SurfaceStore
	surfaceStartupBytes int
}

// surfaceLease validates the current package before issuing an explicit UI lifetime receipt.
func (e *engine) surfaceLease() string {
	if e.closing {
		e.L.RaiseError("surface requests are disabled during teardown")
	}
	active := e.owner == "config" || e.owner == "core"
	for _, record := range e.packages {
		active = active || record.owner == e.owner
	}
	if !active {
		e.L.RaiseError("surface owner is no longer loaded")
	}
	if lease := e.surfaceOwners[e.owner]; lease != "" {
		return lease
	}
	lease, err := e.surfaces.registerOwner(e.owner)
	if err != nil {
		e.L.RaiseError("%v", err)
	}
	e.surfaceOwners[e.owner] = lease
	return lease
}

// cloneSurfaces preserves private reservations and owner receipts across nested callbacks.
func (e *engine) cloneSurfaces() surfaceState {
	copy := surfaceState{surfaceOwners: maps.Clone(e.surfaceOwners), surfaceReleased: slices.Clone(e.surfaceReleased), surfaceReserved: slices.Clone(e.surfaceReserved), surfaceStartupBytes: e.surfaceStartupBytes}
	if e.surfacePreview != nil {
		copy.surfacePreview = e.surfacePreview.snapshot()
	}
	return copy
}

// restoreSurfaces invalidates abandoned receipts before restoring the last accepted owner state.
func (e *engine) restoreSurfaces(previous surfaceState) {
	for owner := range e.surfaceOwners {
		if _, kept := previous.surfaceOwners[owner]; !kept {
			e.surfaces.RevokeOwner(owner)
		}
	}
	for _, owner := range e.surfaceReleased {
		if _, kept := previous.surfaceOwners[owner]; !kept && !slices.Contains(previous.surfaceReleased, owner) {
			e.surfaces.RevokeOwner(owner)
		}
	}
	var abandoned []string
	for _, id := range e.surfaceReserved {
		if !slices.Contains(previous.surfaceReserved, id) {
			abandoned = append(abandoned, id)
		}
	}
	e.surfaces.DiscardReservations(abandoned)
	e.surfaceState = previous
}

// Surfaces exposes the runtime's Go-only display store to the UI owner.
func (r *Runtime) Surfaces() *SurfaceStore { return r.surfaces }

// SurfaceValid checks an explicit UI effect without depending on input focus or draft state.
func (r *Runtime) SurfaceValid(mutation SurfaceMutation) bool { return r.surfaces.Valid(mutation) }

// CommitStartup releases retained bootstrap text after the host installs the candidate layout.
func (r *Runtime) CommitStartup() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.manifest.Surface = nil
	if !r.startupInstalled {
		r.startupInstalled = true
		r.startupReady <- struct{}{}
	}
}
