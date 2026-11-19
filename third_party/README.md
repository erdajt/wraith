# lua runtime

`gopher-lua/` contains [gopher-lua v1.1.2](https://github.com/yuin/gopher-lua/tree/v1.1.2).
Its [mit license](gopher-lua/LICENSE) and upstream tests remain included.
The root `go.mod` selects this copy through a local replacement.

The patch fixes protected-call unwinding:

- caught errors preserve upvalues in the caller's live stack frames
- failed callees and error handlers close escaped upvalues before their registers reset
- nested calls restore the previous protected boundary and error-handler state
- failed coroutines close escaped upvalues after a go callback panics

Both generator sources and generated sources carry the same changes. Upstream
formatting and comments remain unchanged outside the patch.

Run the runtime and application regressions from the repository root:

```sh
go test github.com/yuin/gopher-lua
go test -race ./internal/plugin -run '^TestProtectedCall'
```

Replace this copy with an upstream release once that release passes these
regressions. Do not remove the tests when updating the dependency.
