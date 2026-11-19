---@meta

---@alias Wraith.PackageAction 'install'|'update'|'restore'
---@alias Wraith.PackageSource {kind: 'local', dir: string}|{kind: 'git', url: string}
---@alias Wraith.PackageSelector {kind: 'head'}|{kind: 'branch'|'tag'|'commit', value: string}
---@alias Wraith.PackageDeclarations string|Wraith.PackageSpec|Wraith.PackageImport|Wraith.PackageDeclarations[]
---@alias Wraith.PackageImportSource string|fun(): table
---@class Wraith.PackageImport
---@field import Wraith.PackageImportSource
---@field name? string required for function imports; optional alias for module imports
---@field enabled? boolean|fun(): boolean
---@field cond? boolean|fun(): boolean
---@alias Wraith.PackageCleanup fun()
---@class Wraith.PackageKey
---@field [1]? string shorthand for keys
---@field [2]? string|fun(ctx: Wraith.Context) shorthand for action
---@field keys? string
---@field action? string|fun(ctx: Wraith.Context) explicit first-use action
---@field mode? Wraith.KeyMode defaults to chat.normal
---@field desc? string shorthand for description
---@field description? string
---@class Wraith.PackageSpec
---@field [1]? string repository shorthand, remote url, local path or existing package name
---@field repo? string same source forms as the first position; choose one source
---@field url? string explicit https, ssh, scp-style or file remote
---@field dir? string absolute, home-relative or relative to the effective config directory
---@field name? string defaults to lowercase repository/directory basename
---@field main? string explicit entrypoint; otherwise infer from the selected plugin's lua directory
---@field dependencies? Wraith.PackageDeclarations bare dependency names refer to existing declarations
---@field enabled? boolean|fun(spec: Wraith.PackageSpec): boolean
---@field optional? boolean qualifies only with a surviving nonoptional declaration
---@field specs? Wraith.PackageDeclarations scoped declarations without dependency edges
---@field priority? integer default 50, within -1000000..1000000
---@field lazy? boolean defaults to dependency-only declarations or command, event or key triggers
---@field cmd? string|string[]
---@field event? Wraith.Event|Wraith.Event[]
---@field keys? Wraith.PackageKey[]
---@field init? fun(spec: Wraith.NormalizedPackage): Wraith.PackageCleanup?
---@field opts_extend? string[] dotted list paths, inherited by later fragments
---@field opts? table|fun(spec: Wraith.NormalizedPackage, opts: table): table? nil retains in-place changes
---@field config? boolean|fun(spec: Wraith.NormalizedPackage, opts: table): Wraith.PackageCleanup? false skips configuration only without opts
---@field branch? string choose one branch, tag or commit
---@field tag? string
---@field commit? string full lowercase commit id
---@field pin? boolean
---@field import? Wraith.PackageImportSource collects specs after this explicit source declaration
---@field cond? boolean|fun(): boolean applies to the import only
---@class Wraith.NormalizedPackage
---@field name string
---@field source Wraith.PackageSource
---@field selector Wraith.PackageSelector
---@field pin boolean
---@field main? string declared entrypoint, remains nil when inferred
---@field dependencies string[]
---@field enabled boolean
---@field priority integer
---@field lazy boolean
---@field cmd string[]
---@field event Wraith.Event[]
---@field keys {keys: string, action: string|fun(ctx: Wraith.Context), mode: Wraith.KeyMode, description: string}[]
---@field init? fun(spec: Wraith.NormalizedPackage): Wraith.PackageCleanup?
---@field opts_extend? string[] dotted list paths, inherited by later fragments
---@field opts? table|fun(spec: Wraith.NormalizedPackage, opts: table): table?
---@field config? boolean|fun(spec: Wraith.NormalizedPackage, opts: table): Wraith.PackageCleanup?

---@class Wraith.PackageStorageOptions
---@field root? string absolute; defaults to stdpath('data') .. '/packages'
---@field lockfile? string absolute; defaults to stdpath('config') .. '/wraith-lock.json'
---@field timeout_ms? integer default 300000, 1..86400000
---@field lock_timeout_ms? integer default 5000, 1..300000
---@field git_env? {GIT_ASKPASS?: string, GIT_SSH?: string, GIT_SSH_COMMAND?: string, SSH_ASKPASS?: string}
---@class Wraith.PackageViewKeys
---@field refresh? string|false default r
---@field install? string|false default i
---@field update? string|false default u
---@field restore? string|false default s
---@field cancel? string|false default x
---@field close? string|false default q
---@field inspect? string|false default <Enter>
---@field load? string|false default l
---@field unload? string|false default d
---@field log? string|false default o
---@field confirm? string default <Enter> in the separate plan pane; cannot be disabled
---@class Wraith.PackageViewOptions
---@field position? 'left'|'right'|'above'|'below'
---@field keys? Wraith.PackageViewKeys
---@class Wraith.PackageManagerOptions
---@field storage? Wraith.PackageStorageOptions
---@field ui? Wraith.PackageViewOptions

