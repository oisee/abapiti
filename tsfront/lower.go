package tsfront

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
	"github.com/oisee/abapiti/tsfront/overrides"
)

// LowerDiagnostic is one finding of the lowering. Categories starting with
// "note-" record policy decisions (constructs that were mapped, not dropped);
// every other category marks a construct the lowering does not support, and
// the member that contains it is left out of the program (never lowered
// wrongly and silently).
type LowerDiagnostic struct {
	Category string `json:"category"`
	Loc      string `json:"loc"` // file:line:col
	Message  string `json:"message"`
}

func (d LowerDiagnostic) String() string { return d.Category + " " + d.Loc + ": " + d.Message }

// Lower lowers the named files of the program into the object HIR. The files
// must be part of the program (paths as they appear in SourceFiles); every
// declaration they reference at run time must be in the list, because the
// lowering only resolves to declarations it has lowered. Types come from the
// checker; the lowering never infers a type from syntax alone. The second
// result reports unsupported constructs and policy notes. Files should be
// passed in a deterministic order; declaration order in the HIR follows it.
func (p *Program) Lower(files []string) (*hir.Program, []LowerDiagnostic, error) {
	return p.LowerWithOverrides(files, overrides.Abaplint())
}

func (p *Program) LowerWithOverrides(files []string, registry *overrides.Registry) (*hir.Program, []LowerDiagnostic, error) {
	return p.lowerWithPolicy(files, registry, nil)
}

// LowerOptions makes the integer input contract explicit. Exceptions are source pinned.
type LowerOptions struct {
	AssumeOnlyIntegerCalculations bool
	IntegerExceptions             []IntegerException
}

func (p *Program) LowerWithOptions(files []string, options LowerOptions) (*hir.Program, []LowerDiagnostic, error) {
	return p.lowerWithOptions(files, overrides.Abaplint(), nil, options)
}

func (p *Program) lowerWithPolicy(files []string, registry *overrides.Registry, coverage *Reachability) (*hir.Program, []LowerDiagnostic, error) {
	return p.lowerWithOptions(files, registry, coverage, LowerOptions{AssumeOnlyIntegerCalculations: os.Getenv("ABAPITI_ASSUME_INT") == "1"})
}

