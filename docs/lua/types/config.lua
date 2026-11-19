---@meta

-- complete snapshots and partial setup patches have separate field contracts.
---@class Wraith.Binding
---@field buffer? string
---@field mode Wraith.KeyMode
---@field keys string
---@field action? string choose action or input
---@field input? string
---@field description? string

---@class Wraith.Execution
---@field capture_bytes integer
---@field tail_bytes integer
---@field max_jobs integer
---@field max_reads integer
---@field command_timeout string
---@field auto_share boolean
---@field mode 'agent'|'blocks'
---@field max_steps integer
---@field output_context integer

---@class Wraith.ExecutionPatch
---@field capture_bytes? integer
---@field tail_bytes? integer
---@field max_jobs? integer
---@field max_reads? integer
---@field command_timeout? string
---@field auto_share? boolean
---@field mode? 'agent'|'blocks'
---@field max_steps? integer
---@field output_context? integer

---@class Wraith.Environment
---@field scope string

---@class Wraith.EnvironmentPatch
---@field scope? string

---@class Wraith.Appearance
---@field background string
---@field mouse boolean
---@field thinking boolean

---@class Wraith.AppearancePatch
---@field background? string
---@field mouse? boolean
---@field thinking? boolean

---@class Wraith.Config
---@field keymaps Wraith.Binding[]
---@field key_timeout integer
---@field environment Wraith.Environment
---@field provider string
---@field providers table<string, Wraith.Provider>
---@field clipboard string[]
---@field appearance Wraith.Appearance
---@field execution Wraith.Execution
---@field themes table<string, Wraith.Palette>
---@field leader string
---@field bindings table<string, string>
---@field auto_start boolean
---@field version integer
---@field host string
---@field model string
---@field context integer
---@field predict integer
---@field threads integer
---@field temperature number
---@field thinking string
---@field theme string
---@field editor string[]
---@field terminal Wraith.Terminal
---@field shell string[]
---@field keep_alive string
---@field timeout string
---@field compact Wraith.Compact
---@field keys table<string, string>
---@field animations boolean

---@class Wraith.ConfigPatch
---@field keymaps? Wraith.Binding[]
---@field key_timeout? integer
---@field environment? Wraith.EnvironmentPatch
---@field provider? string
---@field providers? table<string, Wraith.Provider>
---@field clipboard? string[]
---@field appearance? Wraith.AppearancePatch
---@field execution? Wraith.ExecutionPatch
---@field themes? table<string, Wraith.Palette>
---@field leader? string
---@field bindings? table<string, string>
---@field auto_start? boolean
---@field version? integer
---@field host? string
---@field model? string
---@field context? integer
---@field predict? integer
---@field threads? integer
---@field temperature? number
---@field thinking? string
---@field theme? string
---@field editor? string[]
---@field terminal? Wraith.TerminalPatch
---@field shell? string[]
---@field keep_alive? string
---@field timeout? string
---@field compact? Wraith.CompactPatch
---@field keys? table<string, string>
---@field animations? boolean

---@class Wraith.Compact
---@field mode string
---@field threshold number
---@field keep_turns integer
---@field budget integer

---@class Wraith.CompactPatch
---@field mode? string
---@field threshold? number
---@field keep_turns? integer
---@field budget? integer

---@class Wraith.Terminal
---@field background string
---@field foreground string
---@field overlay_width number
---@field overlay_height number
---@field split_width number
---@field scrollback integer
---@field directory string

---@class Wraith.TerminalPatch
---@field background? string
---@field foreground? string
---@field overlay_width? number
---@field overlay_height? number
---@field split_width? number
---@field scrollback? integer
---@field directory? string

---@class Wraith.Provider
---@field context? integer
---@field model_contexts? table<string, integer>
---@field kind? string
---@field action_protocol? string
---@field base_url? string
---@field api_key? string
---@field key_env? string
---@field headers? table<string, string>
---@field model? string
---@field models? string[]
---@field command? string[]
---@field system_prompt? string
---@field thinking? string[]
---@field thinking_mode? string
---@field token_field? string
---@field extra? table<string, Wraith.JSON>

---@class Wraith.Palette
---@field hint_quit? string
---@field hint_refresh? string
---@field hint_cancel? string
---@field variable? string
---@field bg string
---@field fg string
---@field muted string
---@field accent string
---@field second string
---@field border string
---@field warn string

wraith = wraith or {}
---@class Wraith.ConfigAPI
wraith.config = wraith.config or {}
-- no path returns the complete detached settings; a path selects an existing value.
---@overload fun(): Wraith.Config
---@param path string
---@return Wraith.JSON value
function wraith.config.get(path) end
---@param path string
---@param value Wraith.JSON
function wraith.config.set(path, value) end
---@param settings Wraith.ConfigPatch
function wraith.config.setup(settings) end

---@class Wraith.ThemeAPI
wraith.theme = wraith.theme or {}
---@param name string
---@param palette Wraith.Palette
function wraith.theme.register(name, palette) end
---@param name string
function wraith.theme.use(name) end
