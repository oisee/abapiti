# Object-area TS → HIR gap

Branch: `feat/fit-objects`, based only on `origin/main` at
`f834c16ea65c9bd691ca0fe788d0cfbda4332dfc` (#89 includes `abaplint --full`).
PRs #90, #91, #92 and #95 were merged without conflicts in an isolated,
detached measurement worktree. Their commits are not part of this branch.
The exact PR heads, full-closure message histograms, owner counts and every
object-area diagnostic are in [fit-objects.json](../evidence/fit-objects.json).
Locations in that evidence are relative to the pinned source tree.

| Measurement | Total blockers | Object blockers | Object roots | Object cascades | HIR errors (whole closure) |
| --- | ---: | ---: | ---: | ---: | ---: |
| Main before | 635 | 151 | 96 | 55 | 394 |
| Main + object fixes | 558 | 78 | 62 | 16 | 330 |
| Four-PR projection before | 455 | 112 | 57 | 55 | 358 |
| Four-PR projection + object fixes | 377 | 38 | 22 | 16 | 294 |

The 11,060 upstream mocha tests were not run against a compiled full closure:
its remaining blockers and HIR errors still prevent that build.

## Fixed roots

Counts below include the same-root local cascades. These groups are disjoint
and identical on main and the four-PR projection.

| Object root | Before root diagnostics | Before with cascades | After |
| --- | ---: | ---: | ---: |
| Duration-only `Date.now()` calls | 32 | 64 | 0 |
| AFF `JSON.parse` calls | 2 | 10 | 0 |
| Direct `XMLParser` text call | 2 | 3 | 0 |
| `Object.assign` on text-element records | 1 | 1 | 0 |
| Early-return sort comparator | 1 | 1 | 0 |
| Immediately invoked arrow | 1 | 2 | 0 |
| Total | 39 | 81 | 0 |

Clock and parser adaptations use exact enclosing method fingerprints. Clock
values are used only for elapsed parse durations. AFF JSON retains its graph
through the existing strict JSON runtime; the direct text XML call has the same
reviewed options as `AbstractObject.parseRaw2`. Source catches and extraction
code remain in place. These retain the existing JSON/XML runtime subset limits;
clearing the frontend diagnostics does not establish unrestricted parser parity.

`Object.assign` evaluates all arguments before copying, preserves the target's
identity, snapshots source keys, and skips undefined sources. An undefined target
raises after argument evaluation. Only matching string-indexed records are
accepted; JS Map entries are not own properties. Inferred intersections of pure
index-signature record views lower to the same representation.

Callbacks with early returns and immediate arrows use existing lifted methods,
so a return exits their invocation. Optional dynamic values can now be coerced
into optional primitive values while preserving absence and checking the payload.
This removes the AFF paths' newly exposed HIR argument mismatches. No backend,
WASM, object-literal lowering or void-result ABI was changed.

The gross object reduction is 81 diagnostics. Main exposes eight additional
excluded diagnostics: five string-match/type roots and cascades, one void-local
consequence inside the newly lowered arrow, and two empty-array literals inside
AFF object fields. The projection has the void fix already, so exposes seven.
Net object reductions are therefore 73 on main and 74 on the projection.
Generic lowering also clears four blockers outside objects on both bases.

## Remaining owner handoff

These are **diagnostic counts**, including related local cascades. Each diagnostic
is assigned once; regex-result void locals are shown separately under the void
owner on main. Counts do not promise that one owner patch alone makes an entire
function buildable.

| Owner / remaining root | Main before | Main after | Projection before | Projection after |
| --- | ---: | ---: | ---: | ---: |
| String `match` and match-array types/cascades | 13 | 18 | 13 | 18 |
| RegExp `exec` and pattern replacement/cascades | 8 | 8 | 10 | 10 |
| Small builtins: `toLocaleLowerCase` (2), `in` (1) | 3 | 3 | 3 | 3 |
| vscode/LSP creation, replacement and `.is` | 22 | 22 | 2 | 2 |
| `typeof` and dependent parameter locals | 8 | 8 | 0 | 0 |
| Object literals / empty arrays | 9 | 11 | 0 | 2 |
| Void-result ABI | 4 | 5 | 0 | 0 |
| Generator and dependent `icfServices` local | 2 | 2 | 2 | 2 |
| Fractional numeric conversion | 1 | 1 | 1 | 1 |
| Total remaining groups | 70 | 78 | 31 | 38 |

No object-area blockers remain for Math, string repeat, trimEnd, flat, Map.delete,
labels or `++` in these inventories. The requested owner exclusions were left
untouched, including `??` (which did not appear as a blocking diagnostic here).
The two remaining empty arrays are at `src/objects/domain.ts:114:56` and `:122:62`
inside AFF object-literal fields. The remaining LSP projection roots are
`TextDocumentEdit.is` and `RenameFile.is`; Generator is in `RenamerHelper`.

One actionable numeric limit remains at `src/objects/number_range.ts:51:33`:
`parseFloat(attributes?.PERCENTAGE || "")`. This can yield a fraction or NaN,
and `getPercentage()` exposes it. There is no evidence that this result is an
integer, and no existing generic `parseFloat` runtime lowering. Inventing an
integer exception would neither implement parseFloat nor preserve arbitrary
percentages. Correct support needs the numeric runtime/storage ABI, beyond this
TS → HIR-only patch. It remains explicitly blocking; it was not trapped or erased.

## Commands and validation

All builds and package gates used at most `-p 2`; validation and final measurement
runs also used `GOMAXPROCS=2`. Node was placed on PATH:

```sh
export PATH=~/.local/opt/node-v26.9.0-linux-x64/bin:$PATH
export GOMAXPROCS=2

go build -p 2 -o /tmp/fit-objects-main ./cmd/abapiti  # before edits, at origin/main
/tmp/fit-objects-main abaplint --full -o /tmp/fit-objects-before

git worktree add --detach /tmp/fit-objects-projection origin/main
git -C /tmp/fit-objects-projection merge --no-edit origin/pr90 origin/pr91 origin/pr92 origin/pr95
# In the projection worktree, before applying this branch's patch:
go build -p 2 -o /tmp/fit-objects-projected ./cmd/abapiti
/tmp/fit-objects-projected abaplint --full -o /tmp/fit-objects-projected-before

# In this branch:
go build -p 2 -o /tmp/fit-objects-branch ./cmd/abapiti
/tmp/fit-objects-branch abaplint --full -o /tmp/fit-objects-final
# Apply only this branch's diff to the projection, then build there:
go build -p 2 -o /tmp/fit-objects-projected ./cmd/abapiti
/tmp/fit-objects-projected abaplint --full -o /tmp/fit-objects-projected-final

go vet -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
```

Vet and the short tests passed. Executable TS → HIR → Go regressions cover
record identity, inferred record intersections, overwrites, aliasing, self-copy,
undefined sources, evaluation before copying and target rejection, Map rejection,
early-return sorting/filtering with captures, immediate arrows, and optional
dynamic primitive coercion.

Both default builds were generated at the **same** clean output path. Different
output paths embed different README commands and Go build paths, so comparing
them produces unrelated path differences. The final check includes all files,
archives and the generated Go binary:

```sh
/tmp/fit-objects-main abaplint -o /tmp/fit-objects-default --target all,go
cp -a /tmp/fit-objects-default /tmp/fit-objects-default-reference
rm -rf /tmp/fit-objects-default  # only this task's generated temporary output
/tmp/fit-objects-branch abaplint -o /tmp/fit-objects-default --target all,go
diff -r /tmp/fit-objects-default-reference /tmp/fit-objects-default
```

The recursive diff was empty. No PR was created and main was not pushed.
