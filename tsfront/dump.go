package tsfront

import (
	"context"
	"fmt"
	"strings"

	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
)

// FileDump is the deterministic dump of one source file: its classes, its
// named functions, and one entry per expression node in source order.
type FileDump struct {
	File        string      `json:"file"`
	Classes     []ClassDump `json:"classes,omitempty"`
	Functions   []FuncDump  `json:"functions,omitempty"`
	Expressions []ExprDump  `json:"expressions,omitempty"`
}

// HeritageDump names one heritage entry (extends/implements) with the
// location of the declaration it resolves to.
type HeritageDump struct {
	Name string `json:"name"`
	Decl string `json:"decl,omitempty"` // declaration, file:line:col
}

// ClassDump describes a class declaration.
type ClassDump struct {
	Name       string         `json:"name"`
	Abstract   bool           `json:"abstract,omitempty"`
	Extends    []HeritageDump `json:"extends,omitempty"`
	Implements []HeritageDump `json:"implements,omitempty"`
	Members    []MemberDump   `json:"members"`
	Loc        string         `json:"loc"`
}

// MemberDump describes a class member.
type MemberDump struct {
	Kind         string      `json:"kind"` // property, method, constructor, get-accessor, set-accessor
	Name         string      `json:"name"`
	Static       bool        `json:"static,omitempty"`
	Abstract     bool        `json:"abstract,omitempty"`
	Visibility   string      `json:"visibility,omitempty"` // "", public, protected, private
	Optional     bool        `json:"optional,omitempty"`   // members/param properties with ?; parameters with ? or default
	DeclaredType string      `json:"declaredType,omitempty"`
	Parameters   []ParamDump `json:"parameters,omitempty"`
	ReturnType   string      `json:"returnType,omitempty"`
	Loc          string      `json:"loc"`
}

// FuncDump describes a named function declaration.
type FuncDump struct {
	Name       string      `json:"name"`
	Kind       string      `json:"kind"` // function
	Parameters []ParamDump `json:"parameters,omitempty"`
	ReturnType string      `json:"returnType,omitempty"`
	Loc        string      `json:"loc"`
}

// ParamDump describes a parameter; Type is the checker type.
type ParamDump struct {
	Name     string `json:"name"`
	Optional bool   `json:"optional,omitempty"`
	Rest     bool   `json:"rest,omitempty"`
	Type     string `json:"type,omitempty"`
}

// ExprDump describes one expression node: kind, span, checker type, coarse
// type flags and, where applicable, symbol resolution.
type ExprDump struct {
	Kind    string   `json:"kind"`
	Span    string   `json:"span"` // file:line:col
	Type    string   `json:"type,omitempty"`
	Flags   []string `json:"typeFlags,omitempty"`
	Symbol  string   `json:"symbol,omitempty"`     // identifiers: resolved symbol name
	SymDecl string   `json:"symbolDecl,omitempty"` // identifiers: file:line:col of the declaration
	// Property accesses on class-typed receivers:
	DeclaringClass string `json:"declaringClass,omitempty"` // class declaring the member
	Inherited      bool   `json:"inherited,omitempty"`      // true if not the receiver's own class
}

// Dump produces the deterministic dump for each named source file. Files that
// are not part of the program produce an error. The checker runs one file at
// a time; all slices are in source order, so repeated runs of the same
// program produce byte-identical JSON.
func (p *Program) Dump(files []string) ([]FileDump, error) {
	out := make([]FileDump, 0, len(files))
	ctx := context.Background()
	for _, name := range files {
		f, ok := p.File(name)
		if !ok {
			return nil, fmt.Errorf("file %s is not part of the program", name)
		}
		ck, done := p.prog.GetTypeCheckerForFile(ctx, f)
		d := dumper{program: p, checker: ck, file: f}
		d.walkFile(f)
		done()
		out = append(out, d.result)
	}
	return out, nil
}

type dumper struct {
	program *Program
	checker *checker.Checker
	file    *ast.SourceFile
	result  FileDump
}

func (d *dumper) walkFile(f *ast.SourceFile) {
	d.result = FileDump{File: f.FileName()}
	walk(f.AsNode(), d.visit)
}

