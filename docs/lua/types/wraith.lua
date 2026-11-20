---@meta

-- editor declarations only. host validation failures raise lua errors.
---@alias Wraith.JSON nil|boolean|number|string|Wraith.JSON[]|table<string, Wraith.JSON>|userdata
---@alias Wraith.InputMode 'normal'|'insert'|'visual'|'visual-line'|'search'
---@alias Wraith.KeyMode '*'|'command'|'search'|'picker'|'output'|'buffer'|'terminal'|'terminal.copy'|'prompt.insert'|'buffer.insert'|'prompt.normal'|'prompt.visual'|'prompt.operator'|'prompt.argument'|'prompt.search'|'chat.normal'|'chat.visual'|'chat.operator'|'chat.argument'|'chat.search'|'output.normal'|'output.visual'|'output.operator'|'output.argument'|'output.search'|'buffer.normal'|'buffer.visual'|'buffer.operator'|'buffer.argument'|'buffer.search'|'terminal.copy.normal'|'terminal.copy.visual'|'terminal.copy.operator'|'terminal.copy.argument'|'terminal.copy.search'
---@alias Wraith.Event 'ready'|'session_changed'|'window_changed'|'buffer_changed'|'response_finished'|'jobs_changed'|'terminals_changed'|'environment_changed'|string
---@alias Wraith.Mapping string|fun(ctx: Wraith.Context)|{keys: string}

---@class Wraith
---@field api_version integer
---@field capabilities table<string, boolean>
---@field config Wraith.ConfigAPI
---@field theme Wraith.ThemeAPI
---@field keymap Wraith.KeymapAPI
---@field commands Wraith.CommandsAPI
---@field events Wraith.EventsAPI
---@field plugins Wraith.PluginsAPI
---@field paths Wraith.PathsAPI
---@field runtimepath Wraith.RuntimepathAPI
---@field modules Wraith.ModulesAPI
---@field json Wraith.JSONAPI
---@field fs Wraith.FSAPI
---@field http Wraith.HTTPAPI
---@field timer Wraith.TimerAPI
---@field process Wraith.ProcessAPI
---@field resource Wraith.ResourceAPI
---@field buffers Wraith.BuffersAPI
---@field windows Wraith.WindowsAPI
---@field regions Wraith.RegionsAPI
---@field ui Wraith.UIAPI
---@field input Wraith.InputAPI
---@field prompt Wraith.PromptAPI
---@field session Wraith.SessionAPI
---@field sessions Wraith.SessionsAPI
---@field jobs Wraith.JobsAPI
---@field terminals Wraith.TerminalsAPI
---@field environment Wraith.EnvironmentAPI
---@field workspace Wraith.WorkspaceAPI
---@field hooks Wraith.HooksAPI
---@field providers Wraith.ProvidersAPI
---@field tools Wraith.ToolsAPI
---@field invocation Wraith.InvocationAPI
wraith = wraith or {}
wraith.api_version = 2
wraith.capabilities = {
  trusted_lua = true, runtimepath = true, stdpath = true, modules = true, module_sources = true, module_loading = true,
  module_providers = true,
  live_registration = true, custom_events = true, plugin_lifecycle = true, plugins_unload_many = true,
  plugins_state = true, ready_replay = true,
  scheduling = true, json = true, filesystem = true, http = true,
  timers = true, managed_resources = true, process = true,
  buffers = true, windows = true, regions = true, dialogs = true,
  buffer_keymaps = true, hooks = true, providers = true, model_tools = true,
  invocations = true, workspace_queries = true, luajit = false, native_modules = false,
}

-- reads the process environment. configured private variables use environment.list.
---@param name string
---@return string? value
function wraith.getenv(name) end

-- queues a notice; runtime callbacks only, at most 64 kib per message.
---@param message string
function wraith.notify(message) end

-- queues a registered host action; implicit input edits keep their workspace guards.
---@param name string
---@param argument? string
function wraith.action(name, argument) end

-- runs once after the current host batch, with fresh context; 128 pending, 64 at setup.
---@param callback fun(ctx: Wraith.Context)
function wraith.schedule(callback) end

---@class Wraith.KeymapAPI
wraith.keymap = wraith.keymap or {}
---@param mode Wraith.KeyMode
---@param keys string
---@param target Wraith.Mapping
---@param description? string
function wraith.keymap.set(mode, keys, target, description) end
---@param mode Wraith.KeyMode
---@param keys string
function wraith.keymap.del(mode, keys) end
---@param keys string
---@return string[] canonical_tokens
function wraith.keymap.parse(keys) end
---@param mode Wraith.KeyMode
---@param keys string
---@return true
function wraith.keymap.validate(mode, keys) end

---@class Wraith.CommandOptions
---@field replace? boolean
---@field arguments? boolean

