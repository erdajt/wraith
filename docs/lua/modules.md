# modules

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

ordinary modules use the first matching runtime root, searching `lua/` before the root's legacy layout. register roots with `wraith.runtimepath.prepend` or `append`. discovery does not consult the working directory, `LUA_PATH` or manual additions to `package.path`. [module providers](#module-providers) can route `require` separately.

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

## module providers

`wraith.modules.provider(resolve)` registers a resolver and returns an opaque id. `wraith.modules.remove_provider(id)` removes a live registration owned by the caller. check `wraith.capabilities.module_providers` on older hosts. this is a host API for package loaders; the bundled manager does not yet provide require-triggered activation.

resolvers run in registration order. `resolve(name)` returns `nil` for a miss or a plain table with exactly these fields:

| field | contract |
| --- | --- |
| `plugin` | lifecycle module name loaded through `wraith.plugins.load` |
| `activate` | `function(name)` loads the selected plugin; return values are ignored |
| `check` | `function(name, phase, proof, executed)` verifies source and cache ownership |

false is not a miss. extra fields, metatables, invalid names and nonfunction callbacks fail. the host captures the descriptor's values; changing the returned table cannot redirect that request. the first matching resolver wins. resolver callbacks inspect sources and choose a plugin without activating it.

### verification and activation

`check` has three phases:

| phase | arguments and result |
| --- | --- |
| `validate` | `proof` is nil and `executed` is false; verify ownership without loading code; return value is ignored |
| `before` | `proof` is nil; `executed` reports whether the native cache is nil or false; return an opaque per-call proof token |
| `after` | receives the exact token and execution flag from `before`; verify the result and any source execution; return value is ignored |

errors reject the request. checks run as metadata callbacks. the host does not serialize or interpret proof tokens. a loader must bind each module to its selected source, reject foreign caches and distinguish a fresh execution from a cached return. a no-op check provides no source verification. a source that returns nil needs execution proof too: a different loader returning nil must not pass merely because its result looks familiar.

validation runs before activation. when needed, `activate(name)` runs under the resolver owner and must leave the named plugin loaded. the host then requires the source under that plugin's owner. its registrations and managed work belong to the provider plugin. a plugin importing another managed module gains a dependency on its provider, including on cache hits. configuration imports do not create a synthetic dependent that prevents unloading.

setup may require its own module while loading. incomplete imports through a different provider are rejected. nested imports keep each caller's owner; ordinary coroutine calls use the current coroutine state. callbacks share the current host deadline and cannot yield across protected native calls.

### metadata and hooks

resolver and check callbacks may use JSON helpers, module `catalog`, `source` and `loading` queries, synchronous `fs.stat` and `plugins.list`. they cannot activate plugins, execute bound source loaders, print, emit events, start asynchronous work or mutate host registrations. arbitrary Lua state and external effects remain outside host rollback.

metadata may require an unchanged, truthy, never-managed cached helper. it cannot run a resolver or source. pure hooks follow the same cached-helper rule when providers or prior managed claims exist. import dependencies before registering a pure callback and capture their exports. caught prohibited calls still fail the enclosing metadata or pure hook. registration and removal are unavailable in these scopes and during teardown.

### lifecycle and failures

registrations follow the [plugin lifecycle](../plugins.md#events-and-plugin-lifecycle). startup registration belongs to its configuration or plugin owner. runtime changes publish with the callback's registry transaction. owner unload stages provider removal immediately; rejected publication restores the previous providers, ids and order.

managed source failures undo staged host changes and dispose newly owned resources. verified cache entries retire after cleanup on accepted unload. rejected unload retains the previous cache and lifecycle. unverified values written by failed sources are preserved and poison that module; subsequent managed imports require `/reload`. source or setup failure can retry when no unverified cache remains. removing a resolver does not erase prior claims or poison. a previously managed name with no resolver fails rather than falling through to a retained source.

teardown can read only truthy cached exports. managed exports need a matching execution receipt; never-managed cached exports remain readable. nil, false, loading markers, replaced cache tables and cache metatables are rejected. teardown never resolves, activates or executes a source. whole-runtime reload starts with fresh providers, claims and native loading state.

never-managed misses use native `require`, preserving preload/path selection, error objects, source tracebacks and nil/false cache semantics. names outside the managed grammar also use native lookup. replacing global `require` bypasses routing and is unsupported for managed imports. Lua is trusted code, not a security sandbox; plugins can still use raw loaders and OS libraries.

| routing limit | value |
| --- | --- |
| provider registrations | 32 |
| managed module name | dotted identifiers, at most 128 bytes |
| managed names claimed during one runtime | 1024; removal does not free a name |
| nested requests, metadata calls or source invocations | 32 each |

limits fail before further activation or source execution. no provider API fetches packages or searches for installations.
