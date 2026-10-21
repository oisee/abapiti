// Package golang emits a deliberately small, standalone Go HIR prototype.
package golang

import (
	"encoding/json"
	"fmt"
	"go/format"
	"go/token"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/jsregex"
)

// Emit emits one package named main. Add a main function to execute an entry point.
func Emit(p *hir.Program) (map[string]string, error) { return EmitPackage(p, "main") }

// EmitPackage selects the package name. Output contains only standard-library dependencies.
func EmitPackage(p *hir.Program, pkg string) (map[string]string, error) {
	files, _, err := emitPackage(p, pkg, false)
	if err == nil {
		internLiteralFiles(files, pkg)
	}
	return files, err
}

func emitPackage(p *hir.Program, pkg string, profile bool) (map[string]string, *emitter, error) {
	if !token.IsIdentifier(pkg) || token.Lookup(pkg).IsKeyword() || pkg == "_" {
		return nil, nil, fmt.Errorf("invalid Go package name %q", pkg)
	}
	if errs := hir.Verify(p); len(errs) > 0 {
		return nil, nil, errs[0]
	}
	hir.AssignSiteIDs(p)
	e := &emitter{p: p, promiseUsed: hir.UsesSettledPromises(p), names: hir.NewNames(), profile: profile, profileIndices: map[string]int{}}
	e.characterSets()
	e.collectBoxedShapes()
	e.liveClasses = hir.LiveClassViewLayouts(p)
	e.enableLiveMapValueViews()
	for _, iface := range p.Interfaces {
		for _, member := range iface.StructuralMembers {
			if e.liveClasses[member.Type.Name] {
				e.extraRuntimeKeys = true
			}
		}
	}
	_, e.referenceSets = hir.UsedRuntimeOps(p)["set.view"]
	for _, c := range p.Classes {
		if !e.liveClasses[c.Name] {
			continue
		}
		for _, f := range c.Fields {
			if f.Type.Readonly || f.Type.Kind == hir.Optional && f.Type.Args[0].Readonly {
				e.readonlyViews = true
			}
		}
	}
	for op := range hir.UsedRuntimeOps(p) {
		if op == "container.readonlyView" || op == "collection.has" || op == "collection.get" {
			e.readonlyViews = true
		}
	}
	if e.readonlyViews {
		e.mapViews = true
		e.mapViewsKnown = true
		e.collectionProbe = true
		e.usesLiveArrays = true
	}

	if len(p.Unions) > 0 {
		e.extraRuntimeKeys = true
	}
	for _, c := range p.Classes {
		check := func(s *hir.Stmt) {
			walkStmt(s, func(*hir.Stmt) {}, func(x, _ *hir.Expr) {
				if x.Type.Kind == hir.OrderedMap && len(x.Type.Args) == 2 && x.Type.Args[0].Kind == hir.Dynamic {
					e.boxedMaps = true
				}
				if x.Kind == hir.RuntimeOp && x.Op == "arrayLike.length" {
					e.nonfiniteNumbers = true
					e.toNumber = true
				}
				if x.Kind == hir.RuntimeOp && (x.Op == "number.isInteger" || x.Op == "number.isSafeInteger" || x.Op == "number.isFinite" || x.Op == "number.strictIsNaN" || x.Op == "number.abs") {
					e.nonfiniteNumbers = true
				}
				if x.Kind == hir.RuntimeOp && x.Op == "dynamic.values" {
					e.extraRuntimeKeys = true
					e.usesLiveArrays = true
				}
				if x.Kind == hir.RuntimeOp && (x.Op == "string.parseFloatNumber" || x.Op == "string.parseIntNumber" || x.Op == "number.isNaN") {
					e.nonfiniteNumbers = true
				}
				if x.Kind == hir.RuntimeOp && (x.Op == "string.toNumber" || x.Op == "dynamic.toNumber") {
					e.nonfiniteNumbers = true
					e.toNumber = true
				}
				if x.Kind == hir.RuntimeOp && x.Op == "util.isDeepStrictEqual" {
					// Record comparison enumerates own keys of plain records.
					e.extraRuntimeKeys = true
					e.deepStrict = true
				}
				if x.Kind == hir.RuntimeOp && x.Op == "dynamic.delete" {
					e.recordDeletion = true
					e.extraRuntimeKeys = true
				}
				if x.Kind == hir.RuntimeOp && (x.Op == "dynamic.mapView" || x.Op == "dynamic.asObject" || x.Op == "dynamic.keys" || x.Op == "dynamic.arrayView" || x.Op == "dynamic.asRef" && x.Type.Kind == hir.Array && (x.Type.Args[0].Kind == hir.Dynamic || p.StructuralArrayUnion(x.Type))) {
					e.usesLiveArrays = true
				}
				if x.Kind == hir.RuntimeOp && (x.Op == "dynamic.isMap" || x.Op == "dynamic.isSet" || x.Op == "map.markJS" || x.Op == "dynamic.mapView") {
					e.collectionProbe = true // before record allocations are emitted
				}
				if x.Kind == hir.RuntimeOp && (x.Op == "dynamic.keys" ||
					(x.Op == "object.has" || x.Op == "object.hasOwn" || x.Op == "object.get") && (x.X.Type.Name == hir.RootObject || len(e.liveClasses) > 0)) {
					e.extraRuntimeKeys = true
				}
			})
		}
		if c.Ctor != nil {
			check(c.Ctor.Body)
		}
		for _, m := range c.Methods {
			check(m.Body)
		}
	}
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			e.checkOps(m.Body)
		}
		if c.Ctor != nil {
			e.checkOps(c.Ctor.Body)
		}
	}
	if e.err != nil {
		return nil, nil, e.err
	}
	if e.nativeJSON {
		e.extraRuntimeKeys = true
	}
	e.line("package %s", pkg)
	interfaces := append([]*hir.Interface(nil), p.Interfaces...)
	sort.Slice(interfaces, func(i, j int) bool { return interfaces[i].Name < interfaces[j].Name })
	for _, in := range interfaces {
		e.line("type %s interface {", e.ref(in.Name))
		for _, m := range in.Methods {
			e.line("%s(%s)%s", e.member(m.Name), e.params(m, false), e.result(m.Result))
		}
		e.line("}")
	}
	classes := append([]*hir.Class(nil), p.Classes...)
	sort.Slice(classes, func(i, j int) bool { return classes[i].Name < classes[j].Name })
	for _, c := range classes {
		e.class(c)
	}
	if e.err != nil {
		return nil, nil, e.err
	}
	e.line("var goSources = map[string]string{")
	for _, c := range classes {
		methods := append([]*hir.Method(nil), c.Methods...)
		if c.Ctor != nil {
			methods = append(methods, c.Ctor)
		}
		for _, m := range methods {
			source := m.Source
			if source == "" {
				source = c.Source
			}
			e.line("%q:%q,", e.body(c.Name, m.Name), relativeSource(source))
		}
	}
	e.line("}")
	patternJSON, _ := json.Marshal(reviewedPatterns)
	regexTable := "\nvar reviewedRegexPatterns = func() map[string][2]string { var m map[string][2]string; if err:=json.Unmarshal([]byte(" + strconv.Quote(string(patternJSON)) + "), &m);err!=nil {panic(err)};return m }()\n"
	validation := ""
	if e.xmlValidation {
		validation = xmlValidationRuntimeSource
	}
	jsonWriter := ""
	if e.configJSON {
		jsonWriter = configJSONRuntimeSource
	}
	if e.jsonReplacer {
		for _, patch := range configJSONReplacerPatches {
			if !strings.Contains(jsonWriter, patch[0]) {
				panic("JSON replacer runtime patch target moved: " + patch[0])
			}
			jsonWriter = strings.Replace(jsonWriter, patch[0], patch[1], 1)
		}
		jsonWriter += configJSONReplacerSource
	}
	runtime := e.stringA16Runtime(runtimeSource)
	runtime = sameValueZeroRuntime(runtime)
	runtime += e.gogenRuntimeSource()
	runtime = e.mapInsertionsRuntime(runtime)
	runtime = e.setIterationRuntime(runtime)
	if e.usesStorageIdentity() {
		runtime = storageIdentityRuntime(runtime)
	}
	if e.boxedMaps {
		runtime = boxedMapRuntime(runtime)
	}
	if e.usesMapViews() {
		runtime = mapViewRuntime(runtime)
		if _, ok := hir.UsedRuntimeOps(p)["dynamic.mapView"]; ok || e.liveMapValueViews || e.readonlyViews {
			runtime += dynamicMapStorageSource
			if e.readonlyViews {
				runtime = readonlySetRuntime(runtime)
				runtime += readonlyCollectionSource
			}
		}
	}
	if e.recordString {
		runtime += recordStringSource
	}
	if e.nonfiniteNumbers {
		runtime = nonfiniteRuntime(runtime)
	}
	if e.toNumber {
		runtime += toNumberSource
	}
	if e.regexSplit {
		e.extendedRegex = true
		runtime += regexSplitRuntimeSource
		if e.regexA16 {
			runtime = mustReplace(runtime, "func (s jsString) splitRegex(pattern *jsRegExp, limit float64) *array[jsString] {", `func (s jsString) splitRegex(pattern *jsRegExp, limit float64) *array[jsString] {
 if pattern.Flags.indexOf(str("u"))>=0 || pattern.labelGuard {panic(newTrap("regexp split is outside the checked split grammar"))}`, "regex split A16 guard")
		}
	}
	if e.extraRuntimeKeys {
		runtime += ownKeysSource
		if e.recordDeletion {
			runtime += recordDeletionSource
		}
	}
	if e.liveStorage() || e.usesMapViews() {
		runtime = mustReplace(runtime, "return a == b\n}", "return dynEqual(box(a), box(b))\n}", "live equal dispatch")
		runtime = strings.ReplaceAll(mustContain(runtime, "return equal(a.Value, b.Value)", "dynamic equal fast path"), "return equal(a.Value, b.Value)", "return a.Value == b.Value")
	}
	if e.extendedRegex {
		runtime += strings.TrimPrefix(jsregex.RuntimeSource, "package jsregex")
		if e.regexA16 {
			runtime += strings.TrimPrefix(jsregex.IteratorRuntimeSource, "package jsregex")
		}
		runtime = mustReplace(runtime, ` if !ok {panic(newTrap("unreviewed JavaScript regexp /" + p + "/" + f))}`, ` if !ok {if translated,accepted:=TranslateSubset(p,f);accepted {spec=[2]string{translated,""};ok=true}}
 if !ok {panic(newTrap("unreviewed JavaScript regexp /" + p + "/" + f))}`, "unreviewed regexp TranslateSubset fallback")
	}
	if e.regexA16 {
		// The call site is installed by the extendedRegex patch above; regexA16
		// alone (e.g. a reviewed pattern with the iterator grammar) omits it.
		runtime = optionalReplace(runtime, "if translated,accepted:=TranslateSubset(p,f);accepted", "if translated,accepted:=TranslateIteratorSubset(p,f);accepted", "TranslateIteratorSubset dispatch")
	}
	if e.regexExtended {
		runtime += mustReplace(strings.TrimPrefix(jsregex.ExtendedRuntimeSource, "package jsregex"), "import \"strings\"\n", "", "extended regexp strings import")
		runtime = mustReplace(runtime, "if translated,accepted:=TranslateIteratorSubset(p,f);accepted", "if translated,accepted:=TranslateExtendedSubset(p,f);accepted", "TranslateExtendedSubset dispatch")
	}
	if e.constructorArgs {
		runtime = mustReplace(runtime, "Factory func() any", "Factory func() any; ArgsFactory func(*array[any]) any", "constructor args factory")
	}
	if e.usesTypedCatch {
		runtime += "\nfunc (p payload[T]) payloadValue() any {return p.Value}\n"
	}
	if e.usesArrayPresence || e.usesArrayDeletion || e.liveStorage() {
		runtime = sparseRuntime(runtime)
	}
	if e.chaiDeep {
		runtime = mustReplace(runtime, "type orderedMap[K comparable, V any] struct {", "type orderedMap[K comparable, V any] struct { enumerableRecord bool;", "chai enumerable record flag")
		runtime += chaiDeepRuntimeSource
	}
	if e.spreadRuntime {
		runtime += dynSpreadSource
	}
	if e.spreadRuntime || e.collectionProbe {
		runtime += collectionTypeNameSource
	}
	if e.collectionProbe {
		runtime += dynIsCollectionSource
		runtime = mustReplace(runtime, "type orderedMap[K comparable, V any] struct {", "type orderedMap[K comparable, V any] struct { jsMap bool;", "collection probe jsMap flag")
	}
	if e.liveStorage() {
		runtime = liveArrayRuntime(runtime)
		jsonWriter = strings.ReplaceAll(jsonWriter, "range a.Items", "range a.values()")
	}
	if e.dynamicSearch {
		length := "int32(len(a.Items))"
		if e.liveStorage() {
			length = "a.length()"
		}
		runtime += strings.ReplaceAll(dynamicSearchSource, "LENGTH", length)
	}
	generated := e.code.String() + e.extra.String()
	if e.hirStrconv {
		generated = strings.Replace(generated, "package "+pkg+"\n", "package "+pkg+"\nimport \"strconv\"\n", 1)
	}
	if e.hirMath {
		// Checked integer field views may emit math.Trunc in this file;
		// runtime.go's imports are not package-wide Go imports.
		generated = strings.Replace(generated, "package "+pkg+"\n", "package "+pkg+"\nimport \"math\"\n", 1)
	}
	files := map[string]string{"hir.go": generated, "runtime.go": strings.Replace(runtime+spreadArraySource+unicodeUpperSource+jsonRuntimeSource+json5RuntimeSource+json5UnicodeSource+xmlRuntimeSource+validation+regexTable+jsonWriter, "package main", "package "+pkg, 1)}
	if e.promiseUsed {
		files["promise.go"] = strings.Replace(promiseRuntimeSource, "package main", "package "+pkg, 1)
	}
	if source := e.nodePlatformSource(pkg); source != "" {
		files["nodeplatform.go"] = source
	}
	if source := e.externSource(pkg); source != "" {
		files["extern.go"] = source
	}
	fastRuntime, err := fastCollectionRuntime(files["runtime.go"])
	if err != nil {
		return nil, nil, err
	}
	// Source sites refer to e.code, so generated method types are selected
	// by typ() and their adapters directly. Only rewrite the runtime here.
	runtimeFiles := map[string]string{"runtime.go": fastRuntime}
	if err := directCollectionTypes(runtimeFiles); err != nil {
		return nil, nil, err
	}
	files["runtime.go"] = runtimeFiles["runtime.go"]
	for n, s := range files {
		b, err := format.Source([]byte(s))
		if err != nil {
			return nil, nil, fmt.Errorf("format %s: %w", n, err)
		}
		files[n] = string(b)
	}
	return files, e, nil
}

