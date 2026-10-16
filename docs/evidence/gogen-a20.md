A20 lowers expression sequencing, array assignment, and typed data-property
access for the step-4 gogen pilot. The base is
4671a0615ca3a10e278ab14c362c0ffcd6fb6a2d (integ/gogen-stack4, A16 through A19).

Comma expressions evaluate the left operand for effects before yielding the
right operand. Void expressions evaluate their operand once and yield undefined.
Both use existing HIR Seq nodes and require no new backend operation.

Array destructuring assignment evaluates the RHS once, evaluates each target
reference before consuming its element, and returns the original RHS when used
as an expression. Literal swaps capture all RHS values before any assignment;
assignment from an aliased array reads its live elements in order. Missing
primitive local bindings retain undefined, including later literal swaps and
inferred array assignment results. Omissions advance the iterator, and an
exhausted iterator stays exhausted even if a later target grows the array. Non-
array storage and unsupported target patterns retain located refusals.

Optional family properties check receiver presence before reading. Exhaustive
switch diagnostics can read a family's original tag even when checker flow is
never. Family-valued fields remain eligible for flow projection inside inferred
collection rows. Narrowed payloads are matched against the receiver's own
registered family, rather than a structurally equal leaf from another family. A
property read already projected to a payload uses that payload's field ABI.
Plain literals stored as object retain live backing records, so LoopToken cursor
mutation is visible through existing aliases. Typed returned functions reuse the
stored-callable ABI. Dynamic module loading and any callees remain outside this
change.

Sixteen Go-vs-Node oracle tests cover effects/order, undefined, swaps, live
array aliasing, missing elements, assignment values, target-reference effects,
iterator exhaustion, returned functions, optional/nested/mapped/sibling
families, and object cursor aliases, absent-value swaps and returned RHS array
observations. The initial comma, void, array assignment, returned-call,
exhaustive-tag, optional-family, cursor, missing-element and assignment-value
probes failed before their fixes. A later exhaustion probe reproduced a live-
array edge case before its fix. The final absent-swap and object-read probes
also failed before their fixes. Ten final emitted ABAP programs compile through
.github/ci/abap-fixture-compile.mjs on OSG
f83dc5c3e05bfaea71268fd6ad0e9ff88532cce6 with no errors; family programs retain
located ABAP refusals.

Each pilot site was inspected with the TypeScript checker. Fourteen G sites are
listed in /mnt/ssd-510/osg/abapiti/tmp/gogen-a20/g-sites.tsv, with
receiver/source origins retained in inspect.log. These include unannotated step,
run, methodsOf, project and helper parameters, the untyped cdsObservers WeakMap,
and Require calls whose module exports are any. The typed local walk failure
descends from an unannotated helper object. No OSG source annotations were
changed.

Pilot inputs are the requested copied OSG c07c67a3, abaplint 577f875e, and the
prepared step-4 driver. Coverage is unchanged: 56 files, 51 loaded, 422 bodies,
226 executed, three unmatched. Node reference SHA-256:
a13702d55b1eecdb200078205da5909961b4b49ce0f6c3fe3bdbf08e802d6227.

Base measurement:

```
full: 199 blocking (145 root, 54 cascade), 1685 HIR errors in 36 files
146.25 s 1460488 KB
root top: [('method X on dynamic is not lowered', 26), ('property X is not lowered', 15), ('call to KindIdentifier is not lowered', 10), ('identifier X is not a lowered local or module value', 9), ('writes through a mixed index signature require an exact named-slot ali', 8), ('checker type X is not lowered', 6)]
```

All builds, tests, Node programs, abapiti commands and pilots use systemd-run
--user --scope -q, MemoryMax=16G, nice, GOTOOLCHAIN=go1.26.0,
GOFLAGS=-buildvcs=false, GOCACHE=/mnt/ssd-510/osg/go-build and
TMPDIR=/mnt/ssd-510/osg/abapiti/tmp/gogen-a20. An earlier full-Grace run also
passed under MemoryMax=24G. Heavy commands run sequentially; go tests use -p 2
-timeout 40m. Small logs and diagnostics remain in that scratch directory after
large generated outputs and copied checkouts are removed.

Validation: gofmt and go vet -p 2 ./... pass. The mandatory short suite passes
(final tsfront run: 346.142 seconds). ABAPITI_GRACECHECK=1 TestLexerFactsReport
passes; GRACE_FACTS_OUT changed only
op_aliases/op_allocates/op_may_raise/op_writes 172 -> 173 and op_reads 252 ->
253. All other lexer facts are unchanged. ABAPITI_GRACE_FULL=1
TestGraceFullRegistryClosure passes in 197.62 seconds: 1,927 classes, 64
interfaces, zero lowering/verification errors, methods pure=3491,
may_throw=3767, defined=6257; virtual sites single=3937, total=5097; static
writes memo=0, counter=0, other=171. These class/method counts and summary facts
match the base.

Final measurement on the validated implementation:

```
full: 179 blocking (125 root, 54 cascade), 1641 HIR errors in 36 files
133.33 s 1689660 KB
root top: [('method X on dynamic is not lowered', 26), ('call to KindIdentifier is not lowered', 10), ('identifier X is not a lowered local or module value', 8), ('writes through a mixed index signature require an exact named-slot ali', 8), ('checker type X is not lowered', 6), ('sort needs an inline or lowered module comparator', 6)]
```

The pilot loses 20 blocking diagnostics and 20 root diagnostics, with 44 fewer
HIR errors. The targeted emitter type/s/k/c/len, idxVar assignment, comma, void,
and array-assignment diagnostics are gone, including emit-int8 calc. The
remaining property refusals include frontend ArithmeticBranch.reason, ir-writes
expr, and the excluded class-value railroad read. No bigint, generic
PropertyValue mapping, classes-as-values, classref element access, or
Object.create implementation was changed. The overall self-hosting pilot remains
blocked. Reference bytes and coverage are identical to the base.

TestAbaplintCoreTrappedBodies passes in 64.22 seconds. Final full-core emission
retains 2,052 classes and 77 interfaces, with zero lowering/verification errors.
The sum of grep -rc 'assignment type mismatch' occurrences is exactly two. All
seven default zabapgit Go source files are byte-identical to the base; their
SHA-256 hashes are retained in zabapgit-sources.sha256. M1 passes after copying
the provided multifile driver and rebuilding: output is byte-identical to
expected-clean.txt, SHA-256
95d37bc666403557ceecbb6f0e12e31867c18d3c2870e105e3ceafb5b9267428.
