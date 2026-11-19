---@meta

wraith = wraith or {}

---@class Wraith.JSONAPI
---@field null userdata explicit json null sentinel; distinct from absent nil fields
wraith.json = wraith.json or {}
wraith.json.null = newproxy(false)
---@param value Wraith.JSON
---@return string json
function wraith.json.encode(value) end
---@param json string
---@return Wraith.JSON value
function wraith.json.decode(json) end
---@generic T
---@param value? T[]
---@return T[] array tagged, including an empty array
function wraith.json.array(value) end
---@generic T
---@param value? table<string, T>
---@return table<string, T> object tagged, including an empty object
function wraith.json.object(value) end

---@alias Wraith.ResourceState 'reserved'|'running'|'completed'|'failed'|'cancelled'
---@class Wraith.ResourceCompletion<T>
---@field state Wraith.ResourceState
---@field done boolean true only when the worker has finished
---@field result? T
---@field error? string
---@field callback_error? string

---@class Wraith.ResourceStatus<T>: Wraith.ResourceCompletion<T>
---@field id string
---@field kind string
---@field revision integer
---@field sequence integer
---@field dropped integer coalesced or omitted capture notifications

---@class Wraith.ResourceAPI
wraith.resource = wraith.resource or {}
-- owner-scoped; at most 32 unfinished workers/reservations and 128 retained handles.
---@param id string
---@return Wraith.ResourceStatus
function wraith.resource.status(id) end
---@param id string
function wraith.resource.cancel(id) end
-- ordered release invalidates the handle; release completed handles to recover capacity.
---@param id string
function wraith.resource.release(id) end

---@class Wraith.FilePathResult
---@field path string absolute source path
---@class Wraith.FileRead: Wraith.FilePathResult
---@field data string exact bytes, not necessarily utf-8
---@field offset integer
---@field next_offset integer
---@field truncated boolean
---@class Wraith.FileWrite: Wraith.FilePathResult
---@field bytes integer
---@alias Wraith.FileKind 'file'|'directory'|'symlink'|'other'
---@class Wraith.FileStat: Wraith.FilePathResult
---@field exists true
---@field size integer
---@field mode integer permission bits
---@field type Wraith.FileKind
---@field modified_ms integer
---@class Wraith.FileMissing: Wraith.FilePathResult
---@field exists false
---@alias Wraith.FileStatResult Wraith.FileStat|Wraith.FileMissing
---@class Wraith.FileList: Wraith.FilePathResult
---@field entries {name: string, type: Wraith.FileKind}[]
---@field next_offset integer
---@field truncated boolean

---@class Wraith.FSReadOptions
---@field offset? integer zero-based byte offset
---@field max_bytes? integer 1..4194304; default 1048576
---@class Wraith.FSWriteOptions
---@field overwrite? boolean default false
---@field mode? integer permission bits; default 384 (0600)
---@class Wraith.FSStatOptions
---@field follow? boolean default false
---@field missing_ok? boolean default false; report absent paths instead of raising
---@class Wraith.FSListOptions
---@field offset? integer directory enumeration offset
---@field limit? integer 1..1024

---@class Wraith.FSAPI
wraith.fs = wraith.fs or {}
-- synchronous operations return results or raise; native filesystem calls can block the vm.
---@param path string
---@param options? Wraith.FSReadOptions
---@return Wraith.FileRead
function wraith.fs.read(path, options) end
---@param path string
---@param data string at most 4 mib
---@param options? Wraith.FSWriteOptions
---@return Wraith.FileWrite
function wraith.fs.write(path, data, options) end
---@overload fun(path: string, options?: {follow?: boolean, missing_ok?: false}): Wraith.FileStat
---@param path string
---@param options? Wraith.FSStatOptions
---@return Wraith.FileStatResult
function wraith.fs.stat(path, options) end
---@param path string
---@param options? Wraith.FSListOptions
---@return Wraith.FileList
function wraith.fs.list(path, options) end
---@param path string
---@param options? {parents?: boolean, mode?: integer}
---@return Wraith.FilePathResult
function wraith.fs.mkdir(path, options) end
---@param path string
---@param options? {recursive?: boolean}
---@return Wraith.FilePathResult
function wraith.fs.remove(path, options) end
---@param source string
---@param destination string
---@param options? {overwrite?: boolean}
---@return Wraith.FilePathResult
function wraith.fs.rename(source, destination, options) end