func (p *Program) lowerWithOptions(files []string, registry *overrides.Registry, coverage *Reachability, options LowerOptions) (*hir.Program, []LowerDiagnostic, error) {
	l := &lowerer{
		integerOptions:    options,
		integerExceptions: map[*ast.Node]IntegerException{},
		floatSites:        map[string]bool{},
		unexecuted:        map[*ast.Node]string{},
		overrides:         map[*ast.Node]overrides.Entry{},
		prog:              p,
		implicitCtors:     map[*hir.Class]bool{},
		out:               &hir.Program{},
		classes:           map[*ast.Symbol]*hir.Class{},
		ifaces:            map[*ast.Symbol]*hir.Interface{},
		methods:           map[*ast.Symbol]*hir.Method{},
		fields:            map[*ast.Symbol]hir.Field{},
		synths:            map[*ast.Symbol]*hir.Class{},
		modules:           map[*ast.SourceFile]*hir.Class{},
		modvars:           map[*ast.Symbol]string{},
		classesByName:     map[string]*hir.Class{},
		ifacesByName:      map[string]*hir.Interface{},
		methodsBy:         map[string]*hir.Method{},
		fieldsBy:          map[string]hir.Field{},
		modvarsByName:     map[string]modvarRef{},
		synthsByName:      map[string]*hir.Class{},
		enums:             map[string]map[string]string{},
		enumNumbers:       map[string]map[string]float64{},
		nsNeeded:          map[string]bool{},
		views:             map[string]*hir.Interface{},
		ifaceNodes:        map[string]*ast.Node{},
		funcsBy:           map[string]funcRef{},
		covariants:        map[string]map[string]bool{},
	}
	if err := l.validateIntegerExceptions(files); err != nil {
		return nil, nil, err
	}
	if err := l.validateReachability(files, coverage); err != nil {
		return nil, nil, err
	}
	if err := l.validateOverrides(files, registry); err != nil {
		return nil, nil, err
	}
	if coverage != nil && coverage.Schema == 2 {
		var err error
		files, err = l.pruneDeclarations(files)
		if err != nil {
			return nil, nil, err
		}
	}
	for _, name := range files {
		f, ok := p.File(name)
		if !ok {
			return nil, nil, fmt.Errorf("file %s is not part of the program", name)
		}
		ck, done := p.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		l.checkStringLiterals(f.AsNode())
		l.registerFile(f)
		done()
	}
	// Namespace modules used as values need their export map; module-level
	// functions and enums are registered before any body references them.
	for _, name := range files {
		f, _ := p.File(name)
		ck, done := p.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		l.scanModuleUse(f)
		done()
	}
	// Allocate namespace fields before any module body can reference them.
	for _, name := range files {
		f, _ := p.File(name)
		if l.nsNeeded[f.FileName()] {
			mod := l.moduleClassOf(f)
			mod.Fields = append(mod.Fields, hir.Field{Name: "ns", Static: true, Type: hir.T(hir.OrderedMap, hir.T(hir.String), hir.T(hir.ClassValue))})
		}
	}
	// Data interfaces before class signatures: anonymous shapes with the
	// same fields reuse them instead of synthesizing a second class.
	for _, name := range files {
		f, _ := p.File(name)
		ck, done := p.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		for _, stmt := range l.statementNodes(f) {
			if stmt.Kind == ast.KindInterfaceDeclaration && l.isDataInterface(stmt) {
				if c := l.classOf(stmt.Symbol()); c != nil && c.Ctor == nil {
					l.dataInterfaceClass(stmt, &hir.Interface{Node: c.Node, Name: c.Name})
				}
			}
		}
		done()
	}
	// Method interfaces before class signatures: inferred implementations and
	// union views must see the same interface identity in every file.
	for _, name := range files {
		f, _ := p.File(name)
		ck, done := p.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		for _, stmt := range l.statementNodes(f) {
			if stmt.Kind == ast.KindInterfaceDeclaration && !l.isDataInterface(stmt) {
				if i := l.ifaceOf(stmt.Symbol()); i != nil {
					l.interfaceSignatures(stmt, i)
				}
			}
		}
		done()
	}
	// Signatures before module values and bodies: module-level functions
	// (lowered with the modules) call into classes.
	for _, name := range files {
		f, _ := p.File(name)
		ck, done := p.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		l.signaturesFile(f)
		done()
	}
	// An implicit derived constructor forwards the inherited parameter list.
	var inherit func(*hir.Class)
	visited := map[*hir.Class]bool{}
	inherit = func(c *hir.Class) {
		if visited[c] {
			return
		}
		visited[c] = true
		if base := l.classesByQualifiedName(c.Super); base != nil {
			inherit(base)
			if l.implicitCtors[c] {
				if ctor := l.constructorOf(base); ctor != nil {
					c.Ctor.Params = append([]hir.Param{}, ctor.Params...)
				}
			}
		}
	}
	for _, c := range l.out.Classes {
		inherit(c)
	}
	l.completeInterfaceHeritage()
	l.completeIfaceClassHeritage()
	l.eraseGenericOverrides()
	l.abiReady = true
	l.completeUnionInterfaces()
	l.covariantImplements()
	l.eraseGenericOverrides()
	l.completeInterfaceValueSlots()
	// Register all module function signatures before lowering any function body.
	for _, name := range files {
		f, _ := p.File(name)
		ck, done := p.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		for _, fn := range l.statementNodes(f) {
			if fn.Kind != ast.KindFunctionDeclaration || fn.Name() == nil {
				continue
			}
			mod := l.moduleClassOf(f)
			hm := &hir.Method{Node: l.node(fn), Name: fn.Name().Text(), Static: true, Internal: fn.ModifierFlags()&ast.ModifierFlagsExport == 0, Result: hir.T(hir.Void)}
			l.class, l.method = mod, hm
			if e, ok := l.overrides[fn]; ok && e.Method != nil {
				hm = e.Method()
				hm.Node = l.node(fn)
				l.diagf(fn, "note-override", "%s: %s", e.ID, e.Rationale)
			} else {
				l.signature(fn, hm)
			}
			mod.Methods = append(mod.Methods, hm)
			l.funcsBy[f.FileName()+" "+hm.Name] = funcRef{owner: mod.Name, method: hm.Name}
			l.methodsBy[mod.Name+"."+hm.Name] = hm
		}
		done()
	}
	// Register module-variable signatures before any module function body.
	// Bodies may refer to imported values whose initializers occur later.
	for _, name := range files {
		f, _ := p.File(name)
		ck, done := p.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		for _, stmt := range l.statementNodes(f) {
			if stmt.Kind != ast.KindVariableStatement {
				continue
			}
			for _, d := range stmt.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
				if d.Name() == nil || d.Name().Kind != ast.KindIdentifier {
					continue
				}
				before := len(l.diags)
				var typ hir.Type
				if d.Type() != nil {
					typ = l.mapTypeNode(d.Type())
				} else if d.Initializer() != nil && d.Initializer().Kind == ast.KindSatisfiesExpression {
					typ = l.mapTypeNode(d.Initializer().Type())
				} else {
					typ = l.mapCheckerType(d, l.ck.GetTypeAtLocation(d))
				}
				valid := typ.Kind != hir.Void && !hasBlocking(l.diags[before:])
				l.diags = l.diags[:before]
				if !valid {
					continue
				}
				mod := l.moduleClassOf(f)
				mod.Fields = append(mod.Fields, hir.Field{Node: l.node(d), Name: d.Name().Text(), Type: typ, Static: true, Private: ast.IsVarConst(d), Readonly: ast.IsVarConst(d)})
				l.modvars[d.Symbol()] = d.Name().Text()
				l.modvarsByName[f.FileName()+" "+d.Name().Text()] = modvarRef{owner: mod.Name, field: d.Name().Text()}
			}
		}
		done()
	}
	// Module values after signatures: module functions call class members.
	for _, name := range files {
		f, _ := p.File(name)
		ck, done := p.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		l.lowerModule2(f)
		done()
	}
	// Namespace modules outside the lowered file list still need their
	// (possibly empty) export map: nothing in them is lowered, but code
	// iterates them. Name order: module classes and their descriptors are
	// emitted in creation order, so map order would make the output vary.
	var needed []*ast.SourceFile
	for name := range l.nsNeeded {
		if f, ok := p.File(name); ok {
			needed = append(needed, f)
		}
	}
	sort.Slice(needed, func(i, j int) bool { return l.relFile(needed[i]) < l.relFile(needed[j]) })
	for _, f := range needed {
		if l.modules[f] != nil {
			continue
		}
		ck, done := p.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		l.lowerModule2(f)
		done()
	}
	for _, name := range files {
		f, _ := p.File(name)
		ck, done := p.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		l.bodiesFile(f)
		done()
	}
	l.completeUnionInterfaces()
	l.covariantImplements()
	l.propagateBridgeTraps()
	l.inferNumberRanges()
	if options.AssumeOnlyIntegerCalculations {
		l.assumeIntegerTypes()
	}
	hir.AssignSiteIDsWithSource(l.out, p.siteSource)
	return l.out, l.diags, nil
}

