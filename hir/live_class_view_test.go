package hir

import "testing"

func TestLiveStringArrayViewTyping(t *testing.T) {
	for _, receiver := range []Type{T(Dynamic), T(String)} {
		for _, result := range []Type{T(Array, T(String)), T(Array, T(Number)), T(String)} {
			op := &Expr{Kind: RuntimeOp, Op: "dynamic.arrayView", X: V("source", receiver), Type: result}
			p := &Program{Classes: []*Class{{Name: "Probe", Methods: []*Method{{Name: "view", Static: true, Params: []Param{{Name: "source", Type: receiver}}, Result: result, Body: B(&Stmt{Kind: Return, X: op})}}}}}
			want := receiver.Kind == Dynamic && result.Equal(T(Array, T(String)))
			if errors := Verify(p); (len(errors) == 0) != want {
				t.Fatalf("receiver=%s result=%s: %v", receiver, result, errors)
			}
		}
	}
}

func TestLiveNestedRowViewBoundary(t *testing.T) {
	row := &Class{Name: "Row", Structural: true, Fields: []Field{{Name: "name", Type: T(String)}, {Name: "next", Type: T(Optional, Ref("Row"))}}}
	nominal := &Class{Name: "Native"}
	conf := &Class{Name: "Conf", Fields: []Field{{Name: "rows", Type: T(Array, Ref("Row"))}, {Name: "native", Type: Ref("Native")}}}
	use := &Class{Name: "Use", Methods: []*Method{{Name: "view", Static: true, Body: B(&Stmt{Kind: ExprStmt, X: &Expr{Kind: RuntimeOp, Op: "dynamic.classView", Type: Ref("Conf"), X: V("input", T(Dynamic))}})}}}
	p := &Program{Classes: []*Class{conf, row, nominal, use}}
	views := LiveClassViewLayouts(p)
	if !views["Conf"] || !views["Row"] || views["Native"] {
		t.Fatalf("nested rows only: %v", views)
	}
	row.Methods = []*Method{{Name: "read", Result: T(String), Body: B(&Stmt{Kind: Return, X: &Expr{Kind: FieldGet, Name: "name", Type: T(String), X: &Expr{Kind: This, Type: Ref("Row")}}})}}
	use.Methods = append(use.Methods, &Method{Name: "dispatch", Body: B(&Stmt{Kind: ExprStmt, X: &Expr{Kind: VirtualCall, Name: "read", Type: T(String), X: V("row", Ref("Row"))}})})
	SealLiveClassViewBoundary(p)
	use.Methods = use.Methods[:1]
	if !row.ViewUnsafe || LiveClassViewLayouts(p)["Row"] {
		t.Fatal("inlining must preserve a nested row's prototype refusal")
	}
}

func TestLiveClassViewLayoutsWithExpandsAdditionalRoots(t *testing.T) {
	base := &Class{Name: "TargetBase", Fields: []Field{{Name: "inherited", Type: T(String)}}}
	nested := &Class{Name: "TargetNested", Structural: true, Fields: []Field{{Name: "value", Type: T(String)}}}
	row := &Class{Name: "TargetRow", Super: "TargetBase", Structural: true, Fields: []Field{{Name: "child", Type: Ref("TargetNested")}}}
	sourceBase := &Class{Name: "SourceBase", Fields: []Field{{Name: "inherited", Type: T(String)}}}
	sourceNested := &Class{Name: "SourceNested", Structural: true, Fields: []Field{{Name: "value", Type: T(String)}}}
	sourceRow := &Class{Name: "SourceRow", Super: "SourceBase", Structural: true, Fields: []Field{{Name: "child", Type: Ref("SourceNested")}}}
	program := &Class{Name: "Program", Structural: true, Fields: []Field{{Name: "rows", Type: T(OrderedMap, T(String), Ref("TargetRow"))}}}
	use := &Class{Name: "Use", Methods: []*Method{{Name: "view", Static: true, Body: B(&Stmt{Kind: ExprStmt, X: &Expr{Kind: RuntimeOp, Op: "dynamic.classView", Type: Ref("Program"), X: V("input", T(Dynamic))}})}}}
	p := &Program{Classes: []*Class{base, nested, row, sourceBase, sourceNested, sourceRow, program, use}}
	views := LiveClassViewLayoutsWith(p, map[string]bool{"TargetRow": true, "SourceRow": true})
	for _, name := range []string{"Program", "TargetRow", "TargetBase", "TargetNested", "SourceRow", "SourceBase", "SourceNested"} {
		if !views[name] {
			t.Fatalf("additional structural root was not expanded to %s: %v", name, views)
		}
	}
}

func TestDataClassViewBoundary(t *testing.T) {
	base := &Class{Name: "Base", Abstract: true, Fields: []Field{{Name: "optional", Type: T(Optional, T(String))}}}
	child := &Class{Name: "Child", Super: "Base", Fields: []Field{{Name: "value", Type: T(Number)}}}
	p := &Program{Classes: []*Class{base, child}}
	if !DataClassView(p, "Child") {
		t.Fatal("public data ancestry must support a live view")
	}
	base.Fields[0].Private = true
	if DataClassView(p, "Child") {
		t.Fatal("inherited private fields must refuse")
	}
	base.Fields[0].Private = false
	base.ViewUnsafe = true
	if DataClassView(p, "Child") {
		t.Fatal("accessors must refuse even when their lowering was skipped")
	}
	base.ViewUnsafe = false
	base.Methods = []*Method{{Name: "read", Result: T(Number), Body: B(&Stmt{Kind: Return, X: &Expr{Kind: This, Type: Ref("Base")}})}}
	if DataClassView(p, "Child") {
		t.Fatal("prototype method dispatch must refuse")
	}
	base.Methods = []*Method{{Name: "read", Result: T(Optional, T(String)), Body: B(&Stmt{Kind: Return, X: &Expr{Kind: FieldGet, Name: "optional", Type: T(Optional, T(String)), X: &Expr{Kind: This, Type: Ref("Base")}}})}}
	if !DataClassView(p, "Child") {
		t.Fatal("unused field reader is safe")
	}
	p.Classes = append(p.Classes, &Class{Name: "Consumer", Methods: []*Method{{Name: "use", Body: B(&Stmt{Kind: ExprStmt, X: &Expr{Kind: VirtualCall, Name: "read", Type: T(Optional, T(String)), X: V("conf", Ref("Child"))}})}}})
	if DataClassView(p, "Child") {
		t.Fatal("prototype dispatch must refuse before inlining")
	}
	child.Methods = append(child.Methods, &Method{Name: "set", Static: true, Body: B(&Stmt{Kind: ExprStmt, X: &Expr{Kind: RuntimeOp, Op: "dynamic.classView", Type: Ref("Child"), X: V("input", T(Dynamic))}})})
	SealLiveClassViewBoundary(p)
	p.Classes = p.Classes[:2] // Simulate elimination of the dispatch by inlining.
	if DataClassView(p, "Child") {
		t.Fatal("inlining must not erase the prototype refusal")
	}
	child.ViewUnsafe = false
	base.Methods = nil
	child.Super = "Missing"
	if DataClassView(p, "Child") {
		t.Fatal("unknown ancestry must refuse")
	}
}
