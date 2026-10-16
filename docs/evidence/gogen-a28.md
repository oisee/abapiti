A28 starts at 15aaa384 (feat/gogen-a26). The measurement uses the requested
copied OSG 1638d1cb, the pilotkit measure-step4.sh driver, and unchanged
abaplint/transpiler inputs. The Node reference is a13702d55b1eecdb. Coverage
is 56 files, 51 loaded, 422 bodies, 226 executed, and 3 unmatched.

The base measurement is:

```
full: 101 blocking (74 root, 27 cascade), 1490 HIR errors in 31 files
```

Guarded heterogeneous object unions keep their original property-presence ABI.
Family reads prefer actual flow subsets to a retained full-family alias.
Checker-certified partial/subset reads can return the live tagged field value;
uses apply checked projections. Reassigned, unrefined families retain the
existing narrowing refusal. `typeof` recognizes complete structural families
as objects. Property reads on mapped data layouts can use the stored field
name when the checker returns a transient property symbol.

Local-to-Field adaptation preserves the original object and extra keys. A
required checker property whose storage was widened for an erased initializer
uses the existing checked live-field read; absence is never fabricated into a
primitive. Structural Map joins use `map.view`, with invariant key types and
recursively equivalent data layouts (including arrays of equivalent records).
Views delegate gets, sets, deletes, sizes, keys, values and insertion counters
to original storage. Boxing retains original identity, including repeated row
reads used as Map/Set keys. Nested Maps/Sets and primitive storage changes do
not gain covariance. `union.check` validates erased values against complete
family tags and fields; optional absence is preserved before admission.

Fresh unparameterized conditional Maps adopt the other branch's storage,
including constructor markers and checked wrappers. An inferred JavaScript
const whose checker type is Map<any, any> keeps the exact Map storage produced
by its joined initializer; all uses retain ordinary checked coercions. Immediately consumed
Map/Set empty-array fallbacks join iterator elements. A direct Map loop keeps
the existing live-value/deletion behavior and located refusal on appending an
entry during iteration. Tuple-valued map callbacks honor their explicit result
annotation, retaining the array-of-tuples ABI through flatMap and spread.

RegExp replacement callbacks receive match, numbered captures, UTF-16 offset,
original input and, when present, named groups. Literal and const-literal
patterns fix capture arity. The existing checked regex runtime performs all
matching before user callbacks, preserving lastIndex and empty-match behavior.
Callback return values use checked JavaScript string conversion. Unreviewed
regex languages and unsupported callable/storage domains retain refusals. The
exact binding-scanner comment/string pattern and its non-newline mask are
reviewed in the usage-gated iterator grammar. An additional red oracle caught
both patterns; it now covers lazy block comments, escaped quotes, templates,
UTF-16 masking and CRLF.
The used console.error overload is exactly one string: it writes that string
and a newline to stderr, with literal percent sequences and Node UTF-8 output
semantics. Other overloads refuse; local console bindings remain ordinary code.

Default sort uses JavaScript string keys and UTF-16 comparison. Plain record
arrays compare as `[object Object]` and remain stable. The real RTTI site sorts
map-entry tuples whose second elements are arrays of records: the entire tuple
string, including the record-array commas, participates in sorting. Existing
checked conversion hooks handle or refuse custom coercion; no object identity
or native Go formatting is used as a sort key.

Twenty-six Go-versus-Node oracles cover these paths, with a separate refusal
test for a collection mapFn that requires interleaved live iteration. The initial red run demonstrated the
heterogeneous in-guard, callable replacement, empty fallback and object-sort
refusals. Additional red cases covered Dynamic/family joins, direct-loop empty
fallbacks, tuple flatMap and family typeof. Supported ABAP outputs are exported
and compiled with the pinned OSG bridge; Go-only operations have explicit,
source-located ABAP refusals. No new ABAP substring/regex construct is introduced.