-- asynchronous operations reserve managed work and report errors in ctx.data.error.
---@overload fun(path: string, callback: fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.FileRead>>)): string
---@param path string
---@param options Wraith.FSReadOptions?
---@param callback fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.FileRead>>)
---@return string resource_id
function wraith.fs.read_async(path, options, callback) end
---@overload fun(path: string, data: string, callback: fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.FileWrite>>)): string
---@param path string
---@param data string
---@param options Wraith.FSWriteOptions?
---@param callback fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.FileWrite>>)
---@return string resource_id
function wraith.fs.write_async(path, data, options, callback) end
---@overload fun(path: string, callback: fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.FileStat>>)): string
---@param path string
---@param options Wraith.FSStatOptions?
---@param callback fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.FileStatResult>>)
---@return string resource_id
function wraith.fs.stat_async(path, options, callback) end
---@overload fun(path: string, callback: fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.FileList>>)): string
---@param path string
---@param options Wraith.FSListOptions?
---@param callback fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.FileList>>)
---@return string resource_id
function wraith.fs.list_async(path, options, callback) end
---@overload fun(path: string, callback: fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.FilePathResult>>)): string
---@param path string
---@param options {parents?: boolean, mode?: integer}?
---@param callback fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.FilePathResult>>)
---@return string resource_id
function wraith.fs.mkdir_async(path, options, callback) end
---@overload fun(path: string, callback: fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.FilePathResult>>)): string
---@param path string
---@param options {recursive?: boolean}?
---@param callback fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.FilePathResult>>)
---@return string resource_id
function wraith.fs.remove_async(path, options, callback) end
---@overload fun(source: string, destination: string, callback: fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.FilePathResult>>)): string
---@param source string
---@param destination string
---@param options {overwrite?: boolean}?
---@param callback fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.FilePathResult>>)
---@return string resource_id
function wraith.fs.rename_async(source, destination, options, callback) end

---@class Wraith.HTTPOptions
---@field url string absolute http or https url
---@field method? string default GET
---@field body? string exact request bytes, at most 4 mib
---@field headers? table<string, string> host header comes from url
---@field timeout_ms? integer 1..300000; default 30000
---@field max_bytes? integer 1..4194304; default 1048576
---@field redirects? integer 0..10; default 0
---@class Wraith.HTTPResult
---@field status integer non-2xx responses are still responses
---@field url string final url
---@field headers table<string, string[]>
---@field body? string exact bytes
---@field complete boolean
---@field truncated? boolean
---@class Wraith.HTTPAPI
wraith.http = wraith.http or {}
---@param options Wraith.HTTPOptions
---@param callback fun(ctx: Wraith.Context<Wraith.ResourceCompletion<Wraith.HTTPResult>>)
---@return string resource_id
function wraith.http.request(options, callback) end

---@class Wraith.TimerTick
---@field tick integer
---@field elapsed_ms integer
---@class Wraith.TimerAPI
wraith.timer = wraith.timer or {}
-- one-shot results use ctx.data.result; repeating tick notifications use ctx.data directly.
---@param delay_ms integer 1..86400000
---@param callback fun(ctx: Wraith.Context<Wraith.TimerTick|Wraith.ResourceCompletion<Wraith.TimerTick>>)
---@param options? {repeat_ms?: integer}
---@return string resource_id
function wraith.timer.start(delay_ms, callback, options) end

---@class Wraith.ProcessOptions
---@field argv? string[] choose argv or shell, never both
---@field shell? string explicit shell command
---@field shell_path? string absolute shell executable; default /bin/sh
---@field cwd? string
---@field env? table<string, string|false> false removes an inherited name
---@field clear_env? boolean
---@field stdin? boolean enables managed writes
---@field mode? 'capture'|'protocol'
---@field timeout_ms? integer 0..86400000; 0 disables the process timeout
---@field max_output_bytes? integer 1024..4194304 retained per stream
---@class Wraith.ProcessResult
---@field pid integer
---@field exit_code integer
---@field stdin_error? string
---@class Wraith.ProcessChunk
---@field stream 'stdout'|'stderr'
---@field offset integer
---@field next_offset integer
---@field earliest_offset integer
---@field data? string present for protocol chunks; exact bytes
---@class Wraith.ProcessOutput: Wraith.ProcessChunk
---@field data string
---@field observed_bytes integer
---@field more boolean retained bytes remain after this page
---@field truncated boolean requested offset was evicted
---@alias Wraith.ProcessEvent Wraith.ProcessChunk|{pid: integer}|{written: integer, error: string}|Wraith.ResourceCompletion<Wraith.ProcessResult>
---@class Wraith.ProcessAPI
wraith.process = wraith.process or {}
-- trusted plugin processes do not use model-command approval. work starts after host acceptance.
---@param options Wraith.ProcessOptions
---@param callback fun(ctx: Wraith.Context<Wraith.ProcessEvent>)
---@return string resource_id
function wraith.process.start(options, callback) end
---@param id string
---@param data string
---@return integer accepted_bytes not a guarantee the child has read them
function wraith.process.write(id, data) end
---@param id string
function wraith.process.close_stdin(id) end
---@param id string
---@param options? {stream?: 'stdout'|'stderr', offset?: integer, max_bytes?: integer}
---@return Wraith.ProcessOutput
function wraith.process.output(id, options) end