func (d *dumper) visit(node *ast.Node) {
	switch node.Kind {
	case ast.KindClassDeclaration, ast.KindClassExpression:
		if node.Name() != nil {
			d.result.Classes = append(d.result.Classes, d.dumpClass(node))
		}
	case ast.KindFunctionDeclaration:
		if node.Name() != nil {
			d.result.Functions = append(d.result.Functions, d.dumpFunction(node))
		}
	}
	if ast.IsExpressionNode(node) {
		d.result.Expressions = append(d.result.Expressions, d.dumpExpr(node))
	}
}

// walk calls f pre-order on node and its children.
func walk(node *ast.Node, f func(*ast.Node)) {
	f(node)
	node.ForEachChild(func(child *ast.Node) bool {
		walk(child, f)
		return false // visit all siblings
	})
}

func (d *dumper) dumpClass(node *ast.Node) ClassDump {
	class := ClassDump{
		Name: node.Name().Text(),
		Loc:  d.locOf(node),
	}
	modifiers := node.ModifierFlags()
	class.Abstract = modifiers&ast.ModifierFlagsAbstract != 0
	if heritage := node.ClassLikeData().HeritageClauses; heritage != nil {
		for _, clauseNode := range heritage.Nodes {
			clause := clauseNode.AsHeritageClause()
			target := &class.Implements
			if clause.Token == ast.KindExtendsKeyword {
				target = &class.Extends
			}
			if clause.Types == nil {
				continue
			}
			for _, t := range clause.Types.Nodes {
				*target = append(*target, d.dumpHeritage(t))
			}
		}
	}
	for _, m := range node.Members() {
		if md, ok := d.dumpMember(m); ok {
			class.Members = append(class.Members, md)
			// Constructor parameter properties ("constructor(public x: number)")
			// are class properties declared by the constructor's parameters.
			if m.Kind == ast.KindConstructor {
				class.Members = append(class.Members, d.dumpParamProperties(m)...)
			}
		}
	}
	return class
}

func (d *dumper) dumpHeritage(node *ast.Node) HeritageDump {
	h := HeritageDump{}
	expr := node.Expression()
	if expr != nil {
		switch expr.Kind {
		case ast.KindIdentifier, ast.KindPrivateIdentifier, ast.KindStringLiteral, ast.KindNumericLiteral, ast.KindBigIntLiteral:
			if text := expr.Text(); text != "" {
				h.Name = text
			}
		}
		if sym := d.resolveSymbol(expr); sym != nil && sym.Name != "" {
			h.Name = symbolName(sym)
			h.Decl = d.symbolDecl(sym)
		}
	}
	return h
}

func (d *dumper) dumpMember(node *ast.Node) (MemberDump, bool) {
	m := MemberDump{Loc: d.locOf(node)}
	switch node.Kind {
	case ast.KindPropertyDeclaration:
		m.Kind = "property"
	case ast.KindMethodDeclaration:
		m.Kind = "method"
	case ast.KindConstructor:
		m.Kind = "constructor"
	case ast.KindGetAccessor:
		m.Kind = "get-accessor"
	case ast.KindSetAccessor:
		m.Kind = "set-accessor"
	default:
		return MemberDump{}, false // index signatures and friends
	}
	if node.Kind == ast.KindConstructor {
		m.Name = "constructor" // constructors have no name node
	} else if node.Name() != nil {
		m.Name = memberName(node.Name())
	}
	modifiers := node.ModifierFlags()
	m.Static = modifiers&ast.ModifierFlagsStatic != 0
	m.Abstract = modifiers&ast.ModifierFlagsAbstract != 0
	// `?` marks a property or method member optional (constructors and
	// accessors cannot carry one, so their token is simply nil).
	m.Optional = node.QuestionToken() != nil
	if name := node.Name(); name != nil && name.Kind == ast.KindPrivateIdentifier {
		m.Visibility = "private" // #private fields and accessors
	}
	switch {
	case modifiers&ast.ModifierFlagsPrivate != 0:
		m.Visibility = "private"
	case modifiers&ast.ModifierFlagsProtected != 0:
		m.Visibility = "protected"
	case modifiers&ast.ModifierFlagsPublic != 0:
		m.Visibility = "public"
	}
	switch node.Kind {
	case ast.KindPropertyDeclaration:
		if sym := node.Symbol(); sym != nil {
			m.DeclaredType = d.typeString(d.checker.GetTypeOfSymbol(sym))
		}
	default:
		d.dumpSignature(node, &m.Parameters, &m.ReturnType)
	}
	return m, true
}

