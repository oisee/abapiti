# Train-9 namespace rename: missing object_naming issue

Base: `origin/integ/train-9`, `a8038ab`. Fix branch:
`fix/train9-rename-fail`. Original upstream source pin:
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

The failed assertion in `test/lsp/rename_global_class.ts:184` counts
**issues**, not edits or files. The preceding object-count, class-instance,
and `/FOO/CL_BAR` name assertions pass. Node expects exactly one
`object_naming` issue:

```
Object name does not match pattern ^ZC(L|X): /FOO/CL_BAR
```

Train-9 returns zero issues. Reproduction:

```sh
ABAPITI_TESTS_SHARDS=lsp GOFLAGS=-p=2 \
  node tools/abaplint-tests-all.mjs "$UPSTREAM" "$SCRATCH/train9-lsp-before.json"
```

`$UPSTREAM` is a clean checkout of the pinned upstream with its locked npm
packages installed. `$SCRATCH` is a disposable output directory. Commands use
`GOTOOLCHAIN=go1.26.0` and `GOFLAGS=-p=2`.

## Root cause

Default Registry configuration serializes native rule configs into JSON, then
installs the parsed objects through live class views. Property writes and
ordinary typed reads use the original record. The HIR `object.get` operation
for `config[abapType]` instead read the adapter's empty nominal field slots.
`ObjectNaming.run` therefore saw an undefined `clas` pattern and returned an
empty issue array before constructing a RegExp or calling `NameValidator`.

The reusable `tools/rename-naming-probe.mjs` executes original upstream
`ObjectNaming` and `Registry` on a namespaced class without renaming. Its result
is `name|direct-rule-count|direct-rule-key|registry-count|registry-key`:

| Runtime | Result |
| --- | --- |
| Fresh pinned Node | `/FOO/CL_BAR\|1\|object_naming\|1\|object_naming` |
| Train-9 Go | `/FOO/CL_BAR\|1\|object_naming\|0\|missing` |

The native direct rule already produces the right issue in Go. The wrong
answer occurs only after default configuration is parsed and viewed. This
isolates the failure from rename, RegExp, ignore-pattern arrays, and issue
exclusion.

The Go emitter now delegates nominal property lookup to original structural
storage when the receiver has a live backing record. Values, property presence,
and extra keys share the existing structural lookup implementation. Native
objects retain their nominal subclass lookup. Its own-key runtime dependency
is selected before emitting class layouts.

Focused Node-oracle tests cover the naming decision and live reads/writes,
undeclared keys, explicit undefined, inherited native fields, and one evaluation
of receiver and key expressions. The original-source probe independently checks
the actual rule and full default Registry configuration.

```sh
node tools/rename-naming-probe.mjs "$UPSTREAM" "$COMPILER" "$SCRATCH/probe"
go test -short -p 2 ./tsfront \
  -run 'Test(RenameNamingConfig|LiveConfigPropertyLookup)NodeOracle' -v
```

## Fixed shard results

```sh
ABAPITI_TESTS_COMPILER="$COMPILER" \
ABAPITI_TESTS_COMPILER_COMMIT=a1d63b1 \
ABAPITI_TESTS_SHARDS=lsp,objects/rename GOFLAGS=-p=2 \
  node tools/abaplint-tests-all.mjs "$UPSTREAM" "$SCRATCH/final-shards.json"
```

The run reported the pre-amend commit `a1d63b1`; implementation commit
`65320ac` differs only in trailer formatting. Both have source tree
`246b782469a940c28329998767797af6f45c8694`.

Counts follow the harness's ownership accounting, removing six borrowed rule
tests from the LSP shard. The affected namespace rename test is `passed`.

| Shard | Pass | FAIL | Trap | Not-run | Registered | Node active |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| lsp | 180 | 0 | 8 | 5 | 193 | 188 |
| objects/rename | 29 | 0 | 2 | 0 | 31 | 31 |
| Total | 209 | 0 | 10 | 5 | 224 | 219 |

The remaining ten traps are distinct from this wrong answer:

- Three local variable/method rename cases have HIR assignment-type mismatches.
- Semantic highlighting has a classvalue-to-shape narrowing verification error.
- Two object renamer cases, static-modifier hover, and DO help retain checked
  conversion-domain refusals.
- Class help needs JavaScript ToPrimitive for a typed identifier.
- CDS help needs deletion outside the currently supported record boundary.

Five LSP cases are pending/skipped in the original Node suite. This repair fixes
one genuine failing test and the underlying indexed-live-config read generally;
it does not claim the full 11,060-test objective is complete.

## Local gates

All passed:

```sh
export GOTOOLCHAIN=go1.26.0 GOFLAGS='-p=2 -buildvcs=false'
export ABAPITI_TESTS_UPSTREAM="$UPSTREAM"
go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 go test -short -p 2 -timeout 40m \
  ./hir/... ./tsfront/... ./internal/... ./cmd/...
```

