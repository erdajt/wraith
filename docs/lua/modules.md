# module discovery

[plugin guide](../plugins.md) · [packages](packages.md) · [editor types](types/wraith.lua)

`wraith.modules.discover(name)` finds a module and its immediate children without evaluating them. package managers and configuration loaders can inspect sources before loading them:

```lua
for _, source in ipairs(wraith.modules.discover('plugins')) do
  print(source.name, source.kind, source.path or 'preload')
  local specs = source.load()
end
```

records contain `name`, `kind`, `identity`, `load` and a `path` for file sources. `identity` is opaque and stable for the bound source within the runtime. `load()` evaluates that source afresh and preserves every return value, including trailing nil. it does not read or populate `package.loaded`. changing record fields cannot redirect its loader.

## selection

an explicit `package.preload` override takes precedence. otherwise, a selected package namespace supplies only its indexed files. local configuration cannot fill a missing file in that namespace, including when discovery starts at a parent namespace. explicit child preload overrides remain available.

ordinary modules use the first matching runtime root, searching `lua/` before the root's legacy layout. register roots with `wraith.runtimepath.prepend` or `append`. discovery does not consult the working directory, `LUA_PATH` or manual additions to `package.path`; ordinary `require` keeps its existing behavior.

the selected base contributes its module file, directory `init.lua`, immediate `.lua` files and immediate child directories with `init.lua`. results sort by module name. deeper files require another explicit discovery call. two paths producing the same module name fail before any source is evaluated. an empty matching directory does not fall through to a later root.

## limits and failures

names use dotted identifiers and are limited to 128 bytes. discovery checks at most 128 runtime roots, 1024 immediate directory entries and 256 source records. exceeding a limit raises an error rather than returning a partial list.

explicit roots may be symlinks. discovered child components cannot be symlinks. file loads require regular, text-only sources within 1 mib and observe the current callback deadline. errors name the source. captured loaders remain unavailable inside pure hooks.

a loader binds a selected path or preload function. editable files are not immutable snapshots; changing or removing a file can change or fail a later load. this API does not sandbox trusted plugin code.

## explicit plugin roots

`wraith.modules.catalog(root)` lists immediate Lua entry modules inside an explicit root's `lua/` directory. `wraith.modules.source(root, name)` resolves one exact module there, then checks the legacy root layout if neither exact source exists. both return bound file records without evaluating code:

```lua
local root = wraith.paths.stdpath('config') .. '/packages/review'
for _, source in ipairs(wraith.modules.catalog(root)) do
  print(source.name, source.path)
end
local source = wraith.modules.source(root, 'review')
if source then
  local module = source.load()
end
```

roots must be existing absolute directories, at most 4096 bytes, without nul, semicolons or question marks. root symlinks are followed; child symlinks are rejected. these operations do not consult runtime paths, selected package namespaces, preloads or require caches. check `wraith.capabilities.module_sources` before using them on older hosts.

catalog includes immediate `.lua` files and immediate directories containing `init.lua`. `lua/init.lua` is the module named `init`. directories without an entry module are ignored. a missing `lua/` directory yields an empty list; other filesystem errors fail. automatic catalogs exclude legacy root files and native modules.

exact lookup converts dots to directory separators. it checks `lua/name.lua` and `lua/name/init.lua`, then `name.lua` and `name/init.lua`. two matching forms in one layout are ambiguous. an empty Lua directory does not suppress an exact legacy match. absence returns `nil`; invalid types, symlinks and I/O failures do not fall through.

catalog records retain their physical paths. a physical `lua/tool.extra.lua` record loads that file directly, while `source(root, 'tool.extra')` looks for `lua/tool/extra.lua` or `lua/tool/extra/init.lua`. use the catalog record's loader instead of reconstructing its path.

both APIs preserve the discovery limits and bound-loader behavior above. they do not install preload wrappers or change require caching. package managers must define their own entry-point ownership and cache policy.

## require state

`wraith.modules.loading(name)` reports whether native `require` currently holds its loading marker for that name. it reads the runtime registry, without executing loaders or exposing the private marker. check `wraith.capabilities.module_loading` on older hosts. names follow the discovery grammar and length limit.

the predicate is true inside a loader entered by `require`, before that loader replaces its cache entry. a direct preload call on an uncached name returns false. replacing the public `package.loaded` table does not replace the native registry. this is a cache-state query, not proof that arbitrary trusted Lua code is safe.

failed native loads clear only their unchanged loading marker, allowing retries. values written by the source remain intact, including false and userdata. successful nil returns keep normal `require` caching. package managers must still distinguish failed source evaluation from failed setup and reject cache values they do not own.
