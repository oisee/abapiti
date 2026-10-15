// Package abap lowers verified object HIR to ABAP 7.50 style.
// PCRE regex features require ABAP 7.55 or later.
package abap

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/oisee/abapiti/hir"
)

type emitter struct {
	testPackages    bool
	factoryTestPart bool
	chaiUsed        bool
	chaiArrays      []hir.Type
	chaiMaps        []hir.Type
	ownKeysUsed     bool
	ownAdapters     map[string]bool
	classIndex      map[string]*hir.Class
	descendantIndex map[string][]*hir.Class
	liveClasses     map[string]bool
	arrayViews      bool
	recordDelete    bool
	numberExprs     map[*hir.Expr]bool
	numberFields    map[string]map[string]bool
	constructorArgs bool
	constructorGet  bool
	configJSON      bool
	subsetJSONRoots []hir.Type
	jsonOwnKeys     bool
	promiseUsed     bool
	p               *hir.Program
	names           *hir.Names
	files           map[string]string
	types           map[string]bool
	err             error
	refusals        *[]error
	currentMethod   *hir.Method
	assertionOwners map[string]bool
	currentNode     *hir.Node
	valueSlots      map[string]bool
	materialized    map[string]bool
	// Usage gates keep the emitted file set of programs that do not use the
	// phase-2 machinery unchanged.
	descriptors, regexpUsed, dynamicUsed, errorUsed bool
	matchUsed, execUsed, jsRegexpUsed               bool
	isArrayUsed                                     bool
	descIndex                                       map[string]descRef
	descOrder                                       map[string][]string
	descCount                                       map[string]int

	// codeUnits lists the classes whose bodies read UTF-16 code units through
	// a converter they create once and keep in a private CLASS-DATA.
	codeUnits map[string]bool

	// staticConsts are static fields emitted as CONSTANTS (owner.field ->
	// ABAP literal); staticInit marks classes whose initializer still has
	// other work.
	staticConsts map[string]string
	staticInit   map[string]bool
	// constSets are static integer sets built from constants and only ever
	// asked has() (owner.field -> members); has() becomes a comparison chain.
	constSets map[string][]string
}

// Emit returns one source per global declaration, including all runtime dependencies.
func Emit(p *hir.Program) (map[string]string, error) {
	files, _, err := EmitNamed(p)
	return files, err
}

// EmitNamed additionally returns the name table used for emitted identities.
func EmitNamed(p *hir.Program) (map[string]string, *hir.Names, error) {
	return emitNamed(p, nil, nil, false)
}

// EmitTestNamed marks only explicitly identified assertion throws. The marker
// extends the original payload wrapper, preserving source catch semantics.
func EmitTestNamed(p *hir.Program, assertionOwners map[string]bool) (map[string]string, *hir.Names, error) {
	return emitNamed(p, nil, assertionOwners, false)
}

// EmitTestPackageNamed separates global runtime dispatch for test declarations
// behind a registered extension. Ordinary (hashed or readable) OSG emission is
// unchanged; only the two-package full build opts into this ABI.
func EmitTestPackageNamed(p *hir.Program, assertionOwners map[string]bool) (map[string]string, *hir.Names, error) {
	return emitNamed(p, nil, assertionOwners, true)
}

// Audit runs the same emitter, collecting every refusal instead of retaining
// only the final one. Sources are discarded: refused output is never runnable.
// Like EmitNamed, it applies the target's HIR passes in place.
func Audit(p *hir.Program) []error {
	refusals := []error{}
	_, _, err := emitNamed(p, &refusals, nil, false)
	if err != nil && len(refusals) == 0 {
		refusals = append(refusals, err)
	}
	return refusals
}

// Refusal carries the owning body even for synthetic ID-zero operands.
// Signature/layout refusals deliberately carry no Method.
type Refusal struct {
	Method *hir.Method
	Node   hir.Node
	Err    error
}

func (r Refusal) Error() string { return r.Err.Error() }

func (e *emitter) refuse(err error) {
	e.err = err
	if e.refusals != nil {
		var method *hir.Method
		var node hir.Node
		if e.currentNode != nil {
			method = e.currentMethod
			node = *e.currentNode
		} else if strings.HasPrefix(err.Error(), "node ") {
			method = e.currentMethod
		}
		*e.refusals = append(*e.refusals, Refusal{Method: method, Node: node, Err: err})
	}
}

func emitNamed(p *hir.Program, refusals *[]error, assertionOwners map[string]bool, testPackages bool) (map[string]string, *hir.Names, error) {
	if errors := hir.Verify(p); len(errors) > 0 {
		return nil, nil, errors[0]
	}
	if errors := hir.CheckRuntimeOps(p, "ABAP", SupportsRuntimeOp); (refusals == nil || len(p.Unions) > 0) && len(errors) > 0 {
		return nil, nil, errors[0]
	}
	hir.SealLiveClassViewBoundary(p)
	if err := inline(p); err != nil {
		return nil, nil, err
	}
	p, numberExprs, numberFields := retainNonfiniteStorage(p)
	names := hir.NewNames()
	if testPackages || os.Getenv("ABAPITI_NAMES") == "readable" {
		names = hir.NewReadableNames(p, "LNT")
	}
	e := &emitter{p: p, names: names, testPackages: testPackages, files: map[string]string{}, types: map[string]bool{}, descCount: map[string]int{}, refusals: refusals, assertionOwners: assertionOwners, numberExprs: numberExprs, numberFields: numberFields}
	e.promiseUsed = hir.UsesSettledPromises(p)
	e.liveClasses = hir.LiveClassViewLayouts(p)
	e.arrayViews = len(e.liveClasses) > 0
	mapViews := false
	var findStorage func(*hir.Expr)
	findStorage = func(x *hir.Expr) {
		if x == nil {
			return
		}
		if x.Type.Kind == hir.Dynamic && x.X != nil && x.X.Type.Kind == hir.OrderedMap && x.X.Type.Args[0].Kind == hir.String {
			mapViews = true
		}
		if x.Kind == hir.RuntimeOp && x.Op == "dynamic.delete" {
			e.recordDelete = true
		}
		if x.Kind == hir.RuntimeOp && (x.Op == "dynamic.arrayView" || x.Op == "dynamic.asRef" && x.Type.Kind == hir.Array || x.Op == "dynamic.classView" || x.Op == "dynamic.delete" || strings.HasPrefix(x.Op, "lsp.") || strings.HasPrefix(x.Op, "object.") && x.X != nil && x.X.Type.Name == hir.RootObject) {
			e.arrayViews = true
		}
		findStorage(x.X)
		findStorage(x.Y)
		findStorage(x.Z)
		for _, a := range x.Args {
			findStorage(a)
		}
		e.walkStmt(x.Stmt, findStorage)
	}
	for _, c := range p.Classes {
		if c.Ctor != nil {
			e.walkStmt(c.Ctor.Body, findStorage)
		}
		for _, m := range c.Methods {
			e.walkStmt(m.Body, findStorage)
		}
	}
	if e.arrayViews || mapViews {
		e.liveRuntime()
	}
	e.scanUsage()
	if e.ownKeysUsed {
		e.ownKeysRuntime()
	}
	for _, c := range p.Classes {
		if callableClass(c) {
			n := e.name("runtime.callable")
			e.files[n+".intf.abap"] = "INTERFACE " + n + " PUBLIC.\nENDINTERFACE.\n"
			break
		}
	}
	e.findStaticConstants()
	e.findConstSets()
	e.promoteValueSlots()
	if e.descriptors {
		n := e.name("runtime.described")
		e.files[n+".intf.abap"] = "INTERFACE " + n + " PUBLIC.\nMETHODS " + e.name("builtin.classOf") + " RETURNING VALUE(result) TYPE REF TO " + e.name("runtime.classvalue") + ".\nMETHODS " + e.name("builtin.nativeConstructor") + " RETURNING VALUE(result) TYPE REF TO " + e.name("runtime.classvalue") + ".\nENDINTERFACE.\n"
	}
	if e.constructorGet {
		e.dynamicConstructorRuntime()
	}
	for _, i := range p.Interfaces {
		var b strings.Builder
		fmt.Fprintf(&b, "INTERFACE %s PUBLIC.\n", e.name(i.Name))
		for _, m := range i.Methods {
			b.WriteString(e.signature(m, false))
		}
		if e.descriptors {
			fmt.Fprintf(&b, "METHODS %s RETURNING VALUE(result) TYPE REF TO %s.\n", e.name("builtin.classOf"), e.name("runtime.classvalue"))
		}
		b.WriteString("ENDINTERFACE.\n")
		e.files[e.name(i.Name)+".intf.abap"] = originComment(i.Name, i.Source) + b.String()
	}
	for _, c := range p.Classes {
		e.class(c)
	}
	if e.configJSON {
		e.configJSONRuntime()
	}
	e.support()
	e.markArrays()
	if e.chaiUsed {
		e.chaiRuntime()
	}
	if castTrace {
		e.traceCasts()
	}
	for file, src := range e.files {
		out, err := Format(src)
		if err != nil {
			if refusals == nil {
				return nil, nil, fmt.Errorf("%s: %w", file, err)
			}
			e.refuse(fmt.Errorf("%s: %w", file, err))
			continue
		}
		e.files[file] = out
	}
	if e.err != nil {
		return nil, nil, e.err
	}
	return e.files, e.names, nil
}
func (e *emitter) name(s string) string   { return e.names.Get(s) }
func (e *emitter) member(s string) string { return e.name("member." + s) }
func (e *emitter) param(s string) string  { return e.name("param." + s) }
func (e *emitter) classBy(n string) *hir.Class {
	if n == "" {
		return nil
	}
	if e.classIndex == nil {
		e.classIndex = make(map[string]*hir.Class, len(e.p.Classes))
		for _, c := range e.p.Classes {
			e.classIndex[c.Name] = c
		}
	}
	return e.classIndex[n]
}
func (e *emitter) descendants(c *hir.Class) []*hir.Class {
	if e.descendantIndex == nil {
		e.descendantIndex = map[string][]*hir.Class{}
		// Preserve program order, matching the previous per-parent scan.
		for _, child := range e.p.Classes {
			for base := e.classBy(child.Super); base != nil; base = e.classBy(base.Super) {
				e.descendantIndex[base.Name] = append(e.descendantIndex[base.Name], child)
			}
		}
	}
	return e.descendantIndex[c.Name]
}
func (e *emitter) method(c *hir.Class, n string) (*hir.Method, *hir.Class) {
	for ; c != nil; c = e.classBy(c.Super) {
		for _, m := range c.Methods {
			if m.Name == n {
				return m, c
			}
		}
	}
	return nil, nil
}

// Reference arrays share one object table. Typed views cast elements when read,
// preserving the array object and mutations across covariant/generic views.
func arrayStorage(t hir.Type) hir.Type {
	if t.Kind == hir.Array && len(t.Args) == 1 && t.Args[0].IsRef() {
		return hir.T(hir.Array, hir.Ref(hir.RootObject))
	}
	return t
}
func (e *emitter) typ(t hir.Type) string {
	t = arrayStorage(t)
	switch t.Kind {
	case hir.Bool:
		return "abap_bool"
	case hir.Number:
		return "f"
	case hir.I32:
		return "i"
	case hir.I64:
		return "int8"
	case hir.String:
		return "string"
	case hir.ClassRef:
		if t.Name == hir.RootObject {
			return "REF TO object"
		}
		if t.Name == "builtin.Error" {
			e.errorUsed = true
		}
		return "REF TO " + e.name(t.Name)
	case hir.InterfaceRef:
		return "REF TO " + e.name(t.Name)
	case hir.ClassValue:
		e.descriptors = true
		return "REF TO " + e.name("runtime.classvalue")
	case hir.RegExp:
		e.regexpUsed = true
		return "REF TO " + e.name("runtime.regexp")
	case hir.Dynamic:
		e.dynamicUsed = true
		return "REF TO " + e.name("runtime.dynamic")
	case hir.Promise:
		e.promiseUsed = true
		e.runtime(t)
		return "REF TO " + e.name("runtime."+t.String())
	case hir.Optional:
		if t.Args[0].IsRef() {
			return e.typ(t.Args[0])
		}
		fallthrough
	case hir.Array, hir.OrderedMap, hir.OrderedSet:
		e.runtime(t)
		return "REF TO " + e.name("runtime."+t.String())
	default:
		e.refuse(fmt.Errorf("unsupported ABAP type %s", t))
		return "i"
	}
}
func (e *emitter) signature(m *hir.Method, ctor bool) string {
	if m.Name == "class_constructor" && m.Static {
		return "CLASS-METHODS class_constructor.\n"
	}
	kw, name := "METHODS", e.member(m.Name)
	if ctor {
		name = "constructor"
	}
	if m.Static {
		kw = "CLASS-METHODS"
	}
	s := kw + " " + name
	if m.Abstract {
		s += " ABSTRACT"
	}
	if len(m.Params) > 0 {
		s += " IMPORTING"
		for _, p := range m.Params {
			s += " " + e.param(p.Name) + " TYPE " + e.typ(p.Type)
			if ctor && p.Type.Kind == hir.Optional {
				s += " OPTIONAL"
			}
		}
	}
	if m.Result.Kind != hir.Void {
		s += " RETURNING VALUE(result) TYPE " + e.typ(m.Result)
	}
	return s + ".\n"
}

// scanUsage pre-sets the phase-2 gates so the hidden classOf method is
// present in every class whenever descriptors could be needed.
func (e *emitter) scanUsage() {
	need := false
	var scanType func(hir.Type)
	scanType = func(ty hir.Type) {
		need = need || ty.Kind == hir.ClassValue
		for _, arg := range ty.Args {
			scanType(arg)
		}
	}
	scanMethod := func(m *hir.Method) {
		scanType(m.Result)
		for _, p := range m.Params {
			scanType(p.Type)
		}
	}
	var walk func(x *hir.Expr)
	walk = func(x *hir.Expr) {
		if x == nil {
			return
		}
		e.walkStmt(x.Stmt, walk)
		if x.Kind == hir.RuntimeOp && x.Op == "dynamic.get" && len(x.Args) == 1 && x.Args[0].Kind == hir.Lit && (x.Args[0].Value == "constructor" || x.Args[0].Value == "name") {
			need = true
			e.constructorGet = true
		}
		if x.Kind == hir.RuntimeOp && x.Op == "dynamic.keys" {
			e.ownKeysUsed = true
		}
		scanType(x.Type)
		if x.Kind == hir.ClassOf {
			need = true
		}
		if x.Kind == hir.RuntimeOp && (strings.HasPrefix(x.Op, "object.") || x.Op == "classvalue.new" || x.Op == "classvalue.name" || x.Op == "classvalue.has") {
			need = true
		}
		if x.Kind == hir.InstanceOf && x.Y != nil {
			need = true
		}
		if x.Kind == hir.RuntimeOp && strings.HasPrefix(x.Op, "chai.") {
			e.chaiUsed = true
			e.dynamicUsed = true
		}
		if x.Kind == hir.RuntimeOp && (x.Op == "dynamic.classView" || x.Op == "dynamic.arrayView" || x.Op == "dynamic.delete" || strings.HasPrefix(x.Op, "lsp.") || (strings.HasPrefix(x.Op, "object.") && x.X != nil && x.X.Type.Name == hir.RootObject)) {
			e.liveRuntime()
		}
		if x.Kind == hir.RuntimeOp && x.Op == "json.stringifySubset" {
			e.jsonOwnKeys = true
			// The subset writer synthesizes a ClassValue handler for dynamic
			// function values. Discover its descriptor ABI before emitting any
			// interfaces/classes, not later in configJSONRuntime.
			need = true
		}
		if x.Kind == hir.RuntimeOp && x.Op == "dynamic.materialize" {
			e.materializer(x.Type)
		}
		if x.Kind == hir.RuntimeOp && (x.Op == "regexp.exec" || x.Op == "regexp.lastIndex" || x.Op == "regexp.setLastIndex") {
			e.execUsed = true
			e.jsRegexpUsed = true
		}
		if x.Kind == hir.RuntimeOp && x.Op == "string.match" {
			e.matchUsed = true
		}
		if x.Kind == hir.RuntimeOp && x.Op == "regexp.test" && x.X != nil && x.X.Kind == hir.New && regexMayHaveState(x.X) {
			e.execUsed = true
			e.jsRegexpUsed = true
		}
		if x.Kind == hir.Seq {
			e.walkStmt(x.Stmt, walk)
		}
		if x.Type.Kind == hir.RegExp {
			e.regexpUsed = true
			if x.Kind == hir.New && regexNeedsJS(x) {
				e.jsRegexpUsed = true
			}
		}
		if x.Type.Kind == hir.Dynamic {
			e.dynamicUsed = true
		}
		walk(x.X)
		walk(x.Y)
		walk(x.Z)
		for _, a := range x.Args {
			walk(a)
		}
	}
	for _, i := range e.p.Interfaces {
		for _, m := range i.Methods {
			scanMethod(m)
		}
	}
	for _, c := range e.p.Classes {
		for _, f := range c.Fields {
			scanType(f.Type)
		}
		if c.Ctor != nil {
			scanMethod(c.Ctor)
			e.walkStmt(c.Ctor.Body, walk)
		}
		for _, m := range c.Methods {
			scanMethod(m)
			if m.Body != nil {
				e.walkStmt(m.Body, walk)
			}
		}
	}
	if need {
		e.descriptors = true
	}
}

