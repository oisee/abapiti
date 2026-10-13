package abapfixture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// A new ABAP emitter test must use the collecting helper, not silently bypass
// the compile gate. Go-target Emit calls are deliberately not ABAP fixtures.
func TestABAPEmitterTestsUseCollectingHelpers(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, pkg := range []string{"hir/abap", "hir/golang", "tsfront", "llvm", "ts"} {
		dir := filepath.Join(root, pkg)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, "_test.go") || name == "fixture_export_test.go" {
				continue
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			aliases := map[string]bool{}
			for _, spec := range file.Imports {
				path, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				if path == "github.com/oisee/abapiti/hir/abap" {
					alias := "abap"
					if spec.Name != nil {
						alias = spec.Name.Name
					}
					aliases[alias] = true
				}
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				method := ""
				switch fn := call.Fun.(type) {
				case *ast.Ident:
					if pkg == "hir/abap" || aliases["."] ||
						(pkg == "tsfront" && fn.Name == "EmitAbaplintTestsOSG") ||
						(pkg == "llvm" && fn.Name == "Compile") ||
						(pkg == "ts" && fn.Name == "Transpile") {
						method = fn.Name
					}
				case *ast.SelectorExpr:
					if owner, ok := fn.X.(*ast.Ident); ok && aliases[owner.Name] {
						method = fn.Sel.Name
					}
				}
				if method == "Emit" || method == "EmitNamed" || method == "EmitTestNamed" ||
					method == "EmitAbaplintTestsOSG" || method == "Compile" || method == "Transpile" {
					t.Errorf("%s: use a fixture-export helper for %s", fset.Position(call.Pos()), method)
				}
				return true
			})
		}
	}
}