// Match the ABAP source-map convention without modifying the shared HIR.
func relativeSource(source string) string {
	source = strings.ReplaceAll(source, "\\", "/")
	for _, root := range []string{"/node_modules/", "/harness/", "/src/"} {
		if i := strings.Index(source, root); i >= 0 {
			return source[i+1:]
		}
	}
	return source
}

type emitter struct {
	boxedMaps         bool
	referenceSets     bool
	readonlyViews     bool
	externAdapters    []string
	externImports     map[string]string
	hirStrconv        bool
	chaiDeep          bool
	chaiThrow         bool
	deepStrict        bool
	chaiActionUsed    bool
	xmlValidation     bool
	propertyLookups   map[string]bool
	typeofUsed        bool
	extendedRegex     bool
	regexA16          bool
	regexExtended     bool
	regexSplit        bool
	constructorArgs   bool
	configJSON        bool
	nativeJSON        bool
	promiseUsed       bool
	profile           bool
	sites             []emittedSite
	profileIndices    map[string]int
	profileNodes      []hir.Site
	p                 *hir.Program
	names             *hir.Names
	code              strings.Builder
	err               error
	extra             strings.Builder
	ownKeyTables      map[string]bool
	lspGuards         bool
	hirMath           bool
	extraRuntimeKeys  bool
	recordDeletion    bool
	dynamicSearch     bool
	usesLiveArrays    bool
	spreadRuntime     bool
	collectionProbe   bool
	jsonReplacer      bool
	liveClasses       map[string]bool
	liveViews         map[string]bool
	materializers     map[string]bool
	stringLiterals    map[string]string
	classifiers       map[string]string
	boxedFields       map[string]bool
	boxedShapes       map[string]bool
	usesArrayPresence bool
	usesArrayDeletion bool
	recordString      bool
	mapViews          bool
	dynamicMaps       bool
	liveMapValueViews bool
	liveMapValueTypes map[string]bool
	mapViewsKnown     bool
	objCarriers       bool
	objCarriersKnown  bool
	nonfiniteNumbers  bool
	toNumber          bool
	usesTypedCatch    bool
}

