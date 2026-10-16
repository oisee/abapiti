# Step-4 replacement and type lowering

Baseline: origin/integ/selfhost-s7 at
683d0e3036a2358946c4b84873ab89bf1d616b7e; branch dell/types.

## Replacement

RED Node oracle: the original lowerer rejects literal dollar substitutions;
the first backend run also refuses the pilot's /^(-?)0+(?=\d)/ grammar.
The final oracle compares Node and generated Go, and exports ABAP fixtures.
Coverage includes original-input prefix/suffix, first/all, empty needles and
matches, UTF-16 units, capture indices 1–99 and two-digit fallback, absent
captures, named captures, absent names, and literal dollars/backslashes.
The exact zero-prefix grammar consumes a final digit internally, excludes
that digit from the exposed match, and hides its implementation capture.
Named groups admit unique ASCII names; other regex forms retain checked
emitter/runtime refusals. Callback/coercing replacements and replaceAll with
a nonliteral or nonglobal RegExp receive located frontend diagnostics.

Specification: https://tc39.es/ecma262/multipage/text-processing.html#sec-getsubstitution

Baseline pilot (local writable mirror of the supplied OSG checkout):
reference a13702d55b1eecdb; files 56, loaded 51, bodies 422, executed 226,
unmatched 3; 304 blocking (206 root, 98 cascade), 1691 HIR errors in 37 files;
118.40 seconds, 1397244 KB. /logs/pilotsetup is used read-only.

## Generic nested append (two type-reference root sites)

A: append.ts:2:35 and :2:70. Both are correctly typed uses of T in native
array storage, not missing generator annotations. The RED oracle reproduces
both declaration failures. The supported complete two-loop body specializes
at direct calls, independent of identifier names, retaining native element
storage and evaluating all arguments before mutation. Primitive, object and
nested-array aliases and late argument mutation match Node. Other generic
bodies and indirect function values retain located refusals. Sparse arrays retain
undefined when the element ABI permits it; required primitive slots use a located
runtime refusal instead of fabricating a zero value.

## Bigint (three checker-type root sites)

A: emit-int8.ts:11:7, :11:27, :13:30. TypeScript bigint is correctly typed;
changing it to number or int64 would lose precision or overflow at the bounds.
The RED oracle reproduces the checker/annotation failures and BigInt conversion
refusal. HIR now has a distinct bigint primitive, canonical arbitrary-length
signed decimal storage, decimal-string construction, constant +/−/multiply/left
shift, runtime negation, comparisons, formatting and truthiness. Tagged storage
preserves bigint versus Number/String/null. Runtime arithmetic beyond negation,
nondecimal conversion strings and numeric/boolean conversion arguments stay
outside the checked subset. Shift folding caps allocation at 1,048,576 bits.
Go matches Node across both int64 boundaries and 40-digit integers; ABAP emits
string-based sign/length/digit comparisons without fixed-width conversion.

## Optional callable intersections (OwnershipObject)

A: frontend-owned.ts:14:27 and :15:14. ArtifactObject intersected with
Partial<Pick<ABAPObject, 'getABAPFiles'>> is correctly typed TypeScript.
Structural intersection synthesis now maps callable slots by their checker
signatures and uses the existing optional-data-member fallback. The RED oracle
reproduced the alias refusal; absent and present callable slots now match Node
and export ABAP. Behavior-bearing native class conversion remains subject to
the existing checked class-view ABI, rather than copying method values.

## Generic property values and string keys

A: ir-children.ts:10:110 and :53:81 (PropertyValue<T>), plus the two
keyof-T-and-string diagnostics at :55:28. T is constrained to object. The
supported conditional is structurally T extends object ? T[keyof T] : never;
it maps to tagged values, and keyof T & string maps to string keys. Arbitrary
conditional types retain their refusals. Generic indexed reads use original
object storage and Object.keys snapshots. Heterogeneous tuples keep their
existing positional layout; their JSON writers emit arrays, preserving undefined
as null in array slots, and their array identity is retained.
The RED oracle reproduced both conditional failures and key intersections;
children and entries now match Node, including JSON array shape, and emit ABAP.

## ProgramInput (three type-reference root sites)

A: emit-exception.ts:58:44, emit-table-move.ts:12:44,
frontend-owned.ts:42:43. Program | (Class[] & Partial<Program>) is a valid
legacy-input contract, not a generator type error. Optional-data intersections
retain native array identity; record-or-array unions use tagged storage.
Original array references survive wrapping and fallback selection. Missing
named metadata reads return undefined. Named writes on native array
intersections remain a located frontend refusal; metadata on record inputs is
supported. The RED oracle reproduced ProgramInput/ClassList refusal, and the
final record/legacy-array oracle matches Node and exports ABAP.

## Classification and final pilot

The supplied TSV contains **16 selected occurrences**: two replacement sites,
six type-reference sites, and eight checker-type occurrences (including the
same keyof location twice). This differs from the task's headline checker
count. Every selected construct is valid TypeScript and classified **A**;
there are **no G sites** in this set. The original OSG `tsconfig.gogen.json`
also passes its installed TypeScript 5.9.3 checker (exit 0). Exact locations and
reasons are in `dell-types-sites.tsv`; before/after occurrence counts are in
`dell-types-sites.json`.