// dumpParamProperties dumps the constructor parameters that are parameter
// properties (they carry an accessibility or readonly modifier) as the class
// property members they declare.
func (d *dumper) dumpParamProperties(ctor *ast.Node) []MemberDump {
	var out []MemberDump
	for _, p := range ctor.Parameters() {
		if !ast.IsParameterPropertyDeclaration(p, ctor) {
			continue
		}
		m := MemberDump{
			Kind: "property",
			Name: memberName(p.Name()),
			Loc:  d.locOf(p),
			// A default value makes the constructor *parameter* optional, but
			// the declared property stays required (tsc: only `?` makes a
			// property optional); the parameter dump below keeps the default.
			Optional: p.QuestionToken() != nil,
		}
		switch {
		case p.ModifierFlags()&ast.ModifierFlagsPrivate != 0:
			m.Visibility = "private"
		case p.ModifierFlags()&ast.ModifierFlagsProtected != 0:
			m.Visibility = "protected"
		case p.ModifierFlags()&ast.ModifierFlagsPublic != 0:
			m.Visibility = "public"
		}
		if sym := p.Symbol(); sym != nil {
			m.DeclaredType = d.typeString(d.checker.GetTypeOfSymbol(sym))
		}
		out = append(out, m)
	}
	return out
}

func (d *dumper) dumpFunction(node *ast.Node) FuncDump {
	fn := FuncDump{
		Name: node.Name().Text(),
		Kind: "function",
		Loc:  d.locOf(node),
	}
	d.dumpSignature(node, &fn.Parameters, &fn.ReturnType)
	return fn
}

func (d *dumper) dumpSignature(node *ast.Node, params *[]ParamDump, ret *string) {
	sig := d.checker.GetSignatureFromDeclaration(node)
	// Checker parameters correspond to the syntactic parameters minus the
	// explicit `this` parameter, which the checker keeps out of the
	// signature; remember which dumped entries they match.
	checked := []int{}
	for _, p := range node.Parameters() {
		pd := ParamDump{Name: memberName(p.Name())}
		pd.Optional = p.QuestionToken() != nil || p.Initializer() != nil
		pd.Rest = p.AsParameterDeclaration().DotDotDotToken != nil
		if ast.IsThisParameter(p) {
			// typed from the parameter's own annotation
			pd.Type = d.typeString(d.checker.GetTypeAtLocation(p))
		} else {
			checked = append(checked, len(*params))
		}
		*params = append(*params, pd)
	}
	if sig == nil {
		// fall back to syntactic parameter types
		for i := range *params {
			if p := node.Parameters()[i]; p.Type() != nil {
				(*params)[i].Type = d.typeString(d.checker.GetTypeAtLocation(p.Type()))
			}
		}
		return
	}
	for i, psym := range sig.Parameters() {
		if i < len(checked) {
			(*params)[checked[i]].Type = d.typeString(d.checker.GetTypeOfSymbol(psym))
		}
	}
	*ret = d.typeString(d.checker.GetReturnTypeOfSignature(sig))
}

func (d *dumper) dumpExpr(node *ast.Node) ExprDump {
	e := ExprDump{
		Kind: strings.TrimPrefix(node.Kind.String(), "Kind"),
		Span: d.locOf(node),
	}
	if t := d.checker.GetTypeAtLocation(node); t != nil {
		e.Type = d.typeString(t)
		e.Flags = typeFlagNames(t)
	}
	if node.Kind == ast.KindIdentifier || node.Kind == ast.KindPrivateIdentifier {
		if sym := d.resolveSymbol(node); sym != nil {
			e.Symbol = symbolName(sym)
			e.SymDecl = d.symbolDecl(sym)
		}
	}
	if node.Kind == ast.KindPropertyAccessExpression {
		d.fillPropertyAccess(node, &e)
	}
	return e
}

