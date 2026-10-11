# Repeat/match rebase validation

Rebased onto origin/main ea61e23 (includes #97 and #102).

- Syntax probe retains both features; regenerated `syntaxfeatures/oracle.json`
  with `node tools/syntax-features-oracle.mjs` (no hand-merged values).
- Exported lexer facts with `ABAPITI_GRACECHECK=1` and absolute
  `GRACE_FACTS_OUT`. Only catalogue counts changed: aliases/allocates/
  may_raise/writes 96 → 99, reads 141 → 146. All other facts unchanged.
- Added explicit ABAP runtime-op catalogue and a both-backends catalogue guard.
- CI retains all main suites plus repeat/match, feature `--fixture` linting,
  and recorded osgo-gap handling. No transpiler clone or push.
- Grace-enabled `go test ./tsfront/... ./hir/... ./cmd/...` and `go vet ./...`
  passed with the requested Go environment.
- Four quick shards: alternating halves of tsfront tests and alternating
  halves of all remaining non-WASM packages; 775 passed, 15 skipped, 0 failed.
  Current CI itself uses two shards; this local validation used four.

## Default A4H output comparison

Both revisions ran `abaplint -o X --target a4h --quiet`. Raw recursive diff:

- Closure class renamed `z_closure_1063_d771406cea51bd` →
  `z_closure_1063_659efc9f101dad` (closure ID 106339 → 106334).
- `z_runtime_desc_61ac7bd03e15db.clas.abap`: corresponding registration ID/name.
- `z_src_abap_5_s_7035f4eed88c9b.clas.abap`: corresponding declaration/constructor.
- `names.json`: corresponding closure ID/name and source-origin references.
- `a4h/abaplint-577f875e-a4h.zip`: packages the corresponding changes.

Every file, including all decoded ZIP members, was compared after normalizing
only that closure ID/name pair: identical. Generated builds/output removed.