func (e *emitter) line(f string, a ...any) { fmt.Fprintf(&e.code, f+"\n", a...) }
func (e *emitter) unsupported(n hir.Node, s string) {
	if e.err == nil {
		e.err = hir.Error{Node: n, Message: "not supported in the Go prototype: " + s}
	}
}
func (e *emitter) name(s string) string    { return e.names.Get(s) }
func (e *emitter) ref(s string) string     { return e.name("ref." + s) }
func (e *emitter) obj(s string) string     { return e.name("struct." + s) }
func (e *emitter) member(s string) string  { return e.name("member." + s) }
func (e *emitter) getter(s string) string  { return e.name("base." + s) }
func (e *emitter) body(c, m string) string { return e.name("body." + c + "." + m) }
func (e *emitter) classBy(n string) *hir.Class {
	for _, c := range e.p.Classes {
		if c.Name == n {
			return c
		}
	}
	return nil
}
func (e *emitter) method(c *hir.Class, n string) (*hir.Method, *hir.Class) {
	for ; c != nil; c = e.classBy(c.Super) {
		for _, m := range c.Methods {
			if m.Name == n {
				return m, c
			}
		}
		if n == "constructor" && c.Ctor != nil {
			return c.Ctor, c
		}
	}
	return nil, nil
}
func (e *emitter) fieldOwner(c *hir.Class, n string) *hir.Class {
	for ; c != nil; c = e.classBy(c.Super) {
		for _, f := range c.Fields {
			if f.Name == n {
				return c
			}
		}
	}
	return nil
}
func arrayStorage(t hir.Type) hir.Type {
	if t.Kind == hir.Array && t.Args[0].IsRef() {
		return hir.T(hir.Array, hir.Ref(hir.RootObject))
	}
	return t
}
func (e *emitter) typ(t hir.Type) string {
	t = arrayStorage(t)
	switch t.Kind {
	case hir.Promise:
		e.promiseUsed = true
		base := "struct{}"
		if t.Args[0].Kind != hir.Void {
			base = e.typ(t.Args[0])
		}
		return "*settledPromise[" + base + "]"
	case hir.Void:
		return ""
	case hir.Bool:
		return "bool"
	case hir.I32:
		return "int32"
	case hir.I64:
		return "int64"
	case hir.Number:
		return "float64"
	case hir.BigInt:
		return "jsBigInt"
	case hir.String:
		return "jsString"
	case hir.ClassValue:
		return "*classDescriptor"
	case hir.RegExp:
		return "*jsRegExp"
	case hir.Dynamic, hir.UnionRef:
		return "*dynamic"
	case hir.ClassRef:
		if t.Name == hir.RootObject {
			return "any"
		}
		return e.ref(t.Name)
	case hir.InterfaceRef:
		return e.ref(t.Name)
	case hir.Optional:
		if t.Args[0].IsRef() {
			return e.typ(t.Args[0])
		}
		return "optional[" + e.typ(t.Args[0]) + "]"
	case hir.Array:
		return "*array[" + e.typ(t.Args[0]) + "]"
	case hir.OrderedMap:
		name := "orderedMap"
		if e.directCollectionKey(t.Args[0]) {
			name = "fastOrderedMap"
		}
		return "*" + name + "[" + e.typ(t.Args[0]) + "," + e.typ(t.Args[1]) + "]"
	case hir.OrderedSet:
		name := "orderedSet"
		elem := t.Args[0]
		if e.referenceSets && elem.IsRef() {
			// A program with a reference Set view shares erased keys across
			// all aliases, including writes through the original typed Set.
			elem = hir.Ref(hir.RootObject)
		} else if e.directCollectionKey(elem) {
			name = "fastOrderedSet"
		}
		return "*" + name + "[" + e.typ(elem) + "]"
	default:
		e.unsupported(hir.Node{}, "type "+t.String())
		return "any"
	}
}
func (e *emitter) result(t hir.Type) string {
	if t.Kind == hir.Void {
		return ""
	}
	return " " + e.typ(t)
}
func (e *emitter) params(m *hir.Method, named bool) string {
	a := []string{}
	for i, p := range m.Params {
		// The frontend packs rest arguments into a trailing HIR array.
		s := e.typ(p.Type)
		if named {
			s = fmt.Sprintf("p%d %s", i, s)
		}
		a = append(a, s)
	}
	return strings.Join(a, ",")
}
func (e *emitter) effective(c *hir.Class) []string {
	seen := map[string]bool{}
	for ; c != nil; c = e.classBy(c.Super) {
		for _, m := range c.Methods {
			if !m.Static {
				seen[m.Name] = true
			}
		}
	}
	a := []string{}
	for n := range seen {
		a = append(a, n)
	}
	sort.Strings(a)
	return a
}
func (e *emitter) class(c *hir.Class) {
	e.descriptor(c)
	if e.extraRuntimeKeys {
		e.ownKeyClass(c)
	}
	if e.configJSON {
		e.configJSONClass(c)
	}
	e.line("type %s struct {", e.obj(c.Name))
	if e.recordDeletion && c.Super == "" {
		e.line("deletedKeys map[jsString]bool")
		e.line("deletionOrder *fastOrderedMap[jsString,*dynamic]")
	}
	if c.Scoped {
		e.line("lexicalDescriptor *classDescriptor")
	}
	if e.tracksOwnKeys(c) {
		e.line("ownKeys *ownKeyState")
		if e.nativeJSON && !c.Structural && e.classBy(c.Super) == nil {
			e.line("ownKeyStore ownKeyState")
		}
	}
	if len(e.liveClasses) > 0 && c.Structural {
		e.line("viewExtras *fastOrderedMap[jsString,*dynamic]")
	}
	if e.liveClasses[c.Name] {
		e.line("viewSource *dynamic")
		e.line("viewCheck func(*dynamic)")
	}
	// Only materializable data shapes can acquire a dynamic source. Other
	// objects still escape normally, but need no permanently nil source slot.
	source := c.Super == "" && !c.Abstract && len(c.Methods) == 0 && c.Ctor != nil && len(c.Ctor.Params) == len(c.Fields)
	if source {
		e.line("source *dynamic")
	}
	storage := source
	for _, f := range c.Fields {
		storage = storage || !f.Static
	}
	if c.Super != "" {
		e.line("%s", e.obj(c.Super))
	} else if !storage {
		e.line("identity byte")
	}
	for _, f := range c.Fields {
		if !f.Static {
			e.line("%s %s", e.member(f.Name), e.typ(f.Type))
		}
	}
	e.line("}")
	if strings.HasPrefix(c.Name, "tuple.") {
		e.line("func (self *%s) arrayMarker() {}", e.obj(c.Name))
	}
	if e.chaiDeep {
		e.chaiDeepShape(c, source)
	}
	if e.chaiThrow {
		e.chaiThrowShape(c)
	}
	if len(e.liveClasses) > 0 && c.Structural {
		e.line("func (self *%s) structuralRecord() {}", e.obj(c.Name))
	}
	if e.deepStrict && c.Structural {
		e.line("func (self *%s) deepStrictPlain() {}", e.obj(c.Name))
	}
	e.line("func (self *%s) nilReference() bool {return self==nil}", e.obj(c.Name))
	if e.recordDeletion && c.Super == "" {
		e.line("func (self *%s) propertyWritten(key jsString) {delete(self.deletedKeys,key);if self.deletionOrder!=nil {self.deletionOrder.set(key,nil)}}", e.obj(c.Name))
	}
	if e.recordDeletion || e.boxedFields[c.Name] || source && e.boxedShapes[c.Name] || len(e.liveClasses) > 0 && c.Structural {
		e.shapeFields(c)
	}
	if e.liveClasses[c.Name] {
		e.line("func (self *%s) dynamicSource() *dynamic {if self==nil {return nil};return self.viewSource}", e.obj(c.Name))
	} else if source {
		if e.configJSON {
			// Boxing historically returns the parsed input record. After native
			// writes that record is stale; refuse this escape rather than let a
			// later dynamic JSON writer silently serialize the old values.
			e.line("func (self *%s) dynamicSource() *dynamic {if self==nil {return nil};if self.source!=nil && stringifyConfig(self)!=stringifyConfig(self.source) {panic(newTrap(%q))};return self.source}", e.obj(c.Name), "boxing a changed parsed native projection is unsupported")
		} else {
			e.line("func (self *%s) dynamicSource() *dynamic {return self.source}", e.obj(c.Name))
		}
	} else {
		e.line("func (self *%s) dynamicSource() *dynamic {return nil}", e.obj(c.Name))
	}
	e.line("func (self *%s) %s() *%s {return self}", e.obj(c.Name), e.getter(c.Name), e.obj(c.Name))
	if e.concreteClass(c.Name) {
		// A class with no subclasses has one possible concrete representation.
		e.line("type %s = *%s", e.ref(c.Name), e.obj(c.Name))
	} else {
		e.line("type %s interface {", e.ref(c.Name))
		e.line("nilReference() bool")
		if c.Super != "" {
			e.line("%s", e.ref(c.Super))
		}
		e.line("%s() *%s", e.getter(c.Name), e.obj(c.Name))
		for _, n := range e.effective(c) {
			m, _ := e.method(c, n)
			e.line("%s(%s)%s", e.member(n), e.params(m, false), e.result(m.Result))
		}
		e.line("}")
	}
	for _, f := range c.Fields {
		if f.Static {
			e.line("var %s %s", e.name("static."+c.Name+"."+f.Name), e.typ(f.Type))
		}
	}
	// Reentrant first-use initialization matches ABAP's guarded class initialization.
	constructor := false
	for _, m := range c.Methods {
		constructor = constructor || m.Name == "class_constructor"
	}
	if constructor {
		e.line("var %s bool", e.name("initialized."+c.Name))
		e.line("func %s() {if %s {return}; %s=true", e.name("init."+c.Name), e.name("initialized."+c.Name), e.name("initialized."+c.Name))
	} else {
		e.line("func %s() {", e.name("init."+c.Name))
	}
	for _, m := range c.Methods {
		if m.Name == "class_constructor" {
			e.line("%s()", e.body(c.Name, m.Name))
		}
	}
	e.line("}")
	ctor, owner := e.method(c, "constructor")
	params := ""
	args := []string{}
	if ctor != nil {
		params = e.params(ctor, true)
		for i := range ctor.Params {
			args = append(args, fmt.Sprintf("p%d", i))
		}
	}
	e.line("func %s(%s) %s {", e.name("new."+c.Name), params, e.ref(c.Name))
	e.line("%s()", e.name("init."+c.Name))
	e.line("self:=&%s{}", e.obj(c.Name))
	if e.nativeJSON && !c.Structural {
		e.newOwnKeys(c)
	}
	if ctor != nil {
		// Module functions named constructor retain their static calling convention.
		if !ctor.Static {
			args = append([]string{"self"}, args...)
		}
		e.line("%s(%s)", e.body(owner.Name, "constructor"), strings.Join(args, ","))
	}
	e.line("return self }")
	for _, n := range e.effective(c) {
		m, owner := e.method(c, n)
		a := []string{}
		for i := range m.Params {
			a = append(a, fmt.Sprintf("p%d", i))
		}
		e.line("func (self *%s) %s(%s)%s {", e.obj(c.Name), e.member(n), e.params(m, true), e.result(m.Result))
		if m.Abstract {
			e.line("panic(newTrap(%q))", "abstract "+c.Name+"."+n)
		} else {
			prefix := ""
			if m.Result.Kind != hir.Void {
				prefix = "return "
			}
			e.line("%s%s(self%s)", prefix, e.body(owner.Name, n), comma(a))
		}
		e.line("}")
	}
	if e.liveClasses[c.Name] {
		e.liveFields(c)
	}
	for _, m := range c.Methods {
		if !m.Abstract {
			e.emitBody(c, m)
		}
	}
	if c.Ctor != nil {
		e.emitBody(c, c.Ctor)
	}
}
func comma(a []string) string {
	if len(a) == 0 {
		return ""
	}
	return "," + strings.Join(a, ",")
}
func (e *emitter) emitBody(c *hir.Class, m *hir.Method) {
	params := e.params(m, true)
	if !m.Static {
		params = "self " + e.ref(c.Name) + func() string {
			if params != "" {
				return "," + params
			}
			return ""
		}()
	}
	result := ""
	if m.Result.Kind != hir.Void {
		result = "(result " + e.typ(m.Result) + ")"
	}
	e.line("func %s(%s) %s {", e.body(c.Name, m.Name), params, result)
	if hasReturnBoundary(m.Body) || e.profile {
		if m.Result.Kind == hir.Void {
			e.line("defer func(){if x:=recover();x!=nil {if _,ok:=x.(returnedVoid);!ok {panic(x)}}}()")
		} else {
			e.line("defer func(){if x:=recover();x!=nil {if r,ok:=x.(returned[%s]);ok {result=r.Value}else{panic(x)}}}()", e.typ(m.Result))
		}
	}
	if !m.Static {
		if e.concreteClass(c.Name) {
			e.line("if self==nil {panic(newRangeFault())}")
		} else {
			e.line("if self==nil || self.nilReference() {panic(newRangeFault())}")
		}
	}
	if !m.Static && m.Name != "constructor" && e.liveClasses[c.Name] {
		e.line("if self.%s().viewSource!=nil {panic(newTrap(\"prototype method dispatch through structural class view is unsupported\"))}", e.getter(c.Name))
	}
	if m.Name != "class_constructor" {
		e.line("%s()", e.name("init."+c.Name))
	}
	b := &body{e: e, c: c, m: m, locals: map[string]string{}}
	b.initialized = map[string]string{}
	// Local flags track actual execution through branches, loops and catches.
	// Declaring them has no initialization effects; the first access still runs
	// the reentrant global guard at its original evaluation point.
	walkStmt(m.Body, func(*hir.Stmt) {}, func(x, _ *hir.Expr) {
		if x.Kind == hir.StaticGet {
			owner := e.fieldOwner(e.classBy(x.Owner), x.Name).Name
			if _, ok := b.initialized[owner]; !ok {
				flag := b.fresh()
				b.initialized[owner] = flag
				e.line("%s := false; _ = %s", flag, flag)
			}
		}
	})
	for i, p := range m.Params {
		b.locals[p.Name] = fmt.Sprintf("p%d", i)
		e.line("_ = p%d", i)
	}
	b.stmt(m.Body)
	e.line("return")
	e.line("}")
}

