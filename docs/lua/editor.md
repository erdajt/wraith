# lua editor support

wraith ships [luals declarations](types/wraith.lua) for the host api and the bundled [package manager](types/wraith/manager.lua). add the declarations to your language server's library for completion, hover documentation, callback types and argument checks.

## setup

keep a checkout matching your wraith version. create `.luarc.json` in your wraith config directory or plugin project:

```json
{
  "runtime.version": "Lua 5.1",
  "runtime.path": ["lua/?.lua", "lua/?/init.lua", "?.lua", "?/init.lua"],
  "workspace.library": [
    "/absolute/path/to/wraith/docs/lua/types"
  ],
  "workspace.checkThirdParty": false
}
```

replace the library path with your checkout's absolute path. add the whole `types` directory, including its `wraith` subdirectory. restart the lua language server after changing its library.

in neovim, use your existing `lua_ls` setup. if mason already installs `lua-language-server`, another installer or completion plugin is unnecessary. open the config directory as a workspace so luals finds its `.luarc.json`. keep these settings local to wraith projects: neovim's own configuration uses a different runtime and api.

these files are editor declarations. do not `require` them from `init.lua`, add them to wraith's runtimepath or copy them over a package implementation. the running application provides `wraith`; it also provides `require('wraith.manager')` through its embedded loader.

## typed callbacks

direct callbacks infer their context from the api:

```lua
wraith.commands.create('review.status', function(ctx)
  wraith.notify(ctx.session.id .. ': ' .. ctx.argument)
end, 'show the current review')

wraith.ui.select({ 'working tree', 'last commit' }, {
  title = 'review scope',
}, function(ctx)
  if ctx.data and not ctx.data.cancelled then
    wraith.notify('selected ' .. assert(ctx.data.value))
  end
end)
```

for a named callback, declare its context explicitly. replace the example id with a job from your session:

```lua
---@param ctx Wraith.Context<Wraith.QueryResult<Wraith.JobOutput>>
local function show_output(ctx)
  local reply = assert(ctx.data)
  if not reply.ok then
    wraith.notify(assert(reply.error).message)
    return
  end

  for _, line in ipairs(assert(reply.result).page.lines) do
    wraith.notify(line.text)
  end
end

wraith.jobs.output({ id = '0123456789abcdef01234567', limit = 40 }, show_output)
```

`ctx.data`, `ctx.resource` and `ctx.invocation` are optional in the shared context type. their presence depends on the callback source. query replies use `ok`, `result` and a structured `error`; managed worker completions use `state`, `done`, `result` and a string `error`. check the result before reading optional fields.

package specs also have a named type:

```lua
---@type Wraith.PackageSpec
local review = {
  dir = 'plugins/review',
  main = 'review',
  cmd = 'review.open',
  opts = { title = 'review' },
}

require('wraith.manager').setup({ review })
```

see [package configuration](packages.md), [plugin lifecycle](../plugins.md), [services](services.md), [surfaces](surfaces.md), [provider and tool extensions](extensions.md) and [workspace queries](workspace.md) for runtime behavior and examples.

## declaration map

| file | contracts |
| --- | --- |
| [wraith.lua](types/wraith.lua) | api version, capabilities, mappings, commands, events, package lifecycle, paths |
| [config.lua](types/config.lua) | full configuration, partial updates, providers, themes |
| [context.lua](types/context.lua) | callback snapshots, native windows, input and prompt adapters |
| [services.lua](types/services.lua) | json, filesystem, http, timers, processes, managed handles |
| [surfaces.lua](types/surfaces.lua) | buffers, spans, windows, regions, dialogs |
| [extensions.lua](types/extensions.lua) | request and command hooks, provider transports, tool schemas, invocations |
| [queries.lua](types/queries.lua) | session history, jobs, output pages, terminals, environment inventory |
| [wraith/manager.lua](types/wraith/manager.lua) | specs, storage and view options, status, plans, install/update/restore |

## runtime boundaries

wraith uses lua 5.1 semantics through its go runtime. luajit, ffi, native c modules and neovim's `vim` api are unavailable. types describe wraith's public api; they do not make neovim plugins compatible.

invalid host arguments raise lua errors. synchronous functions return their documented value or raise; they do not return an invented `nil, error` pair. asynchronous apis reserve work, return a handle and report completion through their callback. a completed handle remains queryable until release or owner disposal.

luals checks shapes and known types. runtime checks still enforce owner lifetimes, revisions, payload limits, schema complexity, command approval and callback budgets. editor declarations do not sandbox trusted plugins. read the [trust model](../plugins.md#runtime-boundaries) before loading third-party code.

the declarations intentionally leave arbitrary package options and custom event payloads extensible. annotate your own data classes when building a plugin. provider and tool payloads still pass through their documented validation and privacy boundaries.

## validate changes

the runtime export test checks all host and public manager functions, capabilities and json null against these declarations:

```sh
go test ./internal/plugin -run '^TestLuaAnnotationsMatchRuntime$'
```

the diagnostics test runs the real language server against valid and invalid api examples. it uses `lua-language-server` from `PATH`, or an explicit executable:

```sh
WRAITH_LUALS=/absolute/path/to/lua-language-server \
  go test ./internal/plugin -run '^TestLuaAnnotationDiagnostics$'
```

without a server, only the diagnostics test skips. the runtime export test still runs. update declarations and diagnostic fixtures together when changing a public api.
