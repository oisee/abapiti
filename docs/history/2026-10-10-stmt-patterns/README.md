# Statement folding priorities on the pinned abaplint hot paths

Analysis only, 2026-10-10. No emitter or HIR transformation was changed.
Measured clone baseline: `3536c15` (abapiti main); analyser: `cmd/stmt-patterns` on this analysis branch.
Upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`, core 2.120.56.
The exact requested `abapiti abaplint -o out` build produced 2,110 objects
(2,035 classes / 75 interfaces), zero blocking diagnostics and zero HIR
verification errors. Existing Grace inlining and singleton specialization were
enabled. [manifest.json](manifest.json) fingerprints all emitted classes and
`names.json`; `-hir` re-emitted selected classes and required byte equality.

**Start with dead scratch initialization, typed single-use forwarding, and
same-type CAST/receiver folding.** They occur in the combinator loops and in
Lexer.process, including bodies expanded by existing HIR inlining. Then remove
constant reconstruction, especially numeric bounds in tiny Result methods,
and emit predicates without materializing a boolean where that is legal.

## Counts and interpretation

69 method bodies contain 9,881 lexical ABAP statement sites, 656 original TS
statement sites and 1,577 original TS operation sites. Five bodies are coverage
traps and are marked unexecuted. The lexical aggregate ratio is 15.06 ABAP/TS
statement and 6.27 ABAP/TS operation. These are static expansion ratios, not
measurements of executed statement counts.

A lexical ABAP statement ends in a period outside comments/literals/templates.
Counts include inline initialized DATA, plain declarations, block terminators,
ELSE and exceptional branches; METHOD/ENDMETHOD are excluded. The executable
site proxy is 8,207 after excluding plain declarations and control markers.
It remains a static proxy: branch conditions, early exits, exceptions and loops
control visits. The 9,881 figure must not be multiplied by a call count as if
all sites execute on every call.

TS statements are AST statement nodes excluding blocks. TS operations are
call/new, property/index access, binary/unary/conditional operations, assertions,
and array/object creation. Identifiers and primitive literal leaves are not
operations. This deliberately counts a method call and its member access as
two distinct AST operations. Neither TS denominator is dynamic.

Synthesized `_one` methods have no independent original TS body: their ratios
compare against the original method's body, explicitly marked in `methods.csv`.
HIR counts are after the existing singleton/inlining transformations on the
fresh lowering and before expansion into individual ABAP statements. The
backend annotates/promotes types during re-emission; the HIR counts describe
that same generation pipeline. [hir-hot.txt](hir-hot.txt) includes the run
methods and Result.peek/remainingLength. HIR statements exclude block nodes;
HIR operations exclude literal/local/this leaves and include Seq substatements.
The HIR dump renders Seq expressions compactly; the counters traverse their
statement payloads as well.

Generated interface forwarding wrappers and class-descriptor methods are
excluded from original-body comparisons. Wrapper and callee costs still exist
at runtime. Alternative, AlternativePriority, Regex, Star and Permutation
`run_one` bodies are small forwarders that construct a singleton argument and
call `run`; their local counts/savings exclude the delegated `run`. Small
`run_one` numerators therefore do not imply low inclusive cost. Word, Token,
Sequence, Expression and Optional have specialized bodies. Plus.run is itself
small because it delegates.

The A4H context supplied for this analysis is an untraced ~50–100 ns per
executed small statement, ~100 ns LOOP entry and ~20 ns for a `?=` cast.
Traced hit lists exaggerate small-statement cost. The ~200 s zabapgit run is
consistent with the supplied rough 2–4 billion executed statements, but gives
no distribution among methods. No new kernel profile was collected here.

## Ranking and safety

| Priority | Rule | Static evidence across selected bodies | Best placement | Safety condition |
|---:|---|---|---|---|
| 1 | Remove scratch VALUE initialization / CLEAR before an unconditional first write | 786 VALUE sites (439 in loops) + 619 CLEAR sites (306 in loops) = 1,405 separate initialization statements | Emitter-local definite-assignment peephole; Grace for equivalent HIR VarDecl defaults | Prove first read is dominated by the overwrite on every path, including caught exceptions; preserve a typed declaration; no self-read, escape, field-symbol alias or reference to the local slot; do not remove resets that matter on the next loop iteration. |
| 2 | Forward a single-use typed temp into its assignment or sole argument | 768 adjacent assignment sites (372 in loops), 407 adjacent argument/function-use sites (197 in loops); 65 return copies are a subset | Emitter-local for emitter scratch; Grace HIR->HIR for real single-use VarDecl/Assign chains | One use of the definition; no intervening effect or mutation of its inputs; preserve evaluation order, by-value/by-reference parameter behavior and destination conversion. Retain explicit CONV when intermediate type widening/narrowing is observable. No aliased local slots. |
| 3 | Fold CAST temps and redundant receiver copies | 840 single-use CAST/?= definitions (462 in loops); 59 immediate method receiver copies (42 in loops), overlapping CAST/argument families | Emitter-local with resolved HIR types; Grace can remove proven identity Cast/Narrow nodes | Erase a cast only when exact resolved reference types prove it cannot fail; otherwise keep the cast at its original evaluation point. Moving CAST into a call must preserve null behavior, exception order, operand evaluation, identity and aliasing. A downcast through object/union is not automatically redundant. |
| 4 | Reuse immutable constants; move bound construction out of each call | 114 numeric-bound assignment sites, all outside loops; 303 scalar literal materializations (134 in loops). Result.remainingLength/shift and Token/Word each rebuild six bound assignments | Emitter-local CONSTANTS/class constants for target range guards; Grace constant promotion for immutable HIR literal aggregates | Same literal type, overflow/conversion behavior and JS safe-integer guard values. Preserve the guard itself. Only immutable values: never share mutable arrays/structures/objects or change observable identity/allocation. |
| 5 | Emit a direct predicate instead of a one-use bool + IF | 120 direct xsdbool/IF pairs (61 in loops); n-grams also find 117 bool-result/copy pairs | Emitter-local predicate lowering; Grace can forward real HIR boolean bindings | Exactly one read at the branch; preserve lazy &&/||, evaluation count/order, exception behavior and exact ABAP boolean comparison semantics. Do not replace arbitrary truthiness with a predicate. |

Counts in different families overlap. In particular, a CAST temp can also be
an argument, receiver or assignment candidate; a bool can have an overwritten
VALUE initialization and several copies. The ranking is qualitative because
no call/iteration frequencies were supplied. Apply and recount one rule at a
time. The first rule is largest even without relying on small cast timings;
the second provides the use-def machinery needed to prove the third safely.

Most of these scratch temporaries are absent from HIR: a single semantic
HIR expression becomes many DATA/CLEAR/assignment statements. A Grace pass
alone cannot remove emitter-created scratch. HIR rules should handle semantic
bindings and exact-type identity casts, with an emitter-local pass handling
ABAP materialization. Existing inlining exposes both kinds of opportunities.

## Per-call estimates and profile mapping

All six supplied class prefixes resolve through `names.json`:

| Profile class | Emitted class |
|---|---|
| Sequence B56EAE38 | Z_SRC_ABAP_2_S_B56EAE38AA1841 |
| AlternativePriority 97ADC9EF | Z_SRC_ABAP_2_S_97ADC9EFEFB8D1 |
| Expression 4228F5CC | Z_SRC_ABAP_2_S_4228F5CCB84481 |
| Token 53E10374 | Z_SRC_ABAP_2_S_53E103748B7037 |
| Word 63FEEC61 | Z_SRC_ABAP_2_S_63FEEC617D3DA8 |
| Regex DA316F76 | Z_SRC_ABAP_2_S_DA316F761B216A |

Lexer.process/add and the ABAPFileInformation lookup family receive the
supplied qualitative hot label. LexerStream, Result and StatementParser are
supporting hot-path families; the supplied profile does not rank their methods.
The labels identify families, not newly measured per-method frequencies.

For method m and pattern p, the actual candidate statement reduction per call
is `sum(site_visits_per_call(s) * saved_statements(s))`. Each detected folding
site here proposes one removed executed statement. Outside a loop, a site is
visited zero or one times depending on the path. A nested-loop site's visits
can be the product of iteration counts, filtered by branches and exits.

The next table gives a **site-count envelope** `a + bI`: every outside-loop
site once, every loop site I times. It is a comparison scenario/upper envelope,
not a feasible path through every branch, a proven legal fold count, or an
inclusive count of called methods. For unequal/nested loops substitute the
individual site visits from `occurrences.csv`; I represents visits to a site,
not simply the outer loop's trip count. Each column is evaluated independently.
[savings.csv](savings.csv) gives every pattern for every measured method.

| Method | Dead VALUE + CLEAR | Assignment forwarding | CAST temps | Bool IF | Numeric-bound constants |
|---|---:|---:|---:|---:|---:|
| Lexer.add | 263 + 2I | 174 | 89 | 38 | 8 |
| Lexer.process | 8 + 251I | 3 + 169I | 3 + 71I | 0 + 24I | 8 |
| LexerStream.getOffset | 1 | 2 | 0 | 0 | 0 |
| AlternativePriority.run | 2 + 8I | 1 + 2I | 3 + 13I | 0 + 2I | 0 |
| AlternativePriority.run_one | 4 | 2 | 4 | 0 | 0 |
| Expression.run | 4 + 21I | 3 + 9I | 6 + 15I | 0 + 1I | 6 |
| Expression.run_one | 8 + 18I | 6 + 7I | 9 + 12I | 0 + 1I | 6 |
| Regex.run | 2 + 29I | 1 + 12I | 3 + 18I | 0 + 2I | 6 |
| Regex.run_one | 4 | 2 | 4 | 0 | 0 |
| Sequence.run | 2 + 16I | 1 + 7I | 3 + 21I | 0 + 4I | 0 |
| Sequence.run_one | 16 + 3I | 7 + 2I | 16 + 7I | 3 + 1I | 0 |
| Token.run | 2 + 30I | 1 + 11I | 3 + 19I | 0 | 6 |
| Token.run_one | 33 | 13 | 22 | 0 | 6 |
| Word.run | 2 + 32I | 1 + 13I | 3 + 19I | 0 | 6 |
| Word.run_one | 35 | 15 | 22 | 0 | 6 |
| Result.peek | 1 | 0 | 2 | 0 | 0 |
| Result.peekAt | 2 | 1 | 2 | 0 | 8 |
| Result.remainingLength | 4 | 2 | 1 | 0 | 6 |
| Result.shift | 4 | 3 | 4 | 0 | 6 |
| ABAPFileInformation.getClassDefinitionByName | 3 + 9I | 0 + 10I | 1 + 4I | 0 + 2I | 0 |
| ABAPFileInformation.getClassImplementationByName | 3 + 9I | 0 + 10I | 1 + 4I | 0 + 2I | 0 |
| ABAPFileInformation.getInterfaceDefinitionByName | 3 + 9I | 0 + 10I | 1 + 4I | 0 + 2I | 0 |

For an ordinary non-throwing **Result.remainingLength call**, four initial
VALUE assignments and six numeric-bound reconstruction assignments are
unconditionally unnecessary if replaced with declarations and immutable bound
constants: **10 executed statements per call**, before any CAST/copy folding.
The remaining overflow check stays. This is about 0.5–1.0 microseconds/call
under the supplied small-statement model. The one exact-type receiver CAST
and conversion/copy chains offer further reductions after type checks; they
are not included in that ten.

**LexerStream.getOffset** is a five-site straight-line method for one TS
return: initial VALUE, CONV into temp, assignment to another temp, result
assignment and RETURN. Preserving the HIR's i32->i64 conversion in a single
result assignment leaves result assignment + RETURN: **three statements
saved per normal call**, roughly 150–300 ns under the supplied model.

In **Word.run_one**, the envelope is 35 dead initializations, 15 adjacent
assignment-forwarding sites, 22 CAST definitions and six bound assignments.
Most match-arm sites do not run on a mismatch; these values cannot be summed
as an actual per-call reduction. Token.run_one is similarly 33/13/22/6.
In **Sequence.run**, savings live predominantly in the nested combinator and
result loops; use their actual visits, rather than treating all loop sites as
one execution per method call. A copy fold generally does not remove LOOP
entry or the underlying cast: do not add 100 ns/20 ns to every candidate.

The recorded Word/Token/Regex/Expression HIR contains inlined Result operations.
This is why optimizing their standalone tiny getters alone does not repair the
already expanded hot callers. Fold materialization after inlining, so the
same rule catches both cases.

## What the n-grams reveal

Nine Unicode uppercasing expansions each contain 102 REPLACE statements.
The three ABAPFileInformation `get*ByName` methods each contain two such
expansions (204 replacements per body); one is outside the lookup loop and
one is inside. Three StatementParser methods each contain one expansion.
That explains 909 overlapping two-statement and 900 overlapping three-statement
REPLACE n-grams; it does **not** mean 1,809 independent removable sites.
The remaining REPLACE is a separate Lexer.process operation.

If a semantics-equivalent helper could replace one 102-statement expansion
with one call, it would save 101 **caller** statements each time that site
executes. Moving the identical 102 operations into a helper saves no total
executed statements, and replacing the exact Unicode behavior requires proof.
A lookup's two sites occur outside/inside its loop and account for the
large 65.40 lexical ABAP/TS ratios. This is a separate runtime/algorithm
opportunity, not one of the requested safe 2–3-statement folds.

Normalized names and temp numbers collapse to NAME/TMP; literal values
collapse to LIT, preserving the surrounding operation shape. N-grams stay
within each method but can cross control boundaries. `TMP = TMP` can be a
copy between two different variables, not an actual self-assignment.
The top 15 include an original example and location in the generated tables
below; [ngrams.csv](ngrams.csv) includes the full ordering and examples.

## Pattern coverage and limitations

- Temp-then-assign/argument requires one subsequent textual token use;
  literal text is masked and template interpolation expressions are counted.
  Adjacent candidates are a lower bound for broad copy propagation; explicit
  typed temporary assignments are included as well as inline DATA definitions.
- CAST candidates include both inline CAST and `?=` definitions with one
  subsequent use. Nonadjacent CAST candidates need additional effect/type
  analysis; they are not certified rewrites. Method receiver copies require
  an immediate subsequent call, so ordinary field-read copies are excluded
  from that narrower family.
- VALUE/CLEAR candidates search only a straight-line region until the first
  read/branch/loop/explicit alias. Calls that cannot see the local scratch slot
  can intervene. No CFG/dominance proof is claimed; a real pass must also prove
  no escape/alias and preserve reset behavior across iterations.
- Literal VALUE structures/ranges in these method bodies have **zero**
  identified sites. There are 303 scalar CONV-literal materializations and 114
  range-bound assignment statements instead. This is not a claim that the
  whole emitted program has no literal aggregates: the analyser is scoped to
  these families and does not infer constants assembled through arbitrary NEW,
  property stores and APPEND sequences. Mutable singleton arrays used by
  forwarding run_one methods cannot be shared as constants.
- `table_read_copy` is an additional structural idiom: READ TABLE followed by
  an assignment/downcast. Its 30 sites are only hypotheses; prove row/result
  types, aliasing, INDEX and sy-subrc semantics before merging anything.
- Static sites in different methods can arise from copies of the same inlined
  source operation; each is a real emitted site, not a dynamic independent call.
- Traps, exception arms and alternate branches remain in static totals.
  Numeric bounds are prologue assignments; scalar literal sites may be inside
  branches/loops. No traced hits were used as nanosecond weights.

## Statement mix

The exact per-method mix is in [mix.csv](mix.csv). Counts add to 9,881:

| Kind | Count |
|---|---:|
| assignment | 2354 |
| data_value_init | 1161 |
| declaration | 957 |
| clear | 927 |
| data_expression | 920 |
| replace | 919 |
| data_cast | 814 |
| control_marker | 717 |
| condition | 519 |
| transfer | 181 |
| call | 119 |
| cast_assignment | 76 |
| table_write | 66 |
| loop_entry | 56 |
| create | 50 |
| table_read | 35 |
| translate | 9 |
| concatenate | 1 |

Per-method compact mix (Cast includes inline CAST and ?=; Expr is other inline
DATA expressions; Ctrl includes IF/ELSEIF and control markers; Table includes
READ/APPEND/INSERT; Other includes declarations, transfers and runtime string
operations). These columns partition each method's lexical count.

| Method | Assign | VALUE init | CLEAR | Cast | Expr | Ctrl | Table | Loops | Other |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| Lexer.add | 487 | 276 | 95 | 89 | 292 | 322 | 1 | 2 | 115 |
| Lexer.process | 510 | 363 | 55 | 74 | 211 | 289 | 0 | 3 | 134 |
| LexerStream.advance | 34 | 19 | 3 | 0 | 26 | 22 | 0 | 0 | 17 |
| LexerStream.charCodeAt | 21 | 11 | 3 | 0 | 11 | 12 | 0 | 0 | 10 |
| LexerStream.constructor | 0 | 0 | 0 | 0 | 5 | 0 | 0 | 0 | 4 |
| LexerStream.currentChar | 17 | 9 | 3 | 0 | 11 | 11 | 0 | 0 | 8 |
| LexerStream.getCol | 1 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 1 |
| LexerStream.getOffset | 2 | 1 | 0 | 0 | 1 | 0 | 0 | 0 | 1 |
| LexerStream.getRaw | 1 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 1 |
| LexerStream.getRow | 1 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 1 |
| LexerStream.nextChar | 25 | 12 | 3 | 0 | 10 | 10 | 0 | 0 | 10 |
| LexerStream.nextNextChar | 25 | 12 | 3 | 0 | 10 | 10 | 0 | 0 | 10 |
| LexerStream.prevChar | 15 | 8 | 3 | 0 | 9 | 8 | 0 | 0 | 7 |
| LexerStream.prevPrevChar | 15 | 8 | 3 | 0 | 9 | 8 | 0 | 0 | 7 |
| Alternative.run | 8 | 4 | 10 | 15 | 1 | 5 | 3 | 2 | 12 |
| Alternative.run_one | 4 | 0 | 4 | 4 | 0 | 0 | 1 | 0 | 6 |
| AlternativePriority.run | 10 | 6 | 10 | 16 | 2 | 7 | 3 | 2 | 13 |
| AlternativePriority.run_one | 4 | 0 | 4 | 4 | 0 | 0 | 1 | 0 | 6 |
| Expression.run | 39 | 17 | 21 | 23 | 4 | 18 | 2 | 2 | 29 |
| Expression.run_one | 40 | 17 | 20 | 22 | 4 | 17 | 2 | 1 | 28 |
| Optional.run | 8 | 4 | 10 | 17 | 1 | 5 | 4 | 2 | 12 |
| Optional.run_one | 9 | 4 | 9 | 16 | 1 | 4 | 4 | 1 | 11 |
| Permutation.run | 27 | 14 | 21 | 29 | 9 | 11 | 5 | 4 | 24 |
| Permutation.run_one | 4 | 0 | 4 | 4 | 0 | 0 | 1 | 0 | 6 |
| Plus.run | 2 | 0 | 1 | 3 | 0 | 0 | 0 | 0 | 2 |
| Plus.run_one | 6 | 0 | 6 | 6 | 0 | 0 | 1 | 0 | 8 |
| Regex.run | 43 | 17 | 23 | 22 | 5 | 14 | 2 | 1 | 31 |
| Regex.run_one | 4 | 0 | 4 | 4 | 0 | 0 | 1 | 0 | 6 |
| Sequence.run | 20 | 10 | 16 | 24 | 7 | 13 | 4 | 3 | 20 |
| Sequence.run_one | 21 | 10 | 15 | 23 | 7 | 12 | 4 | 2 | 19 |
| Star.run | 20 | 9 | 12 | 24 | 4 | 17 | 3 | 2 | 19 |
| Star.run_one | 4 | 0 | 4 | 4 | 0 | 0 | 1 | 0 | 6 |
| Token.run | 46 | 16 | 27 | 23 | 4 | 14 | 3 | 1 | 34 |
| Token.run_one | 47 | 16 | 26 | 22 | 4 | 13 | 3 | 0 | 33 |
| Word.run | 48 | 18 | 27 | 23 | 5 | 14 | 3 | 1 | 34 |
| Word.run_one | 49 | 18 | 26 | 22 | 5 | 13 | 3 | 0 | 33 |
| Result.constructor | 10 | 5 | 4 | 6 | 4 | 6 | 0 | 0 | 11 |
| Result.fromChain | 3 | 0 | 3 | 7 | 4 | 0 | 0 | 0 | 6 |
| Result.getNodes | 29 | 8 | 14 | 7 | 8 | 17 | 2 | 3 | 25 |
| Result.getTokenIndex | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 2 |
| Result.getTokens | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 2 |
| Result.peek | 2 | 0 | 2 | 2 | 1 | 0 | 1 | 0 | 3 |
| Result.peekAt | 15 | 3 | 4 | 2 | 4 | 7 | 1 | 0 | 11 |
| Result.popNode | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 2 |
| Result.remainingLength | 11 | 4 | 0 | 1 | 1 | 2 | 0 | 0 | 4 |
| Result.setNodes | 0 | 0 | 0 | 1 | 2 | 0 | 0 | 0 | 2 |
| Result.shift | 14 | 3 | 4 | 4 | 5 | 7 | 0 | 0 | 9 |
| Result.wrapConsumed | 33 | 8 | 17 | 13 | 10 | 16 | 1 | 1 | 28 |
| StatementParser.buildSplits | 46 | 15 | 37 | 25 | 11 | 11 | 6 | 1 | 43 |
| StatementParser.categorize | 3 | 0 | 6 | 7 | 0 | 1 | 1 | 1 | 8 |
| StatementParser.categorizeStatement | 44 | 11 | 34 | 24 | 11 | 20 | 2 | 0 | 37 |
| StatementParser.constructor | 7 | 4 | 0 | 8 | 3 | 7 | 0 | 0 | 5 |
| StatementParser.lazyUnknown | 78 | 46 | 28 | 20 | 28 | 52 | 2 | 2 | 135 |
| StatementParser.match | 74 | 10 | 72 | 49 | 13 | 24 | 5 | 2 | 86 |
| StatementParser.nativeAfter | 45 | 30 | 15 | 25 | 17 | 24 | 7 | 2 | 125 |
| StatementParser.nativeSQL | 65 | 26 | 39 | 25 | 25 | 26 | 3 | 3 | 143 |
| StatementParser.process | 85 | 26 | 70 | 64 | 24 | 32 | 7 | 2 | 78 |
| StatementParser.removePragma | 39 | 16 | 22 | 17 | 22 | 27 | 4 | 2 | 29 |
| StatementParser.run | 28 | 2 | 37 | 36 | 4 | 9 | 2 | 4 | 48 |
| StatementParser.tokensToNodes | 3 | 0 | 5 | 6 | 0 | 1 | 1 | 1 | 7 |
| StatementParser.tokensToNodes_one | 4 | 0 | 4 | 5 | 0 | 0 | 1 | 0 | 6 |
| ABAPFileInformation.constructor | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 1 |
| ABAPFileInformation.getClassDefinitionByName | 31 | 15 | 12 | 6 | 18 | 26 | 0 | 1 | 218 |
| ABAPFileInformation.getClassImplementationByName | 31 | 15 | 12 | 6 | 18 | 26 | 0 | 1 | 218 |
| ABAPFileInformation.getInterfaceDefinitionByName | 31 | 15 | 12 | 6 | 18 | 26 | 0 | 1 | 218 |
| ABAPFileInformation.listClassDefinitions | 2 | 0 | 1 | 1 | 0 | 0 | 0 | 0 | 2 |
| ABAPFileInformation.listClassImplementations | 2 | 0 | 1 | 1 | 0 | 0 | 0 | 0 | 2 |
| ABAPFileInformation.listFormDefinitions | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 2 |
| ABAPFileInformation.listInterfaceDefinitions | 2 | 0 | 1 | 1 | 0 | 0 | 0 | 0 | 2 |

## Complete method ratios, pattern totals and top 15 idioms

`Exec sites*` is the static executable-site proxy defined above. Per-TS ratios
for synthesized `_one` methods refer to the original method body. Trapped
methods are visible for completeness and contribute no ordinary runtime calls.

| Method | ABAP | Exec sites* | TS stmts | TS ops | ABAP/TS stmt | ABAP/TS op | HIR stmts | Profile |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| Lexer.add | 1679 | 1395 | 123 | 293 | 13.65 | 5.73 | 145 | A4H hot (qualitative) |
| Lexer.process | 1639 | 1418 | 63 | 236 | 26.02 | 6.94 | 98 | A4H hot (qualitative) |
| LexerStream.advance | 121 | 104 | 9 | 25 | 13.44 | 4.84 | 9 | supporting hot-path family |
| LexerStream.charCodeAt | 68 | 57 | 3 | 8 | 22.67 | 8.50 | 3 | supporting hot-path family |
| LexerStream.constructor | 9 | 9 | 3 | 6 | 3.00 | 1.50 | 4 | supporting hot-path family |
| LexerStream.currentChar | 59 | 50 | 5 | 10 | 11.80 | 5.90 | 5 | supporting hot-path family |
| LexerStream.getCol | 3 | 3 | 1 | 1 | 3.00 | 3.00 | 1 | supporting hot-path family |
| LexerStream.getOffset | 5 | 5 | 1 | 1 | 5.00 | 5.00 | 1 | supporting hot-path family |
| LexerStream.getRaw | 3 | 3 | 1 | 1 | 3.00 | 3.00 | 1 | supporting hot-path family |
| LexerStream.getRow | 3 | 3 | 1 | 1 | 3.00 | 3.00 | 1 | supporting hot-path family |
| LexerStream.nextChar | 70 | 60 | 4 | 8 | 17.50 | 8.75 | 4 | supporting hot-path family |
| LexerStream.nextNextChar | 70 | 60 | 4 | 8 | 17.50 | 8.75 | 4 | supporting hot-path family |
| LexerStream.prevChar | 50 | 43 | 4 | 6 | 12.50 | 8.33 | 4 | supporting hot-path family |
| LexerStream.prevPrevChar | 50 | 43 | 4 | 6 | 12.50 | 8.33 | 4 | supporting hot-path family |
| Alternative.run | 60 | 46 | 7 | 11 | 8.57 | 5.45 | 9 | supporting hot-path family |
| Alternative.run_one | 19 | 15 | 7 | 11 | 2.71 | 1.73 | 3 | supporting hot-path family |
| AlternativePriority.run | 69 | 54 | 9 | 13 | 7.67 | 5.31 | 11 | A4H hot LOOP (qualitative) |
| AlternativePriority.run_one | 19 | 15 | 9 | 13 | 2.11 | 1.46 | 3 | A4H hot LOOP (qualitative) |
| Expression.run | 155 | 121 | 12 | 24 | 12.92 | 6.46 | 16 | A4H hot LOOP (qualitative) |
| Expression.run_one | 151 | 119 | 12 | 24 | 12.58 | 6.29 | 16 | A4H hot LOOP (qualitative) |
| Optional.run | 63 | 49 | 8 | 14 | 7.88 | 4.50 | 10 | supporting hot-path family |
| Optional.run_one | 59 | 47 | 8 | 14 | 7.38 | 4.21 | 10 | supporting hot-path family |
| Permutation.run | 144 | 115 | 12 | 30 | 12.00 | 4.80 | 20 | supporting hot-path family |
| Permutation.run_one | 19 | 15 | 12 | 30 | 1.58 | 0.63 | 3 | supporting hot-path family |
| Plus.run | 8 | 7 | 1 | 3 | 8.00 | 2.67 | 1 | supporting hot-path family |
| Plus.run_one | 27 | 21 | 1 | 3 | 27.00 | 9.00 | 4 | supporting hot-path family |
| Regex.run | 158 | 125 | 8 | 17 | 19.75 | 9.29 | 13 | A4H hot LOOP (qualitative) |
| Regex.run_one | 19 | 15 | 8 | 17 | 2.38 | 1.12 | 3 | A4H hot LOOP (qualitative) |
| Sequence.run | 117 | 92 | 16 | 22 | 7.31 | 5.32 | 20 | A4H hot LOOP (qualitative) |
| Sequence.run_one | 113 | 90 | 16 | 22 | 7.06 | 5.14 | 20 | A4H hot LOOP (qualitative) |
| Star.run | 110 | 86 | 18 | 21 | 6.11 | 5.24 | 20 | supporting hot-path family |
| Star.run_one | 19 | 15 | 18 | 21 | 1.06 | 0.90 | 3 | supporting hot-path family |
| Token.run | 168 | 131 | 5 | 17 | 33.60 | 9.88 | 10 | A4H hot LOOP (qualitative) |
| Token.run_one | 164 | 129 | 5 | 17 | 32.80 | 9.65 | 10 | A4H hot LOOP (qualitative) |
| Word.run | 173 | 136 | 5 | 18 | 34.60 | 9.61 | 11 | A4H hot LOOP (qualitative) |
| Word.run_one | 169 | 134 | 5 | 18 | 33.80 | 9.39 | 11 | A4H hot LOOP (qualitative) |
| Result.constructor | 46 | 37 | 5 | 9 | 9.20 | 5.11 | 8 | supporting hot-path family |
| Result.fromChain | 23 | 20 | 4 | 5 | 5.75 | 4.60 | 4 | supporting hot-path family |
| Result.getNodes | 113 | 87 | 8 | 13 | 14.12 | 8.69 | 12 | supporting hot-path family |
| Result.getTokenIndex | 4 | 4 | 1 | 1 | 4.00 | 4.00 | 1 | trapped (not executed) |
| Result.getTokens | 4 | 4 | 1 | 1 | 4.00 | 4.00 | 1 | trapped (not executed) |
| Result.peek | 11 | 9 | 1 | 3 | 11.00 | 3.67 | 1 | supporting hot-path family |
| Result.peekAt | 47 | 35 | 1 | 4 | 47.00 | 11.75 | 1 | supporting hot-path family |
| Result.popNode | 4 | 4 | 6 | 10 | 0.67 | 0.40 | 1 | trapped (not executed) |
| Result.remainingLength | 23 | 20 | 1 | 4 | 23.00 | 5.75 | 1 | supporting hot-path family |
| Result.setNodes | 5 | 5 | 5 | 10 | 1.00 | 0.50 | 1 | trapped (not executed) |
| Result.shift | 46 | 36 | 1 | 9 | 46.00 | 5.11 | 1 | supporting hot-path family |
| Result.wrapConsumed | 127 | 99 | 14 | 26 | 9.07 | 4.88 | 16 | supporting hot-path family |
| StatementParser.buildSplits | 195 | 151 | 9 | 26 | 21.67 | 7.50 | 21 | supporting hot-path family |
| StatementParser.categorize | 27 | 20 | 4 | 8 | 6.75 | 3.38 | 4 | supporting hot-path family |
| StatementParser.categorizeStatement | 183 | 137 | 15 | 38 | 12.20 | 4.82 | 22 | supporting hot-path family |
| StatementParser.constructor | 34 | 31 | 5 | 11 | 6.80 | 3.09 | 7 | supporting hot-path family |
| StatementParser.lazyUnknown | 391 | 339 | 15 | 68 | 26.07 | 5.75 | 17 | supporting hot-path family |
| StatementParser.match | 335 | 248 | 19 | 59 | 17.63 | 5.68 | 24 | supporting hot-path family |
| StatementParser.nativeAfter | 290 | 262 | 10 | 37 | 29.00 | 7.84 | 20 | supporting hot-path family |
| StatementParser.nativeSQL | 355 | 303 | 18 | 74 | 19.72 | 4.80 | 20 | supporting hot-path family |
| StatementParser.process | 388 | 299 | 29 | 61 | 13.38 | 6.36 | 56 | supporting hot-path family |
| StatementParser.removePragma | 178 | 139 | 15 | 28 | 11.87 | 6.36 | 19 | supporting hot-path family |
| StatementParser.run | 170 | 126 | 13 | 34 | 13.08 | 5.00 | 20 | supporting hot-path family |
| StatementParser.tokensToNodes | 24 | 18 | 4 | 4 | 6.00 | 6.00 | 4 | supporting hot-path family |
| StatementParser.tokensToNodes_one | 20 | 16 | 4 | 4 | 5.00 | 5.00 | 4 | supporting hot-path family |
| ABAPFileInformation.constructor | 3 | 3 | 1 | 2 | 3.00 | 1.50 | 1 | A4H lookup family (qualitative) |
| ABAPFileInformation.getClassDefinitionByName | 327 | 302 | 5 | 10 | 65.40 | 32.70 | 11 | A4H lookup family (qualitative) |
| ABAPFileInformation.getClassImplementationByName | 327 | 302 | 5 | 10 | 65.40 | 32.70 | 11 | A4H lookup family (qualitative) |
| ABAPFileInformation.getInterfaceDefinitionByName | 327 | 302 | 5 | 10 | 65.40 | 32.70 | 11 | A4H lookup family (qualitative) |
| ABAPFileInformation.listClassDefinitions | 6 | 5 | 1 | 2 | 6.00 | 3.00 | 1 | A4H lookup family (qualitative) |
| ABAPFileInformation.listClassImplementations | 6 | 5 | 1 | 2 | 6.00 | 3.00 | 1 | A4H lookup family (qualitative) |
| ABAPFileInformation.listFormDefinitions | 4 | 4 | 1 | 2 | 4.00 | 2.00 | 1 | trapped (not executed) |
| ABAPFileInformation.listInterfaceDefinitions | 6 | 5 | 1 | 2 | 6.00 | 3.00 | 1 | A4H lookup family (qualitative) |

| Pattern | Total | Outside loops | Inside loops |
|---|---:|---:|---:|
| temp_assign | 768 | 396 | 372 |
| temp_argument | 407 | 210 | 197 |
| cast_temp | 840 | 378 | 462 |
| value_init_overwritten | 786 | 347 | 439 |
| literal_constructor | 0 | 0 | 0 |
| literal_range_build | 0 | 0 | 0 |
| receiver_copy | 59 | 17 | 42 |
| bool_if | 120 | 59 | 61 |
| return_copy | 65 | 63 | 2 |
| table_read_copy | 30 | 10 | 20 |
| clear_before_write | 619 | 313 | 306 |
| literal_scalar | 303 | 169 | 134 |
| range_bound_assignment | 114 | 114 | 0 |

| Rank | N | Count | In loops | Normalized | Example location | Example |
|---:|---:|---:|---:|---|---|---|
| 1 | 2 | 909 | 606 | <code>REPLACE ALL OCCURRENCES OF LIT IN TMP WITH LIT. &#124; REPLACE ALL OCCURRENCES OF LIT IN TMP WITH LIT.</code> | src/abap/2_statements/statement_parser.ts.StatementParser.lazyUnknown:294 | <code>REPLACE ALL OCCURRENCES OF &#96;ß&#96; IN t12 WITH &#96;SS&#96;. REPLACE ALL OCCURRENCES OF &#96;ŉ&#96; IN t12 WITH &#96;ʼN&#96;.</code> |
| 2 | 3 | 900 | 600 | <code>REPLACE ALL OCCURRENCES OF LIT IN TMP WITH LIT. &#124; REPLACE ALL OCCURRENCES OF LIT IN TMP WITH LIT. &#124; REPLACE ALL OCCURRENCES OF LIT IN TMP WITH LIT.</code> | src/abap/2_statements/statement_parser.ts.StatementParser.lazyUnknown:294 | <code>REPLACE ALL OCCURRENCES OF &#96;ß&#96; IN t12 WITH &#96;SS&#96;. REPLACE ALL OCCURRENCES OF &#96;ŉ&#96; IN t12 WITH &#96;ʼN&#96;. REPLACE ALL OCCURRENCES OF &#96;ǰ&#96; IN t12 WITH &#96;J̌&#96;.</code> |
| 3 | 2 | 761 | 366 | <code>DATA TMP TYPE REF TO NAME. &#124; CLEAR TMP.</code> | src/abap/1_lexer/lexer.ts.Lexer.add:59 | <code>DATA t4 TYPE REF TO z_src_abap_1_l_cca5bd2c711d39. CLEAR t4.</code> |
| 4 | 2 | 290 | 164 | <code>CLEAR TMP. &#124; DATA(TMP) = CAST NAME( TMP ).</code> | src/abap/1_lexer/lexer.ts.Lexer.add:524 | <code>CLEAR t202. DATA(t203) = CAST z_src_position_90002a97f25e1e( t187 ).</code> |
| 5 | 3 | 290 | 164 | <code>DATA TMP TYPE REF TO NAME. &#124; CLEAR TMP. &#124; DATA(TMP) = CAST NAME( TMP ).</code> | src/abap/1_lexer/lexer.ts.Lexer.add:523 | <code>DATA t202 TYPE REF TO z_src_position_90002a97f25e1e. CLEAR t202. DATA(t203) = CAST z_src_position_90002a97f25e1e( t187 ).</code> |
| 6 | 2 | 241 | 120 | <code>CLEAR TMP. &#124; DATA TMP TYPE REF TO NAME.</code> | src/abap/1_lexer/lexer.ts.Lexer.add:494 | <code>CLEAR t187. DATA t188 TYPE REF TO z_src_position_90002a97f25e1e.</code> |
| 7 | 3 | 241 | 120 | <code>CLEAR TMP. &#124; DATA TMP TYPE REF TO NAME. &#124; CLEAR TMP.</code> | src/abap/1_lexer/lexer.ts.Lexer.add:494 | <code>CLEAR t187. DATA t188 TYPE REF TO z_src_position_90002a97f25e1e. CLEAR t188.</code> |
| 8 | 3 | 241 | 120 | <code>DATA TMP TYPE REF TO NAME. &#124; CLEAR TMP. &#124; DATA TMP TYPE REF TO NAME.</code> | src/abap/1_lexer/lexer.ts.Lexer.add:493 | <code>DATA t187 TYPE REF TO z_src_position_90002a97f25e1e. CLEAR t187. DATA t188 TYPE REF TO z_src_position_90002a97f25e1e.</code> |
| 9 | 2 | 180 | 93 | <code>TMP = TMP. &#124; ENDIF.</code> | src/abap/1_lexer/lexer.ts.Lexer.add:79 | <code>t11 = t13. ENDIF.</code> |
| 10 | 2 | 178 | 127 | <code>DATA(TMP) = CAST NAME( TMP ). &#124; TMP = TMP-&gt;NAME.</code> | src/abap/1_lexer/lexer.ts.Lexer.add:65 | <code>DATA(t8) = CAST z_src_abap_1_l_cca5bd2c711d39( t4 ). t7 = t8-&gt;z_member_raw_eb82124cb04f25.</code> |
| 11 | 2 | 148 | 88 | <code>TMP = TMP. &#124; TMP = TMP.</code> | src/abap/1_lexer/lexer.ts.Lexer.add:77 | <code>t13 = t9. t9 = t11.</code> |
| 12 | 2 | 135 | 78 | <code>ENDIF. &#124; TMP = TMP.</code> | src/abap/1_lexer/lexer.ts.Lexer.add:160 | <code>ENDIF. t1 = t2.</code> |
| 13 | 2 | 125 | 73 | <code>DATA(TMP) = VALUE ABAP_BOOL( ). &#124; DATA(TMP) = VALUE ABAP_BOOL( ).</code> | src/abap/1_lexer/lexer.ts.Lexer.add:234 | <code>DATA(t76) = VALUE abap_bool( ). DATA(t77) = VALUE abap_bool( ).</code> |
| 14 | 2 | 117 | 55 | <code>TMP = XSDBOOL( TMP = TMP ). &#124; TMP = TMP.</code> | src/abap/1_lexer/lexer.ts.Lexer.add:309 | <code>t105 = xsdbool( t106 = t107 ). t104 = t105.</code> |
| 15 | 2 | 113 | 65 | <code>DATA(TMP) = CAST NAME( TMP ). &#124; TMP = TMP.</code> | src/abap/1_lexer/lexer.ts.Lexer.add:525 | <code>DATA(t203) = CAST z_src_position_90002a97f25e1e( t187 ). t202 = t203.</code> |

## Reproduction and verification

```sh
mkdir -p .local/stmt-patterns/source
export TMPDIR="$PWD/.local/stmt-patterns"
export GOCACHE="$HOME/.cache/abapiti-go-cache"
export GOFLAGS=-buildvcs=false
tar -xzf tsfront/testdata/registrycorpus/abaplint-core-577f875e.tar.gz \
  -C .local/stmt-patterns/source
go build -o .local/stmt-patterns/abapiti ./cmd/abapiti
.local/stmt-patterns/abapiti abaplint -o out
go run ./cmd/stmt-patterns -hir
go vet ./...
go test -short ./cmd/...
```

The tool checks all 1,538 closure fingerprints, resolves class/member identities
through `out/names.json`, and with `-hir` checks selected class equality against
fresh lowering/re-emission. `out/` is generated build material and is not
committed. No emitter, front-end or existing HIR pass was edited.

Validation: `go vet ./...` passed; `go test -short ./cmd/...` passed, including
scanner safety/boundary tests. A repeat `-hir` run using a different scratch and
output directory produced byte-identical generated artifacts and stdout. Code
steps were committed as Alice V. <ooisee@gmail.com> and pushed after each green
step to `origin HEAD:refs/heads/analysis/stmt-patterns`; this report is the final
analysis step.

Files: [methods.csv](methods.csv), [mix.csv](mix.csv),
[patterns.csv](patterns.csv), [occurrences.csv](occurrences.csv),
[savings.csv](savings.csv), [ngrams.csv](ngrams.csv),
[tables.md](tables.md), [manifest.json](manifest.json), [hir-hot.txt](hir-hot.txt).
