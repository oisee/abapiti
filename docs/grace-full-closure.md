# Grace on the pinned abaplint closure

The full test uses all 1,538 pinned source files plus `RegistryRun`, embedded
reachability, fingerprinted registry overrides, and assume-int, matching the
production `abapiti abaplint` lowering inputs. It verifies every source hash.
The result has 1,927 classes, 73 interfaces, no blocking diagnostics and no HIR
verification errors. No `hir/golang` dependency is used.

Run with:

```sh
flock /tmp/abapiti-heavy.lock go test ./tsfront -run '^TestGraceFullRegistryClosure$' -count=1 -v -timeout=60m
```

`-short` skips this slow test. `GRACE_FULL_FACTS_OUT=/path/report.txt` exports
all facts, method sets, virtual receiver sets and direct static writes.

## Facts

Counts describe the reachability-pruned, overridden HIR before inlining.
Purity and possible throws are separate properties and can overlap.
Static writes count unique `(method, class, field)` tuples, grouped by the
class owning the static field. The memo/counter/other categories are the
current Grace rule classifications.

```text
  methods pure=3498 may_throw=3767 defined=6257
  virtual sites single=3902 total=5062
  static writes memo=0 counter=0 other=166
  node_modules/vscode-languageserver-types/lib/umd/main.d.ts.module memo=0 counter=0 other=1
  src/abap/1_lexer/lexer.ts.module memo=0 counter=0 other=29
  src/abap/1_lexer/lexer_stream.ts.module memo=0 counter=0 other=2
  src/abap/1_lexer/tokens/index.ts.module memo=0 counter=0 other=1
  src/abap/2_statements/combi.ts.Combi memo=0 counter=0 other=4
  src/abap/2_statements/combi.ts.module memo=0 counter=0 other=5
  src/abap/2_statements/expressions/_dynamic_access.ts.module memo=0 counter=0 other=3
  src/abap/2_statements/expressions/index.ts.module memo=0 counter=0 other=1
  src/abap/2_statements/expressions/sql_aggregation.ts.module memo=0 counter=0 other=2
  src/abap/2_statements/expressions/sql_function.ts.module memo=0 counter=0 other=5
  src/abap/2_statements/expressions/sql_over.ts.module memo=0 counter=0 other=2
  src/abap/2_statements/expressions/sql_typed_literal.ts.module memo=0 counter=0 other=1
  src/abap/2_statements/statement_parser.ts.StatementParser memo=0 counter=0 other=2
  src/abap/2_statements/statement_parser.ts.module memo=0 counter=0 other=2
  src/abap/2_statements/statements/index.ts.module memo=0 counter=0 other=1
  src/abap/3_structures/structure_parser.ts.StructureParser memo=0 counter=0 other=2
  src/abap/3_structures/structures/_combi.ts.module memo=0 counter=0 other=2
  src/abap/3_structures/structures/index.ts.module memo=0 counter=0 other=1
  src/abap/4_file_information/_abap_file_information.ts.module memo=0 counter=0 other=5
  src/abap/4_file_information/visibility.ts.module memo=0 counter=0 other=1
  src/abap/5_syntax/_builtin.ts.BuiltIn memo=0 counter=0 other=11
  src/abap/5_syntax/_reference.ts.module memo=0 counter=0 other=1
  src/abap/5_syntax/_scope_type.ts.module memo=0 counter=0 other=1
  src/abap/5_syntax/_syntax_input.ts.module memo=0 counter=0 other=1
  src/abap/5_syntax/expressions/select.ts.module memo=0 counter=0 other=1
  src/abap/5_syntax/expressions/sql_in.ts.module memo=0 counter=0 other=1
  src/abap/5_syntax/syntax.ts.module memo=0 counter=0 other=2
  src/abap/nodes/_abstract_node.ts.module memo=0 counter=0 other=1
  src/abap/nodes/index.ts.module memo=0 counter=0 other=1
  src/abap/nodes/statement_node.ts.module memo=0 counter=0 other=1
  src/abap/nodes/token_node.ts.module memo=0 counter=0 other=1
  src/abap/types/_typed_identifier.ts.module memo=0 counter=0 other=1
  src/abap/types/basic/any_type.ts.AnyType memo=0 counter=0 other=1
  src/abap/types/basic/cgeneric_type.ts.CGenericType memo=0 counter=0 other=1
  src/abap/types/basic/clike_type.ts.CLikeType memo=0 counter=0 other=1
  src/abap/types/basic/index.ts.module memo=0 counter=0 other=1
  src/abap/types/basic/integer_type.ts.IntegerType memo=0 counter=0 other=1
  src/abap/types/basic/pgeneric_type.ts.PGenericType memo=0 counter=0 other=1
  src/abap/types/basic/simple_type.ts.SimpleType memo=0 counter=0 other=1
  src/abap/types/basic/string_type.ts.StringType memo=0 counter=0 other=1
  src/abap/types/basic/table_type.ts.module memo=0 counter=0 other=2
  src/abap/types/basic/void_type.ts.VoidType memo=0 counter=0 other=1
  src/abap/types/basic/xgeneric_type.ts.XGenericType memo=0 counter=0 other=1
  src/abap/types/basic/xstring_type.ts.XStringType memo=0 counter=0 other=1
  src/abap/types/function_module_definition.ts.module memo=0 counter=0 other=2
  src/abap/types/index.ts.module memo=0 counter=0 other=1
  src/cds/cds_lexer.ts.module memo=0 counter=0 other=1
  src/cds/expressions/cds_name.ts.module memo=0 counter=0 other=1
  src/cds/expressions/cds_relation.ts.module memo=0 counter=0 other=1
  src/cds/expressions/index.ts.module memo=0 counter=0 other=1
  src/lsp/_interfaces.ts.module memo=0 counter=0 other=1
  src/lsp/rename.ts.module memo=0 counter=0 other=1
  src/lsp/semantic.ts.SemanticHighlighting memo=0 counter=0 other=1
  src/lsp/semantic.ts.module memo=0 counter=0 other=2
  src/objects/class.ts.module memo=0 counter=0 other=1
  src/objects/index.ts.module memo=0 counter=0 other=1
  src/objects/table.ts.module memo=0 counter=0 other=2
  src/registry.ts.ParsingPerformance memo=0 counter=0 other=9
  src/rules/_irule.ts.module memo=0 counter=0 other=1
  src/rules/allowed_object_naming.ts.module memo=0 counter=0 other=1
  src/rules/cds_check_syntax.ts.module memo=0 counter=0 other=5
  src/rules/check_abstract.ts.module memo=0 counter=0 other=1
  src/rules/check_comments.ts.module memo=0 counter=0 other=1
  src/rules/definitions_top.ts.module memo=0 counter=0 other=3
  src/rules/index.ts.module memo=0 counter=0 other=1
  src/rules/keyword_case.ts.module memo=0 counter=0 other=1
  src/rules/method_length.ts.module memo=0 counter=0 other=1
  src/rules/newline_between_methods.ts.module memo=0 counter=0 other=1
  src/rules/no_prefixes.ts.module memo=0 counter=0 other=1
  src/rules/prefer_abap_bool.ts.module memo=0 counter=0 other=1
  src/rules/select_performance.ts.module memo=0 counter=0 other=1
  src/rules/sy_read_restriction.ts.module memo=0 counter=0 other=2
  src/severity.ts.module memo=0 counter=0 other=1
  src/stuff.ts.module memo=0 counter=0 other=2
  src/utils/include_graph.ts.module memo=0 counter=0 other=1
  src/version.ts.module memo=0 counter=0 other=8
```

