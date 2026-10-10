package tsfront

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
	gohir "github.com/oisee/abapiti/hir/golang"
)

const resultClass = "src/abap/2_statements/result.ts.Result"

func code(s string) []ast.Stmt {
	f, err := parser.ParseFile(token.NewFileSet(), "inject.go", "package main;func _(){"+s+"}", 0)
	if err != nil {
		panic(err)
	}
	return f.Decls[0].(*ast.FuncDecl).Body.List
}
func expr(s string) ast.Expr {
	e, err := parser.ParseExpr(s)
	if err != nil {
		panic(err)
	}
	return e
}
func quoted(s string) string { return strconv.Quote(s) }
func textOf(n ast.Node) string {
	var b bytes.Buffer
	_ = format.Node(&b, token.NewFileSet(), n)
	return b.String()
}

func RegistryGoResultDebug(files map[string]string, mode string) error {
	if mode != "ownership" && mode != "value" {
		return fmt.Errorf("invalid Result debug mode %q", mode)
	}
	var locals map[string]map[string]gohir.ResultDebugLocal
	if err := json.Unmarshal([]byte(files["result-debug-locals.json"]), &locals); err != nil {
		return fmt.Errorf("Result debug needs EmitResultDebug store metadata: %w", err)
	}
	fs := token.NewFileSet()
	parsed := map[string]*ast.File{}
	var all []*ast.File
	names := make([]string, 0, len(files))
	for name := range files {
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		f, err := parser.ParseFile(fs, name, files[name], 0)
		if err != nil {
			return err
		}
		parsed[name] = f
		all = append(all, f)
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
	stub, _ := parser.ParseFile(fs, "result_stub.go", "package main;func resultDebugSite(string){};func resultDebugMutation(string,string){}", 0)
	all = append(all, stub)
	config := types.Config{Importer: importer.Default()}
	pkg, err := config.Check("debug", fs, all, info)
	if err != nil {
		return fmt.Errorf("debug input typecheck: %w", err)
	}
	n := hir.NewNames()
	obj := n.Get("struct." + resultClass)
	ref := n.Get("ref." + resultClass)
	mutators := map[string]string{}
	for _, m := range []string{"wrapConsumed", "popNode", "setNodes"} {
		mutators[n.Get("body."+resultClass+"."+m)] = m
	}
	// Only interfaces/pointers whose concrete identity is Result are copied.
	isResult := func(t types.Type) bool {
		if t == nil {
			return false
		}
		s := types.TypeString(t, func(*types.Package) string { return "" })
		return s == ref || s == "*"+obj
	}
	// Sources give original TS method file:line; emitted offset distinguishes calls.
	if pkg.Scope().Lookup(obj) == nil {
		return fmt.Errorf("missing emitted Result class %s", obj)
	}
	couldBeResult := func(t types.Type) bool {
		if t == nil {
			return false
		}
		return isResult(t) || types.AssignableTo(types.NewPointer(pkg.Scope().Lookup(obj).Type()), t)
	}
	sources := map[string]string{}
	f := parsed["hir.go"]
	if f == nil {
		return fmt.Errorf("missing hir.go")
	}
	ast.Inspect(f, func(x ast.Node) bool {
		vs, ok := x.(*ast.ValueSpec)
		if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "goSources" {
			return true
		}
		cl := vs.Values[0].(*ast.CompositeLit)
		for _, e := range cl.Elts {
			kv := e.(*ast.KeyValueExpr)
			k, _ := strconv.Unquote(kv.Key.(*ast.BasicLit).Value)
			v, _ := strconv.Unquote(kv.Value.(*ast.BasicLit).Value)
			sources[k] = v
		}
		return false
	})
	stats := map[string]int{}
	if mode == "value" {
		for _, file := range all {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				if strings.HasPrefix(fn.Name.Name, "z_base_") {
					continue
				} // Go field-access borrow, not a TS return.
				semantic := locals[fn.Name.Name]
				ast.Inspect(fn.Body, func(x ast.Node) bool {
					if call, ok := x.(*ast.CallExpr); ok {
						if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "resultCopy" {
							return false
						}
					}
					switch x := x.(type) {
					case *ast.AssignStmt:
						for i, e := range x.Rhs {
							if i >= len(x.Lhs) {
								continue
							}
							store := true
							if id, ok := x.Lhs[i].(*ast.Ident); ok {
								meta, ok := semantic[id.Name]
								store = ok && !meta.Borrow
								if variable := info.Uses[id]; variable != nil && variable.Parent() == pkg.Scope() {
									store = true
								}
							}
							if store && couldBeResult(info.TypeOf(e)) {
								x.Rhs[i] = copyExpr(e)
								stats["stores"]++
							}
						}
					case *ast.ValueSpec:
						for i, e := range x.Values {
							if i >= len(x.Names) {
								continue
							}
							meta, ok := semantic[x.Names[i].Name]
							store := ok && !meta.Borrow
							if store && couldBeResult(info.TypeOf(e)) {
								x.Values[i] = copyExpr(e)
								stats["stores"]++
							}
						}
					case *ast.ReturnStmt:
						for i, e := range x.Results {
							if couldBeResult(info.TypeOf(e)) {
								x.Results[i] = copyExpr(e)
								stats["returns"]++
							}
						}
					case *ast.CompositeLit:
						for i, e := range x.Elts {
							if kv, ok := e.(*ast.KeyValueExpr); ok {
								if couldBeResult(info.TypeOf(kv.Value)) {
									kv.Value = copyExpr(kv.Value)
									stats["elements"]++
								}
							} else if isResult(info.TypeOf(e)) {
								x.Elts[i] = copyExpr(e)
								stats["elements"]++
							}
						}
					case *ast.CallExpr:
						bodyReceiver := false
						if id, ok := x.Fun.(*ast.Ident); ok {
							bodyReceiver = strings.HasPrefix(id.Name, "z_body_")
						}
						for i, e := range x.Args {
							if i == 0 && bodyReceiver {
								continue
							}
							if isResult(info.TypeOf(e)) {
								x.Args[i] = copyExpr(e)
								stats["arguments"]++
							}
						}
					}
					return true
				})
			}
		}
		// Result can be erased to T/any at collection stores. Copy at their public
		// write boundaries and when constructing a new collection from old slots.
		for _, decl := range parsed["runtime.go"].Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if fn.Recv != nil && collectionReceiver(fn) && (fn.Name.Name == "push" || fn.Name.Name == "put" || fn.Name.Name == "set") {
				fn.Body.List = append(code("v=resultCopy(v)"), fn.Body.List...)
				stats["generic_write_barriers"]++
			}
		}
		// New collections copy their Result elements, including splice views.
		for _, decl := range parsed["runtime.go"].Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if (fn.Name.Name == "unshift" || fn.Name.Name == "splice3" || fn.Name.Name == "add") && fn.Recv != nil {
				fn.Body.List = append(code("v=resultCopy(v)"), fn.Body.List...)
			}
			switch fn.Name.Name {
			case "concat", "slice0", "slice1", "slice2", "splice1", "splice2", "splice3", "splice1_view", "values":
				ast.Inspect(fn.Body, func(x ast.Node) bool {
					if ret, ok := x.(*ast.ReturnStmt); ok {
						for i, e := range ret.Results {
							if strings.HasPrefix(types.TypeString(info.TypeOf(e), func(*types.Package) string { return "" }), "*array[") {
								ret.Results[i] = &ast.CallExpr{Fun: ast.NewIdent("resultCopyElements"), Args: []ast.Expr{e}}
								stats["collection_transfers"]++
							}
						}
					}
					return true
				})
			}
		}
		if stats["generic_write_barriers"] != 3 {
			return fmt.Errorf("collection write boundary changed: got %d", stats["generic_write_barriers"])
		}
	}
	if mode == "ownership" {
		// All translated bodies and closures get stack roots. Reflection walks the
		// current values, so overwrite/removal is immediately visible without a
		// stale reference count. Address identity deduplicates container aliases.
		for _, file := range all {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				if file != f {
					continue
				} // runtime has no TS locals; tracked through its objects
				site := sources[fn.Name.Name]
				if site == "" {
					continue // Go dispatch scaffolding borrows arguments; TS bodies own roots.
				}
				instrumentBody(fn.Body, fn.Type, fn.Recv, site, fs, info, stats, locals[fn.Name.Name], isResult)
			}
		}
		// Store provenance follows the actual container after every mutation.
		for _, decl := range parsed["runtime.go"].Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil {
				continue
			}
			switch fn.Name.Name {
			case "push", "put", "set", "unshift", "concat", "slice0", "slice1", "slice2", "splice1", "splice1_view", "splice2", "splice3", "pop", "shift", "reverse", "delete", "add", "copy":
				receiver := fn.Recv.List[0].Names[0].Name
				extra := ""
				if fn.Name.Name == "push" {
					extra = ",v"
				}
				if fn.Name.Name == "put" {
					extra = ",i,v"
				}
				if fn.Name.Name == "set" {
					extra = ",k,v"
				}
				if fn.Name.Name == "put" && !collectionReceiver(fn) {
					continue
				}
				fn.Body.List = append(code("defer resultStored("+receiver+",resultSite,"+quoted(fn.Name.Name)+extra+")"), fn.Body.List...)
				stats["container_origins"]++
			}
		}
		// Static roots retain the actual current value (including fields/containers).
		var globals strings.Builder
		globals.WriteString("package main\nfunc init(){\n")
		var shapes map[string]map[string]bool
		_ = json.Unmarshal([]byte(files["result-debug-shapes.json"]), &shapes)
		for _, decl := range f.Decls {
			g, ok := decl.(*ast.GenDecl)
			if !ok || g.Tok != token.VAR {
				continue
			}
			for _, spec := range g.Specs {
				v := spec.(*ast.ValueSpec)
				for _, id := range v.Names {
					if id.Name == "goSources" {
						continue
					}
					if keep, known := shapes["__statics"][id.Name]; known && !keep {
						continue
					}
					if object := info.Defs[id]; object != nil {
						if _, scalar := object.Type().Underlying().(*types.Basic); scalar {
							continue
						}
						if kind := types.TypeString(object.Type(), func(*types.Package) string { return "" }); kind == "*classDescriptor" || kind == "classDescriptor" {
							continue
						}
					}
					fmt.Fprintf(&globals, "resultGlobals[%q]=func()any{return %s}\n", id.Name, id.Name)
				}
			}
		}
		globals.WriteString("}\n")
		files["result_globals.go"] = globals.String()
		// Check inside the concrete mutator body: covers direct and virtual calls.
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			m, ok := mutators[fn.Name.Name]
			if !ok {
				continue
			}
			check := code("resultCheck(self," + quoted(m) + ")")
			fn.Body.List = append(check, fn.Body.List...)
			stats["mutator_bodies"]++
		}
		if stats["mutator_bodies"] != 3 {
			return fmt.Errorf("expected 3 Result mutators, found %d", stats["mutator_bodies"])
		}
	}
	for name, file := range parsed {
		var out bytes.Buffer
		if err := format.Node(&out, fs, file); err != nil {
			return err
		}
		files[name] = out.String()
	}
	runtime := strings.ReplaceAll(debugRuntime, "@OBJ@", obj)
	shapes := files["result-debug-shapes.json"]
	if shapes == "" {
		shapes = "{}"
	}
	runtime = strings.ReplaceAll(runtime, "@SHAPES@", strconv.Quote(shapes))
	if mode == "value" {
		runtime = strings.ReplaceAll(valueRuntime, "@OBJ@", obj)
	}
	formatted, err := format.Source([]byte(runtime))
	if err != nil {
		return fmt.Errorf("debug runtime: %w", err)
	}
	files["result_debug.go"] = string(formatted)
	// main owns diagnostic publication, including refused corpora.
	main := parsed["main.go"]
	if mode == "ownership" && main != nil {
		ast.Inspect(main, func(x ast.Node) bool {
			if call, ok := x.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Exit" {
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == "os" {
						call.Fun = ast.NewIdent("resultExit")
					}
				}
			}
			return true
		})
		for _, decl := range main.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "main" {
				fn.Body.List = append(code("defer resultReport()"), fn.Body.List...)
			}
		}
		var out bytes.Buffer
		_ = format.Node(&out, fs, main)
		files["main.go"] = out.String()
	}
	b, _ := json.MarshalIndent(stats, "", "  ")
	files["result-instrumentation.json"] = string(b) + "\n"
	return nil
}
func copyExpr(e ast.Expr) ast.Expr {
	return &ast.CallExpr{Fun: ast.NewIdent("resultCopy"), Args: []ast.Expr{e}}
}

