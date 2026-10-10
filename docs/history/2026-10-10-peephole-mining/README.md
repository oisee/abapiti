# Peephole mining on main after #80/#81

Base db8173d; current build emitted 2036 classes and 75 interfaces, with zero blocking diagnostics and zero HIR verification errors. 13656 emitted class methods, 450294 lexical statements, 40459 distinct n=2..4 windows. 722 candidate shapes, 52756 syntactically eligible sites. Analysis-only changes; no runtime measurements.

Ranking: `sum((hot ? 4 : 1) * (in_loop ? 4 : 1)) * saved_per_exec`; ties use sites descending then pattern ascending. In-loop means every statement in the window has lexical loop depth > 0. Nested depth is not multiplied again. Hot methods: all combi.ts run/run_one; Lexer.process/add; LexerStream and Result except constructors; StatementParser run/tokensToNodes/tokensToNodes_one/buildSplits/categorize/categorizeStatement/removePragma/match/process/nativeSQL/nativeAfter/lazyUnknown; ABAPFileInformation, CurrentScope and SpaghettiScope get*/find*/lookup*/resolve*/has*/is*/exists*. Case-insensitive lookup prefixes. This is a prioritization heuristic, not dynamic profile data.

Raw windows may cross control boundaries, overlap, include coverage traps and generated forwarding methods; candidates require a declared TS source, one textual temp definition and one textual value use, equal loop depth and adjacent use with only plain DATA declarations between. 2156 methods containing an unimplemented-coverage exception constructor are conservatively excluded from the shortlist, including methods with partial traps. Each shape is a separate candidate specialization; shared families can share a rule. Safety gates remain pending. Overlaps and alternative specializations must not be added as independent savings.

The full census includes every .clas.abap file, including runtime and exception helpers; helpers without an original TS method retain their runtime/exception identity from names.json and are excluded only from the candidate shortlist. Generated caller/closure relationships are not used to invent inherited hot weights.

HIR tags classify the proposed semantic rule, not proof that every counted emitted scratch variable has an HIR binding. Grace must first establish binding provenance; pure emitter-created scratch instances are ABAP-only. HIR counts here are conditional opportunities, not measured Grace hit counts. Shapes whose consuming use contains ?=, CAST/CONV or a boxed oval field are classified ABAP-only even when the producer looks like a copy. Semantic copyprop, dead-store elimination, immutable constant-set membership and proven identity casts belong in Grace HIR -> HIR so TS-HG@Go also benefits; no unmeasured HIR hit counts are invented for those other families.

Normalization is exactly stmt-patterns: joined multiline/chained lexical statements, temp identities -> TMP, hashed names -> NAME, numeric literals -> NUM, quoted strings/templates -> LIT. It is intentionally lossy: TMP does not imply identity, and templates hide interpolation in normalized text. Concrete token scans retain simple interpolation uses; guards must use full typed def/use and control-flow facts. A chained statement remains one entry, never split at commas in constructor/call syntax.

A site saves **one executed store/assignment** if its guard succeeds; plain DATA declarations are not counted as runtime savings. Repeated iterations do not make an ordinary DATA declaration reset a local. No #80 initialization removals or #81 quoted numeric removal is proposed again.

Artifacts: [full candidate ranking](candidates.csv), [all n-grams](ngrams.csv), [eligible occurrences](occurrences.csv), [normalized statement entries](statements.json.gz), [input fingerprints](manifest.json), [A4H fixtures](A4H.md). Regenerate with the commands in cmd/stmt-patterns/README.md, then `python3 tools/peephole-bench.py`.

