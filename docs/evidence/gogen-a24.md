A24 specializes synchronous module helpers whose unconstrained type parameters
occur in Map storage or callback/record containers. The baseline is
bc511e727a257a8fdea00927b77d597b212ea065 (feat/gogen-a24 before this change).
The checker supplies inferred type arguments and instantiates nested types;
concrete signatures and bodies retain native storage. Names hash the checker
argument spelling and lowered layout. Repeated calls reuse a method; a worklist
handles generic forwarding and recursion. Const arrows retain their existing
initialization guard. More than 64 instantiations of one helper receive a
located refusal. Boundary erasure and generic append keep their existing paths.

The memo expression's possible absent Map lookup remains optional in its
concrete ABI, even when JSDoc omits undefined. No copied/tagged view of caller
Map storage is introduced. Map order, shared identity, object payload mutation,
callback execution, and array payload aliases are covered by Go-vs-Node oracles.

The original two JSDoc oracles failed before implementation at the requested
Map<T>, callback record, type-parameter, and missing-value-local diagnostics.
Five final oracle tests pass, including two type parameters, generic forwarding,
imported helper reuse, and nested erased boundary wrappers. Three emitted ABAP
programs compile without errors through .github/ci/abap-fixture-compile.mjs on
OSG f83dc5c3e05bfaea71268fd6ad0e9ff88532cce6. WeakMap retains its existing
source-located ABAP refusal, checked by a separate Node oracle.

Both pilot runs use the requested copied OSG 1638d1cb and pilotkit script,
with abaplint and transpiler inputs unchanged. The Node reference is
a13702d55b1eecdb; coverage is identical: 56 files, 51 loaded, 422 bodies,
226 executed, 3 unmatched.

```
base: full: 164 blocking (126 root, 38 cascade), 1585 HIR errors in 32 files
A24:  full: 153 blocking (118 root, 35 cascade), 1573 HIR errors in 32 files
```

All eleven diagnostics at frontend-hierarchy.mjs:8 and
frontend-queries.mjs:40-47 disappear; no new blocking diagnostics appear.
Baseline lowering: 145.16 seconds / 1801252 KB. Final A24: 141.22 seconds / 1788520 KB.

Validation logs are retained under /mnt/ssd-510/osg/abapiti/tmp/gogen-a24.
Heavy steps run sequentially under systemd user scopes with MemoryMax=16G
(24G for full Grace), nice, Go 1.26.0, GOFLAGS=-buildvcs=false, and the requested
GOCACHE/TMPDIR. Each heavy step checks at least 40G of free SSD space.

Changed Go files are gofmt-clean; go vet ./... and the required short suite
pass (final tsfront run: 374.975 seconds). A repository-wide gofmt scan also finds six
unrelated files with formatting differences already present in the baseline;
these and the explicitly excluded cmd/abapgit-pack/main.go are unchanged.

The additional opt-in TestLexerFactsReport fails identically on base and A24.
Both exported fact reports are byte-identical. Relative to the checked-in
golden, assign, escapes, expr, flow, local, and operation counters differ;
this is not an A24 change and the golden is left untouched.

ABAPITI_GRACE_FULL=1 TestGraceFullRegistryClosure passes in 200.90 seconds,
with 1,927 classes, 64 interfaces, 6,278 methods (6,257 defined), and zero
lowering/verification errors. Summary facts: pure=3479, may_throw=3767;
virtual sites single=3937, total=5097.

TestAbaplintCoreTrappedBodies passes in 55.72 seconds. The default zabapgit
Go build succeeds on base and A24; all seven .go source files are byte-identical
(a SHA-256 manifest is retained in scratch).

The required full-core CLI Go emission/build succeeds: 2,052 classes,
77 interfaces, zero blocking diagnostics and HIR verification errors. Summing
grep -rc 'assignment type mismatch' across emitted go/*.go yields exactly 2.

The supplied M1 multifile main builds against the emitted full-core sources;
running it in osg-check-kit with files.txt and abaplint.json succeeds and its
output compares equal to expected-clean.txt. Const helper calls place the
initialization guard before any extracted argument-evaluation preludes, matching
the existing direct-call path.

A baseline-only full lowering also exports byte-identical Grace facts, including
all class/method counts. This reuses TestGraceFullRegistryClosure in the scratch
baseline with only its final expensive invariant-engine call skipped; the
candidate runs that invariant engine in full. The final const-call ordering
adjustment is outside these pruned paths and full-core emitted Go contains no
container-specialization methods.

After the const-call ordering review, all five oracles, all three emitted ABAP
fixture compiles, go vet ./..., and the complete required short suite pass again.

Large scratch checkouts, generated Go trees, compiled fixtures, binaries, and
V8 coverage outputs were removed after validation. Small logs, fact reports,
final ABAP fixtures, manifests, and pilot evidence remain in scratch.