func (e *emitter) walkStmt(s *hir.Stmt, walk func(*hir.Expr)) {
	if s == nil {
		return
	}
	walk(s.X)
	walk(s.Y)
	e.walkStmt(s.Body, walk)
	e.walkStmt(s.Else, walk)
	for _, x := range s.List {
		e.walkStmt(x, walk)
	}
}

// slotKey identifies the first declaration of an inherited virtual slot.
func (e *emitter) slotKey(c *hir.Class, name string) string {
	owner := c
	for base := e.classBy(c.Super); base != nil; base = e.classBy(base.Super) {
		if _, found := e.method(base, name); found != nil {
			owner = found
		}
	}
	return owner.Name + "." + name
}

// TS permits a value-returning override of a void method. ABAP needs one
// return signature for the entire inheritance slot so the redefinition can
// retain that value. Void callers and interface wrappers still discard it.
func (e *emitter) promoteValueSlots() {
	e.valueSlots = map[string]bool{}
	for _, c := range e.p.Classes {
		for _, m := range c.Methods {
			original, _, specialized := strings.Cut(m.Name, "_instantiated_")
			if !specialized || !m.Result.IsRef() {
				continue
			}
			slot, _ := e.method(c, original)
			base, _ := e.method(e.classBy(c.Super), original)
			if slot != nil && base != nil && slot.Result.Kind == hir.Void {
				e.valueSlots[e.slotKey(c, original)] = true
			}
		}
	}
}
func (e *emitter) emittedMethod(c *hir.Class, m *hir.Method) *hir.Method {
	if m.Result.Kind != hir.Void || !e.valueSlots[e.slotKey(c, m.Name)] {
		return m
	}
	clone := *m
	clone.Result = hir.Ref(hir.RootObject)
	return &clone
}

// overrideBody finds the checker-typed implementation behind an erased slot.
// ABAP requires SUPER->m to occur in METHOD m, so the implementation body
// belongs to the inherited slot; the checker-typed variant forwards to it.
func (e *emitter) overrideBody(c *hir.Class, slot *hir.Method) *hir.Method {
	if base, _ := e.method(e.classBy(c.Super), slot.Name); base == nil {
		return nil
	}
	for _, m := range c.Methods {
		if m.Name == slot.Name+"_instantiated_"+c.Name {
			return m
		}
	}
	return nil
}

func (e *emitter) narrowedBridge(c *hir.Class, m, slot *hir.Method) string {
	slot = e.emittedMethod(c, slot)
	b := &body{e: e, c: c, m: m, implemented: e.member(m.Name), locals: map[string]string{}}
	for _, p := range m.Params {
		n := b.temp(p.Type)
		b.locals[p.Name] = n
		b.line(n + " = " + e.param(p.Name) + ".")
	}
	args := []string{}
	for j, p := range slot.Params {
		if j >= len(m.Params) {
			// This implementation declares no binding for the trailing argument.
			// The inherited slot body is this same implementation; it cannot
			// observe the ABI filler (JS arguments/rest are not erased here).
			args = append(args, e.param(p.Name)+" = "+b.temp(p.Type))
			continue
		}
		actual := m.Params[j]
		args = append(args, e.param(p.Name)+" = "+b.value(hir.V(actual.Name, actual.Type), p.Type))
	}
	call := "me->" + e.member(slot.Name) + "( " + strings.Join(args, " ") + " )"
	if m.Result.Kind == hir.Void {
		b.line(call + ".")
	} else {
		if slot.Result.Kind == hir.Void {
			e.refuse(fmt.Errorf("class %s method %s: value-returning override of void superclass slot is unsupported", c.Name, slot.Name))
			return ""
		}
		result := b.temp(slot.Result)
		b.line(result + " = " + call + ".")
		op := " = "
		if !m.Result.Equal(slot.Result) {
			op = " ?= "
		}
		b.line("result" + op + result + ".")
	}
	b.line("RETURN.")
	return "METHOD " + b.implemented + ".\n" + b.constantDeclarations() + foldTempInits(b.code.String()) + "ENDMETHOD.\n"
}

// callableClass matches the Go backend's compiled-function predicate. Merely
// having a call method does not prove a source value is a JavaScript function.
func callableClass(c *hir.Class) bool {
	if !strings.HasPrefix(c.Name, "closure.") {
		return false
	}
	for _, i := range c.Implements {
		if i == "closure.thunk" || strings.HasPrefix(i, "closure.function.") {
			return true
		}
	}
	return false
}

