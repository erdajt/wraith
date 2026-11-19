# lua configuration and plugins

wraith embeds gopher-lua and exposes API version 2. `init.lua` is real lua 5.1-style code, with functions, loops, tables, closures and local modules. luaJIT and neovim's `vim` module are not included.

[service APIs](lua/services.md) cover JSON, filesystem operations, HTTP, timers and managed resource lifetimes. [surface APIs](lua/surfaces.md) cover shared buffers, floating and tiled views, presentation regions and modal dialogs. [workspace APIs](lua/workspace.md) page conversations, jobs and local output. [package manager](lua/packages.md) installs locked git revisions and loads modular local configurations. [editor completion](lua/editor.md) documents the LuaLS definitions. [example packages](lua/examples.md) compose these APIs into task selectors, history, Git views and native command policies.

## start

configuration loads in this order:

1. built-in defaults
2. `~/.config/wraith/config.yaml`
3. `~/.config/wraith/init.lua`
4. command-line provider, model and theme overrides

resuming a session restores its provider and model unless the corresponding CLI override is supplied. `$XDG_CONFIG_HOME` and `--config-dir` change the configuration root. demo and snapshot modes do not load plugins.

`wraith.paths.stdpath("data")` and `stdpath("cache")` follow `--data-dir` and `--cache-dir`, including after `/reload`. without overrides they use the XDG locations.

```lua
assert(wraith.api_version == 2)

wraith.config.setup({
  theme = "rose-pine",
  appearance = { background = "inherit" },
  key_timeout = 500,
})

for _, mode in ipairs({ "chat.normal", "chat.visual" }) do
  wraith.keymap.set(mode, "<C-h>", "window.left")
  wraith.keymap.set(mode, "<C-j>", "window.down")
  wraith.keymap.set(mode, "<C-k>", "window.up")
  wraith.keymap.set(mode, "<C-l>", "window.right")
end
```

`ctrl+w s` and `ctrl+w v` already split the conversation horizontally and vertically. every view keeps its own cursor, scroll, search and selection in the same conversation.

copy the [complete example](examples/lua/init.lua) to your configuration root and [its plugin](examples/lua/plugins/review.lua) to `plugins/review.lua` below that root. `/review.draft this package` prepares a prompt. `/review.layout` splits the view. the example is loaded and exercised by the unit tests.

## reload and persistence

`/reload` reads YAML and Lua into a fresh candidate. syntax errors, invalid settings, conflicting bindings and unknown actions leave the current runtime active. errors include the Lua source and line when supplied by the interpreter. a startup failure falls back to YAML and displays the error.

successful reload installs the candidate, clears pending key sequences, cancels old callbacks and closes the old VM. commands, mappings and subscriptions are replaced. a reload is discarded if focus, draft or settings change while it loads. finish a model operation or dialog before reloading.

`/set` and settings pickers save explicit changes to YAML. they do not copy unrelated Lua-only values, callback ids or credentials into it. the config editor opens the YAML source. settings marked `init.lua on reload` will be overridden again when Lua next loads. change those values in `init.lua` for a lasting result. a setting that depends on a Lua-only provider or palette may require editing Lua instead of saving an incomplete YAML configuration.

runtime `wraith.config` updates are temporary. `/reload` restores the saved YAML and Lua layers. it does not preserve one-time CLI overrides.

## settings and themes

| API | behavior |
| --- | --- |
| `wraith.config.get()` | detached table containing effective settings |
| `wraith.config.get("appearance.background")` | one existing setting |
| `wraith.config.set(path, value)` | replace an existing setting after validation |
| `wraith.config.setup(table)` | merge named settings atomically; preserve omitted object fields |
| `wraith.theme.register(name, palette)` | register a custom palette |
| `wraith.theme.use(name)` | select a registered or built-in palette |
| `wraith.getenv(name)` | read a process environment variable; return `nil` when absent |

arrays replace their previous values; `{}` clears an existing array. unknown fields, invalid colors and invalid numeric values fail validation. `config.setup` can add profiles and palettes through their containing maps. `config.set` requires an existing path.

