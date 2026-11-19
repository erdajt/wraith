# lua runtime

in a source checkout, `gopher-lua/` contains [gopher-lua v1.1.2](https://github.com/yuin/gopher-lua/tree/v1.1.2).
the checkout includes its [mit license](gopher-lua/LICENSE) and upstream tests.
the checkout's root `go.mod` selects this copy through a local replacement.

binary archives include this provenance note and the license. they do not include
the runtime source or tests; use a source checkout at the matching release tag.

the patch fixes protected-call unwinding:

- caught errors preserve upvalues in the caller's live stack frames
- failed callees and error handlers close escaped upvalues before their registers reset
- nested calls restore the previous protected boundary and error-handler state
- failed coroutines close escaped upvalues after a go callback panics

both generator sources and generated sources carry the same changes. upstream
formatting and comments remain unchanged outside the patch.

run the runtime and application regressions from the source checkout root:

```sh
go test github.com/yuin/gopher-lua
go test -race ./internal/plugin -run '^TestProtectedCall'
```

replace this copy with an upstream release once that release passes these
regressions. do not remove the tests when updating the dependency.
