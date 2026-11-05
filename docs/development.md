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

focused checks cover stream completion and cancellation, model discovery, saved selection, bounded extraction, omission audits, exact pins and literals, third-decline compaction, original archives, failed replacement, editor save detection, command quoting, terminal control handling, unicode, layout dimensions, modal edits and read-only snapshots.

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

[release](../.github/workflows/release.yml) handles version tags matching `v*`, then validates a semantic version before packaging. it runs the reusable checks first, builds both linux binaries with the version embedded, bundles the documentation and writes sha256 checksums. it creates a draft github release for review. nothing is published by a local build.

actions are pinned to commit hashes, dependency updates are scheduled through dependabot, and only the packaging job has repository write permission. the workflow files require a github remote and a push before github can execute them; local tests do not establish that a hosted run passed.


## visual review

`wraith --demo` opens a read-only sample workspace. `wraith --snapshot --width 110 --height 32` emits one ansi frame for layout inspection. the readme preview is an svg rendering of that demo output, not a live model benchmark. test small dimensions, wide unicode, menus, both terminal panes, and each theme when changing layout code.
