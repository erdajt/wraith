# example packages

[Lua configuration](../plugins.md) · [workspace queries](workspace.md) · [buffers and views](surfaces.md) · [managed services](services.md) · [hooks, providers and tools](extensions.md)

these packages exercise the same public APIs available to your own plugins. their tests load the shipped Lua files, use real host callbacks and verify unload behavior. the git example runs `git` against a temporary repository in tests. no network or model account is required.

## install

copy the selected files from [`examples/plugins`](../../examples/plugins) into `~/.config/wraith/lua/plugins/`. load them from `init.lua`:

```lua
wraith.plugins.load("plugins.tasks")
wraith.plugins.load("plugins.files")
wraith.plugins.load("plugins.history", { max_records = 200 })
wraith.plugins.load("plugins.scope", {
  path = wraith.paths.stdpath("config") .. "/scope.json",
})
```

check [standard paths](../plugins.md#module-paths) for the configured root. plugins use ordinary `require` lookup. unload with `wraith.plugins.unload("plugins.tasks")`; reload by loading it again. unloading removes owned commands, hooks, views and pending callbacks. only load code you trust; plugins have Lua filesystem and process access.

## task context

[`tasks.lua`](../../examples/plugins/tasks.lua) adds `/task`. choose a task with the native searchable picker. the selected task appears in an owned header region and prepends a user context message to future chat requests through `before_request`.

```lua
wraith.plugins.load("plugins.tasks", {
  tasks = {
    { id = "review", label = "review changes",
      prompt = "review the changes, identify defects and cite the affected files" },
    { id = "docs", label = "document behavior",
      prompt = "document public behavior, configuration and failure handling" },
  },
})
```

`tasks.selected` reports the selected task ID. unload removes the header and hook. the package keeps its selection in memory; it does not modify saved conversations or system instructions.

## command history

[`history.lua`](../../examples/plugins/history.lua) adds `/command-history`. it pages native command metadata into a Vim buffer, sorts loaded records by start time and refreshes when native job metadata changes.

| input | behavior |
| --- | --- |
| `j` / `k`, arrows | move through records |
| `/` | search the loaded view |
| `enter` | open the selected command's local output |
| `r` | refresh history |
| `:q` | close the current split |

reopen `/command-history` to restore its view. output uses a reusable split. these mappings are buffer-local. split copies retain independent cursors; Enter selects the record under the focused cursor.

list commands use short UTF-8 previews with `...` when clipped. output detail retains up to 64 KiB of command text and marks clipping.

`max_records` defaults to 200 and accepts integers from 1 through 1000. this limits loaded records, not the underlying history. a limited view is not guaranteed to contain the globally newest commands because source pages follow directory enumeration. active jobs are merged by ID. stopped scans release their continuation instead of retaining an open directory.

output is local and may include captures hidden from the model. opening it does not grant sharing or add model evidence. the view displays a bounded output page and its next line when more is available; [workspace queries](workspace.md#commands-and-output) expose search and further pages for larger viewers.

`history.loaded` reports its buffer and loaded count after pending refreshes settle. `history.output` reports the selected job and output buffer. read failures remain visible in the view.

## changed files

[`files.lua`](../../examples/plugins/files.lua) adds `/changed-files`. it runs `git status --porcelain=v1 -z` through the managed process API and renders the result in a searchable split. native job changes refresh an existing view; an explicit command opens or focuses it.

```lua
wraith.plugins.load("plugins.files", { directory = "/absolute/project/path" })
```

without `directory`, the package uses the current session directory. process output is capped at 64 KiB and the timeout is ten seconds. the view reports command failure or truncation. `files.refreshed` reports its buffer ID. closed views reopen without accumulating buffers.

this shows the current Git working tree. it does not claim that a particular model or command caused each change. rename records preserve Git's two-entry porcelain representation. repositories without Git metadata report Git's error.

## native command scope

[`scope.lua`](../../examples/plugins/scope.lua) adds `/scope` and `/scope-reload`. its JSON policy lists exact native command strings and an execution directory:

```json
{
  "directory": "/absolute/project/path",
  "shell": "/bin/sh",
  "commands": ["git status --short", "go test ./..."]
}
```

`shell` is optional. `/scope` opens or focuses the policy view. `/scope-reload` loads the file asynchronously and denies native commands until a valid policy is available. missing, malformed, truncated or oversized files fail closed. the source limit is 64 KiB. a newer reload supersedes an older pending read.

the `before_command` hook runs before approval and again before execution. edits are checked too. allowed commands still require normal user approval. unload removes this hook; it does not leave a hidden policy behind. `scope.loaded` reports `ready` and an optional error.

this example constrains exact command, directory and optional shell for native command proposals. it does not pin executable hashes, inherited environment, `PATH` resolution, symlink targets or network destinations. it does not constrain raw trusted Lua process calls or plugin-defined tools. use it as an extension example and workflow check, not a sandbox.

## build your own

combine [surfaces](surfaces.md) for presentation, [workspace queries](workspace.md) for native records, [services](services.md) for local I/O and [hooks](extensions.md) for request or approval transformations. reserve work during setup, use callbacks for completion and keep renderable state in owned buffers or regions. read private output only for the feature that needs it; model-directed output must follow the host's sharing policy.