func (e *emitter) class(c *hir.Class) {
	if e.liveClasses[c.Name] && c.Ctor == nil {
		// Even a data class with no initializers needs a source-binding constructor.
		copy := *c
		copy.Ctor = &hir.Method{Node: c.Node, Name: "constructor", Result: hir.T(hir.Void), Body: hir.B()}
		c = &copy
	}

	var b strings.Builder
	s := "CLASS " + e.name(c.Name) + " DEFINITION PUBLIC"
	if c.Super != "" {
		s += " INHERITING FROM " + e.name(c.Super)
	}
	if c.Abstract {
		s += " ABSTRACT"
	}
	s += " CREATE PUBLIC.\nPUBLIC SECTION.\n"
	b.WriteString(s)
	e.liveClassDeclaration(&b, c)
	if e.recordDelete && c.Super == "" {
		fmt.Fprintf(&b, "DATA %s TYPE HASHED TABLE OF string WITH UNIQUE KEY table_line.\n", e.name("record.deleted"))
	}
	if e.jsonOwnKeys && c.Structural && c.Super == "" {
		fmt.Fprintf(&b, "DATA %s TYPE REF TO %s.\n", e.name("json.ownKeys"), e.name("runtime.dynamic"))
	}
	if e.materialized[c.Name] {
		fmt.Fprintf(&b, "DATA %s TYPE REF TO %s.\n", e.name("builtin.materializedSource"), e.name("runtime.dynamic"))
	}
	if callableClass(c) {
		fmt.Fprintf(&b, "INTERFACES %s.\n", e.name("runtime.callable"))
	}
	for _, i := range c.Implements {
		fmt.Fprintf(&b, "INTERFACES %s.\n", e.name(i))
	}
	for _, f := range c.Fields {
		if lit, ok := e.staticConstant(c.Name, f.Name); ok && f.Static {
			fmt.Fprintf(&b, "CONSTANTS %s TYPE %s VALUE %s.\n", e.member(f.Name), e.typ(f.Type), lit)
			continue
		}
		kw := "DATA"
		if f.Static {
			kw = "CLASS-DATA"
		}
		fmt.Fprintf(&b, "%s %s TYPE %s.\n", kw, e.member(f.Name), e.typ(f.Type))
	}
	if c.Ctor != nil && !e.wideShape(c) {
		signature := e.signature(c.Ctor, true)
		if e.liveClasses[c.Name] {
			if len(c.Ctor.Params) == 0 {
				signature = strings.Replace(signature, ".\n", " IMPORTING", 1)
			} else {
				signature = strings.TrimSuffix(signature, ".\n")
			}
			signature += " view_source TYPE REF TO " + e.name("runtime.dynamic") + " OPTIONAL.\n"
		}
		b.WriteString(signature)
	}
	for _, m := range c.Methods {
		if m.Name == "class_constructor" && m.Static {
			fmt.Fprintf(&b, "CLASS-DATA %s TYPE abap_bool.\nCLASS-METHODS %s.\n", e.name("builtin.initialized."+c.Name), e.name("builtin.initialize."+c.Name))
			continue
		}
		base, _ := e.method(e.classBy(c.Super), m.Name)
		if base != nil && m.Abstract {
			// An abstract redeclaration of an inherited (abstract) method adds
			// nothing ABAP can express.
			continue
		}
		if base != nil {
			fmt.Fprintf(&b, "METHODS %s REDEFINITION.\n", e.member(m.Name))
		} else {
			b.WriteString(e.signature(e.emittedMethod(c, m), false))
		}
	}
	if e.descriptors {
		if c.Scoped {
			fmt.Fprintf(&b, "DATA %s TYPE REF TO %s.\n", e.name("class.lexicalDescriptor"), e.name("runtime.classvalue"))
		}
		if c.Super != "" {
			fmt.Fprintf(&b, "METHODS %s REDEFINITION.\nMETHODS %s REDEFINITION.\n", e.name("builtin.classOf"), e.name("builtin.nativeConstructor"))
		} else {
			fmt.Fprintf(&b, "INTERFACES %s.\n", e.name("runtime.described"))
			fmt.Fprintf(&b, "METHODS %s RETURNING VALUE(result) TYPE REF TO %s.\nMETHODS %s RETURNING VALUE(result) TYPE REF TO %s.\n", e.name("builtin.classOf"), e.name("runtime.classvalue"), e.name("builtin.nativeConstructor"), e.name("runtime.classvalue"))
		}
	}
	b.WriteString("PROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + e.name(c.Name) + " IMPLEMENTATION.\n")
	if c.Ctor != nil && !e.wideShape(c) {
		b.WriteString(e.body(c, c.Ctor, "constructor"))
	}
	for _, m := range c.Methods {
		if m.Abstract {
			continue
		}
		name := e.member(m.Name)
		if m.Name == "class_constructor" && m.Static {
			name = e.name("builtin.initialize." + c.Name)
		}
		if impl := e.overrideBody(c, m); impl != nil {
			if m.Body != nil && m.Body.Kind == hir.Async && e.forwardsToImplementation(impl, m) {
				// The lowered async bridge adopts a fresh promise, including
				// rejection. A plain forwarder cannot cast invariant storage.
				b.WriteString(e.body(c, m, name))
			} else if e.forwardsToImplementation(impl, m) {
				b.WriteString(e.slotForwarder(c, m, impl))
			} else {
				b.WriteString(e.overrideImplementation(c, impl, m))
			}
		} else if original, _, specialized := strings.Cut(m.Name, "_instantiated_"); specialized {
			slot, _ := e.method(c, original)
			if slot != nil && e.overrideBody(c, slot) == m && !e.forwardsToImplementation(m, slot) {
				b.WriteString(e.narrowedBridge(c, m, slot))
			} else {
				b.WriteString(e.body(c, m, name))
			}
		} else {
			b.WriteString(e.body(c, m, name))
		}
	}
	if e.descriptors {
		name := e.name("builtin.classOf")
		if c.Super == "" {
			fmt.Fprintf(&b, "METHOD %s~%s.\nresult = me->%s( ).\nENDMETHOD.\n", e.name("runtime.described"), name, name)
		}
		if c.Super == "" {
			fmt.Fprintf(&b, "METHOD %s~%s.\nresult = me->%s( ).\nENDMETHOD.\n", e.name("runtime.described"), e.name("builtin.nativeConstructor"), e.name("builtin.nativeConstructor"))
		}
		fmt.Fprintf(&b, "METHOD %s.\n", e.name("builtin.nativeConstructor"))
		if c.Structural {
			b.WriteString("RAISE EXCEPTION TYPE " + e.classRefusal() + ".\n")
		} else {
			fmt.Fprintf(&b, "result = me->%s( ).\nIF result IS NOT BOUND.\nRAISE EXCEPTION TYPE %s.\nENDIF.\n", name, e.classRefusal())
		}
		b.WriteString("ENDMETHOD.\n")
		if c.Scoped {
			fmt.Fprintf(&b, "METHOD %s.\nresult = me->%s.\nENDMETHOD.\n", name, e.name("class.lexicalDescriptor"))
		} else if e.liveClasses[c.Name] {
			fmt.Fprintf(&b, "METHOD %s.\nIF %s IS NOT BOUND.\nresult = %s.\nENDIF.\nENDMETHOD.\n", name, e.name("view.source"), e.descriptorOf(c.Name))
		} else {
			fmt.Fprintf(&b, "METHOD %s.\nresult = %s.\nENDMETHOD.\n", name, e.descriptorOf(c.Name))
		}
	}
	for _, n := range c.Implements {
		for _, i := range e.p.Interfaces {
			if i.Name != n {
				continue
			}
			if e.descriptors {
				fmt.Fprintf(&b, "METHOD %s~%s.\nresult = me->%s( ).\nENDMETHOD.\n", e.name(n), e.name("builtin.classOf"), e.name("builtin.classOf"))
			}
			for _, m := range i.Methods {
				fmt.Fprintf(&b, "METHOD %s~%s.\n", e.name(n), e.member(m.Name))
				s := "me->" + e.member(m.Name) + "( "
				if len(m.Params) > 0 {
					for _, p := range m.Params {
						s += " " + e.param(p.Name) + " = " + e.param(p.Name)
					}
				}
				if m.Result.Kind != hir.Void {
					s = "result = " + s
				}
				b.WriteString(s + " ).\nENDMETHOD.\n")
			}
		}
	}
	e.liveClassImplementation(&b, c)
	b.WriteString("ENDCLASS.\n")
	out := b.String()
	if e.codeUnits[c.Name] {
		out = strings.Replace(out, "PRIVATE SECTION.\nENDCLASS.\n", "PRIVATE SECTION.\nCLASS-DATA "+e.name("builtin.codeUnit."+c.Name)+" TYPE REF TO cl_abap_conv_out_ce.\nENDCLASS.\n", 1)
	}
	e.files[e.name(c.Name)+".clas.abap"] = originComment(c.Name, c.Source) + out
}

type body struct {
	e                            *emitter
	c                            *hir.Class
	m                            *hir.Method
	implemented                  string
	code                         strings.Builder
	locals                       map[string]string
	serial                       int
	lastInit, lastName, lastType string
	constants                    map[string]string
}

func (e *emitter) body(c *hir.Class, m *hir.Method, name string) string {
	m = e.emittedMethod(c, m)
	e.currentMethod = m
	defer func() { e.currentMethod = nil }()
	b := &body{e: e, c: c, m: m, implemented: name, locals: map[string]string{}}
	if name == "constructor" && c.Super != "" && !e.liveClasses[c.Name] {
		if _, _, _, valid := e.splitSuperConstructor(m.Body); !valid {
			e.refuse(fmt.Errorf("class %s constructor: requires a single top-level super constructor call without early return or THIS access", c.Name))
		}
	}
	if !m.Static && name != "constructor" && e.liveClasses[c.Name] {
		b.line("IF me->" + e.name("view.source") + " IS BOUND.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `emit.go: constructor: live source cannot run native initializer`.\nENDIF.")
	}
	if name == "constructor" && e.liveClasses[c.Name] {
		// Keep the original HIR untouched: Go emission shares this program.
		copy := *m
		copy.Body = e.liveConstructor(b, c, m)
		m = &copy
	}
	if m.Name == "class_constructor" && m.Static {
		flag := e.name("builtin.initialized." + c.Name)
		b.line("IF " + flag + " = abap_true.")
		b.line("RETURN.")
		b.line("ENDIF.")
		b.line(flag + " = abap_true.")
	} else if m.Static || m.Name == "constructor" && !e.liveClasses[c.Name] {
		b.initialize(c.Name)
	}
	if name == "constructor" && !m.Static && !e.liveClasses[c.Name] && e.needsImplicitSuper(c, m) {
		// TypeScript's implicit super() precedes field initializers; ABAP
		// refuses any access to ME before SUPER->CONSTRUCTOR (A4H, 2026-10-09).
		b.line("super->constructor( ).")
	}
	for _, p := range m.Params {
		if b.locals[p.Name] != "" {
			continue
		}
		n := b.temp(p.Type)
		b.locals[p.Name] = n
		b.line(n + " = " + e.param(p.Name) + ".")
	}
	body := m.Body
	if m.Name == "class_constructor" && m.Static && body != nil {
		// Literal static constants are CONSTANTS; their assignments go.
		kept := &hir.Stmt{Node: body.Node, Kind: body.Kind}
		for _, s := range body.List {
			if !e.isStaticConstantInit(c, s) {
				kept.List = append(kept.List, s)
			}
		}
		body = kept
	}
	b.stmt(body)
	return "METHOD " + name + ".\n" + b.constantDeclarations() + foldTempInits(b.code.String()) + "ENDMETHOD.\n"
}

func (e *emitter) overrideImplementation(c *hir.Class, impl, slot *hir.Method) string {
	clone := *impl
	clone.Result = e.emittedMethod(c, slot).Result
	e.currentMethod = impl
	defer func() { e.currentMethod = nil }()
	b := &body{e: e, c: c, m: &clone, implemented: e.member(slot.Name), locals: map[string]string{}}
	for j, p := range impl.Params {
		inherited := slot.Params[j]
		// A required primitive cannot represent undefined passed through the
		// inherited optional slot. Reading only ->value would silently replace
		// undefined with an initial primitive (or dereference an unbound ref).
		if inherited.Type.Kind == hir.Optional && !inherited.Type.Args[0].IsRef() && p.Type.Kind != hir.Optional {
			e.refuse(fmt.Errorf("class %s method %s parameter %s: narrowing an inherited optional primitive cannot preserve undefined", c.Name, slot.Name, p.Name))
			return ""
		}
		n := b.temp(p.Type)
		b.locals[p.Name] = n
		b.line(n + " = " + b.convert(e.param(slot.Params[j].Name), slot.Params[j].Type, p.Type) + ".")
	}
	b.stmt(impl.Body)
	return "METHOD " + b.implemented + ".\n" + b.constantDeclarations() + foldTempInits(b.code.String()) + "ENDMETHOD.\n"
}

// Explicit lazy initialization avoids eager module constructors calling a
// registry module before the runtime has registered all translated classes.
func (b *body) initialize(owner string) {
	c := b.e.classBy(owner)
	if c == nil {
		return
	}
	if !b.e.staticInit[owner] {
		return
	}
	for _, m := range c.Methods {
		if m.Name == "class_constructor" && m.Static {
			// The initializer returns at once when its flag is set; testing the
			// flag here saves a method call on every static access (A4H profile).
			b.line("IF " + b.e.name(owner) + "=>" + b.e.name("builtin.initialized."+c.Name) + " = abap_false.")
			b.line(b.e.name(owner) + "=>" + b.e.name("builtin.initialize."+c.Name) + "( ).")
			b.line("ENDIF.")
			return
		}
	}
}

// Temporaries are initialized at their evaluation point, including each loop
// iteration. Fold an immediately following assignment into its declaration;
// explicit conversions retain the HIR type rather than ABAP literal inference.
func (b *body) line(s string) {
	prefix := b.lastName + " = "
	if b.lastInit != "" && b.lastType != "REF TO object" && strings.HasPrefix(s, prefix) {
		code := b.code.String()
		b.code.Reset()
		b.code.WriteString(strings.TrimSuffix(code, b.lastInit))
		rhs := strings.TrimSuffix(strings.TrimPrefix(s, prefix), ".")
		if strings.HasPrefix(b.lastType, "REF TO ") {
			typ := strings.TrimPrefix(b.lastType, "REF TO ")
			if strings.HasPrefix(rhs, "NEW #( ") {
				rhs = strings.Replace(rhs, "NEW #", "NEW "+typ, 1)
			}
			if strings.HasPrefix(rhs, "NEW "+typ+"(") || strings.HasPrefix(rhs, "NEW "+typ+" (") {
				s = "DATA(" + b.lastName + ") = " + rhs + "."
			} else {
				s = "DATA(" + b.lastName + ") = CAST " + typ + "( " + rhs + " )."
			}
		} else {
			s = "DATA(" + b.lastName + ") = CONV " + b.lastType + "( " + rhs + " )."
		}
	}
	b.lastInit, b.lastName, b.lastType = "", "", ""
	b.code.WriteString(s + "\n")
}
func (b *body) temp(t hir.Type) string { return b.rawTemp(b.e.typ(t)) }
func (b *body) rawTemp(typ string) string {
	b.serial++
	n := fmt.Sprintf("t%d", b.serial)
	var init string
	if strings.HasPrefix(typ, "REF TO ") || strings.Contains(typ, " LENGTH ") {
		init = "DATA " + n + " TYPE " + typ + ".\nCLEAR " + n + ".\n"
	} else {
		init = "DATA(" + n + ") = VALUE " + typ + "( ).\n"
	}
	b.line(strings.TrimSuffix(init, "\n"))
	if !strings.Contains(typ, " LENGTH ") {
		b.lastInit, b.lastName, b.lastType = init, n, typ
	}
	return n
}
func (b *body) convert(value string, src, dst hir.Type) string {
	if src.Equal(dst) {
		return value
	}
	if taggedNumber(src) && (dst.Kind == hir.Number || dst.Kind == hir.I32 || dst.Kind == hir.I64) {
		n := b.temp(dst)
		b.line(n + " = " + b.e.numberBoxRuntime() + "=>finite( " + value + " ).")
		return n
	}
	if dst.Kind == hir.Dynamic {
		// A typed value into an erased `any` slot: box it.
		n := b.temp(dst)
		b.boxDynamic(n, value, src)
		return n
	}
	if src.Kind == hir.Dynamic && (dst.IsRef() || dst.Kind == hir.String || dst.Kind == hir.Number || dst.Kind == hir.Bool || dst.Kind == hir.ClassValue) {
		// An erased `any` into the implementation's type: checked unboxing.
		return b.unbox(value, dst)
	}
	if src.Kind == hir.InterfaceRef && dst.Kind == hir.InterfaceRef && !src.Equal(dst) {
		n := b.temp(dst)
		b.line(n + " ?= " + value + ".")
		return n
	}
	if (src.IsRef() || (src.Kind == hir.Optional && src.Args[0].IsRef())) && (dst.IsRef() || (dst.Kind == hir.Optional && dst.Args[0].IsRef())) {
		from, to := src, dst
		if from.Kind == hir.Optional {
			from = from.Args[0]
		}
		if to.Kind == hir.Optional {
			to = to.Args[0]
		}
		if (from.Kind == hir.ClassRef || from.Kind == hir.InterfaceRef) && (to.Kind == hir.ClassRef || to.Kind == hir.InterfaceRef) && !b.e.upcast(from, to) {
			// A checked cast through the object root (class/interface relation unknown statically).
			root := b.rawTemp("REF TO object")
			b.line(root + " = " + value + ".")
			n := b.temp(dst)
			b.line(n + " ?= " + root + ".")
			return n
		}
	}
	if dst.Kind == hir.Optional && src.Kind != hir.Optional && !dst.Args[0].IsRef() {
		n := b.temp(dst)
		b.line(n + " = NEW #( ).")
		b.line(n + "->has = abap_true.")
		b.line(n + "->value = " + value + ".")
		return n
	}
	return value
}

// receiver evaluates an object operand of a member access. `this` typed as
// the emitted class or one of its ancestors is me itself: the copy into a
// temporary would only be a same-type or upcast assignment.
func (b *body) receiver(x *hir.Expr) string {
	if x.Kind == hir.This && x.Type.Kind == hir.ClassRef && x.Type.Name != hir.RootObject {
		for c := b.c; c != nil; c = b.e.classBy(c.Super) {
			if c.Name == x.Type.Name {
				return "me"
			}
		}
	}
	return b.expr(x)
}
func (b *body) value(x *hir.Expr, dst hir.Type) string {
	// Optional numeric storage has a tagged nonfinite lane. Numeric parsers
	// never turn NaN into undefined (nor into a fabricated finite zero).
	if dst.Kind == hir.Optional && dst.Args[0].Kind == hir.Number && x.Kind == hir.RuntimeOp && (x.Op == "string.parseFloatNumber" || x.Op == "string.parseIntNumber") {
		n := b.temp(dst)
		parser := "float_optional"
		if x.Op == "string.parseIntNumber" {
			parser = "int_optional"
		}
		b.line(n + " = " + b.e.stringOpsRuntime() + "=>" + parser + "( " + b.expr(x.X) + " ).")
		return n
	}
	return b.convert(b.expr(x), x.Type, dst)
}
func (b *body) expr(x *hir.Expr) string {
	if x == nil {
		return ""
	}
	e, t := b.e, x.Type
	if e.refusals != nil {
		old := e.currentNode
		e.currentNode = &x.Node
		defer func() { e.currentNode = old }()
	}
	if x.Kind == hir.ExternCall {
		e.refuse(hir.Error{Node: x.Node, Message: "extern module call " + x.Name + " is Go-only"})
		return ""
	}
	if t.Kind == hir.Void && x.Kind == hir.PromiseAwait {
		b.awaitPromise(x, "")
		return ""
	}
	if t.Kind == hir.Void {
		if x.Kind == hir.RuntimeOp {
			// A void runtime operation at statement level (dynamic.put).
			b.runtimeOp(x, "")
			return ""
		}
		if x.Kind == hir.Seq {
			// A void sequence: its statements, then its (void) value if any.
			old := b.locals
			b.locals = map[string]string{}
			for k, v := range old {
				b.locals[k] = v
			}
			for _, stmt := range x.Stmt.List {
				b.stmt(stmt)
			}
			b.expr(x.Y)
			b.locals = old
			return ""
		}
		b.call(x, "")
		return ""
	}
	n := b.temp(t)
	if b.numberStorageExpr(x, n) {
		return n
	}
	switch x.Kind {
	case hir.PromiseFulfill:
		b.line(n + " = NEW #( ).")
		if x.X != nil && t.Args[0].Kind == hir.Void {
			b.expr(x.X)
		} else if x.X != nil {
			b.line(n + "->value = " + b.value(x.X, t.Args[0]) + ".")
		}
	case hir.PromiseReject:
		v := b.expr(x.X)
		ex := b.rawTemp("REF TO " + e.exception(x.X.Type))
		b.line(ex + " = NEW #( ).")
		b.line(ex + "->payload = " + v + ".")
		if e.promiseUsed {
			b.promisePayload(ex+"->source_value", v, x.X.Type)
		}
		b.line(n + " = NEW #( ).")
		b.line(n + "->rejection = " + ex + ".")
	case hir.PromiseAwait:
		b.awaitPromise(x, n)
	case hir.Lit:
		if x.Value == nil {
			break
		}
		lt := t
		target := n
		if t.Kind == hir.Optional {
			lt = t.Args[0]
			b.line(n + " = NEW #( ).")
			b.line(n + "->has = abap_true.")
			target = n + "->value"
		}
		switch lt.Kind {
		case hir.String:
			value, ok := b.literalString(x)
			if ok {
				b.stringLit(target, value)
			}
		case hir.Bool:
			s := "abap_false"
			value, ok := x.Value.(bool)
			if !ok {
				e.refuse(fmt.Errorf("node %d (%s): expected bool literal, got %T", x.ID, x.Source, x.Value))
				break
			}
			if value {
				s = "abap_true"
			}
			b.line(target + " = " + s + ".")
		case hir.I64:
			value, err := strconv.ParseInt(fmt.Sprint(x.Value), 10, 64)
			if err != nil {
				e.refuse(fmt.Errorf("node %d (%s): invalid I64 literal: %w", x.ID, x.Source, err))
				break
			}
			b.int8Lit(target, value)
		case hir.I32:
			// A numeric literal: a quoted one is text the runtime converts on every pass.
			value, err := strconv.ParseInt(fmt.Sprint(x.Value), 10, 32)
			if err != nil {
				e.refuse(fmt.Errorf("node %d (%s): invalid I32 literal: %w", x.ID, x.Source, err))
				break
			}
			b.line(target + " = " + strconv.FormatInt(value, 10) + ".")
		default:
			if f, err := strconv.ParseFloat(fmt.Sprint(x.Value), 64); err == nil && (math.IsNaN(f) || math.IsInf(f, 0)) {
				e.refuse(fmt.Errorf("node %d (%s): ABAP f cannot represent non-finite Number", x.ID, x.Source))
				break
			} else if err == nil && f == math.Trunc(f) && math.Abs(f) <= math.MaxInt32 && !(f == 0 && math.Signbit(f)) {
				b.line(target + " = " + strconv.FormatInt(int64(f), 10) + ".")
				break
			}
			b.line(target + " = '" + fmt.Sprint(x.Value) + "'.")
		}
	case hir.NumericMinMax:
		a := b.expr(x.X)
		op := "<"
		if x.Op == "max" {
			op = ">"
		}
		combine := func(v string) {
			if t.Kind == hir.Number {
				// Portable ABAP has no signbit; comparing zeros loses their sign.
				b.line("IF " + n + " = 0 AND " + v + " = 0.")
				b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
				b.line("ENDIF.")
			}
			b.line("IF " + v + " " + op + " " + n + ".")
			b.line(n + " = " + v + ".")
			b.line("ENDIF.")
		}
		if x.Y == nil {
			b.line(a + "->dense( ).")
			// An empty input yields Infinity, which neither ABAP f nor an
			// integer can hold: trap instead of returning a wrong value.
			b.line("IF lines( " + a + "->items ) = 0.")
			b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
			b.line("ENDIF.")
			b.line("READ TABLE " + a + "->items INDEX 1 INTO " + n + ".")
			row := b.temp(t)
			b.line("LOOP AT " + a + "->items INTO " + row + " FROM 2.")
			combine(row)
			b.line("ENDLOOP.")
		} else {
			z := b.expr(x.Y)
			// Keep the established integer output byte-for-byte.
			b.line(n + " = " + z + ".")
			if t.Kind == hir.Number {
				combine(a)
			} else {
				b.line("IF " + a + " " + op + " " + z + ".")
				b.line(n + " = " + a + ".")
				b.line("ENDIF.")
			}
		}
	case hir.CheckedNumericConvert:
		a := b.scalarNumber(x.X)
		loName := "check_lo_" + strings.ReplaceAll(strconv.FormatInt(x.Range.Min, 36), "-", "n")
		hiName := "check_hi_" + strings.ReplaceAll(strconv.FormatInt(x.Range.Max, 36), "-", "n")
		if x.X.Type.Kind == hir.Number || taggedNumber(x.X.Type) {
			low := b.constant(loName, "f", strconv.FormatInt(x.Range.Min, 10))
			high := b.constant(hiName, "f", strconv.FormatInt(x.Range.Max, 10))
			b.line("IF " + a + " <> trunc( " + a + " ) OR " + a + " < " + low + " OR " + a + " > " + high + ".")
		} else {
			low, high := b.integerConstant(loName, x.Range.Min), b.integerConstant(hiName, x.Range.Max)
			b.line("IF " + a + " < " + low + " OR " + a + " > " + high + ".")
		}
		b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
		b.line("ENDIF.")
		b.line(n + " = " + a + ".")
	case hir.NumericConvert:
		b.line(n + " = " + b.value(x.X, t) + ".")
	case hir.Local:
		b.line(n + " = " + b.locals[x.Name] + ".")
	case hir.This:
		b.line(n + " = me.")
	case hir.FieldGet:
		owner := e.fieldOwner(e.classBy(x.X.Type.Name), x.Name)
		if owner != nil && e.liveClasses[owner.Name] {
			b.line(n + " = " + b.receiver(x.X) + "->" + e.name("view.read."+owner.Name+"."+x.Name) + "( ).")
		} else {
			b.line(n + " = " + b.receiver(x.X) + "->" + e.member(x.Name) + ".")
		}
	case hir.StaticGet:
		if _, constant := e.staticConstant(x.Owner, x.Name); !constant {
			b.initialize(x.Owner)
		}
		b.line(n + " = " + e.name(x.Owner) + "=>" + e.member(x.Name) + ".")
	case hir.IndexGet:
		a := b.expr(x.X)
		i, constant := tableIndex(x.Y)
		if !constant {
			i = b.expr(x.Y)
			b.line(i + " = " + i + " + 1.")
		}
		if !t.IsRef() && t.Kind != hir.Optional {
			b.line("IF " + a + "->has( " + i + " - 1 ) = abap_false.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.")
		}
		row := n
		if t.IsRef() {
			row = b.temp(hir.Ref(hir.RootObject))
		}
		if e.arrayViews {
			b.line(a + "->load( ).")
		}
		b.line("CLEAR " + row + ".")
		b.line("READ TABLE " + a + "->items INDEX " + i + " INTO " + row + ".")
		if row != n {
			b.arrayElement(n, row, t)
		}
	case hir.New:
		if t.Kind == hir.OrderedSet && len(t.Args) == 1 && (t.Args[0].Kind == hir.Optional || t.Args[0].Kind == hir.Dynamic) {
			e.refuse(fmt.Errorf("node %d (%s): ABAP boxed Set requires value-based SameValueZero", x.ID, x.Source))
			return n
		}
		if t.Kind == hir.ClassRef && t.Name == "builtin.TypeError" {
			e.refuse(fmt.Errorf("node %d (%s): ABAP RequireObjectCoercible cannot raise an equivalent JS TypeError", x.ID, x.Source))
			return n
		}
		if t.Kind == hir.Array {
			b.line("CREATE OBJECT " + n + ".")
			if len(x.Args) == 1 {
				cnt := b.expr(x.Args[0])
				b.line("IF " + cnt + " < 0 OR " + cnt + " > 2147483646.")
				b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.")
				if x.Args[0].Type.Kind == hir.Number {
					b.line("IF " + cnt + " <> trunc( " + cnt + " ).\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.")
				}
				// allocate records holes; observation-specific adapters must
				// either preserve their semantics or refuse at the read boundary.
				count := b.temp(hir.T(hir.I32))
				b.line(count + " = " + cnt + ".")
				b.line(n + "->allocate( " + count + " ).")
			}
			break
		}
		if t.Kind == hir.RegExp {
			e.regexpUsed = true
			if raw, ok := x.Args[0].Value.(string); x.Args[0].Kind == hir.Lit && ok && regexHasClassComplement(raw) {
				e.refuse(fmt.Errorf("JavaScript regexp %q: \\S inside a character class is not expressible on the legacy POSIX path", raw))
			}
			js := regexNeedsJS(x) || e.execUsed && regexMayHaveState(x)
			original := b.value(x.Args[0], hir.T(hir.String))
			pattern, reject := original, ""
			if raw, ok := x.Args[0].Value.(string); !js && x.Args[0].Kind == hir.Lit && ok {
				flags := ""
				if len(x.Args) > 1 {
					flags, _ = x.Args[1].Value.(string)
				}
				translated, excluded := regexPattern(raw, flags)
				if translated != raw || excluded != "" {
					pattern = b.temp(hir.T(hir.String))
					b.stringLit(pattern, translated)
					if excluded != "" {
						reject = b.temp(hir.T(hir.String))
						b.stringLit(reject, excluded)
					}
				}
			}
			id := "runtime.regexp"
			if js {
				id = "runtime.regexp_js"
			}
			s := "CREATE OBJECT " + n + " TYPE " + e.name(id)
			s += " EXPORTING pattern = " + pattern
			if len(x.Args) > 1 {
				s += " flags = " + b.value(x.Args[1], hir.T(hir.String))
			} else {
				s += " flags = ``"
			}
			if reject != "" {
				s += " excluded_pattern = " + reject
			}
			if js {
				s += " checked = abap_true"
			}
			b.line(s + ".")
			if pattern != original {
				b.line(n + "->source = " + original + ".")
			}
			break
		}
		var jsonArgs []string
		class := e.classBy(t.Name)
		if class != nil && class.Structural && x.OwnKeys != nil && (e.jsonOwnKeys && class.Super == "" || e.ownAdapter(class)) {
			ctor := e.p.Constructor(t.Name)
			jsonArgs = make([]string, len(x.Args))
			evaluate := func(i int) {
				v := b.temp(ctor.Params[i].Type)
				b.line(v + " = " + b.value(x.Args[i], ctor.Params[i].Type) + ".")
				jsonArgs[i] = v
			}
			for _, key := range *x.OwnKeys {
				for i, param := range ctor.Params {
					if param.Name == key {
						evaluate(i)
						break
					}
				}
			}
			for i := range x.Args {
				if jsonArgs[i] == "" {
					evaluate(i)
				}
			}
		}
		b.initialize(t.Name)
		if c := e.classBy(t.Name); c != nil && e.wideShape(c) {
			// A wide shape has no constructor: its fields are set one by one.
			values := make([]string, len(x.Args))
			for i, a := range x.Args {
				field := c.Ctor.Body.List[i].X.Type
				if jsonArgs != nil {
					if !ctorParameterMatchesField(c, i) {
						e.refuse(fmt.Errorf("node %d (%s): JSON wide shape constructor narrowing is unproven", x.ID, x.Source))
						return n
					}
					values[i] = jsonArgs[i]
					continue
				}
				if a.Type.Kind == hir.Optional && !a.Type.Equal(field) && c.Ctor.Body.List[i].Y.Kind == hir.Narrow {
					// The constructor narrowed (checked) the optional parameter.
					values[i] = b.expr(&hir.Expr{Kind: hir.Narrow, Node: a.Node, Type: field, X: a})
				} else {
					values[i] = b.value(a, field)
				}
			}
			b.line(n + " = NEW " + strings.TrimPrefix(e.typ(t), "REF TO ") + "( ).")
			for i := range values {
				b.line(n + "->" + e.member(c.Ctor.Params[i].Name) + " = " + values[i] + ".")
			}
			b.attachOwnKeys(x, n)
			b.jsonInitializeKeys(n, x)
			break
		}
		s := n + " = NEW " + strings.TrimPrefix(e.typ(t), "REF TO ") + "( "
		args := []string{}
		ctor := e.p.Constructor(t.Name)
		for i, a := range x.Args {
			value := ""
			if jsonArgs != nil {
				value = jsonArgs[i]
			} else {
				value = b.value(a, ctor.Params[i].Type)
			}
			args = append(args, e.param(ctor.Params[i].Name)+" = "+value)
		}
		if len(args) > 0 {
			s += strings.Join(args, " ")
		}
		b.line(s + " ).")
		if e.chaiUsed && t.Kind == hir.OrderedMap && x.OwnKeys != nil {
			b.line(n + "->chai_enumerable = abap_true.")
		}
		b.attachOwnKeys(x, n)
		b.jsonInitializeKeys(n, x)
	case hir.DirectCall, hir.VirtualCall, hir.SuperCall:
		b.call(x, n)
	case hir.Binary:
		a := b.expr(x.X)
		if x.Op == "&&" || x.Op == "||" {
			b.line(n + " = " + a + ".")
			test := "abap_true"
			if x.Op == "||" {
				test = "abap_false"
			}
			b.line("IF " + a + " = " + test + ".")
			b.line(n + " = " + b.expr(x.Y) + ".")
			b.line("ENDIF.")
			break
		}

		z := b.expr(x.Y)
		if e.types["runtime.liveView"] && (x.Op == "==" || x.Op == "!=") && referenceStorage(x.X.Type) && referenceStorage(x.Y.Type) {
			left, right := b.temp(hir.T(hir.Dynamic)), b.temp(hir.T(hir.Dynamic))
			b.boxDynamic(left, a, x.X.Type)
			b.boxDynamic(right, z, x.Y.Type)
			b.line("IF " + left + " IS BOUND.\n" + n + " = " + left + "->strict_equals( " + right + " ).\nELSE.\n" + n + " = xsdbool( " + right + " IS NOT BOUND ).\nENDIF.")
			if x.Op == "!=" {
				b.line(n + " = xsdbool( " + n + " = abap_false ).")
			}
			break
		}
		if (x.Op == "/" || x.Op == "%") && (t.Kind == hir.I32 || t.Kind == hir.I64) {
			rem := b.temp(t)
			b.line(rem + " = " + a + " MOD " + z + ".")
			if x.Op == "/" {
				b.line(n + " = " + a + " DIV " + z + ".")
			} else {
				b.line(n + " = " + rem + ".")
			}
			b.line("IF " + a + " < 0 AND " + rem + " <> 0.")
			b.line("IF " + z + " > 0.")
			if x.Op == "/" {
				b.line(n + " = " + n + " + 1.")
			} else {
				b.line(n + " = " + n + " - " + z + ".")
			}
			b.line("ELSE.")
			if x.Op == "/" {
				b.line(n + " = " + n + " - 1.")
			} else {
				b.line(n + " = " + n + " + " + z + ".")
			}
			b.line("ENDIF.")
			b.line("ENDIF.")
			break
		}
		if x.Op == "%" && t.Kind == hir.Number {
			e.refuse(fmt.Errorf("node %d (%s): Number remainder requires an IEEE runtime", x.ID, x.Source))
			break
		}
		if x.Op == "/" && t.Kind == hir.Number {
			divisor, err := strconv.ParseFloat(fmt.Sprint(x.Y.Value), 64)
			if x.Y.Kind != hir.Lit || err != nil || divisor == 0 || math.IsNaN(divisor) || math.IsInf(divisor, 0) {
				e.refuse(fmt.Errorf("node %d (%s): Number division requires a non-zero literal divisor", x.ID, x.Source))
				break
			}
		}
		if (x.Op == "==" || x.Op == "!=") && x.X.Type.Kind == hir.Optional && !x.X.Type.Args[0].IsRef() {
			// Presence is independent of box identity; guard both dereferences.
			ah, zh := b.temp(hir.T(hir.Bool)), b.temp(hir.T(hir.Bool))
			for _, pair := range [][2]string{{a, ah}, {z, zh}} {
				b.line("IF " + pair[0] + " IS BOUND.")
				b.line(pair[1] + " = " + pair[0] + "->has.")
				b.line("ENDIF.")
			}
			b.line("IF " + ah + " = " + zh + ".")
			b.line("IF " + ah + " = abap_false.")
			b.line(n + " = abap_true.")
			if x.X.Type.Args[0].Kind == hir.Number {
				b.line("ELSE.\nIF " + a + "->number_special <> 0 OR " + z + "->number_special <> 0.")
				b.line(n + " = xsdbool( " + a + "->number_special = " + z + "->number_special AND " + a + "->number_special <> 1 ).")
				b.line("ELSE.\n" + n + " = xsdbool( " + a + "->value = " + z + "->value ).\nENDIF.")
			} else {
				b.line("ELSEIF " + a + "->value = " + z + "->value.")
				b.line(n + " = abap_true.")
			}
			b.line("ENDIF.")
			b.line("ENDIF.")
			if x.Op == "!=" {
				b.line("IF " + n + " = abap_true.")
				b.line(n + " = abap_false.")
				b.line("ELSE.")
				b.line(n + " = abap_true.")
				b.line("ENDIF.")
			}
			break
		}
		op := map[string]string{"==": "=", "!=": "<>", "%": "MOD"}[x.Op]
		if op == "" {
			op = x.Op
		}
		if t.Kind == hir.Bool {
			b.line(n + " = xsdbool( " + a + " " + op + " " + z + " ).")
		} else {
			b.line(n + " = " + a + " " + op + " " + z + ".")
			if x.CheckIntegerOverflow {
				b.integerOverflow(n)
			}
		}
	case hir.Unary:
		a := b.expr(x.X)
		if x.Op == "!" {
			b.line(n + " = xsdbool( " + a + " = abap_false ).")
		} else if t.Kind == hir.Number {
			// Binary 0 - x loses the sign when x is +0. JS unary negation
			// flips the IEEE sign, which Chai deep equality can observe.
			b.line(n + " = - " + a + ".")
		} else {
			b.line(n + " = 0 - " + a + ".")
			if x.CheckIntegerOverflow {
				b.integerOverflow(n)
			}
		}
	case hir.Conditional:
		a := b.expr(x.X)
		b.line("IF " + a + " = abap_true.")
		b.line(n + " = " + b.value(x.Y, t) + ".")
		b.line("ELSE.")
		b.line(n + " = " + b.value(x.Z, t) + ".")
		b.line("ENDIF.")
	case hir.InstanceOf:
		if len(e.liveClasses) > 0 && x.X.Kind == hir.RuntimeOp && x.X.Op == "dynamic.asRef" && x.X.X.Type.Kind == hir.Dynamic && x.Y == nil {
			d := b.expr(x.X.X)
			b.line("IF " + d + " IS BOUND AND " + d + "->tag = " + e.name("runtime.dynamic") + "=>tag_ref.\n" + n + " = xsdbool( " + d + "->oval IS INSTANCE OF " + e.name(x.Owner) + " ).\nENDIF.")
			break
		}
		a := b.expr(x.X)
		if x.Y != nil {
			// A dynamic class-value operand: walk the descriptor chain.
			cv := b.expr(x.Y)
			d := b.temp(hir.T(hir.ClassValue))
			b.line("IF " + a + " IS BOUND.")
			if x.X.Type.Name == hir.RootObject {
				ref := b.rawTemp("REF TO " + e.name("runtime.described"))
				b.line(ref + " ?= " + a + ".")
				a = ref
			}
			b.line(d + " = " + a + "->" + b.e.name("builtin.classOf") + "( ).")
			b.line("WHILE " + d + " IS BOUND.")
			b.line("IF " + d + " = " + cv + ".")
			b.line(n + " = abap_true.")
			b.line("EXIT.")
			b.line("ENDIF.")
			b.line(d + " = " + d + "->parent.")
			b.line("ENDWHILE.")
			b.line("ENDIF.")
			break
		}
		if e.isInterface(x.Owner) {
			// A checked cast decides interface membership on every runtime
			// (OSG-JS IS INSTANCE OF ignores interfaces: inbox 039).
			probe := b.rawTemp("REF TO " + e.name(x.Owner))
			b.line(n + " = abap_false.")
			b.line("IF " + a + " IS BOUND.")
			b.line("TRY.")
			b.line(probe + " ?= " + a + ".")
			b.line(n + " = abap_true.")
			b.line("CATCH cx_sy_move_cast_error.")
			b.line("ENDTRY.")
			b.line("ENDIF.")
			break
		}
		b.line(n + " = xsdbool( " + a + " IS BOUND AND " + a + " IS INSTANCE OF " + e.name(x.Owner) + " ).")
		if len(e.liveClasses) > 0 && x.X.Type.Kind == hir.ClassRef && e.liveClasses[x.X.Type.Name] {
			b.line("IF " + a + " IS BOUND AND " + a + "->" + e.name("view.source") + " IS BOUND.\n" + n + " = abap_false.\nENDIF.")
		}
	case hir.FreshClassOf:
		b.freshClassDescriptor(x, n)
	case hir.ClassOf:
		e.descriptors = true
		b.line(n + " = " + e.descriptorOf(x.Owner) + ".")
	case hir.Cast:
		a := b.expr(x.X)
		b.line(n + " ?= " + a + ".")
	case hir.Seq:
		old := b.locals
		b.locals = map[string]string{}
		for k, v := range old {
			b.locals[k] = v
		}
		for _, stmt := range x.Stmt.List {
			b.stmt(stmt)
		}
		b.line(n + " = " + b.value(x.Y, t) + ".")
		b.locals = old
	case hir.Narrow:
		a := b.expr(x.X)
		source := x.X.Type
		if source.Kind == hir.Optional {
			source = source.Args[0]
		}
		target := x.Type
		if target.Kind == hir.Optional {
			target = target.Args[0]
		}
		if source.Kind == hir.Array && target.Kind == hir.Array && source.Args[0].Kind == hir.Optional && !source.Args[0].Args[0].IsRef() && source.Args[0].Args[0].Equal(target.Args[0]) {
			// (T | undefined)[] narrowed to T[] (a type-predicate filter): a
			// distinct array class, so the elements are copied unwrapped.
			b.serial++
			row := fmt.Sprintf("t%d", b.serial)
			if e.arrayViews {
				b.line("IF " + a + "->source IS BOUND.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `emit.go: array mutation: graph-backed storage requires live mutation ABI`.\nENDIF.")
			}
			b.line("CREATE OBJECT " + n + ".")
			b.line("LOOP AT " + a + "->items INTO DATA(" + row + ").")
			b.line("IF " + row + " IS NOT BOUND OR " + row + "->has = abap_false.")
			b.line("RAISE EXCEPTION TYPE cx_sy_move_cast_error.")
			b.line("ENDIF.")
			b.line("APPEND " + row + "->value TO " + n + "->items.")
			b.line("ENDLOOP.")
			break
		}
		if (source.Kind == hir.ClassRef && target.Kind == hir.InterfaceRef) || (source.Kind == hir.InterfaceRef && target.Kind == hir.ClassRef) {
			// A class/interface cross cast: widen to the object root first so
			// the checked `?=` is valid whatever the static relation.
			root := b.temp(hir.Ref(hir.RootObject))
			b.line(root + " = " + a + ".")
			a = root
		}
		if x.X.Type.Kind == hir.Optional {
			base := x.X.Type.Args[0]
			if base.Kind == hir.Number {
				b.line("IF " + a + "->number_special <> 0.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.")
			}
			if !base.IsRef() {
				a += "->value"
			}
			if base.Equal(x.Type) {
				b.line(n + " = " + a + ".")
			} else {
				b.line(n + " ?= " + a + ".")
			}
		} else {
			b.line(n + " ?= " + a + ".")
		}
	case hir.IsUndefined:
		a := b.expr(x.X)
		b.line("IF " + a + " IS INITIAL.")
		b.line(n + " = abap_true.")
		if x.X.Type.Kind == hir.Optional && !x.X.Type.Args[0].IsRef() {
			b.line("ELSEIF " + a + "->has = abap_false.")
			b.line(n + " = abap_true.")
		}
		b.line("ENDIF.")
	case hir.ToBoolean:
		a := b.expr(x.X)
		at := x.X.Type
		if at.Kind == hir.Optional && !at.Args[0].IsRef() {
			b.line("IF " + a + " IS BOUND.")
			b.line("IF " + a + "->has = abap_true.")
			if at.Args[0].Kind == hir.Number {
				b.line("IF " + a + "->number_special <> 0.\n" + n + " = xsdbool( " + a + "->number_special <> 1 ).\nELSE.")
			}
			b.truth(n, a+"->value", at.Args[0])
			if at.Args[0].Kind == hir.Number {
				b.line("ENDIF.")
			}
			b.line("ENDIF.")
			b.line("ENDIF.")
		} else {
			b.truth(n, a, at)
		}
	case hir.RuntimeOp:
		b.runtimeOp(x, n)
	default:
		e.refuse(fmt.Errorf("node %d: unsupported expression %s", x.ID, x.Kind))
	}
	return n
}

// jsWhiteSpace are the Unicode code points ECMAScript trim treats as white
// space: the singletons plus the U+2000..U+200A range.
var jsWhiteSpace = []int{9, 10, 11, 12, 13, 32, 160, 5760, 8232, 8233, 8239, 8287, 12288, 65279}

// trimLoop advances lo (or retreats hi) over JavaScript white space in a.
// The one-character section is materialized first: string offsets are not
// allowed as functional-method arguments.
func (b *body) trimLoop(a, lo, hi string, leading bool) {
	code := b.temp(hir.T(hir.I32))
	idx := b.temp(hir.T(hir.I32))
	ch := b.temp(hir.T(hir.String))
	cond := lo + " < " + hi
	init := ""
	step := lo + " = " + lo + " + 1."
	if leading {
		init = idx + " = " + lo + "."
	} else {
		cond = hi + " > " + lo
		init = idx + " = " + hi + " - 1."
		step = hi + " = " + hi + " - 1."
	}
	b.line("WHILE " + cond + ".")
	b.line(init)
	b.line(ch + " = " + a + "+" + idx + "(1).")
	b.codeUnit(code, ch)
	var tests []string
	for _, cp := range jsWhiteSpace {
		tests = append(tests, fmt.Sprintf("%s = %d", code, cp))
	}
	tests = append(tests, fmt.Sprintf("%s >= 8192 AND %s <= 8202", code, code))
	b.line("IF " + strings.Join(tests, " OR ") + ".")
	b.line(step)
	b.line("ELSE.")
	b.line("EXIT.")
	b.line("ENDIF.")
	b.line("ENDWHILE.")
}

// codeUnit reads one UTF-16 unit directly. The pinned library's uccpi
// uses high-byte * 255 on OSG-JS; that is not a correct Unicode code point.
// parseInt10 implements JavaScript parseInt(s, 10): leading ECMAScript white
// space is skipped, an optional sign is read, then decimal digits up to the
// first non-digit. No digit at all yields the absent box (NaN).
func (b *body) parseInt10(n, a, length string) {
	lo := b.temp(hir.T(hir.I32))
	hi := b.temp(hir.T(hir.I32))
	b.line(lo + " = 0.")
	b.line(hi + " = " + length + ".")
	b.trimLoop(a, lo, hi, true)
	sign := b.temp(hir.T(hir.Number))
	value := b.temp(hir.T(hir.Number))
	digits := b.temp(hir.T(hir.I32))
	ch := b.temp(hir.T(hir.String))
	b.line(sign + " = 1.")
	b.line(value + " = 0.")
	b.line(digits + " = 0.")
	b.line("IF " + lo + " < " + hi + ".")
	b.line(ch + " = " + a + "+" + lo + "(1).")
	b.line("IF " + ch + " = '-'.")
	b.line(sign + " = -1.")
	b.line(lo + " = " + lo + " + 1.")
	b.line("ELSEIF " + ch + " = '+'.")
	b.line(lo + " = " + lo + " + 1.")
	b.line("ENDIF.")
	b.line("ENDIF.")
	b.line("WHILE " + lo + " < " + hi + ".")
	b.line(ch + " = " + a + "+" + lo + "(1).")
	b.line("IF " + ch + " CA '0123456789' AND " + ch + " <> ` `.")
	b.line(value + " = " + value + " * 10 + ( " + ch + " ).")
	b.line(digits + " = " + digits + " + 1.")
	b.line(lo + " = " + lo + " + 1.")
	b.line("ELSE.")
	b.line("EXIT.")
	b.line("ENDIF.")
	b.line("ENDWHILE.")
	b.line("IF " + digits + " = 0.")
	b.line("CLEAR " + n + ".")
	b.line("ELSE.")
	b.line(n + " = NEW #( ).")
	b.line(n + "->has = abap_true.")
	b.line(n + "->value = " + sign + " * " + value + ".")
	b.line("ENDIF.")
}

// localeCompareNames implements a.localeCompare(b) for strings over the
// ABAP object-name alphabet. ICU root collation orders these code units as
// "_/0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ" (verified against Node's
// Intl.Collator); a shorter prefix sorts first. Any other code unit in
// either operand raises cx_sy_range_out_of_bounds instead of guessing.
func (b *body) localeCompareNames(n, a, other, length string) {
	const alphabet = "_/0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	rank := func(s, out string) {
		b.line("IF " + s + " = ``.")
		b.line(out + " = -1.")
		b.line("ELSE.")
		b.line("FIND " + s + " IN `" + alphabet + "` MATCH OFFSET " + out + ".")
		b.line("IF sy-subrc <> 0.")
		b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
		b.line("ENDIF.")
		b.line("ENDIF.")
	}
	olen := b.temp(hir.T(hir.I32))
	idx := b.temp(hir.T(hir.I32))
	ca := b.temp(hir.T(hir.String))
	cb := b.temp(hir.T(hir.String))
	ra := b.temp(hir.T(hir.I32))
	rb := b.temp(hir.T(hir.I32))
	b.line(olen + " = strlen( " + other + " ).")
	b.line(idx + " = 0.")
	b.line(n + " = 0.")
	b.line("WHILE " + n + " = 0 AND ( " + idx + " < " + length + " OR " + idx + " < " + olen + " ).")
	b.line("CLEAR " + ca + ".")
	b.line("CLEAR " + cb + ".")
	b.line("IF " + idx + " < " + length + ".")
	b.line(ca + " = " + a + "+" + idx + "(1).")
	b.line("ENDIF.")
	b.line("IF " + idx + " < " + olen + ".")
	b.line(cb + " = " + other + "+" + idx + "(1).")
	b.line("ENDIF.")
	rank(ca, ra)
	rank(cb, rb)
	b.line("IF " + ra + " < " + rb + ".")
	b.line(n + " = -1.")
	b.line("ELSEIF " + ra + " > " + rb + ".")
	b.line(n + " = 1.")
	b.line("ENDIF.")
	b.line(idx + " = " + idx + " + 1.")
	b.line("ENDWHILE.")
}

