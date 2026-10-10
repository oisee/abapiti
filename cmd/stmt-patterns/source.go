package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/core"
	"github.com/oisee/abapiti/internal/tsgo/parser"
	"github.com/oisee/abapiti/tsfront"
)

func countTS(ms []*method, root string) error {
	byFile := map[string][]*method{}
	for _, m := range ms {
		if m.Source != "" {
			file := strings.Split(m.Source, ":")[0]
			byFile[file] = append(byFile[file], m)
		}
	}
	for _, file := range sortedKeys(byFile) {
		data, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if e != nil {
			return e
		}
		abs, e := filepath.Abs(filepath.Join(root, filepath.FromSlash(file)))
		if e != nil {
			return e
		}
		f := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: filepath.ToSlash(abs)}, string(data), core.ScriptKindTS)
		var visit func(*ast.Node) bool
		visit = func(n *ast.Node) bool {
			if n.Kind == ast.KindClassDeclaration {
				class := n.Name().Text()
				for _, member := range n.Members() {
					if member.Kind != ast.KindMethodDeclaration && member.Kind != ast.KindConstructor {
						continue
					}
					name := "constructor"
					if member.Name() != nil {
						name = member.Name().Text()
					}
					if member.Body() == nil {
						continue
					}
					for _, m := range byFile[file] {
						if strings.HasSuffix(m.ID, "."+class) && (m.Name == name || (strings.HasSuffix(m.Name, "_one") && name == strings.TrimSuffix(m.Name, "_one"))) {
							m.TS, m.Ops = tsCounts(member.Body())
						}
					}
				}
			}
			n.ForEachChild(visit)
			return false
		}
		visit(f.AsNode())
	}
	return validateMethods(ms)
}

// Statements exclude blocks. Operations are AST computations, not all leaf
// expressions: calls/new, member/index reads, operators, casts and literals
// that create arrays/objects. Nested function bodies count as static source.
func tsCounts(root *ast.Node) (int, int) {
	s, o := 0, 0
	var walk func(*ast.Node) bool
	walk = func(n *ast.Node) bool {
		if ast.IsStatement(n) && n.Kind != ast.KindBlock {
			s++
		}
		switch n.Kind {
		case ast.KindCallExpression, ast.KindNewExpression, ast.KindPropertyAccessExpression, ast.KindElementAccessExpression, ast.KindBinaryExpression, ast.KindPrefixUnaryExpression, ast.KindPostfixUnaryExpression, ast.KindConditionalExpression, ast.KindAsExpression, ast.KindTypeAssertionExpression, ast.KindArrayLiteralExpression, ast.KindObjectLiteralExpression:
			o++
		}
		n.ForEachChild(walk)
		return false
	}
	walk(root)
	return s, o
}
func countHIR(ms []*method, source, work, input, output string) error {
	// Copy only fingerprinted closure files; all generated material stays in work.
	closure := tsfront.EmbeddedRegistryClosure()
	for _, file := range closure.Files() {
		b, e := os.ReadFile(filepath.Join(source, filepath.FromSlash(file)))
		if e != nil {
			return e
		}
		target := filepath.Join(work, filepath.FromSlash(file))
		if e = os.MkdirAll(filepath.Dir(target), 0755); e != nil {
			return e
		}
		if e = os.WriteFile(target, b, 0644); e != nil {
			return e
		}
	}
	if e := closure.Verify(work); e != nil {
		return e
	}
	for _, pkg := range tsfront.RegistryNodePackages() {
		if e := pkg.WriteEmbedded(filepath.Join(work, "node_modules", pkg.Name)); e != nil {
			return e
		}
	}
	target := filepath.Join(work, tsfront.RegistryHarnessPath)
	if e := os.MkdirAll(filepath.Dir(target), 0755); e != nil {
		return e
	}
	if e := os.WriteFile(target, tsfront.RegistryHarness(), 0644); e != nil {
		return e
	}
	config := `{"compilerOptions":{"module":"commonjs","target":"es2020","lib":["es2020"],"noEmit":true,"skipLibCheck":true,"strictNullChecks":true,"strictFunctionTypes":true,"noImplicitAny":true,"strictPropertyInitialization":false},"include":["src/**/*.ts","harness/**/*.ts"]}`
	if e := os.WriteFile(filepath.Join(work, "tsconfig.json"), []byte(config), 0644); e != nil {
		return e
	}
	if e := os.Setenv("ABAPITI_ASSUME_INT", "1"); e != nil {
		return e
	}
	overrides, e := tsfront.RegistryOverrides()
	if e != nil {
		return e
	}
	coverage, e := tsfront.EmbeddedReachability()
	if e != nil {
		return e
	}
	l, e := tsfront.LowerRegistry(work, append(closure.Files(), tsfront.RegistryHarnessPath), overrides, coverage)
	if e != nil {
		return e
	}
	if e = l.Err(); e != nil {
		return e
	}
	emitted, _, e := l.Emit()
	if e != nil {
		return e
	}
	// Require the same emitted classes, so HIR counts cannot silently describe
	// another optimisation mode or a stale input build.
	for _, m := range ms {
		file := strings.ToLower(m.Class) + ".clas.abap"
		data, e := os.ReadFile(filepath.Join(input, "classes", file))
		if e != nil {
			return e
		}
		if string(data) != emitted[file] {
			return fmt.Errorf("HIR re-emission differs from input class %s", m.Class)
		}
	}
	found := map[string]bool{}
	hot := &hir.Program{}
	for _, c := range l.Prog.Classes {
		chosen := &hir.Class{Name: c.Name, Super: c.Super}
		for _, hm := range append(append([]*hir.Method{}, c.Methods...), c.Ctor) {
			if hm == nil {
				continue
			}
			for _, m := range ms {
				if m.ID == c.Name && m.Name == hm.Name {
					m.HIR, m.HIROps = hirCounts(hm.Body)
					found[identity(m)] = true
					if m.Name == "run" || m.Name == "run_one" || m.Name == "remainingLength" || m.Name == "peek" {
						chosen.Methods = append(chosen.Methods, hm)
					}
				}
			}
		}
		if len(chosen.Methods) > 0 {
			hot.Classes = append(hot.Classes, chosen)
		}
	}
	for _, m := range ms {
		if !found[identity(m)] {
			return fmt.Errorf("missing HIR method %s", identity(m))
		}
	}
	if e = os.MkdirAll(output, 0755); e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(output, "hir-hot.txt"), []byte(hir.Dump(hot)), 0644)
}
func hirCounts(root *hir.Stmt) (int, int) {
	s, o := 0, 0
	var stmt func(*hir.Stmt)
	var expr func(*hir.Expr)
	expr = func(e *hir.Expr) {
		if e == nil {
			return
		}
		switch e.Kind {
		case hir.Lit, hir.Local, hir.This:
		default:
			o++
		}
		expr(e.X)
		expr(e.Y)
		expr(e.Z)
		for _, a := range e.Args {
			expr(a)
		}
		stmt(e.Stmt)
	}
	stmt = func(n *hir.Stmt) {
		if n == nil {
			return
		}
		if n.Kind != hir.Block {
			s++
		}
		expr(n.X)
		expr(n.Y)
		stmt(n.Body)
		stmt(n.Else)
		for _, child := range n.List {
			stmt(child)
		}
	}
	stmt(root)
	return s, o
}
