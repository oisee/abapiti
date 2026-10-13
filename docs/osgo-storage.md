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
no assertion failure was reported before timeout. It is being rerun with
`GOFLAGS=-p=2` inherited by subprocesses and a 30-minute timeout. This timeout
is not recorded as a successful gate.

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
