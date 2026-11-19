package plugin

import (
	"strings"
	"unicode"

	lua "github.com/yuin/gopher-lua"
)

// installRegions stages cached presentation frames and exposes explicit byte-to-text conversion.
func (e *engine) installRegions(api *lua.LTable) {
	L := e.L
	methods := L.NewTable()
	methods.RawSetString("create", L.NewFunction(func(L *lua.LState) int {
		var region SurfaceRegion
		decodeTypedValue(L, L.CheckTable(1), &region)
		id := e.reserveSurface("region")
		e.stageSurface(SurfaceMutation{Operation: "region.create", ID: id, Region: &region})
		L.Push(lua.LString(id))
		return 1
	}))
	methods.RawSetString("list", L.NewFunction(func(L *lua.LState) int {
		return pushSurface(L, e.previewSurfaces().Index().Regions)
	}))
	for _, kind := range []string{"get", "set", "delete"} {
		methods.RawSetString(kind, L.NewFunction(func(L *lua.LState) int {
			id := L.CheckString(1)
			region, err := e.previewSurfaces().Region(id)
			if err != nil {
				L.RaiseError("regions.%s: %v", kind, err)
			}
			if kind == "get" {
				return pushSurface(L, region)
			}
			mutation := SurfaceMutation{Operation: "region." + kind, ID: id, Expected: region.Revision}
			if kind == "set" {
				decodeTypedValue(L, L.CheckTable(2), &region)
				mutation.Region = &region
				mutation.Expected = surfaceRevision(L, 3, mutation.Expected)
			} else {
				mutation.Expected = surfaceRevision(L, 2, mutation.Expected)
			}
			delta := e.stageSurface(mutation)
			if delta.Region != nil {
				L.Push(lua.LNumber(delta.Region.Revision))
				return 1
			}
			return 0
		}))
	}
	api.RawSetString("regions", methods)
	ui := api.RawGetString("ui").(*lua.LTable)
	ui.RawSetString("decode", L.NewFunction(func(L *lua.LState) int {
		text := L.CheckString(1)
		if len(text) > 4<<20 {
			L.RaiseError("ui.decode input exceeds 4 mib")
		}
		L.Push(lua.LString(strings.Map(func(r rune) rune {
			if unicode.IsControl(r) && r != '\n' && r != '\t' {
				return -1
			}
			return r
		}, text)))
		return 1
	}))
}
