# development

```sh
make build
make test
make check
make install
```

`make check` enforces formatting, runs `go vet`, executes application tests with the race detector and shuffled order, then runs the bundled lua runtime's upstream tests. `make test` and `make race` also include that upstream suite. `make build` produces `bin/wraith`; `make install` copies it into `~/.local/bin`. `make clean` removes local build output. module identity is `github.com/erdajt/wraith`; the default branch is `main`.

## tests

fixtures use an in-memory http transport, real linux ptys and temporary data directories. no installed model, gpu or running ollama service is required. the interactive subprocess test drives the actual bubble tea app through a pty, returns from a configured editor, hides and reopens a shell, and verifies that the side shell has independent state.

when neovim is installed, a second integration test runs wraith inside its terminal, opens a nested editor, restarts a shell and searches/yanks terminal output. CI installs neovim to exercise this path and zsh for actual line-editor insertion tests.

focused checks cover stream completion and cancellation, provider request formats, native cli event decoding, model discovery, saved selection, bounded extraction, omission audits, exact pins and literals, third-decline compaction, original archives, corrective retries, failed replacement, output sharing, directory-scoped resume, input history, editor save detection, command quoting, terminal control handling, unicode, layout dimensions, modal edits and read-only snapshots.

```sh
go test -race -shuffle=on -count=1 ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
GOMAXPROCS=2 go test ./internal/terminal -run '^$' -fuzz '^FuzzScreen$' -fuzztime=30s -parallel=2
GOMAXPROCS=2 go test ./internal/vim -run '^$' -fuzz '^FuzzEditing$' -fuzztime=30s -parallel=2
```

fuzz corpus entries that reproduce bugs belong beside the affected package. test behavior and recovery boundaries, rather than private implementation shape. pty integration requires `/dev/ptmx` and a mounted `/dev/pts`.

## ci and releases

[checks](../.github/workflows/ci.yml) runs on pushes and pull requests. it installs checksum-pinned LuaLS 3.19.1, verifies modules, formatting and vet; runs race/integration tests; fuzzes terminal parsing, modal editing and lua json validation; builds linux amd64 and arm64; and uploads coverage. it can also be called by the release workflow.

[release](../.github/workflows/release.yml) handles version tags matching `v*`, then validates a semantic version before packaging. it runs the reusable checks first, builds both linux binaries with the version embedded, bundles the documentation, license, copyright notice and bundled lua runtime notice, and writes sha256 checksums. it creates a draft github release for review. nothing is published by a local build.

actions are pinned to commit hashes, dependency updates are scheduled through dependabot, and only the packaging job has repository write permission. the workflow files require a github remote and a push before github can execute them; local tests do not establish that a hosted run passed.

## visual review

`wraith --demo` opens a read-only sample workspace. `wraith --snapshot --width 110 --height 32` emits one ansi frame for layout inspection. the readme uses cropped captures of the running app in ghostty. [screenshots](screenshots.md) records the fixtures, capture script, binary hash and crop geometry. test small dimensions, wide unicode, menus, both terminal panes, and each theme when changing layout code.

## test coverage

unit tests use the standard [testing package](https://pkg.go.dev/testing), descriptive behavior names, table-driven cases where inputs vary, temporary directories and cleanup-owned resources. independent tests run in parallel; process/environment tests keep their global state isolated. tests that own asynchronous work use cancellation and explicit completion rather than relying on arbitrary sleeps.

the execution suites run real short shell jobs for approval, cancellation, timeout, concurrent output and failed persistence. provider transports are fixtures, so privacy, streaming repair, compaction and evidence ownership are tested without credentials or a GPU. regression cases cover oversized polling lines, retained final errors, interrupted journals, variable rotation, non-shell editor review and CLI export redaction.

CI runs the complete race suite with shuffled order and coverage on every push to `main` and on pull requests. it also verifies module checksums, formatting and vet, installs neovim for editor integration, fuzzes terminal parsing and modal editing, and builds Linux amd64/arm64 binaries. release tags call the same workflow before packaging. local success does not establish that a hosted run passed; inspect the corresponding Actions run when validating a deployment.

lua tests execute real scripts, including the documented configuration and plugins. they cover module confinement, FIFO rejection, retained coroutines, cancellation, reload rollback, YAML layering, actual slash completion, callback targeting, editor continuations and action guards. native Go tests remain beside their packages; example configuration lives under `docs/examples/lua`.


## extension regressions

extension tests use the real lua VM and app publication handshake. rejection cases verify that commands, mappings, resources and owned cleanup retain their previous state. protected-call tests cover live upvalues, escaped closures, nested error handlers, go panics, coroutine failures and VM reuse. the upstream suite is separate from the root module:

```sh
go test -count=1 -timeout=2m github.com/yuin/gopher-lua
go test -race ./internal/plugin -run 'TestProtectedCall|TestManager'
GOMAXPROCS=2 go test ./internal/plugin -run '^$' -fuzz '^FuzzJSONSurrogateValidation$' -fuzztime=30s -parallel=2
```

package tests create local git repositories and exercise the public manager API. they cover initial install, update, exact restore, dirty checkouts, dependency activation, lazy first-input delivery, lock contention, rejected publication, cancellation, cleanup failure and fresh-runtime activation. they require `git`, `flock`, `sh`, `cmp`, `mv` and `mktemp`; no remote repository is needed. managed-service tests cover actual child processes and local http servers, bounded output, cancellation and handle release.

[editor setup](lua/editor.md) documents the LuaLS library. an unconditional test compares its executable export declarations with the real runtime. a second test uses the actual language server to check valid calls and required errors for invalid calls. CI installs the pinned server; locally set `WRAITH_LUALS` to its executable or install `lua-language-server` on `PATH`. without either, the diagnostic test reports a skip. keep public signatures, option tables, callback payloads and result types aligned with the runtime. test documented modules as executable examples; do not replace behavioral checks with source-text assertions. [package examples](lua/packages.md) and [extension examples](lua/examples.md) use the same public APIs available to user code.

## changes and review

group related statements by purpose. keep lookup, validation and assignment together; separate the next logical operation with a blank line. use function comments where they explain the API or lifecycle contract. keep unit tests beside the package they exercise, with integration fixtures in temporary directories or `testdata` when they need files.

change one behavior per reviewable commit. verify the affected recovery paths before broadening to the full suite. API changes need matching types, examples and reference documentation. runtime changes also need rejection, cancellation and unload cases; package changes need real filesystem and git evidence. do not claim a cross-built target was executed or a hosted workflow passed from local results.
