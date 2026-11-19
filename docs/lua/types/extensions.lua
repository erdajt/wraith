---@meta

wraith = wraith or {}

---@class Wraith.ModelOptions
---@field num_ctx integer
---@field num_predict integer
---@field temperature number
---@field num_thread? integer
---@class Wraith.HookMessage
---@field role 'system'|'user'|'assistant'
---@field content string empty for protected references
---@field reference? string opaque evidence reference; do not invent, duplicate or change its role
---@class Wraith.RequestData
---@field model string
---@field messages Wraith.HookMessage[]
---@field think? boolean|string
---@field options Wraith.ModelOptions
---@class Wraith.CommandData
---@field command string
---@field directory string
---@field shell string
---@field environment string[] ordered NAME=value entries
---@field timeout string exact go duration, such as 30s or 1.5ms
---@field background boolean
---@class Wraith.HookIdentity
---@field id string
---@field session_id string
---@field directory string
---@field purpose string
---@class Wraith.RequestHookContext: Wraith.HookIdentity
---@field kind 'before_request'
---@field phase 'prepare'
---@field purpose 'chat'|'extract'|'audit'
---@field request Wraith.RequestData
---@class Wraith.CommandHookContext: Wraith.HookIdentity
---@field kind 'before_command'
---@field phase 'preview'|'execute'
---@field command Wraith.CommandData
---@class Wraith.RequestHookResult
---@field request? Wraith.RequestData
---@field decision? 'allow'|'deny'
---@field reason? string
---@class Wraith.CommandHookResult
---@field command? Wraith.CommandData
---@field decision? 'allow'|'deny'
---@field reason? string
---@class Wraith.HookOptions
---@field priority? integer -10000..10000; lower runs first
---@field purposes? ('chat'|'extract'|'audit')[] defaults to chat
---@alias Wraith.RequestHook fun(ctx: Wraith.RequestHookContext): Wraith.RequestHookResult?
---@alias Wraith.CommandHook fun(ctx: Wraith.CommandHookContext): Wraith.CommandHookResult?
---@class Wraith.HooksAPI
wraith.hooks = wraith.hooks or {}
-- pure hooks return data; host APIs except json are unavailable, including captured references.
---@overload fun(kind: 'before_request', callback: Wraith.RequestHook, options?: Wraith.HookOptions): string
---@param kind 'before_command'
---@param callback Wraith.CommandHook
---@param options? Wraith.HookOptions
---@return string hook_id
function wraith.hooks.add(kind, callback, options) end
---@param id string
function wraith.hooks.remove(id) end

---@class Wraith.Schema
---@field type? string|string[]
---@field properties? table<string, Wraith.Schema|boolean>
---@field patternProperties? table<string, Wraith.Schema|boolean>
---@field dependentSchemas? table<string, Wraith.Schema|boolean>
---@field required? string[]
---@field dependentRequired? table<string, string[]>
---@field additionalProperties? Wraith.Schema|boolean
---@field unevaluatedProperties? Wraith.Schema|boolean
---@field propertyNames? Wraith.Schema|boolean
---@field items? Wraith.Schema|boolean
---@field unevaluatedItems? Wraith.Schema|boolean
---@field contains? Wraith.Schema|boolean
---@field prefixItems? (Wraith.Schema|boolean)[]
---@field allOf? (Wraith.Schema|boolean)[]
---@field anyOf? (Wraith.Schema|boolean)[]
---@field oneOf? (Wraith.Schema|boolean)[]
---@field not? Wraith.Schema|boolean
---@field if? Wraith.Schema|boolean
---@field then? Wraith.Schema|boolean
---@field else? Wraith.Schema|boolean
---@field enum? Wraith.JSON[]
---@field const? Wraith.JSON
---@field minimum? number
---@field maximum? number
---@field exclusiveMinimum? number
---@field exclusiveMaximum? number
---@field multipleOf? number
---@field minProperties? integer
---@field maxProperties? integer
---@field minItems? integer
---@field maxItems? integer
---@field minContains? integer
---@field maxContains? integer
---@field uniqueItems? boolean
---@field minLength? integer
---@field maxLength? integer
---@field pattern? string bounded go regex
---@field title? string
---@field description? string
---@field default? Wraith.JSON
---@field examples? Wraith.JSON[]
---@field deprecated? boolean
---@field readOnly? boolean
---@field writeOnly? boolean
---@field ['$schema']? string only draft 2020-12
---@field ['$comment']? string

---@class Wraith.ProviderModel
---@field name string
---@field size? integer
---@class Wraith.ProviderInfo
---@field context_window? integer
---@field capabilities? string[]
---@field thinking? {values?: (boolean|string)[], default?: boolean|string}
---@class Wraith.ProviderMessage
---@field role string
---@field content string
---@field thinking? string
---@field edited? boolean
---@field interrupted? boolean
---@class Wraith.ProviderRequest
---@field model string
---@field messages Wraith.ProviderMessage[] final private, policy-composed messages
---@field think? boolean|string
---@field options Wraith.ModelOptions
---@field stream boolean
---@field keep_alive string
---@field format? Wraith.JSON
---@field tools? {type: 'function', ['function']: {name: string, description: string, parameters: Wraith.Schema}}[]
---@class Wraith.ProviderDelta
---@field content? string
---@field thinking? string
---@class Wraith.ProviderResult: Wraith.ProviderDelta
---@field calls? {name: string, arguments: table<string, Wraith.JSON>}[] at most one native call
---@field reason? 'stop'|'length'
---@field prompt_tokens? integer
---@field output_tokens? integer
---@field duration_ns? integer
---@class Wraith.ProviderRegistration
---@field description? string
---@field replace? boolean
---@field models fun(ctx: Wraith.Context<table>) finish with ProviderModel[]; no stream
---@field show fun(ctx: Wraith.Context<{model: string}>) finish with ProviderInfo; no stream
---@field chat fun(ctx: Wraith.Context<Wraith.ProviderRequest>) emit ProviderDelta, finish ProviderResult
---@class Wraith.ProvidersAPI
wraith.providers = wraith.providers or {}
---@param name string matches an explicitly selected kind=plugin provider profile
---@param options Wraith.ProviderRegistration
function wraith.providers.register(name, options) end
---@param name string removes the caller's layer; restores any covered definition
function wraith.providers.unregister(name) end

---@class Wraith.ToolInput
---@field arguments table<string, Wraith.JSON> resolved local values after approval
---@field job_id string native background job id
---@field directory string
---@class Wraith.ToolRegistration
---@field description? string
---@field replace? boolean
---@field approval? 'required'|'read_only' metadata only; both require native approval
---@field parameters Wraith.Schema object root required; references are unsupported
---@field run fun(ctx: Wraith.Context<Wraith.ToolInput>) emit and finish arbitrary bounded json
---@class Wraith.ToolsAPI
wraith.tools = wraith.tools or {}
---@param name string
---@param options Wraith.ToolRegistration
function wraith.tools.register(name, options) end
---@param name string
function wraith.tools.unregister(name) end

---@class Wraith.InvocationAPI
wraith.invocation = wraith.invocation or {}
-- replies belong to the current callback lineage; never block lua waiting for a managed reply.
---@param id string ctx.invocation.id
---@param value Wraith.JSON
function wraith.invocation.emit(id, value) end
-- finishing cancels managed children; later effects from the finished invocation are discarded.
---@param id string
---@param value Wraith.JSON
function wraith.invocation.finish(id, value) end
---@param id string
---@param message string 1..4096 bytes
function wraith.invocation.fail(id, message) end
---@param id string
---@return boolean
function wraith.invocation.cancelled(id) end
