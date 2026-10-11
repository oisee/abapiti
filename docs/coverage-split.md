# abaplint 100%: split, lessons, rules (2026-10-11)

Goal: all 11,060 abaplint tests pass on abaplint compiled by abapiti, TS-HG@Go first;
TS-HA@OSGO/OSGB is the gate before A4H. FAIL = issue/assertion mismatch only.

## Lessons from the first 24 hours

1. **Per-PR merging into a moving tsfront does not scale.** Every merge re-conflicted
   the next PR; the queue cost hours of rebases. Integration trains fixed it: one
   branch, `--no-ff` merges in order, each conflict resolved once, goldens once, one
   CI + OSGO + N, one merge. Keep trains.
2. **Build the next train on the previous train's tip, not on main.** Train-2 and
   train-3 were built while their predecessor was still in CI; the merge into main
   was then mechanical.
3. **Split by trap, not by file.** The N table's "top reached traps" is the backlog.
   Each trap has one owner; overlaps were rare once work was cut this way.
4. **Measure with the harness, not with the blocker count.** `--full` blockers fell
   635 → 33 while N stayed near 700; one trap (find_global_definitions arity) gated
   6,144 tests. Prioritise by tests unlocked.
5. **"FAIL" must mean a wrong answer.** 124 "FAILs" were refusals before the
   assertion; the 10 real ones were a silent switch miscompile. Classify, then hunt.
6. **Fail closed, always.** Unsupported paths trap; they never return a default.
   The one silent miscompile (empty case labels) affected four sites quietly.
7. **Gates drift unless they are the CI's own.** Three PRs went red on things the
   local checks skipped: Grace golden (needs ABAPITI_GRACECHECK=1), golangci-lint
   "new issues", osgo known gaps. The verify script now runs exactly the CI gates.
8. **Go-only runtime ops are debt.** Every new op needs an ABAP emitter (or an
   explicit refusal) before the OSGO gate; the backend-coverage guard enforces it.
9. **Keep builders on disk and pushed.** Long runs, worktrees outside /tmp, push per
   green step.

## Ownership (zones)

| Zone | Owner |
|---|---|
| HIR, Grace, type/ABI (unions, covariance, optionals, arity), async/promises, generators, compiler panics, ABAP-emission debt for new ops, train assembly + N measurement | **dell** |
| Test harness + shards, chai/mocha shim, rules-specific lowering, regexp/string, JSON/config conversions, lost registrations, new shards (syntax, lsp, objects, cds…) | **abapiti** |
| gogen typing pilot (ir.d.ts / JSDoc @ts-check, abapiti-friendly TS style), OSG runtime gaps the oracle finds (PCRE etc.), OSGO runs on request | **stoker** |

A trap is claimed by message before work starts; the other side skips it.

## Cadence

- After each train lands: the train builder publishes the N table + top-15 traps;
  both sides claim by zone within the hour; builders start immediately on the
  previous train's tip.
- A train leaves when ≥3 branches are verified or every 6 hours, whichever first.
  Late branches ride the next one; nobody waits.
- abapiti runs OSGO and merges trains; dell assembles them (alternate if one is busy).
- One builder per trap; at most ~4 heavy builders on i3 at a time.

## Gates for every branch (verify-fit.sh)

go vet · go test -short -p 2 · ABAPITI_GRACECHECK=1 Grace golden · golangci-lint gate
vs base · backend-coverage guard · default zabapgit `diff -r` vs base empty (or a
justified, listed change) · harness N on the touched shards, FAIL 0 · leak scan, no
home paths.

## Additions (abapiti, agreed)

- On each train merge, the main README progress block (N/11,060, FAIL, `--full`) is updated by whoever merges (abapiti).
- Any genuine FAIL (wrong answer) pauses claims on that area until its root cause is known; the finder announces it to both immediately.
