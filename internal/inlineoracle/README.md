# Test oracle

`oracle.go` is the exact inliner from `origin/wip/hir-inline` at `1f84049419b22f062573ebbe7f30c48cfcecfae2`,
`hir/inline.go` SHA-256 `8119c5dec8fca66687169abc77113f0798cf39884dcd4db0b58f0820c46afb5b`.
Only its package, HIR import and unexported class-constructor constant are
mechanically adapted. It is imported exclusively by tests. No production
rewrite code calls it. Refresh requires fetching and reviewing that branch.
