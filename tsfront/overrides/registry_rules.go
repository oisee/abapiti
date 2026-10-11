package overrides

// These additional consumers are outside the misc-roots batch. The supported
// config boundary materializes JSON string arrays densely; sparse programmatic
// configurations remain outside that boundary. Neither callback exposes or
// changes receiver membership. Find retains its optional result.
func registryRulesCallbacks() []Entry {
	return []Entry{
		{ID: "abaplint-rules-dense-UncaughtException", Key: Key{"src/rules/uncaught_exception.ts", "UncaughtException.setSinkedFromMethod", "KindMethodDeclaration"}, SHA256: "834b30a17777c2ccb6a6ddaaf94a847edc1f0b34fd8dbd893c256a4cb126e68a", Rationale: "pinned MethodDefinition builds its private raising array densely by push; consumer initializes a fresh sink array and callback only appends strings there, never changing receiver membership", Patterns: &Patterns{DenseCallbacks: map[string]bool{
			"def.getRaising().forEach(r => this.sinked?.push(r))": true,
		}}},
		{ID: "abaplint-rules-raising-producer-guard", Key: Key{"src/abap/types/method_definition.ts", "MethodDefinition", "KindClassDeclaration"}, SHA256: "85bc7c4a7abff05a50eac296ab636c69627be5e4a23be862a4c4099f6eebbde2", Rationale: "density dependency for UncaughtException: all raising writes are constructor push into a private fresh string array; getRaising exposes only the readonly array; audited closure consumers do not mutate it", Patterns: &Patterns{SourceGuard: true}},

		{ID: "abaplint-rules-dense-Parser702Chaining", Key: Key{"src/rules/parser_702_chaining.ts", "Parser702Chaining.ensureSourceHasNoProceduralKeywords", "KindMethodDeclaration"}, SHA256: "f1f1b4497cb670d0f07a4eee3e140250ed3b9d6cd02ac9ef2e141731ecc35365", Rationale: "private local literal contains four nonempty strings; predicate only queries the expression node; no receiver escape or membership writes", Patterns: &Patterns{DenseCallbacks: map[string]bool{
			"forbiddenTokens.find(text => param.findDirectTokenByText(text))": true,
		}}},
		{ID: "abaplint-rules-dense-ShortCase", Key: Key{"src/rules/short_case.ts", "ShortCase", "KindClassDeclaration"}, SHA256: "9801c067ca36b09f946e01ec2b5b884f214d0d832a5c42064f69ec160b88e6fa", Rationale: "supported JSON config boundary produces dense allow:string[]; entire consumer class pins every config assignment; predicate only compares strings and never exposes or mutates allow; an empty-string match remains present", Patterns: &Patterns{DenseCallbacks: map[string]bool{
			"this.conf.allow.find((e) => { return e === caseStatement.getTokens()[1].getStr(); })": true,
		}}},
	}
}
