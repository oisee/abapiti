A16 lowers string padEnd, at, and matchAll for the self-hosting pilot.

padEnd evaluates its receiver, target length and fill in order, uses UTF-16
units, defaults an absent/undefined fill to a space, repeats multi-unit fills
and truncates the final repetition. Empty fills and already-long receivers
return unchanged. at uses ToIntegerOrInfinity and relative negative indexes;
out-of-range results are undefined, and a surrogate half remains one unit.
Its new relativeAt operation leaves the existing indexed string.at ABI intact.

matchAll extends the existing spread/RegExp implementation and uses the
existing lazy generator consumer ABI. It checks for g at call time and throws
a catchable builtin.TypeError otherwise, clones the initial RegExp cursor,
and returns an iterator that stays exhausted once completed. Breaking a
for-of does not close a native RegExp string iterator. Match arrays retain
UTF-16 index, capture-presence metadata and named groups (including undefined
nonparticipating groups). Empty matches advance by one unit, or a whole
surrogate pair with u. All newly admitted exec/match/replace consumers share
the same offset mapping. The existing certified grammar remains the foundation:
a small extension validates named captures and the Unicode code-point alphabet.
The exact label scanner's trailing (?!=) is certified by its deterministic
name/whitespace/colon prefix and a hidden finite guard; the guard neither
consumes the following unit nor exposes an extra capture to callers.

The legacy translator and pruned runtime remain byte-identical. Unpruned
dynamic RegExp construction opts into the extended grammar. Unicode case
folding, general lookarounds and the previously unmodeled regex syntax remain
refusals. The existing signed-32-bit integral RegExp cursor domain remains;
padding retains the explicit 1,048,576-unit allocation domain used by padStart.
Non-numeric targets/indexes and non-string fill coercions remain located
refusals. The new ABAP operations are explicitly unsupported and have located
refusals, exercised through the fixture-export helper.

Go-vs-Node oracles cover multi-character/empty/default fills, truncation inside
a surrogate pair, negative/fractional/nonfinite indexes, evaluation order,
RegExp aliasing and cursor isolation, Unicode and non-Unicode empty matches,
sticky matching, cursor positions above 1,000 units, iterator resumption,
exhaustion, named-group aliases/keys/JSON and absent captures. Baseline tests
fail at the missing methods; the reduced label-regex oracle separately failed
at Go emission before its runtime extension.

Reproduction uses GOTOOLCHAIN=go1.26.0, GOFLAGS=-buildvcs=false,
GOCACHE=/mnt/ssd-510/osg/go-build, nice, and go test -p 2 -timeout 40m.
Every heavy run is serial and wrapped in
systemd-run --user --scope -q -p MemoryMax=16G nice.
The kit is the requested pilotkit/measure-step4.sh. OSG was copied from
/mnt/ram/abapiti/gogen-pilot4/osg, fetched and checked out at dc975855,
and npm ci completed; the copied .local/step4 input folders were rewritten
to the scratch checkout. abaplint is 577f875e and the transpiler checkout is
/home/alice/dev/transpiler. Baseline compiler: 2f2fcaff (A15).

Pilot measurements use the same inputs for both compilers:

```
base: full: 420 blocking (278 root, 142 cascade), 1752 HIR errors in 38 files
A16:  full: 412 blocking (270 root, 142 cascade), 1752 HIR errors in 38 files
```

Baseline: 150.61 s, 1,472,792 KB peak RSS. Final A16: 138.98 s,
1,448,476 KB peak RSS. Both were under the 16 GiB scope.

The reference SHA-256 remains
61618e3b5618ce52b919a37e0e746765f2c2c2631bd35268e3debcfc7847be17.
Both runs cover 58 files, 53 loaded files, 429 bodies, 217 executed bodies
and three unmatched coverage entries. All eight requested method diagnostics
are removed, with no new source diagnostics; see gogen-a16-sites.tsv.
The overall pilot remains blocked. Optional target 2, mixed tagged logical
operands, is deferred.

Validation commands (with the environment and scope above):

```
gofmt -w <changed Go files>
go vet -p 2 ./...
go test ./tsfront/ ./hir/... ./cmd/abapiti/ ./internal/abapfixture/ -short -p 2 -timeout 40m
ABAPITI_GRACE_FULL=1 go test ./tsfront/ ./cmd/abapiti/ -run 'TestGraceFullRegistryClosure|TestAbaplintCoreTrappedBodies' -p 2 -count=1 -timeout 40m -v
abapiti abaplint /home/alice/dev/abaplint -o <dir> --target go
abapiti abaplint /home/alice/dev/abaplint -o <dir> --target go --scope zabapgit
```

Grace and TestAbaplintCoreTrappedBodies pass. Grace retains 1,927 classes,
64 interfaces, zero lowering/verification errors, and unchanged facts:
methods pure=3491, may_throw=3767, defined=6257; virtual sites single=3937,
total=5097; static writes memo=0, counter=0, other=171.

The rebuilt final compiler reproduces the pilot counts above. All seven
zabapgit Go sources (including runtime.go and literals.go) match the A15
baseline byte-for-byte. Full-core generation retains 2,052 classes and
77 interfaces, with zero lowering/verification errors; the sum from
`grep -rc 'assignment type mismatch' <dir>/go/*.go` is exactly 2.

M1 passes on that final generated core: copy
/mnt/ram/abapiti/selfhost-m1/go-multifile/main.go into <dir>/go/main.go,
build with go build -p 2 -o zabaplint-mf ., then run it in
/mnt/ram/abapiti/selfhost-m1/osg-check-kit using
--file files.txt --config abaplint.json. The result is byte-identical to
expected-clean.txt. Final Go-vs-Node oracles, catalogue tests and vet pass.

The complete requested short suite passes on the final sources (tsfront:
384.474 s); every requested HIR, cmd/abapiti and internal/abapfixture package
passes. Gofmt and git diff --check are clean. Large scratch checkouts,
generated modules, binaries, V8 coverage blobs and reference Go outputs are
removed after validation. Small logs, pilot summaries/diagnostics, manifests,
hashes and M1 output remain under /mnt/ram/abapiti/tmp/gogen-a16.
