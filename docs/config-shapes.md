# Config shapes and Domain conversion

Base: `origin/integ/train-8@4b6a465c5663360a5f08cb249ef7cb936dfe36df`.
Frozen semantic compiler: `ca46a94` on `feat/fit-config-shapes`.
Unmodified upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`;
fresh Node 26.9.0 / TypeScript 6.0.3 oracles, compiler built with Go 1.26.9,
and shard execution with the runner's pinned Go 1.26.0. Workers: one; Go `-p 2`.

The five touched shards cover 234 files, 3,433 active Node tests and 11 pending.
Passes rise **3,362 → 3,404 (+42)**, traps fall **69 → 27**, and **FAIL stays 0**.
All 37 requested-root executions now pass. There are no regressions, unmatched
executions, infrastructure failures or hidden borrowed FAILs. Registered title
sequences, including duplicates, are identical. Not-run stays 13: 11
skipped/focused and two unregistered. Registrations stay 3,442 of 3,444 titles.
No whole-suite result is inferred from this selected measurement.

| Shard | Passed before → after | Trapped before → after | FAIL | Not-run |
| --- | ---: | ---: | ---: | ---: |
| root | 88 → 90 | 6 → 4 | 0 | 0 |
| objects | 124 → 128 | 20 → 16 | 0 | 1 |
| pretty_printer | 21 → 30 | 10 → 1 | 0 | 0 |
| rules | 3121 → 3141 | 26 → 6 | 0 | 12 |
| utils | 8 → 15 | 7 → 0 | 0 | 0 |
| Total | **3,362 → 3,404** | **69 → 27** | **0** | **13** |

## Fixed roots and storage

| Root | Refusals before → after | Recovered |
| --- | ---: | ---: |
| Pretty-printer `keyword_case` / `sequential_blank` writes | 9 → 0 | 9 |
| NameValidator `patternKind` config literals | 7 → 0 | 7 |
| Config `applyUnspecifiedRules` writes | 2 → 0 | 2 |
| Domain XML conversion | 19 → 0 | 19 |

Dotted writes resolve the receiver's HIR fields, including inherited fields,
and retain typed live setters. Erased or additional properties use existing
ordered-map or dynamic storage adapters. Parsed records and structural rows
keep their JS identity and insertion order through native and erased aliases.

Nominally annotated literals use ordered records plus the certified live class
view. A literal never runs a nominal constructor or invents its defaults.
Inherited properties, omitted fields and explicit undefined values remain
observable. Required primitive fields initialized from erased data retain
optional storage; literal own-key metadata separately preserves presence and
order. Undefined is not converted to an empty string, false or zero.

Required public primitive fields explicitly compared with undefined retain
presence in nominal live layouts too. Only those certified data-only layouts
permit primitive-backed field reads. This lets rule setters inspect `true` or
other primitives and install the defaults that the original JS installs.
Primitive writes, private/accessor layouts and prototype-dependent operations
retain checked refusals. Optional receiver checks alone do not authorize a
primitive view, and ordinary incompatible class conversion still refuses.

A typed immutable scalar initialized from erased data can retain undefined
when every observation is a strict comparison. Captures, escapes and mutable
bindings retain their existing ABI. Conditional initialization stays lazy.
Required primitive call/constructor contexts retain checked narrowing, so
existing constructor and numeric intrinsic lowering is preserved. RegExp,
string repeat/match, Math, typeof, small builtins, LSP types and void ABI
implementations were not modified. No WASM work was performed.

Node/Go probes exercise identity, inherited fields, native/parsed aliases,
extra-key insertion and overwrites, explicit undefined versus omission,
nominal literal construction, typed setters, default filling, primitive-backed
reads, lazy comparison constants and guarded constructor arguments. Existing
prototype and invalid-conversion boundaries still pass.

The five additional recoveries are View XML parsing, the BADI enhancement-spot
reference, `allowNoTextGlobal` in UnnecessaryPragma, and two `skipAbstract`
UnusedVariables configurations. Exact matched titles and messages are retained
in the certificate; none is attributed to the 37 requested roots.

## Full closure and default gate

`abaplint --full` is available in origin/main. The production full-closure
command reports **29 → 29 blocking diagnostics** (19 root, ten cascade), and
**77 → 77 HIR errors in 20 files**. Exact blocking diagnostics, including source
locations and cascade status, are identical after removing temporary prefixes.
All-message histograms and the CLI's normalized root histograms are recorded.
The owned category contributes **0 → 0** static blockers: these fixes remove
runtime/literal-test boundaries, not unrelated core lowering gaps.

Vet, short tests with `ABAPITI_GRACECHECK=1`, the lexer Grace golden,
`TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend` and lint against
origin/integ/train-8 pass. Goldens are unchanged. Lint reports zero new issues,
canary 5/5. Lint runs separately from Go package discovery because it writes a
temporary deliberate-error canary.

Default zabapgit emission against the archived train-8 baseline has raw
`diff -r` exit 0 over **6,321 files**, including the Go executable and ABAP
archives. Both compilers emit to the same path before comparison.
ABAP runtime execution was not performed. No credentials were used; commits
remain local on the feature branch, with no PR or main push.

## Remaining boundaries

The 27 selected-shard traps are outside the repaired roots: malformed/unsupported
JSON/XML adapters, later conversions in MessageClass/Dynpros/SpaceBeforeDot and
other classes, table/proxy signatures and Chai enumeration, delete storage, and
an unresolved NumberRange callable. Domain's malformed-input tests still refuse;
its 19 XML field-storage cases pass. Existing JSON.stringify formatting,
prototype/private storage and unrelated ABI domains are not fabricated.
The unchanged full-closure blockers include integer evidence and LSP
coercion/call/delete/JSON boundaries. Complete messages are in the certificate.

## Reproduction and evidence

Set `UPSTREAM` to a clean checkout of the pinned upstream, with `npm ci` in
both its root and packages/core. `SCRATCH` is a disposable directory. Create
`$SCRATCH/base` before unpacking the archive. Run the commands recorded in
[verification certificate](evidence/config-shapes-verification.json), using
`GOMAXPROCS=2`, Go `-p 2` and one shard worker throughout. The full-closure and
default commands require no external upstream argument because the pinned core
is embedded in abapiti.

```sh
export GOMAXPROCS=2 GOFLAGS=-p=2
"$SCRATCH/base-abapiti" abaplint --full -o "$SCRATCH/full-before"
"$SCRATCH/branch-abapiti" abaplint --full -o "$SCRATCH/full-after"

go vet -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 go test -short -p 2 -timeout 40m \
  ./hir/... ./tsfront/... ./internal/... ./cmd/...
./.github/ci/lint.sh gate origin/integ/train-8
```

- [Baseline five-shard report](evidence/config-shapes-before.json).
- [Final five-shard report](evidence/config-shapes-after.json).
- [Source blobs, commands, paired changes, every remaining trap message,
  full-closure diagnostics and gates](evidence/config-shapes-verification.json).
