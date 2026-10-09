# Test oracle

The oracle calls the real `hir.InlineStats` from main (including the 70394ff
uninitialized-declaration guard). Both programs are independently deep-copied
with `internal/hirclone` before either mutating inliner runs. The comparison
checks byte-identical HIR dumps, total and per-callee counts, and verification.
There is no pinned implementation to refresh. Production code does not import
this test helper.