The G row and nativeRttiData remain outside this change. G candidates and
measurement/validation logs are retained only in the requested scratch directory.

Validation uses Go 1.26.0, GOFLAGS=-buildvcs=false, the shared SSD GOCACHE,
and only the requested A28 scratch directory. Every heavy command runs in a
systemd user scope under nice, capped at 16 GiB (24 GiB for the full tsfront
closure), with a preflight SSD free-space check. Tests use -p 2 and a 40-minute
timeout. Every preflight check exceeded the required 40 GiB free-space threshold.

The lexer golden changes only op_aliases, op_allocates, op_may_raise,
op_reads and op_writes for the new catalogue entries. The full A26 closure is
also measured independently from a scratch archive; comparison excludes only
operation counters, keeping all class, method and other facts byte-identical.

Final-source gates completed:

- gofmt and git diff --check; go vet ./... .
- go test ./tsfront/ ./hir/... ./cmd/abapiti/ ./internal/abapfixture/
  -short -count=1 -p 2 -timeout 40m: all pass (tsfront 374.953 s).
- ABAPITI_GRACECHECK=1 TestLexerFactsReport: pass with the permitted golden.
- ABAPITI_GRACE_FULL=1 TestGraceFullRegistryClosure under 24 GiB: pass
  (213.51 s), zero blocking/verification errors, 1,927 classes and 6,278
  methods. All non-operation facts match the independently measured A26 base.
- ABAPITI_GRACE_FULL=1 TestAbaplintCoreTrappedBodies: pass (60.43 s),
  with pinned expectations unchanged.
- abapiti abaplint /home/alice/dev/abaplint --target go: 2,052 classes,
  zero HIR verification errors, generated Go builds. The sum of grep -rc
  'assignment type mismatch' over go/*.go is exactly 2.
- M1: copied the supplied multifile main.go, built zabaplint-mf, ran it
  in the supplied osg-check-kit with files.txt and abaplint.json; cmp against
  expected-clean.txt passes byte-for-byte.
- Default zabapgit generation at the same output path with the A26 and A28
  binaries: all eight Go source/module files are byte-identical (SHA-256 map
  comparison, including the file set).
- ABAPITI_ABAP_FIXTURE_DIR export with all TestGogenA28 tests: pass; the pinned
  .github/ci/abap-fixture-compile.mjs bridge on OSG f83dc5c3e05bfaea71268fd6ad0e9ff88532cce6
  compiles all 18 emitted fixtures (12 unique programs), without errors.

The concluding pilot has the same OSG/ref hashes and coverage as the base:

```
full: 61 blocking (49 root, 12 cascade), 1357 HIR errors in 30 files
```

That removes 40 blocking diagnostics (25 roots and 15 cascades) and 133 HIR
errors. The original emit-go:334 Map join is now absent. The measurement still
has refused bodies and HIR errors; disappearing root diagnostics are not a
claim that the whole pilot emits or executes successfully.

Remaining assigned-file roots are:

- tools/gogen/emit-table-move.ts:51:27: property args is not lowered on interfaceref<union.ff1ba6d00dd4492997b525fd1b4905b4a15841b6184d783402ebf9a9306b75eb>.
- tools/gogen/emit-table-move.ts:80:41: call to KindIdentifier is not lowered.
- tools/gogen/sink-locals.ts:39:23: property e is not lowered on unionref<tools/gogen/sink-locals.ts.Expression>.
- tools/ir-ranges.mjs:67:95: reduce without an initial value needs a callback returning the element type.

The args/e property projections and reduce accumulator are newly exposed
refusals. The node:util call remains the original G row. No OSG source or
nativeRttiData block was edited. Detailed per-site comparison is in scratch
at a28-root-audit.tsv; G candidates remain in g-sites.tsv. Base/final pilot
summaries, diagnostic evidence, facts, hashes, fixture exports and gate logs
are retained there. Large copied checkouts, generated Go trees, binaries,
compile outputs and V8 coverage are removed after validation.