---@class Wraith.CommandsAPI
wraith.commands = wraith.commands or {}
---@param name string
---@param callback fun(ctx: Wraith.Context)
---@param description? string
---@param options? Wraith.CommandOptions
function wraith.commands.create(name, callback, description, options) end
---@param name string
function wraith.commands.delete(name) end

---@class Wraith.EventsAPI
wraith.events = wraith.events or {}
-- observers run in registration order with individual 50 ms cooperative budgets.
---@generic T
---@param name Wraith.Event
---@param callback fun(ctx: Wraith.Context<T>)
---@return string listener_id
function wraith.events.on(name, callback) end
---@param listener_id string
function wraith.events.off(listener_id) end
-- only namespaced custom events can be broadcast by plugins.
---@param name string
---@param data? Wraith.JSON
function wraith.events.emit(name, data) end
---@param name Wraith.Event
---@return string[] listener_ids
function wraith.events.listeners(name) end
-- explicit empty listeners deliver nothing; retired listeners are skipped.
---@param name Wraith.Event
---@param data Wraith.JSON
---@param options {listeners: string[]}
function wraith.events.dispatch(name, data, options) end
-- catches up ready subscribers once per effect batch after native ready starts.
---@param listeners string[]
function wraith.events.replay_ready(listeners) end

---@class Wraith.PluginsAPI
wraith.plugins = wraith.plugins or {}
---@return string[] module_names
function wraith.plugins.list() end
-- snapshots the current host lifecycle; absent names return nil.
---@param module string
---@return 'loading'|'loaded'|nil state
function wraith.plugins.state(module) end
-- requires a module with setup(options); optional teardown and returned cleanup are owned.
---@param module string
---@param options? table
---@param load_options? {dependency?: boolean}
function wraith.plugins.load(module, options, load_options) end
-- refuses unloading an executing package or one with active dependents.
---@param module string
function wraith.plugins.unload(module) end
-- stages up to 128 unique loaded names together; empty is a no-op, outside dependents reject.
---@param modules string[]
function wraith.plugins.unload_many(modules) end
---@param module string
function wraith.plugins.depend(module) end
-- calls one captured export under its package owner; one return value, at most 16 arguments.
---@generic T
---@param module string
---@param method string
---@param ... any
---@return T
function wraith.plugins.call(module, method, ...) end

---@class Wraith.PathsAPI
wraith.paths = wraith.paths or {}
---@param kind 'config'|'data'|'cache'|'state'
---@return string absolute_path
function wraith.paths.stdpath(kind) end

---@class Wraith.RuntimepathAPI
wraith.runtimepath = wraith.runtimepath or {}
---@return string[] roots
function wraith.runtimepath.list() end
---@param path string
function wraith.runtimepath.prepend(path) end
---@param path string
function wraith.runtimepath.append(path) end
---@param path string
function wraith.runtimepath.remove(path) end

---@class Wraith.ModuleSource
---@field name string normalized module name
---@field kind 'file'|'preload'
---@field path? string absolute source path for files
---@field identity string opaque source identity, stable within this runtime
---@field load fun(): ... runs the bound source afresh and preserves all return values

---@alias Wraith.ModuleCheckPhase 'validate'|'before'|'after'
---@alias Wraith.ModuleCheck fun(name: string, phase: Wraith.ModuleCheckPhase, proof: any, executed: boolean): any
---@class Wraith.ModuleResolution
---@field plugin string selected lifecycle module
---@field activate fun(name: string) loads the selected plugin under the resolver owner
---@field check Wraith.ModuleCheck verifies source/cache ownership; before returns the token passed to after
---@alias Wraith.ModuleResolver fun(name: string): Wraith.ModuleResolution?

---@class Wraith.ModulesAPI
wraith.modules = wraith.modules or {}
--- registers owned metadata resolution; nil means a miss, at most 32 providers.
---@param resolve Wraith.ModuleResolver
---@return string provider_id opaque id, never reused within this runtime
function wraith.modules.provider(resolve) end

--- removes a live provider owned by the caller; prior claims and poison remain.
---@param provider_id string
function wraith.modules.remove_provider(provider_id) end

--- reports the native require marker without evaluating a module or exposing its cache value.
---@param name string dotted module name, at most 128 bytes
---@return boolean loading
function wraith.modules.loading(name) end

-- discovers an exact module and immediate children from the first selected source root.
---@param name string dotted module name, at most 128 bytes
---@return Wraith.ModuleSource[] sources sorted by name, at most 256
function wraith.modules.discover(name) end

--- lists immediate file modules in one explicit root's lua directory without evaluating them.
---@param root string absolute existing directory; explicit root symlinks are followed
---@return Wraith.ModuleSource[] sources sorted by name, at most 256
function wraith.modules.catalog(root) end

--- binds one file module from an explicit root, checking lua layout before legacy root files.
---@param root string absolute existing directory
---@param name string dotted module name
---@return Wraith.ModuleSource? source nil only when neither layout contains the module
function wraith.modules.source(root, name) end
