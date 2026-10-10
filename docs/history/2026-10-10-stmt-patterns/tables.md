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