type body struct {
	e           *emitter
	c           *hir.Class
	m           *hir.Method
	locals      map[string]string
	next        int
	tryDepth    int
	loops       []loopState
	capacity    map[*hir.Expr][]*hir.Expr
	initialized map[string]string
}

func (b *body) line(f string, a ...any) { b.e.line(f, a...) }
func (b *body) fresh() string           { b.next++; return fmt.Sprintf("v%d", b.next) }
func (b *body) temp(t hir.Type, code string) string {
	if t.Kind == hir.Void {
		b.line("%s", code)
		return ""
	}
	n := b.fresh()
	b.line("var %s %s = %s; _ = %s", n, b.e.typ(t), code, n)
	return n
}
func (b *body) value(x *hir.Expr, t hir.Type) string {
	v := b.expr(x)
	if t.Kind == hir.Optional && x.Type.Kind != hir.Optional && !t.Args[0].IsRef() {
		return "present(" + v + ")"
	}
	if t.Kind == hir.Array && !arrayStorage(t).Equal(arrayStorage(x.Type)) {
		b.e.unsupported(x.Node, "covariant array view")
	}
	return v
}
func (b *body) literal(t hir.Type, v any, node hir.Node) string {
	if t.Kind == hir.Optional {
		if v == nil {
			if t.Args[0].IsRef() {
				return "nil"
			}
			return b.e.typ(t) + "{}"
		}
		s := b.literal(t.Args[0], v, node)
		if t.Args[0].IsRef() {
			return s
		}
		return "present(" + s + ")"
	}
	switch t.Kind {
	case hir.BigInt:
		digits, ok := v.(string)
		if !ok {
			b.e.unsupported(node, "bigint literal has non-string value")
			return `jsBigInt("0")`
		}
		return "jsBigInt(" + strconv.Quote(digits) + ")"
	case hir.String:
		text, ok := v.(string)
		if !ok {
			b.e.unsupported(node, "string literal has non-string value")
			return `str("")`
		}
		return b.e.stringLiteral(text)
	case hir.Bool:
		return fmt.Sprint(v)
	case hir.Number:
		f := numberLiteral(v)
		if math.IsInf(f, 0) || math.IsNaN(f) {
			b.e.unsupported(node, "non-finite Number literal")
			return "0"
		}
		if f == 0 && math.Signbit(f) {
			return "negativeZero()"
		}
		return "float64(" + strconv.FormatFloat(f, 'g', -1, 64) + ")"
	default:
		return b.e.typ(t) + "(" + fmt.Sprint(v) + ")"
	}
}
func (b *body) truth(v string, t hir.Type) string {
	switch t.Kind {
	case hir.BigInt:
		return v + "!=jsBigInt(\"0\")"
	case hir.Optional:
		if t.Args[0].Kind == hir.Dynamic {
			// The tagged box decides: boxed false, null, "", 0 and a nil box
			// (absent/undefined) are falsy; pointer presence is not truthiness.
			return "dynTruth(" + v + ")"
		}
		if t.Args[0].IsRef() {
			return "!nilRef(" + v + ")"
		}
		return v + ".Has && (" + b.truth(v+".Value", t.Args[0]) + ")"
	case hir.Dynamic:
		return "dynTruth(" + v + ")"
	case hir.Bool:
		return v
	case hir.String:
		return v + ".length()!=0"
	case hir.Number:
		if b.e.nonfiniteNumbers {
			return "numberTruth(" + v + ")"
		}
		return v + "!=0"
	case hir.I32, hir.I64:
		return v + "!=0"
	default:
		return "!nilRef(" + v + ")"
	}
}
func (b *body) lvalue(x *hir.Expr) string {
	e := b.e
	switch x.Kind {
	case hir.Local:
		return b.locals[x.Name]
	case hir.FieldGet:
		v := b.expr(x.X)
		c := e.fieldOwner(e.classBy(x.X.Type.Name), x.Name)
		return v + "." + e.getter(c.Name) + "()." + e.member(x.Name)
	case hir.StaticGet:
		c := e.fieldOwner(e.classBy(x.Owner), x.Name)
		flag := b.initialized[c.Name]
		b.line("if !%s { %s(); %s=true }", flag, e.name("init."+c.Name), flag)
		return e.name("static." + c.Name + "." + x.Name)
	}
	e.unsupported(x.Node, "assignment "+string(x.Kind))
	return ""
}
func (b *body) expr(x *hir.Expr) string {
	e := b.e
	t := x.Type
	code := ""
	switch x.Kind {
	case hir.ExternCall:
		return b.externCall(x)
	case hir.PromiseFulfill:
		value := "struct{}{}"
		if x.X != nil && t.Args[0].Kind == hir.Void {
			b.expr(x.X)
		} else if x.X != nil {
			value = b.value(x.X, t.Args[0])
		}
		return b.temp(t, "&settledPromise["+promiseValueType(e, t)+"]{Value:"+value+"}")
	case hir.PromiseReject:
		value := b.expr(x.X)
		return b.temp(t, fmt.Sprintf("&settledPromise[%s]{Rejection:payload[%s]{Type:%q,Value:%s}}", promiseValueType(e, t), e.typ(x.X.Type), x.X.Type.String(), value))
	case hir.PromiseAwait:
		value := b.expr(x.X)
		if x.X.Type.Kind == hir.Optional {
			b.line("if %s != nil { drainSettled(%s) }", value, value)
			return ""
		}
		return b.temp(t, "drainSettled("+value+")")
	case hir.Lit:
		if v, ok := x.Value.(string); ok && !utf8.ValidString(v) {
			e.unsupported(x.Node, "invalid UTF-8 or lone surrogate literal")
		}
		code = b.literal(t, x.Value, x.Node)
	case hir.Local:
		code = b.locals[x.Name]
	case hir.This:
		code = "self"
	case hir.FieldGet:
		c := e.fieldOwner(e.classBy(x.X.Type.Name), x.Name)
		if c != nil && e.liveClasses[c.Name] {
			code = b.expr(x.X) + "." + e.getter(c.Name) + "()." + e.name("view.read."+c.Name+"."+x.Name) + "()"
		} else {
			code = b.lvalue(x)
		}
	case hir.StaticGet:
		code = b.lvalue(x)
	case hir.IndexGet:
		a, i := b.expr(x.X), b.expr(x.Y)
		code = a + ".get(" + i + ").Value"
		if !t.IsRef() && t.Kind != hir.Optional {
			code = "unwrap(" + a + ".get(" + i + "))"
		}
		if e.liveStorage() {
			code = "liveArrayRead(" + a + "," + i + ")"
		}
		if x.X.Type.Args[0].IsRef() {
			if e.liveStorage() && t.Kind == hir.Dynamic {
				code = "box(" + code + ")"
			} else {
				code = e.arrayReference(t, code)
			}
		}
	case hir.New:
		a := []string{}
		if t.Kind == hir.ClassRef {
			m, _ := e.method(e.classBy(t.Name), "constructor")

			if e.extraRuntimeKeys && e.classBy(t.Name).Structural && x.OwnKeys != nil {
				a = make([]string, len(x.Args))
				for _, key := range *x.OwnKeys {
					for i, param := range m.Params {
						if param.Name == key {
							a[i] = b.temp(param.Type, b.value(x.Args[i], param.Type))
							break
						}
					}
				}
				for i, v := range x.Args {
					if a[i] == "" {
						a[i] = b.value(v, m.Params[i].Type)
					}
				}
			} else {
				for i, v := range x.Args {
					a = append(a, b.value(v, m.Params[i].Type))
				}
			}
			code = e.name("new."+t.Name) + "(" + strings.Join(a, ",") + ")"
			if e.extraRuntimeKeys && e.classBy(t.Name).Structural && x.OwnKeys != nil {
				result := b.temp(t, code)
				e.literalOwnKeys(b, result, *x.OwnKeys)
				return result
			}
		} else {
			for _, v := range x.Args {
				a = append(a, b.expr(v))
			}
			code = "&" + strings.TrimPrefix(e.typ(t), "*") + "{}"
			if e.chaiDeep && t.Kind == hir.OrderedMap && x.OwnKeys != nil {
				code = "&" + strings.TrimPrefix(e.typ(t), "*") + "{enumerableRecord:true}"
			}
			if t.Kind == hir.RegExp {
				flags := `str("")`
				if len(a) > 1 {
					flags = a[1]
				}
				code = "newRegExp(" + a[0] + "," + flags + ")"
			}
			if t.Kind == hir.Array && len(a) > 0 {
				code = "sizedArray[" + e.typ(arrayStorage(t).Args[0]) + "](float64(" + a[0] + "))"
			}
		}
	case hir.DirectCall, hir.VirtualCall, hir.SuperCall:
		owner := x.Owner
		recv := ""
		if x.Kind == hir.SuperCall {
			owner = b.c.Super
			recv = "self"
		} else if x.X != nil {
			recv = b.expr(x.X)
			owner = x.X.Type.Name
		}
		m, c := e.method(e.classBy(owner), x.Name)
		if m == nil {
			for _, in := range e.p.Interfaces {
				if in.Name == owner {
					for _, f := range in.Methods {
						if f.Name == x.Name {
							m = f
						}
					}
				}
			}
		}
		a := []string{}
		for i, v := range x.Args {
			a = append(a, b.value(v, m.Params[i].Type))
		}
		if fields := b.capacity[x]; len(fields) != 0 {
			for i, p := range m.Params {
				if p.Type.Kind == hir.String {
					for _, f := range fields {
						array := b.expr(f)
						b.line("%s.reserve(min(65536,int(%s.length())/5))", array, a[i])
					}
					break
				}
			}
		}
		if x.Kind == hir.VirtualCall {
			code = recv + "." + e.member(x.Name) + "(" + strings.Join(a, ",") + ")"
		} else {
			if recv != "" {
				a = append([]string{recv}, a...)
			}
			code = e.body(c.Name, m.Name) + "(" + strings.Join(a, ",") + ")"
		}
		if e.profile && x.Kind == hir.VirtualCall {
			e.mark(x.Node, "virtual_call")
			b.line("siteHit(%d,%s)", e.profileIndex(x.Node, "virtual_call"), recv)
		}
	case hir.Binary:
		a := b.expr(x.X)
		if x.Op == "&&" || x.Op == "||" {
			n := b.temp(t, a)
			condition := n
			if x.Op == "||" {
				condition = "!" + n
			}
			b.line("if %s {", condition)
			v := b.expr(x.Y)
			b.line("%s=%s }", n, v)
			return n
		}
		v := b.expr(x.Y)
		code = "(" + a + " " + x.Op + " " + v + ")"
		if x.Op == "==" || x.Op == "!=" {
			if x.X.Type.IsRef() || x.X.Type.Kind == hir.Optional && x.X.Type.Args[0].IsRef() {
				code = "equal(" + a + "," + v + ")"
				if x.Op == "!=" {
					code = "!" + code
				}
			}
		}
		if t.Kind == hir.Number && (x.Op == "+" || x.Op == "-" || x.Op == "*" || x.Op == "/") {
			if x.Op == "/" && (x.Y.Kind != hir.Lit || numberLiteral(x.Y.Value) == 0) {
				e.unsupported(x.Node, "Number division requires a non-zero literal divisor")
			}
			if !e.nonfiniteNumbers {
				code = "finite(" + code + ")"
			}
		}
		if t.Kind == hir.Number && x.Op == "%" {
			e.unsupported(x.Node, "Number remainder")
		}
		if t.Kind == hir.I32 || t.Kind == hir.I64 {
			if t.Kind == hir.I32 && (x.Op == "+" || x.Op == "-" || x.Op == "*") {
				// int64 holds every exact result of these operations on int32 inputs.
				code = fmt.Sprintf("checkedI32(int64(%s) %s int64(%s))", a, x.Op, v)
			} else if t.Kind == hir.I64 && (x.Op == "+" || x.Op == "-") {
				helper := "checkedAddI64"
				if x.Op == "-" {
					helper = "checkedSubI64"
				}
				code = helper + "(" + a + "," + v + ")"
				if x.CheckIntegerOverflow {
					code = "safeInteger(" + code + ")"
				}
			} else {
				code = fmt.Sprintf("%s(integerArithmetic(int64(%s),int64(%s),%q,%t,%d))", e.typ(t), a, v, x.Op, x.CheckIntegerOverflow, integerBits(t))
			}
		}
	case hir.Unary:
		a := b.expr(x.X)
		code = x.Op + a
		if t.Kind == hir.I32 || t.Kind == hir.I64 {
			if t.Kind == hir.I32 {
				code = "checkedI32(-int64(" + a + "))"
			} else {
				code = fmt.Sprintf("%s(integerArithmetic(0,int64(%s),%q,%t,%d))", e.typ(t), a, "-", x.CheckIntegerOverflow, integerBits(t))
			}
		} else if t.Kind == hir.Number && x.Op == "-" {
			if !e.nonfiniteNumbers {
				code = "finite(" + code + ")"
			}
		}
	case hir.Conditional:
		a := b.expr(x.X)
		n := b.fresh()
		b.line("var %s %s; _ = %s; if %s {", n, e.typ(t), n, a)
		v := b.value(x.Y, t)
		b.line("%s=%s }else{", n, v)
		v = b.value(x.Z, t)
		b.line("%s=%s }", n, v)
		return n
	case hir.FreshClassOf:
		code = e.name("fresh.descriptor."+x.Owner) + "()"
	case hir.ClassOf:
		code = "&" + e.name("descriptor."+x.Owner)
	case hir.InstanceOf:
		if len(e.liveClasses) > 0 && x.Y == nil && x.X.Kind == hir.RuntimeOp && x.X.Op == "dynamic.asRef" && x.X.X.Type.Kind == hir.Dynamic {
			d := b.expr(x.X.X)
			return b.temp(t, "func() bool {d:="+d+";if d==nil || d.Tag!=2 {return false};if s,ok:=d.Value.(interface{dynamicSource() *dynamic});ok && s.dynamicSource()!=nil {return false};_,ok:=d.Value.("+e.ref(x.Owner)+");return !nilRef(d.Value) && ok}()")
		}
		a := b.expr(x.X)
		if x.Y != nil {
			cv := b.expr(x.Y)
			return b.temp(t, "descriptorInstance("+a+","+cv+")")
		}
		if x.X.Type.Kind == hir.Optional && !x.X.Type.Args[0].IsRef() {
			a = "unwrap(" + a + ")"
		}
		code = "func() bool {_,ok:=any(" + a + ").(" + e.ref(x.Owner) + ");return !nilRef(" + a + ") && ok}()"
		if len(e.liveClasses) > 0 {
			code = "func() bool {v:=any(" + a + ");if s,ok:=v.(interface{dynamicSource() *dynamic});ok && s.dynamicSource()!=nil {return false};_,ok:=v.(" + e.ref(x.Owner) + ");return !nilRef(v) && ok}()"
		}
	case hir.IsUndefined:
		a := b.expr(x.X)
		code = "nilRef(" + a + ")"
		if x.X.Type.Kind == hir.Optional && !x.X.Type.Args[0].IsRef() {
			code = "!" + a + ".Has"
		}
	case hir.ToBoolean:
		a := b.expr(x.X)
		code = b.truth(a, x.X.Type)
	case hir.Narrow, hir.Cast:
		a := b.expr(x.X)
		src := x.X.Type
		if src.Kind == hir.Optional {
			if !src.Args[0].IsRef() {
				a = "unwrap(" + a + ")"
			}
			src = src.Args[0]
		}
		if src.Equal(t) || src.Kind == hir.Array && t.Kind == hir.Array && arrayStorage(src).Equal(arrayStorage(t)) {
			code = a
		} else if b.freshOptionalArray(x) {
			code = "presentArray(" + a + ")"
		} else if t.Kind == hir.ClassRef || t.Kind == hir.InterfaceRef || t.Kind == hir.Optional && t.Args[0].IsRef() {
			code = "castRef[" + e.typ(t) + "](" + a + ")"
		} else {
			e.unsupported(x.Node, "narrow "+t.String())
			code = a
		}
	case hir.NumericConvert, hir.CheckedNumericConvert:
		a := b.expr(x.X)
		if x.Kind == hir.CheckedNumericConvert {
			if t.Kind == hir.I32 {
				// An exact round trip proves integrality, finiteness and int32 range.
				narrowed := b.temp(t, e.typ(t)+"("+a+")")
				checks := []string{a + " != " + e.typ(x.X.Type) + "(" + narrowed + ")"}
				if x.Range.Min > math.MinInt32 {
					checks = append(checks, fmt.Sprintf("int64(%s) < %d", narrowed, x.Range.Min))
				}
				if x.Range.Max < math.MaxInt32 {
					checks = append(checks, fmt.Sprintf("int64(%s) > %d", narrowed, x.Range.Max))
				}
				b.line("if %s {panic(newRangeFault())}", strings.Join(checks, " || "))
				return narrowed
			} else {
				b.line("if float64(%s) < %d || float64(%s) > %d || float64(%s)!=float64(%s) || float64(%s)!=float64(%s) {panic(newRangeFault())}", a, x.Range.Min, a, x.Range.Max, a, a, a, e.typ(t)+"("+a+")")
			}
		}
		code = e.typ(t) + "(" + a + ")"
	case hir.NumericMinMax:
		helper := "numberMin"
		if x.Op == "max" {
			helper = "numberMax"
		}
		a := b.expr(x.X)
		items := a + ".Items"
		if e.liveStorage() {
			items = a + ".values()"
		}
		if x.Y == nil {
			if t.Kind == hir.Number {
				code = helper + "Array(" + items + ")"
			} else {
				n := b.temp(t, "0")
				b.line("if len(%s)==0 {panic(newRangeFault())}", items)
				b.line("%s=%s[0]", n, items)
				b.line("for _,v:=range %s { %s=%s(%s,v) }", items, n, x.Op, n)
				return n
			}
		} else {
			v := b.expr(x.Y)
			if t.Kind == hir.Number {
				code = helper + "(" + a + "," + v + ")"
			} else {
				code = x.Op + "(" + a + "," + v + ")"
			}
		}
	case hir.Seq:
		old := b.locals
		b.locals = clone(old)
		n := b.fresh()
		if t.Kind == hir.Void {
			b.line("{")
		} else {
			b.line("var %s %s; _=%s; {", n, e.typ(t), n)
		}
		b.stmts(x.Stmt.List)
		v := b.expr(x.Y)
		if t.Kind == hir.Void {
			b.line("}")
			n = ""
		} else {
			b.line("%s=%s }", n, v)
		}
		b.locals = old
		return n
	case hir.RuntimeOp:
		e.mark(x.Node, "runtime_op")
		return b.runtime(x)
	default:
		e.unsupported(x.Node, string(x.Kind))
		code = "*new(" + e.typ(t) + ")"
	}
	if kind := hir.SiteKind(x.Kind); kind != "" {
		if !e.profile || x.Kind != hir.VirtualCall {
			e.mark(x.Node, kind)
		}
		if e.profile && x.Kind != hir.VirtualCall {
			b.line("siteHit(%d,nil)", e.profileIndex(x.Node, kind))
		}
	}
	return b.temp(t, code)
}
func clone(m map[string]string) map[string]string {
	n := map[string]string{}
	for k, v := range m {
		n[k] = v
	}
	return n
}

