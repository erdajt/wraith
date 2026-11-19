---@meta

wraith = wraith or {}

---@class Wraith.Position
---@field line integer zero-based
---@field column integer zero-based rune column, not bytes or terminal cells
---@class Wraith.Style
---@field foreground? string theme palette name or #rrggbb
---@field background? string theme palette name or #rrggbb
---@field bold? boolean
---@field italic? boolean
---@field underline? boolean
---@class Wraith.Span
---@field start Wraith.Position
---@field end Wraith.Position half-open; use ["end"] in lua constructors
---@field style Wraith.Style

---@class Wraith.BufferOptions
---@field name? string
---@field lines? string[] valid utf-8 text without terminal controls; empty normalizes to one empty line
---@field editable? boolean default false
---@field spans? Wraith.Span[]
---@class Wraith.BufferInfo
---@field id string
---@field owner string
---@field name string
---@field revision integer text, decoration or mapping revision
---@field text_revision integer changes only with canonical text
---@field editable boolean
---@field line_count integer
---@field byte_count integer
---@field projection? 'prompt'|'conversation'
---@field truncated? boolean native projection is incomplete; truncated prompts are read-only
---@class Wraith.Buffer
---@field id string
---@field owner string
---@field name string
---@field revision integer
---@field text_revision integer
---@field editable boolean
---@field lines string[]
---@field spans Wraith.Span[]
---@field keymaps Wraith.Binding[]
---@field projection? 'prompt'|'conversation'
---@field truncated? boolean
---@class Wraith.BufferPage: Wraith.BufferInfo
---@field start integer
---@field lines string[]
---@field next integer next line offset

---@class Wraith.BuffersAPI
wraith.buffers = wraith.buffers or {}
---@param options Wraith.BufferOptions
---@return string buffer_id
function wraith.buffers.create(options) end
---@return Wraith.BufferInfo[] metadata
function wraith.buffers.list() end
---@param slot 'prompt'|'conversation'
---@return string? buffer_id absent before the host creates its native projections
function wraith.buffers.native(slot) end
---@param id string
---@return Wraith.Buffer
function wraith.buffers.get(id) end
---@param id string
---@param start? integer default 0
---@param finish? integer exclusive; -1 means end
---@return Wraith.BufferPage
function wraith.buffers.get_lines(id, start, finish) end
-- optimistic edits raise on stale revisions; omitted expected uses the callback's observed revision.
---@param id string
---@param start integer
---@param finish integer exclusive; -1 means end
---@param lines string[]
---@param expected? integer
---@return integer revision
function wraith.buffers.set_lines(id, start, finish, lines, expected) end
---@param id string
---@param spans Wraith.Span[] replaces this caller's decoration layer
---@param expected? integer
---@return integer revision
function wraith.buffers.set_spans(id, spans, expected) end
---@param id string
---@param expected? integer
function wraith.buffers.delete(id, expected) end
---@param id string
---@param mode Wraith.KeyMode
---@param keys string
---@param target Wraith.Mapping
---@param description? string
---@param expected? integer
---@return integer revision
function wraith.buffers.set_keymap(id, mode, keys, target, description, expected) end
---@param id string
---@param mode Wraith.KeyMode
---@param keys string
---@param expected? integer
function wraith.buffers.del_keymap(id, mode, keys, expected) end

---@class Wraith.WindowOptions
---@field kind? 'split'|'float' default split
---@field position? 'left'|'right'|'above'|'below' default right
---@field relative_to? string owned tiled window id
---@field title? string
---@field width? integer
---@field height? integer
---@field row? integer
---@field column? integer
---@field focus? boolean transient focus request
---@field cursor? Wraith.Position
---@field mode? 'normal'|'insert'
---@class Wraith.WindowPatch: Wraith.WindowOptions
---@field buffer_id? string
---@class Wraith.Window
---@field id string
---@field owner string
---@field buffer_id string
---@field revision integer
---@field kind 'split'|'float'
---@field mode 'normal'|'insert'
---@field cursor Wraith.Position
---@field position? 'left'|'right'|'above'|'below'
---@field relative_to? string
---@field title? string
---@field width? integer
---@field height? integer
---@field row? integer
---@field column? integer
---@field focus? boolean stored state never replays a focus request

