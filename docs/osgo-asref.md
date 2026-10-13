# Parsed nested Config views on TS-HA@OSGO

Base: `origin/osgo-storage` at `0a61c21`. The storage branch already covers
this boundary. No production emitter, runtime catalogue, frontend lowering,
Go backend or harness changes are required. This step adds a source-level
regression and executes the existing nested-view fixtures.

The handoff's `origin/main` JSON5 boundary evidence reported
`CX_SY_MOVE_CAST_ERROR` in `runtime.dynamic.as_ref` for the parsed object
`{"child":{"flag":true}}`, with Node returning `true`. The new
`TestParsedNestedObjectABAPHost` uses that exact JSON inside a Config-style
constructor assigning a private readonly typed field, then reads
`this.config.child.flag`. It also exercises direct assignment and assertion
conversions from parsed objects. The source is lowered by the existing
TypeScript frontend, emitted by the ABAP backend, compiled by gogen, and
executed as ABAP Unit on OSG. Its Node oracle and generated-Go execution agree.

The existing unpruned conversion boundary uses `dynamic.classView` for
structural class layouts. Nested structural fields activate their own layouts
and delegate reads and writes to the original parsed graph. Boxing recovers
that graph; equality compares original storage identities. Replacing a child
leaves old aliases attached to the old child. Native compatible references
retain their identity; view construction does not execute source constructors.
The nominal `dynamic.asRef`/`as_ref` path remains a checked-reference boundary.
Admitting arbitrary parsed records through it would bypass the proven layout
boundary and could invent prototype behavior.

The new source probe additionally checks writes through both typed and erased
aliases, repeated child identity, replacement, unknown fields, missing optional
values, and a present `undefined` optional field. Presence goes through the
existing `object`-typed `in` helper, so it reads storage metadata. A numeric
child receives Go's range refusal and ABAP's explicit view refusal. The
valid Node/Go/ABAP result is
`true:true:false:true:true:true:true:true:true:kept:replacement`.

`TestNestedClassViewFixtures` and `TestExplicitNestedClassLayoutFixtures`
each also compile and execute on OSG. These existing fixtures cover constructor
avoidance, absent optional children, null/number/string children, activated
data-only layouts, and compatible versus incompatible nominal children.
Together with the new source probe, three emitted ABAP Unit fixtures return
three SUCCESS rows on pinned OSG
`f557e12f4217addcec7834b0607234aecf9b723a`. These are executed host checks,
not merely abaplint syntax acceptance.

An expanded probe reached the existing unconditional `dynamic.keys` runtime
refusal *after* the nested cast and field reads succeeded. It was isolated from
the Config regression, not enabled or answered with fabricated keys. Optional
shape `in`, dynamic `any` `in`, and
`Object.prototype.hasOwnProperty.call` also retain frontend refusals; the
supported object-typed presence helper is used instead. Prototype-dependent
classes, private/accessor layouts and unrepresentable field values retain their
existing checked casts or explicit refusals. JSON5, Chai and general JSON
serialization remain outside this change's ownership.

Reproduce the executed fixtures with `ABAPITI_OSG_HOME` pointing to a checkout
at the branch's `.github/ci/osgo.ref` and inherited `GOFLAGS=-p=2`:

```sh
go test -p 2 -count=1 -run TestParsedNestedObjectABAPHost -v ./tsfront/
go test -p 2 -count=1 \
  -run '^(TestNestedClassViewFixtures|TestExplicitNestedClassLayoutFixtures)$' \
  -v ./hir/abap/
```

Final code gates pass: `go vet ./...`; `go test -short -p 2 -timeout 30m ./...`
with inherited `GOFLAGS=-p=2`; uncached
`ABAPITI_GRACECHECK=1 go test -p 2 -count=1 -run TestLexerFactsReport ./tsfront/`;
`TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`; and
`./.github/ci/lint.sh gate origin/integ/o1` (zero new issues, canary 5/5).
Grace goldens are unchanged. The initial short run captured the probe's earlier
unsupported presence syntax; it was red and is not counted as a successful gate.
The corrected final full short run passes, including the new regression.

Fresh default zabapgit Go generation, including its executable, is byte-identical
to a fresh build of `origin/integ/o1` at the identical generation path:
raw `diff -r` exit 0, zero output bytes across nine output files. No Go production
output, catalogue entry or golden was changed.

The fresh static recount uses the unchanged M0 counting tool,
`abaplint-tests --target osg --gap-only`, with `ABAPITI_INLINE=classic`.
It uses the same upstream pin `577f875ebec44cfaf64841cfe71c8ab8dc32622e`
and manifest SHA-256
`8bc69eca3e2735470d1f5aa2b955127eaf378f13cd0b781c750e2493185fa189`
as the storage certification: 625 roots, 17 shards, 2,166 files, 7,052 classes
and 85 interfaces. Blocking diagnostics, HIR errors, frontend coverage traps
and nonfinite literal sites are all zero. The raw census is byte-identical to
the storage branch's final census.

| Assigned item | Before (`0a61c21`) | After | Counting stage |
| --- | ---: | ---: | --- |
| `dynamic.asRef` | 0 | 0 | Catalogue-refused HIR occurrences before ABAP passes |
| `dynamic.classView` | 0 | 0 | Same |
| Nested structural class-field views | 0 | 0 | Distinct emitter refusals after ABAP passes |

These zeros count refusals, not total admitted uses. The historical native cast
trap was a runtime boundary, not a catalogue or emitter-diagnostic refusal.
Runtime guards, including the unconditional `dynamic.keys` refusal, are not
counted by M0's emitter-diagnostic census. No operation was newly enabled and
no `unsupportedOps` entry was removed in this step.

The full closure remains at 16 unsupported catalogue operations, 15 present
unsupported operations and 75 pre-pass runtime occurrences. The 70 distinct
post-pass emitter refusals are 66 dynamic/nonfinite Number `parseInt` cases
and four i64 boxing cases. These numeric cases need proven conversion/boxing
semantics; the other catalogue refusals belong to Chai, JSON/JSON5, strings,
XML and `number.ceil`. Neither inventory decreased because the assigned
structural boundary was already implemented on the requested base. Pre-pass
runtime sites and post-pass diagnostic sites remain separate inventories.
[Compact evidence](evidence/osgo-asref.json) retains the census and host results.

For a materialized full closure, reproduce the static recount with:

```sh
ABAPITI_INLINE=classic abapiti abaplint-tests "$CLOSURE" \
  --target osg --gap-only -o "$OUT"
```

The complete upstream statements shard was not rerun
on this storage-only branch, which still refuses JSON5 and Chai operations.
The historical 4,528 cast traps are not credited as recovered suite passes from
these focused probes. Integrated O3 suite measurement remains separate; this
is not an 11,060/11,060 ABAP-host certificate.
