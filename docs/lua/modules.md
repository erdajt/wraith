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