| ID | Tag | Family | Sites | In loop | Share | Hot weight | Saved/exec | Normalized pattern |
|---|---|---|---:|---:|---:|---:|---:|---|
| PM0001 | ABAP-only | cast | 5135 | 1453 | 0.2830 | 10970 | 1 | <code>DATA(TMP) = CAST NAME( TMP ). TMP = TMP.</code> |
| PM0002 | ABAP-only | cast | 3354 | 1245 | 0.3712 | 9081 | 1 | <code>DATA(TMP) = CAST NAME( TMP ). TMP = TMP-&gt;NAME.</code> |
| PM0003 | ABAP-only | conv | 3408 | 762 | 0.2236 | 6096 | 1 | <code>DATA(TMP) = CONV ABAP_BOOL( ABAP_TRUE ). IF TMP = ABAP_TRUE.</code> |
| PM0004 | HIR | copy | 2076 | 755 | 0.3637 | 5166 | 1 | <code>TMP = TMP-&gt;NAME. TMP = TMP.</code> |
| PM0005 | ABAP-only | abap-copy | 4477 | 21 | 0.0047 | 4564 | 1 | <code>TMP = TMP. TMP-&gt;OVAL = TMP.</code> |
| PM0006 | HIR | copy | 1796 | 570 | 0.3174 | 4172 | 1 | <code>TMP = TMP. TMP = TMP.</code> |
| PM0007 | ABAP-only | cast | 2521 | 19 | 0.0075 | 2602 | 1 | <code>DATA(TMP) = CAST NAME( TMP ). TMP = NAME=&gt;NAME( NAME = TMP NAME = TMP NAME = TMP ).</code> |
| PM0008 | HIR | copy | 2514 | 14 | 0.0056 | 2556 | 1 | <code>TMP = NAME=&gt;NAME( NAME = TMP NAME = TMP NAME = TMP ). TMP = TMP.</code> |
| PM0009 | ABAP-only | cast | 1297 | 342 | 0.2637 | 2539 | 1 | <code>DATA(TMP) = CAST NAME( TMP ). IF TMP IS INITIAL.</code> |
| PM0010 | ABAP-only | cast | 684 | 285 | 0.4167 | 1671 | 1 | <code>DATA(TMP) = CAST NAME( TMP ). TMP = TMP-&gt;NAME( ).</code> |
| PM0011 | ABAP-only | cast | 841 | 224 | 0.2663 | 1603 | 1 | <code>DATA(TMP) = CAST NAME( TMP ). TMP = TMP-&gt;VALUE.</code> |
| PM0012 | ABAP-only | abap-copy | 657 | 300 | 0.4566 | 1587 | 1 | <code>TMP = TMP. TMP ?= TMP.</code> |
| PM0013 | ABAP-only | cast | 822 | 206 | 0.2506 | 1509 | 1 | <code>DATA(TMP) = CAST NAME( TMP ). TMP = XSDBOOL( TMP IS BOUND ).</code> |
| PM0014 | HIR | copy | 728 | 171 | 0.2349 | 1463 | 1 | <code>TMP = TMP-&gt;NAME( NAME = TMP ). TMP = TMP.</code> |
| PM0015 | ABAP-only | bool | 738 | 194 | 0.2629 | 1392 | 1 | <code>TMP = XSDBOOL( TMP IS BOUND ). IF TMP = ABAP_TRUE.</code> |
| PM0016 | ABAP-only | downcast | 514 | 204 | 0.3969 | 1336 | 1 | <code>TMP ?= TMP. TMP = TMP.</code> |
| PM0017 | ABAP-only | conv | 407 | 173 | 0.4251 | 1202 | 1 | <code>DATA(TMP) = CONV STRING( LIT ). TMP = XSDBOOL( TMP = TMP ).</code> |
| PM0018 | ABAP-only | abap-copy | 555 | 166 | 0.2991 | 1155 | 1 | <code>TMP = TMP. DATA TMP TYPE REF TO NAME. DATA(TMP) = CAST NAME( TMP ).</code> |
| PM0019 | HIR | copy | 523 | 177 | 0.3384 | 1150 | 1 | <code>TMP = TMP-&gt;NAME( ). TMP = TMP.</code> |
| PM0020 | ABAP-only | abap-copy | 380 | 201 | 0.5289 | 1112 | 1 | <code>TMP = TMP. DATA TMP TYPE STRING. DATA(TMP) = CAST NAME( TMP ).</code> |

## Candidate details

### PM0001 — ABAP-only

`Z_SRC_ABAP_1_L_22309FB1998E15=>z_member_proce_57f9d1e4b4e34e:1690`; TS `src/abap/1_lexer/lexer.ts.Lexer.process` (`src/abap/1_lexer/lexer.ts:237:3`).

Before:
```abap
DATA(t22) = CAST z_src_abap_1_l_4d08aedee14b5b( t2 ).
t21 = t22.
```

