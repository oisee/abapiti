package tsfront

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

type ExternModule struct {
	File     string `json:"file"`
	GoImport string `json:"goImport"`
	GoDir    string `json:"goDir,omitempty"`
	// ABAPClass binds the module to static methods of a global ABAP class.
	// Without it, ABAP emission refuses calls into the module.
	ABAPClass string `json:"abapClass,omitempty"`
}

func ReadExterns(file string) ([]ExternModule, error) {
	if file == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var config struct {
		Modules []ExternModule `json:"modules"`
	}
	if err = json.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, m := range config.Modules {
		if m.File == "" || filepath.IsAbs(m.File) || m.GoImport == "" || strings.ContainsAny(m.GoImport, "\"\n\r\t ") || seen[filepath.Clean(m.File)] || m.ABAPClass != "" && !abapClassName.MatchString(m.ABAPClass) {
			return nil, fmt.Errorf("invalid or duplicate extern module %q", m.File)
		}
		seen[filepath.Clean(m.File)] = true
	}
	return config.Modules, nil
}

var abapClassName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,29}$`)

func ExternGoMod(module string, modules []ExternModule, configDir string) string {
	s := "module " + module + "\n\ngo 1.26.0\n"
	seen := map[string]bool{}
	for _, m := range modules {
		if seen[m.GoImport] {
			continue
		}
		seen[m.GoImport] = true
		s += "\nrequire " + m.GoImport + " v0.0.0\n"
		if m.GoDir != "" {
			dir := m.GoDir
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(configDir, dir)
			}
			s += fmt.Sprintf("replace %s => %q\n", m.GoImport, dir)
		}
	}
	return s
}

func (l *lowerer) prepareExterns(files []string, modules []ExternModule) ([]string, error) {
	l.externs = map[string]ExternModule{}
	l.externFunctions = map[*ast.Node]*hir.ExternSignature{}
	for _, m := range modules {
		f, ok := l.prog.File(filepath.Join(l.prog.configDir, m.File))
		if !ok {
			return nil, fmt.Errorf("extern file %s is not part of the program", m.File)
		}
		l.externs[f.FileName()] = m
		ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		for _, n := range f.Statements.Nodes {
			if n.Kind != ast.KindFunctionDeclaration || n.ModifierFlags()&ast.ModifierFlagsExport == 0 {
				continue
			}
			sig := &hir.ExternSignature{}
			if m.ABAPClass != "" {
				sig.ABAPClass = strings.ToLower(m.ABAPClass)
			}
			valid := n.Name() != nil && n.Type() != nil && n.AsFunctionDeclaration().TypeParameters == nil
			if n.Type() != nil {
				valid = l.externType(n.Type(), true) && valid
			}
			for i, p := range n.Parameters() {
				rest := p.AsParameterDeclaration().DotDotDotToken != nil
				if p.Type() == nil || !l.externType(p.Type(), false) || p.AsParameterDeclaration().QuestionToken != nil || p.Initializer() != nil {
					valid = false
					continue
				}
				t := l.mapTypeNode(p.Type())
				sig.Params = append(sig.Params, t)
				if sig.ABAPClass != "" {
					if p.Name() == nil || p.Name().Kind != ast.KindIdentifier {
						valid = false
						continue
					}
					sig.ParamNames = append(sig.ParamNames, p.Name().Text())
				}
				if rest {
					sig.Rest = true
					if i != len(n.Parameters())-1 || t.Kind != hir.Array || t.Args[0].Kind != hir.String {
						valid = false
					}
				}
			}
			if !valid {
				l.diagf(n, "unsupported-extern-signature", "unsupported extern signature for %s", externDeclName(n))
			} else {
				l.externFunctions[n] = sig
			}
		}
		done()
	}
	out := []string{}
	for _, name := range files {
		f, ok := l.prog.File(name)
		if !ok {
			return nil, fmt.Errorf("file %s is not part of the program", name)
		}
		if _, ok = l.externs[f.FileName()]; !ok {
			out = append(out, name)
		}
	}
	l.checkExternNamespaces(out)
	return out, nil
}

func (l *lowerer) externType(n *ast.Node, result bool) bool {
	// Only the public ABI types are admitted, before general type mapping.
	switch n.Kind {
	case ast.KindStringKeyword, ast.KindBooleanKeyword, ast.KindNumberKeyword:
		return true
	case ast.KindVoidKeyword:
		return result
	case ast.KindUnionType:
		ts := n.AsUnionTypeNode().Types.Nodes
		return result && len(ts) == 2 && ((ts[0].Kind == ast.KindStringKeyword && ts[1].Kind == ast.KindUndefinedKeyword) || (ts[1].Kind == ast.KindStringKeyword && ts[0].Kind == ast.KindUndefinedKeyword))
	case ast.KindArrayType:
		el := n.AsArrayTypeNode().ElementType
		if el.Kind == ast.KindStringKeyword {
			return true
		}
		if el.Kind == ast.KindTupleType {
			es := el.AsTupleTypeNode().Elements.Nodes
			return len(es) == 2 && es[0].Kind == ast.KindStringKeyword && es[1].Kind == ast.KindBooleanKeyword
		}
	}
	return false
}

func (l *lowerer) externDeclaration(n *ast.Node) (*ast.Node, bool) {
	if n == nil {
		return nil, false
	}
	s := l.resolve(n)
	if s == nil {
		return nil, false
	}
	d := s.ValueDeclaration
	if d == nil {
		for _, decl := range s.Declarations {
			if decl.Kind == ast.KindSourceFile {
				d = decl
				break
			}
		}
	}
	if d == nil {
		return nil, false
	}
	if d.Kind == ast.KindPropertySignature || d.Kind == ast.KindMethodSignature {
		// A member of an exported data interface (host-io.ts DirectoryEntry's
		// name) is read off the caller's own object, not an extern export.
		return nil, false
	}
	f := ast.GetSourceFileOfNode(d)
	if f == nil {
		return nil, false
	}
	_, ok := l.externs[f.FileName()]
	return d, ok
}
func (l *lowerer) externValue(n *ast.Node) bool {
	if n.Kind != ast.KindIdentifier && n.Kind != ast.KindPropertyAccessExpression {
		return false
	}
	if _, ok := l.externDeclaration(n); ok {
		l.diagf(n, "unsupported-extern-use", "extern exports may only be called as exported functions")
		return true
	}
	return false
}
func (l *lowerer) externCall(n *ast.Node) (*hir.Expr, bool) {
	c := n.AsCallExpression()
	d, ok := l.externDeclaration(c.Expression)
	if !ok {
		return nil, false
	}
	sig := l.externFunctions[d]
	if sig == nil {
		l.diagf(n, "unsupported-extern-use", "extern export is not a supported exported function")
		return nil, true
	}
	f := ast.GetSourceFileOfNode(d)
	m := l.externs[f.FileName()]
	name := d.Name().Text()
	r, size := utf8.DecodeRuneInString(name)
	name = string(unicode.ToUpper(r)) + name[size:]
	min := len(sig.Params)
	if sig.Rest {
		min--
	}
	if len(c.Arguments.Nodes) < min || !sig.Rest && len(c.Arguments.Nodes) != min {
		l.diagf(n, "unsupported-extern-use", "extern call argument count does not match declaration")
		return nil, true
	}
	args := []*hir.Expr{}
	copySig := *sig
	sig = &copySig
	for i, a := range c.Arguments.Nodes {
		if a.Kind == ast.KindSpreadElement {
			if !sig.Rest || i != len(c.Arguments.Nodes)-1 {
				l.diagf(a, "unsupported-extern-use", "extern spread must be the final rest argument")
				return nil, true
			}
			sig.Spread = true
			a = a.Expression()
		}
		x := l.expr(a)
		if x == nil {
			return nil, true
		}
		args = append(args, x)
	}
	savedFile, savedCK := l.file, l.ck
	ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
	l.file, l.ck = f, ck
	t := l.mapTypeNode(d.Type())
	l.file, l.ck = savedFile, savedCK
	done()
	l.builtinError()
	return &hir.Expr{Kind: hir.ExternCall, Node: l.node(n), Owner: m.GoImport, Name: name, Args: args, Type: t, Extern: sig}, true
}

func (p *numberPass) externRangeArgs(e *hir.Expr, info *numberMethod) {
	for i, a := range e.Args {
		j := i
		if j >= len(e.Extern.Params) {
			j = len(e.Extern.Params) - 1
		}
		t := e.Extern.Params[j]
		if e.Extern.Rest && j == len(e.Extern.Params)-1 && !(e.Extern.Spread && i == len(e.Args)-1) {
			t = t.Args[0]
		}
		e.Args[i] = rangeConversion(a, t, info.exprs[a])
	}
}

func externDeclName(n *ast.Node) string {
	if n.Name() != nil {
		return n.Name().Text()
	}
	return "<anonymous>"
}
func (l *lowerer) assumeExtern(e *hir.Expr) *hir.Expr {
	for i, a := range e.Args {
		if numberType(a.Type) {
			e.Args[i] = integerFloat(a)
		}
	}
	if e.Type.Kind == hir.Number {
		return checkedInteger(e, hir.T(hir.I64))
	}
	return e
}

func (l *lowerer) checkExternNamespaces(files []string) {
	if len(l.externs) == 0 {
		return
	}
	for _, file := range files {
		f, _ := l.prog.File(file)
		ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		namespaces := map[*ast.Symbol]bool{}
		for _, n := range f.Statements.Nodes {
			if n.Kind != ast.KindImportDeclaration {
				continue
			}
			m := l.moduleOfDeclaration(n)
			if m == nil {
				continue
			}
			if _, ok := l.externs[m.FileName()]; !ok {
				continue
			}
			c := n.AsImportDeclaration().ImportClause
			if c == nil {
				continue
			}
			bindings := c.AsImportClause().NamedBindings
			if bindings != nil && bindings.Kind == ast.KindNamespaceImport {
				namespaces[l.ck.GetSymbolAtLocation(bindings.Name())] = true
			}
		}
		var walk func(*ast.Node)
		walk = func(n *ast.Node) {
			if n.Kind == ast.KindImportDeclaration || n.Kind == ast.KindExportDeclaration {
				return
			}
			if n.Kind == ast.KindIdentifier && namespaces[l.ck.GetSymbolAtLocation(n)] {
				if n.Parent == nil || n.Parent.Kind != ast.KindPropertyAccessExpression || n.Parent.Expression() != n {
					l.diagf(n, "unsupported-extern-use", "extern namespaces may only qualify exported function calls")
				}
			}
			n.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
		}
		walk(f.AsNode())
		done()
	}
}
