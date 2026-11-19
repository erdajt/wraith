---@meta

---@class Wraith.MessageSummary
---@field role string
---@field content string

---@class Wraith.SessionSnapshot
---@field id string
---@field model string
---@field provider string
---@field directory string
---@field messages Wraith.MessageSummary[] bounded recent excerpt, not the complete transcript
---@field message_count integer

---@class Wraith.InputSnapshot
---@field mode Wraith.InputMode|string
---@field available boolean
---@field cursor? integer zero-based rune offset in a text buffer
---@field selection_start? integer
---@field selection_end? integer
---@field row? integer native conversation row
---@field column? integer
---@field anchor_row? integer
---@field anchor_column? integer
---@field count? string
---@field operator_count? string
---@field pending? string
---@field pending_g? boolean
---@field pending_z? boolean
---@field search? string

---@class Wraith.NativeWindow
---@field id integer native ids differ from opaque owned window ids
---@field focused boolean
---@field cursor integer
---@field column integer
---@field search string
---@field offset integer
---@field visible boolean
---@field width? integer
---@field height? integer

---@class Wraith.ResourceEvent
---@field id string
---@field revision integer
---@field sequence integer monotonic event sequence; not a byte offset
---@field kind string
---@field final boolean cancellation may be final before the worker is reaped

---@class Wraith.InvocationContext
---@field id string
---@field kind? 'provider'|'tool' only present at initial invocation dispatch
---@field name? string
---@field method? 'models'|'show'|'chat'|'run'

---@class Wraith.SurfaceIndex
---@field buffers Wraith.BufferInfo[]
---@field windows Wraith.Window[]
---@field regions Wraith.RegionInfo[]

---@class Wraith.Context<T>
---@field argument string
---@field argument_truncated boolean
---@field event string
---@field prompt string bounded compatibility draft snapshot
---@field prompt_truncated boolean
---@field input Wraith.InputSnapshot
---@field session Wraith.SessionSnapshot
---@field windows Wraith.NativeWindow[]
---@field surfaces Wraith.SurfaceIndex
---@field target string opaque original-input receipt; compare for equality only
---@field data? T
---@field resource? Wraith.ResourceEvent
---@field invocation? Wraith.InvocationContext

wraith = wraith or {}
---@class Wraith.SessionAPI
wraith.session = wraith.session or {}
---@return Wraith.SessionSnapshot
function wraith.session.get() end

---@class Wraith.InputAPI
wraith.input = wraith.input or {}
---@return Wraith.InputSnapshot
function wraith.input.get() end
-- feeds at most 128 native keys per batch, without recursively applying plugin mappings.
---@param keys string
function wraith.input.feed(keys) end
---@param mode Wraith.InputMode
function wraith.input.set_mode(mode) end

---@class Wraith.PromptAPI
wraith.prompt = wraith.prompt or {}
---@return string captured_draft
function wraith.prompt.get() end
---@param text string
function wraith.prompt.set(text) end
