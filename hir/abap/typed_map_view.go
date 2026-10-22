package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
)

func (e *emitter) typedMapViews() bool { return e.usesReviewedOp("map.view") }

// A single erased protocol keeps proxies live across value layout changes and
// string-to-boxed query views. Native keys are decoded only after tag checks.
func (e *emitter) typedMapProtocol() string {
	s := e.name("map.viewStorage")
	if e.types[s] {
		return s
	}
	e.types[s] = true
	d := e.typ(hir.T(hir.Dynamic))
	var out strings.Builder
	out.WriteString("INTERFACE " + s + " PUBLIC.\n")
	for _, m := range []string{"get", "has", "remove"} {
		result := d
		if m != "get" {
			result = "abap_bool"
		}
		out.WriteString("METHODS " + m + " IMPORTING p0 TYPE " + d + " RETURNING VALUE(result) TYPE " + result + ".\n")
	}
	out.WriteString("METHODS put IMPORTING p0 TYPE " + d + " p1 TYPE " + d + ".\nMETHODS clear.\nMETHODS keys RETURNING VALUE(result) TYPE " + d + ".\nMETHODS size RETURNING VALUE(result) TYPE i.\n")
	if e.usesReviewedOp("map.insertions") {
		out.WriteString("METHODS insertions RETURNING VALUE(result) TYPE i.\n")
	}
	out.WriteString("ENDINTERFACE.\n")
	e.files[s+".intf.abap"] = out.String()
	return s
}

func (e *emitter) typedMapDefinition(t hir.Type) string {
	if !e.typedMapViews() || t.Kind != hir.OrderedMap {
		return ""
	}
	return "INTERFACES " + e.typedMapProtocol() + ".\nDATA view_source TYPE REF TO " + e.typedMapProtocol() + ".\nDATA view_box TYPE " + e.typ(hir.T(hir.Dynamic)) + ".\n"
}

func (b *body) typedContainerView(x *hir.Expr, n string) bool {
	if x.Op == "set.view" {
		b.line(n + " = " + b.expr(x.X) + ".")
		return true
	}
	if x.Op != "map.view" {
		return false
	}
	a := b.expr(x.X)
	boxed := b.temp(hir.T(hir.Dynamic))
	b.mapLine("IF " + a + " IS NOT BOUND.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.")
	b.boxDynamic(boxed, a, x.X.Type)
	b.mapLine("CREATE OBJECT " + n + ".\n" + n + "->view_source ?= " + boxed + "->oval.\n" + n + "->view_box = " + boxed + ".")
	return true
}

func (e *emitter) typedMapMethod(t hir.Type, method, code string) string {
	if !e.typedMapViews() || t.Kind != hir.OrderedMap {
		return code
	}
	b := &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}, serial: 1000}
	b.mapLine("IF view_source IS BOUND.")
	var k string
	if method == "has" || method == "get" || method == "set" || method == "delete" {
		k = b.temp(hir.T(hir.Dynamic))
		b.boxDynamic(k, "p0", t.Args[0])
	}
	switch method {
	case "size", "insertions":
		b.line("result = view_source->" + method + "( ).")
	case "has", "delete":
		op := method
		if op == "delete" {
			op = "remove"
		}
		b.line("result = view_source->" + op + "( " + k + " ).")
	case "clear":
		b.line("view_source->clear( ).")
	case "get":
		raw := b.temp(hir.T(hir.Dynamic))
		b.line(raw + " = view_source->get( " + k + " ).")
		b.mapViewUnbox("result", raw, hir.T(hir.Optional, t.Args[1]))
	case "set":
		if t.Args[0].Kind == hir.Dynamic {
			b.line("RAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `writes through a boxed string-key Map view are unsupported`.")
			break
		}
		raw := b.temp(hir.T(hir.Dynamic))
		b.boxDynamic(raw, "p1", t.Args[1])
		b.mapLine("view_source->put( p0 = " + k + " p1 = " + raw + " ).\nresult = me.")
	case "keys", "values":
		keys, key := b.temp(hir.T(hir.Dynamic)), b.temp(hir.T(hir.Dynamic))
		b.mapLine(keys + " = view_source->keys( ).\nresult = NEW #( ).\nLOOP AT " + keys + "->items INTO " + key + ".")
		valueType := t.Args[0]
		raw := key
		if method == "values" {
			valueType = t.Args[1]
			raw = b.temp(hir.T(hir.Dynamic))
			b.line(raw + " = view_source->get( " + key + " ).")
		}
		val := b.temp(valueType)
		b.mapViewUnbox(val, raw, valueType)
		b.mapLine("APPEND " + val + " TO result->items.\nENDLOOP.")
	default:
		return code
	}
	b.mapLine("RETURN.\nENDIF.")
	if method == "set" && t.Args[0].Kind == hir.Dynamic {
		b.line("RAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `boxed Map writes require value-based SameValueZero`.")
		code = ""
	}
	return strings.ReplaceAll(b.constantDeclarations()+b.code.String(), "range_", "map_view_range_") + code
}

