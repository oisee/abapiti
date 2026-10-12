# ABAP class-view and constructor ABI gap

## Result and measurement boundary

All four assigned **static refusal groups are gone from the pinned full test
closure**. This is not an ABAP-host suite-pass certificate. **OSGO execution
was unavailable and was not attempted**; validation used the repository's
ABAP emitter tests, emitted-source inspection, ABAP syntax checking, and the
existing Node/Go semantic oracles. No runner/harness changes, push, clone, or
changes to the coordinator's OSG checkout were made.

Compact evidence: [osgo-classview.json](evidence/osgo-classview.json).

| Assigned item | Before | After | Counting stage |
| --- | ---: | ---: | --- |
| `object.checkedClassView` | 77 | 0 | Unsupported runtime-operation occurrences, before ABAP passes |
| `classvalue.newArgs` | 2 | 0 | Unsupported runtime-operation occurrences, before ABAP passes |
| `classOf` on root `object` | 3 | 0 | Distinct emitter refusals, after ABAP passes |
| Fresh lexical constructor descriptor | 1 | 0 | Distinct emitter refusals, after ABAP passes |

The two operation rows remove **79 catalogue-refused occurrences**. The other
two rows remove **4 transformed-HIR refusals**. They must not be added into a
single pre-pass source-site census, nor interpreted as newly passing tests.
An owning body can still be refused by another operation.

### Counting provenance

Baseline: `origin/main` at `07714f1`; implementation: `526a857`.
The existing closure materializer was used with all 625 original test roots,
the original mocha/chai shim, and upstream pin
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. The resulting manifest SHA-256 is
`8bc69eca3e2735470d1f5aa2b955127eaf378f13cd0b781c750e2493185fa189`, **identical
to the M0 full-closure manifest**:

- 17 shards; 2,166 files; 7,052 classes; 85 interfaces.
- Blocking diagnostics: **0**; HIR errors: **0**.
- `ABAPITI_INLINE=classic` in both audits.
- Unsupported catalogue entries: **26 → 24**; present operations: **24 → 22**.
- Unsupported runtime occurrences: **192 → 113**.
- Other emitter refusals: **76 → 72**.
- Nonfinite literal sites: **0 → 0**.

Counting used `MeasureABAPTestGap` from
`origin/feat/osgo-suite-m0:tsfront/abaplint_tests_abap.go`, with that branch's
`Audit`/`Refusal` diagnostic collector temporarily applied to each emitter.
Runtime sites were measured before emission; emitter diagnostics were
normalized and deduplicated with owning-method/node context, including ID-zero
operands. The frontend HIR was cached for the after audit with integer literal
domains restored before replay, not converted to binary64 literals. Temporary
counter/collector sources and generated closures were removed. The harness and
runner were neither imported nor edited. Evidence source paths are relative.

When integrated with the M0 counting command, the equivalent measurement is:

```sh
ABAPITI_INLINE=classic nice -n 10 abapiti abaplint-tests CLOSURE \
  --target osg --gap-only -o OUT
```

This needs a materialized full closure, but no OSG execution.

## Implemented semantics

### Checked nominal views

`object.checkedClassView` is a nominal checked cast, **not** a structural
materializer. ABAP emits `?=` on the already-evaluated receiver. Successful
casts preserve the original reference, fields, mutation visibility, and
identity; initial references remain initial. Downcasts, related sibling casts,
and erased-root casts must pass the actual ABAP runtime type check. A mismatch
raises a native cast fault rather than allocating a fake class or supplying an
initial/default value. This matches the Go backend's checked `castRef` domain.
The existing structural live-view ABI is not widened by this operation.

### Root-object constructor reflection

An erased `REF TO object` is checked against the runtime described interface.
A separate, polymorphic native-constructor method dispatches to the object's
actual class descriptor, including inherited classes and builtin `Error`.
It is deliberately separate from the descriptor lookup used by `instanceof`:
structural storage is not evidence of a native constructor prototype.

Unbound receivers, unknown native storage, structural records, and live
structural proxies without a native descriptor **still raise explicit native
refusals**. Optional root receivers use the same boundary; absence does not
silently return an empty descriptor.

### Argument-bearing factories

