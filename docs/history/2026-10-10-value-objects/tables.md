# Per-class value-object screen

All proven-removal scores tie at zero. Order uses conditional parser-hot sites as the tie breaker. U = unproven, F = witnessed failure, P = pass, A = absent flag required. Full types, source sites and class identities are in candidates.csv.

| Order | Class | C1 | C2 | C3 | C4 | C5 | Bytes / refs | New / in-loop | Conditional hot | Proven removed | First witnessed blocker (or proof barrier) |
| ---: | --- | :---: | :---: | :---: | :---: | :---: | ---: | ---: | ---: | ---: | --- |
| 1 | tuple.473287f8298d | U | U | U | A | P | 32 / 2 | 218 / 0 | 218 | 0 | src/config.ts:11:3 potential transitive/alias write (Grace site_writes) |
| 2 | combi.IVerOptions | U | U | U | A | P | 24 / 1 | 134 / 0 | 134 | 0 | src/config.ts:11:3 potential transitive/alias write (Grace site_writes) |
| 3 | SQLNamedParam | U | U | U | A | P | 41 / 2 | 62 / 0 | 62 | 0 | src/config.ts:11:3 potential transitive/alias write (Grace site_writes) |
| 4 | StatementNode | F | F | U | A | P | 40 / 4 | 19 / 11 | 19 | 0 | src/abap/nodes/_abstract_node.ts:20:7 field write outside own constructor: .children |
| 5 | tuple.473287f8298d | U | U | U | A | P | 32 / 3 | 19 / 0 | 19 | 0 | src/config.ts:11:3 potential transitive/alias write (Grace site_writes) |
| 6 | IMatch | F | U | U | A | P | 41 / 3 | 11 / 4 | 11 | 0 | src/abap/3_structures/structures/_combi.ts:183:5 field write outside own constructor: .error |
| 7 | builtin.Error | U | F | F | A | P | 16 / 1 | 79 / 9 | 7 | 0 | src/abap/2_statements/combi.ts:435:11 instanceof/class test through classref<builtin.Error> |
| 8 | TokenNode | U | F | F | A | P | 8 / 1 | 7 / 7 | 7 | 0 | src/abap/2_statements/_select_reclassify.ts:15:11 instanceof/class test through interfaceref<union.66719939903e43ae9586894a3271d3c1e0bc1a87886c0b01d3e32f95e19d70c3> |
| 9 | combi.Sequence (profile #5) | U | F | U | A | P | 8 / 1 | 4 / 0 | 4 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 10 | SQLJoinSource | F | F | U | A | P | 16 / 1 | 3 / 0 | 3 | 0 | src/abap/2_statements/combi.ts:706:7 field write outside own constructor: .runnable |
| 11 | SQLPathSegment | F | F | U | A | P | 17 / 1 | 3 / 0 | 3 | 0 | src/abap/2_statements/combi.ts:706:7 field write outside own constructor: .runnable |
| 12 | Empty | U | F | U | A | P | 0 / 0 | 3 / 0 | 3 | 0 | src/abap/2_statements/_select_reclassify.ts:111:7 instanceof/class test through interfaceref<src/abap/2_statements/statements/_statement.ts.IStatement> |
| 13 | Unknown | U | F | U | A | P | 0 / 0 | 3 / 2 | 3 | 0 | src/abap/2_statements/_select_reclassify.ts:111:7 instanceof/class test through interfaceref<src/abap/2_statements/statements/_statement.ts.IStatement> |
| 14 | IStructureResult | U | U | U | A | P | 24 / 2 | 3 / 0 | 3 | 0 | src/config.ts:11:3 potential transitive/alias write (Grace site_writes) |
| 15 | Position | U | F | F | A | P | 16 / 0 | 31 / 5 | 3 | 0 | src/issue.ts:142:9 instanceof/class test through classref<src/position.ts.Position> |
| 16 | shape.1349f3595317f64ed80ed692f1b3f40aeac7457cf750e87b74fd43117396fde9 | U | U | U | A | P | 16 / 2 | 2 / 0 | 2 | 0 | src/config.ts:11:3 potential transitive/alias write (Grace site_writes) |
| 17 | shape.7e8c4203d26cc50517941a28b5cbce37cdb0fb6abdee06ebaa72071cab7d32db | F | U | U | A | P | 24 / 2 | 2 / 2 | 2 | 0 | src/abap/2_statements/statement_parser.ts:52:9 field write outside own constructor: .matcher |
| 18 | IFilenameAndToken | U | U | U | A | P | 24 / 2 | 3 / 2 | 2 | 0 | src/config.ts:11:3 potential transitive/alias write (Grace site_writes) |
| 19 | token.Identifier | U | F | U | A | P | 40 / 3 | 32 / 6 | 2 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 20 | combi.Permutation (profile #5) | U | F | U | A | P | 8 / 1 | 2 / 1 | 2 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 21 | combi.Star (profile #5) | U | F | U | A | P | 8 / 1 | 2 / 0 | 2 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 22 | combi.StarPriority | U | F | U | A | P | 8 / 1 | 2 / 0 | 2 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 23 | combi.Word (profile #5) | U | F | U | A | P | 16 / 1 | 2 / 1 | 2 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 24 | Result | F | U | U | A | P | 36 / 2 | 2 / 0 | 2 | 0 | src/abap/2_statements/result.ts:30:5 field write outside own constructor: .nodes |
| 25 | token.AssociationName | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 26 | token.At | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 27 | token.AtW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 28 | token.BracketLeft | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 29 | token.BracketLeftW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 30 | token.BracketRight | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 31 | token.BracketRightW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 32 | token.Comment | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 33 | token.Dash | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 34 | token.DashW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 35 | token.InstanceArrow | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 36 | token.InstanceArrowW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 37 | token.ParenLeft | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 38 | token.ParenLeftW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 39 | token.ParenRight | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 40 | token.ParenRightW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 41 | token.Plus | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 42 | token.PlusW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 43 | token.Pragma | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 44 | token.Punctuation | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 45 | token.StaticArrow | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 46 | token.StaticArrowW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 47 | token.StringToken | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 48 | token.StringTemplate | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 49 | token.StringTemplateBegin | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 50 | token.StringTemplateEnd | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 51 | token.StringTemplateMiddle | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 52 | token.WAt | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 53 | token.WAtW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 54 | token.WBracketLeft | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 55 | token.WBracketLeftW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 56 | token.WBracketRight | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 57 | token.WBracketRightW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 58 | token.WDash | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 59 | token.WDashW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 60 | token.WInstanceArrow | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 61 | token.WInstanceArrowW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 62 | token.WParenLeft | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 63 | token.WParenLeftW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 64 | token.WParenRight | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 65 | token.WParenRightW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 66 | token.WPlus | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 67 | token.WPlusW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 68 | token.WStaticArrow | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 69 | token.WStaticArrowW | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/expand_macros.ts:252:18 instanceof/class test through classref<src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken> |
| 70 | combi.Alternative (profile #5) | U | F | U | A | P | 8 / 1 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 71 | combi.AlternativePriority | U | F | U | A | P | 8 / 1 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 72 | combi.FailCombinator | U | F | U | A | P | 0 / 0 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 73 | combi.LangVers | U | F | U | A | P | 24 / 2 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 74 | combi.LangVersNot | U | F | U | A | P | 24 / 2 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 75 | combi.Optional (profile #5) | U | F | U | A | P | 8 / 1 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 76 | combi.OptionalPriority | U | F | U | A | P | 8 / 1 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 77 | combi.Plus (profile #5) | U | F | U | A | P | 16 / 2 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 78 | combi.PlusPriority | U | F | U | A | P | 16 / 2 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 79 | combi.Regex | U | F | U | A | P | 8 / 1 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 80 | combi.StopBefore1 | U | F | U | A | P | 8 / 1 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 81 | combi.StopBefore2 | U | F | U | A | P | 32 / 2 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 82 | combi.Token (profile #5) | U | F | U | A | P | 24 / 2 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 83 | combi.Vers (profile #5) | U | F | U | A | P | 40 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 84 | combi.WordSequence (profile #5) | U | F | U | A | P | 32 / 3 | 1 / 0 | 1 | 0 | src/abap/2_statements/_select_reclassify.ts:18:9 instanceof/class test through interfaceref<src/abap/2_statements/statement_runnable.ts.IStatementRunnable> |
| 85 | ExpressionNode | F | F | U | A | P | 24 / 2 | 1 / 1 | 1 | 0 | src/abap/nodes/_abstract_node.ts:20:7 field write outside own constructor: .children |
| 86 | VirtualPosition | U | F | U | A | P | 32 / 0 | 1 / 0 | 1 | 0 | src/issue.ts:142:9 instanceof/class test through classref<src/position.ts.Position> |
| 87 | token.AbstractToken | U | F | F | A | P | 40 / 3 | 0 / 0 | 0 | 0 | src/abap/2_statements/combi.ts:120:14 dynamic class operation: object.classOf |
| 88 | combi.Combi | F | U | U | A | P | 0 / 0 | 0 / 0 | 0 | 0 | src/abap/2_statements/combi.ts:1057:5 static field write outside constructor/initializer: src/abap/2_statements/combi.ts.Combi.langVer |
| 89 | combi.Expression (profile #5) | F | F | F | A | P | 16 / 1 | 0 / 0 | 0 | 0 | src/abap/2_statements/combi.ts:706:7 field write outside own constructor: .runnable |
| 90 | combi.FailCombinatorError | U | F | U | A | P | 16 / 1 | 0 / 0 | 0 | 0 | src/abap/2_statements/combi.ts:435:11 instanceof/class test through classref<builtin.Error> |
| 91 | combi.FailStarError | U | F | U | A | P | 16 / 1 | 0 / 0 | 0 | 0 | src/abap/2_statements/combi.ts:435:11 instanceof/class test through classref<builtin.Error> |
| 92 | Identifier | U | F | F | A | P | 24 / 2 | 17 / 15 | 0 | 0 | src/abap/5_syntax/_type_utils.ts:154:11 instanceof/class test through classref<src/abap/4_file_information/_identifier.ts.Identifier> |
