# lua runtime

in a source checkout, `gopher-lua/` contains [gopher-lua v1.1.2](https://github.com/yuin/gopher-lua/tree/v1.1.2).
the checkout includes its [mit license](gopher-lua/LICENSE) and upstream tests.
the checkout's root `go.mod` selects this copy through a local replacement.

binary archives include this provenance note and the license. they do not include
the runtime source or tests; use a source checkout at the matching release tag.

the protected-call patch fixes unwinding:

- caught errors preserve upvalues in the caller's live stack frames
- failed callees and error handlers close escaped upvalues before their registers reset
- nested calls restore the previous protected boundary and error-handler state
- failed coroutines close escaped upvalues after a go callback panics

native require also clears its own unchanged loading marker when a source fails,
so ordinary require can retry a corrected source. cache values written by the
source are preserved. successful caching, recursion checks and error objects
retain their existing behavior, including source frames visible to error handlers.

`LState.IsModuleLoading(name)` checks the native registry marker without exposing
it or running cache metamethods. wraith uses this boolean to distinguish bound
loaders entered through require from direct function calls. trusted code can
still mutate the registry; this is not a sandbox.

the logical-expression compiler copies an operand to its destination only when
short-circuiting selects the final result. this preserves nil and false operands
and prevents intermediate values from changing aliased locals before evaluation
finishes. regression tests cover selected values, operand order and assignment
contexts.

both generator sources and generated sources carry the protected-call changes.
the upstream finalizer test reads its shared counter atomically to match the
finalizer writes. upstream formatting and comments remain unchanged outside the
patches.

run the runtime and application regressions from the source checkout root:

```sh
go test github.com/yuin/gopher-lua
go test -race ./internal/plugin -run '^TestProtectedCall'
```

replace this copy with an upstream release once that release passes these
regressions. do not remove the tests when updating the dependency.