func (b *body) codeUnit(target, ch string) {
	unit := b.rawTemp("c LENGTH 1")
	bytes := b.rawTemp("xstring")
	low := b.rawTemp("x LENGTH 1")
	high := b.rawTemp("x LENGTH 1")
	highInt := b.temp(hir.T(hir.I32))
	// uccpi is exact below 255 on every runtime; OSG-JS computes high * 255,
	// so only results from 255 up need the converter (A4H profile: one
	// CONVERT per lexer character).
	b.line(unit + " = " + ch + ".")
	b.line(target + " = cl_abap_conv_out_ce=>uccpi( " + unit + " ).")
	b.line("IF " + target + " >= 255.")
	var conv string
	if b.c != nil {
		// Creating a converter per call dominated the lexer on A4H (one per character).
		if b.e.codeUnits == nil {
			b.e.codeUnits = map[string]bool{}
		}
		b.e.codeUnits[b.c.Name] = true
		conv = b.e.name("builtin.codeUnit." + b.c.Name)
		b.line("IF " + conv + " IS NOT BOUND.")
		b.line(conv + " = cl_abap_conv_out_ce=>create( encoding = '4103' ).")
		b.line("ENDIF.")
	} else {
		conv = b.rawTemp("REF TO cl_abap_conv_out_ce")
		b.line(conv + " = cl_abap_conv_out_ce=>create( encoding = '4103' ).")
	}
	b.line(conv + "->convert( EXPORTING data = " + ch + " IMPORTING buffer = " + bytes + " ).")
	b.line(low + " = " + bytes + "(1).")
	b.line(high + " = " + bytes + "+1(1).")
	b.line(target + " = " + low + ".")
	b.line(highInt + " = " + high + ".")
	b.line(target + " = " + target + " + " + highInt + " * 256.")
	b.line("ENDIF.")
}

