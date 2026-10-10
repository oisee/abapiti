# Grace inline flag: parity and cost gates

Measured on 10 October 2026 at main `93adeae` (Grace merged in #63), on
`feat/grace-inline-flag`, with Go 1.26.0 on Linux amd64, AMD Ryzen AI 7 PRO
350 (8 logical CPUs). Heavy runs held `flock /tmp/abapiti-heavy.lock`.
Build artifacts and caches used disk-backed `/var/tmp` after the small
`/tmp` tmpfs ran out of space during the initial compilation.

`ABAPITI_INLINE=grace` selects the existing production `rewrite.Inline`
API. The default remains `hir.InlineStats`; `ABAPITI_INLINE=0` bypasses
inlining. The emitter retains its shared stats formatting and verification
step. No Grace performance changes were made.

## Gate 1: byte parity

Built `abapiti abaplint -o /tmp/gflag-evidence/output` with the default
and Grace, using the same output path and moving each completed tree aside.
SHA-256 manifests include every regular output file, sorted by relative path;
all 6,336 files match, including `names.json`, the three target READMEs,
and the reproducible A4H zip. There are 6,331 loose ABAP files; the zip has
4,226 entries, including 2,112 ABAP files. No first difference exists.

The full production build prints identical `ABAPITI_INLINE_STATS=1` output:
1,487 call sites and 190 callees.

The lexer and registry fixture outputs also match. Existing emission tests
exported their output via `ABAPITI_TEST_OUT`. JSON and XML tests normally
require external runtime oracle inputs before emission; temporary hooks
emitted their verified lowered programs directly, without the oracle drivers.
These hooks were removed before validation and are not part of the change.

| Output | Files compared | Result |
|---|---:|---|
| All CLI targets | 6,336 | Identical |
| Lexer, including driver | 70 | Identical |
| Registry features | 22 | Identical |
| Registry arrays | 26 | Identical |
| Registry iterators | 14 | Identical |
| Registry JSON, raw emission | 28 | Identical |
| Registry singletons | 16 | Identical |
| Registry sorts | 10 | Identical |
| Registry XML, raw emission | 16 | Identical |
| Total regular output files | 6,538 | Identical |

SHA-256 of the complete CLI manifest (lines `hash  relative/path\n`):
`671e26255be239ba661e0621e8c77a3ce2e35b1bd64f014700c6a177bf5a9c69`.
Per-file manifests and raw measurement logs are retained in
`/var/tmp/abapiti-gflag/evidence` (also accessible as `/tmp/gflag-evidence`).

## Gate 2: cost

Three independent processes per implementation and workload, alternating
default and Grace. Medians are computed separately for wall time and RSS.
Whole builds used the built binary's `abaplint -o <same path> --quiet`, with
all targets. `/usr/bin/time` measured elapsed time and maximum resident set.
The binary compilation is excluded.

For the pass alone, a temporary helper loaded the production CLI's lowered
1,538-file closure (1,927 classes, 73 interfaces) before starting the timer.
The capture preserves nil entries and restores shared HIR pointers, which
matters to call-site counts. After loading, `debug.FreeOSMemory` released
unused memory and `/proc/self/clear_refs` reset the RSS high-water mark.
The measured interval calls `hir.InlineStats` or `rewrite.Inline` directly;
it excludes loading, stats printing and the emitter's common verification
step. Grace's own internal verification remains included. Peak RSS includes
the resident HIR and Go runtime; it excludes the TypeScript frontend process.
Both implementations produced the same 1,487-site, 190-callee totals.

| Workload | Default wall | Grace wall | Wall ratio | Default peak RSS | Grace peak RSS |
|---|---:|---:|---:|---:|---:|
| Inlining pass | 0.035710 s | 24.234552 s | 678.6× | 89.58 MiB | 1,305.55 MiB |
| Whole `abapiti abaplint` build | 10.23 s | 35.30 s | 3.45× | 381.01 MiB | 1,338.45 MiB |

Raw runs, wall seconds / RSS KiB:

| Workload | Implementation | Run 1 | Run 2 | Run 3 |
|---|---|---|---|---|
| Pass | Default | 0.039054 / 91732 | 0.035710 / 92012 | 0.033863 / 91712 |
| Pass | Grace | 24.587105 / 1336888 | 24.234552 / 1326332 | 23.751492 / 1362188 |
| Build | Default | 10.21 / 389772 | 10.35 / 394344 | 10.23 / 390152 |
| Build | Grace | 34.83 / 1353956 | 35.30 / 1370572 | 36.55 / 1426044 |

Grace exceeds 2×, so an additional pass process captured a CPU profile
(25.09 s wall, 56.34 s CPU samples across threads). The hot application part
is fixed-point relation evaluation: `rewrite.Evaluate` accounts for 40.31%
of total CPU samples cumulatively, `rewrite.join` 39.26%, and
`rewrite.matches` 27.41% (11.70% flat). `rewrite.Analyze` includes 32.89%
cumulatively. Garbage collection is substantial: background mark workers
account for 50.82% cumulatively. These cumulative percentages overlap;
they are not additive. The next performance investigation should examine
relation joins and their allocation/GC cost. No optimization was attempted.

## Validation

`TestInlineGrace` checks fixture stats against `hir.InlineStats`, matching
HIR and emitted stats text in both flag modes, and a Grace-specific input
validation error to prove the flag selects Grace. Existing disabled-mode
tests still cover `ABAPITI_INLINE=0`.

- `go build ./...`: passed.
- `go vet ./...`: passed.
- `go test -short ./...`: passed.
- `./.github/ci/lint.sh gate origin/main`: passed, 0 new issues,
  canary 5/5, golangci-lint 2.13.2. Installed with the repository installer
  and pinned CI checksum; missing `jq` was also installed temporarily from
  its checksum-verified official release.
