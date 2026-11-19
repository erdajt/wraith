# packages

wraith ships Shroud, a package manager written in lua from a separate repository. it loads local modules, installs pinned git checkouts, resolves dependencies and activates packages on commands, events or keys. it uses the same [public APIs](../plugins.md) available to your plugins.

startup installs the bundled Shroud revision into a verified local checkout before reading your configuration. it requires git and works offline. Shroud then reads your configuration and lock file without fetching or updating plugins. plugin installation and updates require an explicit action. see [distribution maintenance](../distribution.md). packages run as trusted code with your user account's permissions; installing a package does not sandbox it.

## start with a local package

copy the contents of [this complete configuration](../examples/lua/packages/) into a separate wraith configuration directory. it contains:

```text
init.lua
lua/packages/init.lua
lua/packages/review.lua
packages/review/lua/review/init.lua
```

run `wraith --config-dir /absolute/path/to/that/directory`, then enter `/review`. the manager loads the local module and prepares a review prompt. the example is executed in the test suite, including unload and reload of its command.

its entry point is:

```lua
require("wraith.manager").setup({
  { import = "packages" },
})
```

`import` discovers and evaluates specification modules below `lua/`; return a specification or a list of specifications from each module. see [imports](#imports) for discovery and cache rules. relative local package directories resolve against the configuration directory, not the current project.

for a remote package, replace the local source with a repository you trust:

```lua
require("wraith.manager").setup({
  {
    url = "https://github.com/your-account/wraith-review.git",
    name = "review",
    main = "review",
    branch = "main",
    cmd = "review",
    opts = { prompt = "review the current changes" },
  },
})
```

this URL illustrates the format; it is not a published package. open `/packages`, select the package with vim motions, then press `i`. inspect the plan, press enter and confirm the selection. after installation, `/reload` creates a runtime using the installed revision. `/review` then activates it.

## specification

choose a repository or local directory. a repository declaration may also specify `dir` to use a local checkout:

| field | meaning |
| --- | --- |
| `"owner/repository"` or `repo = "owner/repository"` | GitHub repository shorthand |
| `"https://github.com/owner/repository"` or `"git@github.com:owner/repository.git"` | direct remote; also accepted in the first table position or `repo` |
| `"./packages/review"`, `"~/projects/review"` or `"/path/to/review"` | explicit local module root |
| `url = "..."` | HTTPS, SSH, SCP-style SSH, `file://` or an absolute local git repository path |
| `dir = "packages/review"` | local module root relative to the effective config directory; home expansion, absolute paths and explicit symlinks also work |

HTTP, remote helper protocols, embedded URL credentials, query strings, fragments and control characters are rejected. local directories are never installed, rewritten or deleted. a standalone local package cannot select or pin git revisions. a repository with a local override retains its declared revision settings but uses an unpinned local source operationally.

```lua
{ "owner/review", branch = "work", dir = "~/projects/review" }
```

this keeps the repository identity `review`, even if the directory has a different basename. remove `dir` to use the declared Git source again. `{ name = "review", dir = "..." }` and `{ "review", dir = "..." }` fragments can supply the override before or after the repository declaration. a positional local path cannot also specify `dir`.

normalized callback specifications expose `source`, `selector` and `pin` for the effective checkout, plus `declared_source`, `declared_selector` and `declared_pin` for the original intent. those declared fields do not enter the installation descriptor or lock file. changing an exposed source still requires `/reload`.

| field | default and behavior |
| --- | --- |
| `name` | lowercase source basename without `.git`; identity across fragments, at most 64 bytes, using lowercase letters, digits, `.`, `_` and `-`, starting with a letter or digit |
| `main` | explicit setup module; otherwise infer from the plugin's `lua/` entrypoints |
| `branch`, `tag`, `commit` | at most one; omitted means remote HEAD; commits require a full 40- or 64-digit hexadecimal object id |
| `pin` | `false`; keep the locked revision during update, except when an explicit commit selector changes |
| `dependencies` | package names or nested specifications; loaded before the dependent package |
| `enabled` | `true`, or a function receiving a copied specification and returning a boolean |
| `cond` | boolean or function receiving the initial merged specification; false skips activation and preserves existing dependency pins |
| `optional` | `false`; keep this contribution only when a surviving nonoptional declaration names the same plugin |
| `specs` | declarations owned by this fragment, without dependency edges or dependency-only loading defaults |
| `priority` | `50`; higher values start first, with declaration order breaking ties; dependencies still load before their parent |
| `lazy` | true for dependency-only declarations, `defaults.lazy = true`, or command, event or key triggers; explicit booleans override inference |
| `cmd` | one slash command name or a list of names that activate the package |
| `event` | one lifecycle/custom event name or a list that activates the package |
| `keys` | key specifications, described below |
| `init` | function receiving a copied specification; runs during manager setup, before activation |
| `opts` | nested maps merge across fragments; lists replace; a function mutates inherited options or returns their replacement |
| `opts_extend` | dotted option paths whose lists append from this declaration onward |
| `config` | function receiving `(spec, opts)`, or `true` for automatic setup; `false` skips configuration only when `opts` is absent |

`opts` or `config = true` requests automatic setup. the entry module must return a table with callable `setup`; missing setup is an error. `config = false` with options still requests setup, matching lazy.nvim. without options or a truthy config, activation exposes the runtime path without requiring a module. `main` and `opts_extend` alone do not request setup. `config = function(spec, opts) ... end` controls loading itself and may require a different module.

`init` and setup/config may return a cleanup function. automatic setup also calls `module.teardown()` when present; custom config does not gain an implicit module teardown. failed setup rejects activation and rolls back owned registrations, unlike lazy.nvim’s logged setup errors. lazy triggers remain available for retry. arbitrary file writes, module-cache changes and other external effects cannot be rolled back.

automatic setup selects its entrypoint from the installed plugin. an explicit `main` wins; otherwise one normalized name match or a sole candidate is required. normalization ignores case and separators, strips `wraith-`, `.wraith`, `.lua` and `-lua` affixes, and keeps digits. ambiguous or missing entrypoints require `main` or custom `config`. explicit names support legacy root files as well as `lua/` layout. inferred names use `lua/` only.

inferred entrypoints cannot fall back to global modules. explicit `main` may deliberately select a preload override. existing foreign cache values are rejected, and source changes after exposure require `/reload`. this binds automatic setup's entrypoint, not every submodule or custom config's require calls. callbacks keep the declared `main`; status reports `requested_main`, `selected_main`, `loaded_main`, `main_source_kind` and `main_source_path`. a synthetic preload is labeled separately from the installed checkout commit.

ordinary source failures may retry with the same binding; setup failures reuse the owned module. a source that changes its cache/preload and fails, or whose load times out, requires `/reload`. interrupted bookkeeping never silently adopts retained cache values. managed load errors include source traces as diagnostics. package/cache/preload table replacement before manager loading is unsupported; later replacement is rejected. use `require` rather than calling private managed preload wrappers directly.

```lua
{
  dir = "packages/review",
  name = "review",
  main = "review",
  dependencies = { "notes" },
  keys = {
    { "<Space>r", "command.review", mode = "chat.normal", desc = "review changes" },
  },
  opts = function(_, opts)
    opts.prompt = "review the current changes"
  end,
}
```

key entries accept positional `{sequence, action}` or named `{keys = sequence, action = action}` fields. `mode` defaults to `chat.normal`; `description` and `desc` are aliases. the action is a registered action name or a function. canonical key aliases are checked for conflicts.

command triggers activate the package and replay that command with its original argument. the package must register the promised command. key triggers run their declared action after activation; they do not replay a physical key recursively. event triggers deliver the original event only to the newly activated packages' listeners. existing listeners do not receive a duplicate event. a package activated after startup also receives `ready` through its newly registered ready listeners.

manager `defaults.lazy` accepts a boolean and defaults to false. explicit package `lazy = false` requests startup even with global lazy loading, dependency status or triggers. `defaults.lazy = false` preserves ordinary dependency/trigger inference. gate callbacks see declared fields before this final inference.

a separate top-level declaration removes the dependency-only default. completed import replays do not add declarations or change that default. eager startup order is independent of lazy plugins’ priorities.

`lazy = true` without a trigger means explicit `manager.load(name)` activation. unloading removes activation and init owners, including their triggers. load it explicitly again or reload the configuration to restore them. unload dependents before their dependencies.

normalization rejects unknown fields, conflicting sources or module identities, conflicting triggers, missing dependencies and dependency/import cycles before running init functions. disabled packages are excluded and references to them are removed from surviving dependency lists. genuinely unknown dependencies still fail. there is no semver resolver, submodule installation, Neovim plugin compatibility or build-hook support.

## scoped specifications

```lua
{
  "owner/workspace.wraith",
  enabled = false,
  specs = {
    { "owner/history.wraith", optional = true, opts = { limit = 200 } },
  },
}
```

removing a parent removes its owned declarations and their contributions. an independently declared child survives with its remaining settings. `specs` does not make a child a dependency; use `dependencies` when activation order matters. scoped self-overrides are allowed. cached import replay never re-parents declarations or rescues a removed scope.

`optional = true` applies only while every surviving actual declaration is optional. a nonoptional source declaration, name-only override or full dependency declaration qualifies the plugin. a bare dependency reference does not. optional-only declarations can omit a source; surviving plugins must have one. declarations are collected before pruning, so trusted import code can run even when its parent is later excluded.

source conflicts and malformed declarations fail before pruning. final dependency, main and trigger conflicts are checked among survivors. exclusions cannot be bypassed by manual loading or installation: change the configuration and reload. package status reports `disabled`, `conditional`, `optional` or `scope`, with a `retained pin` only when an eligible existing lock entry remains. that pin does not claim an installed checkout or loaded code.

## imports

`{ import = "plugins" }` discovers the module file, directory `init.lua`, immediate `.lua` files and immediate child directories with `init.lua`, then loads them in module-name order. each must return exactly one table of specs. deeper directories need another import. see [module discovery](modules.md) for source precedence and limits.

```lua
require("shroud").setup({
  { import = "plugins" },
  { name = "local tools", import = function()
    return { { dir = "packages/review", opts = {} } }
  end },
})
```

function imports require a name. string imports can use an alias. `cond` and `enabled` accept booleans or zero-argument functions returning a boolean. false skips loading. completed import names skip repeated gates and reuse their top-level package contributions, preserving dependency edges for each caller. distinct names may evaluate the same file again, including overlapping directory and child imports, as in lazy.nvim.

imports load sources afresh and clear their own require-cache entries. conflicting names, recursive imports, ambiguous file/init sources and extra returns raise errors. each normalization allows 128 import identities and 256 source evaluations. arbitrary effects from trusted import code are not rolled back on failure.

an explicit package source can include `import`; its fragment is collected first. `name` also names that import. `cond` and `enabled` participate in both contexts. their import calls receive no arguments; their package calls receive a copied specification. manager defaults apply to packages, not import gates.

## conditional activation

```lua
require("shroud").setup({
  { "owner/review.wraith", cond = function()
    return os.getenv("WRAITH_REVIEW") == "1"
  end },
}, { defaults = { cond = true } })
```

an explicit `cond` overrides `defaults.cond`. conditions run once in declaration order, before disabled/optional pruning, with detached initial merged fields and original dependency names. false prevents the package's `enabled` callback and activation. that decision remains fixed if later pruning changes the plugin's contributions. malformed defaults fail before import code runs; unknown defaults are rejected.

false conditions preserve existing pins for the original declared dependency closure, including cycles later excluded from the active graph. scoped children are not dependencies unless explicitly referenced. unknown references never protect unrelated lock entries. a conditional child removed with its parent still preserves this closure, even when status labels the removal `scope`. independently active packages keep their normal source and revision rules. there is no installed-object garbage collector yet; conditional preservation does not claim a usable checkout.

## repeated declarations

use a bare name to override a plugin declared elsewhere. the source may appear before or after the override:

```lua
require("shroud").setup({
  { "owner/review", opts = { window = { width = 80 }, sources = { "history" } } },
  { "review", opts_extend = { "sources" }, opts = { sources = { "commands" } } },
  { name = "review", opts = function(_, opts) opts.window.height = 24 end },
})
```

later scalar fields replace earlier values, including `false`; absent values remain inherited. dependencies, commands and events combine in first-seen order. key mappings replace the same normalized key and mode across fragments. aliases duplicated inside a single fragment remain invalid.

nested option maps merge; lists replace unless their dotted path is in `opts_extend`. its value is inherited by later fragments, not applied retroactively. `wraith.json.null` removes a map field. callbacks run on activation with detached arguments; returning a table replaces the options, while returning `nil` retains mutations. `enabled` runs during normalization, once per effective contribution and dependency revision. pruning can change that revision; unchanged predicates do not run again. callbacks receive detached merged fields and effective dependency names, without recursive `specs` declarations. normalization stops after 1,024 combined `cond` and `enabled` predicate calls.

the first surviving repository stays selected. remote `.git` suffix variants match, but different repositories or transports conflict. the last surviving `dir` overrides the checkout; removing its scope restores the previous directory or repository. a removed repository cannot supply metadata to a surviving local declaration. choose one revision selector across the merged plugin; unlike lazy.nvim, a later branch does not silently coexist with an earlier commit. merged selectors, trigger counts and option bounds are validated before evaluating gates for that revision. option recipes and callback results have a 4096-entry, depth-16 structural limit; declarations allow 256 fragments and 63 enabled plugins.

## package view

`/packages` opens a reusable buffer. ordinary vim navigation, search, selection and yank apply. closing the view does not cancel an installation.

| key | action |
| --- | --- |
| `r` | refresh status |
| `i` | plan installation of the selected package and dependencies |
| `u` | plan an update |
| `s` | plan restoration of locked revisions |
| `l` / `d` | load / unload the selected package |
| enter | inspect a package; in a plan, open its confirmation picker |
| `o` | inspect the current operation and its bounded stderr excerpt |
| `x` | request cancellation |
| `q` | close the view |

```lua
require("wraith.manager").setup({ { import = "packages" } }, {
  ui = {
    position = "below",
    keys = { refresh = "R", update = "U", log = "L", inspect = "<Enter>" },
  },
})
```

positions are `right`, `left`, `above` and `below`. override any action in the table above, plus `confirm`. use `false` to disable a binding except confirmation. confirmation must remain distinct from close; other active bindings cannot collide.

`/packages.install NAME`, `/packages.update NAME` and `/packages.restore NAME` execute explicit operations directly. `/packages.load NAME`, `/packages.unload NAME` and `/packages.cancel` control runtime state. use the view when you want to inspect a plan before applying it.

## plans and lua control

```lua
local manager = require("wraith.manager")

wraith.commands.create("packages.review-update", function()
  local plan = manager.plan("update", { "review" })
  wraith.ui.select({ "apply", "cancel" }, { title = "update review and its dependencies" }, function(ctx)
    if not ctx.data.cancelled and ctx.data.index == 1 then
      manager.execute(plan.id)
    end
  end)
end)
```

| method | result |
| --- | --- |
| `setup(specs, options)` | configure once and return the manager module |
| `status()` | detached `{packages, operation}`; operation is absent before the first operation |
| `plan(action, names)` | `{id, summary}` without filesystem mutation or network access |
| `execute(plan_id)` | start the frozen plan and return an operation id |
| `install(names)`, `update(names)`, `restore(names)` | plan and execute; names may be one string, a list or omitted for the enabled graph |
| `cancel(id)` | request cancellation; omitted id means current; false when already terminal |
| `load(name)`, `unload(name)`, `unload_all()` | change active package owners |
| `open()` | open or focus the package view |
| `recover()` | inspect bounded leftover names without modifying them |

`plan` accepts a list of names or no list. its summary contains `action`, `packages` and `pruned`. package rows describe source, selector, pin, prior revision and either a known commit or a revision to resolve. dependencies are included. mutating the summary does not change the private executable plan. the oldest plan expires after 16 retained plans; a plan is consumed once execution starts.

| action | revision selection |
| --- | --- |
| install | reuse the locked commit when the selector is unchanged; use update to change a selector |
| update | resolve unpinned refs; preserve pinned commits unless an explicit commit selector changes |
| restore | fetch exact locked commits; preserve the lock bytes, selector and pin metadata |

install and update reconcile the lock against the enabled graph. removed, optional-only and scope-removed packages lose their git lock entries unless a false condition protects them. newly local packages always lose previous git pins; their files remain. explicitly disabled nonoptional packages retain existing pins without installing or fetching them. false conditions also preserve the original declared dependency closure, including optional roots, without turning those packages into install targets. retention never creates a missing lock entry. active unselected git entries remain. the plan lists every pruned name. restore does not prune or rewrite the lock. changing a git source under the same package name is rejected; use a separate identity or deliberately reconcile the old entry first.

## storage and credentials

```lua
require("wraith.manager").setup({ { import = "packages" } }, {
  storage = {
    root = wraith.paths.stdpath("data") .. "/packages",
    lockfile = wraith.paths.stdpath("config") .. "/wraith-lock.json",
    timeout_ms = 300000,
    lock_timeout_ms = 5000,
    git_env = { GIT_SSH_COMMAND = "ssh -o BatchMode=yes" },
  },
})
```

these are the default paths and timeouts. `root` and `lockfile` must be absolute paths without control characters, `;` or `?`. the lock file's parent must already exist and be readable. setup does not create configuration ancestry. effective data/config paths honor the CLI and XDG settings described in [module paths](../plugins.md#module-paths).

`timeout_ms` is a per-process timeout from 1 to 86,400,000 milliseconds. it is not a deadline for the entire dependency graph. `lock_timeout_ms` ranges from 1 to 300,000 milliseconds. unknown options fail validation.

`git_env` accepts only `GIT_ASKPASS`, `GIT_SSH`, `GIT_SSH_COMMAND` and `SSH_ASKPASS`. authentication helpers are trusted programs. credentials are not stored in the lock. git receives a controlled environment, including selected SSH-agent, proxy and certificate settings; interactive terminal prompts, inherited global/system git configuration, hooks, fsmonitor and recursive submodules are disabled. commands outside an owned checkout do not discover the project's git configuration.

explicit mutations require git, `flock`, `/bin/sh`, `cmp`, `mv` and `mktemp`. missing tools are reported before staging writes. wraith does not install them automatically.

## publication and cancellation

checkouts live at `root/objects/NAME/COMMIT`. each completed checkout is immutable to the manager. reuse verifies its source, exact commit and cleanliness, including untracked and ignored files. dirty or mismatched copies cause an error; there is no reset or clean step that discards your changes.

all requested packages are prepared before publication. a fixed child process holds `flock` while comparing the original lock bytes and atomically renaming the candidate lock. a concurrent update produces a conflict instead of overwriting its graph. the guard inode is retained between operations. restore performs the same final comparison without rewriting the lock.

the lock selects the graph in one atomic rename. this guarantees visibility, not power-loss durability. old immutable objects remain for running runtimes and rollback inspection; automatic garbage collection is not implemented. an installed update takes effect in a fresh runtime after `/reload`. an already exposed module cannot switch source or commit inside the same VM.

canceling stops managed work and waits for process reaping before deciding its outcome. if publication already won, status remains `installed`. if its outcome cannot be proved, status is `uncertain`; inspect the lock and recovery report before creating another plan. there is no automatic retry of an uncertain publication.

after confirmed publication, cleanup removes only that operation's private staging directory and baseline, plus the unused restore candidate. cleanup errors or cancellation preserve the installed generation and add `cleanup_warning`. conflict, failure and uncertain publication preserve private artifacts for inspection. cleanup never deletes immutable objects or the lock guard.

## status and recovery

package rows include source, selected path, installed commit, loaded commit, `loaded`, `managed`, `pin` and `restart_required`. `installed` means the selected directory exists; it is not a fresh integrity check. mutation and reuse perform git verification.

operation status includes `id`, `state`, `phase`, optional package `name`, `message`, `error`, `generation`, `restart_required` names and `cleanup_warning`. active phases describe preparation, fetching, verification, publication and cleanup. terminal states are `installed`, `failed`, `cancelled`, `conflict` and `uncertain`. the stderr excerpt is capped at 8 kib; the view displays at most 256 lines and marks truncation.

`recover()` returns `root`, `lock_directory`, `staging`, `publication`, `truncated` and `errors`. it scans at most 1,024 entries in each directory. publication results contain candidate and baseline names, not lock contents. this is an inspection report, not proof that an artifact is unused. it never deletes files, breaks locks or guesses whether another process is alive.

completed resource receipts stay tracked until the host accepts their release. a rejected callback can be reconciled by a later status check. if 16 completed operations still await accepted cleanup, a new operation fails before consuming its plan; accept cleanup and retry. committed installation status is preserved through this reconciliation.

## limits and boundaries

at most 63 enabled specifications, including dependencies, fit the manager graph. each uses separate init and activation owners; together with the manager and other manually loaded plugins they must fit the host's 128-plugin limit. normalization allows 256 declarations, depth 16 and 1,024 walked entries. each command, event or key trigger list is capped at 64. lock files are limited to 1 mib.

one installation operation runs at a time in each manager. independent processes serialize publication through the same guard. lua callbacks retain the [runtime deadlines](../plugins.md#runtime-boundaries); git and filesystem work use [managed resources](services.md#resource-lifecycle). plugins can bypass these APIs through trusted Lua libraries, so this is a lifecycle boundary, not a security boundary.

see [plugin authoring](../plugins.md#events-and-plugin-lifecycle), [editor completion](editor.md), [buffer APIs](surfaces.md) and [tested extension examples](examples.md) for the modules installed by this manager.