func (b *body) truth(n, a string, t hir.Type) {
	test := a + " IS NOT INITIAL"
	if t.Kind == hir.Dynamic {
		b.line("IF " + a + " IS BOUND.")
		b.line(n + " = " + a + "->truth( ).")
		b.line("ENDIF.")
		return
	}
	if t.Kind == hir.String {
		test = "strlen( " + a + " ) > 0"
	}
	if t.IsRef() || t.Kind == hir.Optional {
		test = a + " IS BOUND"
	}
	b.line(n + " = xsdbool( " + test + " ).")
}
func (b *body) stringLit(n, s string) {
	// String templates preserve trailing blanks. Chunk before escaping so source
	// tokens always fit the 255-byte envelope (also bounds UTF-16 source length).
	// Newline, carriage return and tab are spelled \n, \r, \t inside templates;
	// other control codes take a uccpi temporary.
	if s == "" {
		b.line(n + " = ||.")
		return
	}
	hard := func(c rune) bool {
		return (c < 32 && c != '\n' && c != '\r' && c != '\t') || c == 127 || c == 0xfeff || c == 0x2028 || c == 0x2029
	}
	soft := func(c rune) bool { return c == '\n' || c == '\r' || c == '\t' }
	first := true
	for s != "" {
		r := []rune(s)
		k, size, slash, firstSoft := 0, 0, false, -1
		for k < len(r) && !hard(r[k]) && size+len(string(r[k]))*2 <= 120 {
			if soft(r[k]) && firstSoft < 0 {
				firstSoft = k
			}
			slash = slash || r[k] == '\\'
			size += len(string(r[k])) * 2
			k++
		}
		if slash && firstSoft >= 0 {
			// A backslash forces the backquoted form, which has no escapes:
			// stop before the first control code.
			k = firstSoft
		}
		if k == 0 {
			if first {
				b.line(n + " = ||.")
			}
			char := b.rawTemp("c LENGTH 1")
			b.line(fmt.Sprintf("%s = cl_abap_conv_in_ce=>uccpi( %d ).", char, r[0]))
			b.line("CONCATENATE " + n + " " + char + " INTO " + n + " RESPECTING BLANKS.")
			s = string(r[1:])
			first = false
			continue
		}
		chunk := string(r[:k])
		s = string(r[k:])
		literal := "|" + strings.NewReplacer("\\", "\\\\", "{", "\\{", "}", "\\}", "|", "\\|", "\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(chunk) + "|"
		if strings.Contains(chunk, "\\") {
			// abaplint's lexer misreads an escaped backslash before an
			// escaped template delimiter (open-steamgate inbox 040); a
			// backquoted literal has no escapes and keeps trailing blanks.
			literal = "`" + strings.ReplaceAll(chunk, "`", "``") + "`"
		}
		if first {
			b.line(n + " = " + literal + ".")
		} else {
			b.line(n + " = " + n + " && " + literal + ".")
		}
		first = false
	}
}

// int8Lit uses only i-range literals, avoiding character-to-int8 conversion.
// Signed remainders also handle MinInt64 without negating it. Each intermediate
// is a prefix of the final value and fits int8; the target keeps arithmetic exact.
func (b *body) int8Lit(target string, value int64) {
	const base int64 = 1_000_000_000
	var parts []int64
	for value < math.MinInt32 || value > math.MaxInt32 {
		parts = append(parts, value%base)
		value /= base
	}
	b.line(fmt.Sprintf("%s = %d.", target, value))
	for i := len(parts) - 1; i >= 0; i-- {
		b.line(fmt.Sprintf("%s = %s * %d.", target, target, base))
		// Subtraction avoids a binary plus followed by a unary minus in ABAP.
		if parts[i] < 0 {
			b.line(fmt.Sprintf("%s = %s - %d.", target, target, -parts[i]))
		} else {
			b.line(fmt.Sprintf("%s = %s + %d.", target, target, parts[i]))
		}
	}
}

func (b *body) literalString(x *hir.Expr) (string, bool) {
	value, ok := x.Value.(string)
	if !ok {
		b.e.refuse(fmt.Errorf("node %d (%s): expected string literal, got %T", x.ID, x.Source, x.Value))
	}
	return value, ok
}

func (b *body) call(x *hir.Expr, n string) {
	e := b.e
	owner := x.Owner
	recv := ""
	var m *hir.Method
	if x.Kind == hir.SuperCall {
		owner = b.c.Super
		recv = "super->"
	} else if x.X != nil {
		recv = b.receiver(x.X) + "->"
		owner = x.X.Type.Name
	} else {
		recv = e.name(owner) + "=>"
	}
	callName := x.Name
	if x.Kind == hir.SuperCall {
		if original, _, specialized := strings.Cut(callName, "_instantiated_"); specialized {
			callName = original
		}
	}
	m, _ = e.method(e.classBy(owner), callName)
	if m != nil {
		m = e.emittedMethod(e.classBy(owner), m)
	}
	member := e.member(callName)
	if x.Kind == hir.SuperCall && x.Name == "constructor" {
		m = e.p.Constructor(owner)
		member = "constructor"
	}
	if x.Kind == hir.SuperCall && member != b.implemented && !e.overriddenBelow(b.c.Name, callName) {
		// No class from here down redefines the member: virtual dispatch on
		// me reaches exactly the inherited implementation super names.
		recv = "me->"
	} else if x.Kind == hir.SuperCall && member != b.implemented {
		e.refuse(fmt.Errorf("node %d (%s): unsupported super call to %s from %s: SUPER-> can only call the previous implementation of the same method", x.ID, x.Source, callName, b.m.Name))
		return
	}
	if m == nil {
		for _, i := range e.p.Interfaces {
			if i.Name == owner {
				for _, f := range i.Methods {
					if f.Name == x.Name {
						m = f
					}
				}
				// Interface-typed receivers use the unqualified interface method.
			}
		}
	}
	args := []string{}
	for i, a := range x.Args {
		args = append(args, e.param(m.Params[i].Name)+" = "+b.value(a, m.Params[i].Type))
	}
	target := n
	if x.Kind == hir.SuperCall && n != "" && !x.Type.Equal(m.Result) {
		target = b.temp(m.Result)
	}
	s := recv + member + "( " + strings.Join(args, " ") + " )"
	if n != "" {
		s = target + " = " + s
	}
	b.line(s + ".")
	if target != n {
		b.line(n + " ?= " + target + ".")
	}
}

// literalIndex is the integer value of an index expression that is a numeric
// literal holding an exact integer within i32 (directly or via number.index).
func literalIndex(x *hir.Expr) (int64, bool) {
	if x == nil {
		return 0, false
	}
	if x.Kind == hir.RuntimeOp && x.Op == "number.index" {
		x = x.X
		if x == nil || x.Type.Kind != hir.Number {
			return 0, false
		}
	} else if x.Type.Kind != hir.I32 {
		return 0, false
	}
	if x.Kind != hir.Lit || x.Value == nil {
		return 0, false
	}
	var k int64
	switch v := x.Value.(type) {
	case float64:
		if v != math.Trunc(v) || v < -2147483648 || v > 2147483647 {
			return 0, false
		}
		k = int64(v)
	default:
		// Plain decimal digits only; anything else keeps the generic path.
		parsed, err := strconv.ParseInt(fmt.Sprint(x.Value), 10, 64)
		if err != nil {
			return 0, false
		}
		k = parsed
	}
	if k < -2147483648 || k > 2147483647 {
		return 0, false
	}
	return k, true
}

// tableIndex is the constant 1-based table index of a literal JavaScript
// index, when its +1 cannot overflow i.
func tableIndex(x *hir.Expr) (string, bool) {
	k, ok := literalIndex(x)
	if !ok || k < 0 || k >= 2147483647 {
		return "", false
	}
	return strconv.FormatInt(k+1, 10), true
}

func (b *body) runtimeOp(x *hir.Expr, n string) {
	if x.Op == "array.spread0" {
		b.e.refuse(fmt.Errorf("node %d (%s): ABAP array spread with undefined values is unsupported", x.ID, x.Source))
		return
	}
	if x.X != nil && x.X.Type.Kind == hir.OrderedSet && len(x.X.Type.Args) == 1 && (x.X.Type.Args[0].Kind == hir.Optional || x.X.Type.Args[0].Kind == hir.Dynamic) {
		b.e.refuse(fmt.Errorf("node %d (%s): ABAP boxed Set requires value-based SameValueZero", x.ID, x.Source))
		return
	}

	if b.numberStorageRuntime(x, n) {
		return
	}
	// Immediate boxing does not change a native string array's ToPrimitive.
	// Keep evaluation once and use the existing checked, dense comma join;
	// erased refs, other element types and hooks stay outside this proof.
	if x.Op == "dynamic.toString" && x.X != nil && x.X.Kind == hir.RuntimeOp && x.X.Op == "dynamic.of" && x.X.X != nil && x.X.X.Type.Kind == hir.Array && len(x.X.X.Type.Args) == 1 && x.X.X.Type.Args[0].Kind == hir.String {
		a := b.expr(x.X.X)
		b.line("IF " + a + " IS BOUND.")
		b.line(n + " = " + a + "->join( VALUE #( ) ).")
		b.line("ELSE.\n" + n + " = `undefined`.\nENDIF.")
		return
	}
	if b.constSetHas(x, n) || b.replaceFreshCharacterRegex(x, n) {
		return
	}
	if k, ok := literalIndex(x); ok {
		// A literal index needs neither the f temporary nor saturation.
		b.line(n + " = " + strconv.FormatInt(k, 10) + ".")
		return
	}
	if x.Op == "string.parseIntNumber" {
		value, ok := finiteParseIntLiteral(x.X)
		if !ok {
			if x.X.Kind == hir.Lit {
				b.e.refuse(fmt.Errorf("node %d (%s): ABAP parseInt Number requires a finite safe-integer string literal; nonfinite/unsafe/negative-zero literals are unsupported", x.ID, x.Source))
				return
			}
			a := b.expr(x.X)
			b.parseIntNumber(n, a)
			return
		}
		// The literal has no effects. Prove its Number value before emission;
		// ABAP never fabricates a finite replacement for NaN or Infinity.
		b.line(n + " = " + b.expr(hir.L(hir.T(hir.Number), value)) + ".")
		return
	}
	a := b.expr(x.X)
	if x.Op == "number.ceil" {
		b.numberCeil(x, n, a)
		return
	}
	if x.Op == "number.isNaN" {
		// The ABAP Number domain refuses NaN at every producing boundary.
		// b.expr still evaluates the operand once for its effects.
		b.line(n + " = abap_false.")
		return
	}
	args := []string{}
	ps, _, _ := hir.RuntimeSignature(x.Op, x.X.Type)
	if x.Op == "json.stringifyConfig" {
		ps = []hir.Type{hir.T(hir.Optional, hir.T(hir.Dynamic)), hir.T(hir.Optional, hir.T(hir.String))}
	}
	for i, v := range x.Args {
		typ := v.Type
		if i < len(ps) {
			typ = ps[i]
		}
		args = append(args, b.value(v, typ))
	}
	if x.Op == "map.get" && x.X.Type.Args[1].Kind == hir.Optional {
		// Keep the physical getter ABI: its outer box records membership.
		// JS lookup exposes the stored optional, with absence when no box
		// was returned. The generated getter sets has on every returned box.
		lookup := b.temp(hir.T(hir.Optional, x.X.Type.Args[1]))
		b.line(lookup + " = " + a + "->get( p0 = " + args[0] + " ).")
		b.line("IF " + lookup + " IS BOUND.")
		b.line(n + " = " + lookup + "->value.")
		b.line("ENDIF.")
		return
	}
	if x.Op == "array.matchIndex" {
		b.e.matchUsed = true
		b.line("IF " + a + "->match_has_index <> abap_true.")
		b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
		b.line("ENDIF.")
		b.line(n + " = " + a + "->match_index.")
		return
	}
	if x.Op == "regexp.lastIndex" {
		b.line(n + " = " + a + "->last_index.")
		return
	}
	if x.Op == "regexp.setLastIndex" {
		v := args[0]
		b.line("IF " + v + " <> trunc( " + v + " ) OR " + v + " < -2147483648 OR " + v + " > 2147483647.")
		b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
		b.line("ENDIF.")
		b.line(a + "->last_index = " + v + ".")
		return
	}
	if x.Op == "regexp.exec" {
		b.e.matchUsed = true
		b.e.execUsed = true
		b.line(n + " = " + b.e.name("runtime.regexp_js") + "=>exec_legacy( p0 = " + a + " p1 = " + args[0] + " ).")
		return
	}
	if x.Op == "string.match" {
		b.e.matchUsed = true
		b.line(n + " = " + args[0] + "->match( " + a + " ).")
		return
	}
	if x.Op == "i64.remainder2" {
		b.line(n + " = " + a + " MOD 2.")
		b.line("IF " + a + " < 0 AND " + n + " <> 0.")
		b.line(n + " = " + n + " - 2.")
		b.line("ENDIF.")
		return
	}
	if x.Op == "number.remainder2" {
		// Division/multiplication by two are exact for finite binary64.
		// Truncating the quotient and subtracting gives JS signed remainder.
		q := b.temp(hir.T(hir.Number))
		b.line(q + " = " + a + " / 2.")
		b.line(q + " = trunc( " + q + " ).")
		b.line(q + " = " + q + " * 2.")
		b.line(n + " = " + a + " - " + q + ".")
		return
	}
	if x.Op == "number.fromI32" {
		b.line(n + " = " + a + ".")
		return
	}
	if x.Op == "number.index" {
		// ToIntegerOrInfinity: saturating indices preserves clamping/bounds
		// semantics while preventing target i32 conversion overflow.
		upper := b.constant("i32_upper", "f", "2147483647")
		lower := b.constant("i32_lower", "f", "-2147483648")
		b.line("IF " + a + " > " + upper + ".")
		b.line(n + " = " + upper + ".")
		b.line("ELSEIF " + a + " < " + lower + ".")
		b.line(n + " = " + lower + ".")
		b.line("ELSE.")
		b.line(n + " = trunc( " + a + " ).")
		b.line("ENDIF.")
		return
	}
	if x.Op == "number.toString" {
		// This phase supports decimal rendering of safe integers. Other
		// dynamic values trap rather than silently adopting ABAP formatting.
		limit := b.constant("safe_integer", "f", "9007199254740991")
		b.line("IF " + a + " <> trunc( " + a + " ) OR " + a + " > " + limit + " OR " + a + " < 0 - " + limit + ".")
		b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
		b.line("ENDIF.")
		integer := b.temp(hir.T(hir.I64))
		b.line(integer + " = " + a + ".")
		b.line(n + " = |{ " + integer + " }|.")
		return
	}
	if x.X.Type.Kind == hir.String && strings.HasPrefix(x.Op, "string.") {
		if x.Op == "string.compareRegistryKey" || x.Op == "string.compareObjectName" {
			b.e.orderingSubsetRuntime()
			method := "rule_key"
			if x.Op == "string.compareObjectName" {
				method = "object_name"
			}
			b.line(n + " = " + b.e.name("runtime.orderingSubset") + "=>" + method + "( p0 = " + a + " p1 = " + args[0] + " ).")
			return
		}
		length := b.temp(hir.T(hir.I32))
		b.line(length + " = strlen( " + a + " ).")
		// ABAP strlen and sections count UTF-16 code units, like JavaScript.
		// Runtime implementations that count runes are a runtime gap, not a
		// reason to convert the entire receiver on every access.
		switch x.Op {
		case "string.length":
			b.line(n + " = " + length + ".")
		case "string.concat":
			b.line(n + " = |{ " + a + " }{ " + args[0] + " }|.")
		case "string.slice":
			lo, hi := args[0], args[1]
			for _, v := range []string{lo, hi} {
				b.line("IF " + v + " < 0.")
				b.line(v + " = " + length + " + " + v + ".")
				b.line("ENDIF.")
				b.line(v + " = COND i( WHEN " + v + " < 0 THEN 0 WHEN " + v + " > " + length + " THEN " + length + " ELSE " + v + " ).")
			}
			b.line("IF " + hi + " > " + lo + ".")
			b.line(length + " = " + hi + " - " + lo + ".")
			b.line(n + " = " + a + "+" + lo + "(" + length + ").")
			b.line("ELSE.")
			b.line("CLEAR " + n + ".")
			b.line("ENDIF.")
		case "string.repeat":
			count := b.temp(hir.T(hir.Number))
			b.line(count + " = trunc( " + args[0] + " ).")
			b.line("IF " + count + " < 0.")
			b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
			b.line("ENDIF.")
			b.line("CLEAR " + n + ".")
			b.line("IF " + a + " IS NOT INITIAL.")
			b.line("IF " + count + " > 2147483647 OR " + count + " * strlen( " + a + " ) > 2147483647.")
			b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
			b.line("ENDIF.")
			b.line(n + " = repeat( val = " + a + " occ = CONV i( " + count + " ) ).")
			b.line("ENDIF.")
		case "string.repeatIndent":
			// The enclosing source certificate proves non-negative integer indentation.
			// The helper remains fail-closed if called without that domain proof.
			b.line("IF " + args[0] + " < 0 OR " + args[0] + " > 2147483647 OR " + args[0] + " <> trunc( " + args[0] + " ).")
			b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
			b.line("ENDIF.")
			index := b.temp(hir.T(hir.I32))
			b.line(index + " = 0.")
			b.line("CLEAR " + n + ".")
			b.line("WHILE " + index + " < " + args[0] + ".")
			b.line(n + " = |{ " + n + " }{ " + a + " }|.")
			b.line(index + " = " + index + " + 1.")
			b.line("ENDWHILE.")

		case "string.substring":
			// JavaScript substring clamps both indices into [0, length] and
			// swaps them when start is past end. Indices count UTF-16 units.
			lo, hi := args[0], args[1]
			for _, v := range []string{lo, hi} {
				b.line(v + " = COND i( WHEN " + v + " < 0 THEN 0 WHEN " + v + " > " + length + " THEN " + length + " ELSE " + v + " ).")
			}
			b.line("IF " + lo + " > " + hi + ".")
			b.line(length + " = " + lo + ".")
			b.line(lo + " = " + hi + ".")
			b.line(hi + " = " + length + ".")
			b.line("ENDIF.")
			b.line(length + " = " + hi + " - " + lo + ".")
			b.line(n + " = " + a + "+" + lo + "(" + length + ").")
		case "string.charAt":
			// charAt never raises: out of range is the empty string.
			b.line("IF " + args[0] + " < 0 OR " + args[0] + " >= " + length + ".")
			b.line("CLEAR " + n + ".")
			b.line("ELSE.")
			b.line(n + " = " + a + "+" + args[0] + "(1).")
			b.line("ENDIF.")
		case "string.charCodeAt":
			if x.X.Kind == hir.Lit && x.Args[0].Kind == hir.Lit {
				value, ok := b.literalString(x.X)
				if !ok {
					return
				}
				units := utf16.Encode([]rune(value))
				index, err := strconv.Atoi(fmt.Sprint(x.Args[0].Value))
				if err == nil && index >= 0 && index < len(units) {
					b.line(fmt.Sprintf("%s = %d.", n, units[index]))
					return
				}
			}
			// Out of range JavaScript returns NaN, which the Number domain
			// does not carry; raise instead of returning a wrong code unit
			// (a documented divergence, like Number division by zero).
			b.line("IF " + args[0] + " < 0 OR " + args[0] + " >= " + length + ".")
			b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
			b.line("ENDIF.")
			// The section is materialized first: string offsets are not
			// allowed as functional-method arguments.
			ch := b.temp(hir.T(hir.String))
			b.line(ch + " = " + a + "+" + args[0] + "(1).")
			b.codeUnit(n, ch)
		case "string.substr":
			// Legacy substr: a negative start counts from the end, a
			// non-positive length is the empty string, the result is cut at
			// the end of the receiver.
			start, count := args[0], args[1]
			b.line("IF " + start + " < 0.")
			b.line(start + " = " + start + " + " + length + ".")
			b.line("IF " + start + " < 0.")
			b.line(start + " = 0.")
			b.line("ENDIF.")
			b.line("ENDIF.")
			b.line("IF " + start + " > " + length + " OR " + count + " <= 0.")
			b.line("CLEAR " + n + ".")
			b.line("ELSE.")
			b.line("IF " + count + " > " + length + " - " + start + ".")
			b.line(count + " = " + length + " - " + start + ".")
			b.line("ENDIF.")
			b.line(n + " = " + a + "+" + start + "(" + count + ").")
			b.line("ENDIF.")
		case "string.trim", "string.trimStart", "string.trimEnd":
			lo := b.temp(hir.T(hir.I32))
			hi := b.temp(hir.T(hir.I32))
			b.line(lo + " = 0.")
			b.line(hi + " = " + length + ".")
			if x.Op != "string.trimEnd" {
				b.trimLoop(a, lo, hi, true)
			}
			if x.Op != "string.trimStart" {
				b.trimLoop(a, lo, hi, false)
			}
			b.line("IF " + hi + " > " + lo + ".")
			b.line(length + " = " + hi + " - " + lo + ".")
			b.line(n + " = " + a + "+" + lo + "(" + length + ").")
			b.line("ELSE.")
			b.line("CLEAR " + n + ".")
			b.line("ENDIF.")
		case "string.toUpperCase":
			// Printable ASCII upper-cases with TRANSLATE alone (JS's special
			// cases are all outside ASCII); anything else goes through the
			// one full mapping in runtime.upper instead of ~100 inline REPLACEs.
			b.line(n + " = " + a + ".")
			b.line("IF " + n + " CO " + asciiPrintable + ".")
			b.line("TRANSLATE " + n + " TO UPPER CASE.")
			b.line("ELSE.")
			b.line(n + " = " + b.e.upperRuntime() + "=>full( " + n + " ).")
			b.line("ENDIF.")
		case "string.toLowerCase":
			b.line(n + " = " + a + ".")
			b.line("TRANSLATE " + n + " TO LOWER CASE.")
		case "string.startsWith", "string.endsWith":
			count := b.temp(hir.T(hir.I32))
			offset := b.temp(hir.T(hir.I32))
			section := b.temp(hir.T(hir.String))
			b.line(count + " = strlen( " + args[0] + " ).")
			b.line("IF " + count + " = 0.")
			b.line(n + " = abap_true.")
			b.line("ELSEIF " + count + " <= " + length + ".")
			b.line(offset + " = 0.")
			if x.Op == "string.endsWith" {
				b.line(offset + " = " + length + " - " + count + ".")
			}
			b.line(section + " = " + a + "+" + offset + "(" + count + ").")
			b.line("IF " + section + " = " + args[0] + ".")
			b.line(n + " = abap_true.")
			b.line("ENDIF.")
			b.line("ENDIF.")
		case "string.indexOf":
			b.line("IF " + args[0] + " IS INITIAL.")
			b.line(n + " = 0.")
			b.line("ELSE.")
			b.line("FIND " + args[0] + " IN " + a + " MATCH OFFSET " + n + ".")
			b.line("IF sy-subrc <> 0.")
			b.line(n + " = -1.")
			b.line("ENDIF.")
			b.line("ENDIF.")
		case "string.padStart":
			b.line(n + " = " + b.e.stringOpsRuntime() + "=>pad( text = " + a + " target = " + args[0] + " fill = " + args[1] + " ).")
		case "string.parseFloatNumber":
			b.line(n + " = " + b.e.stringOpsRuntime() + "=>float_prefix( " + a + " ).")
		case "string.toNumber":
			b.line(n + " = " + b.e.stringOpsRuntime() + "=>to_number( " + a + " ).")
		case "string.sha1Hex":
			b.line(n + " = " + b.e.stringOpsRuntime() + "=>sha1( " + a + " ).")
		case "string.splitRegex":
			b.line(n + " = " + b.e.stringOpsRuntime() + "=>split( text = " + a + " pattern = " + args[0] + " limit = " + args[1] + " ).")
		case "string.split":
			b.stringSplit(n, a, args[0], length)
		case "string.replaceRegex":
			if x.Args[1].Kind != hir.Lit {
				b.line("IF " + args[1] + " CS '$'.")
				b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
				b.line("ENDIF.")
			}
			b.line(n + " = " + args[0] + "->replace( p0 = " + a + " p1 = " + args[1] + " ).")
		case "string.replaceAll":
			b.line(n + " = " + a + ".")
			b.line("REPLACE ALL OCCURRENCES OF " + args[0] + " IN " + n + " WITH " + args[1] + ".")
		case "string.replaceFirst":
			if x.Args[1].Kind != hir.Lit {
				b.line("IF " + args[1] + " CS '$'.")
				b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
				b.line("ENDIF.")
			}
			// JavaScript replace with a string pattern: the first occurrence
			// only; an empty needle inserts before the first code unit.
			b.line(n + " = " + a + ".")
			b.line("IF " + args[0] + " = ``.")
			b.line(n + " = |{ " + args[1] + " }{ " + a + " }|.")
			b.line("ELSE.")
			b.line("REPLACE FIRST OCCURRENCE OF " + args[0] + " IN " + n + " WITH " + args[1] + ".")
			b.line("ENDIF.")
		case "string.at":
			// s[i]: absent outside [0, length), otherwise one UTF-16 unit.
			b.line("IF " + args[0] + " < 0 OR " + args[0] + " >= " + length + ".")
			b.line("CLEAR " + n + ".")
			b.line("ELSE.")
			b.line(n + " = NEW #( ).")
			b.line(n + "->has = abap_true.")
			b.line(n + "->value = " + a + "+" + args[0] + "(1).")
			b.line("ENDIF.")
		case "string.parseInt10", "string.parseInt10i64":
			b.parseInt10(n, a, length)
		case "string.localeCompareNames":
			b.localeCompareNames(n, a, args[0], length)
		default:
			// A new verified HIR string op is not automatically implemented
			// on every target. Never leave its result silently initial.
			b.e.refuse(fmt.Errorf("node %d (%s): unsupported ABAP runtime op %s", x.ID, x.Source, x.Op))
		}
		return
	}
	if x.Op == "i32.toString" || x.Op == "i64.toString" {
		// A string template renders an i exactly like JavaScript String(int32).
		b.line(n + " = |{ " + a + " }|.")
		return
	}
	switch x.Op {
	case "json.stringifySubset":
		if !b.e.jsonSubsetType(x.X.Type) {
			b.e.refuse(fmt.Errorf("node %d (%s): json.stringifySubset ABAP own-property order/toJSON is unproven for %s", x.ID, x.Source, x.X.Type))
			return
		}
		b.e.configJSON = true
		b.e.subsetJSONRoots = append(b.e.subsetJSONRoots, x.X.Type)
		w := b.rawTemp("REF TO " + b.e.name("runtime.configJSON"))
		b.line("CREATE OBJECT " + w + ".")
		b.line(n + " = " + w + "->" + b.e.name("json.write.subset."+x.X.Type.String()) + "( " + a + " ).")
		// JSON.stringify(undefined/function) has no string result. The HIR
		// string ABI cannot represent it, so refuse rather than return empty.
		b.line("IF " + n + " IS INITIAL.")
		b.line("RAISE EXCEPTION TYPE " + b.e.name("exception.RegistryJSONSubsetError") + ".")
		b.line("ENDIF.")
		return
	case "json.indent":
		b.e.jsonIndentRuntime()
		b.line(n + " = " + b.e.name("runtime.jsonIndent") + "=>format( p0 = " + a + " p1 = " + args[0] + " ).")
		return
	case "json.stringifyConfig":
		b.e.configJSON = true
		w := b.rawTemp("REF TO " + b.e.name("runtime.configJSON"))
		b.line("IF " + a + " IS NOT BOUND.")
		b.line("RAISE EXCEPTION TYPE " + b.e.name("exception.RegistryJSONSubsetError") + ".")
		b.line("ENDIF.")
		if c := b.e.classBy("src/version.ts.VersionNew"); c != nil {
			v := b.rawTemp("REF TO " + b.e.name(c.Name))
			b.line("IF ( " + args[1] + " IS NOT BOUND OR " + args[1] + "->has = abap_false ) AND " + args[0] + " IS BOUND.")
			b.line("IF " + args[0] + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_ref.")
			b.line("TRY.")
			b.line(v + " ?= " + args[0] + "->oval.")
			b.line("CATCH cx_sy_move_cast_error.")
			b.line("CLEAR " + v + ".")
			b.line("ENDTRY.")
			b.line("IF " + v + " IS BOUND.")
			if b.e.materialized[c.Name] {
				b.line("IF " + v + "->" + b.e.name("builtin.materializedSource") + " IS NOT BOUND.")
			}
			b.line("RAISE EXCEPTION TYPE " + b.e.name("exception.RegistryJSONSubsetError") + ".")
			if b.e.materialized[c.Name] {
				b.line("ENDIF.")
			}
			b.line("ENDIF.\nENDIF.\nENDIF.")
		}
		b.line("CREATE OBJECT " + w + ".")
		b.line(n + " = " + w + "->" + b.e.name("json.write."+x.X.Type.String()) + "( " + a + " ).")
		return
	case "lsp.isTextDocumentEdit", "lsp.isRenameFile", "lsp.isCreateFile", "lsp.isDeleteFile":
		guard := b.e.lspGuardsRuntime()
		if x.Op == "lsp.isTextDocumentEdit" {
			b.line(n + " = " + guard + "=>text_document( " + a + " ).")
		} else if x.Op == "lsp.isRenameFile" {
			b.line(n + " = " + guard + "=>rename( p0 = " + a + " condition = " + args[0] + " ).")
		} else {
			kind, option1, option2 := "create", "overwrite", "ignoreIfExists"
			if x.Op == "lsp.isDeleteFile" {
				kind, option1, option2 = "delete", "recursive", "ignoreIfNotExists"
			}
			b.line(n + " = " + guard + "=>resource( p0 = " + a + " kind = `" + kind + "` option1 = `" + option1 + "` option2 = `" + option2 + "` condition = " + args[0] + " ).")
		}
		return
	case "dynamic.call0":
		b.nativeCall0(x, n, a)
		return
	case "dynamic.keys":
		b.e.ownKeysRuntime()
		b.line(n + " = " + b.e.name("runtime.ownKeys") + "=>keys( " + a + " ).")
		return
	case "dynamic.classView":
		b.liveView(x, n, a)
		return
	case "dynamic.arrayView":
		b.arrayView(x.Type, n, a)
		return
	case "dynamic.materialize":
		b.line(n + " = " + b.e.materializer(x.Type) + "=>project( " + a + " ).")
		return
	case "json.parseSubset":
		b.e.jsonSubsetRuntime()
		b.e.errorUsed = true
		b.line("TRY.")
		b.line(n + " = " + b.e.name("runtime.jsonSubset") + "=>parse( " + a + " ).")
		b.line("CATCH " + b.e.name("exception.JSONSyntaxError") + ".")
		message := b.temp(hir.T(hir.Optional, hir.T(hir.String)))
		b.line(message + " = NEW #( ).\n" + message + "->has = abap_true.\n" + message + "->value = `Invalid JSON`.")
		syntax := b.e.name("builtin.SyntaxError")
		if b.e.classBy("builtin.SyntaxError") == nil {
			b.e.files[syntax+".clas.abap"] = "CLASS " + syntax + " DEFINITION PUBLIC INHERITING FROM " + b.e.name("builtin.Error") + " CREATE PUBLIC.\nPUBLIC SECTION.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + syntax + " IMPLEMENTATION.\nENDCLASS.\n"
		}
		payload := b.temp(hir.Ref("builtin.SyntaxError"))
		b.line(payload + " = NEW " + syntax + "( " + b.e.param("message") + " = " + message + " ).")
		// Ordinary source catches bind an Error; settled async catches bind
		// Dynamic through the shared source_value slot. Preserve both lanes.
		err := b.rawTemp("REF TO " + b.e.exception(hir.Ref("builtin.Error")))
		b.line(err + " = NEW #( ).\n" + err + "->payload = " + payload + ".")
		if b.e.promiseUsed {
			boxed := b.temp(hir.T(hir.Dynamic))
			b.boxDynamic(boxed, payload, hir.Ref("builtin.SyntaxError"))
			b.line(err + "->source_value = " + boxed + ".")
		}
		b.line("RAISE EXCEPTION " + err + ".\nENDTRY.")
		return
	case "json5.parse":
		b.e.json5SubsetRuntime()
		b.line(n + " = " + b.e.name("runtime.json5Subset") + "=>parse( " + a + " ).")
		return
	case "xml.validate":
		b.e.xmlValidationRuntime()
		b.line(n + " = " + b.e.name("runtime.xmlValidation") + "=>validate( " + a + " ).")
		return
	case "object.get", "object.hasOwn", "object.has":
		if x.X.Type.Name == hir.RootObject {
			if x.Op != "object.get" {
				b.erasedPropertyHas(x, n, a, args[0])
				return
			}
			b.e.refuse(fmt.Errorf("node %d (%s): erased structural %s is not emitted on ABAP", x.ID, x.Source, x.Op))
			return
		}
		method := map[string]string{"object.get": "get", "object.hasOwn": "own", "object.has": "has"}[x.Op]
		b.line(n + " = " + b.e.propertyLookup(x.X.Type) + "=>" + method + "( p0 = " + a + " key = " + args[0] + " ).")
		return
	case "xml.parseWithErrors":
		b.line(n + " = " + b.e.xmlWithErrorsRuntime() + "=>parse( " + a + " ).")
		return
	case "xml.parseSubset":
		b.e.xmlSubsetRuntime()
		b.line(n + " = " + b.e.name("runtime.xmlSubset") + "=>parse( " + a + " ).")
		return
	case "dynamic.isNullish":
		b.line("IF " + a + " IS NOT BOUND.")
		b.line(n + " = abap_true.")
		b.line("ELSE.")
		b.line(n + " = xsdbool( " + a + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_null ).")
		b.line("ENDIF.")
		return
	case "dynamic.null":
		b.line("CREATE OBJECT " + n + ".")
		b.line(n + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_null.")
		return
	case "dynamic.get", "dynamic.put", "dynamic.delete", "dynamic.strictEquals", "dynamic.asBoolean":
		if x.Op == "dynamic.strictEquals" {
			b.line("IF " + a + " IS NOT BOUND.")
			b.line(n + " = xsdbool( " + args[0] + " IS NOT BOUND ).")
			b.line("ELSE.")
			b.line(n + " = " + a + "->strict_equals( " + args[0] + " ).")
			b.line("ENDIF.")
			return
		}
		method := map[string]string{"dynamic.get": "get", "dynamic.put": "put", "dynamic.delete": "delete", "dynamic.asBoolean": "as_boolean"}[x.Op]
		params := []string{}
		for i, arg := range args {
			params = append(params, fmt.Sprintf("p%d = %s", i, arg))
		}
		if x.Type.Kind == hir.Void {
			b.line(a + "->" + method + "( " + strings.Join(params, " ") + " ).")
		} else {
			b.line(n + " = " + a + "->" + method + "( " + strings.Join(params, " ") + " ).")
		}
		return
	case "clock.telemetry":
		b.e.telemetryRuntime()
		b.line(n + " = " + b.e.name("runtime.telemetry") + "=>now( ).")
		return
	case "object.checkedClassView":
		// A nominal assertion is not a structural adapter: preserve the
		// original object and let a failed downcast raise, just like castRef.
		b.line(n + " ?= " + a + ".")
		return
	case "object.classOf":
		b.e.descriptors = true
		optional := x.X.Type.Kind == hir.Optional
		operand := x.X.Type
		if optional {
			operand = operand.Args[0]
			b.line("IF " + a + " IS BOUND.")
		}
		if operand.Kind == hir.ClassRef && operand.Name == hir.RootObject {
			ref := b.rawTemp("REF TO " + b.e.name("runtime.described"))
			trap := b.e.classRefusal()
			b.line("IF " + a + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + trap + ".\nENDIF.")
			b.line("TRY.\n" + ref + " ?= " + a + ".\nCATCH cx_sy_move_cast_error.\nRAISE EXCEPTION TYPE " + trap + ".\nENDTRY.")
			b.line(n + " = " + ref + "->" + b.e.name("builtin.nativeConstructor") + "( ).")
		} else {
			b.line(n + " = " + a + "->" + b.e.name("builtin.classOf") + "( ).")
		}
		if optional {
			b.line("ENDIF.")
		}
		return
	case "classvalue.name":
		b.line(n + " = " + a + "->name.")
		return
	case "classvalue.has":
		b.line(n + " = " + a + "->has( p0 = " + args[0] + " ).")
		return
	case "classvalue.newArgs":
		b.e.descriptors = true
		b.e.constructorArgs = true
		obj := b.rawTemp("REF TO object")
		b.line(obj + " = " + b.e.name("runtime.classvalue.factory") + "=>new_args( p0 = " + a + " p1 = " + args[0] + " ).")
		b.line(n + " ?= " + obj + ".")
		return
	case "classvalue.new":
		b.e.descriptors = true
		obj := b.rawTemp("REF TO object")
		b.line(obj + " = " + b.e.name("runtime.classvalue.factory") + "=>new( p0 = " + a + " ).")
		b.line(n + " ?= " + obj + ".")
		return
	case "chai.box":
		b.chaiBox(n, a, x.X.Type)
		if x.X.Type.Kind == hir.Number {
			zero, negated := x.X, false
			for zero.Kind == hir.Unary && (zero.Op == "-" || zero.Op == "+") {
				negated = negated != (zero.Op == "-")
				zero = zero.X
			}
			if value, ok := zero.Value.(float64); zero.Kind == hir.Lit && ok && value == 0 {
				sign := "1"
				if math.Signbit(value) != negated {
					sign = "2"
				}
				b.line(n + "->chai_zero_sign = " + sign + ".")
			}
		}
		return
	case "chai.deepEqual", "chai.deepInclude", "chai.arrayInclude", "chai.members", "chai.throwMatch":
		params := []string{"p0 = " + a}
		for i, arg := range args {
			params = append(params, fmt.Sprintf("p%d = %s", i+1, arg))
		}
		if x.Op == "chai.throwMatch" && x.X.Type.Kind != hir.Dynamic {
			b.chaiThrowTyped(n, a, x.X.Type, args[0])
			return
		}
		b.line(n + " = " + b.e.name("runtime.chai") + "=>" + strings.TrimPrefix(x.Op, "chai.") + "( " + strings.Join(params, " ") + " ).")
		return
	case "dynamic.of":
		if x.X.Kind == hir.StaticGet && strings.HasSuffix(x.X.Name, "_namespace") && x.X.Type.Kind == hir.OrderedMap && x.X.Type.Args[0].Kind == hir.String {
			// An enum namespace is immutable: boxing it as a plain object (a
			// snapshot of its entries) reads like the JavaScript object does.
			dyn := b.e.name("runtime.dynamic")
			b.serial++
			row := fmt.Sprintf("t%d", b.serial)
			v := b.temp(hir.T(hir.Dynamic))
			b.line("CREATE OBJECT " + n + ".")
			b.line(n + "->tag = " + dyn + "=>tag_object.")
			b.line("LOOP AT " + a + "->entries INTO DATA(" + row + ").")
			b.boxDynamic(v, row+"-v", x.X.Type.Args[1])
			b.line("APPEND VALUE #( k = " + row + "-k v = " + v + " ) TO " + n + "->entries.")
			b.line("ENDLOOP.")
			return
		}
		b.boxDynamic(n, a, x.X.Type)
		return
	case "dynamic.typeof", "dynamic.typeofValue", "dynamic.toString", "dynamic.recordString":
		if x.Op == "dynamic.recordString" {
			b.line("IF " + a + " IS BOUND.")
			b.line("IF " + a + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_array OR " + a + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_ref.")
			b.stmt(&hir.Stmt{Kind: hir.Trap, Name: x.Source + ": ABAP record string conversion requires primitive dynamic storage"})
			b.line("ENDIF.")
			b.line("ENDIF.")
		}
		b.line("IF " + a + " IS BOUND.")
		method := "type_of"
		if x.Op == "dynamic.toString" || x.Op == "dynamic.recordString" {
			method = "to_string"
		}
		if x.Op == "dynamic.recordString" {
			// Ordinary tagged records have Object.prototype's default coercion.
			// Presence, including an own undefined hook, leaves that domain.
			b.line("IF " + a + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_object.")
			for _, key := range []string{"toString", "valueOf", "__proto__"} {
				b.line("READ TABLE " + a + "->entries WITH KEY k = `" + key + "` TRANSPORTING NO FIELDS.\nIF sy-subrc = 0.")
				b.stmt(&hir.Stmt{Kind: hir.Trap, Name: x.Source + ": record string conversion requires the original default Object coercion hooks"})
				b.line("ENDIF.")
			}
			b.line(n + " = `[object Object]`.\nELSE.")
		}
		b.line(n + " = " + a + "->" + method + "( ).")
		if x.Op == "dynamic.recordString" {
			b.line("ENDIF.")
		}
		if x.Op == "dynamic.typeofValue" {
			// Keep the ordinary box representation and the Go callable predicate,
			// but test one shared marker instead of enumerating every closure here.
			marker := b.e.name("runtime.callable")
			if _, ok := b.e.files[marker+".intf.abap"]; ok {
				b.line("IF " + a + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_ref AND " + a + "->oval IS INSTANCE OF " + marker + ".")
				b.line(n + " = `function`.")
				b.line("ENDIF.")
			}
		}
		b.line("ELSE.")
		b.line(n + " = `undefined`.")
		b.line("ENDIF.")
		return
	case "dynamic.isArray":
		// An unbound box is undefined, which is not an array.
		b.line(n + " = abap_false.")
		b.line("IF " + a + " IS BOUND.")
		b.e.isArrayUsed = true
		b.line("IF " + a + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_array.")
		b.line(n + " = abap_true.")
		b.line("ELSEIF " + a + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_ref AND " + a + "->oval IS BOUND.")
		mark := b.rawTemp("REF TO " + b.e.name("runtime.arraymark"))
		b.line("TRY.")
		b.line(mark + " ?= " + a + "->oval.")
		b.line(n + " = abap_true.")
		b.line("CATCH cx_sy_move_cast_error.")
		b.line("ENDTRY.")
		b.line("ENDIF.")
		b.line("ENDIF.")
		return
	case "dynamic.isNumber", "dynamic.asNumber", "dynamic.isString", "dynamic.isFunction", "dynamic.asString", "dynamic.asClassValue", "dynamic.asRef":
		if x.Op == "dynamic.asRef" && x.Type.Kind == hir.Array {
			// A tagged array's as_ref returns the graph itself, not a typed
			// table wrapper. Publish a live view instead of downcasting it.
			b.arrayView(x.Type, n, a)
			return
		}
		// The box methods use snake_case ABAP names.
		op := map[string]string{"dynamic.isNumber": "is_number", "dynamic.asNumber": "as_number", "dynamic.isString": "is_string", "dynamic.isFunction": "is_function", "dynamic.asString": "as_string", "dynamic.asClassValue": "as_classvalue", "dynamic.asRef": "as_ref"}[x.Op]
		target := n
		if x.Op == "dynamic.asRef" {
			target = b.temp(hir.Ref(hir.RootObject))
		}
		record := x.Op == "dynamic.asRef" && x.Type.Kind == hir.OrderedMap && x.Type.Args[0].Kind == hir.String
		if record {
			// A record read from a tagged object graph (an any-typed {} bag):
			// copy its entries into the record map; a boxed map stays itself.
			b.line("IF " + a + " IS BOUND AND " + a + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_object.")
			b.line("CREATE OBJECT " + n + ".")
			b.serial++
			row := fmt.Sprintf("t%d", b.serial)
			b.line("LOOP AT " + a + "->entries INTO DATA(" + row + ").")
			v := row + "-v"
			if x.Type.Args[1].Kind != hir.Dynamic {
				box := b.temp(hir.T(hir.Dynamic))
				b.line(box + " = " + row + "-v.")
				v = b.unbox(box, x.Type.Args[1])
			}
			b.line(n + "->set( p0 = " + row + "-k p1 = " + v + " ).")
			b.line("ENDLOOP.")
			b.line("ELSE.")
		}
		b.line(target + " = " + a + "->" + op + "( ).")
		if record && x.Type.Args[1].Kind != hir.Dynamic {
			// A boxed map<string, any> bag stands for the narrower record too.
			bagType := hir.T(hir.OrderedMap, hir.T(hir.String), hir.T(hir.Dynamic))
			bag := b.temp(bagType)
			b.line("TRY.")
			b.line(n + " ?= " + target + ".")
			b.line("CATCH cx_sy_move_cast_error.")
			b.line(bag + " ?= " + target + ".")
			b.line("CREATE OBJECT " + n + ".")
			b.serial++
			row := fmt.Sprintf("t%d", b.serial)
			b.line("LOOP AT " + bag + "->entries INTO DATA(" + row + ").")
			box := b.temp(hir.T(hir.Dynamic))
			b.line(box + " = " + row + "-v.")
			b.line(n + "->set( p0 = " + row + "-k p1 = " + b.unbox(box, x.Type.Args[1]) + " ).")
			b.line("ENDLOOP.")
			b.line("ENDTRY.")
		} else if target != n {
			b.line(n + " ?= " + target + ".")
		}
		if record {
			b.line("ENDIF.")
		}
		return
	case "regexp.source":
		b.line(n + " = " + a + "->source.")
		return
	case "regexp.toString":
		// JavaScript prints the flags in canonical order: g before i.
		if b.e.jsRegexpUsed {
			b.line(n + " = |/{ " + a + "->source }/{ " + a + "->flags }|.")
		} else {
			b.line(n + " = |/{ " + a + "->source }/|.")
			b.line("IF " + a + "->global = abap_true.")
			b.line(n + " = " + n + " && `g`.")
			b.line("ENDIF.")
			b.line("IF " + a + "->ignore_case = abap_true.")
			b.line(n + " = " + n + " && `i`.")
			b.line("ENDIF.")
		}
		return
	}
	switch x.Op {
	case "array.length":
		if b.e.arrayViews {
			b.line(a + "->load( ).")
		}
		// The items table is public: lines( ) saves a method call per access.
		b.line(n + " = lines( " + a + "->items ).")
		return
	case "array.push":
		if b.e.arrayViews {
			b.line(a + "->load( ).")
		}
		b.line("APPEND " + args[0] + " TO " + a + "->items.")
		if n != "" {
			b.line(n + " = lines( " + a + "->items ).")
		}
		if b.e.arrayViews {
			b.line(a + "->save( ).")
		}
		return
	}
	if x.Op == "map.delete" {
		b.line("DELETE " + a + "->entries WHERE k = " + args[0] + ".")
		b.line(n + " = xsdbool( sy-subrc = 0 ).")
		return
	}
	if x.Op == "record.delete" {
		b.line("DELETE " + a + "->entries WHERE k = " + args[0] + ".")
		b.line(n + " = abap_true.")
		return
	}
	op := strings.Split(x.Op, ".")[1]
	params := []string{}
	for i, arg := range args {
		params = append(params, fmt.Sprintf("p%d = %s", i, arg))
	}
	target := n
	if (x.Op == "array.get" || x.Op == "array.pop" || x.Op == "array.shift") && (x.X.Type.Args[0].IsRef() || (x.X.Type.Args[0].Kind == hir.Optional && x.X.Type.Args[0].Args[0].IsRef())) {
		target = b.temp(hir.Ref(hir.RootObject))
	}
	s := a + "->" + op + "( " + strings.Join(params, " ") + " )"
	if target != "" {
		s = target + " = " + s
	}
	b.line(s + ".")
	if target != n {
		b.arrayElement(n, target, x.Type)
	}
}

func (b *body) stmt(s *hir.Stmt) {
	if s == nil {
		return
	}
	if castTrace && s.Node.Source != "" {
		b.code.WriteString("*@src " + s.Node.Source + "\n")
		b.lastInit = ""
	}
	e := b.e
	if e.refusals != nil {
		old := e.currentNode
		e.currentNode = &s.Node
		defer func() { e.currentNode = old }()
	}
	switch s.Kind {
	case hir.Block:
		old := b.locals
		b.locals = map[string]string{}
		for k, v := range old {
			b.locals[k] = v
		}
		for _, x := range s.List {
			b.stmt(x)
		}
		b.locals = old
	case hir.VarDecl:
		n := b.temp(s.Type)
		if s.X != nil {
			b.line(n + " = " + b.value(s.X, s.Type) + ".")
		}
		b.locals[s.Name] = n
	case hir.Assign:
		target := ""
		switch s.X.Kind {
		case hir.Local:
			target = b.locals[s.X.Name]
		case hir.FieldGet:
			owner := e.fieldOwner(e.classBy(s.X.X.Type.Name), s.X.Name)
			if owner != nil && e.liveClasses[owner.Name] {
				a := b.receiver(s.X.X)
				v := b.value(s.Y, s.X.Type)
				b.line(a + "->" + e.name("view.write."+owner.Name+"."+s.X.Name) + "( p0 = " + v + " ).")
				return
			}
			a := b.receiver(s.X.X)
			v := b.value(s.Y, s.X.Type)
			b.line(a + "->" + e.member(s.X.Name) + " = " + v + ".")
			if e.recordDelete {
				b.line("DELETE " + a + "->" + e.name("record.deleted") + " WHERE table_line = `" + strings.ReplaceAll(s.X.Name, "`", "``") + "`.")
			}
			if c := e.classBy(s.X.X.Type.Name); c != nil && c.Structural && e.types["runtime.liveView"] {
				b.markOwnKey(a, s.X.Name)
			}
			if owner != nil && e.jsonOwnKeys && owner.Structural && owner.Super == "" {
				b.jsonRecordKey(a, b.expr(hir.L(hir.T(hir.String), s.X.Name)))
			}
			return
		case hir.StaticGet:
			b.initialize(s.X.Owner)
			target = e.name(s.X.Owner) + "=>" + e.member(s.X.Name)
		case hir.IndexGet:
			a := b.expr(s.X.X)
			i := b.expr(s.X.Y)
			v := b.value(s.Y, s.X.Type)
			b.line(a + "->put( p0 = " + i + " p1 = " + v + " ).")
			return
		}
		b.line(target + " = " + b.value(s.Y, s.X.Type) + ".")
	case hir.ExprStmt:
		if s.X.Kind == hir.RuntimeOp && s.X.Op == "array.push" {
			// The new length is unused.
			b.runtimeOp(s.X, "")
			return
		}
		b.expr(s.X)
	case hir.If:
		a := b.expr(s.X)
		b.line("IF " + a + " = abap_true.")
		b.stmt(s.Body)
		if s.Else != nil {
			b.line("ELSE.")
			b.stmt(s.Else)
		}
		b.line("ENDIF.")
	case hir.While:
		b.line("DO.")
		a := b.expr(s.X)
		b.line("IF " + a + " = abap_false.")
		b.line("EXIT.")
		b.line("ENDIF.")
		b.stmt(s.Body)
		b.line("ENDDO.")
	case hir.ForEach:
		a := b.expr(s.X)
		if e.chaiUsed && e.arrayViews {
			b.line(a + "->load( ).")
		}
		b.chaiDenseIteration(a)
		n := b.temp(s.Type)
		old, ok := b.locals[s.Name]
		b.locals[s.Name] = n
		row := n
		if s.Type.IsRef() {
			row = b.temp(hir.Ref(hir.RootObject))
		}
		if !s.Type.IsRef() && s.Type.Kind != hir.Optional {
			b.line(a + "->dense( ).")
		}
		if e.arrayViews {
			idx := b.temp(hir.T(hir.I32))
			b.line("DO.\n" + a + "->load( ).\nIF " + idx + " >= lines( " + a + "->items ).\nEXIT.\nENDIF.")
			b.chaiDenseIteration(a)
			b.line("CLEAR " + row + ".\nREAD TABLE " + a + "->items INDEX " + idx + " + 1 INTO " + row + ".")
			if !s.Type.IsRef() && s.Type.Kind != hir.Optional {
				b.line("IF " + a + "->has( " + idx + " ) = abap_false.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.")
			}
			b.line(idx + " = " + idx + " + 1.")
			if row != n {
				b.arrayElement(n, row, s.Type)
			}
			b.stmt(s.Body)
			b.line("ENDDO.")
		} else {
			b.line("LOOP AT " + a + "->items INTO " + row + ".")
			b.chaiDenseIteration(a)
			if row != n {
				b.arrayElement(n, row, s.Type)
			}
			b.stmt(s.Body)
			b.line("ENDLOOP.")
		}
		if ok {
			b.locals[s.Name] = old
		} else {
			delete(b.locals, s.Name)
		}
	case hir.Break:
		b.line("EXIT.")
	case hir.Continue:
		b.line("CONTINUE.")
	case hir.Return:
		if s.X != nil && (b.m.Result.Kind == hir.Void || s.X.Type.Kind == hir.Void) {
			b.expr(s.X)
		} else if s.X != nil {
			b.line("result = " + b.value(s.X, b.m.Result) + ".")
		}
		b.line("RETURN.")
	case hir.Trap:
		name := e.name("exception.unexecuted")
		e.files[name+".clas.abap"] = "CLASS " + name + " DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.\nPUBLIC SECTION.\nDATA source_location TYPE string.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\nENDCLASS.\n"
		n := b.rawTemp("REF TO " + name)
		b.line(n + " = NEW #( ).")
		b.line(n + "->source_location = " + b.expr(hir.L(hir.T(hir.String), s.Name)) + ".")
		b.line("RAISE EXCEPTION " + n + ".")
	case hir.Async:
		n := b.rawTemp("REF TO " + e.promiseExceptionBase())
		b.line("TRY.")
		b.stmt(s.Body)
		b.line("CATCH " + e.promiseExceptionBase() + " INTO " + n + ".")
		b.line("result = NEW #( ).")
		b.line("result->rejection = " + n + ".")
		b.line("RETURN.")
		b.line("ENDTRY.")
	case hir.Throw:
		name := e.exception(s.X.Type)
		if e.assertionOwners[s.SiteOwner] {
			base := name
			name = e.name("exception.testAssertion." + s.X.Type.String())
			e.files[name+".clas.abap"] = "CLASS " + name + " DEFINITION PUBLIC INHERITING FROM " + base + " CREATE PUBLIC.\nPUBLIC SECTION.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\nENDCLASS.\n"
		}
		n := b.rawTemp("REF TO " + name)
		v := b.expr(s.X)
		b.line(n + " = NEW #( ).")
		b.line(n + "->payload = " + v + ".")
		if e.promiseUsed {
			b.promisePayload(n+"->source_value", v, s.X.Type)
		}
		b.line("RAISE EXCEPTION " + n + ".")
	case hir.Finally:
		// Normal completion runs the finally block after the TRY; anything
		// leaving the body is caught, the block runs, and the same exception
		// object is raised again.
		n := b.rawTemp("REF TO cx_root")
		b.line("TRY.")
		b.stmt(s.Body)
		b.line("CATCH cx_root INTO " + n + ".")
		b.stmt(s.Else)
		b.line("RAISE EXCEPTION " + n + ".")
		b.line("ENDTRY.")
		b.stmt(s.Else)
	case hir.Try:
		name := e.exception(s.Type)
		value := "payload"
		if e.promiseUsed && s.Type.Kind == hir.Dynamic {
			name = e.promiseExceptionBase()
			value = "source_value"
		}
		n := b.rawTemp("REF TO " + name)
		b.line("TRY.")
		b.stmt(s.Body)
		b.line("CATCH " + name + " INTO " + n + ".")
		v := b.temp(s.Type)
		payload := n + "->" + value
		if value == "payload" {
			payload = b.convert(payload, e.exceptionPayloadType(s.Type), s.Type)
		}
		b.line(v + " = " + payload + ".")
		old, ok := b.locals[s.Name]
		b.locals[s.Name] = v
		b.stmt(s.Else)
		if ok {
			b.locals[s.Name] = old
		} else {
			delete(b.locals, s.Name)
		}
		b.line("ENDTRY.")
	}
}

// wrap only splits outside literal tokens. Long strings are already chunked.
func wrap(src string) (string, error) {
	var out strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(src, "\n"), "\n") {
		prefix := line[:len(line)-len(strings.TrimLeft(line, " "))]
		for len(line) > 255 {
			quote := byte(0)
			cut := -1
			for i := 0; i < len(line) && i <= 240; i++ {
				c := line[i]
				if quote != 0 {
					if quote == '|' && c == '\\' {
						i++
					} else if c == quote {
						if quote != '|' && i+1 < len(line) && line[i+1] == quote {
							i++
						} else {
							quote = 0
						}
					}
				} else if c == '`' || c == '\'' || c == '|' {
					quote = c
				} else if c == ' ' {
					cut = i
				}
			}
			if cut <= len(prefix) {
				return "", fmt.Errorf("unbreakable source line (%s bytes)", strconv.Itoa(len(line)))
			}
			out.WriteString(line[:cut] + "\n")
			line = prefix + strings.TrimLeft(line[cut:], " ")
		}
		out.WriteString(line + "\n")
	}
	return out.String(), nil
}

// boxDynamic preserves identity and absence instead of boxing the ABI wrapper
// itself. Objects already represented as graph nodes flow through unchanged.
func (b *body) boxDynamic(n, a string, t hir.Type) {
	if t.Kind == hir.Dynamic {
		b.line(n + " = " + a + ".")
		return
	}
	b.line("CLEAR " + n + ".")
	if t.Kind == hir.Optional {
		test := a + " IS BOUND"
		base := t.Args[0]
		leaf := base
		for leaf.Kind == hir.Optional {
			leaf = leaf.Args[0]
		}
		if leaf.Kind == hir.I64 {
			// The newly admitted integer box may revisit an absent value in a loop.
			b.line("CLEAR " + n + ".")
		}
		if !base.IsRef() {
			test += " AND " + a + "->has = abap_true"
		}
		b.line("IF " + test + ".")
		optional := a
		if !base.IsRef() {
			a += "->value"
		}
		b.boxDynamic(n, a, base)
		if base.Kind == hir.Number {
			b.line(n + "->number_special = " + optional + "->number_special.")
		}
		b.line("ENDIF.")
		return
	}
	if t.IsRef() {
		b.line("IF " + a + " IS BOUND.")
	}
	tag := "tag_ref"
	field := "oval"
	switch t.Kind {
	case hir.String:
		tag, field = "tag_string", "sval"
	case hir.I64:
		// Preserve the proven integer domain before the binary64 graph ABI.
		limit := b.constant("box_safe_integer", "int8", "9007199254740991")
		b.line("IF " + a + " > " + limit + " OR " + a + " < 0 - " + limit + ".")
		b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
		b.line("ENDIF.")
		tag, field = "tag_number", "nval"
	case hir.Number, hir.I32:
		tag, field = "tag_number", "nval"
	case hir.Bool:
		tag, field = "tag_boolean", "bval"
	case hir.ClassValue:
		tag, field = "tag_class", "cval"
	default:
		if !t.IsRef() {
			b.e.refuse(fmt.Errorf("dynamic boxing of %s is not supported", t))
			return
		}
	}
	if field == "oval" && b.e.types["runtime.liveView"] {
		b.liveBox(n, a, t)
		if t.IsRef() {
			b.line("ENDIF.")
		}
		return
	}
	b.line("CREATE OBJECT " + n + ".")
	b.line(n + "->tag = " + b.e.name("runtime.dynamic") + "=>" + tag + ".")
	if field == "oval" {
		converted := b.rawTemp("REF TO object")
		b.line(converted + " = " + a + ".")
		a = converted
	}
	b.line(n + "->" + field + " = " + a + ".")
	if t.IsRef() {
		b.line("ENDIF.")
	}
}

// overriddenBelow reports whether class or any of its subclasses declares
// method name itself.
func (e *emitter) overriddenBelow(class, name string) bool {
	for _, c := range e.p.Classes {
		for k := c; k != nil; k = e.classBy(k.Super) {
			if k.Name == class {
				for _, m := range c.Methods {
					if m.Name == name || strings.HasPrefix(m.Name, name+"_instantiated_") {
						return true
					}
				}
				break
			}
			if k.Super == "" {
				break
			}
		}
	}
	return false
}

// wideShape: a class whose constructor would exceed the kernel's statement
// length (hundreds of parameters, e.g. the all-rules config shape) and whose
// constructor body only copies each parameter into the same-named field.
// Such a class is emitted without a constructor; NEW sets the fields.
func (e *emitter) wideShape(c *hir.Class) bool {
	if c.Ctor == nil || len(c.Ctor.Params) <= 40 || c.Super != "" || c.Ctor.Body == nil {
		return false
	}
	if len(c.Ctor.Body.List) != len(c.Ctor.Params) {
		return false
	}
	for i, s := range c.Ctor.Body.List {
		p := c.Ctor.Params[i]
		y := s.Y
		if y != nil && y.Kind == hir.Narrow {
			y = y.X
		}
		if s.Kind != hir.Assign || s.X == nil || y == nil || s.X.Kind != hir.FieldGet || s.X.Name != p.Name || y.Kind != hir.Local || y.Name != p.Name {
			return false
		}
	}
	return true
}

func (e *emitter) isInterface(name string) bool {
	for _, i := range e.p.Interfaces {
		if i.Name == name {
			return true
		}
	}
	return false
}

// needsImplicitSuper: a subclass constructor without its own super call,
// below an ancestor constructor (explicit or implicit) that takes no required
// parameters.
func (e *emitter) needsImplicitSuper(c *hir.Class, m *hir.Method) bool {
	if c.Super == "" {
		return false
	}
	// ABAP demands the call whenever there is a superclass, even one whose
	// constructor is implicit (no HIR constructor up the chain).
	parent := e.p.Constructor(c.Super)
	if parent == nil {
		parent = &hir.Method{}
	}
	for _, p := range parent.Params {
		if p.Type.Kind != hir.Optional {
			return false
		}
	}
	called := false
	if m.Body != nil {
		e.walkStmt(m.Body, func(x *hir.Expr) {
			if x != nil && x.Kind == hir.SuperCall && x.Name == "constructor" {
				called = true
			}
		})
	}
	return !called
}

// Numeric bounds are constants declared once per method, never rebuilt on a
// loop back edge (in particular the packed spelling of the i32 lower bound).
func (b *body) constant(name, typ, value string) string {
	if b.constants == nil {
		b.constants = map[string]string{}
	}
	name = "range_" + name
	b.constants[name] = "CONSTANTS " + name + " TYPE " + typ + " VALUE '" + value + "'.\n"
	return name
}
func (b *body) constantDeclarations() string {
	keys := make([]string, 0, len(b.constants))
	for k := range b.constants {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out strings.Builder
	for _, k := range keys {
		out.WriteString(b.constants[k])
	}
	return out.String()
}

// The opt-in integer contract must reject loss of JS integer precision long
// before native int8 overflows, including in bigint-backed runtimes.
func (b *body) integerOverflow(n string) {
	low, high := b.int8Constant("range_int8_bound_min", -9007199254740991), b.int8Constant("range_int8_bound_max", 9007199254740991)
	b.line("IF " + n + " < " + low + " OR " + n + " > " + high + ".")
	b.line("RAISE EXCEPTION TYPE cx_sy_arithmetic_overflow.")
	b.line("ENDIF.")
}

func (b *body) integerConstant(name string, value int64) string {
	prefix := "range_int_"
	if len(prefix)+len(name) > 30 {
		prefix = "range_i_"
	}
	return b.int8Constant(prefix+name, value)
}

// int8Constant declares an int8 CONSTANTS once per method. The value is a
// text literal, converted when the method is compiled, not rebuilt from i
// pieces on every call (6 statements per call in hot methods before).
func (b *body) int8Constant(name string, value int64) string {
	if b.constants == nil {
		b.constants = map[string]string{}
	}
	b.constants[name] = fmt.Sprintf("CONSTANTS %s TYPE int8 VALUE '%d'.\n", name, value)
	return name
}