## Inline oracle

Grace and main's real `hir.InlineStats` produce byte-identical HIR dumps and
identical per-callee counters: **1,487 call sites across 190 callees**. Both
outputs pass `hir.Verify`. `internal/inlineoracle` now contains only the
comparison harness; it deep-copies the program twice before either mutating
inliner runs. The old pinned implementation is removed. Comparison runtime:
28.59 seconds. Main includes the 70394ff guard; Grace also rejects callees with
any VarDecl lacking an initializer, including declarations inside Seq. The
synthetic TypeScript loop executes in Node with result `[5,-1]` and stays a
call under both inliners.

```text
src/abap/1_lexer/lexer.ts.Lexer.run 1
src/abap/1_lexer/lexer_buffer.ts.LexerBuffer.add 1
src/abap/1_lexer/lexer_buffer.ts.LexerBuffer.clear 1
src/abap/1_lexer/lexer_buffer.ts.LexerBuffer.get 2
src/abap/1_lexer/lexer_buffer.ts.LexerBuffer.length 4
src/abap/1_lexer/lexer_stream.ts.LexerStream.advance 1
src/abap/1_lexer/lexer_stream.ts.LexerStream.charCodeAt 2
src/abap/1_lexer/lexer_stream.ts.LexerStream.currentChar 2
src/abap/1_lexer/lexer_stream.ts.LexerStream.getCol 1
src/abap/1_lexer/lexer_stream.ts.LexerStream.getOffset 3
src/abap/1_lexer/lexer_stream.ts.LexerStream.getRaw 1
src/abap/1_lexer/lexer_stream.ts.LexerStream.getRow 1
src/abap/1_lexer/lexer_stream.ts.LexerStream.nextChar 3
src/abap/1_lexer/lexer_stream.ts.LexerStream.nextNextChar 1
src/abap/1_lexer/lexer_stream.ts.LexerStream.prevChar 4
src/abap/1_lexer/lexer_stream.ts.LexerStream.prevPrevChar 1
src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken.getCol 4
src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken.getEnd 30
src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken.getRow 11
src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken.getStart 67
src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken.getStr 163
src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken.getUpperStr 4
src/abap/2_statements/expand_macros.ts.Macros.isMacro 1
src/abap/2_statements/expressions/sql_function.ts.SQLFunction.fn_valueFor 1
src/abap/2_statements/result.ts.Result.peek 7
src/abap/2_statements/result.ts.Result.peekAt 1
src/abap/2_statements/result.ts.Result.remainingLength 10
src/abap/2_statements/result.ts.Result.shift 3
src/abap/2_statements/statement_parser.ts.WorkArea.addUnknown 3
src/abap/2_statements/statement_parser.ts.WorkArea.toResult 1
src/abap/3_structures/structures/_combi.ts.SubStatement.className 2
src/abap/3_structures/structures/_combi.ts.SubStructure.setupMatcher 2
src/abap/4_file_information/_identifier.ts.Identifier.getEnd 1
src/abap/4_file_information/_identifier.ts.Identifier.getFilename 7
src/abap/4_file_information/_identifier.ts.Identifier.getName 83
src/abap/4_file_information/_identifier.ts.Identifier.getStart 3
src/abap/4_file_information/_identifier.ts.Identifier.getToken 24
src/abap/4_file_information/abap_file_information.ts.ABAPFileInformation.listClassDefinitions 1
src/abap/4_file_information/abap_file_information.ts.ABAPFileInformation.listClassImplementations 1
src/abap/4_file_information/abap_file_information.ts.ABAPFileInformation.listInterfaceDefinitions 1
src/abap/4_file_information/abap_file_information_parser.ts.ABAPFileInformationParser.parse 1
src/abap/5_syntax/_builtin.ts.BuiltIn.buildVariable 1
src/abap/5_syntax/_builtin.ts.BuiltIn.getTypes 1
src/abap/5_syntax/_current_scope.ts.CurrentScope.addDeferred 2
src/abap/5_syntax/_current_scope.ts.CurrentScope.addExtraLikeType 1
src/abap/5_syntax/_current_scope.ts.CurrentScope.addIdentifier 42
src/abap/5_syntax/_current_scope.ts.CurrentScope.addType 12
src/abap/5_syntax/_current_scope.ts.CurrentScope.findClassDefinition 27
src/abap/5_syntax/_current_scope.ts.CurrentScope.findFormDefinition 2
src/abap/5_syntax/_current_scope.ts.CurrentScope.findInterfaceDefinition 26
src/abap/5_syntax/_current_scope.ts.CurrentScope.findObjectDefinition 26
src/abap/5_syntax/_current_scope.ts.CurrentScope.findType 5
src/abap/5_syntax/_current_scope.ts.CurrentScope.findVariable 23
src/abap/5_syntax/_current_scope.ts.CurrentScope.getDDIC 36
src/abap/5_syntax/_current_scope.ts.CurrentScope.getDDICReferences 8
src/abap/5_syntax/_current_scope.ts.CurrentScope.getLanguageVersion 8
src/abap/5_syntax/_current_scope.ts.CurrentScope.getMSAGReferences 3
src/abap/5_syntax/_current_scope.ts.CurrentScope.getOpenABAP 4
src/abap/5_syntax/_current_scope.ts.CurrentScope.getParentObj 8
src/abap/5_syntax/_current_scope.ts.CurrentScope.getRegistry 11
src/abap/5_syntax/_current_scope.ts.CurrentScope.getRelease 5
src/abap/5_syntax/_current_scope.ts.CurrentScope.isAllowHeaderUse 1
src/abap/5_syntax/_current_scope.ts.CurrentScope.isLocalFriend 1
src/abap/5_syntax/_current_scope.ts.CurrentScope.isTypePool 1
src/abap/5_syntax/_current_scope.ts.CurrentScope.push 17
src/abap/5_syntax/_current_scope.ts.CurrentScope.setAllowHeaderUse 1
src/abap/5_syntax/_object_oriented.ts.ObjectOriented.findMethodInInterface 2
src/abap/5_syntax/_object_oriented.ts.ObjectOriented.fn_isFriend 2
src/abap/5_syntax/_object_oriented.ts.ObjectOriented.fromSuperClassesAndInterfaces 2
src/abap/5_syntax/_object_oriented.ts.ObjectOriented.methodReferenceExtras 2
src/abap/5_syntax/_type_utils.ts.TypeUtils.isAssignableNew 1
src/abap/5_syntax/_type_utils.ts.TypeUtils.isCastable 2
src/abap/5_syntax/_type_utils.ts.TypeUtils.isCharLikeField 2
src/abap/5_syntax/_type_utils.ts.TypeUtils.isCharLikeForCompare 1
src/abap/5_syntax/basic_types.ts.BasicTypes.cloneType 3
src/abap/5_syntax/basic_types.ts.BasicTypes.isOccurs 4
src/abap/5_syntax/basic_types.ts.BasicTypes.isRAPTypeStructure 1
src/abap/5_syntax/spaghetti_scope.ts.ScopeData.getData 23
src/abap/5_syntax/spaghetti_scope.ts.SpaghettiScopeNode.getIdentifier 9
src/abap/5_syntax/spaghetti_scope.ts.SpaghettiScopeNode.setEnd 1
src/abap/5_syntax/statements/constant.ts.Constant.isNameTooLong 2
src/abap/5_syntax/statements/constant.ts.Constant.isOnlyDigits 2
src/abap/5_syntax/statements/data.ts.Data.isNameTooLong 2
src/abap/5_syntax/statements/data.ts.Data.isOnlyDigits 2
src/abap/5_syntax/statements/find.ts.Find.inline 6
src/abap/5_syntax/statements/get_time.ts.GetTime.compatibleTimeStamp 1
src/abap/5_syntax/syntax.ts.SyntaxLogic.isSelectionEventBoundaryStructure 1
src/abap/5_syntax/syntax.ts.SyntaxLogic.opensSelectionEventScope 1
src/abap/5_syntax/syntax.ts.SyntaxLogic.updateSelectionEventScope 1
src/abap/abap_file.ts.ABAPFile.getInfo 10
src/abap/abap_file.ts.ABAPFile.getStatements 11
src/abap/abap_file.ts.ABAPFile.getStructure 9
src/abap/nodes/_abstract_node.ts.AbstractNode.getChildren 84
src/abap/nodes/_abstract_node.ts.AbstractNode.getFirstChild 12
src/abap/nodes/_abstract_node.ts.AbstractNode.getLastChild 4
src/abap/nodes/expression_node.ts.ExpressionNode.countTokens 2
src/abap/nodes/statement_node.ts.StatementNode.getColon 12
src/abap/nodes/statement_node.ts.StatementNode.getEnd 2
src/abap/nodes/statement_node.ts.StatementNode.getStart 3
src/abap/nodes/token_node.ts.TokenNode.concatTokens 1
src/abap/nodes/token_node.ts.TokenNode.getFirstToken 2
src/abap/types/_typed_identifier.ts.TypedIdentifier.getMeta 11
src/abap/types/_typed_identifier.ts.TypedIdentifier.getType 68
src/abap/types/_typed_identifier.ts.TypedIdentifier.getValue 11
src/abap/types/alias.ts.Alias.getComponent 7
src/abap/types/basic/_abstract_type.ts.AbstractType.getAbstractTypeData 12
src/abap/types/basic/_abstract_type.ts.AbstractType.getQualifiedName 3
src/abap/types/basic/character_type.ts.CharacterType.cloneType 1
src/abap/types/basic/character_type.ts.CharacterType.getLength 11
src/abap/types/basic/data_reference_type.ts.DataReference.getType 7
src/abap/types/basic/hex_type.ts.HexType.getLength 7
src/abap/types/basic/object_reference_type.ts.ObjectReferenceType.getIdentifier 6
src/abap/types/basic/object_reference_type.ts.ObjectReferenceType.getIdentifierName 12
src/abap/types/basic/structure_type.ts.StructureType.getComponentByName 17
src/abap/types/basic/structure_type.ts.StructureType.getComponents 16
src/abap/types/basic/table_type.ts.TableType.getAccessType 6
src/abap/types/basic/table_type.ts.TableType.getOptions 5
src/abap/types/basic/table_type.ts.TableType.getRowType 56
src/abap/types/basic/table_type.ts.TableType.isWithHeader 25
src/abap/types/basic/unknown_type.ts.UnknownType.getError 5
src/abap/types/basic/void_type.ts.VoidType.getVoided 3
src/abap/types/class_attribute.ts.ClassAttribute.getVisibility 1
src/abap/types/class_attributes.ts.Attributes.findByName 2
src/abap/types/class_attributes.ts.Attributes.getAliases 1
src/abap/types/class_constant.ts.ClassConstant.getVisibility 1
src/abap/types/class_definition.ts.ClassDefinition.addReference 1
src/abap/types/class_definition.ts.ClassDefinition.checkClassNameLength 1
src/abap/types/class_definition.ts.ClassDefinition.findSuper 1
src/abap/types/class_definition.ts.ClassDefinition.getImplementing 1
src/abap/types/class_definition.ts.ClassDefinition.getSuperClass 2
src/abap/types/class_definition.ts.ClassDefinition.isAbstract 1
src/abap/types/form_definition.ts.FormDefinition.findType 2
src/abap/types/form_definition.ts.FormDefinition.getChangingParameters 1
src/abap/types/form_definition.ts.FormDefinition.getTablesParameters 1
src/abap/types/form_definition.ts.FormDefinition.getUsingParameters 1
src/abap/types/interface_definition.ts.InterfaceDefinition.getImplementing 1
src/abap/types/method_definitions.ts.MethodDefinitions.getByName 1
src/abap/types/method_parameters.ts.MethodParameters.getChanging 2
src/abap/types/method_parameters.ts.MethodParameters.getExporting 1
src/abap/types/method_parameters.ts.MethodParameters.getImporting 2
src/abap/types/method_parameters.ts.MethodParameters.getOptional 2
src/abap/types/method_parameters.ts.MethodParameters.getReturning 1
src/abap/types/method_parameters.ts.MethodParameters.isPassByValue 2
src/abap/types/type_definitions.ts.TypeDefinitions.getByName 2
src/config.ts.Config.getGlobal 1
src/config.ts.Config.getRelease 1
src/config.ts.Config.getSyntaxSetttings 1
src/ddic.ts.DDIC.inErrorNamespace 25
src/ddic.ts.DDIC.lookup 1
src/ddic.ts.DDIC.lookupTableOrView 6
src/ddic.ts.DDIC.lookupView 1
src/ddic_references.ts.DDICReferences.listUsing 1
src/files/_abstract_file.ts.AbstractFile.getFilename 33
src/issue.ts.Issue.getEnd 4
src/issue.ts.Issue.getFilename 3
src/issue.ts.Issue.getKey 4
src/issue.ts.Issue.getMessage 2
src/issue.ts.Issue.getSeverity 2
src/issue.ts.Issue.getStart 6
src/objects/_abap_object.ts.ABAPObject.getABAPFiles 17
src/objects/_abstract_object.ts.AbstractObject.getFiles 3
src/objects/_abstract_object.ts.AbstractObject.getName 9
src/objects/_abstract_object.ts.AbstractObject.getXML 1
src/objects/_abstract_object.ts.AbstractObject.isDirty 1
src/objects/class.ts.Class.getCategory 3
src/objects/class.ts.Class.getClassDefinition 5
src/objects/class.ts.Class.setDefinition 3
src/objects/interface.ts.Interface.getDefinition 1
src/objects/interface.ts.Interface.getInterface 1
src/objects/program.ts.Program.isInclude 5
src/position.ts.Position.getCol 13
src/position.ts.Position.getRow 10
src/position.ts.Position.isBefore 5
src/registry.ts.Registry.clear 1
src/registry.ts.Registry.getConfig 4
src/registry.ts.Registry.isDependency 2
src/registry.ts.Registry.parsePrivate 1
src/registry.ts.Registry.removeDependency 1
src/rules/allowed_object_naming.ts.AllowedObjectNaming.getMetadata 1
src/rules/check_include.ts.CheckInclude.getMetadata 2
src/rules/implement_methods.ts.ImplementMethods.getMetadata 7
src/rules/indentation.ts.Indentation.getMetadata 1
src/rules/keyword_case.ts.KeywordCase.getMetadata 1
src/rules/parser_error.ts.ParserError.getMetadata 4
src/rules/superclass_final.ts.SuperclassFinal.getMessage 1
src/rules/superclass_final.ts.SuperclassFinal.getMetadata 2
src/rules/unknown_types.ts.UnknownTypes.getMetadata 6
src/utils/include_graph.ts.Graph.addVertex 5
src/utils/include_graph.ts.Graph.findVertexByFilename 1
src/utils/include_graph.ts.Graph.findVertexViaIncludename 1
```

