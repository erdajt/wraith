---@meta

wraith = wraith or {}

---@class Wraith.QueryError
---@field code 'cancelled'|'deadline'|'not_found'|'wrong_directory'|'stale_revision'|'limit'|'io'
---@field message string
---@class Wraith.QueryResult<T>
---@field id string query handle
---@field kind string
---@field ok boolean
---@field result? T present on success
---@field error? Wraith.QueryError present on failure
---@class Wraith.CatalogPage<T>
---@field items T[]
---@field errors {id: string, message: string}[]
---@field next string opaque single-use cursor; empty when complete
---@field complete boolean an empty filtered page may still be incomplete
---@field examined integer
---@class Wraith.CatalogResult<T>
---@field page Wraith.CatalogPage<T>
---@field active T[] captured live records on the first page only

---@class Wraith.SessionMetadata
---@field id string
---@field title string
---@field directory string
---@field model string
---@field provider string
---@field created string timestamp
---@field updated string timestamp
---@field message_count integer
---@field draft_bytes integer
---@field revision string transcript revision
---@class Wraith.StoredMessageInfo
---@field index integer zero-based
---@field id string
---@field role string
---@field preview string at most 1 kib
---@field content_bytes integer
---@field thinking_bytes integer
---@field edited boolean
---@field interrupted boolean
---@class Wraith.MessagesPage
---@field items Wraith.StoredMessageInfo[]
---@field revision string
---@field next integer
---@field complete boolean
---@class Wraith.MessageText
---@field revision string
---@field text string
---@field next integer utf-8 byte offset
---@field complete boolean
---@class Wraith.MessageQuery
---@field id? string current session when omitted
---@field revision string required expected transcript revision
---@field index integer zero-based message index
---@field field? 'content'|'thinking'
---@field from? integer utf-8 byte boundary
---@field limit? integer default 16384, maximum 65536

---@class Wraith.SessionsAPI
wraith.sessions = wraith.sessions or {}
---@param options {directory?: string, cursor?: string, limit?: integer}
---@param callback fun(ctx: Wraith.Context<Wraith.QueryResult<Wraith.CatalogResult<Wraith.SessionMetadata>>>)
---@return string query_id
function wraith.sessions.list(options, callback) end
---@param options {id?: string, directory?: string}
---@param callback fun(ctx: Wraith.Context<Wraith.QueryResult<Wraith.SessionMetadata>>)
---@return string query_id
function wraith.sessions.get(options, callback) end
---@param options {id?: string, revision?: string, from?: integer, limit?: integer}
---@param callback fun(ctx: Wraith.Context<Wraith.QueryResult<Wraith.MessagesPage>>)
---@return string query_id
function wraith.sessions.messages(options, callback) end
---@param options Wraith.MessageQuery
---@param callback fun(ctx: Wraith.Context<Wraith.QueryResult<Wraith.MessageText>>)
---@return string query_id
function wraith.sessions.message(options, callback) end

---@class Wraith.JobMetadata
---@field id string
---@field session string
---@field directory string
---@field command string
---@field shell string
---@field source string
---@field started string timestamp
---@field finished string timestamp
---@field status string
---@field exit_code integer
---@field error string
---@field background boolean
---@field agent boolean
---@field share_output boolean
---@field output_hidden boolean
---@field observed_bytes integer
---@field captured_bytes integer
---@field truncated boolean
---@field revision string
---@class Wraith.OutputStats
---@field observed_bytes integer
---@field prefix_bytes integer
---@field tail_bytes integer
---@field retained_lines integer
---@field matched_lines integer
---@field elapsed_ms integer
---@field gap_from_byte? integer
---@field gap_to_byte? integer
---@class Wraith.OutputPage
---@field stats Wraith.OutputStats
---@field lines {line: integer, text: string}[] native output line numbers
---@field next? integer
---@field capture_truncated boolean
---@class Wraith.JobOutput
---@field revision string
---@field page Wraith.OutputPage
---@field record Wraith.JobMetadata
---@class Wraith.OutputQuery
---@field id string native 24-character lowercase hexadecimal job id
---@field revision? string rejects changed status or output extent
---@field from? integer minimum native line number; 0 starts at the beginning
---@field pattern? string literal by default
---@field regex? boolean bounded go regex
---@field ignore_case? boolean
---@field before? integer 0..10
---@field after? integer 0..10
---@field limit? integer default 80, maximum 200

---@class Wraith.JobsAPI
wraith.jobs = wraith.jobs or {}
---@param options {session?: string, directory?: string, cursor?: string, limit?: integer} choose session or directory
---@param callback fun(ctx: Wraith.Context<Wraith.QueryResult<Wraith.CatalogResult<Wraith.JobMetadata>>>)
---@return string query_id
function wraith.jobs.list(options, callback) end
---@param options {id: string}
---@param callback fun(ctx: Wraith.Context<Wraith.QueryResult<Wraith.JobMetadata>>)
---@return string query_id
function wraith.jobs.get(options, callback) end
-- trusted local reads can inspect hidden output but never change model sharing flags.
---@param options Wraith.OutputQuery
---@param callback fun(ctx: Wraith.Context<Wraith.QueryResult<Wraith.JobOutput>>)
---@return string query_id
function wraith.jobs.output(options, callback) end

---@class Wraith.VariableInfo
---@field name string
---@field description string private values in descriptions are redacted
---@class Wraith.VariableInventory
---@field items Wraith.VariableInfo[] values are never included
---@field directory string
---@field session string
---@field scope string
---@class Wraith.EnvironmentAPI
wraith.environment = wraith.environment or {}
---@param options table empty options table
---@param callback fun(ctx: Wraith.Context<Wraith.QueryResult<Wraith.VariableInventory>>)
---@return string query_id
function wraith.environment.list(options, callback) end

---@class Wraith.TerminalInfo
---@field id string
---@field index integer
---@field generation integer
---@field visible boolean
---@field running boolean
---@field width integer
---@field height integer
---@field error string
---@class Wraith.TerminalsAPI
wraith.terminals = wraith.terminals or {}
---@param options table empty options table
---@param callback fun(ctx: Wraith.Context<Wraith.QueryResult<{items: Wraith.TerminalInfo[]}>>)
---@return string query_id
function wraith.terminals.list(options, callback) end

---@class Wraith.WorkspaceAPI
wraith.workspace = wraith.workspace or {}
-- closes owned pending work and its unconsumed cursor; cancel the latest page's query id.
---@param query_id string
function wraith.workspace.cancel(query_id) end
