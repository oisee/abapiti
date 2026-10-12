# Last conversion-domain refusals

Branch: `feat/fit-last-absence`, based on
`origin/integ/train-11@b4cba6363f96073e870f0bfb45252397b085ad55`.
Original upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.
Only `abap/types`, `root`, `objects`, and `rules` were measured, against fresh
original Node. No upstream source, test title, assertion, or registration was
changed. The 17-shard suite was not run.

**11 of the 12 listed tests recover. Listed refusals: 12 → 1. Total touched-shard
traps: 14 → 3. Node-active passes: 3,400 → 3,411 / 3,416. FAIL 0 throughout
both completed measurements; async-pending 0.** The two active unregistered
line_length cases and 11 Node-pending cases are unchanged.

| Shard | Pass before | Pass after | Trap before | Trap after | FAIL after |
| --- | ---: | ---: | ---: | ---: | ---: |
| abap/types | 28 | 29 | 1 | 0 | 0 |
| root | 91 | 93 | 3 | 1 | 0 |
| objects | 138 | 142 | 6 | 2 | 0 |
| rules | 3,143 | 3,147 | 4 | 0 | 0 |
| Total | 3,400 | 3,411 | 14 | 3 | 0 |

## Exact listed tests

| Group | Test title | Final status |
| --- | --- | --- |
| 4 | Rule: dynpro_checks Element SDFSDF length longer than 132 | passed |
| 4 | Rule: dynpro_checks overlapping dynpro fields | passed |
| 5 | malformed object XML ignores structured values where text is expected | passed |
| 5 | Message rule no number | passed |
| 6 | exclude list will exclude issues based on the local exclude patterns | passed |
| 6 | severity an issue will have the severity specified in the config | passed |
| 11 | malformed object XML ignores malformed inner object metadata | trapped; unchanged |
| 12 | test line_only_punc rule "." should have 1 issue(s) | passed |
| 13 | Types, TYPES TYPES structure should sometimes inherit qualified name | passed |
| 17 | Domain, parse main xml parser error | passed |
| 18 | Domain, parse main xml parser error, valid xml | passed |
| 19 | View, parse XML database view with 2 append views | passed |

[Per-title evidence](evidence/last-absence-recovered.json) retains each exact
baseline diagnostic, final status, and all three remaining touched-shard traps.

## Repaired boundaries

Named type aliases now consume the same precomputed primitive absence metadata
as anonymous shapes and data interfaces. Dynpro `NAME` and View `KEYFLAG` retain
undefined in their existing row storage, including own-property presence.
The object-literal lowering implementation is unchanged.

Primitive parameters copied into fields that the source checks for undefined
retain absence across the call ABI. This includes Message's private fields.
The original constructor receives undefined and performs its own empty-string
defaulting. The caller never invents an empty string or the string "undefined".
Parameter absence metadata is separate from field metadata, so native parameter
properties keep their constructor ABI.

Boolean fields used as conditions can retain undefined on live config views.
The original `if`/short-circuit expression then performs JavaScript truthiness.
The backing config stays live and omitted keys stay omitted; no default config
is reconstructed. This widening is bounded to boolean fields, excluding native
parameter properties. String layouts are unchanged by the truthiness analysis.

LineOnlyPunc constructs a provisional fix before discarding it for the first
row. The previous row's optional length is undefined. Numeric arithmetic now
performs `ToNumber(undefined) = NaN` on optional numeric operands in the
unpruned binary64 path, using the existing NaN-producing runtime operation.
Both operands evaluate once, left to right, before coercion. The source's NaN
flows into the provisional Position; no zero or other default is substituted.
The assume-integer and default pruned paths retain their prior behavior.

The Types helper asserts an actual Program as Class, then passes it to
SyntaxLogic's genuine ancestor parameter. The existing nonescaping common
ancestor proof now handles constructor arguments and preserves evaluation
preludes. It retains the actual object and its dispatch.

The two Domain tests actually load `.dtel.xml` files. Original Node returns
**DataElement**, and its own `parseType` returns **UnknownType** for both XML
inputs. The asserted Domain check was therefore wrong. A nonescaping assertion
used solely through compatible public method calls now uses a certified sibling
method union. It dispatches DataElement's real method, including the existing
optional-tail bridges, without fabricating a Domain or borrowing its body.
Escaping values, field-specific uses, writes, and incompatible method signatures
remain outside this proof.

Node differential probes cover row own-key presence, constructor defaulting,
live config identity and mutation, indexed ancestor arguments, actual sibling
method dispatch, NaN, operand order, and stable string layouts. The existing
common-view refusal probes also pass. No runtime operation was added.

## Remaining and ownership

Group 11 is **not an absent value** in the pinned original test. Node observes
an own `R3_NAME` property whose value is `{nested: ""}`, with type `object`.
Its `typeof` guard returns an empty generated-object list. Go's live optional
string read refuses that structured value before the guard can observe it.
This needs preservation of erased nonstring values through the guarded read;
it is left to the explicitly excluded `typeof` work. No default, skipped input
row, or weakened assertion is supplied.