// stmts emits a statement list, folding the lowered array-literal shape
// into one composite literal (see arrayLiteralItems).
func (b *body) stmts(list []*hir.Stmt) {
	for i := 0; i < len(list); i++ {
		if n := b.arrayLiteralItems(list, i); n > 0 {
			i += n
			continue
		}
		b.stmt(list[i])
	}
}

// arrayLiteralItems folds seq { var a = new array[T]; a.push(e1); ...; a.push(en) }
// into var a = &array[T]{Items:[]T{e1,...,en}}: the slice is born with
// len=cap=n, so a dense literal no longer grows from zero capacity and no
// longer pays a push call per element. Elements keep their per-expression
// evaluation statements, in the same order; the push calls they replace have
// no observable effect of their own. Literals with a spread (array.concat
// follows the pushes) and any interleaved statement keep the push path.
// Returns the number of push statements consumed, 0 when list[i] is not
// foldable.
func (b *body) arrayLiteralItems(list []*hir.Stmt, i int) int {
	s := list[i]
	if s == nil || s.Kind != hir.VarDecl || s.X == nil || s.X.Kind != hir.New || len(s.X.Args) != 0 ||
		s.X.OwnKeys != nil || s.Type.Kind != hir.Array || s.X.Type.Kind != hir.Array ||
		!arrayStorage(s.Type).Equal(arrayStorage(s.X.Type)) {
		return 0
	}
	run := 0
	for i+1+run < len(list) {
		p := list[i+1+run]
		if p == nil || p.Kind != hir.ExprStmt || p.X == nil || p.X.Kind != hir.RuntimeOp || p.X.Op != "array.push" ||
			p.X.X == nil || p.X.X.Kind != hir.Local || p.X.X.Name != s.Name || len(p.X.Args) != 1 {
			break
		}
		run++
	}
	if run == 0 {
		return 0
	}
	if next := i + 1 + run; next < len(list) {
		if n := list[next]; n.Kind == hir.VarDecl && n.X != nil && n.X.Kind == hir.RuntimeOp &&
			n.X.Op == "array.concat" && n.X.X != nil && n.X.X.Kind == hir.Local && n.X.X.Name == s.Name {
			return 0
		}
	}
	ps, _, ok := hir.RuntimeSignature("array.push", list[i+1].X.X.Type)
	if !ok || len(ps) != 1 {
		return 0
	}
	items := make([]string, 0, run)
	for _, p := range list[i+1 : i+1+run] {
		items = append(items, b.value(p.X.Args[0], ps[0]))
	}
	b.e.mark(s.X.Node, hir.SiteKind(hir.New))
	if b.e.profile {
		b.line("siteHit(%d,nil)", b.e.profileIndex(s.X.Node, hir.SiteKind(hir.New)))
	}
	elem := b.e.typ(arrayStorage(s.Type).Args[0])
	n := b.fresh()
	b.line("var %s %s = &array[%s]{Items:[]%s{%s}}; _ = %s", n, b.e.typ(s.Type), elem, elem, strings.Join(items, ","), n)
	b.locals[s.Name] = n
	return run
}