// lowerer carries the state of one lowering run; file and ck are the per-file
// state of the pass being run.
type lowerer struct {
	integerOptions    LowerOptions
	integerExceptions map[*ast.Node]IntegerException
	floatSites        map[string]bool
	retained          map[*ast.Node]bool
	typeOnly          map[*ast.Node]bool
	unexecuted        map[*ast.Node]string
	overrides         map[*ast.Node]overrides.Entry
	prog              *Program
	ck                *checker.Checker
	file              *ast.SourceFile
	out               *hir.Program
	diags             []LowerDiagnostic
	classes           map[*ast.Symbol]*hir.Class
	ifaces            map[*ast.Symbol]*hir.Interface
	methods           map[*ast.Symbol]*hir.Method
	fields            map[*ast.Symbol]hir.Field
	synths            map[*ast.Symbol]*hir.Class // object-literal alias symbol -> class
	modules           map[*ast.SourceFile]*hir.Class
	modvars           map[*ast.Symbol]string // module variable symbol -> field name
	scope             []map[string]hir.Type
	class             *hir.Class  // class whose member is being lowered
	method            *hir.Method // method being lowered
	serial            int
	hint              hir.Type // contextual type for undefined literals
	// Anonymous checker graphs may contain cycles without a nominal reference
	// to stop structural expansion. Reject those instead of overflowing the
	// Go stack. Entries live only for the current recursive mapping call.
	mappingTypes map[*checker.Type]bool

	// Phase 2: preludes turn expressions with inlined loops into hir.Seq.
	pend []*hir.Stmt
	// enums maps "file name" + " " + enum name -> member -> string value.
	enums       map[string]map[string]string
	enumNumbers map[string]map[string]float64
	// nsNeeded marks module files that are imported as a namespace and used
	// as a value; they get an export map in their module class.
	nsNeeded map[string]bool
	// views holds lazily grown cast-view interfaces for non-lowered classes.
	views    map[string]*hir.Interface
	unions   map[string]*unionView
	abiReady bool
	// Interface heritage (lower_iface_heritage.go).
	ifaceDone       map[*hir.Interface]bool
	ifaceBases      map[*hir.Interface][]*hir.Interface
	ifaceClassBases []ifaceClassBase
	// ifaceClassBaseNames: lowered interface name -> class names it extends
	// (directly or through base interfaces).
	ifaceClassBaseNames map[string][]string
	// shapeLike marks data-interface classes: plain objects that an
	// anonymous shape with the same fields may reuse.
	shapeLike map[string]bool
	// thisOverride replaces `this` while a closure body is lowered.
	thisOverride *hir.Expr
	// widenedLets: let symbols whose HIR type is the common base of their
	// assignments; the checker's narrower view of them is not applied.
	widenedLets map[*ast.Symbol]bool
	// guards: locals proven present by an enclosing definedness guard
	// (`x !== undefined && ...`, `if (x) {...}`), narrowed at every read.
	guards map[*ast.Symbol]hir.Type
	// `continue`/`break` statements rewritten inside a for loop with an
	// update expression (lower_syntax.go).
	breakViaFlag     map[*ast.Node]*hir.Expr
	ifaceClassBaseOf map[string]string
	bridgeTargets    map[*hir.Method]*hir.Method
	// ifaceNodes maps interface names to their declarations (for checker
	// queries about interface types).
	ifaceNodes map[string]*ast.Node
	// funcsBy registers module-level functions (by owner file + name).
	funcsBy map[string]funcRef
	// covariants records implicit "class implements interface" edges that the
	// structural conversion discovered (owner class -> interface name).
	covariants map[string]map[string]bool
	// localFns are the arrow functions lifted to methods in the current
	// method body, by local name; pendingFns are the ones not lifted yet.
	localFns   map[string]*localFn
	pendingFns map[string]*ast.Node
	// replacements rebind one identifier node during an optional-chain
	// rewrite ([node, replacement] pairs).
	replacements  [][2]any
	implicitCtors map[*hir.Class]bool

	// The per-file checkers hand out distinct symbol pointers for the same
	// cross-file declaration, so every registry also has a by-name view keyed
	// by the TypeScript simple names (owner name plus member name), which are
	// stable. A name registered twice for different declarations is rejected
	// rather than resolved by order.
	classesByName map[string]*hir.Class
	ifacesByName  map[string]*hir.Interface
	methodsBy     map[string]*hir.Method
	fieldsBy      map[string]hir.Field
	modvarsByName map[string]modvarRef // current file + var name -> module field
	constReads    map[*ast.Symbol]bool // module constants read anywhere in the program
	synthsByName  map[string]*hir.Class

	// Retain checker binding identity for post-lowering boundary rewrites.
	localSymbols map[*hir.Expr]*ast.Symbol
	declSymbols  map[*hir.Stmt]*ast.Symbol
	paramSymbols map[*hir.Method]map[string]*ast.Symbol
}