[Original Node observations](evidence/last-absence-node.json) record both real
DataElement/UnknownType pairs and the exact ProxyObject value.

The other remaining touched-shard traps are the unchanged Domain strict-JSON
error boundary and Registry native config deletion. The two unregistered
line_length cases remain with their owner. No claim is made about unmeasured
shards or the current whole-suite pass count.

## Full closure and gates

`abaplint --full` is present in `origin/main`. The exact recorded command is:

```sh
GOMAXPROCS=2 abapiti abaplint --full "$UPSTREAM" -o "$SCRATCH/full-before"
GOMAXPROCS=2 abapiti abaplint --full "$UPSTREAM" -o "$SCRATCH/full-after"
```

Full-closure blocking diagnostics stay **26 → 26: 17 root + 9 cascade**.
HIR errors stay **67 → 67 in 19 files**. Exact blocking-message counts and
categories are unchanged. The runtime conversion-domain message has **0 → 0**
static blocking diagnostics; its runtime counts are measured separately above.
[Before](evidence/last-absence-full-before.json) and
[after](evidence/last-absence-full-after.json) retain the complete message counts
and located diagnostics.

All gates passed with `GOMAXPROCS=2` and package concurrency at most two:

```sh
go vet -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -timeout 30m -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 go test -short -timeout 30m -p 2 ./tsfront \
  -run 'Test(LexerFactsReport|EmitRegistryFeatures|EmitRegistrySingletons)$'
go test -short -p 2 ./cmd/abapiti \
  -run TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend
./.github/ci/lint.sh gate origin/integ/train-11
```

Lint: zero new issues, canary 5/5, pinned golangci-lint 2.13.2. Grace lexer
facts remain byte-identical; no golden regeneration was needed. The initial
broad truthiness prototype caused local layout/registration verification
failures, which were corrected before final certification. An initial short
run also hit its default ten-minute timeout under contention; the final complete
short suite passed with `-timeout 30m`. Superseded intermediate shard runs were
stopped and are not counted as measurements. No genuine Node-active FAIL was
accepted.

## Reproduction and default output

Use the pinned clean upstream with locked dependencies installed in both its
root and `packages/core`. Scratch paths are disposable; for example:

```sh
export GOMAXPROCS=2
SCRATCH=${TMPDIR:-/tmp}/fit-last-absence
UPSTREAM=${TMPDIR:-/tmp}/abaplint-upstream
mkdir -p "$SCRATCH"
git worktree add --detach "$SCRATCH/base" b4cba63
go -C "$SCRATCH/base" build -p 2 -o "$SCRATCH/before-abapiti" ./cmd/abapiti
go build -p 2 -o "$SCRATCH/after-abapiti" ./cmd/abapiti

ABAPITI_TESTS_COMPILER="$SCRATCH/before-abapiti" \
ABAPITI_TESTS_COMPILER_COMMIT=b4cba63 \
ABAPITI_TESTS_SHARDS=root,objects,rules,abap/types ABAPITI_TESTS_WORKERS=1 \
node tools/abaplint-tests-all.mjs "$UPSTREAM" "$SCRATCH/before.json"
ABAPITI_TESTS_COMPILER="$SCRATCH/after-abapiti" \
ABAPITI_TESTS_COMPILER_COMMIT=HEAD \
ABAPITI_TESTS_SHARDS=root,objects,rules,abap/types ABAPITI_TESTS_WORKERS=1 \
node tools/abaplint-tests-all.mjs "$UPSTREAM" "$SCRATCH/after.json"

GOFLAGS=-p=2 "$SCRATCH/before-abapiti" abaplint "$UPSTREAM" \
  -o "$SCRATCH/default" --target all,go
mv "$SCRATCH/default" "$SCRATCH/default-base"
GOFLAGS=-p=2 "$SCRATCH/after-abapiti" abaplint "$UPSTREAM" \
  -o "$SCRATCH/default" --target all,go
diff -r "$SCRATCH/default-base" "$SCRATCH/default"
```

Default output versus the task-specific train-11 base is **raw diff exit 0,
zero bytes**, including the executable and ABAP archives. The frozen comparison
emitted the branch first and then the base at the same absolute output path;
the reproduction above reverses that order with the same comparison.

Origin/main was independently built and compared too. Its output already
differs from train-11 in five files: `go/hir.go`, `go/hir_profile.go`,
`go/runtime.go`, `go/zabaplint-go`, and `sites.json`, including inherited
constructor-descriptor support. This branch adds no default-output difference
to train-11; it does not revert those other owners' integration changes.

[Baseline shards](evidence/last-absence-before.json),
[final shards](evidence/last-absence-after.json), and
[verification](evidence/last-absence-verification.json) identify the frozen
compiler by executable SHA-256 and exact Git source blobs. The final binary was
built from the worktree over train-11; its runner revision argument is recorded
separately from its implementation identity.
