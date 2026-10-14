# Native PCRE word boundaries at OSG 16c024c8

Verified implementation `c6bb3c8` against OSG
`16c024c8fc175b5656d643034b0fe8dde484740f` and original abaplint
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`, using Go 1.26.0.

`pcreRegexPattern` now preserves native `\b` instead of rewriting the reviewed
DDL pattern's leading boundary to negative lookbehind. `TranslateSubset`
already preserves both `\b` and `\B`. Non-UCP PCRE and RE2 agree with JS non-u
ASCII `[A-Za-z0-9_]` membership; Unicode flags remain refused.

The Node oracle checks `/\bdefine\s+behavior/i` on `"define behavior"`,
`"xdefine behavior"`, `"é define behavior"`, and `"_define behavior"`, plus
adjacent non-ASCII characters, uppercase text, `\B`, and the reviewed DDL
pattern. Both emitted spelling and RE2 results must agree with Node.
An emission regression also requires the DDL constructor's native boundary.

## Focused corpus

Selected the original `test/rules/unknown_types.ts` through
`ABAPITI_TESTS_FILES`, retaining closure materialization and the independent
Node oracle. Saved pre-fix harness evidence contains exactly two
negative-lookbehind test diagnostics, both in this file; no additional matching
files were found. Both `FOR LOCK, entity referenced via BDEF alias` and
`FOR LOCK, alias not defined in BDEF` now report `passed` with empty messages.

| Measurement | Pass | FAIL | Trap | Registered | Node-pending |
| --- | ---: | ---: | ---: | ---: | ---: |
| Saved pre-fix unknown_types evidence | 136 | 0 | 2 | 139 | 1 |
| Focused unknown_types, OSG | 138 | 0 | 0 | 139 | 1 |
| Focused unknown_types, Go | 138 | 0 | 0 | 139 | 1 |
| abap/lexer, OSG | 200 | 0 | 0 | 200 | 0 |

Both new runs pass **138/138 Node-active tests**, including the nine
parameterized cases. The one Node-pending case remains explicitly not-run,
outside the active denominator. Neither run has initialization diagnostics,
async-pending results, active unregistered tests, or unmatched results.

## Recovery fixtures and gates

An arbitrary lookbehind literal does not reach FIND PCRE through the checked
regex constructor. Instead, `/a/y` with `lastIndex = 1001` and
`exec("a".repeat(1002))` emits a cursor repeat exceeding RE2's 1000-repeat
limit. Node accepts and matches this valid JS program. OSG emits a real
`NOT_COMPILED in FIND PCRE` with an invalid-repeat-count diagnostic.
See [the recovery protocol](osg-runner-resume.md).

Both `TestAbaplintTestHostOSGEmission` and
`TestAbaplintTestShardsOSGEmission` pass on the pin. The host fixture preserves
**4 real dumps, 5 invocations, all 14 registrations, 8 passes, 2 intentional
assertion FAILs, 4 test traps**, and its **2 host initialization traps plus
1 subsequent uninitialized-binding trap**. Shards a/b/a/b retain their original
counts and isolation. All four host-dump reasons are checked.

Gates passed: `go vet ./...`;
`GOFLAGS=-buildvcs=false go test -short -p 2 ./hir/... ./tsfront/ ./cmd/... -count=1`;
each of the **15** `tools/*-test.mjs` files individually (**48/48 Node tests**);
and lint against `origin/main` (**0 new issues**, canary **5/5**, golangci-lint
**2.13.2**). Heavy steps held the shared flock in a 48 GiB systemd scope at
nice 10, admitted at load <12. Caller scratch and the OSG gogen Go-cache
artifact are removed after verification; OSG source files were not changed.