// funcRef names the module class and static method of a module function.
type funcRef struct {
	owner, method string
}

// localFn is one arrow function lifted to a private method: its captures are
// passed as leading parameters (they are immutable locals).
type localFn struct {
	method   *hir.Method
	captures []localCapture
	owner    *hir.Class
}

type localCapture struct {
	name   string
	typ    hir.Type
	symbol *ast.Symbol
	node   *ast.Node
}

// classOf resolves a class symbol through both registries. The by-name
// fallback is scoped to the declaring file: TypeScript files are namespaces,
// and abaplint has Plus the token and Plus the combinator.
func (l *lowerer) classOf(sym *ast.Symbol) *hir.Class {
	if sym == nil {
		return nil
	}
	if c, ok := l.classes[sym]; ok {
		return c
	}
	if f := l.fileOfSymbol(sym); f != nil {
		if c, ok := l.classesByName[f.FileName()+" "+sym.Name]; ok {
			return c
		}
	}
	return nil
}

// ifaceOf resolves an interface symbol through both registries.
func (l *lowerer) ifaceOf(sym *ast.Symbol) *hir.Interface {
	if sym == nil {
		return nil
	}
	if i, ok := l.ifaces[sym]; ok {
		return i
	}
	if f := l.fileOfSymbol(sym); f != nil {
		return l.ifacesByName[f.FileName()+" "+sym.Name]
	}
	return nil
}