func (b *body) stmt(s *hir.Stmt) {
	if s == nil {
		return
	}
	e := b.e
	switch s.Kind {
	case hir.Block:
		capacity := b.capacity
		b.capacity = b.capacityPlans(s)
		defer func() { b.capacity = capacity }()
		old := b.locals
		b.locals = clone(old)
		b.line("{")
		b.stmts(s.List)
		b.line("}")
		b.locals = old
	case hir.VarDecl:
		n := b.fresh()
		v := ""
		if s.X != nil {
			v = " = " + b.value(s.X, s.Type)
		}
		b.line("var %s %s%s; _=%s", n, e.typ(s.Type), v, n)
		b.locals[s.Name] = n
	case hir.Assign:
		if s.X.Kind == hir.FieldGet && e.liveClasses[e.fieldOwner(e.classBy(s.X.X.Type.Name), s.X.Name).Name] {
			c := e.fieldOwner(e.classBy(s.X.X.Type.Name), s.X.Name)
			a := b.expr(s.X.X)
			v := b.value(s.Y, s.X.Type)
			b.line("%s.%s().%s(%s)", a, e.getter(c.Name), e.name("view.write."+c.Name+"."+s.X.Name), v)
		} else if s.X.Kind == hir.FieldGet && e.tracksOwnKeys(e.classBy(s.X.X.Type.Name)) {
			a := b.temp(s.X.X.Type, b.expr(s.X.X))
			v := b.value(s.Y, s.X.Type)
			owner := e.fieldOwner(e.classBy(s.X.X.Type.Name), s.X.Name)
			storage := a + "." + e.getter(owner.Name) + "()"
			b.line("%s.%s=%s", storage, e.member(s.X.Name), v)
			if !strings.HasPrefix(s.X.Name, "#") {
				b.line("if %s.ownKeys!=nil {%s.ownKeys.set(str(%q),nil)}", storage, storage, s.X.Name)
				if e.recordDeletion {
					b.line("%s.propertyWritten(str(%q))", storage, s.X.Name)
				}
			}
		} else if s.X.Kind == hir.FieldGet && e.recordDeletion {
			a := b.temp(s.X.X.Type, b.expr(s.X.X))
			v := b.value(s.Y, s.X.Type)
			c := e.fieldOwner(e.classBy(s.X.X.Type.Name), s.X.Name)
			b.line("%s.%s().%s=%s", a, e.getter(c.Name), e.member(s.X.Name), v)
			b.line("%s.%s().propertyWritten(str(%q))", a, e.getter(c.Name), s.X.Name)
		} else if s.X.Kind == hir.IndexGet {
			a, i := b.expr(s.X.X), b.expr(s.X.Y)
			v := b.value(s.Y, s.X.Type)
			b.line("%s.put(%s,%s)", a, i, v)
		} else {
			a := b.lvalue(s.X)
			v := b.value(s.Y, s.X.Type)
			b.line("%s=%s", a, v)
		}
	case hir.ExprStmt:
		b.expr(discardedResult(s.X))
	case hir.If:
		a := b.expr(s.X)
		b.line("if %s {", a)
		old := b.locals
		b.locals = clone(old)
		b.stmt(s.Body)
		b.locals = old
		if s.Else != nil {
			b.line("}else{")
			b.locals = clone(old)
			b.stmt(s.Else)
			b.locals = old
		}
		b.line("}")
	case hir.While:
		finish, trips := b.profileLoop(s)
		defer finish()
		e.mark(s.Node, "loop")
		b.line("for {")
		a := b.expr(s.X)
		b.line("if !%s {break}", a)
		if e.profile {
			b.line("%s++", trips)
		}
		b.beginLoop(s.Body)
		old := b.locals
		b.locals = clone(old)
		b.stmt(s.Body)
		b.locals = old
		b.endLoop()
		b.line("}")
	case hir.ForEach:
		finish, trips := b.profileLoop(s)
		defer finish()
		a := b.expr(s.X)
		index := b.fresh()
		e.mark(s.Node, "loop")
		if e.liveStorage() {
			b.line("for %s:=0; int32(%s)<%s.length(); %s++ {", index, index, a, index)
		} else {
			b.line("for %s:=0; %s<len(%s.Items); %s++ {", index, index, a, index)
		}
		if e.profile {
			b.line("%s++", trips)
		}
		code := a + ".Items[" + index + "]"
		if e.liveStorage() {
			code = "liveArrayRead(" + a + ",int32(" + index + "))"
		}
		if s.Type.IsRef() {
			if e.liveStorage() && s.Type.Kind == hir.Dynamic {
				code = "box(" + code + ")"
			} else {
				code = e.arrayReference(s.Type, code)
			}
		}
		n := b.temp(s.Type, code)
		old := b.locals
		b.locals = clone(old)
		b.locals[s.Name] = n
		b.beginLoop(s.Body)
		b.stmt(s.Body)
		b.endLoop()
		b.locals = old
		b.line("}")
	case hir.Break, hir.Continue:
		loop := b.loops[len(b.loops)-1]
		if loop.wrapped || loop.depth != b.tryDepth {
			b.line("panic(loopControl{Target:%q,Continue:%t})", loop.id, s.Kind == hir.Continue)
		} else {
			b.line(string(s.Kind))
		}
	case hir.Return:
		if b.m.Result.Kind == hir.Void {
			if s.X != nil {
				b.expr(s.X)
			}
			if b.tryDepth == 0 {
				b.line("return")
			} else {
				b.line("panic(returnedVoid{})")
			}
		} else {
			v := b.value(s.X, b.m.Result)
			if b.tryDepth == 0 {
				b.line("return %s", v)
			} else {
				b.line("panic(returned[%s]{%s})", e.typ(b.m.Result), v)
			}
		}
	case hir.Async:
		b.line("defer func(){if x:=recover();x!=nil {if _,ok:=x.(tsThrown);ok {result=&settledPromise[%s]{Rejection:x}}else{panic(x)}}}()", promiseValueType(e, b.m.Result))
		b.stmt(s.Body)
	case hir.Throw:
		v := b.expr(s.X)
		b.line("panic(payload[%s]{Type:%q,Value:%s})", e.typ(s.X.Type), s.X.Type.String(), v)
	case hir.Trap:
		b.line("panic(newTrap(%q))", s.Name)
	case hir.Finally:
		b.line("func(){defer func(){")
		old := b.locals
		b.locals = clone(old)
		b.tryDepth++
		b.stmt(s.Else)
		b.tryDepth--
		b.locals = old
		b.line("}()")
		b.locals = clone(old)
		b.tryDepth++
		b.stmt(s.Body)
		b.tryDepth--
		b.locals = old
		b.line("}()")
	case hir.Try:
		value := "caught.Value"
		if e.promiseUsed && s.Type.Kind == hir.Dynamic {
			b.line("func(){defer func(){if x:=recover();x!=nil {if caught,ok:=x.(tsThrown);ok {")
			value = "caught.tsValue()"
		} else if s.Type.Kind == hir.ClassRef || s.Type.Kind == hir.InterfaceRef {
			// Generic Go instantiations are invariant: payload[Derived] is
			// not payload[Base]. Match the thrown reference's ancestry, not
			// its static throw-site type. Only TS payloads implement this
			// interface; host faults and lowering traps still escape.
			e.usesTypedCatch = true
			b.line("func(){defer func(){if x:=recover();x!=nil {if thrown,ok:=x.(interface{payloadValue() any});ok {if caught,ok:=thrown.payloadValue().(%s);ok {", e.typ(s.Type))
			value = "caught"
		} else {
			b.line("func(){defer func(){if x:=recover();x!=nil {if caught,ok:=x.(payload[%s]);ok && caught.Type==%q {", e.typ(s.Type), s.Type.String())
		}
		old := b.locals
		b.locals = clone(old)
		n := b.temp(s.Type, value)
		b.locals[s.Name] = n
		b.tryDepth++
		b.stmt(s.Else)
		b.tryDepth--
		b.locals = old
		if s.Type.Kind == hir.ClassRef || s.Type.Kind == hir.InterfaceRef {
			b.line("}else{panic(x)}}else{panic(x)}}}()")
		} else {
			b.line("}else{panic(x)}}}()")
		}
		b.locals = clone(old)
		b.tryDepth++
		b.stmt(s.Body)
		b.tryDepth--
		b.locals = old
		b.line("}()")
	default:
		e.unsupported(s.Node, string(s.Kind))
	}
}

