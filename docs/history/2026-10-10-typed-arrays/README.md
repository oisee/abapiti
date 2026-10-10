Typed-array storage analysis, 2026-10-10

Worth a targeted A4H experiment: start with `Result`, then `IStatementRunnable`, then `AbstractToken`. Two new array classes cover the profiled reference loops; a third also covers token reads introduced by today's inliner. Keep the shared object-array fallback. A blanket expansion to every nominal element type is hard to justify from this profile alone. This change is analysis only; no emitter or storage behavior changes.

The pinned production closure verified 1,538 TypeScript files plus RegistryRun, 1,927 classes and 73 interfaces, with zero blocking diagnostics or HIR verification errors. Current Singleton/Inline defaults were enabled. `go vet ./...` and `go test -short ./cmd/... ./hir/rewrite/...` passed.

The independent `abapiti abaplint -o $HOME/.cache/typed-arrays-abap --target a4h` run produced names.json and all seven saved ABAP evidence files byte for byte identical to the census. names.json SHA-256: `5f3fd4eaca759419fbae4d0c402619817db6eba5645a513d365e3e94d00a8fa3`.

Counts distinguish source lowering from actual current emission. The shared object-table subset is the direct answer for runtime.array of class/interface references. Optional reference rows are additionally inventoried because they already use typed tables; their get/pop/shift path can still insert a cast. Fields, parameters, return signatures, locals and expressions contribute to the element-type inventory, including types with zero allocation or read sites.

| HIR phase | Shared allocations | Shared row reads / casts | Optional-row allocations | Optional-row reads / casts | All allocations | All reads / casts |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| lowered | 486 | 808 / 808 | 7 | 26 / 6 | 493 | 834 / 814 |
| emitted | 521 | 847 / 847 | 7 | 26 / 6 | 528 | 873 / 853 |

| Storage scope | Distinct element types | Added classes | Estimated added ABAP lines |
| --- | ---: | ---: | ---: |
| All nominal element types | 134 | 133 | 31,122 |
| Read types in lexer/statements/structures/syntax/rules | 52 | 52 | 12,168 |
| Six profile #6 classes, including inlined reads | 3 | 3 | 702 |
| Their reference LOOP AT rows only | 2 | 2 | 468 |

The existing builtin.object array is one of the 134 types. Each estimate copies its measured 234-line class with its full runtime method surface and keeps the generic fallback. Bridge code, metadata, optional variants, compile/load cost and narrower runtime designs are excluded. These are source-size estimates, not timing predictions.

| First candidates (current emission) | Allocations | Reads / casts | Reads in six profile classes | Closed-world result |
| --- | ---: | ---: | ---: | --- |
| AbstractToken | 31 | 53 / 53 | 9 | Implementing classes / subtype hierarchy; no extra checked cast needed for assignment to the declared interface/base type. |
| Result | 46 | 36 / 36 | 13 | Resolved non-null rows are exactly Result; some allocations have no resolved rows. |
| IStatementRunnable | 33 | 33 / 33 | 6 | Implementing classes / subtype hierarchy; no extra checked cast needed for assignment to the declared interface/base type. |

All 28 reference-read sites in the six classes have one current read cast, matching a single element view in this conservative alias analysis. Ten current Result loops and three IStatementRunnable loops offer one removed cast per executed row. The original `run` methods account for eight Result and two runnable loops; the three additional loops are in current `run_one` clones. Nine inlined AbstractToken indexing sites offer one further read cast removed per execution. Result is the first experiment because the closed-world row proof is strongest and its loops occur throughout the parser combinators.

No profile hit counts, individual historical line numbers, cast-versus-loop CPU split or timings were supplied. The ranking prioritizes the supplied profile class IDs; it does not invent dynamic weights. Current ABAP positions below are verified against the CLI output. A performance verdict in seconds requires an A4H comparison with the same workload.

