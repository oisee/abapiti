package overrides

import "github.com/oisee/abapiti/hir"

// ddicViewLookups: DDIC fetches VIEW objects but asserts them `as Table`.
// The assertion erases in JavaScript, so the object stays a View and its own
// parseType runs; a nominal Go/ABAP cast to Table would refuse instead. The
// local takes the class the registry actually returns for "VIEW", and its
// parseType call is the View's (the checker would otherwise narrow the local
// back to the stated Table at the call).
func ddicViewLookups() []Entry {
	view := hir.Ref("src/objects/view.ts.View")
	optionalView := hir.T(hir.Optional, view)
	const reason = "registry VIEW objects are Views; the upstream `as Table` assertion is erased in JS, so the View's own parseType runs"
	parse := func(local string) func() *hir.Expr {
		return func() *hir.Expr {
			this := &hir.Expr{Kind: hir.This, Type: hir.Ref("src/ddic.ts.DDIC")}
			reg := &hir.Expr{Kind: hir.FieldGet, Name: "reg", Type: hir.Type{Kind: hir.InterfaceRef, Name: "src/_iregistry.ts.IRegistry"}, X: this}
			receiver := &hir.Expr{Kind: hir.Narrow, Type: view, X: hir.V(local, optionalView)}
			return &hir.Expr{Kind: hir.VirtualCall, Name: "parseType", Type: hir.Ref("src/abap/types/basic/_abstract_type.ts.AbstractType"), X: receiver, Args: []*hir.Expr{reg}}
		}
	}
	assertion := map[string]hir.Type{`this.reg.getObject("VIEW", name) as Table | undefined`: optionalView}
	return []Entry{
		{ID: "abaplint-ddic-lookup-view-assertion", Key: Key{"src/ddic.ts", "DDIC.lookupNoVoid", "KindMethodDeclaration"}, SHA256: "cdb9ebe82dc3a277e4c063849472e6ff18ed345a842497bbd0d7844b53f8f818", Rationale: reason,
			Patterns: &Patterns{AssertionTypes: assertion, Expressions: map[string]func() *hir.Expr{"foundVIEW.parseType(this.reg)": parse("foundVIEW")}}},
		{ID: "abaplint-ddic-lookup-view-direct-assertion", Key: Key{"src/ddic.ts", "DDIC.lookupView", "KindMethodDeclaration"}, SHA256: "cafd3298519e4d20a1e2091609e61dc674da88cd5fa47b78080634d46fb1b37b", Rationale: reason,
			Patterns: &Patterns{AssertionTypes: assertion, Expressions: map[string]func() *hir.Expr{"found.parseType(this.reg)": parse("found")}}},
	}
}