func integerBits(t hir.Type) int {
	if t.Kind == hir.I32 {
		return 32
	}
	return 64
}

// Check the entire body before emission so an unsupported operation is always
// diagnosed by its catalogue name, even if its operand type is also unsupported.
func (e *emitter) checkOps(s *hir.Stmt) {
	if s == nil {
		return
	}
	e.checkExprOps(s.X)
	e.checkExprOps(s.Y)
	e.checkOps(s.Body)
	e.checkOps(s.Else)
	for _, v := range s.List {
		e.checkOps(v)
	}
}
func (e *emitter) checkExprOps(x *hir.Expr) {
	if x == nil {
		return
	}
	if x.Kind == hir.New && x.Type.Kind == hir.RegExp || x.Kind == hir.RuntimeOp && x.Op == "regexp.new" {
		e.checkRegexp(x)
	}
	if x.Kind == hir.RuntimeOp && x.Op == "classvalue.newArgs" {
		e.constructorArgs = true
	}
	if x.Kind == hir.RuntimeOp && (x.Op == "json.stringifyConfig" || x.Op == "json.stringifySubset" || x.Op == "json.stringifyReplacer") {
		e.configJSON = true
		e.nativeJSON = e.nativeJSON || x.Op != "json.stringifyConfig"
		e.jsonReplacer = e.jsonReplacer || x.Op == "json.stringifyReplacer"
	}
	if x.Kind == hir.RuntimeOp && !supportedOps[x.Op] {
		e.unsupported(x.Node, x.Op)
	}
	e.checkExprOps(x.X)
	e.checkExprOps(x.Y)
	e.checkExprOps(x.Z)
	for _, a := range x.Args {
		e.checkExprOps(a)
	}
	e.checkOps(x.Stmt)
}