| Profile ID / class | Method | Current ABAP LOOP AT line | TS line | Array | Element | Current / potentially removed casts per row |
| --- | --- | ---: | --- | --- | --- | ---: |
| 97ADC9EF AlternativePriority | run | 81 | 973:5 | list | IStatementRunnable | 1 / 1 |
| 97ADC9EF AlternativePriority | run | 121 | 981:11 | temp | Result | 1 / 1 |
| 97ADC9EF AlternativePriority | first | 283 | 1018:5 | f2 | string | 0 / 0 |
| 4228F5CC Expression | run | 79 | 709:5 | r | Result | 1 / 1 |
| 4228F5CC Expression | run | 97 | 712:7 | temp | Result | 1 / 1 |
| 4228F5CC Expression | run_one | 289 | 712:7 | temp | Result | 1 / 1 |
| DA316F76 Regex | run | 59 | 29:5 | r | Result | 1 / 1 |
| B56EAE38 Sequence | run | 81 | 611:5 | r | Result | 1 / 1 |
| B56EAE38 Sequence | run | 107 | 614:7 | list | IStatementRunnable | 1 / 1 |
| B56EAE38 Sequence | run | 169 | 629:11 | temp | Result | 1 / 1 |
| B56EAE38 Sequence | run_one | 249 | 614:7 | list | IStatementRunnable | 1 / 1 |
| B56EAE38 Sequence | run_one | 311 | 629:11 | temp | Result | 1 / 1 |
| 53E10374 Token | run | 64 | 118:5 | r | Result | 1 / 1 |
| 63FEEC61 Word | run | 165 | 74:5 | r | Result | 1 / 1 |

Every row above links to exact class/method/site identities, LOOP AT text and following `?=` in [profile-6-loops.csv](profile-6-loops.csv). The string loop in AlternativePriority.first has zero row casts. Receiver-temporary `CAST` expressions elsewhere and explicit semantic Cast/Narrow checks are not counted as row extraction casts and are not promised removable.

The fixed point propagates allocation identities, row writes and aliases through lexical locals, fields merged by declaring owner, constructors, parameters, returns, compatible dispatch, index stores, push/unshift/splice and array copies. Unknown native producers or mutations remain unknown; missing row flows are never exact proofs. Context/field merging can reject opportunities. A hierarchy alone does not require the object-to-declared-view cast once the table has that declared base/interface row type. A narrower row view still needs a cast. Cross-element aliases require a representation design that preserves identity and mutations; the CSV single-view screen is not a complete rewrite-legality proof. Scope is the coverage-restricted lowered production program.

Current emitted caller stages (ForEach row reads include their own loop in depth; index writes are excluded):

| Caller stage / combi class | Allocations | In-loop reads | Outside-loop reads | Read casts | Casts with no cross-element view |
| --- | ---: | ---: | ---: | ---: | ---: |
| lexer | 1 | 0 | 0 | 0 | 0 |
| other | 187 | 235 | 8 | 242 | 185 |
| rules | 28 | 38 | 2 | 40 | 39 |
| statements | 92 | 55 | 10 | 65 | 61 |
| statements/combi.Alternative | 2 | 3 | 2 | 5 | 5 |
| statements/combi.AlternativePriority | 2 | 3 | 2 | 5 | 5 |
| statements/combi.Combi | 0 | 1 | 0 | 1 | 1 |
| statements/combi.Expression | 2 | 3 | 0 | 3 | 3 |
| statements/combi.FailCombinator | 1 | 0 | 0 | 0 | 0 |
| statements/combi.LangVers | 1 | 0 | 0 | 0 | 0 |
| statements/combi.LangVersNot | 3 | 0 | 0 | 0 | 0 |
| statements/combi.Optional | 2 | 4 | 1 | 5 | 5 |
| statements/combi.OptionalPriority | 2 | 6 | 1 | 7 | 7 |
| statements/combi.Permutation | 4 | 5 | 0 | 5 | 5 |
| statements/combi.Plus | 2 | 0 | 0 | 0 | 0 |
| statements/combi.PlusPriority | 2 | 0 | 0 | 0 | 0 |
| statements/combi.Regex | 2 | 2 | 0 | 2 | 2 |
| statements/combi.Sequence | 6 | 6 | 2 | 8 | 8 |
| statements/combi.Star | 3 | 2 | 0 | 2 | 2 |
| statements/combi.StarPriority | 2 | 1 | 0 | 1 | 1 |
| statements/combi.StopBefore1 | 2 | 2 | 0 | 2 | 2 |
| statements/combi.StopBefore2 | 2 | 3 | 0 | 3 | 3 |
| statements/combi.Token | 2 | 3 | 2 | 5 | 5 |
| statements/combi.Vers | 5 | 0 | 0 | 0 | 0 |
| statements/combi.Word | 2 | 3 | 2 | 5 | 5 |
| statements/combi.WordSequence | 2 | 0 | 1 | 1 | 1 |
| statements/combi.module | 8 | 4 | 0 | 4 | 4 |
| structures | 60 | 7 | 6 | 13 | 13 |
| syntax | 101 | 356 | 92 | 429 | 360 |