Inline CAST target( x ) into the single argument/receiver/value use; remove its dead temp store. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t21 = CAST z_src_abap_1_l_4d08aedee14b5b( t2 ).
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: This candidate targets the emitted ABAP CAST/receiver syntax, not a proven redundant HIR cast.

### PM0002 — ABAP-only

`Z_SRC_ABAP_1_L_22309FB1998E15=>z_member_proce_57f9d1e4b4e34e:1695`; TS `src/abap/1_lexer/lexer.ts.Lexer.process` (`src/abap/1_lexer/lexer.ts:237:3`).

Before:
```abap
DATA(t26) = CAST z_src_abap_1_l_4d08aedee14b5b( t21 ).
t25 = t26->z_member_offse_4eb7e0915312eb.
```

Inline CAST target( x ) into the single argument/receiver/value use; remove its dead temp store. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t25 = CAST z_src_abap_1_l_4d08aedee14b5b( t21 )->z_member_offse_4eb7e0915312eb.
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: This candidate targets the emitted ABAP CAST/receiver syntax, not a proven redundant HIR cast.

### PM0003 — ABAP-only

`Z_SRC_ABAP_2_S_133BEB1C8EC314=>z_member_build_6105188c97623d:590`; TS `src/abap/2_statements/statement_parser.ts.StatementParser.buildSplits` (`src/abap/2_statements/statement_parser.ts:180:3`).

Before:
```abap
DATA(t41) = CONV abap_bool( abap_true ).
IF t41 = abap_true.
```

Inline the same explicitly typed CONV into the single use; remove its dead temp store. Do not erase CONV without a separate type proof. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
IF CONV abap_bool( abap_true ) = abap_true.
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: Explicit CONV and its ABAP conversion semantics only appear after emission.

### PM0004 — HIR

`Z_SRC_ABAP_1_L_22309FB1998E15=>z_member_proce_57f9d1e4b4e34e:1778`; TS `src/abap/1_lexer/lexer.ts.Lexer.process` (`src/abap/1_lexer/lexer.ts:237:3`).

Before:
```abap
t62 = t63->z_member_offse_4eb7e0915312eb.
t61 = t62.
```

Substitute the defining expression at its single use and delete the dead definition; keep any declaration needed for scope/type. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t61 = t63->z_member_offse_4eb7e0915312eb.
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals. Establish correspondence to a semantic HIR VarDecl/Assign binding before sending the rule to Grace; emitter-created scratch has no HIR binding and must be handled as ABAP-only. ABAP counts are not confirmed HIR rewrite counts.

Tag: Single-use copy propagation is expressible as HIR -> HIR in Grace and benefits TS-HG@Go. Binding provenance is a pending gate; emitter-only instances must be retagged ABAP-only.

### PM0005 — ABAP-only

`Z_SRC_ABAP_2_S_133BEB1C8EC314=>z_member_match_497af1c2c3e97f:1351`; TS `src/abap/2_statements/statement_parser.ts.StatementParser.match` (`src/abap/2_statements/statement_parser.ts:289:3`).

Before:
```abap
t44 = t43.
t42->oval = t44.
```

Forward the expression into the sole ABAP downcast/constructor/boxed-field use, retaining its exact conversion and evaluation order. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t42->oval = t43.
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: The consuming cast, conversion or boxed oval field is an ABAP emission artifact; this is emission materialization.

### PM0006 — HIR

`Z_SRC_ABAP_1_L_22309FB1998E15=>z_member_proce_57f9d1e4b4e34e:1702`; TS `src/abap/1_lexer/lexer.ts.Lexer.process` (`src/abap/1_lexer/lexer.ts:237:3`).

Before:
```abap
t28 = t29.
t23 = t28.
```

Substitute the defining expression at its single use and delete the dead definition; keep any declaration needed for scope/type. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t23 = t29.
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals. Establish correspondence to a semantic HIR VarDecl/Assign binding before sending the rule to Grace; emitter-created scratch has no HIR binding and must be handled as ABAP-only. ABAP counts are not confirmed HIR rewrite counts.

Tag: Single-use copy propagation is expressible as HIR -> HIR in Grace and benefits TS-HG@Go. Binding provenance is a pending gate; emitter-only instances must be retagged ABAP-only.

