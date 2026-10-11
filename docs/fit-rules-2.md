# Rules closure follow-up

Branch: `feat/fit-rules-2`. Prerequisite base: `origin/feat/fit-rules`
`785f1f47945218682be75faf36ff3dc0a0471cef`, which was not merged into
`origin/main` when this task started. Default-output baseline:
`origin/main` `53c2866745f40d909d4d9ff29a3971fafad67464`.
Scope: TS → HIR, Go first; no WASM changes.

## Results

| Measurement | Clean prerequisite | Follow-up |
| --- | ---: | ---: |
| Indexed config diagnostics | 4 | 0 |
| XML validation diagnostics | 3 | 0 |
| Rules except downport | 102 | 92 |
| Rules root / cascade diagnostics | 60 / 42 | 56 / 36 |
| Full closure blocking diagnostics | 624 | 614 |
| Full closure root / cascade diagnostics | 349 / 275 | 345 / 269 |
| HIR verification errors | 386 | 381 |

The two initiating owned roots are `ObjectNaming.run`'s indexed lookup and
`XMLConsistency.run`'s external validator call. Besides the seven assigned
diagnostics, config lookup unblocks three existing RegExp diagnostics in
ObjectNaming without changing RegExp lowering. Diagnostic root flags count
four removed roots because the previously failed RegExp constructor also
reported two independent diagnostics.

The full-closure message-count changes are:

| Message | Before | After |
| --- | ---: | ---: |
| element access on classref is not lowered | 1 | 0 |
| identifier pattern is not a lowered local or module value | 3 | 0 |
| identifier XMLValidator is not a lowered local or module value | 1 | 0 |
| identifier res is not a lowered local or module value | 2 | 0 |
| identifier RegExp is not a lowered local or module value | 1 | 0 |
| new RegExp is not a lowered class | 1 | 0 |
| identifier regex is not a lowered local or module value | 1 | 0 |

No message count increased. The remaining 92 rules diagnostics belong to the
excluded owners: RegExp 34, string match 28, string repeat 16, small builtins 7,
and misc-roots 7. The other 522 blockers are outside this rules batch. This is
a closure reduction, not a claim that the 11,060 mocha tests pass.

## Indexed config model

String-keyed reads on nominal rule config classes use `object.get`, returning a
heterogeneous tagged value or absence. Each used receiver class gets a static
property table. Base-constructor fields are own properties of the instance;
class methods and Object.prototype properties are prototype properties.
Presence is independent of value: an own field holding undefined still exists.
Reads access the current field, and subclasses select their actual class table.
Unknown keys on the closed class instance return undefined. Tables/runtime
helpers are emitted only when used, preserving default output.

`object.hasOwn` lowers the inherited library `hasOwnProperty` call on config
instances. `object.has` exposes prototype-aware presence in HIR; tests compare
it with Node's `in`, without extending the separately owned TS `in` lowering.
Prototype function properties retain opaque stable identities; dynamic
invocation remains outside the callable ABI. General dynamic property writes,
prototype mutation and arbitrary JSON-to-nominal config conversion are outside
this static class boundary. Existing dynamic JSON graph reads keep their own
representation; this operation does not discard that graph or project it into
a declared-field-only JSON object.

Node/compiled-Go tests cover inherited fields, missing and undefined values,
false/zero/empty strings, current values after mutation, subclasses, own versus
prototype presence and receiver/key evaluation order. Refusal probes cover
setPrototypeOf, class prototype writes and __proto__ assignment.

## XML validation model

A source-fingerprinted adapter for `XMLConsistency.run` lowers the default
`fast-xml-parser@5.10.1` validator to `xml.validate`. The runtime ports the pinned
validator and name grammar, retaining its acceptance quirks, duplicate-attribute
checks, entities, comments, CDATA, declarations and UTF-16 positions. The result
is tagged true or `{err: {code, msg, line, col}}`; the undefined column of the
start-tag error remains absent. The adapter uses tagged strict equality for
`res !== true` and preserves `res.err.msg`, leaving the surrounding rule live.
The existing XML parser subset is not used as a validation substitute.

An offline copy of the pinned Node validator provides the oracle. The Go
adapter matches all 444 result and issue-message observations, including 50
literal inputs from the pinned abaplint xml_consistency tests and deterministic
malformed-input mutations. Source-pin tests reject changed consumer source.
Nondefault validator options are not admitted by this adapter. The new runtime
operations currently have Go emission; full ABAP emission of these operations
remains unsupported. The requested default ABAP/Go output is unchanged.

## Reproduction and gate

All commands used Node on PATH and limited Go concurrency:

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$PATH"
export GOMAXPROCS=2 GOFLAGS=-p=2
mkdir -p /tmp/fit-rules-2/main-src /tmp/fit-rules-2/base-src
git archive 53c2866745f40d909d4d9ff29a3971fafad67464 | tar -x -C /tmp/fit-rules-2/main-src
git archive 785f1f47945218682be75faf36ff3dc0a0471cef | tar -x -C /tmp/fit-rules-2/base-src
(cd /tmp/fit-rules-2/main-src && go build -o /tmp/fit-rules-2/main ./cmd/abapiti)
(cd /tmp/fit-rules-2/base-src && go build -o /tmp/fit-rules-2/base-clean ./cmd/abapiti)
/tmp/fit-rules-2/base-clean abaplint --full -o /tmp/fit-rules-2/before-clean
python3 tools/rules-gap.py /tmp/fit-rules-2/before-clean/evidence/registry-blocking.json

go build -o /tmp/fit-rules-2/branch-final ./cmd/abapiti
/tmp/fit-rules-2/branch-final abaplint --full -o /tmp/fit-rules-2/after-final
python3 tools/rules-gap.py /tmp/fit-rules-2/after-final/evidence/registry-blocking.json

go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...

/tmp/fit-rules-2/main abaplint -o /tmp/fit-rules-2/default --target all,go
mv /tmp/fit-rules-2/default /tmp/fit-rules-2/default-main
/tmp/fit-rules-2/branch-final abaplint -o /tmp/fit-rules-2/default --target all,go
diff -r /tmp/fit-rules-2/default-main /tmp/fit-rules-2/default
```

Both default generations use the same output path because README/build metadata
contains that path. The recursive diff is empty (exit 0, zero bytes). Vet and
the entire requested short-test package set pass. The reporting script records
full-closure counts by message as well as the rules ownership counts.

Evidence under `/tmp/fit-rules-2`: `before-clean`, `after`, `after-final`, `before-counts.json`,
`after-counts.json`, `after-final-counts.json`, `owned-focused-final.log`, `vet-final.log`, `short.log`,
`default-main.log`, `default-branch.log`, and `default.diff`. An earlier working
build was discarded as a baseline; all reported before counts use the isolated
clean prerequisite archive.