## Issues exposed by the full input

Template fact extraction visited large methods containing nil block entries and
panicked. It now rejects those template shapes safely. Test copies preserve nil
entries and literal types; gob could not encode these programs. Regression tests
cover nil entries, shared pointers, copy isolation and integers beyond binary64's
exact range.

The first completed inline comparison found equal call-site totals (1,487) but a
dump difference at line 31,780 in generated local names. The `inline_template`
guard skipped eager preparation of eligible callees whose original return shape
could not be flattened. The reference inliner prepares those callees first, including
their own calls, then rejects expansion if needed. Grace now follows that order;
a small shadowed-declaration fixture locks the behavior. The full oracle dump and
all per-callee counters then matched exactly.

## Adapter effects and initialization

`hir/rewrite/effects.go` covers **87/87 catalogue operations and 6/6 SpecialOps**.
There are 93 records: MayRaise None=68, Trap=4, Catchable=21. Nine records mark
conservative numeric parsing, regex, replacement or invoked-constructor behavior, with reasons. Only telemetry
reads Global. Optional lookups do not raise; charCodeAt and localeCompareNames
trap outside their domains. Wrong dynamic tags and classvalue.new/materialize
raise catchable exceptions. set.fromArray allocates without writing its input;
splice1_view may alias its receiver. Native Cast/Narrow checks and checked
integer arithmetic also contribute to may_throw (including unary arithmetic).
Allocates is a descriptive fact, not a confinement proof: an Optional wrapper
can contain an existing reference. A regression ensures mutating such a
reference remains impure.