### PM0007 — ABAP-only

`Z_SRC_ABAP_2_S_133BEB1C8EC314=>z_member_match_497af1c2c3e97f:1435`; TS `src/abap/2_statements/statement_parser.ts.StatementParser.match` (`src/abap/2_statements/statement_parser.ts:289:3`).

Before:
```abap
DATA(t84) = CAST z_runtime_arra_c22708faf2e354( t11 ).
t80 = z_src_abap_2_s_ccda787b966b54=>z_member_recla_9cd6c9cb349aad( z_param_node_1324e81b8c7326 = t81 z_param_stmt_e3d6aed010b3f1 = t82 z_param_pragma_9ae3ad041053f6 = t84 ).
```

Inline CAST target( x ) into the single argument/receiver/value use; remove its dead temp store. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t80 = z_src_abap_2_s_ccda787b966b54=>z_member_recla_9cd6c9cb349aad( z_param_node_1324e81b8c7326 = t81 z_param_stmt_e3d6aed010b3f1 = t82 z_param_pragma_9ae3ad041053f6 = CAST z_runtime_arra_c22708faf2e354( t11 ) ).
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: This candidate targets the emitted ABAP CAST/receiver syntax, not a proven redundant HIR cast.

### PM0008 — HIR

`Z_SRC_ABAP_2_S_6D74A0F4B06FDB=>z_member_fn_na_02627bb26d218f:1712`; TS `src/abap/2_statements/expressions/sql_function.ts.SQLFunction.fn_namedFn` (`src/abap/2_statements/expressions/sql_function.ts:394:21`).

Before:
```abap
t60 = z_src_abap_2_s_2bcd0cea398bae=>z_member_seq_ebca191709e4e4( z_param_first_866ec5f6546ef7 = t61 z_param_second_6dbcfcd2e6b248 = t62 z_param_rest_2b9ae255a83323 = t63 ).
t24 = t60.
```

Substitute the defining expression at its single use and delete the dead definition; keep any declaration needed for scope/type. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t24 = z_src_abap_2_s_2bcd0cea398bae=>z_member_seq_ebca191709e4e4( z_param_first_866ec5f6546ef7 = t61 z_param_second_6dbcfcd2e6b248 = t62 z_param_rest_2b9ae255a83323 = t63 ).
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals. Establish correspondence to a semantic HIR VarDecl/Assign binding before sending the rule to Grace; emitter-created scratch has no HIR binding and must be handled as ABAP-only. ABAP counts are not confirmed HIR rewrite counts.

Tag: Single-use copy propagation is expressible as HIR -> HIR in Grace and benefits TS-HG@Go. Binding provenance is a pending gate; emitter-only instances must be retagged ABAP-only.

### PM0009 — ABAP-only

`Z_SRC_ABAP_2_S_133BEB1C8EC314=>z_member_proce_57f9d1e4b4e34e:1776`; TS `src/abap/2_statements/statement_parser.ts.StatementParser.process` (`src/abap/2_statements/statement_parser.ts:322:3`).

Before:
```abap
DATA(t124) = CAST z_src_abap_1_l_506101b34e5cc0( t119 ).
IF t124 IS INITIAL.
```

Inline CAST target( x ) into the single argument/receiver/value use; remove its dead temp store. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
IF CAST z_src_abap_1_l_506101b34e5cc0( t119 ) IS INITIAL.
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: This candidate targets the emitted ABAP CAST/receiver syntax, not a proven redundant HIR cast.

### PM0010 — ABAP-only

`Z_SRC_ABAP_2_S_133BEB1C8EC314=>z_member_lazyu_b62d9521cd020f:256`; TS `src/abap/2_statements/statement_parser.ts.StatementParser.lazyUnknown` (`src/abap/2_statements/statement_parser.ts:148:3`).

Before:
```abap
DATA(t10) = CAST z_src_abap_nod_e445d254902ced( t6 ).
t9 = t10->z_member_get_i_d31d9d82ef6654( ).
```

Inline CAST target( x ) into the single argument/receiver/value use; remove its dead temp store. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t9 = CAST z_src_abap_nod_e445d254902ced( t6 )->z_member_get_i_d31d9d82ef6654( ).
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: This candidate targets the emitted ABAP CAST/receiver syntax, not a proven redundant HIR cast.

