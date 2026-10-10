# Opt-in Grace copy propagation and dead-store elimination

Implementation: `8f0d5eb`, based on the Grace flow-facts merge `0d49b50`.
No PR has been opened. `ABAPITI_COPYPROP=1` enables both rules before emission
in `hir/abap` and `hir/golang`; the default is off.

The adapter proves one definition/read, exact HIR types, and safe motion within
one statement list. Pure, total expressions can move past inert stores that do
not write their local dependencies. Calls and heap/runtime writes are barriers,
including unknown summaries and possible alias writes. Effectful/raising RHS
expressions only move into an immediately following whole-expression use.
Loop/try/finally crossings and lowered closure captures are rejected. A changed
RHS invalidates a dependent plan until a fresh verified round. DSE rejects
calls, allocation, checked arithmetic and unreviewed runtime operations. It
keeps emitter declarations and drops only their initializers; dead local
assignments become empty blocks.

`store_next` contracts the existing CFG to the first same-binding read or kill
on every path, including exceptional/finally edges. Grace derives
`not_read_after` from these boundaries. The full-input test checks these proofs
against the existing liveness relation for all 2,079 inert store candidates;
that relation is also checked by the independent full reference evaluator.
Later rounds refresh changed methods only, with full recomputation retained as
an oracle. Each round runs `hir.Verify`; existing round/growth limits remain.

## Go measurements

`go-timings.json` records every sample. The unchanged
`tools/go-target-check.py` compared raw output with the release kit at
`$HOME/dev/dell-work/kits/zabapgit-check-kit`. Both flag modes used the same
binary generation command, Go cache, four workers, input/config/dependency
paths and timing instrument. Each of all twelve samples was byte-identical to
the kit: clean has zero issues and seeded has five.

| Workload | Flag off median | Flag on median | Change |
| --- | ---: | ---: | ---: |
| Clean | 10.412s | 10.287s | -1.20% |
| Seeded | 10.294s | 10.354s | +0.58% |

These small, mixed timing changes do not establish a runtime speedup. Cached
end-to-end build observations were 41.23s off and 42.65s on, including lowering,
emission and Go compilation. These are single build observations, not medians.

## Validation

The implementation passed `go vet ./...`, `go test -short ./...` in both flag
modes, and `./.github/ci/lint.sh gate origin/main` with golangci-lint 2.13.2:
zero new issues, five of five canaries. Four existing tests pin baseline emitter
or inliner shape and explicitly disable this separate pass. Synthetic tests
cover effects/exceptions, intervening local/alias writes, loop duplication,
optional conversion, finally reads, closure capture, allocation, unused raising
RHS, definition kills, dependent plans, action safety, opt-in behavior,
idempotence, determinism, independent rule evaluation and compact/full CFG
parity. The Go kit executions above exercise the actual flag-on generated CLI.

Full-closure, explicit executable-oracle and OSGO results will be recorded with
the final measurement step.
