package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestCollectionFastPaths(t *testing.T) {
	keys := []hir.Type{hir.T(hir.String), hir.T(hir.I32), hir.T(hir.I64), hir.Ref("Item"), hir.T(hir.Number), hir.Ref(hir.RootObject), hir.T(hir.Optional, hir.T(hir.Number))}
	p := &hir.Program{Classes: []*hir.Class{{Name: "Item"}, {Name: "Collections"}}}
	for i, key := range keys {
		if key.Kind != hir.Optional {
			p.Classes[1].Fields = append(p.Classes[1].Fields, hir.Field{Name: "map" + string(rune('A'+i)), Type: hir.T(hir.OrderedMap, key, i32)})
		}
		p.Classes[1].Fields = append(p.Classes[1].Fields, hir.Field{Name: "set" + string(rune('A'+i)), Type: hir.T(hir.OrderedSet, key)})
	}
	files, err := Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"jsString", "int32", "int64", hir.NewNames().Get("ref.Item")} {
		for _, want := range []string{"*fastOrderedMap[" + typ + ", int32]", "*fastOrderedSet[" + typ + "]"} {
			if !strings.Contains(files["hir.go"], want) {
				t.Errorf("missing direct-key type %s", want)
			}
		}
	}
	for _, typ := range []string{"float64", "any"} {
		for _, want := range []string{"*orderedMap[" + typ + ", int32]", "*orderedSet[" + typ + "]"} {
			if !strings.Contains(files["hir.go"], want) {
				t.Errorf("missing normalized type %s", want)
			}
		}
	}
	if !strings.Contains(files["hir.go"], "*orderedSet[optional[float64]]") {
		t.Error("missing optional numeric set")
	}
	tree, err := parser.ParseFile(token.NewFileSet(), "runtime.go", files["runtime.go"], 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range tree.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil {
			continue
		}
		recv, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		var base ast.Expr
		switch x := recv.X.(type) {
		case *ast.IndexExpr:
			base = x.X
		case *ast.IndexListExpr:
			base = x.X
		default:
			continue
		}
		id, ok := base.(*ast.Ident)
		if !ok || (id.Name != "fastOrderedMap" && id.Name != "fastOrderedSet") {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.TypeSwitchStmt:
				t.Errorf("%s.%s switches on runtime key type", id.Name, fn.Name)
			case *ast.CallExpr:
				if f, ok := x.Fun.(*ast.Ident); ok {
					switch f.Name {
					case "collectionKey", "collectionValue", "setKey", "normalizeSetValue", "floatByte", "nilRef":
						t.Errorf("%s.%s calls %s", id.Name, fn.Name, f.Name)
					}
				}
			}
			return true
		})
	}
	// Exercise the emitted direct indexes through growth, updates, compaction,
	// reinsertion and typed nil pointer keys; existing oracles cover normalized keys.
	main := `check:=func(ok bool){if !ok {panic("direct index")}}
 m:=&orderedMap[jsString,int32]{};s:=&orderedSet[jsString]{}
 ints:=&orderedMap[int32,int64]{};wide:=&orderedSet[int64]{}
 for i:=int32(0);i<40;i++ {k:=numberString(float64(i));m.set(k,i);s.add(k);ints.set(i,int64(i));wide.add(int64(i))}
 for i:=int32(0);i<40;i+=2 {k:=numberString(float64(i));check(m.delete(k)&&s.delete(k)&&ints.delete(i)&&wide.delete(int64(i)))}
 for i:=int32(1);i<40;i+=2 {k:=numberString(float64(i));check(m.get(k).Value==i&&s.has(k)&&ints.get(i).Value==int64(i)&&wide.has(int64(i)));m.set(k,i+1)}
 m.set(str("0"),99);s.add(str("0"));check(m.Entries[20].Key==str("0")&&s.Items[20]==str("0"))
 // Interface storage can still hold floats and must retain SameValueZero.
 zero:=float64(0);nan:=zero/zero;negativeZero:=-zero
 mixed:=&orderedMap[any,int32]{};mixed.set(nan,1).set(nan+nan,2).set(negativeZero,3)
 check(len(mixed.Entries)==2&&mixed.get(nan).Value==2&&mixed.has(zero)&&fmt.Sprint(mixed.keys().Items[1])=="0")
 mixedSet:=&orderedSet[any]{};mixedSet.add(nan).add(nan+nan).add(negativeZero).add(zero).add(int32(1)).add(float64(1))
 check(len(mixedSet.Items)==3&&mixedSet.has(nan)&&fmt.Sprint(mixedSet.Items[1])=="0"&&mixedSet.delete(nan)&&!mixedSet.has(nan))
 `
	item := hir.NewNames().Get("ref.Item")
	main += "refs:=&orderedMap[" + item + ",int32]{};set:=&orderedSet[" + item + "]{};var ref " + item + ";refs.set(ref,3);set.add(ref);check(refs.get(nil).Value==3&&set.has(nil));fmt.Println(\"ok\")"
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}
