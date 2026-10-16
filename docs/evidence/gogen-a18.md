A18 lowers mixed tagged logical operands and the gogen pilot's numeric,
environment, Object.values/fromEntries, and Array.from operations.

Logical expressions retain the selected operand's value and reference identity.
Existing expression-local Seq nodes keep RHS pending statements in the selected
conditional arm. Tests compare Go with Node for zero, negative zero, NaN, empty
strings, null, undefined, false, truthy primitives, empty arrays and objects,
mixed native operand types, and RHS callbacks with observable side effects.

Number predicates inspect numeric tags without coercion. Math.abs uses binary64;
Infinity and NaN use the existing nonfinite runtime. Safe-integer constants
already existed. Environment reads use os.LookupEnv on every read, preserving
the distinction between an unset variable and an empty string. Builtin dispatch
resolves symbols so locally shadowed names retain their own behavior.

Object.values enumerates own properties in JavaScript order and preserves
element references. Tagged fromEntries destinations retain an ordinary object,
including own properties whose value is undefined. Existing Object.keys,
entries, freeze and matching-record assign support is reused. The pilot's
Object.assign(Object.create(Object.getPrototypeOf(o)), ...) chain still requires
a prototype-preserving method ABI and has a specific located refusal. process
values other than the reviewed env reads also have located host-ABI refusals.

Array.from supports the pilot's string/mapFn use with UTF-16-aware code-point
iteration, including the unannotated JS helper through checked string
projection. Other dynamically typed iterable inputs remain outside that checked
domain. Array callbacks read live length, observe holes as undefined, and
run in iteration order. Array-like objects snapshot ToLength before iteration.
Existing Set and generator drains remain available without mapFn; unsupported
iterator/mapFn and thisArg combinations retain located refusals. Base-16
formatting supports the safe-integer domain needed by charCodeAt, with a checked
trap outside that domain. New target-specific operations explicitly refuse ABAP
with source locations; supported cases export through the fixture helpers.

The untouched merge baseline is 41e01e8. Its required short suite passes.
The new A18 tests fail against that implementation at the targeted logical,
builtin, host and Array.from gaps, then pass with A18. The complete changed
short suite passes with ABAP fixture export enabled. After the two pilot-context
fixes, the repeated final suite passes (tsfront: 336.208 seconds).
Runtime effects include host environment global reads. Gofmt, targeted vet and
diff whitespace checks pass.

All heavy validation is sequential under systemd-run --user --scope -q with
MemoryMax=16G (24G for the full Grace test), nice, GOTOOLCHAIN=go1.26.0,
GOFLAGS=-buildvcs=false, GOCACHE=/mnt/ssd-510/osg/go-build, and
go test -p 2 -timeout 40m. Scratch and retained logs are under
/mnt/ssd-510/osg/abapiti/tmp/gogen-a18.

Pilot inputs: OSG c07c67a3, abaplint 577f875eb, the requested pilotkit script,
and transpiler ee5c2cd1 (rearranger.ts SHA-256
34a03b01ece6faecc5ee1779017637b715d5a8afd40fe21b91b8cc9a5da6d259).
Both compiler measurements use the same copied step-4 driver and inputs.

The first pilot audit exposed two additional contextual gaps after the initial
oracles passed: the unannotated segment helper's any input and a fromEntries
call inside a typed record spread. Reduced Go-vs-Node oracles reproduced both
failures before their fixes. Record spreads now pass their destination's map
type into the nested call, preserving the key/value pair context.

Final pilot comparison on those identical inputs:

```
base: full: 340 blocking (222 root, 118 cascade), 1738 HIR errors in 37 files
A18:  full: 294 blocking (192 root, 102 cascade), 1720 HIR errors in 36 files
```

Base: 139.02 seconds, 1,493,880 KB peak RSS. A18: 150.16 seconds,
1,489,672 KB. Both use the same coverage: 56 files, 51 loaded files, 422 bodies,
226 executed bodies, three unmatched entries. Reference SHA-256 is unchanged:
a13702d55b1eecdb200078205da5909961b4b49ce0f6c3fe3bdbf08e802d6227.
All nine mixed-tagged logical refusals and every targeted builtin-identifier
refusal disappear. The prototype-cloning site retains the specific located
Object.create refusal described above. See gogen-a18-sites.tsv. The overall
self-hosting pilot remains blocked.

All seven final default zabapgit Go sources are byte-identical to the merge
baseline, including hir.go, hir_profile.go, runtime.go and literals.go.

Final mandatory gates pass:

```
go vet -p 2 ./...
go test ./tsfront/ ./hir/... ./cmd/abapiti/ ./internal/abapfixture/ -short -p 2 -timeout 40m
ABAPITI_GRACE_FULL=1 go test ./tsfront/ ./cmd/abapiti/ -run 'TestGraceFullRegistryClosure|TestAbaplintCoreTrappedBodies' -p 2 -count=1 -timeout 40m -v
abapiti abaplint /home/alice/dev/abaplint -o <dir> --target go
abapiti abaplint /home/alice/dev/abaplint -o <dir> --target go --scope zabapgit
```

Grace retains 1,927 classes, 64 interfaces, zero lowering/verification errors,
and unchanged facts: methods pure=3491, may_throw=3767, defined=6257; virtual
sites single=3937, total=5097; static writes memo=0, counter=0, other=171.
Full Grace passes in 238.99 seconds; the trapped-body test passes in 63.04
seconds. Full-core generation retains 2,052 classes and 77 interfaces with zero
lowering/verification errors. The sum of grep -rc 'assignment type mismatch'
over its Go sources is exactly 2.

M1 passes after installing the supplied go-multifile/main.go driver, building
with go build -p 2 -o zabaplint-mf ., and running in the supplied osg-check-kit
with --file files.txt --config abaplint.json. Its output is byte-identical to
expected-clean.txt (SHA-256
95d37bc666403557ceecbb6f0e12e31867c18d3c2870e105e3ceafb5b9267428).
Large copied checkouts, generated modules, executables, fixture exports and
temporary build outputs are removed after validation. Small gate logs, pilot
diagnostics/summaries, hashes and M1 output remain in the task scratch directory.