The table is exposed as op_reads, op_writes, op_allocates, op_may_raise and
op_aliases base facts. Coverage fails when any catalogue or SpecialOp lacks a
record. The full input has 137 op_reads facts, 93 of each other effect predicate,
and nine op_conservative facts.

On the rebased pre-change adapter, the same full closure gives **3,776
may_throw methods**; with the new effects and initializer guards it gives
**3,767**, a reduction of nine. Pure methods change from 3,493 to 3,498. These
are possible-effect classifications, not a handler-coverage proof. The baseline
was measured from an isolated snapshot of rebased commit 08075da, with only its
colliding pinned-oracle symbols replaced to allow compilation; the baseline
analysis was unchanged (27.58 seconds including lowering and reporting).

The adapter emits **2,470 ensure_init(Method,Site,Class) facts** from the emitter's
rules: non-constant static reads, static writes, class New, and static-method or
constructor entry. The initializer itself sets its flag rather than guarding
its own entry. Constant-only and empty initializers have no guards. Literal
promotion mirrors hir/abap/constants.go. An assignment LHS is a write, not a
static read. There is no EnsureInit HIR node. Static/field facts retain separate
owner/name columns, flag_init includes the field name, and constant tracking
uses a structured FieldKey; collision and inherited-owner tests cover identity.

