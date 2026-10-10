Single-use HIR locals and unread local stores emit avoidable temporary stores.
Add two shared Grace rules before ABAP and Go emission, enabled with
`ABAPITI_COPYPROP=1` and off by default.

Copy propagation proves identical HIR types, one definition/read, unchanged
local inputs and safe evaluation order. It rejects loop/try/finally crossings
and closure captures; effectful expressions move only into the immediately
following whole-expression use. DSE uses CFG read/kill boundaries on every
path, including exceptions/finally, and requires a positively reviewed inert
RHS. Preserve emitter declarations, remove their stores, detach shared mutable
syntax, and verify HIR after every bounded round. Later rounds refresh changed
methods and are checked against full recomputation.

The full closure rewrites Go 116 copies/655 stores and ABAP 118 copies/961
stores. ABAP statements decrease 568,019→564,916;
requested hot methods decrease 7,258→7,228. Go kit clean and seeded outputs are
byte-identical in all twelve runs. Median checks are
10.404→10.745s clean and
10.395→10.418s seeded.
The measured Go pass costs 1.006s,
4.93% of the baseline cached build.

Passed `go vet ./...`, `go test -short ./...` with the flag off and on,
and `./.github/ci/lint.sh gate origin/main` (golangci-lint 2.13.2, zero new
issues, 5/5 canaries). `ABAPITI_GRACE_FULL=1 ABAPITI_COPYPROP=1` full closure
passed in 2,606.73 seconds, including independent comparisons of 435,383,414
and 443,152,814 premises, ordinary/contracted liveness agreement, unfiltered
recomputation, determinism, budgets, termination and idempotence. Executable
flag-on lexer (44/44 and mutation rejection), registry JSON/XML, arrays,
iterators, features and sorts tests passed.

Conservative limits: ordinary motion stops at calls and heap/runtime writes.
DSE excludes calls, identity allocation, potentially raising arithmetic and
nullable runtime receivers. With the correct fixture digest and 8 GB heap,
both OSGO attempts returned invalid unit results before reporting any tests. ABAP equality remains for
abapiti's A4H/OSGO run. Detailed samples and counts are in
`docs/history/2026-10-10-copyprop-dse/`.