// Local roots have lexical lifetimes. Slots retain their first registration
// site and read their current value; conservative last-use handling is explicit.
func instrumentBody(body *ast.BlockStmt, typ *ast.FuncType, recv *ast.FieldList, site string, fs *token.FileSet, info *types.Info, stats map[string]int, semantic map[string]gohir.ResultDebugLocal, isResult func(types.Type) bool) {
	// Compute last lexical read, ignoring the emitter's dummy unused-variable
	// assignments. Any read in a loop remains live through its back edge.
	last := map[types.Object]token.Pos{}
	ast.Inspect(body, func(x ast.Node) bool {
		if a, ok := x.(*ast.AssignStmt); ok && len(a.Lhs) == 1 {
			if id, ok := a.Lhs[0].(*ast.Ident); ok && id.Name == "_" {
				return false
			}
		}
		if id, ok := x.(*ast.Ident); ok {
			if obj := info.Uses[id]; obj != nil && id.Pos() > last[obj] {
				last[obj] = id.Pos()
			}
		}
		return true
	})
	ast.Inspect(body, func(x ast.Node) bool {
		loop, ok := x.(*ast.ForStmt)
		if !ok {
			return true
		}
		ast.Inspect(loop, func(x ast.Node) bool {
			if id, ok := x.(*ast.Ident); ok {
				if obj := info.Uses[id]; obj != nil && loop.End() > last[obj] {
					last[obj] = loop.End()
				}
			}
			return true
		})
		return true
	})
	prefix := code("resultFrame:=resultEnter(" + quoted(site) + ");defer resultLeave(resultFrame)")
	addRoot := func(id *ast.Ident, scope int) []ast.Stmt {
		if id.Name == "_" || id.Name == "result" {
			return nil
		}
		obj := info.Defs[id]
		if obj == nil {
			return nil
		}
		// A receiver/temporary is a borrow, not an extra TS holder. Named TS
		// locals, parameters and arrays held for traversal are ordinary roots.
		meta, real := semantic[id.Name]
		if real && !meta.Root {
			return nil
		}
		if isResult(obj.Type()) && (!real || meta.Borrow || id.Name == "self") {
			return nil
		}
		switch obj.Type().Underlying().(type) {
		case *types.Basic, *types.Signature:
			return nil
		}
		name := id.Name
		created := fmt.Sprintf("%s [Go:%d]", site, fs.Position(id.Pos()).Line)
		if real {
			name = meta.Name
			if meta.Source != "" {
				created = meta.Source
			}
		}
		end := last[obj]
		if end == 0 {
			return nil
		}
		stats["roots"]++
		return code(fmt.Sprintf("resultRootValue(resultFrame,%q,&%s,%q,%d,%d)", id.Name+"["+name+"]", id.Name, created, end, scope))
	}
	addParams := func(fields *ast.FieldList) {
		if fields == nil {
			return
		}
		for _, field := range fields.List {
			for _, id := range field.Names {
				prefix = append(prefix, addRoot(id, 0)...)
			}
		}
	}
	addParams(typ.Params)
	addParams(recv)
	var walk func(*ast.BlockStmt)
	walk = func(block *ast.BlockStmt) {
		scope := int(block.Pos())
		var out []ast.Stmt
		out = append(out, code(fmt.Sprintf("resultClear(resultFrame,%d)", scope))...)
		for _, stmt := range block.List {
			// Capture values are published conservatively. Generated closure fields
			// are also visited normally when represented as ordinary objects.
			var captures []string
			ast.Inspect(stmt, func(x ast.Node) bool {
				switch x := x.(type) {
				case *ast.FuncLit:
					seen := map[string]bool{}
					ast.Inspect(x.Body, func(z ast.Node) bool {
						if id, ok := z.(*ast.Ident); ok {
							if obj := info.Uses[id]; obj != nil && obj.Pos() < x.Pos() {
								if variable, ok := obj.(*types.Var); ok && !variable.IsField() && !seen[id.Name] {
									switch obj.Type().Underlying().(type) {
									case *types.Basic, *types.Signature:
									default:
										captures = append(captures, "resultPublish("+id.Name+","+quoted(site+" closure capture")+")")
										seen[id.Name] = true
									}
								}
							}
						}
						return true
					})
					instrumentBody(x.Body, x.Type, nil, site+" closure", fs, info, stats, semantic, isResult)
					return false
				case *ast.BlockStmt:
					walk(x)
					return false
				}
				return true
			})
			out = append(out, code(fmt.Sprintf("resultAt(resultFrame,%d)", stmt.Pos()))...)
			for _, capture := range captures {
				out = append(out, code(capture)...)
				stats["published_captures"]++
			}
			out = append(out, stmt)
			if assign, ok := stmt.(*ast.AssignStmt); ok {
				for _, lhs := range assign.Lhs {
					if _, ok := lhs.(*ast.SelectorExpr); ok {
						if isResult(info.TypeOf(lhs)) {
							out = append(out, code("resultFieldStored(&"+textOf(lhs)+",resultSite)")...)
							stats["field_origins"]++
						}
					}
				}
			}
			var ids []*ast.Ident
			if d, ok := stmt.(*ast.DeclStmt); ok {
				if g, ok := d.Decl.(*ast.GenDecl); ok && g.Tok == token.VAR {
					for _, s := range g.Specs {
						ids = append(ids, s.(*ast.ValueSpec).Names...)
					}
				}
			}
			if a, ok := stmt.(*ast.AssignStmt); ok && a.Tok == token.DEFINE {
				for _, e := range a.Lhs {
					if id, ok := e.(*ast.Ident); ok && info.Defs[id] != nil {
						ids = append(ids, id)
					}
				}
			}
			for _, id := range ids {
				out = append(out, addRoot(id, scope)...)
			}
		}
		if len(block.List) == 0 || !resultTerminating(block.List[len(block.List)-1]) {
			out = append(out, code(fmt.Sprintf("resultClear(resultFrame,%d)", scope))...)
		}
		block.List = out
	}
	walk(body)
	body.List = append(prefix, body.List...)
}

func collectionReceiver(fn *ast.FuncDecl) bool {
	if fn.Recv == nil {
		return false
	}
	found := false
	ast.Inspect(fn.Recv, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok && (id.Name == "array" || id.Name == "orderedMap") {
			found = true
		}
		return true
	})
	return found
}

func resultTerminating(stmt ast.Stmt) bool {
	switch x := stmt.(type) {
	case *ast.ReturnStmt, *ast.BranchStmt:
		return true
	case *ast.BlockStmt:
		return len(x.List) > 0 && resultTerminating(x.List[len(x.List)-1])
	case *ast.ExprStmt:
		if c, ok := x.X.(*ast.CallExpr); ok {
			if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "panic" {
				return true
			}
		}
	case *ast.IfStmt:
		return x.Else != nil && resultTerminating(x.Body) && resultTerminating(x.Else)
	case *ast.ForStmt:
		return x.Cond == nil
	}
	return false
}