func (e *emitter) typedMapImplementation(t hir.Type) string {
	if !e.typedMapViews() || t.Kind != hir.OrderedMap {
		return ""
	}
	s := e.typedMapProtocol()
	var out strings.Builder
	method := func(n, code string) { out.WriteString("METHOD " + s + "~" + n + ".\n" + code + "ENDMETHOD.\n") }
	method("size", "result = size( ).\n")
	method("clear", "clear( ).\n")
	if e.usesReviewedOp("map.insertions") {
		method("insertions", "result = insertions( ).\n")
	}
	for _, op := range []string{"get", "has", "remove", "put"} {
		b := &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}}
		key := b.temp(t.Args[0])
		if t.Args[0].Kind == hir.String {
			b.mapLine("IF p0 IS NOT BOUND.\nRETURN.\nENDIF.\nIF p0->tag <> " + e.name("runtime.dynamic") + "=>tag_string.\nRETURN.\nENDIF.")
		}
		b.mapViewUnbox(key, "p0", t.Args[0])
		switch op {
		case "has", "remove":
			name := op
			if op == "remove" {
				name = "delete"
			}
			b.line("result = " + name + "( " + key + " ).")
		case "get":
			v := b.temp(hir.T(hir.Optional, t.Args[1]))
			b.line(v + " = get( " + key + " ).")
			b.boxDynamic("result", v, hir.T(hir.Optional, t.Args[1]))
		case "put":
			v := b.temp(t.Args[1])
			b.mapViewUnbox(v, "p1", t.Args[1])
			ignored := b.temp(t)
			b.line(ignored + " = set( p0 = " + key + " p1 = " + v + " ).")
		}
		method(op, b.constantDeclarations()+b.code.String())
	}
	b := &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}}
	keys := b.temp(hir.T(hir.Array, t.Args[0]))
	row := b.temp(arrayStorage(hir.T(hir.Array, t.Args[0])).Args[0])
	b.mapLine(keys + " = keys( ).\nCREATE OBJECT result.\nresult->tag = " + e.name("runtime.dynamic") + "=>tag_array.\nLOOP AT " + keys + "->items INTO " + row + ".")
	native := row
	if t.Args[0].IsRef() {
		native = b.temp(t.Args[0])
		b.line(native + " ?= " + row + ".")
	}
	box := b.temp(hir.T(hir.Dynamic))
	b.boxDynamic(box, native, t.Args[0])
	b.mapLine("APPEND " + box + " TO result->items.\nENDLOOP.")
	method("keys", b.constantDeclarations()+b.code.String())
	return out.String()
}

