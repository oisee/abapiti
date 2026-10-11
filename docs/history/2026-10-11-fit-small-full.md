# Small-area TS→HIR closure measurement

Branch: `feat/fit-small`, based on `origin/main` at
`f834c16ea65c9bd691ca0fe788d0cfbda4332dfc` (includes PR #89).
No projection PRs (#90, #91, #92, #95, #98, #99) were merged, even for
measurement. These are actual main/branch counts, not projection counts.
Upstream abaplint is the embedded `577f875e` closure, with integer assumptions.

## Measurement commands

The baseline binary was built on the clean branch immediately after branching
from `origin/main`; the branch binary includes the frontend fixes.

```sh
mkdir -p /tmp/fit-small
go build -p 2 -o /tmp/fit-small/abapiti-main ./cmd/abapiti
/tmp/fit-small/abapiti-main abaplint --full -o /tmp/fit-small/before
go build -p 2 -o /tmp/fit-small/abapiti-branch ./cmd/abapiti
/tmp/fit-small/abapiti-branch abaplint --full -o /tmp/fit-small/after
```

Both commands lower the entire 1,538-file pinned closure plus harness, without
the reachability manifest. The accompanying
[JSON](2026-10-11-fit-small-full.json) records CLI summaries, complete histograms
by exact blocking/root message, and every assigned-area diagnostic. Source
locations are relative to the closure, with temporary prefixes removed.
The histograms are `Counter(d["message"] for d in registry-blocking.json)`,
with `not d.get("cascade")` selecting roots.

| Measurement | Before | After |
|---|---:|---:|
| All blocking diagnostics | 635 | 631 |
| Root blocking diagnostics | 355 | 352 |
| Cascades | 280 | 279 |
| HIR verification errors | 394 | 391 |
| Files with blockers or HIR errors | 100 | 98 |

| Assigned area | Blockers before → after | Roots before → after | Cascades before → after |
|---|---:|---:|---:|
| `src/*.ts` | 14 → 14 | 10 → 10 | 4 → 4 |
| `abap/types` | 6 → 6 | 3 → 3 | 3 → 3 |
| `ddl` | 2 → 0 | 1 → 0 | 1 → 0 |
| `abap/5_syntax` | 4 → 3 | 3 → 2 | 1 → 1 |
| Assigned areas total | 26 → 23 | 17 → 15 | 9 → 8 |

## Fixed roots

| Area and root message | Root sites before → after | Cascade sites before → after |
|---|---:|---:|
| `ddl`: constructor of the object root is not lowered | 1 → 0 | 1 → 0 |
| `abap/5_syntax`: every needs a source-pinned dense/stable receiver proof | 1 → 0 | 0 → 0 |
| Incidental `lsp`: constructor of the object root is not lowered | 1 → 0 | 0 → 0 |

`DDLParser.hasKey` reads `expr?.constructor?.name`. The frontend previously
rejected constructor inspection solely because the translated receiver's
reference type had been erased to `builtin.object`. The existing HIR
`object.classOf` operation accepts this type and Go obtains its concrete class
descriptor. It now follows the ordinary reference lowering. This also removes
the same diagnostic in `src/lsp/help.ts:243`, without an LSP-specific change.
No new diagnostics appeared anywhere in the closure.

`Catch.findCommonSuperclass` builds `lineages` and each lineage from fresh arrays
using only `push`, before searching. Its nested `find`/`every`/`some` callbacks
only inspect class names; array membership remains stable. The exact `every`
call now carries a density proof tied to the entire method's SHA-256. Unproved
callbacks remain blocking and changes to the method invalidate the proof.

## Remaining assigned-area roots and owners

Counts below are diagnostic sites; ABI/type diagnostics following a failed match
are still classified as roots by the existing CLI, rather than local cascades.

| Area / location | Root message or operation | Roots | Cascades | Owner |
|---|---|---:|---:|---|
| `src/ddic.ts:63` | string `match` | 2 | 1 | string repeat/match |
| `src/ddic.ts:386` | `Math.ceil` integral-result evidence | 1 | 1 | Math / integer evidence |
| `src/registry.ts:370` | async body | 1 | 0 | async/Promise runtime; unassigned |
| `src/skip_logic.ts:78,91` | `RegExpMatchArray` type and string `match` | 4 | 2 | string repeat/match |
| `src/skip_logic.ts:78,91` | void local result ABI | 2 | 0 | void ABI; also depends on match lowering |
| `abap/types/basic/structure_type.ts:48` | string `repeat` | 1 | 2 | string repeat/match |
| `abap/types/message.ts:35,36` | string `match` | 2 | 1 | string repeat/match |
| `abap/5_syntax/_current_scope.ts:501` | unnamed object literal shape | 1 | 0 | object literals |
| `abap/5_syntax/basic_types.ts:691` | variadic `Math.min` | 1 | 1 | Math |

`Registry.parseAsync` genuinely awaits optional progress ticks and returns a
Promise. HIR currently has only an opaque Promise reference, with no fulfillment,
rejection or await scheduling ABI; the progress interface is also overridden as
an opaque object. A synchronous rewrite or trap would not translate this method
faithfully. It remains an explicit blocker requiring async/Promise and progress
callback support. No additional coverage exclusions or traps were introduced.

Outside the assigned areas, remaining blockers are `rules` 312, `objects` 151,
`lsp` 64, and vscode language-server declarations 81. The user assigned these
areas to other owners. Full-closure target emission and the 11,060 upstream mocha
tests are not green yet. The existing ABAP emitter also rejects class identity
on root-object references; this task changes TS→HIR and validates TS-HG@Go only.

## Validation

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$PATH"
GOFLAGS='-p=2' go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
GOFLAGS='-p=2' go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
```

Both gates passed. Focused regressions execute class descriptors through erased
and optional object references on Go, and compare the verbatim CATCH method
against Node for sibling classes, identical classes, a direct superclass, empty
input, missing classes, unrelated roots and cyclic inheritance. The density
proof test also checks refusal without the proof and rejection of changed source.

For the default-output gate, both binaries use the same output path so generated
README instructions and the Go binary's build paths compare exactly:

```sh
export GOMAXPROCS=2
/tmp/fit-small/abapiti-main abaplint -o /tmp/fit-small/default-shared --target all,go
mv /tmp/fit-small/default-shared /tmp/fit-small/default-main-shared
/tmp/fit-small/abapiti-branch abaplint -o /tmp/fit-small/default-shared --target all,go
mv /tmp/fit-small/default-shared /tmp/fit-small/default-branch-shared
diff -r /tmp/fit-small/default-main-shared /tmp/fit-small/default-branch-shared
```

The final `diff -r` exited 0 with empty output, including the compiled Go binary.
An initial comparison using different output paths differed only in README
instructions and the binary's embedded build paths; no generated source differed.
