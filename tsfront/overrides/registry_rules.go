package overrides

// These additional consumers are outside the misc-roots batch. The supported
// config boundary materializes JSON string arrays densely; sparse programmatic
// configurations remain outside that boundary. Neither callback exposes or
// changes receiver membership. Find retains its optional result.
func registryRulesCallbacks() []Entry {
	return []Entry{
		{ID: "abaplint-rules-dense-Parser702Chaining", Key: Key{"src/rules/parser_702_chaining.ts", "Parser702Chaining.ensureSourceHasNoProceduralKeywords", "KindMethodDeclaration"}, SHA256: "f1f1b4497cb670d0f07a4eee3e140250ed3b9d6cd02ac9ef2e141731ecc35365", Rationale: "private local literal contains four nonempty strings; predicate only queries the expression node; no receiver escape or membership writes", Patterns: &Patterns{DenseCallbacks: map[string]bool{
			"forbiddenTokens.find(text => param.findDirectTokenByText(text))": true,
		}}},
		{ID: "abaplint-rules-dense-ShortCase", Key: Key{"src/rules/short_case.ts", "ShortCase", "KindClassDeclaration"}, SHA256: "9801c067ca36b09f946e01ec2b5b884f214d0d832a5c42064f69ec160b88e6fa", Rationale: "supported JSON config boundary produces dense allow:string[]; entire consumer class pins every config assignment; predicate only compares strings and never exposes or mutates allow; an empty-string match remains present", Patterns: &Patterns{DenseCallbacks: map[string]bool{
			"this.conf.allow.find((e) => { return e === caseStatement.getTokens()[1].getStr(); })": true,
		}}},
	}
}
