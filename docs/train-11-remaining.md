# Train 11 remaining Node-active tests

Measured compiler: `8c40fe1`; original upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`. All 17 shards and 625 files were measured against fresh original Node, with 11,060 active cases.

**28 remaining = 25 traps + 3 unregistered + 0 skipped active cases. FAIL 0; async-pending 0.** Node-pending cases are excluded from this tail.

Groups below use the exact diagnostic message after removing disposable closure prefixes. Source coordinates, node IDs and message wording are retained; messages are not normalized into broader groups. Each entry lists the full original test title, shard, and TypeScript registration source sites from original Node source maps. Repeated titles are counted as distinct registrations.

## 1. not-run: 3 tests

```text
unregistered (no Go test registration)
```

- **abap/structures** — structure combi, complex1 complex1 match
  Source sites: `test/abap/structures/_combi.ts:284:3`, `test/abap/structures/_combi.ts:276:1`.
- **rules** — test line_length rule "hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello" should have 1 issue(s)
  Source sites: `test/rules/_utils.ts:60:9`, `test/rules/_utils.ts:39:11`, `test/rules/_utils.ts:36:3`, `test/rules/line_length.ts:17:9`.
- **rules** — test line_length rule "WRITE: / 'abc'." should have 0 issue(s)
  Source sites: `test/rules/_utils.ts:60:9`, `test/rules/_utils.ts:39:11`, `test/rules/_utils.ts:36:3`, `test/rules/line_length.ts:17:9`.

## 2. trapped: 2 tests

```text
src/lsp/dump_scope.ts:12:3: lowering trap: unsupported-expr src/lsp/dump_scope.ts:31:14: string coercion of classref<src/abap/types/_typed_identifier.ts.TypedIdentifier> requires JavaScript ToPrimitive; unsupported-expr src/lsp/dump_scope.ts:32:14: string coercion of classref<src/abap/types/_typed_identifier.ts.TypedIdentifier> requires JavaScript ToPrimitive
```

- **lsp** — LSP, help first character = DO
  Source sites: `test/lsp/help.ts:9:3`, `test/lsp/help.ts:7:1`.
- **lsp** — LSP, help class
  Source sites: `test/lsp/help.ts:25:3`, `test/lsp/help.ts:7:1`.

## 3. trapped: 2 tests

```text
src/lsp/rename.ts:93:3: lowering trap: src/lsp/rename.ts:93:3: HIR verification: node 168536 (src/lsp/rename.ts:98:52): assignment type mismatch
```

- **lsp** — LSP, rename variable simple
  Source sites: `test/lsp/rename_variable.ts:36:3`, `test/lsp/rename_variable.ts:8:1`.
- **lsp** — LSP, rename variable method parameter
  Source sites: `test/lsp/rename_variable.ts:56:3`, `test/lsp/rename_variable.ts:8:1`.

## 4. trapped: 2 tests

```text
src/objects/_dynpros.ts:30:1: value outside supported numeric, ordering or conversion domain
```

- **rules** — Rule: dynpro_checks Element SDFSDF length longer than 132
  Source sites: `test/rules/dynpro_checks.ts:21:3`, `test/rules/dynpro_checks.ts:19:1`.
- **rules** — Rule: dynpro_checks overlapping dynpro fields
  Source sites: `test/rules/dynpro_checks.ts:197:3`, `test/rules/dynpro_checks.ts:19:1`.

## 5. trapped: 2 tests

```text
src/objects/message_class.ts:65:3: value outside supported numeric, ordering or conversion domain
```

- **objects** — malformed object XML ignores structured values where text is expected
  Source sites: `test/objects/malformed_xml.ts:120:3`, `test/objects/malformed_xml.ts:19:1`.
- **rules** — Message rule no number
  Source sites: `test/rules/msag_consistency.ts:41:3`, `test/rules/msag_consistency.ts:14:1`.

## 6. trapped: 2 tests

```text
src/rules/space_before_dot.ts:50:3: value outside supported numeric, ordering or conversion domain
```

- **root** — exclude list will exclude issues based on the local exclude patterns
  Source sites: `test/registry.ts:466:3`, `test/registry.ts:368:1`.
- **root** — severity an issue will have the severity specified in the config
  Source sites: `test/severity.ts:30:3`, `test/severity.ts:6:1`.

## 7. trapped: 1 test

```text
src/lsp/help.ts:39:3: lowering trap: unsupported-expr src/lsp/help.ts:46:5: delete supports ordinary lowered records only
```

- **lsp** — LSP, help CDS
  Source sites: `test/lsp/help.ts:39:3`, `test/lsp/help.ts:7:1`.

## 8. trapped: 1 test

```text
src/lsp/rename.ts:104:3: lowering trap: src/lsp/rename.ts:104:3: HIR verification: node 168637 (src/lsp/rename.ts:109:52): assignment type mismatch
```

- **lsp** — LSP, rename method rename method
  Source sites: `test/lsp/rename_method.ts:10:3`, `test/lsp/rename_method.ts:8:1`.

## 9. trapped: 1 test

```text
src/lsp/semantic.ts:42:3: lowering trap: src/lsp/semantic.ts:42:3: HIR verification: node 169022 (src/lsp/semantic.ts:50:23): invalid narrowing classvalue to classref<shape.c3b70776d029edfc69fd75ab61b91f27fe8deaade7fdd314df00aa7ee5f66866>
```

- **lsp** — LSP, semantic highlighting basic
  Source sites: `test/lsp/semantic.ts:10:3`, `test/lsp/semantic.ts:8:1`.

## 10. trapped: 1 test

```text
src/objects/domain.ts:100:3: input is outside the supported strict JSON subset
```

- **objects** — Domain, parse AFF json parser error
  Source sites: `test/objects/domain.ts:92:3`, `test/objects/domain.ts:7:1`.

## 11. trapped: 1 test

```text
src/objects/proxy_object.ts:86:3: value outside supported numeric, ordering or conversion domain
```

- **objects** — malformed object XML ignores malformed inner object metadata
  Source sites: `test/objects/malformed_xml.ts:71:3`, `test/objects/malformed_xml.ts:19:1`.

## 12. trapped: 1 test

```text
src/rules/line_only_punc.ts:46:3: value outside supported numeric, ordering or conversion domain
```

- **rules** — test line_only_punc rule "." should have 1 issue(s)
  Source sites: `test/rules/_utils.ts:60:9`, `test/rules/_utils.ts:39:11`, `test/rules/_utils.ts:36:3`, `test/rules/line_only_punc.ts:23:9`.

## 13. trapped: 1 test

```text
test/abap/types/types.ts:10:1: value outside supported numeric, ordering or conversion domain
```

- **abap/types** — Types, TYPES TYPES structure should sometimes inherit qualified name
  Source sites: `test/abap/types/types.ts:18:3`, `test/abap/types/types.ts:17:1`.

## 14. trapped: 1 test

```text
test/cds/cds_parser.ts:3725:73: lowering trap: unsupported-expr test/cds/cds_parser.ts:3745:57: union field constructor requires a public data slot in every member; unsupported-expr test/cds/cds_parser.ts:3746:12: identifier rootAnnos is not a lowered local or module value
```

- **cds** — CDS Parser define table entity — field annotations sit under CDSTableField
  Source sites: `test/cds/cds_parser.ts:3725:3`, `test/cds/cds_parser.ts:7:1`.

## 15. trapped: 1 test

```text
test/cds/cds_parser.ts:3803:3: lowering trap: unsupported-call test/cds/cds_parser.ts:3804:15: method get on dynamic is not lowered; unsupported-expr test/cds/cds_parser.ts:3805:9: identifier k is not a lowered local or module value; unsupported-call test/cds/cds_parser.ts:3806:21: method getChildren on dynamic is not lowered
```

- **cds** — CDS Parser define table entity with association/composition wrapped in CDSTableField
  Source sites: `test/cds/cds_parser.ts:3782:3`, `test/cds/cds_parser.ts:7:1`.

## 16. trapped: 1 test

```text
test/cds/cds_parser.ts:3842:78: lowering trap: unsupported-call test/cds/cds_parser.ts:3852:17: method getFirstToken on dynamic is not lowered; unsupported-expr test/cds/cds_parser.ts:3853:12: identifier tok is not a lowered local or module value
```

- **cds** — CDS Parser — quoted identifiers FROM-clause alias '"=A0"' token value preserved in CDSAs → CDSName
  Source sites: `test/cds/cds_parser.ts:3842:3`, `test/cds/cds_parser.ts:3815:1`.

## 17. trapped: 1 test

```text
test/objects/domain.ts:129:22: value outside supported numeric, ordering or conversion domain
```

- **objects** — Domain, parse main xml parser error
  Source sites: `test/objects/domain.ts:129:3`, `test/objects/domain.ts:101:1`.

## 18. trapped: 1 test

```text
test/objects/domain.ts:138:33: value outside supported numeric, ordering or conversion domain
```

- **objects** — Domain, parse main xml parser error, valid xml
  Source sites: `test/objects/domain.ts:138:3`, `test/objects/domain.ts:101:1`.

## 19. trapped: 1 test

```text
test/objects/view.ts:97:43: value outside supported numeric, ordering or conversion domain
```

- **objects** — View, parse XML database view with 2 append views
  Source sites: `test/objects/view.ts:97:3`, `test/objects/view.ts:7:1`.

## 20. trapped: 1 test

```text
test/pretty_printer/pretty_printer.ts:179:31: lowering trap: unsupported-expr test/pretty_printer/pretty_printer.ts:183:7: delete supports ordinary lowered records only
```

- **pretty_printer** — Remove sequential blanks, no config REPORT zfoo.
  Source sites: `test/pretty_printer/pretty_printer.ts:179:5`, `test/pretty_printer/pretty_printer.ts:178:9`, `test/pretty_printer/pretty_printer.ts:170:1`.

## 21. trapped: 1 test

```text
test/registry.ts:452:62: lowering trap: unsupported-expr test/registry.ts:460:5: delete supports ordinary lowered records only
```

- **root** — exclude list will not crash with an undefined global exclude list
  Source sites: `test/registry.ts:452:3`, `test/registry.ts:368:1`.
