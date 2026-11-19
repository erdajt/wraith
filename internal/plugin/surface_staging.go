package plugin

import (
	lua "github.com/yuin/gopher-lua"
	"slices"
)

// previewSurfaces fixes observed revisions until the current callback has finished.
func (e *engine) previewSurfaces() *SurfaceStore {
	if e.surfacePreview == nil {
		e.surfacePreview = e.surfaces.snapshot()
	}
	return e.surfacePreview
}

// reserveSurface keeps a creation invisible until the host accepts its ordered effect.
func (e *engine) reserveSurface(kind string) string {
	lease := e.surfaceLease()
	preview := e.previewSurfaces()
	id, err := e.surfaces.Reserve(e.owner, kind)
	if err != nil {
		e.L.RaiseError("surface: %v", err)
	}
	preview.owners[e.owner] = lease
	preview.reserved[id] = surfaceReservation{owner: e.owner, kind: kind}
	e.surfaceReserved = append(e.surfaceReserved, id)
	return id
}

// stageSurface validates read-your-writes before publishing a detached host mutation.
func (e *engine) stageSurface(mutation SurfaceMutation) SurfaceDelta {
	L := e.L
	mutation.Owner, mutation.Lease = e.owner, e.surfaceLease()
	preview := e.previewSurfaces()
	preview.owners[e.owner] = mutation.Lease
	if mutation.TargetOwner != "" {
		preview.owners[mutation.TargetOwner] = e.surfaces.OwnerLease(mutation.TargetOwner)
	}
	before := preview.snapshot()
	accepted := false
	defer func() {
		if !accepted {
			e.surfacePreview = before
			if mutation.Expected == 0 {
				e.surfaces.DiscardReservations([]string{mutation.ID})
				delete(before.reserved, mutation.ID)
				e.surfaceReserved = slices.DeleteFunc(e.surfaceReserved, func(id string) bool { return id == mutation.ID })
			}
		}
	}()
	delta, err := preview.Apply(mutation)
	if err != nil {
		L.RaiseError("surface: %v", err)
	}
	if e.setup {
		if len(e.manifest.Surface) >= 128 {
			L.RaiseError("startup surface mutation limit reached")
		}
		e.manifest.Surface = append(e.manifest.Surface, mutation)
	} else {
		e.emit(L, Effect{Name: "surface.apply", Argument: mutation.ID, Data: mutation})
	}
	accepted = true
	return delta
}

// surfaceRevision accepts an explicit optimistic revision or the callback's observed value.
func surfaceRevision(L *lua.LState, index int, observed uint64) uint64 {
	if L.Get(index) == lua.LNil {
		return observed
	}
	var revision uint64
	decodeSurface(L, L.Get(index), &revision)
	if revision == 0 || revision > 1<<53 {
		L.RaiseError("surface revision must be a positive exact integer")
	}
	return revision
}