func numberLiteral(v any) float64 {
	r := reflect.ValueOf(v)
	switch {
	case r.Kind() <= reflect.Int64:
		return float64(r.Int())
	case r.Kind() <= reflect.Uintptr:
		return float64(r.Uint())
	default:
		return r.Float()
	}
}

// Returns crossing generated try/finally closures must unwind to the method.
// Ordinary returns use Go control flow and need no recovery wrapper.
func hasReturnBoundary(s *hir.Stmt) bool {
	if s == nil {
		return false
	}
	if s.Kind == hir.Try || s.Kind == hir.Finally {
		return true
	}
	if hasExprReturnBoundary(s.X) || hasExprReturnBoundary(s.Y) || hasReturnBoundary(s.Body) || hasReturnBoundary(s.Else) {
		return true
	}
	for _, child := range s.List {
		if hasReturnBoundary(child) {
			return true
		}
	}
	return false
}
func hasExprReturnBoundary(x *hir.Expr) bool {
	if x == nil {
		return false
	}
	if hasReturnBoundary(x.Stmt) || hasExprReturnBoundary(x.X) || hasExprReturnBoundary(x.Y) || hasExprReturnBoundary(x.Z) {
		return true
	}
	for _, arg := range x.Args {
		if hasExprReturnBoundary(arg) {
			return true
		}
	}
	return false
}

func (e *emitter) concreteClass(name string) bool {
	if name == hir.RootObject {
		return false
	}
	for _, c := range e.p.Classes {
		if c.Super == name {
			return false
		}
	}
	return true
}

// Convert immutable source literals once, rather than in each execution.
func (e *emitter) stringLiteral(value string) string {
	if e.stringLiterals == nil {
		e.stringLiterals = map[string]string{}
	}
	if name, ok := e.stringLiterals[value]; ok {
		return name
	}
	name := e.name("literal." + value)
	e.stringLiterals[value] = name
	fmt.Fprintf(&e.extra, "var %s = str(%q)\n", name, value)
	return name
}

type loopState struct {
	depth   int
	id      string
	wrapped bool
}

func (b *body) beginLoop(s *hir.Stmt) {
	l := loopState{depth: b.tryDepth, id: b.fresh(), wrapped: hasReturnBoundary(s)}
	if l.wrapped {
		b.line("%s:=0;func(){defer func(){if x:=recover();x!=nil{if c,ok:=x.(loopControl);ok&&c.Target==%q{if c.Continue{%s=2}else{%s=1}}else{panic(x)}}}()", l.id, l.id, l.id, l.id)
		b.tryDepth++
	}
	b.loops = append(b.loops, l)
}
func (b *body) endLoop() {
	l := b.loops[len(b.loops)-1]
	b.loops = b.loops[:len(b.loops)-1]
	if l.wrapped {
		b.tryDepth--
		b.line("}();if %s==1 {break};if %s==2 {continue}", l.id, l.id)
	}
}