// methodOf resolves a member symbol through both registries.
func (l *lowerer) methodOf(sym *ast.Symbol) *hir.Method {
	if sym == nil {
		return nil
	}
	if m, ok := l.methods[sym]; ok {
		return m
	}
	if sym.Parent != nil {
		if f := l.fileOfSymbol(sym.Parent); f != nil {
			key := f.FileName() + " " + sym.Parent.Name + "." + sym.Name
			return l.methodsBy[key]
		}
	}
	return nil
}

// fieldOf resolves a field symbol through both registries.
func (l *lowerer) fieldOf(sym *ast.Symbol) (hir.Field, bool) {
	if sym == nil {
		return hir.Field{}, false
	}
	if f, ok := l.fields[sym]; ok {
		return f, true
	}
	if sym.Parent != nil {
		if f := l.fileOfSymbol(sym.Parent); f != nil {
			if fd, ok := l.fieldsBy[f.FileName()+" "+sym.Parent.Name+"."+sym.Name]; ok {
				return fd, true
			}
		}
	}
	return hir.Field{}, false
}

// modvarRef names the module class and its static field for one value.
type modvarRef struct {
	owner, field string
}

// modvarOf resolves a module-level value symbol to its module class and
// static field name. Module values are visible only inside their own file,
// so the current file disambiguates equal names in different files (each
// lexer file has its own EOF).
func (l *lowerer) modvarOf(sym *ast.Symbol) (string, string, bool) {
	if name, ok := l.modvars[sym]; ok {
		if mod := l.modules[l.fileOfSymbol(sym)]; mod != nil {
			return mod.Name, name, true
		}
	}
	if f := l.fileOfSymbol(sym); f != nil {
		if r, ok := l.modvarsByName[f.FileName()+" "+sym.Name]; ok {
			return r.owner, r.field, true
		}
	}
	// The symbol may carry no declaration in this checker's view; a module
	// variable name that is unique across all lowered files resolves
	// unambiguously by name.
	var found modvarRef
	count := 0
	suffix := " " + sym.Name
	for key, r := range l.modvarsByName {
		if len(key) > len(suffix) && key[len(key)-len(suffix):] == suffix {
			found = r
			count++
		}
	}
	if count == 1 {
		return found.owner, found.field, true
	}
	return "", "", false
}

// synthOf resolves an object-shape alias symbol through both registries.
func (l *lowerer) synthOf(sym *ast.Symbol) *hir.Class {
	if sym == nil {
		return nil
	}
	if c, ok := l.synths[sym]; ok {
		return c
	}
	if f := l.fileOfSymbol(sym); f != nil {
		return l.synthsByName[f.FileName()+" "+sym.Name]
	}
	return nil
}

