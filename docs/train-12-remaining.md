# Train 12 remaining Node-active tests

Measured production compiler: `4594997`; final test-only repair: `f679289`. Original upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`. All 17 shards and 625 files were measured against fresh original Node, with 11,060 active cases.

**9 remaining = 7 traps + 2 unregistered + 0 skipped active cases. FAIL 0; async-pending 0.** Node-pending cases are excluded from this tail.

Groups use the exact diagnostic message after removing disposable closure prefixes. Source coordinates, node IDs and wording are retained. Every entry includes the full original title, shard, and TypeScript registration sites captured from original Node source maps. Repeated titles represent distinct registrations.

## 1. not-run: 2 tests

```text
unregistered (no Go test registration)
```

- **rules** — test line_length rule "hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello hello" should have 1 issue(s)
  Source sites: `test/rules/_utils.ts:60:9`, `test/rules/_utils.ts:39:11`, `test/rules/_utils.ts:36:3`, `test/rules/line_length.ts:17:9`.
- **rules** — test line_length rule "WRITE: / 'abc'." should have 0 issue(s)
  Source sites: `test/rules/_utils.ts:60:9`, `test/rules/_utils.ts:39:11`, `test/rules/_utils.ts:36:3`, `test/rules/line_length.ts:17:9`.

## 2. trapped: 2 tests

```text
src/lsp/rename.ts:93:3: lowering trap: src/lsp/rename.ts:93:3: HIR verification: node 168590 (src/lsp/rename.ts:98:52): assignment type mismatch
```

- **lsp** — LSP, rename variable simple
  Source sites: `test/lsp/rename_variable.ts:36:3`, `test/lsp/rename_variable.ts:8:1`.
- **lsp** — LSP, rename variable method parameter
  Source sites: `test/lsp/rename_variable.ts:56:3`, `test/lsp/rename_variable.ts:8:1`.

## 3. trapped: 1 test

```text
src/lsp/help.ts:39:3: config JSON own-property order is unproven for src/abap/1_lexer/tokens/identifier.ts.Identifier
```

- **lsp** — LSP, help CDS
  Source sites: `test/lsp/help.ts:39:3`, `test/lsp/help.ts:7:1`.

## 4. trapped: 1 test

```text
src/lsp/help.ts:93:3: config JSON own-property order is unproven for src/abap/4_file_information/_identifier.ts.Identifier
```

- **lsp** — LSP, help class
  Source sites: `test/lsp/help.ts:25:3`, `test/lsp/help.ts:7:1`.

## 5. trapped: 1 test

```text
src/lsp/rename.ts:104:3: lowering trap: src/lsp/rename.ts:104:3: HIR verification: node 168691 (src/lsp/rename.ts:109:52): assignment type mismatch
```

- **lsp** — LSP, rename method rename method
  Source sites: `test/lsp/rename_method.ts:10:3`, `test/lsp/rename_method.ts:8:1`.

## 6. trapped: 1 test

```text
src/lsp/semantic.ts:42:3: lowering trap: src/lsp/semantic.ts:42:3: HIR verification: node 169076 (src/lsp/semantic.ts:50:23): invalid narrowing classvalue to classref<shape.c3b70776d029edfc69fd75ab61b91f27fe8deaade7fdd314df00aa7ee5f66866>
```

- **lsp** — LSP, semantic highlighting basic
  Source sites: `test/lsp/semantic.ts:10:3`, `test/lsp/semantic.ts:8:1`.

## 7. trapped: 1 test

```text
src/objects/domain.ts:100:3: input is outside the supported strict JSON subset
```

- **objects** — Domain, parse AFF json parser error
  Source sites: `test/objects/domain.ts:92:3`, `test/objects/domain.ts:7:1`.
