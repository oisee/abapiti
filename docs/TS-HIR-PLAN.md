# Plan: abaplint in ABAP, through a typed object HIR

Decided 2026-10-07. A second track next to [PLAN.md](PLAN.md) (make the
WebAssembly path fast). It replaces the parser-only `ts/` prototype.

## Vision

Translate the TypeScript that abaplint and the abaplint transpiler are written
in into readable ABAP classes: a TypeScript class becomes an ABAP class, a
method a method, `extends` becomes `INHERITING FROM`. The result runs wherever
ABAP runs: on a real kernel, on open-steamgate's Go runtime (osgo) and on
OSG-JS. osgo then gets an ABAP parser and linter without Node and without a Go
port, because it already runs ABAP.

We do not need all of TypeScript, only the subset these two code bases use, and
we measured which one (below). The measurement covers the packages' own
sources; their npm dependencies (`json5`, `fast-xml-parser`,
`vscode-languageserver-types`, `source-map`, Node's `crypto`) need adapters,
translation or exclusion, decided per phase.

## Why not the other routes

- **ramune's TS → Go** (`ramune transpile`, commit 91cd313): on abaplint core
  and the transpiler none of the seven outputs builds. Its `--hybrid` mode
  extracts 1 function and 2 of 1,728 classes from core and nothing from the
  transpiler; the main reason is that it rejects every class with
  `extends`/`implements`.
- **Low-level IRs** (MIR/SSA as in MinZ, LLVM IR, WebAssembly) turn classes into
  structs with byte offsets and strings into bytes. ABAP has classes,
  inheritance, interfaces, exceptions and UTF-16 strings; going below the object
  level throws that away and has to rebuild it. The WebAssembly path keeps the
  JavaScript semantics only by shipping a whole JavaScript engine.
- **The `ts/` prototype** reads the syntax tree without a type checker and
  emits every class `FINAL`, without inheritance or interfaces.

## The subset (measured 2026-10-07)

TypeScript 6.0.3 checker over abaplint core 577f875eb (1,541 files, 81K lines,
1,731 classes) and the transpiler 67df3280 with 48 uncommitted changes in the
measured worktree (320 files, 15K lines, 308 classes). Counts are for both
packages together:

| Feature | Sites | Consequence |
|---|---|---|
| `extends` / `implements` / `instanceof` | 941 / 913 / 3,047 | object-oriented core; `instanceof` is how nodes are inspected |
| classes as values (`typeof X`, sets of constructors) | used by the parser combinators | class descriptors at run time |
| `T \| undefined` (type sites) / class unions | 18,397 / 1,636 | optional values, narrowing from the checker |
| arrow functions / closures capturing outer bindings that are written somewhere | 296 / 6 | simple closures |
| division / `parseFloat` | 2 / 1 | explicit fractions are rare, but most `number` sites are not proven integers; keep binary64 by default |
| `Map` / `Set` (type sites) | 106 / 287 | insertion-ordered collections |
| regex literals / `new RegExp` | 324 / 42 | in the statically known patterns: no lookbehind, named groups or Unicode, 5 lookaheads; 41 patterns are built at run time and must be checked |
| generators / async | 4 / 3 | rewritten by hand |
| getters, setters, decorators, namespaces | 0 | not needed |
| files in import cycles | 971 | modules flatten into one namespace |

## Architecture

```
tsgo Program + checker  →  typed object HIR  →  normalisation  →  ABAP legalisation  →  ABAP emitter
       (tsfront)               (hir)              (lower)             (7.02 rules)        (emit/abap)
```

- **Front end: tsgo** (Microsoft's TypeScript 7 compiler in Go), vendored at a
  pinned commit the way ramune does it (`scripts/sync-tsgo.sh`: copy the
  `internal/` packages and rewrite import paths). Only `tsfront` sees tsgo
  types.
- **HIR: new, owned by abapiti.** MinZ's HIR (`minzc/pkg/hir`) is the model for
  the layering (typed AST, names resolved, a type on every expression,
  structured control flow), the text dump and the oracle tests, but not for the
  types or nodes: its type interface `mir2.Ty` is sealed and has no strings,
  doubles or objects, fields carry byte offsets, methods are free functions and
  interfaces are monomorphised without vtables. Two independent reviews (codex
  gpt-6-astra and Claude Fable 5.1, 2026-10-07; their answers are not
  published) chose this option over reusing or extending
  MinZ's HIR. A shared, versioned dump subset can come later if both sides want
  it.
- **HIR core.** Types: `Bool`, `Number` (binary64) with proven `I32`/`I64`,
  `String` (UTF-16), `Optional<T>`, `Union`, `ClassRef`, `InterfaceRef`,
  `ClassValue`, `Array`, `Tuple`, `OrderedMap`, `OrderedSet`, `RegExp`,
  `Function`, `Dynamic`. Statements: block, variable, assign, if, loop, for-each,
  switch, labelled break/continue, return, throw, try/catch/finally. Expressions
  carry a type and a source span: field/static/index access, direct, virtual,
  interface and super calls, `New`, `InstanceOf`, checked cast, `IsUndefined`,
  `IsNull`, `ToBoolean`, closures with explicit captures and shared cells, and
  `RuntimeOp` for library calls (string, array, map, regex) from one typed
  catalogue.
- **Semantics are explicit in the HIR**, so emitters never guess: truthiness
  and null checks are nodes; narrowing the checker proves and `as` assertions
  (no run-time check in TypeScript) become typed views; `instanceof` is a
  boolean ancestry test. ABAP's downcast (`?=`) checks at run time and can
  raise where TypeScript would not, so it is emitted only where the checker or
  a dominating `instanceof` proves it; other assertions are rejected with their
  source location until handled;
  `Number` stays a double unless the integer range is proven (abaplint has
  `Math.ceil(x / 2)`); evaluation order is preserved when temporaries are
  introduced.
- **ABAP mapping.** Classes and inheritance map directly. `instanceof` goes
  through a runtime helper, because `IS INSTANCE OF` is not in 7.02. Class
  values become descriptors (name, parent, factory). Closures become small
  classes behind one interface per signature. `Map`/`Set` become runtime
  classes that keep insertion order. Long names go through one name table
  (30-character limit). The rules we already hold apply: no comments, lines of
  at most 255 characters, ABAP 7.02 plus `int8`, no writes to importing
  parameters.
- A Go emitter can be added later on the same HIR (virtual dispatch through
  interfaces); osgo may not need it.

## How it is verified

1. **Static gate on every build:** abaplint at `v702` plus our generation rules
   on the generated code.
2. **Differential runs on a corpus:** the same ABAP sources go through the
   original TypeScript (Node) and the generated ABAP; outputs are compared as
   JSON (tokens, statements, structures, findings). Generated ABAP runs on osgo
   and on OSG-JS. OSG-JS itself runs on the transpiler we translate, so its
   agreement alone is not proof; osgo and the kernel are independent.
3. **A4H smoke after every phase,** not only at the end: the kernel is stricter
   than both runtimes (see open-steamgate issues
   [#537–#543](https://github.com/oisee/open-steamgate/issues?q=label%3Akernel-diff)),
   and the kernel's speed for `instanceof` and the collections must be
   measured there.
4. abaplint's own unit tests later, once the runtime can carry chai-style
   assertions.

## Phases

| Phase | Content | Done when |
|---|---|---|
| 0 | `hir` package: nodes, verifier, versioned dump, name table; ABAP emitter for hand-built HIR; runtime classes for optional values, `instanceof`, ordered map and set | hand-built fixtures run on osgo and OSG-JS, pass the static gate; one A4H run |
| 1 | `tsfront`: vendored tsgo, Program from a tsconfig, lowering of classes, methods, fields, statements, strings, arrays, and the `Set` and regex operations the lexer uses | abaplint's lexer (`src/abap/1_lexer`) translated; tokens equal to Node's on the corpus, on osgo; A4H run |
| 2 | Closures, class values, the rest of `Map`/`Set` and regex, exceptions | the statement parser and combinators (`2_statements`) equal on the corpus |
| 3 | Structures and syntax (`3_structures`, `5_syntax`), flattened modules | structures and syntax findings equal on the corpus |
| 4 | Rules and the transpiler | selected rules equal; the transpiler's output for the corpus equal |

Each phase is a pull request with a review and green CI. Code is written by
codex (gpt-6.1-sol), reviewed by a codex critic.

## Risks

1. **Weakened semantics:** treating numbers as integers, strings as bytes, or
   `undefined` as `null` silently changes behaviour. The HIR keeps the JS
   meaning; narrowing to `int8` or a plain reference needs a proof.
2. **Linking and initialisation order** across 971 files in cycles: type-only
   imports must not create run-time dependencies; static initialisers need an
   explicit order.
3. **Size of the runtime surface** (strings, arrays, regex, dynamic property
   access on 187 sites with unresolved types): every operation is listed in the
   catalogue and either supported on all three runtimes or rejected with a
   source location.
4. **Platform limits:** 2,039 classes (1,731 in core), 30-character names, method counts per
   class (osgo fails with too many test methods), kernel nesting limits.