---@class Wraith.PackageStatus
---@field name string
---@field source? Wraith.PackageSource
---@field selector? Wraith.PackageSelector
---@field pin? boolean
---@field commit? string currently selected installed commit
---@field path? string usable local or installed directory
---@field state 'local'|'installed'|'missing'|'source-conflict'|'unloaded'|'excluded'
---@field error? string
---@field loaded boolean
---@field managed boolean init wrapper is present
---@field installed? boolean
---@field lazy? boolean
---@field loaded_commit? string commit chosen by the active runtime
---@field requested_main? string explicitly declared entrypoint
---@field selected_main? string bound entrypoint, retained after unload or failure
---@field loaded_main? string entrypoint of active automatic setup
---@field main_source_kind? 'file'|'preload' a preload's code is independent of the checkout commit
---@field main_source_path? string selected file path
---@field restart_required? boolean disk selection differs from this runtime's frozen source
---@field exclusion? 'disabled'|'optional'|'scope'
---@field retained_commit? string existing lock pin; does not imply an installed checkout
---@field origins? {index: integer, module?: string, path?: string}[] surviving declaration origins before exclusion
---@class Wraith.PackageOperation
---@field id string
---@field state 'preparing'|'fetching'|'verifying'|'publishing'|'cleaning'|'installed'|'failed'|'cancelled'|'conflict'|'uncertain'
---@field phase string latest progress phase; retained after completion
---@field restart_required string[]
---@field name? string package being processed
---@field message? string
---@field error? string
---@field code? integer child exit code
---@field generation? string selected lock generation
---@field log? string at most 8 kib of stderr
---@field log_truncated? boolean
---@field cleanup_warning? string publication succeeded; private cleanup needs inspection
---@class Wraith.PackageManagerStatus
---@field packages Wraith.PackageStatus[]
---@field operation? Wraith.PackageOperation
---@class Wraith.PlannedPackage
---@field name string
---@field source Wraith.PackageSource
---@field selector Wraith.PackageSelector
---@field pin boolean
---@field state 'local'|'locked'|'resolve'
---@field path? string local path
---@field commit? string selected exact commit when already known
---@field previous? string previous locked commit
---@class Wraith.PackagePlan
---@field id string frozen single-use plan handle; only 16 most recent plans are retained
---@field summary {id: string, action: Wraith.PackageAction, packages: Wraith.PlannedPackage[], pruned: string[]}
---@class Wraith.PackageRecovery
---@field root string
---@field lock_directory string
---@field staging string[] first bounded directory page, not a stale-file judgment
---@field publication string[] candidate/baseline filenames in the bounded directory page
---@field truncated {staging: boolean, publication: boolean}
---@field errors {path: string, error: string}[]

---@class Wraith.PackageManager
local manager = {}

-- validates the graph before init; at most 63 enabled specs and available lifecycle capacity.
-- setup reads installed state without fetching. repeated setup requires unloading the manager first.
---@param specs Wraith.PackageDeclarations
---@param options? Wraith.PackageManagerOptions
---@return Wraith.PackageManager
function manager.setup(specs, options) end

-- reconciles abandoned work and pending cleanup receipts; no activation or network access.
---@return Wraith.PackageManagerStatus
function manager.status() end
---@param name string
function manager.load(name) end
-- removes activation and init wrappers; active dependents prevent unloading.
---@param name string
function manager.unload(name) end
-- cancels installation and removes managed wrappers in reverse dependency order.
function manager.unload_all() end

---@param action Wraith.PackageAction
---@param names? string[] omitted selects all configured packages and dependencies
---@return Wraith.PackagePlan
function manager.plan(action, names) end
-- executes the exact frozen plan without another confirmation; one active operation.
-- at most 16 completed cleanup receipts may await host acceptance before execution pauses.
---@param plan_id string
---@return string operation_id
function manager.execute(plan_id) end
---@param operation_id? string current operation when omitted
---@return boolean cancellation_requested
function manager.cancel(operation_id) end
---@return Wraith.PackageRecovery
function manager.recover() end

-- direct operations prepare and execute without a confirmation dialog.
---@param names? string|string[]
---@return string operation_id
function manager.install(names) end
---@param names? string|string[]
---@return string operation_id
function manager.update(names) end
---@param names? string|string[]
---@return string operation_id
function manager.restore(names) end
-- the package pane previews frozen install/update/restore plans before confirmation.
function manager.open() end

return manager