func (l *lowerer) diagf(n *ast.Node, category, format string, args ...any) {
	l.diags = append(l.diags, LowerDiagnostic{Category: category, Loc: l.locOf(n), Message: fmt.Sprintf(format, args...)})
}

// node returns the identity (ID and source location) for a new HIR node.
func (l *lowerer) node(n *ast.Node) hir.Node {
	return hir.Node{ID: l.nextID(), Source: l.locOf(n)}
}

func (l *lowerer) nextID() int {
	l.serial++
	return l.serial
}

func (l *lowerer) locOf(n *ast.Node) string {
	if n == nil {
		if l.file == nil {
			return "<unknown>"
		}
		return locString(l.file, 0)
	}
	return locString(l.file, scanner.GetTokenPosOfNode(n, l.file, false /*includeJSDoc*/))
}

// qualifiedName builds the stable identity of a declaration: the file path
// relative to the tsconfig directory plus the declaration name.
func (l *lowerer) qualifiedName(f *ast.SourceFile, name string) string {
	return l.relFile(f) + "." + name
}

// relFile is f's path relative to the tsconfig directory, with forward
// slashes. A package that resolves outside that directory (a symlinked
// node_modules) is named from its node_modules segment: generated names and
// locations never depend on where the project or its packages live.
func (l *lowerer) relFile(f *ast.SourceFile) string {
	rel, err := filepath.Rel(l.prog.configDir, f.FileName())
	if err != nil {
		return f.FileName()
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "../") {
		if i := strings.Index(rel, "/node_modules/"); i >= 0 {
			rel = rel[i+1:]
		}
	}
	return rel
}

// trapLocation is the location a trap reports at run time: relative
// "file:line:col", like the reachability traps.
func (l *lowerer) trapLocation(n *ast.Node) string {
	if l.file == nil {
		return "<unknown>"
	}
	pos := 0
	if n != nil {
		pos = scanner.GetTokenPosOfNode(n, l.file, false /*includeJSDoc*/)
	}
	line, col := lineCol(l.file, pos)
	return fmt.Sprintf("%s:%d:%d", l.relFile(l.file), line, col)
}

// resolve returns the symbol at node with import aliases resolved. Property
// accesses on interface-typed receivers can resolve without a parent through
// the checker pool; the member name node carries the full symbol.
func (l *lowerer) resolve(n *ast.Node) *ast.Symbol {
	sym := l.ck.GetSymbolAtLocation(n)
	if sym == nil {
		return nil
	}
	if sym.Parent == nil && n != nil && n.Kind == ast.KindPropertyAccessExpression {
		if name := n.Name(); name != nil {
			if ns := l.ck.GetSymbolAtLocation(name); ns != nil && ns.Parent != nil {
				sym = ns
			}
		}
	}
	if sym.Flags&ast.SymbolFlagsAlias != 0 {
		if target, ok := l.ck.ResolveAlias(sym); ok {
			return target
		}
	}
	return sym
}