---@class Wraith.WindowsAPI
wraith.windows = wraith.windows or {}
---@param buffer_id string
---@param options? Wraith.WindowOptions
---@return string window_id
function wraith.windows.open(buffer_id, options) end
---@return (Wraith.NativeWindow|Wraith.Window)[] windows native windows are available in callbacks
function wraith.windows.list() end
---@param id string owned window id; native numeric ids use the snapshot
---@return Wraith.Window
function wraith.windows.get(id) end
---@param id string
---@param options Wraith.WindowPatch
---@param expected? integer
---@return integer revision
function wraith.windows.configure(id, options, expected) end
---@overload fun(id: integer)
---@param id string
---@param expected? integer
---@return integer revision owned windows only
function wraith.windows.focus(id, expected) end
---@overload fun(id?: integer)
---@param id string
---@param expected? integer
function wraith.windows.close(id, expected) end
---@param axis? 'horizontal'|'vertical' native conversation split; default horizontal
function wraith.windows.split(axis) end

---@alias Wraith.RegionSlot 'header'|'footer'|'sidebar'|'body'|'welcome'|'prompt'
---@class Wraith.RegionOptions
---@field slot Wraith.RegionSlot
---@field lines? string[]
---@field spans? Wraith.Span[]
---@field width? integer sidebar width at most 120
---@field height? integer
---@field row? integer
---@field column? integer
---@class Wraith.RegionPatch
---@field slot? Wraith.RegionSlot
---@field lines? string[]
---@field spans? Wraith.Span[]
---@field width? integer
---@field height? integer
---@field row? integer
---@field column? integer
---@class Wraith.RegionInfo
---@field id string
---@field owner string
---@field slot Wraith.RegionSlot
---@field revision integer
---@field order integer
---@field width integer
---@field height integer
---@field row integer
---@field column integer
---@field line_count integer
---@class Wraith.Region
---@field id string
---@field owner string
---@field slot Wraith.RegionSlot
---@field revision integer
---@field order integer
---@field width? integer
---@field height? integer
---@field row? integer
---@field column? integer
---@field lines string[]
---@field spans Wraith.Span[]

---@class Wraith.RegionsAPI
wraith.regions = wraith.regions or {}
---@param options Wraith.RegionOptions
---@return string region_id
function wraith.regions.create(options) end
---@return Wraith.RegionInfo[] metadata
function wraith.regions.list() end
---@param id string
---@return Wraith.Region
function wraith.regions.get(id) end
---@param id string
---@param options Wraith.RegionPatch
---@param expected? integer
---@return integer revision
function wraith.regions.set(id, options, expected) end
---@param id string
---@param expected? integer
function wraith.regions.delete(id, expected) end

---@class Wraith.DialogItem
---@field id? string defaults to the one-based item position as text
---@field label string
---@field detail? string
---@class Wraith.DialogResult
---@field id string
---@field cancelled boolean
---@field value? string selected label or accepted input
---@field index? integer one-based original selected item position
---@field item_id? string accepted selection identity
---@field reason? string cancellation reason
---@class Wraith.UIAPI
wraith.ui = wraith.ui or {}
-- lossy display conversion: replace invalid utf-8 and remove controls except newline/tab.
---@param bytes string at most 4 mib
---@return string text
function wraith.ui.decode(bytes) end
---@param items (string|Wraith.DialogItem)[] at most 1024
---@param options {title?: string}
---@param callback fun(ctx: Wraith.Context<Wraith.DialogResult>)
---@return string dialog_id
function wraith.ui.select(items, options, callback) end
---@param options {title?: string, initial?: string}
---@param callback fun(ctx: Wraith.Context<Wraith.DialogResult>)
---@return string dialog_id
function wraith.ui.input(options, callback) end