### PM0011 — ABAP-only

`Z_SRC_ABAP_2_S_3983C0A0FEB031=>z_member_run_71be1bca84374a:147`; TS `src/abap/2_statements/combi.ts.StopBefore1.run` (`src/abap/2_statements/combi.ts:1245:3`).

Before:
```abap
DATA(t41) = CAST z_runtime_opti_4ba3bdccd169bb( t7 ).
t40 = t41->value.
```

Inline CAST target( x ) into the single argument/receiver/value use; remove its dead temp store. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t40 = CAST z_runtime_opti_4ba3bdccd169bb( t7 )->value.
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: This candidate targets the emitted ABAP CAST/receiver syntax, not a proven redundant HIR cast.

### PM0012 — ABAP-only

`Z_HARNESS_REGI_54F04D96E888D8=>z_member_repor_049386302ba6f9:412`; TS `harness/registry_run.ts.RegistryRun.report` (`harness/registry_run.ts:63:3`).

Before:
```abap
t31 = t30.
t29 ?= t31.
```

Forward the expression into the sole ABAP downcast/constructor/boxed-field use, retaining its exact conversion and evaluation order. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t29 ?= t30.
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: The consuming cast, conversion or boxed oval field is an ABAP emission artifact; this is emission materialization.

### PM0013 — ABAP-only

`Z_SRC_ABAP_2_S_133BEB1C8EC314=>z_member_match_497af1c2c3e97f:1371`; TS `src/abap/2_statements/statement_parser.ts.StatementParser.match` (`src/abap/2_statements/statement_parser.ts:289:3`).

Before:
```abap
DATA(t51) = CAST z_runtime_arra_c22708faf2e354( t36 ).
t50 = xsdbool( t51 IS BOUND ).
```

Inline CAST target( x ) into the single argument/receiver/value use; remove its dead temp store. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t50 = xsdbool( CAST z_runtime_arra_c22708faf2e354( t36 ) IS BOUND ).
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: This candidate targets the emitted ABAP CAST/receiver syntax, not a proven redundant HIR cast.

### PM0014 — HIR

`Z_SRC_ABAP_1_L_22309FB1998E15=>z_member_proce_57f9d1e4b4e34e:2619`; TS `src/abap/1_lexer/lexer.ts.Lexer.process` (`src/abap/1_lexer/lexer.ts:237:3`).

Before:
```abap
t408 = t409->z_member_count_aef1818d79c2bf( z_param_char_da5c30c615561e = t410 ).
t373 = t408.
```

Substitute the defining expression at its single use and delete the dead definition; keep any declaration needed for scope/type. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t373 = t409->z_member_count_aef1818d79c2bf( z_param_char_da5c30c615561e = t410 ).
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals. Establish correspondence to a semantic HIR VarDecl/Assign binding before sending the rule to Grace; emitter-created scratch has no HIR binding and must be handled as ABAP-only. ABAP counts are not confirmed HIR rewrite counts.

Tag: Single-use copy propagation is expressible as HIR -> HIR in Grace and benefits TS-HG@Go. Binding provenance is a pending gate; emitter-only instances must be retagged ABAP-only.

### PM0015 — ABAP-only

`Z_SRC_ABAP_2_S_133BEB1C8EC314=>z_member_match_497af1c2c3e97f:1372`; TS `src/abap/2_statements/statement_parser.ts.StatementParser.match` (`src/abap/2_statements/statement_parser.ts:289:3`).

Before:
```abap
t50 = xsdbool( t51 IS BOUND ).
IF t50 = abap_true.
```

Replace t = xsdbool( condition ); IF t = abap_true with IF condition. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
IF t51 IS BOUND.
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: ABAP xsdbool/abap_true forwarding is an emitted representation of a boolean.

### PM0016 — ABAP-only

`Z_SRC_ABAP_2_S_133BEB1C8EC314=>z_member_build_6105188c97623d:569`; TS `src/abap/2_statements/statement_parser.ts.StatementParser.buildSplits` (`src/abap/2_statements/statement_parser.ts:180:3`).

Before:
```abap
t27 ?= t30.
t26 = t27.
```

