# development

```sh
make build
make test
make check
make install
```

`make check` enforces formatting, runs `go vet` and executes all tests with the race detector. `make build` produces `bin/wraith`; `make install` copies it into `~/.local/bin`. `make clean` removes local build output. module identity is `github.com/erdajt/wraith`; the development branch is `main`.

## tests

fixtures use an in-memory http transport, real linux ptys and temporary data directories. no installed model, gpu or running ollama service is required. the interactive subprocess test drives the actual bubble tea app through a pty, returns from a configured editor, hides and reopens a shell, and verifies that the side shell has independent state.

when neovim is installed, a second integration test runs wraith inside its terminal, opens a nested editor, restarts a shell and searches/yanks terminal output. CI installs neovim to exercise this path.

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

[checks](../.github/workflows/ci.yml) runs on pushes and pull requests. it verifies modules, formatting and vet; runs race/integration tests; fuzzes both parsers; builds linux amd64 and arm64; and uploads coverage. it can also be called by the release workflow.

[release](../.github/workflows/release.yml) handles version tags matching `v*`, then validates a semantic version before packaging. it runs the reusable checks first, builds both linux binaries with the version embedded, bundles the documentation, license and copyright notice, and writes sha256 checksums. it creates a draft github release for review. nothing is published by a local build.

actions are pinned to commit hashes, dependency updates are scheduled through dependabot, and only the packaging job has repository write permission. the workflow files require a github remote and a push before github can execute them; local tests do not establish that a hosted run passed.

## visual review

`wraith --demo` opens a read-only sample workspace. `wraith --snapshot --width 110 --height 32` emits one ansi frame for layout inspection. the readme uses cropped captures of the running app in ghostty. [screenshots](screenshots.md) records the fixtures, capture script, binary hash and crop geometry. test small dimensions, wide unicode, menus, both terminal panes, and each theme when changing layout code.

## test coverage

unit tests use the standard [testing package](https://pkg.go.dev/testing), descriptive behavior names, table-driven cases where inputs vary, temporary directories and cleanup-owned resources. independent tests run in parallel; process/environment tests keep their global state isolated. tests that own asynchronous work use cancellation and explicit completion rather than relying on arbitrary sleeps.

the execution suites run real short shell jobs for approval, cancellation, timeout, concurrent output and failed persistence. provider transports are fixtures, so privacy, streaming repair, compaction and evidence ownership are tested without credentials or a GPU. regression cases cover oversized polling lines, retained final errors, interrupted journals, variable rotation, non-shell editor review and CLI export redaction.

CI runs the complete race suite with shuffled order and coverage on every push to `main` and on pull requests. it also verifies module checksums, formatting and vet, installs neovim for editor integration, fuzzes terminal parsing and modal editing, and builds Linux amd64/arm64 binaries. release tags call the same workflow before packaging. local success does not establish that a hosted run passed; inspect the corresponding Actions run when validating a deployment.
