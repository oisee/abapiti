package overrides

import "github.com/oisee/abapiti/hir"

// TelemetryClock is authorized only for source-pinned duration measurements.
// General Date and epoch time remain unsupported.
func TelemetryClock() *hir.Expr {
	return &hir.Expr{Kind: hir.RuntimeOp, Op: "clock.telemetry", Type: hir.T(hir.Number), X: hir.L(hir.T(hir.Number), 0)}
}
func registryClocks() []Entry {
	return []Entry{
		{ID: "abaplint-registry-parser-clock", Key: Key{"src/abap/abap_parser.ts", "ABAPParser.parse", "KindMethodDeclaration"}, SHA256: "601d43a2b7e0db3851b483291ddd440330dd16c767738a81ce0642b445364273", Rationale: "Addendum 5: timing telemetry only, monotonic GET RUN TIME milliseconds; general Date deferred", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
		{ID: "abaplint-registry-rules-clock", Key: Key{"src/rules_runner.ts", "RulesRunner.runRules", "KindMethodDeclaration"}, SHA256: "64d961f9544b6b6f9ca71ce171873fee993bc5e122405f49df6f61ebeaf0fda4", Rationale: "Addenda 4/5: timing telemetry only; reached object enumeration is an immediate read-only snapshot; general Date/iterator ABI deferred", Patterns: &Patterns{Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}, Annotations: map[string]hir.Type{"Iterable<IObject>": hir.T(hir.Array, hir.Type{Kind: hir.InterfaceRef, Name: "src/objects/_iobject.ts.IObject"})}}},
		{ID: "abaplint-harness-parse-clock", Key: Key{"harness/registry_run.ts", "RegistryRun.parse", "KindMethodDeclaration"}, SHA256: "d89e5fcae2b8de806e2ea9a957dc4342583212ca26229cb91c12e2430be81937", Rationale: "deployment harness stage timing (lexer, statements, structures, macros, global definitions); telemetry only", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
		{ID: "abaplint-harness-report-clock", Key: Key{"harness/registry_run.ts", "RegistryRun.report", "KindMethodDeclaration"}, SHA256: "2d984847a5213442674a4f02765c9e24d4dcfff5b1e81738eed78afc7ee835ca", Rationale: "deployment harness stage timing (syntax, each rule); telemetry only", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
	}
}