The latter includes `TestLexerFactsReport`'s read-only comparison against
`tsfront/testdata/lexer.facts.golden`. The first short-suite attempt lacked
`ABAPITI_TESTS_UPSTREAM`, so an existing Chai oracle could not locate its module;
the complete suite was rerun with the pinned upstream path and passed. No test or
golden was changed to address that environment setup.

## Combination bisect

Each compiler was built from its listed revision with `go build -p 2` and run
against the same pinned LSP shard and Node oracle:

```sh
ABAPITI_TESTS_COMPILER="$COMPILER" \
ABAPITI_TESTS_COMPILER_COMMIT="$REVISION" \
ABAPITI_TESTS_SHARDS=lsp GOFLAGS=-p=2 \
  node tools/abaplint-tests-all.mjs "$UPSTREAM" "$SCRATCH/bisect-$VARIANT.json"
```

| Compiler | Revision | Genuine FAIL | Result / masking trap |
| --- | --- | ---: | --- |
| main | `e2ba6ac` | 0 | 178 pass, 10 trap; CreateFile/DeleteFile guards block apply |
| union-fields | `6bb3191` | 0 | 178 pass, 10 trap; same resource-guard trap |
| main + config-shapes | `63120fd` | 0 | 178 pass, 10 trap; same resource-guard trap |
| main + traps-K | `d6c4f4a` | 0 | 178 pass, 10 trap; unresolved `range` in applyEdit |
| union-fields + traps-K | `45227fe` | **1** | Namespace rename expects 1 issue, gets 0 |
| train-9, including config-shapes | `a8038ab` | **1** | Same assertion and wrong answer |
| fixed train-9 | `65320ac` | **0** | Namespace rename passes; LSP 180 pass, 8 trap |

The isolated union/K merge has parents `6bb3191` and `d6c4f4a` and tree
`1155bb7401b8c9876eb22f12a53bf6ee1176f34d`; its temporary merge commit metadata
is incidental. Recreate it by branching from union-fields and merging traps-K.
The existing wrong read predates these combinations. K opens the first apply
boundary; union-fields fixes the later common `range` read. Together they expose
the wrong answer. Config-shapes is not required to reproduce it.

## Default output and full closure

Build the main, train-9, and fixed compilers, then run each at the same absolute
output directory before moving its completed output into a separate snapshot:

```sh
"$COMPILER" abaplint "$UPSTREAM" -o "$SCRATCH/common" --target all,go
diff -r "$SCRATCH/train9-before" "$SCRATCH/after"
```

The required comparison against `a8038ab` is empty: **0 diff bytes across 6,321
files**, including ABAP targets, Go sources, and the Go executable. Comparison
with `e2ba6ac` finds inherited K runtime additions in `go/runtime.go` and the
corresponding Go binary. The normalized main-to-train-9 and main-to-fixed diffs
are identical; this fix introduces no further default-output changes. The
specific train-9 baseline governs this repair.

Full closure is available in main, so no reachability-manifest workaround was
used. The exact CLI mode and its default assume-integer contract were retained:

```sh
GOTOOLCHAIN=go1.26.0 GOFLAGS='-p=2 -buildvcs=false' \
  "$COMPILER" abaplint "$UPSTREAM" --full -o "$SCRATCH/full-$VARIANT"
```

| Compiler | Blocking | Roots | Cascades | HIR errors |
| --- | ---: | ---: | ---: | ---: |
| main | 29 | 19 | 10 | 77 |
| train-9 before | 26 | 17 | 9 | 67 |
| fixed train-9 | 26 | 17 | 9 | 67 |

The repaired indexed-live-config category has **0 blocking diagnostics before
and after**. It was an accepted path producing a wrong answer. Before/after
blocking diagnostics, including their normalized source locations, are identical.
The existing 26 messages remain as follows; the evidence JSON retains exact
messages, fingerprints, and counts:

| Message | Before | After |
| --- | ---: | ---: |
| Math.ceil requires integral-result evidence | 9 | 9 |
| `packedLength` not lowered (cascade of the above) | 9 | 9 |
| Method `getStr` on classref not lowered | 2 | 2 |
| TypedIdentifier string coercion needs ToPrimitive | 2 | 2 |
| Delete outside supported records | 1 | 1 |
| parseFloat may produce a fraction under assume-int | 1 | 1 |
| String-array string coercion needs ToPrimitive | 1 | 1 |
| IInterfaceDefinition string coercion needs ToPrimitive | 1 | 1 |

These are separate roots and existing conversion boundaries, including categories
explicitly excluded from this task. No WASM, RegExp, Math, small-builtin, object
literal, LSP type, or void-ABI implementation was modified.

Evidence:

- `evidence/train9-rename-shards.json`: Node/Go probe, exact repaired title,
  final shard accounting, and remaining trap groups.
- `evidence/train9-rename-bisect.json`: all five requested compiler combinations
  and the train-9 assertion failure.
- `evidence/train9-rename-full-gap.json`: full-closure counts by exact message
  and normalized source diagnostics before/after.
- `evidence/train9-rename-gates.json`: gate commands, output parity, and source
  tree identities.