func (l *lowerer) registerFile(f *ast.SourceFile) {
	for _, stmt := range l.statementNodes(f) {
		switch stmt.Kind {
		case ast.KindClassDeclaration:
			if name := stmt.Name(); name != nil && stmt.Symbol() != nil {
				c := &hir.Class{Node: l.node(stmt), Name: l.qualifiedName(f, name.Text())}
				key := f.FileName() + " " + name.Text()
				if old, ok := l.classesByName[key]; ok && old != c {
					l.diagf(stmt, "unsupported-decl", "duplicate class name %s", name.Text())
					continue
				}
				l.classes[stmt.Symbol()] = c
				l.classesByName[key] = c
				l.out.Classes = append(l.out.Classes, c)
			}
		case ast.KindInterfaceDeclaration:
			// A pure index-signature interface has no nominal method/data
			// slots. Its checker type already lowers to the ordered-map ABI.
			if members := stmt.Members(); len(members) == 1 && members[0].Kind == ast.KindIndexSignature && stmt.AsInterfaceDeclaration().HeritageClauses == nil {
				continue
			}
			if l.isDataInterface(stmt) && stmt.Name() != nil && stmt.Symbol() != nil {
				c := &hir.Class{Node: l.node(stmt), Name: l.qualifiedName(f, stmt.Name().Text())}
				l.classes[stmt.Symbol()] = c
				l.classesByName[f.FileName()+" "+stmt.Name().Text()] = c
				l.out.Classes = append(l.out.Classes, c)
				continue
			}
			if name := stmt.Name(); name != nil && stmt.Symbol() != nil {
				i := &hir.Interface{Node: l.node(stmt), Name: l.qualifiedName(f, name.Text())}
				key := f.FileName() + " " + name.Text()
				if old, ok := l.ifacesByName[key]; ok && old != i {
					l.diagf(stmt, "unsupported-decl", "duplicate interface name %s", name.Text())
					continue
				}
				l.ifaces[stmt.Symbol()] = i
				l.ifacesByName[key] = i
				l.ifaceNodes[i.Name] = stmt
				l.out.Interfaces = append(l.out.Interfaces, i)
			}
		}
	}
}

// moduleVar lowers one module-level const/let into a static field plus the
// initializing statement of the class constructor. The field type comes from
// the annotation if present, otherwise from the lowered initializer (the
// checker's type of `new Set<number>` carries no readable type arguments).
func (l *lowerer) moduleVar(d *ast.Node, mod *hir.Class) []*hir.Stmt {
	name := d.Name().Text()
	init := d.Initializer()
	sym := d.Symbol()
	if init != nil && !l.moduleInitializer(init) {
		l.diagf(init, "unsupported-static-init", "module initializer is not provably pure and order-independent")
		return nil
	}
	var typ hir.Type
	var stmts []*hir.Stmt
	switch {
	case d.Type() != nil:
		typ = l.mapTypeNode(d.Type())
		if typ.Kind == hir.Void {
			return nil
		}
		l.hint = typ
		x := l.expr(init)
		l.hint = hir.Type{}
		if x == nil {
			return nil
		}
		stmts = []*hir.Stmt{l.assignStatic(mod, d, name, x, typ)}
	case l.isNewCollection(init):
		var expr *hir.Expr
		stmts, expr = l.collectionInit(init)
		if expr == nil {
			return nil
		}
		typ = expr.Type
		stmts = append(stmts, l.assignStatic(mod, d, name, expr, typ))
	default:
		// Node-based type queries are valid across the checker pool;
		// symbol-based ones are not (types do not mix between checkers).
		mapped := l.mapCheckerType(d, l.ck.GetTypeAtLocation(d))
		if mapped.Kind == hir.Void && sym != nil {
			before := len(l.diags)
			mapped = l.mapCheckerType(d, l.ck.GetTypeOfSymbol(sym))
			if len(l.diags) != before {
				l.diags = l.diags[:before]
				mapped = hir.T(hir.Void)
			}
		}
		if mapped.Kind == hir.Void {
			return nil
		}
		typ = mapped
		l.hint = typ
		expr := l.expr(init)
		l.hint = hir.Type{}
		if expr == nil {
			return nil
		}
		typ = expr.Type
		stmts = []*hir.Stmt{l.assignStatic(mod, d, name, expr, typ)}
	}
	found := false
	for i := range mod.Fields {
		if mod.Fields[i].Name == name {
			mod.Fields[i].Type = typ
			found = true
			break
		}
	}
	if !found {
		mod.Fields = append(mod.Fields, hir.Field{Node: l.node(d), Name: name, Type: typ, Static: true, Private: ast.IsVarConst(d), Readonly: ast.IsVarConst(d)})
	}
	l.modvars[d.Symbol()] = name
	l.modvarsByName[l.file.FileName()+" "+name] = modvarRef{owner: mod.Name, field: name}
	return stmts
}

