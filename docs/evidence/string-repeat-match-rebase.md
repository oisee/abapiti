# Repeat/match rebase validation

Rebased again onto origin/main `6d081fd8585f93a1b11e3f47efb55dc9573bca9f`
(includes #98 generators). The sole conflict, `tsfront/lower_types.go`, retains
both generator and match-array type handling.

- Probes retain both sides; reran the syntax-features, registry-features, and
  string-repeat-match Node oracle tools. All generated values were unchanged.
- Exported lexer facts with `ABAPITI_GRACECHECK=1` and absolute
  `GRACE_FACTS_OUT`: identical to the existing golden. Against main, only
  catalogue counts differ: aliases/allocates/may_raise/writes 96 → 99,
  reads 141 → 146. No golden regeneration needed.
- Added explicit ABAP runtime-op catalogue and a both-backends catalogue guard.
- CI retains all main suites plus repeat/match, feature `--fixture` linting,
  and recorded osgo-gap handling. No transpiler clone or push.
- Grace-enabled `go test ./tsfront/... ./hir/... ./cmd/...` and `go vet ./...`
  passed with the requested Go environment.
- Four quick shards: alternating halves of tsfront tests and alternating
  halves of all remaining non-WASM packages (`-short -count=1`, grace enabled);
  784 passed (≥661), 14 skipped, 0 failed.
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