Replace declaration + t ?= x + single use with explicit CAST target( x ) at that use; retain a typed temporary when syntax requires it. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t26 = t30.
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: REF TO / ?= form is created by ABAP emission; runtime cast failure must stay at the same evaluated effect.

### PM0017 — ABAP-only

`Z_SRC_ABAP_2_S_133BEB1C8EC314=>z_member_proce_57f9d1e4b4e34e:1654`; TS `src/abap/2_statements/statement_parser.ts.StatementParser.process` (`src/abap/2_statements/statement_parser.ts:322:3`).

Before:
```abap
DATA(t52) = CONV string( |.| ).
t50 = xsdbool( t51 = t52 ).
```

Inline the same explicitly typed CONV into the single use; remove its dead temp store. Do not erase CONV without a separate type proof. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t50 = xsdbool( t51 = CONV string( |.| ) ).
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: Explicit CONV and its ABAP conversion semantics only appear after emission.

### PM0018 — ABAP-only

`Z_SRC_ABAP_2_S_133BEB1C8EC314=>z_member_match_497af1c2c3e97f:1408`; TS `src/abap/2_statements/statement_parser.ts.StatementParser.match` (`src/abap/2_statements/statement_parser.ts:289:3`).

Before:
```abap
t70 = t71.
DATA t72 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
DATA(t73) = CAST z_src_abap_nod_e445d254902ced( t70 ).
```

Forward the expression into the sole ABAP downcast/constructor/boxed-field use, retaining its exact conversion and evaluation order. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
DATA t72 TYPE REF TO z_src_abap_1_l_506101b34e5cc0.
DATA(t73) = CAST z_src_abap_nod_e445d254902ced( t71 ).
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: The consuming cast, conversion or boxed oval field is an ABAP emission artifact; this is emission materialization.

### PM0019 — HIR

`Z_SRC_ABAP_2_S_133BEB1C8EC314=>z_member_nativ_8e9b7af8387489:718`; TS `src/abap/2_statements/statement_parser.ts.StatementParser.nativeSQL` (`src/abap/2_statements/statement_parser.ts:196:3`).

Before:
```abap
t18 = t19->z_member_get_i_d31d9d82ef6654( ).
t17 = t18.
```

Substitute the defining expression at its single use and delete the dead definition; keep any declaration needed for scope/type. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
t17 = t19->z_member_get_i_d31d9d82ef6654( ).
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals. Establish correspondence to a semantic HIR VarDecl/Assign binding before sending the rule to Grace; emitter-created scratch has no HIR binding and must be handled as ABAP-only. ABAP counts are not confirmed HIR rewrite counts.

Tag: Single-use copy propagation is expressible as HIR -> HIR in Grace and benefits TS-HG@Go. Binding provenance is a pending gate; emitter-only instances must be retagged ABAP-only.

### PM0020 — ABAP-only

`Z_SRC_ABAP_2_S_133BEB1C8EC314=>z_member_proce_57f9d1e4b4e34e:1638`; TS `src/abap/2_statements/statement_parser.ts.StatementParser.process` (`src/abap/2_statements/statement_parser.ts:322:3`).

Before:
```abap
t41 = t42.
DATA t43 TYPE string.
DATA(t44) = CAST z_src_abap_1_l_506101b34e5cc0( t41 ).
```

Forward the expression into the sole ABAP downcast/constructor/boxed-field use, retaining its exact conversion and evaluation order. Saves 1 statement per successful execution.

Proposed substitution (ABAP syntax/type gate pending; receiver forms may need explicit target CAST):
```abap
DATA t43 TYPE string.
DATA(t44) = CAST z_src_abap_1_l_506101b34e5cc0( t42 ).
```

Soundness: Require one definition and one value use on all paths, including handlers; no intervening write to RHS dependencies or alias; no escaped local, closure capture, GET REFERENCE or ASSIGN/field-symbol access; preserve exact source/destination type and conversion; evaluate RHS exactly once at the next evaluated effect, preserving operand and exception order; same basic block and loop execution domain. Reject CATCH/CLEANUP observers and uncertain effects. Token counts alone are not proof. Preserve formal argument passing modes; never substitute into CHANGING/EXPORTING output slots or writable actuals.

Tag: The consuming cast, conversion or boxed oval field is an ABAP emission artifact; this is emission materialization.