// Go erases reference Set elements to a single identity-key representation.
func (e *emitter) containerStorage(t hir.Type) hir.Type {
	if e.typedMapViews() && t.Kind == hir.OrderedMap && len(t.Args) == 2 && t.Args[0].IsRef() && t.Args[0].Kind != hir.Dynamic {
		return hir.T(hir.OrderedMap, hir.Ref(hir.RootObject), t.Args[1])
	}
	if e.usesReviewedOp("set.view") && t.Kind == hir.OrderedSet && len(t.Args) == 1 && t.Args[0].IsRef() {
		return hir.T(hir.OrderedSet, hir.Ref(hir.RootObject))
	}
	return t
}

func (e *emitter) referenceSetMethod(t hir.Type, method, code string) string {
	canonical := e.usesReviewedOp("set.view") || len(t.Args) > 0 && t.Args[0].Name == hir.RootObject && e.hasInterfaceAdapters()
	if !canonical || t.Kind != hir.OrderedSet || !t.Args[0].IsRef() || (method != "add" && method != "has" && method != "delete") {
		return code
	}
	b := &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}}
	box := b.temp(hir.T(hir.Dynamic))
	key := b.temp(hir.Ref(hir.RootObject))
	b.boxDynamic(box, "p0", hir.Ref(hir.RootObject))
	b.mapLine("IF " + box + " IS BOUND.\n" + key + " = " + box + "->as_ref( ).\nENDIF.")
	return b.constantDeclarations() + b.code.String() + strings.ReplaceAll(code, "p0", key)
}

func (b *body) mapViewUnbox(n, raw string, t hir.Type) {
	if t.Kind == hir.BigInt {
		b.mapLine("IF " + raw + " IS NOT BOUND.\nRAISE EXCEPTION TYPE cx_sy_move_cast_error.\nENDIF.\nIF " + raw + "->tag <> " + b.e.name("runtime.dynamic") + "=>tag_bigint.\nRAISE EXCEPTION TYPE cx_sy_move_cast_error.\nENDIF.\n" + n + " = " + raw + "->sval.")
		return
	}
	if t.Kind == hir.Optional && t.Args[0].Kind == hir.BigInt {
		b.mapLine("CLEAR " + n + ".\nIF " + raw + " IS BOUND.\n" + n + " = NEW #( ).\n" + n + "->has = abap_true.")
		b.mapViewUnbox(n+"->value", raw, t.Args[0])
		b.line("ENDIF.")
		return
	}
	if t.IsRef() && t.Kind != hir.Dynamic {
		b.mapLine("CLEAR " + n + ".\nIF " + raw + " IS BOUND.")
		if t.Kind == hir.RegExp || t.Kind == hir.Promise {
			object := b.temp(hir.Ref(hir.RootObject))
			b.mapLine(object + " = " + raw + "->as_ref( ).\n" + n + " ?= " + object + ".")
		} else {
			b.liveUnbox(n, raw, t)
		}
		b.line("ENDIF.")
		return
	}
	b.liveUnbox(n, raw, t)
}

// Reference-key Maps use canonical storage identity, including keys returned
// through a structural view. Keep this ABI change gated on Map view usage.
func (e *emitter) referenceMapMethod(t hir.Type, method, code string) string {
	canonical := e.typedMapViews() || len(t.Args) > 0 && t.Args[0].Name == hir.RootObject && e.hasInterfaceAdapters()
	if !canonical || t.Kind != hir.OrderedMap || !t.Args[0].IsRef() || t.Args[0].Kind == hir.Dynamic || (method != "set" && method != "get" && method != "has" && method != "delete") {
		return code
	}
	b := &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}, serial: 3000}
	box, key := b.temp(hir.T(hir.Dynamic)), b.temp(hir.Ref(hir.RootObject))
	b.boxDynamic(box, "p0", hir.Ref(hir.RootObject))
	b.mapLine("IF " + box + " IS BOUND.\n" + key + " = " + box + "->as_ref( ).\nENDIF.")
	code = strings.ReplaceAll(code, "p0", key)
	code = strings.ReplaceAll(code, " "+key+" =", " p0 =")
	return b.constantDeclarations() + b.code.String() + code
}
