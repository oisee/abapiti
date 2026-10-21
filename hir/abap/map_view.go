package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
)

// The erased Map protocol is distinct from property storage: only library Map
// constructors supply its brand. Values cross the typed boundary when read.
func (e *emitter) mapStorageRuntime() {
	d := e.name("runtime.dynamic")
	s := e.name("map.storage")
	e.files[s+".intf.abap"] = "INTERFACE " + s + " PUBLIC.\nMETHODS brand RETURNING VALUE(result) TYPE abap_bool.\nMETHODS get IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE REF TO " + d + ".\nMETHODS put IMPORTING p0 TYPE string p1 TYPE REF TO " + d + ".\nMETHODS has IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE abap_bool.\nMETHODS remove IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE abap_bool.\nMETHODS clear.\nMETHODS keys RETURNING VALUE(result) TYPE REF TO " + d + ".\nMETHODS size RETURNING VALUE(result) TYPE i.\nENDINTERFACE.\n"
}
func (b *body) dynamicMapView(x *hir.Expr, n, a string) {
	e := b.e
	s := e.name("map.storage")
	adapter := b.rawTemp("REF TO " + s)
	b.mapLine("IF " + a + " IS NOT BOUND OR " + a + "->tag <> " + e.name("runtime.dynamic") + "=>tag_ref.\nRAISE EXCEPTION TYPE cx_sy_move_cast_error.\nENDIF.")
	b.mapLine(adapter + " ?= " + a + "->oval.\nIF " + adapter + "->brand( ) <> abap_true.\nRAISE EXCEPTION TYPE cx_sy_move_cast_error.\nENDIF.")
	b.mapLine("CREATE OBJECT " + n + ".\n" + n + "->js_map = abap_true.\n" + n + "->map_source = " + adapter + ".\n" + n + "->map_box = " + a + ".")
}
func (e *emitter) mapViewDefinition(t hir.Type) string {
	if t.Kind != hir.OrderedMap {
		return ""
	}
	out := "DATA js_map TYPE abap_bool.\n"
	if e.types["runtime.mapView"] && t.Args[0].Kind == hir.String {
		out += "INTERFACES " + e.name("map.storage") + ".\nDATA map_source TYPE REF TO " + e.name("map.storage") + ".\nDATA map_box TYPE REF TO " + e.name("runtime.dynamic") + ".\n"
	}
	return out
}
func (e *emitter) mapViewMethod(t hir.Type, method, code string) string {
	if !e.types["runtime.mapView"] || t.Kind != hir.OrderedMap || t.Args[0].Kind != hir.String {
		return code
	}
	b := &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}}
	b.mapLine("IF map_source IS BOUND.")
	value := t.Args[1]
	switch method {
	case "size", "has":
		b.mapLine("result = map_source->" + method + "( " + map[string]string{"size": "", "has": "p0"}[method] + " ).")
	case "delete":
		b.mapLine("result = map_source->remove( p0 ).")
	case "clear":
		b.mapLine("map_source->clear( ).")
	case "get":
		raw := b.temp(hir.T(hir.Dynamic))
		b.mapLine(raw + " = map_source->get( p0 ).\nIF " + raw + " IS BOUND.")
		b.liveUnbox("result", raw, hir.T(hir.Optional, value))
		b.mapLine("ENDIF.")
	case "set":
		raw := b.temp(hir.T(hir.Dynamic))
		b.boxDynamic(raw, "p1", value)
		b.mapLine("map_source->put( p0 = p0 p1 = " + raw + " ).\nresult = me.")
	case "keys", "values":
		raw := b.temp(hir.T(hir.Dynamic))
		key := b.temp(hir.T(hir.Dynamic))
		b.mapLine(raw + " = map_source->keys( ).\nresult = NEW #( ).\nLOOP AT " + raw + "->items INTO " + key + ".")
		if method == "keys" {
			b.mapLine("APPEND " + key + "->sval TO result->items.")
		} else {
			item := b.temp(hir.T(hir.Dynamic))
			v := b.temp(value)
			b.mapLine(item + " = map_source->get( " + key + "->sval ).")
			b.liveUnbox(v, item, value)
			b.mapLine("APPEND " + v + " TO result->items.")
		}
		b.mapLine("ENDLOOP.")
	default:
		return code
	}
	b.mapLine("RETURN.\nENDIF.")
	return b.constantDeclarations() + b.code.String() + code
}
func (e *emitter) mapStorageImplementation(t hir.Type) string {
	if !e.types["runtime.mapView"] || t.Kind != hir.OrderedMap || t.Args[0].Kind != hir.String {
		return ""
	}
	s := e.name("map.storage")
	d := e.name("runtime.dynamic")
	var out strings.Builder
	method := func(n, code string) { out.WriteString("METHOD " + s + "~" + n + ".\n" + code + "ENDMETHOD.\n") }
	method("brand", "result = js_map.\n")
	method("size", "result = size( ).\n")
	method("has", "result = has( p0 ).\n")
	method("remove", "result = delete( p0 ).\n")
	method("clear", "clear( ).\n")
	b := &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}, wholeMethod: true}
	v := b.temp(hir.T(hir.Optional, t.Args[1]))
	b.mapLine(v + " = get( p0 ).")
	b.boxDynamic("result", v, hir.T(hir.Optional, t.Args[1]))
	method("get", b.constantDeclarations()+b.code.String())
	b = &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}, wholeMethod: true}
	v = b.temp(t.Args[1])
	if t.Args[1].Kind != hir.Dynamic && t.Args[1].Kind != hir.Optional {
		b.mapLine("IF p1 IS NOT BOUND.\nRAISE EXCEPTION TYPE cx_sy_move_cast_error.\nENDIF.")
	}
	b.liveUnbox(v, "p1", t.Args[1])
	ignored := b.temp(t)
	b.mapLine(ignored + " = set( p0 = p0 p1 = " + v + " ).")
	method("put", b.constantDeclarations()+b.code.String())
	b = &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}, wholeMethod: true}
	keys := b.temp(hir.T(hir.Array, hir.T(hir.String)))
	k := b.temp(hir.T(hir.String))
	node := b.temp(hir.T(hir.Dynamic))
	b.mapLine(keys + " = keys( ).\nCREATE OBJECT result.\nresult->tag = " + d + "=>tag_array.\nLOOP AT " + keys + "->items INTO " + k + ".")
	b.boxDynamic(node, k, hir.T(hir.String))
	b.mapLine("APPEND " + node + " TO result->items.\nENDLOOP.")
	method("keys", b.constantDeclarations()+b.code.String())
	return out.String()
}

// body.line folds one declaration/assignment at a time.
func (b *body) mapLine(s string) {
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			b.line(line)
		}
	}
}
