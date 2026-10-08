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
		{ID: "abaplint-registry-rules-clock", Key: Key{"src/rules_runner.ts", "RulesRunner.runRules", "KindMethodDeclaration"}, SHA256: "64d961f9544b6b6f9ca71ce171873fee993bc5e122405f49df6f61ebeaf0fda4", Rationale: "Addendum 5: timing telemetry only, monotonic GET RUN TIME milliseconds; general Date deferred", Expressions: map[string]func() *hir.Expr{"Date.now()": TelemetryClock}},
	}
}
