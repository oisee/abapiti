package tsfront

import (
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

func TestTypeRefNameQualified(t *testing.T) {
	f := ast.NewNodeFactory(ast.NodeFactoryHooks{})
	for _, tc := range []struct {
		name *ast.Node
		want string
	}{
		{f.NewIdentifier("const"), "const"},
		{f.NewIdentifier("Value"), "Value"},
		{f.NewQualifiedName(f.NewIdentifier("Basic"), f.NewIdentifier("TableType")), "Basic.TableType"},
		{f.NewQualifiedName(f.NewQualifiedName(f.NewIdentifier("Outer"), f.NewIdentifier("Basic")), f.NewIdentifier("Value")), "Outer.Basic.Value"},
		{nil, "?"},
	} {
		if got := typeRefName(f.NewTypeReferenceNode(tc.name, nil)); got != tc.want {
			t.Fatalf("got %q, want %q", got, tc.want)
		}
	}
}

func TestNamespaceQualifiedCastDoesNotPanic(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{
		"basic.ts": `export class Value {value = 7;}`,
		"probe.ts": `import * as Basic from "./basic"; export class Probe {run():number {const x = new Basic.Value(); const y = x as Basic.Value; return y.value;}}`,
	}, []string{"basic.ts", "probe.ts"})
	if got := runGoHIR(t, p, "fmt.Println("+hir.NewNames().Get("new.probe.ts.Probe")+"()."+hir.NewNames().Get("member.run")+"())"); got != "7\n" {
		t.Fatal(got)
	}
}