// fillPropertyAccess records, for a property access on a class-typed
// receiver, the accessed member, which class declares it, and whether it is
// inherited.
func (d *dumper) fillPropertyAccess(node *ast.Node, e *ExprDump) {
	sym := d.resolveSymbol(node)
	if sym == nil || sym.Parent == nil || sym.Parent.Name == "" {
		return
	}
	e.Symbol = symbolName(sym)
	e.DeclaringClass = symbolName(sym.Parent)
	receiver := d.checker.GetTypeAtLocation(node.Expression())
	if receiver == nil {
		return
	}
	rsym := receiver.Symbol()
	if rsym == nil {
		return
	}
	e.Inherited = rsym != sym.Parent
}

// symbolName returns a symbol's stable public name. Private-identifier
// symbols (#private members) carry an internal unique ID ("\ufffd#<n>@name")
// whose counter differs between program instances; strip it so dumps stay
// deterministic across fresh loads.
func symbolName(sym *ast.Symbol) string {
	name := sym.Name
	if at := strings.LastIndex(name, "@"); at >= 0 && strings.Contains(name, "#") {
		name = name[at+1:]
	}
	return name
}

// resolveSymbol returns the symbol at node with import aliases resolved to
// their target symbol.
func (d *dumper) resolveSymbol(node *ast.Node) *ast.Symbol {
	sym := d.checker.GetSymbolAtLocation(node)
	if sym == nil {
		return nil
	}
	if sym.Flags&ast.SymbolFlagsAlias != 0 {
		if target, ok := d.checker.ResolveAlias(sym); ok {
			return target
		}
	}
	return sym
}

func (d *dumper) symbolDecl(sym *ast.Symbol) string {
	decl := sym.ValueDeclaration
	if decl == nil && len(sym.Declarations) > 0 {
		decl = sym.Declarations[0]
	}
	if decl == nil {
		return ""
	}
	f := ast.GetSourceFileOfNode(decl)
	if f == nil {
		return ""
	}
	return locString(f, scanner.GetTokenPosOfNode(decl, f, false /*includeJSDoc*/))
}

// locOf renders "file:line:col" at node's first token (skipping leading
// trivia), which is where the node is declared — not node.Pos(), which
// points at the end of the preceding node's trivia.
func (d *dumper) locOf(node *ast.Node) string {
	return locString(d.file, scanner.GetTokenPosOfNode(node, d.file, false /*includeJSDoc*/))
}

func (d *dumper) typeString(t *checker.Type) string {
	if t == nil {
		return ""
	}
	return d.checker.TypeToString(t)
}

// typeFlagNames reduces a checker type to coarse, HIR-relevant flags.
func typeFlagNames(t *checker.Type) []string {
	flags := t.Flags()
	var out []string
	add := func(name string) { out = append(out, name) }
	switch {
	case flags&checker.TypeFlagsAny != 0:
		add("any")
	case flags&checker.TypeFlagsUnknown != 0:
		add("unknown")
	case flags&checker.TypeFlagsUndefined != 0:
		add("undefined")
	case flags&checker.TypeFlagsNull != 0:
		add("null")
	case flags&(checker.TypeFlagsNumber|checker.TypeFlagsNumberLiteral) != 0:
		add("number")
	case flags&checker.TypeFlagsStringLike != 0:
		add("string")
	case flags&(checker.TypeFlagsBoolean|checker.TypeFlagsBooleanLiteral) != 0:
		add("boolean")
	case flags&checker.TypeFlagsUnion != 0:
		add("union")
	case flags&checker.TypeFlagsIntersection != 0:
		add("intersection")
	case flags&checker.TypeFlagsObject != 0:
		add("object")
	}
	return out
}

func memberName(name *ast.Node) string {
	if name == nil {
		return ""
	}
	switch name.Kind {
	case ast.KindIdentifier, ast.KindPrivateIdentifier, ast.KindStringLiteral, ast.KindNumericLiteral, ast.KindBigIntLiteral:
		return name.Text()
	case ast.KindComputedPropertyName:
		return "[computed]"
	default:
		return "<" + strings.TrimPrefix(name.Kind.String(), "Kind") + ">" // binding patterns
	}
}