Every static element type, current emission (full identities remain in the CSV):

| Element | Allocations | Reads | Casts | Same-view potential | No cross-view | Profile-class reads | Shared allocations / reads | Proof categories |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- |
| builtin.object | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.02852a7f4246688b3f14697008cf69c06cf781cdc4ffd9ca59d21f90a03488b9 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.06a1d4f5c04f974480b81aea8a7f0d8f276c8fd0f4e203212758f1df69de797f | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.0adceb99073db09f4701195584c11e7787d425833ecefca44f213f7edbedaa85 | 1 | 0 | 0 | 0 | 0 | 0 | 1 / 0 | exact |
| shape.0c21b215a8ec99b06f4a3babffabfb13313afa4decaeb1efc6f5c5033cd9f923 | 1 | 0 | 0 | 0 | 0 | 0 | 1 / 0 | no non-null rows resolved (not an exact proof) |
| shape.0e2a55b560b2070de0d4f42adce6818b9548a0f0a33ea5ae139ebb198c63252a | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.0e610234a61c59728f9f67694a386e1b3a2f7dc5de0a2d3b1efe237fc1aa716c | 0 | 1 | 1 | 1 | 0 | 0 | 0 / 1 | unknown native/unresolved flow |
| shape.0eae1ba652600774df287924ca9c04933ca1d6434c241ccf77b12ad82fb82ca5 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.1151944adac9f18a8849f542a5c47c372b7f92aca79cd05aecd892bffd63ed2b | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.17f22d80434cb867441f2fccb92b8d8bf72640249ffbeb5f272c352c876cfe2f | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.26075853820ac3c29e636045eff99267436001784ab55247f897ecb68d257014 | 5 | 1 | 1 | 1 | 1 | 0 | 5 / 1 | no non-null rows resolved (not an exact proof);unknown native/unresolved flow |
| shape.473586f1dc44c79d419996faa2ead4c9e89cb998867ac438e617815967c3af7d | 1 | 1 | 1 | 1 | 1 | 0 | 1 / 1 | exact |
| shape.579d8e7c65abdbd91061ac54815a210af5a5eab8e42a49cb0785a38e014e6a12 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.596776dd783529cbd7aa471163cc95976664d520942f5d8f3a251de46285a5dd | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.668af6faebd97357d1640deac893f912b72bcd1cf26d888041b0bdfa60120d83 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.6787ec6bd565f704fe6c22a21fe4235b1ec166557fa7ab9551cc770f0c5a1c61 | 1 | 14 | 14 | 14 | 14 | 0 | 1 / 14 | exact |
| shape.6be70910bc78606319e3c0d3635b9fa2886522706a55e71f3817baf856ac3f60 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.741665cded50d564a824bb81b6869fb36630b092374ff1337a12e1fd68376bd2 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.7caff569122e6588e8b867a5e0823708654b34e9047b3ef84247ffdad970636a | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.7e8c4203d26cc50517941a28b5cbce37cdb0fb6abdee06ebaa72071cab7d32db | 5 | 4 | 4 | 4 | 4 | 0 | 5 / 4 | exact;no non-null rows resolved (not an exact proof);unknown native/unresolved flow |
| shape.98af2be8ec00ffea2ce4f8f894a6ccab6eeec900282ac0f45951da0218529bfd | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.9a2880cecfba4feed82205266048b869927e166885e968a525748e506407e01a | 1 | 0 | 0 | 0 | 0 | 0 | 1 / 0 | no non-null rows resolved (not an exact proof) |
| shape.a67a15173c1437619ae004d157dd6f567d1a423733059ab10e3d151b9d38c218 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.c0bf7470c0d65134bf2c2bc5006ff32978968e3b9f5cd00ad621be3facd4fa63 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.c41b7491225505bc1de20777f7e89571cd56cd4c4c6efa031f2f1afc14e9a1e0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.cc2b695581659623e924161c8df8ce8d129ee97f428a2bd0b6d8ebbe5beb9126 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.ccab811d0a1e0685eff885023de8dd53dc44941f932922e40ef82a276678e954 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.cdf309a87d061faba64a110dae8efc4199711cfab57b7b7f9b4c35ec5658d6e0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.cf26b458fc74140e407e7b8f31dd4a4da28eafde8af9d9d2f64c729d6d65a81c | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.d184d639b396286a74f614f4fc726acfc24441b4971c69640385ddf95d828f31 | 2 | 5 | 5 | 5 | 5 | 0 | 2 / 5 | exact |
| shape.d7262d2c4a49cced07fd6bf34dd385987b433e7bdc221b387876c69238930036 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.da3d528f34bcd63cd8dd8058576555174e7e961dfef89951a813f3513b086ed9 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.dd44bc035e4c4e168337ddd005068db21044dc68e68eaa08e83ca7f9852155c1 | 1 | 5 | 5 | 5 | 5 | 0 | 1 / 5 | exact |
| shape.e017bc2b8bf633c43e0328ef39a8431775f8fc68308abba251576386fa52f3b3 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.e35eec429dc1d9d6b5cf2d167d9211e2f25c45a515255fdd6b6668ef5138b6d5 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.ee3582a671520d5f971e43bff998c5f375f7c56beec50668926f0c6b4c5e0b98 | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| shape.fdbf434f7f7dbe66107f55ec17e8fb62eb662a8c5ce24298759590708e7d888b | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| IDependency | 1 | 0 | 0 | 0 | 0 | 0 | 1 / 0 | exact |
| IObjectAndToken | 13 | 2 | 2 | 2 | 2 | 0 | 13 / 2 | no non-null rows resolved (not an exact proof);unknown native/unresolved flow |
| IFilenameAndToken | 5 | 1 | 1 | 1 | 1 | 0 | 5 / 1 | no non-null rows resolved (not an exact proof);unknown native/unresolved flow |
| IABAPLexerResult | 1 | 1 | 1 | 1 | 1 | 0 | 1 / 1 | exact |
| AbstractToken | 31 | 53 | 53 | 53 | 53 | 9 | 31 / 53 | hierarchy/mixed upper bound;no non-null rows resolved (not an exact proof) |
| SQLNamedParam | 19 | 1 | 1 | 1 | 1 | 0 | 19 / 1 | exact |
| Result | 46 | 36 | 36 | 36 | 36 | 13 | 46 / 36 | exact;no non-null rows resolved (not an exact proof) |
| WorkArea | 1 | 3 | 3 | 3 | 3 | 0 | 1 / 3 | exact |
| IStatementResult | 1 | 1 | 1 | 1 | 1 | 0 | 1 / 1 | exact |
| InfoAlias | 2 | 3 | 3 | 3 | 3 | 0 | 2 / 3 | exact |
| InfoAttribute | 3 | 2 | 2 | 2 | 2 | 0 | 3 / 2 | exact |
| InfoClassDefinition | 3 | 3 | 3 | 3 | 3 | 0 | 3 / 3 | exact;no non-null rows resolved (not an exact proof) |
| InfoClassImplementation | 3 | 1 | 1 | 1 | 1 | 0 | 3 / 1 | exact;no non-null rows resolved (not an exact proof) |
| InfoConstant | 2 | 2 | 2 | 2 | 2 | 0 | 2 / 2 | exact |
| InfoFormDefinition | 3 | 0 | 0 | 0 | 0 | 0 | 3 / 0 | exact;no non-null rows resolved (not an exact proof) |
| InfoImplementing | 1 | 4 | 4 | 4 | 4 | 0 | 1 / 4 | exact |
| InfoInterfaceDefinition | 3 | 2 | 2 | 2 | 2 | 0 | 3 / 2 | exact;no non-null rows resolved (not an exact proof) |
| InfoMethodDefinition | 4 | 5 | 5 | 5 | 5 | 0 | 4 / 5 | exact;no non-null rows resolved (not an exact proof) |
| InfoMethodParameter | 1 | 0 | 0 | 0 | 0 | 0 | 1 / 0 | exact |
| Identifier | 1 | 3 | 3 | 3 | 3 | 0 | 1 / 3 | exact |
| IReference | 1 | 1 | 1 | 1 | 1 | 0 | 1 / 1 | exact |
| IScopeVariable | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| IListItemS | 2 | 1 | 1 | 1 | 1 | 0 | 2 / 1 | exact;no non-null rows resolved (not an exact proof) |
| IListItemT | 2 | 2 | 2 | 2 | 2 | 0 | 2 / 2 | exact;no non-null rows resolved (not an exact proof) |
| DatabaseSourceInfo | 2 | 3 | 3 | 3 | 3 | 0 | 2 / 3 | exact |
| SpaghettiScopeNode | 1 | 1 | 1 | 1 | 0 | 0 | 1 / 1 | exact |
| Parameter | 4 | 1 | 1 | 1 | 1 | 0 | 4 / 1 | exact |
| ABAPFile | 8 | 22 | 22 | 22 | 22 | 0 | 8 / 22 | exact;no non-null rows resolved (not an exact proof) |
| IKeyword | 1 | 0 | 0 | 0 | 0 | 0 | 1 / 0 | no non-null rows resolved (not an exact proof) |
| ExpressionNode | 31 | 244 | 244 | 244 | 243 | 0 | 31 / 244 | hierarchy/mixed upper bound;no non-null rows resolved (not an exact proof);unknown native/unresolved flow |
| StatementNode | 33 | 46 | 46 | 46 | 45 | 0 | 33 / 46 | exact;hierarchy/mixed upper bound;no non-null rows resolved (not an exact proof);unknown native/unresolved flow |
| StructureNode | 5 | 9 | 8 | 8 | 8 | 0 | 4 / 8 | hierarchy/mixed upper bound;no non-null rows resolved (not an exact proof) |
| TokenNode | 5 | 0 | 0 | 0 | 0 | 0 | 5 / 0 | exact |
| IImplementing | 4 | 17 | 17 | 17 | 17 | 0 | 4 / 17 | exact;no non-null rows resolved (not an exact proof) |
| TypeDefinitionsEntry | 1 | 5 | 5 | 5 | 5 | 0 | 1 / 5 | exact |
| TypedIdentifier | 24 | 43 | 43 | 43 | 39 | 0 | 24 / 43 | exact;hierarchy/mixed upper bound;no non-null rows resolved (not an exact proof);unknown native/unresolved flow |
| Alias | 7 | 9 | 9 | 9 | 9 | 0 | 7 / 9 | exact;no non-null rows resolved (not an exact proof) |
| AbstractType | 2 | 13 | 0 | 0 | 0 | 0 | 0 / 0 | hierarchy/mixed upper bound |
| IStructureComponent | 22 | 20 | 20 | 20 | 20 | 0 | 22 / 20 | exact;no non-null rows resolved (not an exact proof);unknown native/unresolved flow |
| ITableKey | 1 | 1 | 1 | 1 | 1 | 0 | 1 / 1 | exact |
| ClassAttribute | 3 | 6 | 6 | 6 | 3 | 0 | 3 / 6 | exact |
| ClassConstant | 1 | 4 | 4 | 4 | 0 | 0 | 1 / 4 | exact |
| FunctionModuleDefinition | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| IFunctionModuleParameter | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| Message | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| MethodImplementation | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| IRange | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| ITextEdit | 20 | 0 | 0 | 0 | 0 | 0 | 20 / 0 | exact;no non-null rows resolved (not an exact proof);unknown native/unresolved flow |
| MemoryFile | 2 | 0 | 0 | 0 | 0 | 0 | 2 / 0 | exact |
| Fix | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| Issue | 35 | 13 | 13 | 13 | 13 | 0 | 35 / 13 | exact;hierarchy/mixed upper bound;no non-null rows resolved (not an exact proof) |
| Token | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| ABAPObject | 2 | 3 | 3 | 3 | 3 | 0 | 2 / 3 | no non-null rows resolved (not an exact proof);unknown native/unresolved flow |
| ITranslationTextElements | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| AbstractObject | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| DynproField | 1 | 0 | 0 | 0 | 0 | 0 | 1 / 0 | exact |
| DynproHeader | 2 | 0 | 0 | 0 | 0 | 0 | 2 / 0 | exact;no non-null rows resolved (not an exact proof) |
| DomainValue | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| DomainValueTranslation | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| IBadiDefinition | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| Program | 1 | 1 | 1 | 1 | 1 | 0 | 1 / 1 | no non-null rows resolved (not an exact proof) |
| ProxyDataItem | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| SecondaryIndex | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| Position | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| ICandidate | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| IParameterData | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| Source | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| DomainClassMapping | 1 | 0 | 0 | 0 | 0 | 0 | 1 / 0 | no non-null rows resolved (not an exact proof) |
| Recommendations | 1 | 0 | 0 | 0 | 0 | 0 | 1 / 0 | exact |
| IMethod | 1 | 1 | 1 | 1 | 1 | 0 | 1 / 1 | exact |
| TokenAndKeyword | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| ICyclomaticComplexityResult | 2 | 0 | 0 | 0 | 0 | 0 | 2 / 0 | exact;no non-null rows resolved (not an exact proof) |
| IVertex | 1 | 2 | 2 | 2 | 2 | 0 | 1 / 2 | no non-null rows resolved (not an exact proof) |
| IMethodLengthResult | 6 | 0 | 0 | 0 | 0 | 0 | 6 / 0 | exact;no non-null rows resolved (not an exact proof) |
| ABAPRelease | 1 | 1 | 1 | 1 | 1 | 0 | 1 / 1 | exact |
| tuple.473287f8298dba7163a897908958f7c0eae733e25d2e027992ea2edc9bed2fa8.12886f9d00055adf24c40579e22d31b2b45f2023f892954ffd5567beb60825f8.8f9108e347ad289a33aa5df99962abce4e6d371906c6508e3a0fcfd0983f8809 | 3 | 3 | 3 | 3 | 3 | 0 | 3 / 3 | exact |
| tuple.473287f8298dba7163a897908958f7c0eae733e25d2e027992ea2edc9bed2fa8.8f9108e347ad289a33aa5df99962abce4e6d371906c6508e3a0fcfd0983f8809.98b7639e585fae5238ff43662490d7623581819bd94f3e3d769d74b4152b48ee | 1 | 1 | 1 | 1 | 1 | 0 | 1 / 1 | exact |
| tuple.473287f8298dba7163a897908958f7c0eae733e25d2e027992ea2edc9bed2fa8.8f9108e347ad289a33aa5df99962abce4e6d371906c6508e3a0fcfd0983f8809 | 1 | 1 | 1 | 1 | 1 | 0 | 1 / 1 | exact |
| tuple.473287f8298dba7163a897908958f7c0eae733e25d2e027992ea2edc9bed2fa8.9392c2913985a8d9dece285370a034dbfd0b9818f6378b05362544daf2088e35 | 1 | 2 | 2 | 2 | 2 | 0 | 1 / 2 | exact |
| closure.thunk | 5 | 1 | 1 | 1 | 1 | 0 | 5 / 1 | hierarchy/mixed upper bound |
| IStatementRunnable | 33 | 33 | 33 | 33 | 33 | 6 | 33 / 33 | hierarchy/mixed upper bound;no non-null rows resolved (not an exact proof) |
| IStatement | 1 | 1 | 1 | 1 | 1 | 0 | 1 / 1 | no non-null rows resolved (not an exact proof) |
| IStructureRunnable | 43 | 5 | 5 | 5 | 5 | 0 | 43 / 5 | hierarchy/mixed upper bound;no non-null rows resolved (not an exact proof);unknown native/unresolved flow |
| ISpaghettiScopeNode | 0 | 1 | 1 | 1 | 0 | 0 | 0 / 1 | hierarchy/mixed upper bound |
| INode | 5 | 33 | 33 | 33 | 6 | 0 | 5 / 33 | hierarchy/mixed upper bound;no non-null rows resolved (not an exact proof) |
| IClassDefinition | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| IEventDefinition | 4 | 7 | 7 | 7 | 7 | 0 | 4 / 7 | hierarchy/mixed upper bound;no non-null rows resolved (not an exact proof) |
| IFormDefinition | 3 | 3 | 3 | 3 | 3 | 0 | 3 / 3 | hierarchy/mixed upper bound;no non-null rows resolved (not an exact proof) |
| IInterfaceDefinition | 0 | 0 | 0 | 0 | 0 | 0 | 0 / 0 |  |
| IMethodDefinition | 3 | 8 | 8 | 8 | 8 | 0 | 3 / 8 | no non-null rows resolved (not an exact proof) |
| IFile | 5 | 11 | 11 | 11 | 9 | 0 | 5 / 11 | hierarchy/mixed upper bound;no non-null rows resolved (not an exact proof) |
| IObject | 3 | 17 | 17 | 17 | 17 | 0 | 3 / 17 | no non-null rows resolved (not an exact proof) |
| IRule | 2 | 10 | 10 | 10 | 10 | 0 | 2 / 10 | no non-null rows resolved (not an exact proof) |
| union.66719939903e43ae9586894a3271d3c1e0bc1a87886c0b01d3e32f95e19d70c3 | 11 | 85 | 85 | 85 | 12 | 0 | 11 / 85 | hierarchy/mixed upper bound;no non-null rows resolved (not an exact proof) |
| union.a4d0ccb535ead84cf0a3fba4afdbd09580e2c7b3f2e611d359e2cec125f49f13 | 4 | 12 | 6 | 6 | 6 | 0 | 0 / 0 | no non-null rows resolved (not an exact proof) |
| union.b0b6a58b84dc8bc589d0501b1bd3651c334e7360f1d90cb261317c96490bb097 | 2 | 3 | 3 | 3 | 3 | 0 | 2 / 3 | no non-null rows resolved (not an exact proof) |
| union.ef34b1fae8df131c6c97185482820bb32cd957a6b0ed51ce6b23465fc63eb844 | 0 | 13 | 13 | 13 | 0 | 0 | 0 / 13 | hierarchy/mixed upper bound |