func (l *lowerer) assignStatic(mod *hir.Class, at *ast.Node, name string, x *hir.Expr, t hir.Type) *hir.Stmt {
	return &hir.Stmt{Kind: hir.Assign, Node: l.node(at),
		X: &hir.Expr{Kind: hir.StaticGet, Node: l.node(at), Owner: mod.Name, Name: name, Type: t}, Y: x}
}

// isNewCollection reports whether the expression is `new Set<T>([...])`,
// `new Map<K,V>([...])` or `new Array<T>([...])`, which lower to a
// construction sequence in statement context.
func (l *lowerer) isNewCollection(n *ast.Node) bool {
	if n == nil || n.Kind != ast.KindNewExpression || n.Expression() == nil {
		return false
	}
	t := n.Expression()
	if t.Kind != ast.KindIdentifier {
		return false
	}
	sym := l.resolve(t)
	if sym == nil {
		return false
	}
	// Only the library collections, never a lowered class of the same name.
	if l.classOf(sym) != nil {
		return false
	}
	declFile := l.fileOfSymbol(sym)
	if declFile == nil || !l.prog.prog.IsSourceFileDefaultLibrary(declFile.Path()) {
		return false
	}
	if sym.Name != "Set" && sym.Name != "Map" && sym.Name != "Array" {
		return false
	}
	// Only the array-literal initializers go through collectionInit; empty
	// collections and copies are handled by newExpression.
	args := n.Arguments()
	return len(args) == 1 && args[0].Kind == ast.KindArrayLiteralExpression
}

// collectionInit lowers `new Set<T>([...])` in a statement context into a
// fresh collection plus one insert per element.
func (l *lowerer) collectionInit(n *ast.Node) ([]*hir.Stmt, *hir.Expr) {
	name := n.Expression().Text()
	args := n.Arguments()
	var elemType hir.Type
	var ok bool
	switch name {
	case "Set":
		elemType, ok = l.typeArgAt(n, 0)
	case "Array":
		elemType, ok = l.typeArgAt(n, 0)
	default:
		l.diagf(n, "unsupported-new", "new %s is not lowered", name)
		return nil, nil
	}
	if !ok {
		return nil, nil
	}
	if len(args) != 1 || args[0].Kind != ast.KindArrayLiteralExpression {
		l.diagf(n, "unsupported-new", "new %s needs one array literal argument", name)
		return nil, nil
	}
	kind := hir.OrderedSet
	if name == "Array" {
		kind = hir.Array
	}
	typ := hir.T(kind, elemType)
	l.serial++
	tmp := "c" + itoa(l.serial)
	l.declare(tmp, typ)
	stmts := []*hir.Stmt{{Kind: hir.VarDecl, Node: l.node(n), Name: tmp, Type: typ,
		X: &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ}}}
	for _, el := range args[0].AsArrayLiteralExpression().Elements.Nodes {
		v := l.expr(el)
		if v == nil {
			return nil, nil
		}
		op := "set.add"
		if name == "Array" {
			op = "array.push"
		}
		recv := hir.V(tmp, typ)
		var res hir.Type = typ
		if name == "Array" {
			res = hir.T(hir.I32)
		}
		stmts = append(stmts, &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(el), X: l.rtOp(op, recv, res, v)})
	}
	return stmts, hir.V(tmp, typ)
}

// typeArgAt maps the i-th syntactic type argument of a new expression.
func (l *lowerer) typeArgAt(n *ast.Node, i int) (hir.Type, bool) {
	targs := n.TypeArguments()
	if len(targs) <= i {
		before := len(l.diags)
		mapped := l.mapCheckerType(n, l.ck.GetTypeAtLocation(n))
		if !hasBlocking(l.diags[before:]) && i < len(mapped.Args) {
			return mapped.Args[i], true
		}
		l.diags = l.diags[:before]
		l.diagf(n, "unsupported-new", "missing explicit or inferable type argument")
		return hir.Type{}, false
	}
	return l.mapTypeNode(targs[i]), true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func (l *lowerer) classesByQualifiedName(name string) *hir.Class {
	for _, c := range l.out.Classes {
		if c.Name == name {
			return c
		}
	}
	return nil
}
