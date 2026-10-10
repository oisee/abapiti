# Peephole ledger — 2026-10-10

Analysis baseline: db8173d, after #80 and #81. [Top 20 and full methodology](2026-10-10-peephole-mining/README.md); [ledger CSV](2026-10-10-peephole-ledger.csv).

Each row starts pending. Promote only after typed correctness proofs and positive/negative guard tests, OSGO differential equality (record corpus/hash and delta), and A4H median ns/execution before/after. Record decision with reason. HIR rows belong in Grace HIR -> HIR for TS-HG@Go too. ABAP-only rows belong in pure method-local window rules with side conditions, iterated to a bounded fixed point; require decreasing store count, deterministic rule order and idempotence.

Columns: rule id, pattern, tag, sites, in-loop, hot weight, stmts saved/exec, correctness gate (pending), OSGO delta (pending), A4H ns (pending), decision (pending). Counts are lexical, overlapping, conditional opportunities. No correctness or performance gate has been claimed.
