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

`import` uses ordinary `require`. split specifications into modules below `lua/`; return a specification or a list of specifications from each module. relative local package directories resolve against the configuration directory, not the current project.

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

choose exactly one source:

| field | meaning |
| --- | --- |
| `"owner/repository"` or `repo = "owner/repository"` | GitHub repository shorthand |
| `"https://github.com/owner/repository"` or `"git@github.com:owner/repository.git"` | direct remote; also accepted in the first table position or `repo` |
| `"./packages/review"`, `"~/projects/review"` or `"/path/to/review"` | explicit local module root |
| `url = "..."` | HTTPS, SSH, SCP-style SSH, `file://` or an absolute local git repository path |
| `dir = "packages/review"` | local module root relative to the effective config directory; home expansion, absolute paths and explicit symlinks also work |

HTTP, remote helper protocols, embedded URL credentials, query strings, fragments and control characters are rejected. local directories are never installed, rewritten or deleted. they cannot select or pin git revisions.

| field | default and behavior |
| --- | --- |
| `name` | lowercase source basename without `.git`; identity across fragments, at most 64 bytes, using lowercase letters, digits, `.`, `_` and `-`, starting with a letter or digit |
| `main` | source basename without `.git`; module passed to `require` |
| `branch`, `tag`, `commit` | at most one; omitted means remote HEAD; commits require a full 40- or 64-digit hexadecimal object id |
| `pin` | `false`; keep the locked revision during update, except when an explicit commit selector changes |
| `dependencies` | package names or nested specifications; loaded before the dependent package |
| `enabled` | `true`, or a function receiving a copied specification and returning a boolean |
| `priority` | `0`; higher values activate first, with declaration order breaking ties; dependencies still load first |
| `lazy` | true when `cmd`, `event` or `keys` is present, otherwise false |
| `cmd` | one slash command name or a list of names that activate the package |
| `event` | one lifecycle/custom event name or a list that activates the package |
| `keys` | key specifications, described below |
| `init` | function receiving a copied specification; runs during manager setup, before activation |
| `opts` | nested maps merge across fragments; lists replace; a function mutates inherited options or returns their replacement |
| `opts_extend` | dotted option paths whose lists append from this declaration onward |
| `config` | function receiving `(spec, opts)`, or `false` to require the module without calling its setup function |

without a custom `config`, activation requires `main` and calls `module.setup(opts)` if that field exists. a module can also be a library with no setup function. `config = function(spec, opts) ... end` controls loading itself and may require a different module. the installer does not assume a conventional module entry point or run build scripts.

`init` and setup/config may return a cleanup function. automatic module loading also calls `module.teardown()` when present. owned registrations, views and managed resources are released by the host. arbitrary file writes, module-cache changes and other external effects cannot be rolled back.

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

`lazy = true` without a trigger means explicit `manager.load(name)` activation. unloading removes activation and init owners, including their triggers. load it explicitly again or reload the configuration to restore them. unload dependents before their dependencies.

normalization rejects unknown fields, conflicting sources or module identities, conflicting triggers, missing dependencies and dependency/import cycles before running init functions. disabled packages are excluded; an enabled package cannot depend on a disabled package. there is no semver resolver, submodule installation, Neovim plugin compatibility or build-hook support.

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

an explicit package source can include `import`; its fragment is collected first. `name` also names that import. `cond` gates the import only; a shared `enabled` callback runs separately as an import gate and as a final package predicate.

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

nested option maps merge; lists replace unless their dotted path is in `opts_extend`. its value is inherited by later fragments, not applied retroactively. `wraith.json.null` removes a map field. callbacks run on activation with detached arguments; returning a table replaces the options, while returning `nil` retains mutations. `enabled` runs once per merged plugin during normalization.

the first source stays selected. remote `.git` suffix variants match, but different transports or local directories conflict. choose one revision selector across the merged plugin; unlike lazy.nvim, a later branch does not silently coexist with an earlier commit. option recipes and callback results have a 4096-entry, depth-16 structural limit; declarations allow 256 fragments and 63 enabled plugins.

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

install and update reconcile the lock against the enabled graph. removed, disabled and newly local packages lose their git lock entries; their files remain. active unselected git entries remain. the plan lists every pruned name. restore does not prune or rewrite the lock. changing a git source under the same package name is rejected; use a separate identity or deliberately reconcile the old entry first.

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