`classvalue.newArgs` emits a checked dispatcher over known class names. The
caller evaluates the descriptor and argument array once before dispatch.
Factories decode arguments in parameter order and invoke the actual
constructor, including inherited constructors and lazy class initialization.
Missing entries are undefined; optional primitive wrappers retain absence,
and optional references retain an initial reference. Surplus entries have
already been evaluated by the caller and are ignored by the factory.

Supported parameter conversions are strings, booleans, binary64 Numbers,
Dynamic values, ClassValues, native class/interface/root references, and their
optionals. Required missing or wrongly tagged values are refused, not replaced
with zero/false/empty values. Native reference conversions retain the original
object and use checked casts. Elided wide field-copy constructors, including
inherited ones, use the existing field-write ABI rather than calling a
nonexistent ABAP constructor; primitive optional narrowing retains an explicit
presence check.

### Lexical identity

Each proven `FreshClassOf` evaluation allocates a new descriptor. Instances
created through that descriptor retain that exact descriptor in a lexical
slot. Constructor reflection and dynamic `instanceof` consequently distinguish
two evaluations of the same lexical class definition. Fresh descriptors have
the Go backend's empty parent/static-member metadata, not copied singleton
metadata. Ordinary singleton factories do not attach a fresh identity.

## Deliberately remaining refusals

There are no remaining static refusals for these four groups **in this
closure**, but the following dynamic/unproven domains remain fail-closed:

- Invalid nominal downcasts never invent structural class behavior.
- Root constructor reflection refuses missing/native-prototype-incompatible
  storage, as described above.
- Argument factories refuse abstract classes, variadics, unsupported parameter
  storage (`i32`, `i64`, arrays, maps, sets, RegExp, and optionals thereof),
  unknown constructor descriptors, and behavioral constructor calls above the
  existing 40-parameter envelope. Wide *field-copy* constructors are supported.
  Reference arguments require native reference boxes: parsed structural object
  or dynamic-array storage is not rebuilt/copied into a nominal reference.
- Fresh descriptors require a concrete scoped zero-argument class without
  ancestry. Non-scoped, abstract, inherited, and argument-bearing forms retain
  explicit emitter/verification refusals. Frontend capture/local-class limits
  remain unchanged; no captured environment or prototype behavior is invented.

The unchanged **72 other emitter refusals** are dynamic/nonfinite Number
`parseInt` (**66**), dynamic `i64` boxing (**4**), and erased structural
`object.has` (**2**). The remaining **113 runtime-operation occurrences** are
listed in the evidence. Operations owned by the other jobs were not enabled or
modified. Static counts do not measure their runtime reachability or test
impact.

## Validation and output parity

Green gates:

```sh
nice -n 10 go vet -p 2 ./...
nice -n 10 go test -short -p 2 ./...
ABAPITI_GRACECHECK=1 nice -n 10 go test -p 2 -count=1 \
  -run TestLexerFactsReport ./tsfront/
nice -n 10 go test -short -p 2 -count=1 \
  -run TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend ./cmd/abapiti/
nice -n 10 ./.github/ci/lint.sh gate origin/main
```

Environment: `GOCACHE=$HOME/.cache/go-build`, `GOTOOLCHAIN=go1.26.0`,
`GOFLAGS=-buildvcs=false`. Lint: **0 new issues**, canary **5/5**,
golangci-lint **2.13.2**. The lexer facts golden is **unchanged**; no regeneration
was needed.

`hir/abap/class_abi_test.go` and `class_wide_abi_test.go` inspect the emitted
ABAP for checked casts, constructor dispatch, absence/tag guards, all admitted
parameter domains, inherited/wide construction, builtin Error, lexical
allocation/attachment, and rejected storage/ABI cases. Seven exported fixture
sets (**100 ABAP files**) pass the repository's HIR ABAP syntax-check rules with
**0 issues**, using the installed pinned abaplint build and read-only
open-abap-core dependencies, not an OSG checkout. Existing TypeScript
class-view/local-class Node/Go oracle tests now also inspect the emitted ABAP
instead of asserting the obsolete blanket refusal. **ABAP runtime execution
was not performed.**

Default zabapgit Go output was generated from baseline and final executables at
the same output path, retaining the complete source/module/binary/site output:
**`diff -r` exit 0, 0 bytes, 9 files**. No Go emitter/runtime code changed.