```lua
wraith.config.setup({ clipboard = {} })
wraith.config.set("providers.openai.api_key", wraith.getenv("OPENAI_API_KEY") or "")
wraith.theme.register("night", {
  bg = "#151b22", fg = "#d8dee9", muted = "#8692a6",
  accent = "#8fbcbb", second = "#b48ead", border = "#394454",
  warn = "#ebcb8b",
})
wraith.theme.use("night")
```

optional palette fields are `variable`, `hint_quit`, `hint_refresh` and `hint_cancel`. colors use `#rrggbb`. [theme settings](themes.md) cover inherited backgrounds. fonts and numerical opacity remain properties of the host terminal.

## mappings and commands

```lua
wraith.keymap.set("chat.normal", "<Space>sv", "window.split_vertical")
wraith.keymap.set("prompt.normal", "<Space>p", function(ctx)
  wraith.prompt.set("review " .. ctx.session.directory)
end, "prepare a directory review")

wraith.commands.create("review.note", function(ctx)
  wraith.prompt.set("review " .. ctx.argument)
end, "prepare a review prompt")
```

`keymap.set(mode, sequence, action_or_function_or_keys, description)` registers an override. descriptions are optional. use `/keymaps` to inspect active mappings. [keybindings](keybindings.md#mode-specific-mappings) lists notation and modes. numeric prefixes retain the current normal/visual mappings. pending operators and literal arguments have separate modes.

`keymap.parse(sequence)` returns canonical key tokens without registering a binding. `keymap.validate(mode, sequence)` returns true for a valid mode and sequence, otherwise raises an error. validation checks mode and notation, not action availability or host-specific binding conflicts. these APIs let a plugin validate configurable bindings before changing the registry.

`keymap.del(mode, sequence)` hides configured overrides for that sequence; built-in defaults become available again. the deletion belongs to the calling configuration or plugin, so user configuration can disable a mapping installed by a plugin. map to `noop` to consume a default key. mappings and commands can be registered during setup or inside callbacks. runtime registrations are validated and installed before their callback effects run. rejected registrations leave the previous host registry active.

`commands.create(name, callback, description)` registers a slash command and a `command.NAME` action. command names use lowercase letters, digits and hyphens, with optional dotted children. built-in names and namespaces are reserved unless an existing command is explicitly replaced with `{replace = true}`. a trailing `!` is accepted. `commands.delete(name)` hides a custom command and restores any builtin it replaced. command definitions and deletions belong to their configuration or plugin owner. custom commands appear in slash completion and `/help`. a virtual parent expands to its dotted children when selected.

`wraith.action(name, argument)` queues a named operation from a callback. arguments are optional strings. host actions include `prompt.send`, `editor.open`, `focus.next`, `terminal.overlay`, `terminal.side`, `operation.cancel`, `app.quit`, `noop`, all `command.NAME` actions, and the `window.*` actions listed in [keybindings](keybindings.md). native `input.feed` edits the focused buffer; action calls never synthesize live shell input.

bindings resolve once. an exception consumes the mapping and reports the error. ambiguous sequences use `key_timeout`; escape cancels the prefix. failed prefixes replay once into the same input owner. direct `ctrl+c` and `ctrl+q` remain available.

## workspace snapshots and effects

| API | behavior in a callback |
| --- | --- |
| `wraith.session.get()` | copy of session id, model, provider, directory and bounded messages |
| `wraith.windows.list()` | copies of window ids, focus, cursor, column, search, offset and visibility; visible windows include width and height |
| `wraith.windows.split("horizontal")` | queue an above/below split; `vertical` places panes side by side |
| `wraith.windows.focus(id)` | queue focus by stable window id |
| `wraith.windows.close(id)` | close that view; omitted id means the focused view |
| `wraith.prompt.get()` | draft captured when the callback was queued |
| `wraith.prompt.set(text)` | queue replacement of the shared draft |
| `wraith.input.get()` | detached mode, cursor, selection, search and pending-key snapshot |
| `wraith.input.feed(keys)` | queue a nonrecursive native editing sequence |
| `wraith.input.set_mode(mode)` | choose normal, insert, visual, visual-line or search |
| `wraith.notify(text)` | show a notice |

each callback receives `ctx` with `argument`, `event`, `data`, `prompt`, `input`, `windows` and `session`. `data` contains a detached payload for custom events. session messages contain role and content, limited to the latest 64 messages and 64 kib of content. `message_count` records the full count. a large message may be truncated. modifying snapshot tables does not change the app.

operations run after the callback returns successfully. snapshot reads within that callback continue to describe its original state. split operations return no new id immediately; inspect a later window event to find it. an exception or timeout discards the callback's effect batch.

effects retain order. a nested custom command finishes its effects before its caller resumes. an empty callback also returns control to its caller; a failed or stale callback stops the chain. editor actions hold the chain until their own saved edit is applied; closing without saving, editor errors and workspace changes stop it. workspace ownership is checked before dispatch. approval dialogs, editors and raw shell input retain control of their surfaces. a later effect can be refused if an earlier operation opened a dialog. plugin effects cannot approve a proposed model command. model requests and approved jobs retain the normal privacy, context and execution paths.

implicit workspace effects are discarded after runtime replacement or changes to the captured focus, selection, navigation, mode, draft or settings. [explicit surface effects](lua/surfaces.md#lifecycle-errors-and-limits) instead check their owner, target id and revision, so unrelated typing does not discard a background buffer update. ids from closed windows are rejected. nested custom-command callbacks stop after eight levels. queued host actions follow these guards even from event hooks. trusted code can execute programs directly through Lua libraries; those operations are outside model-command approval.

## events and plugin lifecycle

```lua
local id = wraith.events.on("window_changed", function(ctx)
  wraith.notify(tostring(#ctx.windows) .. " conversation views")
end)

return function()
  wraith.events.off(id)
end
```

| event | trigger |
| --- | --- |
| `ready` | initial workspace start or successful reload |
| `session_changed` | the active session id changes |
| `window_changed` | view focus or visible window geometry changes |
| `buffer_changed` | canonical surface text changes |
| `jobs_changed` | bounded native job metadata changes |
| `terminals_changed` | scratch terminal lifecycle or dimensions change |
| `environment_changed` | variable names, redacted comments or scope change |
| `response_finished` | an active model response stops; compaction is excluded |

`events.on(name, function)` registers during setup or a callback and returns an id. `events.off(id)` removes the subscription, including from a callback or teardown. `window_changed`, `session_changed` and `buffer_changed` coalesce to the newest queued snapshot. native workspace service events retain their latest pending resync state under queue pressure; see [lifecycle events](lua/workspace.md#lifecycle-events). completion and custom events retain their order. superseded state-event callers are released without effects.

custom events use dotted lowercase names, such as `review.selected`. `wraith.events.emit("review.selected", {id = "task-1"})` queues delivery after the emitting callback succeeds. listeners read the payload from `ctx.data`. a callback can register a listener and emit its event in the same batch; host registration finishes before delivery. emission cannot impersonate built-in lifecycle events. each observer has its own deadline and registration transaction. a failing observer reports its event and owner, loses its own staged changes, and leaves healthy observers running. removed listeners are skipped; newly registered listeners start with the next event. host-only validation can still reject the final publication.

`events.listeners(name)` returns the currently registered listener ids in dispatch order. `events.dispatch(name, data, {listeners = ids})` delivers to an explicit, unique list of at most 32 current subscribers. it supports native and custom event names, preserves the requested order and does not add later subscribers. removed listeners are skipped. this supports lazy activation without broadcasting a second native event to every observer; it does not change the actual workspace state. ordinary `events.emit` remains restricted to custom names.

`wraith.plugins.load("plugins.review", options)` uses `require` to load a local module once. it must return a table with `setup(options)`. setup may return a cleanup function; the table may also provide `teardown()`. successful runtimes release dependents before their dependencies when replaced or closed, with reverse registration order within each owner. `init.lua` may itself return a cleanup function. failed candidates are closed without running their runtime actions. failed plugin setup restores its staged registrations even if Lua catches the failure with `pcall`; cleanup releases resources registered by that failed setup. arbitrary Lua globals, module-cache changes and external effects are not rolled back.

`plugins.list()` returns loaded module names in sorted order. `plugins.unload(name)` removes that plugin's commands, mappings and subscriptions, restores covered definitions, and releases its owned resources after the UI accepts the new registry. rejected unloads leave those resources active. callbacks copied from the unloaded plugin are removed too. ordinary module caches remain intact; [module providers](lua/modules.md#module-providers) retire their verified source caches after accepted unload and preserve unverified values.

a plugin that calls `plugins.load` owns a runtime dependency on the loaded plugin. `plugins.load(name, options, {dependency = false})` suppresses that automatic edge when a loader manages dependency order explicitly. `plugins.depend(name)` adds an edge to an already loaded plugin. unload dependents before their dependencies. unloading an initializing plugin or the currently executing plugin is rejected.

`plugins.call(name, method, ...)` invokes a loaded module method under that plugin's owner, returning one Lua value. it accepts at most 16 arguments and does not introduce a method-style `self` argument. registrations and managed work created inside the call belong to the called plugin. calls restore the previous owner on success or error; unloaded plugins and nonfunction methods fail. calls share the current callback deadline and do not provide an asynchronous wait. nesting is limited to 16 calls; `setup` and `teardown` cannot be invoked as exports. a failed export restores its staged registry changes even when the caller catches its error. arbitrary Lua state and external effects remain outside rollback.

these runtime edges describe loaded instances. the [package manager](lua/packages.md) handles installed revisions, dependency specifications and lazy activation through these same APIs.

setup can register APIs and change candidate configuration. runtime workspace actions and snapshot inspection are unavailable during initial configuration setup. teardown runs with runtime actions and configuration changes disabled. cleanup errors do not prevent VM closure. asynchronous failures appear as notices, including errors from the previous configuration after reload. host registry publication does not wait for arbitrary teardown code.

## deferred callbacks

`wraith.schedule(function(ctx) ... end)` runs a function after the current host-effect chain completes. it returns no handle. each scheduled callback receives a fresh workspace snapshot after earlier scheduled callbacks have applied their changes. siblings keep their order across registry updates; a failed callback does not cancel its already queued siblings.

```lua
wraith.commands.create("prepare", function()
  wraith.prompt.set("review the current package")
  wraith.schedule(function(ctx)
    wraith.notify("draft ready: " .. tostring(#ctx.prompt) .. " bytes")
  end)
end)
```

callbacks scheduled during initial setup wait for the workspace's first `ready` event. work belongs to the scheduling plugin and is removed on unload or runtime replacement. an abandoned host-effect chain discards work it has not dispatched. editor actions hold later work until the editor finishes; canceling that editor still releases independent scheduled siblings. a scheduled callback is consumed once, including when it fails.

## module paths

`wraith.paths.stdpath(name)` returns the effective `config`, `data`, `cache` or `state` directory. data and cache honor `--data-dir` and `--cache-dir`, falling back to XDG paths; state follows XDG paths and config honors `--config-dir`. paths are not created by this call.

`wraith.runtimepath.prepend(path)`, `append(path)`, `remove(path)` and `list()` manage ordered roots. each root searches `lua/?.lua`, `lua/?/init.lua`, `?.lua` and `?/init.lua`. paths become absolute; symlinks remain usable. adding an existing root in its current position is idempotent; prepend or append moves it when needed. empty paths and paths containing `;`, `?` or nul are rejected because those characters have module-path syntax.

changing roots preserves explicitly added `package.path` patterns after the managed roots. `list()` returns a copy. `require` retains Lua's module cache: moving or removing a root does not unload modules already in `package.loaded`. clear the module entry explicitly or reload the configuration to resolve it again.

## runtime boundaries

modules resolve through `package.path`, initially `config/lua/?.lua`, `config/lua/?/init.lua`, `config/?.lua` and `config/?/init.lua`. trusted code can change these paths and use symlinks or `package.preload`. the working directory and `LUA_PATH` are not added automatically. project files and assistant responses are never loaded as configuration.

the VM includes base, string, table, math, coroutine, package, `io`, `os` and `debug`. plugins run with your user account authority: they can read files and credentials, write files, run programs or exit the process. `dofile` and `loadfile` are available. `print` produces startup diagnostics or callback notices instead of writing into the terminal frame. native Lua modules and LuaJIT are unsupported. `wraith.capabilities` describes available runtime features. `trusted_lua`, `runtimepath`, `stdpath`, `live_registration`, `custom_events`, `plugin_lifecycle` and `scheduling` are true; `luajit` and `native_modules` are false.

callback deadlines interrupt Lua instructions, not blocking native operations such as `os.execute` or file reads. reload and callback failure cannot undo external effects. suspended coroutines resume under the current callback's deadline.

| limit | value |
| --- | --- |
| candidate setup | 2 seconds |
| command or scheduled callback | 250 milliseconds |
| individual event observer | 50 milliseconds |
| listeners per event | 32; startup scheduling uses one ready listener |
| all teardown hooks together | 200 milliseconds |
| waiting callback requests | 128 |
| ordinary effects per callback or event dispatch | 64 |
| deferred effects per callback or event dispatch | 128 |
| pending scheduled callbacks / startup schedules | 128 / 64 |
| argument or environment value | 64 kib |
| `init.lua`, required source file or serialized snapshot | 1 mib |
| retained callbacks / commands / loaded plugins | 512 / 128 / 128 |

queue overflow is reported. cancellation releases waiting callers immediately. a blocking native call can still delay worker shutdown and teardown. one worker owns the VM; rendering does not wait for Lua execution. initial and required source files must be regular files; direct trusted loaders are unrestricted. these limits protect lifecycle and message handling, but in-process Lua allocation is not a hard memory sandbox. install only local code you trust.

## command overrides

```lua
wraith.commands.create("q", function()
  wraith.action("app.quit")
end, "quit all windows", { replace = true })

wraith.commands.create("note", function(ctx)
  wraith.action("command.pin", ctx.argument)
end, "pin a fact", { arguments = true })
```

`replace` and `arguments` are optional booleans. `arguments = true` makes completion wait for input. replacement applies to both `:name` and `/name`, including `q!`, `qa!`, `w!` and existing dotted commands. completion and help use the replacement description. deletion restores the original command.

physical keys remain separate mappings. `q` in normal mode uses `window.close`; the quit key uses `app.quit`. bind them explicitly to `command.q` if they should follow your override. stable actions `window.close`, `window.close_force`, `app.quit`, `app.quit_force`, `app.save_quit` and `prompt.send` invoke the original operations. calling `command.q` from its own replacement recurses and hits the callback depth limit.

callbacks keep dialog, input ownership and persistence guards. configuration does not grant command execution approval. `:w` and `:w!` send by default; `:wq` saves the draft and quits. invalid reloads retain the previous command definitions.

## native vim mappings

```lua
wraith.keymap.set("prompt.insert", "jk", { keys = "<Esc>" })
for _, mode in ipairs({ "prompt.normal", "prompt.visual", "chat.normal", "chat.visual" }) do
  wraith.keymap.set(mode, "H", { keys = "^" })
  wraith.keymap.set(mode, "L", { keys = "$" })
end
wraith.keymap.set("prompt.operator", "H", { keys = "^" })
wraith.keymap.set("output.normal", "J", { keys = "5j" })
wraith.keymap.set("terminal.copy.normal", "J", { keys = "5j" })
```

native sequences bypass mappings and workspace actions. `2H` keeps its count; pending `d`, `c` and `y` use operator mappings. `f`, `t`, `r` and text-object selectors use argument mappings, so a normal-mode mapping cannot replace a literal target. each sequence allows 16 keys and 256 bytes; callback batches allow 128 native keys. unsupported native controls reject the entire sequence before editing. callbacks can use `wraith.input.feed("0dw")` or change modes with `wraith.input.set_mode("visual")`.

prompt editing supports insert, normal, visual, visual-line and search. conversations, command output and terminal copy buffers remain read-only. native input cannot open a command editor, approve a command, send the prompt, close the app or write into a live shell. invoke the corresponding named action for workspace operations. raw shells, approval dialogs, ordinary pickers and the Ex command line reject native editor input.

`input.get().mode` includes the surface, such as `prompt.operator` or `output.visual`. `cursor`, `selection_start` and `selection_end` are zero-based rune offsets; selection end is exclusive and inactive bounds are `-1`. chat uses zero-based `row` and display-cell `column`, plus `anchor_row` and `anchor_column`. `count`, `pending`, `search` and chat `operator_count` describe queued intent. `available` identifies a surface that accepts native input. snapshots describe callback-start state, not earlier queued effects. changed readers, text, cursors, search or pending operators invalidate delayed callbacks.
