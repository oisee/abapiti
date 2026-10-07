Vendored TypeScript closure for the lexer lowering test.

`src/` is copied verbatim from abaplint core at commit
577f875ebec44cfaf64841cfe71c8ab8dc32622e (MIT, see LICENSE) — only the files
the lexer's transitive import closure needs: `position.ts`,
`virtual_position.ts`, `files/_ifile.ts` and `abap/1_lexer/**`.

`harness/` is abapiti's own differential-test driver (TestFile, TokenName,
LexerDump), written against the vendored interfaces; it is not part of
abaplint.

The tsconfig mirrors the options of abaplint core's own tsconfig that matter
for lowering (strictNullChecks on, strictPropertyInitialization off).
