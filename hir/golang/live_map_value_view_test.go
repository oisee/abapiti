package golang

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// A live structural Map field needs the proxy runtime independently of the
// dynamic.mapView HIR operation. Keep this fixture isolated so another Map
// view cannot accidentally pull the runtime helpers into the generated code.
func TestLiveStructuralMapFieldIncludesProxyRuntimeWithoutDynamicMapView(t *testing.T) {
	sourceValue := &hir.Class{Name: "probe.ts.SourceValue", Structural: true, Fields: []hir.Field{{Name: "value", Type: str}}}
	targetValue := &hir.Class{Name: "probe.ts.TargetValue", Structural: true, Fields: []hir.Field{{Name: "value", Type: str}}}
	sourceMap := hir.T(hir.OrderedMap, str, hir.Ref(sourceValue.Name))
	targetMap := hir.T(hir.OrderedMap, str, hir.Ref(targetValue.Name))
	program := &hir.Class{Name: "probe.ts.Program", Structural: true, Fields: []hir.Field{{Name: "values", Type: targetMap}}}
	view := &hir.Method{Name: "view", Static: true, Params: []hir.Param{{Name: "value", Type: hir.T(hir.Dynamic)}}, Result: hir.Ref(program.Name),
		Body: hir.B(ret(rt("dynamic.classView", local("value", hir.T(hir.Dynamic)), hir.Ref(program.Name))))}
	mark := &hir.Method{Name: "mark", Static: true, Params: []hir.Param{{Name: "value", Type: sourceMap}}, Result: sourceMap,
		Body: hir.B(ret(rt("map.markJS", local("value", sourceMap), sourceMap)))}
	module := &hir.Class{Name: "probe.ts", Methods: []*hir.Method{view, mark}}
	p := &hir.Program{Classes: []*hir.Class{sourceValue, targetValue, program, module}}
	files, _, err := emitPackage(p, "main", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(files["hir.go"], "dynamicMapView(") {
		t.Fatal("fixture unexpectedly emitted a dynamic.mapView operation")
	}
	for _, helper := range []string{"type mapProxy[", "type stringMapStorage interface", "stringMapSetView(k jsString", "if m.proxy != nil {\n\t\treturn m.proxy.get(k)"} {
		if !strings.Contains(files["runtime.go"], helper) {
			t.Fatalf("live structural Map did not include %q", helper)
		}
	}
	n := hir.NewNames()
	main := `source:=&fastOrderedMap[jsString,*` + n.Get("struct."+sourceValue.Name) + `]{};source.set(str("a"),&` + n.Get("struct."+sourceValue.Name) + `{` + n.Get("member.value") + `:str("before")});source=` + entry(module.Name, "mark") + `(source);
record:=parseJSON(str("{}"));record.put(str("values"),box(source));view:=` + entry(module.Name, "view") + `(record);values:=view.` + n.Get("view.read."+program.Name+".values") + `();if values.get(str("a")).Value.` + n.Get("view.read."+targetValue.Name+".value") + `()!=str("before") {panic("projected Map read")};values.set(str("b"),&` + n.Get("struct."+targetValue.Name) + `{` + n.Get("member.value") + `:str("after")});if source.get(str("b")).Value.` + n.Get("view.read."+sourceValue.Name+".value") + `()!=str("after") {panic("source alias did not observe projected Map write")};fmt.Println("ok")`
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}

func TestStructuralMapValueDecoderRequiresCompatibleFields(t *testing.T) {
	source := &hir.Class{Name: "probe.ts.Source", Structural: true, Fields: []hir.Field{{Name: "value", Type: str}}}
	target := &hir.Class{Name: "probe.ts.Target", Structural: true, Fields: []hir.Field{{Name: "value", Type: hir.T(hir.Number)}}}
	p := &hir.Program{Classes: []*hir.Class{source, target}}
	e := &emitter{p: p}
	if e.structuralMapValueMatch(source.Name, map[string]bool{target.Name: true}) {
		t.Fatal("same field name with incompatible value types must not enable write-back decoding")
	}
	target.Fields[0].Type = str
	if !e.structuralMapValueMatch(source.Name, map[string]bool{target.Name: true}) {
		t.Fatal("same data-only field contract should enable write-back decoding")
	}
	target.Fields = append(target.Fields, hir.Field{Name: "extra", Type: str})
	if e.structuralMapValueMatch(source.Name, map[string]bool{target.Name: true}) {
		t.Fatal("asymmetric field sets must not enable write-back decoding")
	}
}

func TestLiveMapValueViewsExpandNestedMapFields(t *testing.T) {
	sourceInner := &hir.Class{Name: "probe.ts.SourceInner", Structural: true, Fields: []hir.Field{{Name: "value", Type: str}}}
	targetInner := &hir.Class{Name: "probe.ts.TargetInner", Structural: true, Fields: []hir.Field{{Name: "value", Type: str}}}
	sourceOuter := &hir.Class{Name: "probe.ts.SourceOuter", Structural: true, Fields: []hir.Field{{Name: "inner", Type: hir.T(hir.OrderedMap, str, hir.Ref(sourceInner.Name))}}}
	targetOuter := &hir.Class{Name: "probe.ts.TargetOuter", Structural: true, Fields: []hir.Field{{Name: "inner", Type: hir.T(hir.OrderedMap, str, hir.Ref(targetInner.Name))}}}
	program := &hir.Class{Name: "probe.ts.Program", Structural: true, Fields: []hir.Field{{Name: "outer", Type: hir.T(hir.OrderedMap, str, hir.Ref(targetOuter.Name))}}}
	view := &hir.Method{Name: "view", Static: true, Params: []hir.Param{{Name: "value", Type: hir.T(hir.Dynamic)}}, Result: hir.Ref(program.Name), Body: hir.B(ret(rt("dynamic.classView", local("value", hir.T(hir.Dynamic)), hir.Ref(program.Name))))}
	markOuterType := hir.T(hir.OrderedMap, str, hir.Ref(sourceOuter.Name))
	markInnerType := hir.T(hir.OrderedMap, str, hir.Ref(sourceInner.Name))
	markOuter := &hir.Method{Name: "markOuter", Static: true, Params: []hir.Param{{Name: "value", Type: markOuterType}}, Result: markOuterType, Body: hir.B(ret(rt("map.markJS", local("value", markOuterType), markOuterType)))}
	markInner := &hir.Method{Name: "markInner", Static: true, Params: []hir.Param{{Name: "value", Type: markInnerType}}, Result: markInnerType, Body: hir.B(ret(rt("map.markJS", local("value", markInnerType), markInnerType)))}
	module := &hir.Class{Name: "probe.ts", Methods: []*hir.Method{view, markOuter, markInner}}
	p := &hir.Program{Classes: []*hir.Class{sourceInner, targetInner, sourceOuter, targetOuter, program, module}}
	e := &emitter{p: p, liveClasses: hir.LiveClassViewLayouts(p)}
	e.enableLiveMapValueViews()
	for _, name := range []string{targetOuter.Name, sourceOuter.Name, targetInner.Name, sourceInner.Name} {
		if !e.liveClasses[name] {
			t.Fatalf("nested Map value class was not expanded: %s in %v", name, e.liveClasses)
		}
	}
	if !e.liveMapValueTypes[sourceOuter.Name] || !e.liveMapValueTypes[sourceInner.Name] {
		t.Fatalf("nested Map sources lack write-back decoders: %v", e.liveMapValueTypes)
	}
}

// A structural class field may expose a Map with a different nominal value
// type. Projection must retain the Map's storage and translate values lazily.
func TestLiveStructuralMapValueViewPreservesAliases(t *testing.T) {
	sourceBase := &hir.Class{Name: "probe.ts.FrontendBase", Fields: []hir.Field{{Name: "inherited", Type: str}}}
	sourceNested := &hir.Class{Name: "probe.ts.FrontendNested", Structural: true, Fields: []hir.Field{{Name: "nestedValue", Type: str}}}
	sourceValue := &hir.Class{Name: "probe.ts.FrontendConstant", Super: sourceBase.Name, Structural: true, Fields: []hir.Field{{Name: "value", Type: str}, {Name: "label", Type: str}, {Name: "child", Type: hir.Ref(sourceNested.Name)}, {Name: "inner", Type: hir.T(hir.OrderedMap, str, hir.Ref(sourceNested.Name))}}}
	targetBase := &hir.Class{Name: "probe.ts.ConstantBase", Fields: []hir.Field{{Name: "inherited", Type: str}}}
	targetNested := &hir.Class{Name: "probe.ts.Nested", Structural: true, Fields: []hir.Field{{Name: "nestedValue", Type: str}}}
	foreignValue := &hir.Class{Name: "probe.ts.ForeignValue", Structural: true, Fields: []hir.Field{{Name: "label", Type: str}}}
	targetValue := &hir.Class{Name: "probe.ts.Constant", Super: targetBase.Name, Structural: true, Fields: []hir.Field{{Name: "value", Type: str}, {Name: "label", Type: str}, {Name: "child", Type: hir.Ref(targetNested.Name)}, {Name: "inner", Type: hir.T(hir.OrderedMap, str, hir.Ref(targetNested.Name))}}}
	sourceMap := hir.T(hir.OrderedMap, str, hir.Ref(sourceValue.Name))
	foreignMap := hir.T(hir.OrderedMap, str, hir.Ref(foreignValue.Name))
	sourceInnerMap := hir.T(hir.OrderedMap, str, hir.Ref(sourceNested.Name))
	targetInnerMap := hir.T(hir.OrderedMap, str, hir.Ref(targetNested.Name))
	targetMap := hir.T(hir.OrderedMap, str, hir.Ref(targetValue.Name))
	program := &hir.Class{Name: "probe.ts.Program", Structural: true, Fields: []hir.Field{{Name: "consts", Type: hir.T(hir.Optional, targetMap)}}}
	view := &hir.Method{Name: "view", Static: true, Params: []hir.Param{{Name: "value", Type: hir.T(hir.Dynamic)}}, Result: hir.Ref(program.Name),
		Body: hir.B(ret(rt("dynamic.classView", local("value", hir.T(hir.Dynamic)), hir.Ref(program.Name))))}
	mark := &hir.Method{Name: "mark", Static: true, Params: []hir.Param{{Name: "value", Type: sourceMap}}, Result: sourceMap,
		Body: hir.B(ret(rt("map.markJS", local("value", sourceMap), sourceMap)))}
	markForeign := &hir.Method{Name: "markForeign", Static: true, Params: []hir.Param{{Name: "value", Type: foreignMap}}, Result: foreignMap,
		Body: hir.B(ret(rt("map.markJS", local("value", foreignMap), foreignMap)))}
	markInner := &hir.Method{Name: "markInner", Static: true, Params: []hir.Param{{Name: "value", Type: sourceInnerMap}}, Result: sourceInnerMap,
		Body: hir.B(ret(rt("map.markJS", local("value", sourceInnerMap), sourceInnerMap)))}
	markTargetInner := &hir.Method{Name: "markTargetInner", Static: true, Params: []hir.Param{{Name: "value", Type: targetInnerMap}}, Result: targetInnerMap,
		Body: hir.B(ret(rt("map.markJS", local("value", targetInnerMap), targetInnerMap)))}
	erasedView := &hir.Method{Name: "erasedView", Static: true, Params: []hir.Param{{Name: "value", Type: hir.T(hir.Dynamic)}}, Result: targetMap,
		Body: hir.B(ret(rt("dynamic.mapView", local("value", hir.T(hir.Dynamic)), targetMap)))}
	module := &hir.Class{Name: "probe.ts", Methods: []*hir.Method{view, mark, markForeign, markInner, markTargetInner, erasedView}}
	p := &hir.Program{Classes: []*hir.Class{sourceBase, sourceNested, sourceValue, targetBase, targetNested, foreignValue, targetValue, program, module}}
	n := hir.NewNames()
	sourceValueLiteral := `&` + n.Get("struct."+sourceValue.Name) + `{` + n.Get("struct."+sourceBase.Name) + `:` + n.Get("struct."+sourceBase.Name) + `{` + n.Get("member.inherited") + `:str("base")},` + n.Get("member.value") + `:str("one"),` + n.Get("member.label") + `:str("source-label"),` + n.Get("member.child") + `:&` + n.Get("struct."+sourceNested.Name) + `{` + n.Get("member.nestedValue") + `:str("nested-one")},` + n.Get("member.inner") + `:innerSource}`
	targetValueLiteral := `&` + n.Get("struct."+targetValue.Name) + `{` + n.Get("struct."+targetBase.Name) + `:` + n.Get("struct."+targetBase.Name) + `{` + n.Get("member.inherited") + `:str("target-base")},` + n.Get("member.value") + `:str("two"),` + n.Get("member.label") + `:str("target-label"),` + n.Get("member.child") + `:&` + n.Get("struct."+targetNested.Name) + `{` + n.Get("member.nestedValue") + `:str("nested-two")},` + n.Get("member.inner") + `:innerTarget}`
	main := `source:=&fastOrderedMap[jsString,*` + n.Get("struct."+sourceValue.Name) + `]{};
innerSource:=&fastOrderedMap[jsString,*` + n.Get("struct."+sourceNested.Name) + `]{};innerSource.set(str("i"),&` + n.Get("struct."+sourceNested.Name) + `{` + n.Get("member.nestedValue") + `:str("nested-one")});innerSource=` + entry(module.Name, "markInner") + `(innerSource);
innerTarget:=&fastOrderedMap[jsString,*` + n.Get("struct."+targetNested.Name) + `]{};innerTarget.set(str("i"),&` + n.Get("struct."+targetNested.Name) + `{` + n.Get("member.nestedValue") + `:str("nested-two")});innerTarget=` + entry(module.Name, "markTargetInner") + `(innerTarget);
source.set(str("a"),` + sourceValueLiteral + `);
source=` + entry(module.Name, "mark") + `(source);
record:=parseJSON(str("{}"));record.put(str("consts"),box(source));view:=` + entry(module.Name, "view") + `(record);
projected:=view.` + n.Get("view.read."+program.Name+".consts") + `();
if projected.get(str("a")).Value.` + n.Get("view.read."+targetBase.Name+".inherited") + `()!=str("base") {panic("inherited projected read")};
if projected.get(str("a")).Value.` + n.Get("view.read."+targetValue.Name+".value") + `()!=str("one") || projected.get(str("a")).Value.` + n.Get("view.read."+targetValue.Name+".child") + `().` + n.Get("view.read."+targetNested.Name+".nestedValue") + `()!=str("nested-one") {panic("initial projected read")};
innerProjected:=projected.get(str("a")).Value.` + n.Get("view.read."+targetValue.Name+".inner") + `();if innerProjected.get(str("i")).Value.` + n.Get("view.read."+targetNested.Name+".nestedValue") + `()!=str("nested-one") {panic("nested Map read")};
innerProjected.set(str("j"),&` + n.Get("struct."+targetNested.Name) + `{` + n.Get("member.nestedValue") + `:str("nested-write")});if innerSource.get(str("j")).Value.` + n.Get("view.read."+sourceNested.Name+".nestedValue") + `()!=str("nested-write") {panic("nested Map write did not reach source alias")};
projected.set(str("b"),` + targetValueLiteral + `);
if source.get(str("b")).Value.` + n.Get("view.read."+sourceBase.Name+".inherited") + `()!=str("target-base") || source.get(str("b")).Value.` + n.Get("view.read."+sourceValue.Name+".child") + `().` + n.Get("view.read."+sourceNested.Name+".nestedValue") + `()!=str("nested-two") {panic("write did not reach source alias")};
source.set(str("c"),` + sourceValueLiteral + `);
if projected.get(str("c")).Value.` + n.Get("view.read."+targetValue.Name+".value") + `()!=str("one") {panic("source write not visible through projection")};
if !projected.delete(str("a")) || source.has(str("a")) {panic("delete did not reach source alias")};
projected.set(str("a"),&` + n.Get("struct."+targetValue.Name) + `{` + n.Get("member.value") + `:str("four")});
keys:=projected.keys().Items;if len(keys)!=3 || keys[0]!=str("b") || keys[1]!=str("c") || keys[2]!=str("a") {panic("insertion order after delete/reinsert")};
if projected.mapSize()!=3 || source.mapSize()!=3 {panic("map size differs across aliases")};
bad:=` + entry(module.Name, "view") + `(parseJSON(str("{\"consts\":[]}")));func(){defer func(){if x:=recover();x!=nil {if _,ok:=x.(rangeFault);!ok {panic(x)}} else {panic("array carrier was accepted")}}();bad.` + n.Get("view.read."+program.Name+".consts") + `()}();
foreign:=&fastOrderedMap[jsString,*` + n.Get("struct."+foreignValue.Name) + `]{};foreign=` + entry(module.Name, "markForeign") + `(foreign);
erased:=` + entry(module.Name, "erasedView") + `(box(foreign));erased.set(str("fresh"),` + targetValueLiteral + `);
if foreign.get(str("fresh")).Value.` + n.Get("view.read."+foreignValue.Name+".label") + `()!=str("target-label") {panic("mixed dynamic Map view lost its markJS decoder")};
fmt.Println("ok")`
	if got := execute(t, p, main); got != "ok\n" {
		t.Fatal(got)
	}
}
