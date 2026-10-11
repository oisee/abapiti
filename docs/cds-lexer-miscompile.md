# CDS lexer silent miscompile: grouped switch labels

Upstream remains read-only at `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.
Baseline: `origin/feat/tests-all-shards`, `0d420aa28ef6330dbcf27cc572abaadc7652f908`.

## Root cause and regression

`tsfront/lower_stmt2.go:switchStatement` translated each case to a separate
if/else arm. Empty case labels got empty bodies instead of falling through to
the next nonempty clause. `noFallthroughCasesInSwitch` does **not** prohibit
empty labels. In CDS, ordinary spaces and all punctuation except the last `/`
label therefore did not flush the token buffer. This was not a string, UTF-16,
regex, increment or instance-field bug.

A fresh original-Node CDS oracle and generated-Go run reproduced all ten
assertion mismatches. The standalone `TestSwitchGroupedLabelsMatchNode`
regression also fails on the baseline with `defineviewfooasselectfrombarfield`
instead of `define|view|foo|as|select|from|bar|{|field|}`.

The shared frontend now resolves empty labels to the next nonempty body. It
also evaluates the discriminant once, preserves source-order short-circuit
case evaluation, and treats default as fallback after **all** cases, including
when default is in the middle or is itself an empty label. The only snapshot
elision is a literal/local discriminant tested exclusively against literals.
Both Go and ABAP consume this structured HIR; there is no CDS-specific override.

Nonempty fallthrough, nested switch-targeting breaks, and direct case-block
bindings requiring a shared lexical environment/TDZ refuse explicitly. Nested
loops/switches retain their own breaks; continue still targets the surrounding
loop. Tests cover evaluation traces, a mutated discriminant, duplicates,
empty switches, loop control, strict diagnostics, ABAP emission and reached
whole-body Go traps under `TrapOnFailure`. Direct switch-scoped declarations
have no sites in either upstream core source/tests or the harness shims.

## Fresh CDS results

| Run | Passed | Failed | Trapped | Not-run | Registered | Node active + pending |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Baseline | 3 | 10 | 345 | 1 | 359 | 358 + 1 |
| Fixed | 12 | 0 | 346 | 1 | 359 | 358 + 1 |

All nine lexer failures now pass. `CDS Parser basic` now passes its token-count
assertion and reaches the existing unsupported `/^@\w+$/` regexp trap. The
remaining traps are 324 regexp refusals, 15 captured-`parseView` refusals, and
seven captured-`parse` refusals. No initialization failures or unmatched titles.
Genuine CDS assertion mismatches: **zero**, not relabeled failures.

## Audit of the rest of core

A TypeScript AST traversal of every `packages/core/src/**/*.ts` found four
switches with empty labels (53 labels total):

- `src/cds/cds_lexer.ts:152`: 18 labels, whitespace and punctuation.
- `src/ddic.ts:373`: 31 labels, DDIC datatype families (packed, character,
  hex, time, floating and integer types). The default workload uses its
  fingerprinted override; general unpruned lowering may benefit.
- `src/abap/5_syntax/expressions/source.ts:59`: three labels, `(`, `-`, `+`
  sharing `BIT`'s bare break. This particular shared body has no effects, so
  the old empty arm happened to be equivalent.
- `src/lsp/help.ts:239`: one label, `TokenNode` sharing `TokenNodeRegex`'s
  node/token detail rendering. This is outside the default reached workload.

## Default output comparison and full workload

Built the baseline executable from the exact base revision, then used the same
absolute output path for baseline and fixed default `abaplint` invocations.
`diff -r` changes only the following paths; the class paths occur identically
under **`classes/`, `osg/`, and `native/lib/`**:

| Relative path | Reason |
| --- | --- |
| `z_src_cds_cds__9ff42c48675695.clas.abap` | Correct shared switch bodies in CDSLexer. |
| `z_src_abap_5_s_0548eb8d704afe.clas.abap` | CurrentScope's `def.ooType` discriminant is snapshotted once; generated temporary numbering shifts. |
| `z_closure_1063_659efc9f101dad.clas.abap` → `z_closure_1063_fd9340a501a5f1.clas.abap` | Deferred Perform callback identity shifts from `closure.106334` to `closure.106336` after the new HIR snapshot nodes; body unchanged. |
| `z_src_abap_5_s_7035f4eed88c9b.clas.abap` | Perform's reference to that renamed closure. |
| `z_runtime_desc_61ac7bd03e15db.clas.abap` | Descriptor references the new closure identity/name. |
| `names.json` | Records that closure identity/name. |
| `a4h/abaplint-577f875e-a4h.zip` | Contains the changed classes. |

No other files changed. The final TDZ refusal guard does not change default
ABAP output or any generated Go module bytes relative to the first fixed build.

Because output changed, the full zabapgit workload was rerun, not waived:
`tools/hir-go-check.mjs` rebuilt original pinned Node and compared clean (zero
issues) and seeded (five issues) against generated Go and the release kit.
`tools/go-target-check.py` also executed the final generated Go binary. All
outputs match byte-for-byte. This is a TS-HG@Go full-workload check; ABAP
emission is checked, but no ABAP runtime execution is claimed.

- Release kit SHA-256: `2e3311517885ed72e651c97004d6cba064fa5093873c75f1104955891e965d7a`.
- Clean stdout SHA-256: `9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`.
- Seeded stdout SHA-256: `0a08fa85133e89faa08a841a01572b228428191c50844b62e0c8bb9965145e50`.

The optional all-directory command also completed these three additional
shards, with results unchanged from the base (full title-multiset reconciliation):

| Shard | Passed | Failed* | Trapped | Not-run | Registered | Node total |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap` | 44 | 2 | 20 | 154 | 81 | 220 |
| `abap/expressions` | 41 | 2 | 7 | 57 | 50 | 107 |
| `abap/lexer` | 200 | 0 | 0 | 0 | 200 | 200 |

*These four failures are existing conversion/domain refusals, not assertions.
The optional run was stopped after these completed shards because each
unpruned module took several minutes. **All 17 Go shards were not rerun**;
no whole-suite Go total is inferred. The whole-suite original-Node oracle was
freshly rebuilt and executed (11,060 passed, 32 pending). These extra shards
used the grouped-switch fix before the final, unexercised TDZ refusal guard;
the final CDS run and requested gates used the final source.

Gates use `GOCACHE=$HOME/.cache/go-build`, `GOTOOLCHAIN=go1.26.0`,
`GOFLAGS=-buildvcs=false`, `GOMAXPROCS=4`, and `ABAPITI_GRACECHECK=1`:

```sh
go test ./... -short
go test ./tsfront/... ./hir/... ./cmd/...
go vet ./...
go test ./tsfront -run TestSwitch -count=1
node tools/abaplint-tests-report-test.mjs
node tools/abaplint-tests-all-test.mjs
```

All of these gates passed.

Only this compact report and suite summaries are retained. Generated modules,
binaries, source closures, release kit, profiles and raw records are disposable.
No upstream source was edited; no transpiler checkout was cloned.
