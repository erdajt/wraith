package plugin

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestSelectedManagerModulesUseOrdinaryRequireWithoutInstallingPlugins(t *testing.T) {
	r := openManagerRuntime(t, `assert(require("shroud")==require("wraith.manager"))
local planner=require("wraith.manager.store_plan")
assert(planner==require("wraith.manager.store_plan"))
assert(type(planner.create)=="function")
assert(#wraith.plugins.list()==0)
local specs={{name="local",source={kind="local",dir="/not-created"},selector={kind="head"},pin=false}}
local plan=planner.create(specs,{version=1,generation="test",packages={}},"install")
assert(plan.packages[1].path=="/not-created")
assert(package.loaded["wraith.manager.runtime"]==nil)
wraith.commands.create("verify",function() wraith.notify("available") end)`)
	if len(r.Manifest().Commands) != 1 {
		t.Fatal("package source changed user registration")
	}
}

func TestSelectedManagerSourcesCompileWithNamedErrors(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	options := managerOptions(t)
	root := os.DirFS(options.Packages[0].Directory)
	err := fs.WalkDir(root, "lua", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".lua") {
			return nil
		}
		source, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		if _, err = L.Load(strings.NewReader(string(source)), "@"+filepath.Join(options.Packages[0].Directory, path)); err != nil {
			t.Errorf("%s: %v", path, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
