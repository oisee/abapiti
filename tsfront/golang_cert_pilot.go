package tsfront

import (
	"bytes"
	_ "embed"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"

	"github.com/oisee/abapiti/hir"
)

//go:embed testdata/certpilot/prestore_test.go.txt
var pilotPrestoreTests string

// RegistryGoCertificatePilot instruments emitted Go only. It never edits HIR.
// The caller must first validate accepted certificates against original source.
func RegistryGoCertificatePilot(files map[string]string) error {
	n := hir.NewNames()
	combi := "src/abap/3_structures/structures/_combi.ts."
	root := "src/abap/3_structures/structure_parser.ts.StructureParser"
	member := func(s string) string { return n.Get("member." + s) }
	obj := func(s string) string { return n.Get("struct." + combi + s) }
	body := func(c, m string) string { return n.Get("body." + c + "." + m) }
	static := n.Get("static." + root + ".singletons")
	subcache := n.Get("static." + combi + "module.singletons")
	checks := map[string]string{
		body(combi+"Alternative", "setupMap"):      "if pilotFrozen && !nilRef(self) && self." + n.Get("base."+combi+"Alternative") + "()." + member("map") + " == nil {pilotStore(\"Alternative.map\")}",
		body(combi+"SubStructure", "setupMatcher"): "if pilotFrozen && !nilRef(self) && nilRef(self." + n.Get("base."+combi+"SubStructure") + "()." + member("matcher") + ") {pilotStore(\"SubStructure.matcher\")}",
		body(root, "runFile"):                      "if pilotFrozen && nilRef(" + static + ".get(classOf(p0).Name).Value) {pilotStore(\"StructureParser.singletons\")}",
		body(combi+"module", "sub"):                "if pilotFrozen && nilRef(" + subcache + ".get(p0.Name).Value) {pilotStore(\"sub.singletons\")}",
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "hir.go", files["hir.go"], 0)
	if err != nil {
		return err
	}
	found := map[string]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if check, ok := checks[fn.Name.Name]; ok {
			extra, err := pilotStatements(check)
			if err != nil {
				return err
			}
			fn.Body.List = append(extra, fn.Body.List...)
			found[fn.Name.Name] = true
		}
	}
	for k := range checks {
		if !found[k] {
			return fmt.Errorf("pilot cache barrier missing emitted function %s", k)
		}
	}
	// Locate the structures for-loop by its direct StructureParser.run call.
	region := body("src/abap/abap_parser.ts.ABAPParser", "parse")
	run := body(root, "run")
	regions := 0
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != region {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			block, ok := node.(*ast.BlockStmt)
			if !ok {
				return true
			}
			for i, stmt := range block.List {
				loop, ok := stmt.(*ast.ForStmt)
				if !ok {
					continue
				}
				calls := false
				ast.Inspect(loop.Body, func(x ast.Node) bool {
					if call, ok := x.(*ast.CallExpr); ok {
						if id, ok := call.Fun.(*ast.Ident); ok && id.Name == run {
							calls = true
						}
					}
					return true
				})
				if calls {
					extra, err := pilotStatements("pilotWarmup(); pilotNegative()")
					if err != nil {
						panic(err)
					}
					block.List = append(block.List[:i], append(extra, block.List[i:]...)...)
					regions++
					return false
				}
			}
			return true
		})
	}
	if regions != 1 {
		return fmt.Errorf("pilot needs one structures region, found %d", regions)
	}
	var out bytes.Buffer
	if err := format.Node(&out, fset, f); err != nil {
		return err
	}
	files["hir.go"] = out.String()
	// Collection barriers include mutations through map/array aliases. Frozen
	// matcher fields are guarded at their miss sites before RHS construction.
	rfset := token.NewFileSet()
	rf, err := parser.ParseFile(rfset, "runtime.go", files["runtime.go"], 0)
	if err != nil {
		return err
	}
	for _, decl := range rf.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || (ts.Name.Name != "orderedMap" && ts.Name.Name != "array") {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					return fmt.Errorf("pilot collection shape changed")
				}
				st.Fields.List = append(st.Fields.List, &ast.Field{Names: []*ast.Ident{ast.NewIdent("pilotCache")}, Type: ast.NewIdent("string")})
			}
		}
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil {
			continue
		}
		recv := fn.Recv.List[0]
		var typeName string
		ast.Inspect(recv.Type, func(x ast.Node) bool {
			if id, ok := x.(*ast.Ident); ok && (id.Name == "array" || id.Name == "orderedMap") {
				typeName = id.Name
			}
			return true
		})
		if typeName == "" {
			continue
		}
		mutates := false
		ast.Inspect(fn.Body, func(x ast.Node) bool {
			if a, ok := x.(*ast.AssignStmt); ok {
				for _, lhs := range a.Lhs {
					ast.Inspect(lhs, func(y ast.Node) bool {
						if sel, ok := y.(*ast.SelectorExpr); ok {
							if id, ok := sel.X.(*ast.Ident); ok && id.Name == recv.Names[0].Name {
								mutates = true
							}
						}
						return true
					})
				}
			}
			return true
		})
		if !mutates {
			continue
		}
		check := fmt.Sprintf("if %s.pilotCache != \"\" {pilotStore(%s.pilotCache)}", recv.Names[0].Name, recv.Names[0].Name)
		if fn.Name.Name == "ensureIndex" {
			check = fmt.Sprintf("if %s.index == nil && %s.pilotCache != \"\" {pilotStore(%s.pilotCache)}", recv.Names[0].Name, recv.Names[0].Name, recv.Names[0].Name)
		}
		extra, err := pilotStatements(check)
		if err != nil {
			return err
		}
		fn.Body.List = append(extra, fn.Body.List...)
	}
	out.Reset()
	if err := format.Node(&out, rfset, rf); err != nil {
		return err
	}
	files["runtime.go"] = out.String()
	runnable := n.Get("ref.src/abap/3_structures/structures/_structure_runnable.ts.IStructureRunnable")
	var code strings.Builder
	code.WriteString(`package main
import("os";"fmt")
type pilotViolation struct{Cache string}
func (v pilotViolation) Error()string{return "frozen cache write blocked before store: "+v.Cache}
var pilotFrozen bool
var pilotEnabled bool
var pilotWarmed bool
var pilotWarmNodes int
var pilotAlternatives []any
func pilotStore(cache string){if pilotFrozen{panic(pilotViolation{cache})}}
func pilotWarmup(){
 if !pilotEnabled || pilotWarmed {return}
 defer func(){if x:=recover();x!=nil{panic(pilotViolation{fmt.Sprintf("warm-up refused: %v",x)})}}()
 seen:=map[any]bool{}
`)
	fmt.Fprintf(&code, "var walk func(%s)\nwalk=func(node %s){if nilRef(node){panic(pilotViolation{\"nil matcher\"})};if seen[node]{return};seen[node]=true;switch x:=any(node).(type){\n", runnable, runnable)
	for _, kind := range []string{"Sequence", "Alternative", "Optional", "Star", "SubStructure", "SubStatement"} {
		fmt.Fprintf(&code, "case *%s:\n", obj(kind))
		switch kind {
		case "Sequence", "Alternative":
			if kind == "Alternative" {
				code.WriteString("pilotAlternatives=append(pilotAlternatives,x)\n")
				fmt.Fprintf(&code, "%s(x)\n", body(combi+kind, "setupMap"))
			}
			fmt.Fprintf(&code, "for _,child:=range x.%s.Items {walk(castRef[%s](child))}\n", member("list"), runnable)
			if kind == "Alternative" {
				fmt.Fprintf(&code, "x.%s.ensureIndex();x.%s.pilotCache=\"Alternative.map\";for _,e:=range x.%s.Entries{e.Value.pilotCache=\"Alternative.map\"}\n", member("map"), member("map"), member("map"))
			}
		case "Optional", "Star":
			fmt.Fprintf(&code, "walk(x.%s)\n", member("obj"))
		case "SubStructure":
			fmt.Fprintf(&code, "%s(x);walk(x.%s)\n", body(combi+kind, "setupMatcher"), member("matcher"))
		case "SubStatement":
			code.WriteString("_ = x\n")
		}
	}
	code.WriteString("default:panic(pilotViolation{fmt.Sprintf(\"unknown matcher %T\",node)})}}\n")
	fmt.Fprintf(&code, "%s();%s()\n", n.Get("init."+root), n.Get("init."+combi+"module"))
	for _, kind := range []string{"Any", "ClassGlobal", "InterfaceGlobal", "DynproLogic"} {
		file := map[string]string{"Any": "any", "ClassGlobal": "class_global", "InterfaceGlobal": "interface_global", "DynproLogic": "dynpro_logic"}[kind]
		klass := "src/abap/3_structures/structures/" + file + ".ts." + kind
		fmt.Fprintf(&code, "{root:=%s();key:=classOf(root).Name;matcher:=%s.get(key).Value;if nilRef(matcher){matcher=root.%s();%s.set(key,matcher)};walk(matcher)}\n", n.Get("new."+klass), static, member("getMatcher"), static)
	}
	fmt.Fprintf(&code, "%s.ensureIndex();%s.ensureIndex();%s.pilotCache=\"StructureParser.singletons\";%s.pilotCache=\"sub.singletons\"\npilotWarmNodes=len(seen);pilotWarmed=true;pilotFrozen=true;if os.Getenv(\"ABAPITI_CERT_TRACE\")==\"1\"{fmt.Fprintf(os.Stderr,\"certificate warm-up: %%d matcher nodes frozen before structures loop\\n\",pilotWarmNodes)}\n}\n", static, subcache, static, subcache)
	// Injection uses real cache-miss paths, without changing frozen cache state.
	fmt.Fprintf(&code, `type pilotLateRoot struct{%s}
func(x *pilotLateRoot)descriptor()*classDescriptor{return &classDescriptor{Name:str("pilot-late-root")}}
func pilotNegative(){if !pilotEnabled{return};switch os.Getenv("ABAPITI_CERT_LATE_WRITE"){
case "StructureParser.singletons":%s(&pilotLateRoot{%s()},nil,&array[any]{})
case "Alternative.map":%s(&%s{%s:&array[any]{}})
case "SubStructure.matcher":%s(&%s{%s:%s()})
case "sub.singletons":%s(&classDescriptor{Name:str("pilot-late-sub"),Factory:func()any{return %s()}})
case "":return
default:panic(pilotViolation{"unknown negative test"})}}
`, n.Get("ref.src/abap/3_structures/structures/_structure.ts.IStructure"), body(root, "runFile"), n.Get("new.src/abap/3_structures/structures/any.ts.Any"), body(combi+"Alternative", "setupMap"), obj("Alternative"), member("list"), body(combi+"SubStructure", "setupMatcher"), obj("SubStructure"), member("s"), n.Get("new.src/abap/3_structures/structures/any.ts.Any"), body(combi+"module", "sub"), n.Get("new.src/abap/3_structures/structures/any.ts.Any"))
	formatted, err := format.Source([]byte(code.String()))
	if err != nil {
		return fmt.Errorf("pilot runtime: %w", err)
	}
	files["cert_pilot.go"] = string(formatted)
	files["main.go"] = RegistryGoPilotCLI()
	files["cert_pilot_test.go"] = pilotPrestoreTests
	return nil
}
func pilotStatements(code string) ([]ast.Stmt, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "inject.go", "package main;func inject(){"+code+"}", 0)
	if err != nil {
		return nil, err
	}
	fn, ok := f.Decls[0].(*ast.FuncDecl)
	if !ok {
		return nil, fmt.Errorf("pilot injection is not a function")
	}
	return fn.Body.List, nil
}

// PilotWarmupCoverage retains source bodies needed by graph construction.
// This is only used by the Go-only pilot, never the ABAP lowering path.
func PilotWarmupCoverage(r *Reachability) *Reachability {
	out := *r
	out.Spans = nil
	for _, s := range r.Spans {
		graph := strings.HasPrefix(s.File, "src/abap/3_structures/") || strings.HasPrefix(s.File, "src/abap/2_statements/")
		needed := s.Kind == "Constructor" || strings.HasSuffix(s.Symbol, ".first") || strings.HasSuffix(s.Symbol, ".getMatcher") || strings.HasSuffix(s.Symbol, ".setupMap") || strings.HasSuffix(s.Symbol, ".setupMatcher") || s.Kind == "FunctionDeclaration"
		// The original INTF sequencing and naming bodies is needed by the pinned multi-file source kit.
		if s.File == "src/objects/interface.ts" && (strings.HasSuffix(s.Symbol, ".getSequencedFiles") || strings.HasSuffix(s.Symbol, ".getAllowedNaming")) {
			continue
		}
		if graph && needed {
			continue
		}
		out.Spans = append(out.Spans, s)
	}
	return &out
}
