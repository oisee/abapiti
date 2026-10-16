# Recursive contextual array literal regression

The pinned abaplint expressions shard at `8b625f1` registered 107 tests,
passed 101, and trapped six SQL set-operation tests. The trap was
`recursive checker type has no lowered nominal reference` on the expected
AST literal passed to `assertASTEqual`.

## Minimal oracle and bisect

`TestRecursiveTupleLiteralNodeOracle` supplies a recursive readonly tuple
context through a TypeScript overload and uses the existing `any[]`
implementation ABI. A finite, deeply nested literal exercises that context;
Go output is compared with Node, including evaluation order of nested names
and attribute values. The leaf array is explicitly typed to isolate this
regression from existing never-array tuple inference limitations.

With the original minimal oracle, `git bisect run` tested:

| Revision | Result |
| --- | --- |
| origin/selfhost (`9b7aef5`) | pass |
| merge base `a3a9edb` | pass |
| `df03ff8` | pass |
| `459a0c0` | pass |
| `7910112` | recursive checker type trap |
| `fcecfd5` | recursive checker type trap |
| `6bd1fa7` | recursive checker type trap; first bad commit |

Culprit: `6bd1fa72b4d40414377875a0db51820934e718ab`,
`feat(tsfront): lower resolved aliases and anonymous data shapes` (A9).

## Cause and repair

A9 introduced a numeric-index lookup for array types originating in another
pooled checker. It also intercepted canonical arrays, before their existing
type-argument mapping. Unlike that path, the new path propagated a recursive
element-mapping diagnostic instead of letting speculative contextual mapping
fail quietly. The finite literal could therefore be lowered but retained a
blocking diagnostic, which the suite converted into a trap.

Limit the numeric-index fallback to arrays that `IsArrayType` does not
recognize. Canonical arrays keep their existing type-argument mapping and
failure handling. Cross-checker mapped-array support remains enabled.

## Validation

Environment: `GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false`,
`GOCACHE=/mnt/ssd-510/osg/go-build`; commands run with `nice`.

- `gofmt` and `git diff --check`: clean.
- `go vet ./...`: passed.
- `go test ./tsfront/ ./hir/... ./cmd/abapiti/ ./internal/abapfixture/ -short -p 2`: passed.
- Final strengthened Node oracle: passed. The parent/A9 comparison of the
  probe passed on A9's parent and reproduced the recursive checker trap at A9.
- `abap/expressions`: 107 registered, 107 passed, zero trapped or failed
  (before repair: 101 passed, six trapped).
- `abap/statements`: 4,962 registered and passed, zero trapped or failed.
- `abap/syntax`: 1,373 registered, 1,363 passed, 10 Node-pending tests,
  zero trapped or failed (owned-test accounting).
- `abapiti abaplint /home/alice/dev/abaplint -o <scratch>/m1 --target go`:
  passed; summed `grep -rc 'assignment type mismatch' <scratch>/m1/go/*.go`
  was exactly 2.
- M1: copied `/mnt/ram/abapiti/selfhost-m1/go-multifile/main.go`, built
  `zabaplint-mf`, ran `--file files.txt --config abaplint.json` in
  `/mnt/ram/abapiti/selfhost-m1/osg-check-kit`, and compared the output with
  `expected-clean.txt`: byte-identical (`cmp` exit 0).