Top 20 current read sites: profile class first, potential saving, loop depth, then stable HIR path. Ties are not performance differences.

| Rank | Caller | TS location | Operation / array | Element | Depth | Casts / potential removals |
| ---: | --- | --- | --- | --- | ---: | ---: |
| 1 | AlternativePriority::run | src/abap/2_statements/combi.ts:981:11 | foreach | Result | 2 | 1 / 1 |
| 2 | Expression::run | src/abap/2_statements/combi.ts:712:7 | foreach | Result | 2 | 1 / 1 |
| 3 | Sequence::run | src/abap/2_statements/combi.ts:614:7 | foreach | IStatementRunnable | 2 | 1 / 1 |
| 4 | Sequence::run | src/abap/2_statements/combi.ts:629:11 | foreach | Result | 2 | 1 / 1 |
| 5 | AlternativePriority::run | src/abap/2_statements/combi.ts:973:5 | foreach | IStatementRunnable | 1 | 1 / 1 |
| 6 | AlternativePriority::run | src/abap/2_statements/combi.ts:979:23 | index | Result | 1 | 1 / 1 |
| 7 | Expression::run | src/abap/2_statements/combi.ts:709:5 | foreach | Result | 1 | 1 / 1 |
| 8 | Expression::run_one | src/abap/2_statements/combi.ts:712:7 | foreach | Result | 1 | 1 / 1 |
| 9 | Regex::run | src/abap/2_statements/combi.ts:29:5 | foreach | Result | 1 | 1 / 1 |
| 10 | Regex::run | src/abap/2_statements/combi.ts:33:21 inlined src/abap/2_statements/result.ts.Result.peek | index | AbstractToken | 1 | 1 / 1 |
| 11 | Sequence::run | src/abap/2_statements/combi.ts:611:5 | foreach | Result | 1 | 1 / 1 |
| 12 | Sequence::run | src/abap/2_statements/combi.ts:627:23 | index | Result | 1 | 1 / 1 |
| 13 | Sequence::run_one | src/abap/2_statements/combi.ts:614:7 | foreach | IStatementRunnable | 1 | 1 / 1 |
| 14 | Sequence::run_one | src/abap/2_statements/combi.ts:629:11 | foreach | Result | 1 | 1 / 1 |
| 15 | Token::run | src/abap/2_statements/combi.ts:118:5 | foreach | Result | 1 | 1 / 1 |
| 16 | Token::run | src/abap/2_statements/combi.ts:121:47 inlined src/abap/2_statements/result.ts.Result.peek | index | AbstractToken | 1 | 1 / 1 |
| 17 | Token::run | src/abap/2_statements/combi.ts:120:14 inlined src/abap/2_statements/result.ts.Result.peek | index | AbstractToken | 1 | 1 / 1 |
| 18 | Word::run | src/abap/2_statements/combi.ts:74:5 | foreach | Result | 1 | 1 / 1 |
| 19 | Word::run | src/abap/2_statements/combi.ts:78:47 inlined src/abap/2_statements/result.ts.Result.peek | index | AbstractToken | 1 | 1 / 1 |
| 20 | Word::run | src/abap/2_statements/combi.ts:76:14 inlined src/abap/2_statements/result.ts.Result.peek | index | AbstractToken | 1 | 1 / 1 |

Reproduce (all writable temporary work stays under the clone or $HOME/.cache):

```sh
mkdir -p "$HOME/.cache/abapiti-tmp"
GOCACHE="$HOME/.cache/abapiti-go-cache" GOFLAGS=-buildvcs=false \
GOTMPDIR="$HOME/.cache/abapiti-tmp" TMPDIR="$HOME/.cache/abapiti-tmp" \
go run ./cmd/grace-counts -typed-arrays \
  -output docs/history/2026-10-10-typed-arrays
```

Full raw tables and both top-20 lists: [typed-array-tables.txt](typed-array-tables.txt). Per-site data: [lowered](typed-array-lowered-sites.csv), [emitted](typed-array-emitted-sites.csv). Per-element tables: [lowered](typed-array-lowered-types.csv), [emitted](typed-array-emitted-types.csv). Naming evidence: [names.json](names.json). The [command README](../../../cmd/grace-counts/README.md) documents contracts and limitations.