## Verification and runtimes

The branch was rebased without conflicts onto origin/main e8e8c35 (abapiti
v0.2.0). Every heavy run uses flock /tmp/abapiti-heavy.lock. This shared machine's
/tmp was nearly full and its build cache was read-only inside the sandbox, so
successful runs use the existing Go cache and disk-backed build/test temporary
files under /var/tmp/abapiti-grace-build.

The final `go test -short ./...` passed in **32.30 seconds** wall time
(`hir/rewrite`: 4.73 seconds; `tsfront`: 25.00 seconds). The lexer facts golden
was reviewed and refreshed: may_throw stays at 20 methods, pure changes from 64
to 70, and constant-only initialization calls and assignment-LHS reads are
removed. The new source loop, native Cast/Narrow/overflow, Optional reference
allocation, constructor-effect, field-identity and EnsureInit regressions pass.

The complete `TestGraceFullRegistryClosure` passed in **13m 1.596s**
(Go test: 781.69 seconds for the test, 781.855 for the package). Successful-run
phase measurements:

| Phase | Runtime | Evidence |
| --- | ---: | --- |
| Lowering and input verification | 8.21s | 1,538 files; zero blocking diagnostics or HIR verification errors |
| Real main inliner oracle | 28.59s | 1,487 sites, 190 callees; identical bytes and counters |
| Independent analysis comparison | 93.15s | 64,811,461 premises; equal facts and engine termination |
| Independent combined comparison | 130.59s | 72,580,861 premises; equal analysis and inline facts |
| Depth/method/program budgets | 186.09s | Independent actual statement/expression growth checks |
| All Grace checks | 12m 35.113s | Every invariant passed; one verified fixed-point round |

Reevaluation and phased preparation passed. Positive-rule monotonicity passed
after adding throws and static writes. Declaration-order determinism passed for
base and derived facts and for inline output after sorting declarations and
alpha-normalizing generated locals. Executable statement order was preserved.

A second Inline changed no sites or HIR. The bounded rewrite reached a verified
fixed point in one round (below its 16-round limit), and another Inline again
changed no sites or HIR. All requested full-closure checks are green. No push or
PR creation was performed.
