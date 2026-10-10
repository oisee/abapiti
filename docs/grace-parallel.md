Grace milestone 5, step 1 — per-file and per-object proof

This change supplies `mark_parallel(Loop)` facts and a reproducible source audit.
It performs no rewrites, starts no workers, and changes no default build pass.
One of seven candidate regions is conditionally proven: the per-file lexer map.
The statement stage is one call in `ABAPParser.parse`; its two actual per-file
loops live in `StatementParser.run`, and both are reported separately.

Run `go run ./cmd/grace-parallel` for the candidate table, classifier counts, and
sorted join/warm-up obligations. The certificate is in
[cmd/grace-parallel/audit.grace](../cmd/grace-parallel/audit.grace); the proof rule
and input contract are in
[hir/rewrite/parallel](../hir/rewrite/parallel/README.md).
The command checks the embedded archive's SHA-256 before accepting the audit:
`5ea3cd76e2058563ca5311dcc50707e78df5fd04bcb0ce84255ccbd3d0c8be4b`.
The upstream source pin is `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.
All source paths below are relative to that pin's `packages/core/src/`.

The facts are source-audited certificates. They are not claimed to be automatic
ownership extraction from HIR. The old extractor treats returns and field stores
as escapes and cannot discharge the lexer's returned mutable object graph.
The certificate supplies that missing contextual argument, with exact source
identity and a finite closed-world input contract. The six negative candidates
retain decisive source witnesses and deliberately lack complete certificates;
removing a witness alone would not prove one of them parallel. The independent
naive evaluators check the Grace inference over the supplied premises, not the
truth of the source audit itself.

The lexer proof covers `abap/abap_parser.ts:68` (`files.map(f => new Lexer().run(f))`)
and the transitive methods below. `Lexer::*`, `LexerStream::*`, `LexerBuffer::*`,
`Token::*`, and `Position::*` are conservative aggregate summaries, rather than
claims that CHA has resolved each HIR site. Their source-level call/receiver
coverage is:

| Receiver / allocation | Source argument | Mutable ownership / shared reads |
|---|---|---|
| `Lexer.run`, `process`, `add` | Fresh `new Lexer()` for each map element. `run` initializes state; `process` allocates its stream and buffer. | All lexer fields belong to this iteration. `run` reads only `file.getRaw()` from input. |
| `LexerStream` constructor and accessors, `advance` | `abap/1_lexer/lexer_stream.ts`; raw is a string; all other fields are integer position state. | Stream is fresh; raw immutable; no static stores or external calls. |
| `LexerBuffer` constructor, `add`, `get`, `length`, `clear`, `countIsEven` | `abap/1_lexer/lexer_buffer.ts`; integer offsets over immutable raw. | Fresh buffer; no shared mutable fields. |
| Tokens and `AbstractToken` constructor/accessors | All `abap/1_lexer/tokens/*.ts`, including inherited constructors; statically named allocations in `Lexer.add`. | Fresh tokens and array. Token strings are immutable; positions are fresh. Token subclass railroad methods have no shared writes. Debug descriptions read class metadata; module `Symbol.for` runs before the region. |
| `Position` / `VirtualPosition` | `position.ts`, `virtual_position.ts`; constructors only store scalar position fields; comparisons read fields. | Fresh positions. This map passes no virtual position, so its virtual branch is unreachable; the audited envelope also covers its readonly scalar copying. |
| `IFile.getRaw` | `files/memory_file.ts:11` reads immutable raw; `abap/abap_file.ts:30` forwards to its wrapped file. | Only `MemoryFile` or acyclic `ABAPFile` wrappers over the same closed set. No input mutation during the map. A custom/effectful IFile is outside this proof. |
| Runtime primitives | String slicing/case/trim/char access/replacement, scalar arithmetic/comparisons, Set.has, local array push. | Pure primitive computations and writes to the fresh token array. Bounds/domain traps are allowed. No clock, registry, cache, progress or I/O call occurs in the iteration. Known direct token constructors discharge the otherwise conservative constructor-effect gap. |

The returned `{file,tokens}` contains a shared **read-only** file edge and a fresh
mutable token graph. It does not publish any object during the iteration.
`SPLITS`, `BUFS`, `AFTER_LITERAL`, numeric constants and class metadata are only
read. The `lexer-closure-initializers` warm-up precondition means all emitter
EnsureInit guards and module initialization in the above finite closure must
complete before workers could begin: lexer module (all sets/constants), stream
module constants, tokens index/module registrations, token classes, Lexer,
LexerBuffer, LexerStream, Position, VirtualPosition and the accepted file classes.
At the TS call site imports already precede the loop, but this audit does not
assume that proves the emitter's guarded first-use ordering. No memo/counter
exception is required. Results occupy private index slots, published in source
order. A join rethrows the lowest-index error and publishes no later results on
failure. Allocation failure remains excluded by the existing Effects contract.

The other candidates have these decisive chains:

| Actual loop | Proven | Preconditions / first blocking fact chain | Classification |
|---|---|---|---|
| `Registry.parse` object loop, calling `parsePrivate` | No | `input.parse → ABAPParser.parse → StatementParser.match → Combi.run → Combi.release / langVer = ...`. These fields are read by `Vers.run`, `LangVers.run`, etc. `ParsingPerformance.push` also updates shared floating-point timing totals and appends results. | Real dependency. Same-valued setting assignments still fail the requested memo/counter-only static-write contract. |
| `ABAPParser.parse` lexer `files.map` | Yes, conditional | Closed immutable IFile inputs; complete lexer warm-up above; private source-order result slots; lowest-index exception join. | Previous ownership blocker was an analysis gap, discharged by this source certificate. |
| `StatementParser.run`, first `for (w of wa)` | No | `categorize → match → Combi.run` as above; also `macros.find → Macros.addMacro`, first definition wins across files. `ExpandMacros.find` clears/adds registry macro references and recursively calls `Program.parse` for includes. | Real dependency. |
| `StatementParser.run`, second `for (w of wa)` | No | `handleMacros → expandContents → new StatementParser(...).run → Combi.run`; `MacroReferences.addReference` appends by **definition filename**, potentially shared by several input files. Readers/deduplication/clear operations forbid treating these as isolated output appends. | Real dependency. |
| `ABAPParser.parse`, `for (f of statementResult)` (structure + file information) | No | Ordered join discharges `output.push` and `issues.push` only. `StructureParser.runFile → singletons[class] = getMatcher()`; cached matcher run reaches `Alternative.setupMap` and `SubStructure.setupMatcher` guarded writes, plus module `sub()` singletons. | Analysis gap: object identity/immutability, all virtual matcher receivers, nested warm-up coverage and file-information effects lack a complete certificate. The cache exists but is not established as a value-semantic memo. |
| `RulesRunner.runRules`, first object loop (syntax) | No | `SyntaxLogic.run → traverseObject → CurrentScope.findTypePoolType / findTypePoolConstant → SyntaxLogic(typePool).run → typePool.syntaxResult = result`; the type pool can also be another loop element. Shared DDIC/MSAG references are cleared/updated. Optional progress/performance callbacks add visible effects. | Real dependency, with additional unresolved partition/receiver analysis. |
| `RulesRunner.runRules`, last object loop (all rules) | No | Shared enabled-rule receivers: `UnusedVariables.run → this.workarea = new WorkArea()`, followed by reads/writes through that field. `rulePerformance[key] = old + runtime` is shared read-modify-write and uses Number timing values. | Real dependency. Ordered join can discharge issue appends, but cannot discharge those receivers or floating-point totals. |

A macro pre-pass can deterministically collect local definitions in file order,
respect first-definition-wins and include traversal, then freeze the definition
map before expansion. It cannot by itself prove the present statement loops:
`Combi.run` still unconditionally stores settings, StatementMap and matcher lazy
caches still need complete warm-up, include parsing mutates other objects, and
macro-reference updates have shared readers and writers. This milestone neither
implements that pre-pass nor certifies a hypothetical rewritten loop.

The memo classifier requires purity **and** key completeness, immutable
value semantics/no observable allocation identity, no invalidation, and coherent
memo reads; it does not promote an arbitrary pure allocation into a memo.
Warm-before-loop requires a dominance certificate or outputs a mandatory warm-up
precondition, independently for every nested guard. Ordered appends require a
buffered concatenation plan and reject any shared array read, including length.
Commutative counters require exact nontrapping integer arithmetic and unobserved
intermediate totals, and output an atomic-counter condition. Counter effects
still block a possibly throwing iteration because they are externally visible.
No existing abaplint static cache or timing counter has been silently promoted.

The synthetic suite covers positive and near-miss negative cases for each of
those guards, recursive/transitive calls, mixed virtual receivers, missing targets,
unknown effects, missing complete summaries, exceptions, and input immutability.
Every case agrees on its full fact set with the independent Cartesian and hashed
naive evaluators. Reversed insertion order and repeat reports are deterministic.

Classifier counts are explicit about their scope. The audit's “before” withholds
only the complete `Lexer::*` receiver-graph certificate, preserving every source
witness. It is a controlled audit-local comparison, not a rerun of the older-base
static pass. Actual evaluated counts are:

| Proof classifier / relation | Before | After |
|---|---:|---:|
| Candidate loops | 7 | 7 |
| Complete effect summaries | 7 | 8 |
| Reachable (loop,summary) pairs | 23 | 23 |
| Accepted memo writes | 0 | 0 |
| Accepted counter writes | 0 | 0 |
| Allowed write tuples | 9 | 9 |
| Blocked candidates | 7 | 6 |
| `mark_parallel` | 0 | 1 |
| Preconditions, including partial plans | 7 | 8 |

The unmodified production HIR analysis on the pinned 1,538-file closure still
has the following counts. Since no extractor, Effects entry or analysis rule
changed, these are identical before and after this work:

| Legacy classifier | Before | After |
|---|---:|---:|
| `defined` | 6,257 | 6,257 |
| `confined` | 111 | 111 |
| `pure` | 3,498 | 3,498 |
| `may_throw` | 3,767 | 3,767 |
| `receivers` | 42,766 | 42,766 |
| `memo_store` | 0 | 0 |
| `counter_store` | 0 | 0 |
| `lazy_init` | 82 | 82 |
| `ensure_init` | 2,470 | 2,470 |
| Static write tuples classified memo / counter / other | 0 / 0 / 166 | 0 / 0 / 166 |

The current default Grace pass was measured on that full lowered program,
cloning it and running GC **outside** each timed interval. After one warm-up run excluded from
statistics, ten passes had median **0.450786 s**, range
0.439843–0.467547 s; every pass reported **1,487 sites, 190 callees**.
Lowering took 16.18 s and is excluded from those pass times. The temporary helper
was removed. The proof package is not imported by the default compiler and its
rules are not embedded into the default rule files. The default-cost gate also
compares the before/after compiler binaries with identical build metadata;
byte equality demonstrates zero added default-pass code or runtime overhead,
within the requested 5% budget, independently of timing noise. These pass samples
are a current-cost measurement, not a claim of a measured before/after speedup.
Evidence is retained in `$HOME/.cache/grace-parallel/` (`cost.log`,
`default-deps.txt`, `default-binary.sha256`, `candidates.md`, and gate logs).

Validation completed on `analysis/grace-parallel`, with heavy runs serialized by
`flock /tmp/abapiti-heavy.lock` and all artifacts in the clone or `$HOME/.cache`:

- `go vet ./...`: passed.
- `go test -short ./...`: passed, including all new proof and stale-input cases.
- `./.github/ci/lint.sh gate origin/main`: passed, 0 new issues, canary 5/5,
  golangci-lint 2.13.2.
- `ABAPITI_GRACECHECK=1 go test ./tsfront -run '^TestLexerFactsReport$' -count=1 -v`:
  passed; byte-identical fact golden, independent reference and inline oracle
  checks, 44-case corpus cardinality unchanged.
- The candidate report is deterministic and every displayed first blocker has
  an evaluated fact witness. Synthetic cases and the real certificate agree with
  both independent naive reference modes.
- Default compiler binary comparison: identical SHA-256
  `b6b2441dd62d95d50c985698108962515d3ca34b9852bfc678c4c7e2b25bf178`.
  `go list -deps ./cmd/abapiti` contains no parallel-proof package. Default-pass
  added code/runtime overhead is zero, within 5%.
