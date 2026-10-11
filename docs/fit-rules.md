# Rules closure batch

Base: origin/main `f834c16` (includes #89, `abaplint --full`). Branch:
`feat/fit-rules`. Scope: TS to HIR, `src/rules/` except `downport.ts`.
The misc-roots batch notes were read before selecting work; its implementations
and the other assigned batches are excluded.

## Commands

Node on PATH: `export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$PATH"`.
Build and generated-Go builds use `GOMAXPROCS=2 GOFLAGS=-p=2`.

```sh
go build -p 2 -o /tmp/fit-rules/main ./cmd/abapiti
/tmp/fit-rules/main abaplint --full -o /tmp/fit-rules/before
python3 tools/rules-gap.py /tmp/fit-rules/before/evidence/registry-blocking.json
go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
/tmp/fit-rules/main abaplint -o /tmp/fit-rules/default-main --target all,go
```

After each root, build the branch binary and repeat `abaplint --full`, the
message/owner count, vet/short tests, and `abaplint --target all,go` to a separate
output directory. The CLI embeds the output path in README files and Go build
metadata, so generate both binaries at `/tmp/fit-rules/default-main`: move the
baseline directory to `default-baseline`, run the branch at `default-main`,
and compare `diff -r /tmp/fit-rules/default-baseline /tmp/fit-rules/default-main`.
The diff must be empty before pushing.
Full measurement uses the default assume-int mode; no reachability modification
or other branch commits are needed. Evidence stays under `/tmp/fit-rules`.
The reporting script attributes cascades to their initiating root, so the void
local errors downstream of unsupported regexp calls count toward that owner,
not as independent void ABI fixes.

## Completed roots

| Root | Rules before → after | Total before → after | HIR errors before → after |
| --- | --- | --- | --- |
| Primitive find: parser_702_chaining and short_case | 113 → 108 (root group 5 → 0) | 635 → 630 | 394 → 390 |
| Reference `Class || undefined`: intf_referencing_clas | 108 → 105 (root group 3 → 0) | 630 → 627 | 390 → 388 |
| Record Object.entries: xml_consistency | 105 → 103 (root group 2 → 0) | 627 → 625 | 388 → 386 |

Primitive find uses the existing optional-result dense callback mechanism with
new source fingerprints for these two consumers. The parser receiver is a
private four-string literal. ShortCase's supported config boundary produces
dense JSON string arrays; the whole consumer class is pinned, with predicates
that only compare strings and never mutate/escape the array. Arbitrary sparse
programmatic configs are outside that boundary. Missing and found-empty-string
results remain distinct; the caller still applies JavaScript truthiness.
Tests compare generated Go with Node for matches, absence, empty receivers,
visit order and early termination, verify ABAP emission, reject stale pins,
and keep unproven primitive find blocked. Required vet and short tests pass;
the path-consistent all,go default `diff -r` is empty. This does not duplicate the
misc-roots adapter for identical_conditions.

## Remaining ownership

The baseline is 635 total blockers: 113 rules blockers (66 roots, 47 cascades).
Remaining root work and excluded owner counts are recorded below as each step
is measured. This is a closure measurement, not an assertion that the 11,060
abaplint tests pass.

| Excluded owner | Rules blockers including cascades at baseline |
| --- | --- |
| String repeat | 16 |
| String match | 28 |
| RegExp, including exec result types/void locals and downstream assignments | 37 |
| Small builtins | 7 |
| Misc-roots: callback returns, contains_tab density, identical_conditions, XMLParser | 7 |
| Math/typeof, LSP types, independent void ABI, object literals | 0 |

These are disjoint attribution counts of existing diagnostics, not predictions
of final net reductions: removing a blocker can expose another root. In
particular, contains_tab's dense callback adapter exposes the currently silent
two-argument string.indexOf overload; the follow-on belongs here.

Reference logical OR now contextually types a literal undefined arm through the
existing optional-reference representation when the checker says the result
is a nominal reference. An assertion can erase absence in the checker without
making the runtime object present. Both class and interface reference probes
compare Go with Node, including absence and exactly one lookup per expression;
ABAP emission is checked. Required vet/short tests pass and the same-path
all,go default diff is empty. This change does not alter nullish coalescing or void
results.

Investigated unchanged boundaries:

* Indexed nominal config (4 diagnostics): `ObjectNamingConf[abapType]` needs
  heterogeneous own/prototype lookup and retained unknown JSON fields. A switch
  over declared config fields with undefined for all other keys would discard
  observable values. General property-presence/prototype metadata is needed;
  this batch leaves those diagnostics intact.
* `XMLValidator.validate` (3 diagnostics): the embedded fast-xml-parser package
  provides declarations, not the validation implementation. The existing XML
  parsing subset cannot reproduce the validator's full input acceptance, error
  locations/messages and structured `true | ValidationError` result. A complete
  validation adapter or lowering the upstream implementation is needed; do not
  substitute successful parsing or a bool for that result.

`Object.entries` lowers string-indexed records to a key/value tuple-array
snapshot with the same value identities. Canonical array-index keys from 0
through 4294967294 sort numerically before the other string keys in insertion
order. Empty strings, leading zeroes, signs, Unicode and out-of-range decimal
strings remain ordinary keys. Node/Go differential tests cover ordering,
receiver evaluation once, snapshots after replacement/insertion and shared
value objects; ABAP emission and refusal probes cover unsupported inputs.
Nominal objects, JavaScript Map, strings and optional-valued records remain
refused: this models existing plain records, not enumerable-property metadata
or a nested optional ABI.

Lowering runTextPool also exposes a lifted-member naming collision: four
lexically separate `push` callbacks in XMLConsistency shared `fn_push`.
Allocate a source-position suffix only on collision, retaining existing unique
names. A generated-Go regression checks two same-named callbacks with distinct
captures. This avoids the newly exposed error and removes two pre-existing HIR
errors in that class; no new verification errors remain. Required vet/short
tests pass and the same-path all,go default diff is empty.
