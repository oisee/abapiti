A15 completes lexical capture discovery across cycles of lifted local functions.
The complete reachable input set is collected before a method signature is
published or a body is lowered. Recursive callees receive the existing shared
cells for mutable checked bindings; function declarations remain callable
before their source declaration. This applies to full/unpruned lowering and
preserves the legacy pruned build.

The `param` site is A (abapiti), not G. Its baseline lowering diagnostics were:

```
tools/gogen/frontend.mjs:3670:65: capture intf has no checked binding
tools/gogen/frontend.mjs:3672:49: call to KindIdentifier is not lowered
```

The root is `const [intf, meth] = name.includes("~") ? name.split("~") :
[null, name]`. The initializer's union lowers to Dynamic, but array
destructuring previously treated it as a tuple class and read nonexistent
`f0`/`f1` fields with Void types. In addition, the tuple-contextualized literal
could silently return no lowered initializer. Tagged array literals now
allocate boxed element storage, the initializer is coerced to its declared
storage, and checked array unions are destructured through a live dynamic
array view. Pattern declarations record checked symbols so their mutable
captures use the same cell rewrite as ordinary declarations. Unsupported
boxed rest/unchecked dynamic bindings retain located refusals.

Red Go-vs-Node tests ran on the untouched base before final fixes. Oracles
cover two- and three-function cycles, transitive-only captures, function
hoisting, escaped function values, later writes, and shared destructured
array/object bindings. ABAP fixture-export helpers emit the recursive cases;
the boxed-array case has a located ABAP refusal (`dynamic array view across
native element storages`) rather than casting incompatible native tables.

Reproduction uses GOTOOLCHAIN=go1.26.0, GOFLAGS=-buildvcs=false,
GOCACHE=/mnt/ssd-510/osg/go-build, nice, and go test -p 2. Pilot inputs use
OSG 8998148d, abaplint 577f875e, the requested measure-step4.sh kit, and
systemd-run --user --scope -q -p MemoryMax=16G for both base and changed runs.
Base compiler: 8e636df (integ/gogen-stack2, A13+A12).

Validation commands:

```
gofmt -w tsfront/gogen_a15_test.go tsfront/lower_class2.go tsfront/lower_expr.go tsfront/lower_hof.go tsfront/lower_stmt2.go hir/verify.go hir/abap/emit.go
go vet -p 2 ./...
go test ./tsfront/ ./hir/... ./cmd/abapiti/ ./internal/abapfixture/ -short -p 2 -timeout 30m
ABAPITI_GRACE_FULL=1 go test ./tsfront/ ./cmd/abapiti/ -run 'TestGraceFullRegistryClosure|TestAbaplintCoreTrappedBodies' -p 2 -count=1 -timeout 30m -v
```

A ten-minute short-suite attempt expired without assertion failures; the
complete suite uses the
explicit 30-minute limit above. All commands use the environment and priority
specified above.


Pilot results (both measured on OSG 8998148d):

```
base: full: 419 blocking (279 root, 140 cascade), 1748 HIR errors in 38 files
A15:  full: 441 blocking (289 root, 152 cascade), 1744 HIR errors in 38 files
```

Base: 230.38 s, 1,613,192 KB peak RSS. A15: 581.82 s, 1,400,904 KB.
Reference SHA-256 is unchanged:
`61618e3b5618ce52b919a37e0e746765f2c2c2631bd35268e3debcfc7847be17`.
Both runs cover 58 files, 53 loaded files, 429 bodies, 217 executed bodies,
and three unmatched coverage entries.

The final diagnostic audit has no entries at frontend.mjs:3670:65 or
3672:49, nor sink-locals.ts:97:18, 106:7, or 110:3. Both targets are A and
resolved for the requested call/capture ABI. Scoped identifier-call refusals
drop from 28 to 26. Nine distinct source diagnostics disappear and 31 appear;
the overall pilot remains blocked. The newly visited scan body exposes
conditional joins at sink-locals.ts:73/74 and array length assignment at :89.
Checked-array guards also locate unsupported dynamic destructuring at
emit-go.mjs:967, frontend.mjs:2562, and sink-locals.ts:112; their dependent
identifier/assignment diagnostics replace previously malformed HIR. This is
not a claim that either whole module is executable.

Final Grace full and TestAbaplintCoreTrappedBodies pass. Grace retains 1,927
classes and 64 interfaces with zero lowering/verification errors. Its facts
match base exactly: methods pure=3491, may_throw=3767, defined=6257; virtual
sites single=3937, total=5097; static writes memo=0, counter=0, other=171.
The final full-core generation has assignment-mismatch sum 2, and hir.go /
hir_profile.go match base byte-for-byte. All seven final default zabapgit Go
sources match base byte-for-byte. The oracles and vet pass.

M1 passes on the final generated core: copy
/mnt/ram/abapiti/selfhost-m1/go-multifile/main.go into verified-lint/go/main.go,
build with go build -o zabaplint-mf ., run from selfhost-m1/osg-check-kit
with --file files.txt --config abaplint.json, then cmp against
expected-clean.txt. The files are byte-identical.

The complete requested short suite passes on the final revision (tsfront:
1350.987 s); all requested hir, cmd/abapiti, and internal/abapfixture packages
pass as well. Gofmt and git diff --check are clean. Large scratch checkouts,
generated Go modules, and binaries are removed after validation; small logs,
pilot summaries/diagnostic evidence, ABAP fixtures, and hashes remain under
/mnt/ram/abapiti/tmp/gogen-a15.
