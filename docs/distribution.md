# package distribution

[development](development.md) · [architecture](architecture.md) · [package manager](lua/packages.md)

Shroud's source lives in `erdajt/shroud.wraith`. wraith distributes a full git bundle of a selected commit, its license and a strict [manifest](../internal/distribution/assets/manifest.json). the bundle is a generated release artifact. edit Lua in the package repository, then export a new pin. do not maintain another editable copy inside wraith.

## startup

wraith prepares Shroud before evaluating `init.lua`. git is required; network access is not. checkouts live at `~/.local/share/wraith/bootstrap/objects/shroud/COMMIT`, or below your configured data directory. subsequent starts verify and reuse that checkout. startup does not update the selected revision.

verification checks the manifest, host API interval, artifact size and checksum, self-contained bundle reference, literal git tree, source URL, commit, license and entry module. files must match the selected tree, including executable bits. untracked, ignored or changed files cause an error. bootstrap does not reset or discard modified sources.

installation stages under the destination filesystem, synchronizes files and publishes without replacing another process's checkout. cancellation and interrupted staging cannot publish an incomplete tree. an existing valid pin remains usable without the seed. a bootstrap error is shown through the configuration error path; the application retains its YAML configuration.

`require("shroud")` and `require("wraith.manager")` resolve to the same module. selected namespaces bind to the verified checkout, including nested imports. this protects source selection from accidental module shadowing. Lua remains trusted user code with filesystem and process access.

## update a pin

commit and review the package source first. use a full commit id from that repository:

```sh
go run ./cmd/wraith-bundle \
  --source /path/to/shroud.wraith \
  --output internal/distribution/assets \
  --name shroud \
  --repository https://github.com/erdajt/shroud.wraith.git \
  --module shroud \
  --commit FULL_COMMIT_ID
```

the exporter rejects dirty or shallow source checkouts. it exports only the selected commit's ancestry, with no prerequisite bundle. the output names the package, commit and SHA-256 digest. concurrent imports serialize manifest publication without dropping other package pins.

the manifest records the source URL, git object format, commit, artifact path, checksum, size, Lua entry module, host API range and license path. defaults use API 2; explicit `--api-min` and `--api-max` flags select another supported interval. hashes identify the exported bytes; byte-for-byte reproducibility across different git versions is not promised.

review the manifest and source change together. publish the package commit before publishing a host release that selects it. remove superseded, unreferenced bundle files after updating the manifest so they are not embedded in the binary.

## verify a candidate

```sh
go test -race ./internal/distribution ./cmd/wraith-bundle ./internal/plugin
```

distribution tests export real repositories, verify their raw trees and install the resulting bundles. manager tests use the shipped pin through the same installation path before loading user configuration. they do not read a sibling checkout or fall back to embedded Lua source. pin a candidate using the exporter before running these tests against a changed package.

the generic plugin runtime can accept explicit package roots without installing Shroud. this separates the Lua host from its default manager. CLI startup and configuration reload select the distribution pin explicitly.
