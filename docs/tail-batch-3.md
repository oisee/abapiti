# Tail batch 3: range traps

Compiler fix: `06cff86`; OSG: `f83dc5c3`; upstream abaplint:
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

## Diagnosis

The current upstream contains two `findTextFile` tests, not three. Both
trapped in `_abap_object.findTexts`: missing XML `LENGTH` became
`parseInt(undefined, 10)`, whose NaN result crossed a finite-only ABAP boundary.
The registry test trapped in `NewlineBetweenMethods.runParsed` when an absent
`rows[nextRow]` was passed to `SequentialBlank.isBlankOrWhitespace`.

Numeric parser fields now retain tagged NaN/Infinity, separately from
undefined. Regex input coercion retains absent scalar indexes through typed
helper parameters and converts undefined to `"undefined"`. Required scalar
reads outside these observation contexts remain checked.

## Verification

One file-focused selection included `test/objects/function_group.ts`,
`test/objects/program.ts`, and `test/registry.ts`:

- Baseline: 61/64, three range traps, FAIL 0; program tests already passed.
- Fixed OSGO and Go: 64/64 each, FAIL 0, traps 0.
- OSGO `abap/lexer`: 200/200, FAIL 0, traps 0.
- Three Node-oracle emitter fixtures: OSGO SUCCESS.
- Mandatory `hir-unit.sh` with `HIR_RUNTIMES=" "`: exit 0.
- `go vet ./...`, requested short Go suite, all 15 `tools/*-test.mjs`: exit 0.
- Lint versus `origin/main`: 0 new issues; canary 5/5; golangci-lint 2.13.2.

OSG's core library checkout did not match its lock. Runs used the locked
`84125c41c22e4c0d45e8037e8b60c691bb64de8d` source archive through
`OSD_LIB_OPEN_ABAP_CORE`, without changing OSG files. Heavy runs used the shared
lock, 48 GiB scope, and nice 10. Kept reports, emitted sources, and diagnostic
traces are retained in external tail3 evidence storage.