| Construct | Before | After |
| --- | ---: | ---: |
| Replacement substitutions | 2 | 0 |
| Generic append T[] declarations | 2 | 0 |
| ProgramInput | 3 | 0 |
| OwnershipObject reference/checker type | 2 | 0 |
| PropertyValue<T> | 2 | 0 |
| bigint checker types | 3 | 0 |
| keyof T & string (one location, two reports) | 2 | 0 |
| **Total** | **16** | **0** |

No final diagnostic remains at any of those original locations.

| Step-4 pilot | S7 baseline | Final |
| --- | ---: | ---: |
| Blocking diagnostics | 304 | 362 |
| Root / cascade | 206 / 98 | 249 / 113 |
| HIR errors | 1691 | 1717 |
| Files with blocking diagnostics | 37 | 36 |
| Lower wall seconds | 118.40 | 161.73 |
| Peak RSS, KB | 1397244 | 1725960 |

The whole pilot remains blocked: lowering the selected signatures exposes
additional downstream blockers. This is not a claim that step 4 is executable.
Both runs have identical Node reference bytes, SHA-256
`a13702d55b1eecdb200078205da5909961b4b49ce0f6c3fe3bdbf08e802d6227`,
and identical coverage: 56 files, 51 loaded, 422 bodies, 226 executed,
3 unmatched. The supplied measurement script writes a temporary tsconfig into
OSG, so it ran unchanged against a writable mirror of the pinned checkout.
All supplied `/logs/pilotsetup` material was read-only. Full local evidence is
under `/work/.local/m-base` and `/work/.local/m-final`.

## Final gates

All heavy commands ran with `ulimit -v 16777216`; Go package concurrency was
at most `-p 2`. The final short suite used `-timeout 60m`.

| Gate | Result | Wall seconds |
| --- | --- | ---: |
| `go build -p 2 ./...` | PASS | 21 |
| `go vet -p 2 ./...` | PASS | 2 |
| Required `go test -short -p 2` package set | PASS | 404 |
| `extern/hostio` tests | PASS | cached |
| Assignment-mismatch matching-line sum | PASS: baseline 2, final 2 | |
| v0.5.2 kit, clean + seeded | PASS: raw bytes equal across revisions and expected files | |
| M1 TS-HG, OSG's own ABAP | PASS: 80 issues, cmp-equal | 31.26 |

The short suite includes hir, tsfront, cmd/abapiti and internal/abapfixture;
tsfront completed in 385.817 s. The six new construct oracles pass against Node.
All 96 ABAP files exported from their final fixtures parse with zero unknown
statements. Initial broad-suite failures were fixed before this final run;
constructor-signature and tuple ABIs are retained, obsolete dollar-rejection
probes are now executable Node oracles, and bigint effect records are complete.

The pinned abaplint checkout is
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`; `npm ci` ran at its root and in
packages/core. Baseline generation/build took 98.498 / 59.8 s; final generation/
build took 96.798 / 63.4 s. Both have 2052 classes, 77 interfaces, zero blocking
frontend diagnostics and zero HIR verification errors. The v0.5.2 release kit
ZIP has SHA-256
`2be51f82cfc1947416faef522c21b52c485e94f43bc6ea65d40fb8e9fb421de8`.

| Raw kit output | Baseline seconds | Final seconds | Expected issues |
| --- | ---: | ---: | ---: |
| Clean | 14.031 | 22.837 | 0 |
| Seeded | 14.301 | 19.840 | 5 |

Comparison uses raw stdout, without the kit script's trailing-whitespace
normalization. Clean output is 2 bytes; seeded output is 601 bytes.
Matching-line and occurrence counts for `assignment type mismatch` are both
exactly 2 for both generated Go directories.

The runnable M1 command is documented in `docs/evidence/gogen-a15.md:86`:
copy the multifile host driver into the generated Go module, build it, run
`--file files.txt --config abaplint.json`, then `cmp expected-clean.txt`.
The available frozen driver/workload were copied locally from `/logs/m1wl`.
All 2785 workload file hashes match the manifest. Final output has 81 lines,
80 issues, and SHA-256
`95d37bc666403557ceecbb6f0e12e31867c18d3c2870e105e3ceafb5b9267428`,
exactly the documented Node reference. M1 used TS-HG; this does not claim a
new TS-HA@OSGO runtime certification.

The compiler binaries used for the final pilot and final abaplint generation
are byte-identical (SHA-256
`629170903e1813e3828161a698c39c1bcc872bd22784b63650f6672c9941c038`).
Author and committer periods, requested trailers, and a clean diff were checked.
No push was made. Machine-readable results are in `dell-types-validation.json`.


An additional detached checkout of the recorded S7 SHA is source-clean and
reproduces all eight initial baseline Go source files byte-for-byte. Its clean
and seeded raw outputs also equal the final outputs and expected files
(13.241 / 13.113 s). Its generation/build took 66.000 / 46.4 s.
