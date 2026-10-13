# ABAP storage work

Base: train O1 `63b91b7` (`origin/integ/o1`). Default Go emission is the
compatibility baseline. This branch owns array presence/deletion, record
deletion, native zero-argument calls, erased structural property presence,
LSP predicates, and shared live array storage. Chai, JSON and string emitters
and the suite harness remain outside this branch's scope.

The first foundation imports the focused nested child view implementation and
fixtures from `db9fbd3` (#133), without its Chai/JSON5 dependencies. Child reads
retain the parent's original graph, reuse cached activated layouts, preserve
unknown keys, and keep old aliases attached to replaced children. Nominal
classes with behavior still require a compatible native reference. This is
the appropriate foundation for arrays: their backing graph must likewise stay
shared rather than being projected into detached tables.

`TestNestedClassViewFixtures` passes through pinned OSG gogen and executes its
ABAP Unit assertions (one SUCCESS row). It covers alias identity, writes through
both graph and typed views, replacement, absent optional children, incompatible
parsed children and compatible nominal children. The explicitly activated
layout variant is also emitted by `TestExplicitNestedClassLayoutFixtures`.

The static inventory uses the existing full closure materializer and
`abaplint-tests --target osg --gap-only`, with `ABAPITI_INLINE=classic`.
The pinned upstream is `577f875ebec44cfaf64841cfe71c8ab8dc32622e`:
625 roots, 17 shards, 2,166 files. Pre-pass operation occurrences and post-pass
emitter refusals are separate inventories; no test-pass impact is inferred
from static sites. The foundation does not enable any assigned catalogue op.

Both foundation variants compile and execute on OSG
`f557e12f4217addcec7834b0607234aecf9b723a`; each returns one SUCCESS row.
[Compact host evidence](evidence/osgo-storage-foundation.json).
Default Go source and executable compared at the identical generation path
against O1: raw `diff -r` exit 0, zero bytes. The default runtime-op catalogue
gate also passes. No frontend, Go backend, golden or harness changes.

Foundation checks: `go vet ./...`, both emitted host units, the default
backend catalogue gate, uncached Grace lexer facts, and lint against O1 pass
(0 new issues, canary 5/5). The initial full short-suite invocation hit its
10-minute package timeout while running generated-Go frontend fixtures;
no assertion failure was reported before timeout. The later final run passes
with `GOFLAGS=-p=2` inherited by subprocesses and a 30-minute timeout. The
initial timeout itself is not recorded as a successful gate.

Sparse/native-call step: arrays now retain a separate missing-index set, so
preallocation creates holes and indexed writes mark only their target present.
Deletion clears the value without shrinking length; present undefined remains
present. Reverse, pop and whole-array slice preserve presence. Other sparse
mutation/search forms explicitly refuse before touching storage until their
variants are proven. Scalar direct reads/iteration and serialization refuse
holes rather than consuming padding as zero/false/empty strings. Reference
arrays retain the O1 shared object-table ABI and checked element casts.

`TestSparseStorageFixtures` and `TestNativeCall0Fixtures` compile and execute
on the pinned OSG host, each with one SUCCESS row. The former includes the
`Result.getNodes` backwards-fill shape; the latter checks virtual override
results, undefined and void value bridges, excluded accessor subclasses and
unknown names. Native calls use the same closed catalogue as Go and reject an
excluded subclass before a parent case can accept it. The emitter package,
focused static-gap catalogue test and package vet pass. Full final gates and
the final closure recount follow the remaining storage extensions.

The shared-array extension binds typed arrays to the original parsed graph.
Typed tables are read buffers: each read refreshes them, and every admitted
mutation writes back to the same graph node. Boxing a view recovers that node;
reference searches compare original identities rather than adapter objects.
Two views observe each other's writes, child records retain their unknown
keys and identities, and replacing an array leaves old aliases on the old
backing. Parsed null, present undefined and holes remain distinct in erased
reference storage. Publication and later reads both validate typed elements.
This implementation has O(n) buffer refreshes; suite performance is unmeasured.

Record deletion removes graph entries and uses a separate deletion mask for
representable optional/dynamic native slots. Reassignment restores presence.
Erased `object.has`/`hasOwn` use original record keys or explicit HIR `OwnKeys`
metadata, including a present undefined value; ordinary inherited Object
properties are recognized by `has`. Native shapes without a presence proof,
erased nominal objects, and erased prototype-value reads remain refused.
Deleting required native slots or slots that would reveal an inherited
prototype value remains refused. Dynamic deletion of arrays remains refused,
matching Go's separate record-deletion boundary; `array.delete` owns indices.

All four source-pinned LSP guards now run on graph and supported native data.
They retain vscode-languageserver-types 3.18's fractional integer-range check,
annotation/options checks, and the condition-versus-value boundary for
non-boolean falsy resource candidates. Null option/property receivers raise.
`TestLSPGuards` and `TestLSPErasedInNodeOracle` now execute their actual lowered
TypeScript on OSG as well as retain their Node/Go oracles. The unknown optional
factory-presence case still refuses. The concat oracle retains its JSON
serialization refusal, which is outside this branch's scope.

Executed storage units also cover poisoned aliases after publication,
non-array inputs, missing scalar reads, backwards preallocation filling,
scalar alias mutations, child identity and reference searches. Repeated
native dispatch, undefined boxing after a value, and changing erased receivers
are tested inside loops; all reusable temporaries are reset. Unsupported live
int64 writes refuse before changing the original graph, while native int64
arrays still work. Sparse shift/unshift, partial slice/splice, concat, search
and join still use explicit dense guards; optional-scalar comparisons and
live optional-array narrowing retain refusals. Array lengths/indices outside
the reviewed signed-i32 storage range refuse. Chai integration must reconcile
its old `e.chaiUsed` positive-preallocation guard with this ABI; the four real
statements tests are not credited here, and no Chai operation was enabled.

The final consumer audit refreshes live constructor argument arrays before
factory reads. Sparse `dynamic.materialize` array snapshots retain an explicit
refusal before they can turn holes into present slots. Their host fixture
checks both boundaries. Native structural records retain `OwnKeys` even when
the same layout is activated for live views; nominal live-view property reads
and presence use the original graph. Unknown slots of an inherited native
adapter refuse when its slot map is insufficient, rather than return a
fabricated missing value.

Final static inventory uses the unchanged M0 counting tool and the identical
manifest SHA-256
`8bc69eca3e2735470d1f5aa2b955127eaf378f13cd0b781c750e2493185fa189`.
Baseline is the O1 catalogue/census recorded in
[evidence/osgo-classview.json](evidence/osgo-classview.json); the full closure
still has 625 roots, 17 shards, 2,166 files, 7,052 classes and 85 interfaces,
with zero blocking diagnostics, HIR errors or frontend coverage traps.

| Assigned item | Before | After | Counting stage |
| --- | ---: | ---: | --- |
| `array.has` | 23 | 0 | Catalogue-refused HIR operation occurrences, before ABAP passes |
| `array.delete` | 0 | 0 | Same; covered by executed fixtures despite no closure occurrence |
| `dynamic.delete` | 3 | 0 | Catalogue-refused HIR operation occurrences |
| `dynamic.call0` | 6 | 0 | Catalogue-refused HIR operation occurrences |
| `lsp.isTextDocumentEdit` | 2 | 0 | Catalogue-refused HIR operation occurrences |
| `lsp.isRenameFile` | 2 | 0 | Catalogue-refused HIR operation occurrences |
| `lsp.isCreateFile` | 1 | 0 | Catalogue-refused HIR operation occurrences |
| `lsp.isDeleteFile` | 1 | 0 | Catalogue-refused HIR operation occurrences |
| Erased structural `object.has` | 2 | 0 | Distinct emitter refusals after ABAP passes |

The operation rows remove 38 catalogue-refused occurrences; the erased row
removes two transformed-HIR refusals. These are separate inventories, not a
combined pre-pass source-site count. Unsupported catalogue entries go from
24 to 16, present unsupported operations from 22 to 15, runtime occurrences
from 113 to 75, and other emitter refusals from 72 to 70. Nonfinite literal
sites remain zero. The remaining 70 are 66 dynamic/nonfinite Number `parseInt`
and four i64 boxing refusals, owned by the numeric work. Chai, JSON, string,
XML and `number.ceil` retain their catalogue refusals. Runtime guards described
above remain explicit; static zero does not certify all runtime inputs.

[Final compact evidence](evidence/osgo-storage.json) records the sanitized
inventory and 12 SUCCESS host units/probes through pinned gogen and OSG.
The branch was rebased onto `origin/main` at merged O1 `428a8d5` before final
gates. Vet, the full short suite with inherited `GOFLAGS=-p=2` and a 30-minute
package timeout, uncached Grace lexer facts, the default runtime-op gate and
lint against O1 pass. Grace goldens are unchanged; lint reports zero new issues
and canary 5/5. Default zabapgit Go sources, ancillary files and executable
remain byte-identical to O1 at the identical generation path (`diff -r`: exit
0, zero bytes). The initial foundation timeout was resolved by the bounded
parallelism/longer-timeout run, not counted as a pass. Class lookup and
inheritance indexes preserve emission order; all ten ABAP fixture source trees
are byte-identical before and after that performance change.

No harness/runner, Go backend, frontend production lowering or assertion
oracle was changed. No push was made. The four statements regressions and the
roughly 4,533 traps from the handoff were not rerun against an integrated
Chai/JSON5 train, so no test-pass impact or 11,060/11,060 ABAP certificate is
claimed. Whole-suite integration and measurements remain with abapiti.
