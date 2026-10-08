package abap

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

func (e *emitter) runtime(t hir.Type) {
	t = arrayStorage(t)
	id := "runtime." + t.String()
	if e.types[id] {
		return
	}
	e.types[id] = true
	name := e.name(id)
	var def, impl strings.Builder
	line := func(s string) { def.WriteString(s + "\n") }
	method := func(n, s string) { impl.WriteString("METHOD " + n + ".\n" + s + "ENDMETHOD.\n") }
	switch t.Kind {
	case hir.Optional:
		line("DATA value TYPE " + e.typ(t.Args[0]) + ".")
		line("DATA has TYPE abap_bool.")
	case hir.Array:
		elem := e.typ(t.Args[0])
		opt := hir.T(hir.Optional, t.Args[0])
		line("TYPES items_type TYPE STANDARD TABLE OF " + elem + " WITH DEFAULT KEY.")
		line("DATA items TYPE items_type.")
		line("DATA view_bound TYPE abap_bool.")
		line("DATA view_base TYPE REF TO " + name + ".")
		line("DATA view_from TYPE i.")
		line("DATA view_to TYPE i.")
		line("METHODS view_length RETURNING VALUE(result) TYPE i.")
		method("view_length", "IF view_bound = abap_true.\nresult = view_to - view_from.\nELSE.\nresult = lines( items ).\nENDIF.\n")
		line("METHODS view_materialize.")
		method("view_materialize", "DATA base TYPE REF TO "+name+".\nDATA rows TYPE items_type.\nIF view_bound = abap_true.\nbase = view_base.\nAPPEND LINES OF base->items FROM view_from + 1 TO view_to TO rows.\nitems = rows.\nCLEAR view_bound.\nCLEAR view_base.\nview_from = 0.\nview_to = 0.\nENDIF.\n")
		line("METHODS reverse RETURNING VALUE(result) TYPE REF TO " + name + ".")
		method("reverse", "view_materialize( ).\nDATA reversed TYPE items_type.\nDATA row TYPE "+elem+".\nDATA idx TYPE i.\nidx = lines( items ).\nWHILE idx > 0.\nREAD TABLE items INDEX idx INTO row.\nAPPEND row TO reversed.\nidx = idx - 1.\nENDWHILE.\nitems = reversed.\nresult = me.\n")
		line("METHODS push IMPORTING p0 TYPE " + elem + " RETURNING VALUE(result) TYPE i.")
		method("push", "view_materialize( ).\nAPPEND p0 TO items.\nresult = lines( items ).\n")
		line("METHODS length RETURNING VALUE(result) TYPE i.")
		method("length", "result = view_length( ).\n")
		line("METHODS concat IMPORTING p0 TYPE REF TO " + name + " RETURNING VALUE(result) TYPE REF TO " + name + ".")
		method("concat", "view_materialize( ).\np0->view_materialize( ).\nCREATE OBJECT result.\nAPPEND LINES OF items TO result->items.\nAPPEND LINES OF p0->items TO result->items.\n")
		line("METHODS slice0 RETURNING VALUE(result) TYPE REF TO " + name + ".")
		method("slice0", "view_materialize( ).\nCREATE OBJECT result.\nAPPEND LINES OF items TO result->items.\n")
		line("METHODS slice1 IMPORTING p0 TYPE i RETURNING VALUE(result) TYPE REF TO " + name + ".")
		method("slice1", "view_materialize( ).\nDATA from TYPE i.\nfrom = p0.\nIF from < 0.\nfrom = lines( items ) + from.\nENDIF.\nIF from < 0.\nfrom = 0.\nENDIF.\nCREATE OBJECT result.\nIF from < lines( items ).\nAPPEND LINES OF items FROM from + 1 TO result->items.\nENDIF.\n")
		line("METHODS slice2 IMPORTING p0 TYPE i p1 TYPE i RETURNING VALUE(result) TYPE REF TO " + name + ".")
		method("slice2", "view_materialize( ).\nDATA from TYPE i.\nDATA upto TYPE i.\nfrom = p0.\nupto = p1.\nIF from < 0.\nfrom = lines( items ) + from.\nENDIF.\nIF upto < 0.\nupto = lines( items ) + upto.\nENDIF.\nIF from < 0.\nfrom = 0.\nENDIF.\nIF upto > lines( items ).\nupto = lines( items ).\nENDIF.\nCREATE OBJECT result.\nIF from < upto.\nAPPEND LINES OF items FROM from + 1 TO upto TO result->items.\nENDIF.\n")
		line("METHODS splice1 IMPORTING p0 TYPE i RETURNING VALUE(result) TYPE REF TO " + name + ".")
		method("splice1", "view_materialize( ).\nDATA from TYPE i.\nfrom = p0.\nIF from < 0.\nfrom = lines( items ) + from.\nENDIF.\nIF from < 0.\nfrom = 0.\nENDIF.\nIF from >= lines( items ).\nCREATE OBJECT result.\nRETURN.\nENDIF.\nCREATE OBJECT result.\nAPPEND LINES OF items FROM from + 1 TO result->items.\nDELETE items FROM from + 1.\n")
		line("METHODS splice1_view IMPORTING p0 TYPE i RETURNING VALUE(result) TYPE REF TO " + name + ".")
		method("splice1_view", "DATA base TYPE REF TO "+name+".\nDATA from TYPE i.\nDATA upto TYPE i.\nIF p0 <> 1.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.\nIF view_bound = abap_true.\nbase = view_base.\nfrom = view_from.\nupto = view_to.\nELSE.\nfrom = 0.\nupto = lines( items ).\nENDIF.\nCREATE OBJECT result.\nIF upto - from <= 1.\nRETURN.\nENDIF.\nIF view_bound = abap_false.\nCREATE OBJECT base.\nbase->items = items.\nCLEAR items.\nENDIF.\nresult->view_bound = abap_true.\nresult->view_base = base.\nresult->view_from = from + 1.\nresult->view_to = upto.\nview_bound = abap_true.\nview_base = base.\nview_from = from.\nview_to = from + 1.\n")
		line("METHODS splice2 IMPORTING p0 TYPE i p1 TYPE i RETURNING VALUE(result) TYPE REF TO " + name + ".")
		method("splice2", "view_materialize( ).\nDATA from TYPE i.\nDATA cnt TYPE i.\nDATA last TYPE i.\nfrom = p0.\nIF from < 0.\nfrom = lines( items ) + from.\nENDIF.\nIF from < 0.\nfrom = 0.\nENDIF.\nIF from > lines( items ).\nfrom = lines( items ).\nENDIF.\nfrom = from + 1.\ncnt = p1.\nIF cnt < 0.\ncnt = 0.\nENDIF.\nIF from + cnt - 1 > lines( items ).\ncnt = lines( items ) - from + 1.\nENDIF.\nIF cnt < 0.\ncnt = 0.\nENDIF.\nCREATE OBJECT result.\nIF cnt > 0.\nlast = from + cnt - 1.\nAPPEND LINES OF items FROM from TO last TO result->items.\nDELETE items FROM from TO last.\nENDIF.\n")
		line("METHODS splice3 IMPORTING p0 TYPE i p1 TYPE i p2 TYPE " + elem + " RETURNING VALUE(result) TYPE REF TO " + name + ".")
		method("splice3", "view_materialize( ).\nDATA from TYPE i.\nDATA cnt TYPE i.\nfrom = p0.\nIF from < 0.\nfrom = lines( items ) + from.\nENDIF.\nIF from < 0.\nfrom = 0.\nENDIF.\nIF from > lines( items ).\nfrom = lines( items ).\nENDIF.\nfrom = from + 1.\ncnt = p1.\nIF cnt < 0.\ncnt = 0.\nENDIF.\nIF from + cnt - 1 > lines( items ).\ncnt = lines( items ) - from + 1.\nENDIF.\nIF cnt < 0.\ncnt = 0.\nENDIF.\nCREATE OBJECT result.\nIF cnt > 0.\nAPPEND LINES OF items FROM from TO from + cnt - 1 TO result->items.\nDELETE items FROM from TO from + cnt - 1.\nENDIF.\nINSERT p2 INTO items INDEX from.\n")
		line("METHODS pop RETURNING VALUE(result) TYPE " + e.typ(opt) + ".")
		if t.Args[0].IsRef() {
			method("pop", "view_materialize( ).\nIF lines( items ) > 0.\nREAD TABLE items INDEX lines( items ) INTO result.\nDELETE items INDEX lines( items ).\nENDIF.\n")
		} else {
			method("pop", "view_materialize( ).\nDATA v TYPE "+elem+".\nIF lines( items ) > 0.\nREAD TABLE items INDEX lines( items ) INTO v.\nCREATE OBJECT result.\nresult->has = abap_true.\nresult->value = v.\nDELETE items INDEX lines( items ).\nENDIF.\n")
		}
		line("METHODS indexOf IMPORTING p0 TYPE " + elem + " RETURNING VALUE(result) TYPE i.")
		if t.Args[0].Kind == hir.String {
			method("indexOf", "view_materialize( ).\nDATA row TYPE "+elem+".\nresult = -1.\nLOOP AT items INTO row.\nIF row = p0.\nresult = sy-tabix - 1.\nEXIT.\nENDIF.\nENDLOOP.\n")
		} else {
			method("indexOf", "view_materialize( ).\nDATA row TYPE "+elem+".\nresult = -1.\nLOOP AT items INTO row.\nIF row = p0.\nresult = sy-tabix - 1.\nEXIT.\nENDIF.\nENDLOOP.\n")
		}
		line("METHODS includes IMPORTING p0 TYPE " + elem + " RETURNING VALUE(result) TYPE abap_bool.")
		method("includes", "view_materialize( ).\nREAD TABLE items WITH KEY table_line = p0 TRANSPORTING NO FIELDS.\nIF sy-subrc = 0.\nresult = abap_true.\nENDIF.\n")
		if !t.Args[0].IsRef() && t.Args[0].Kind != hir.Optional {
			line("METHODS join IMPORTING p0 TYPE " + e.typ(hir.T(hir.Optional, hir.T(hir.String))) + " RETURNING VALUE(result) TYPE string.")
		}
		if t.Args[0].Kind == hir.String {
			method("join", "view_materialize( ).\nIF p0 IS BOUND AND p0->has = abap_true.\nCONCATENATE LINES OF items INTO result SEPARATED BY p0->value RESPECTING BLANKS.\nELSE.\nCONCATENATE LINES OF items INTO result SEPARATED BY `,` RESPECTING BLANKS.\nENDIF.\n")
		} else if !t.Args[0].IsRef() && t.Args[0].Kind != hir.Optional {
			method("join", "view_materialize( ).\nDATA row TYPE "+elem+".\nDATA part TYPE string.\nLOOP AT items INTO row.\npart = |{ row }|.\nIF sy-tabix = 1.\nresult = part.\nELSEIF p0 IS BOUND AND p0->has = abap_true.\nCONCATENATE result p0->value part INTO result RESPECTING BLANKS.\nELSE.\nCONCATENATE result `,` part INTO result RESPECTING BLANKS.\nENDIF.\nENDLOOP.\n")
		}
		line("METHODS get IMPORTING p0 TYPE i RETURNING VALUE(result) TYPE " + e.typ(opt) + ".")
		if t.Args[0].IsRef() {
			code := "view_materialize( ).\nIF p0 < 0.\nRETURN.\nENDIF.\nDATA(idx) = p0 + 1.\nREAD TABLE items INDEX idx INTO DATA(val).\nIF sy-subrc = 0.\nresult = val.\nENDIF.\n"
			method("get", code)
		} else {
			code := "view_materialize( ).\nIF p0 < 0.\nRETURN.\nENDIF.\nDATA(idx) = p0 + 1.\nREAD TABLE items INDEX idx INTO DATA(val).\nIF sy-subrc = 0.\nresult = NEW #( ).\nresult->has = abap_true.\nresult->value = val.\nENDIF.\n"
			method("get", code)
		}
	case hir.OrderedMap, hir.OrderedSet:
		key := e.typ(t.Args[0])
		value := hir.T(hir.Bool)
		if t.Kind == hir.OrderedMap {
			value = t.Args[1]
		}
		vt := e.typ(value)
		line("TYPES: BEGIN OF entry, k TYPE " + key + ", v TYPE " + vt + ", END OF entry.")
		line("TYPES entries_type TYPE STANDARD TABLE OF entry WITH DEFAULT KEY.")
		line("DATA entries TYPE entries_type.")
		line("METHODS size RETURNING VALUE(result) TYPE i.")
		method("size", "result = lines( entries ).\n")
		line("METHODS has IMPORTING p0 TYPE " + key + " RETURNING VALUE(result) TYPE abap_bool.")
		method("has", "READ TABLE entries WITH KEY k = p0 TRANSPORTING NO FIELDS.\nIF sy-subrc = 0.\nresult = abap_true.\nENDIF.\n")
		op := "add"
		params := ""
		val := "abap_true"
		if t.Kind == hir.OrderedMap {
			op = "set"
			params = " p1 TYPE " + vt
			val = "p1"
		}
		line("METHODS " + op + " IMPORTING p0 TYPE " + key + params + " RETURNING VALUE(result) TYPE REF TO " + name + ".")
		method(op, "READ TABLE entries WITH KEY k = p0 ASSIGNING FIELD-SYMBOL(<row>).\nIF sy-subrc = 0.\n<row>-v = "+val+".\nELSE.\nAPPEND VALUE #( k = p0 v = "+val+" ) TO entries.\nENDIF.\nresult = me.\n")
		if t.Kind == hir.OrderedMap {
			opt := e.typ(hir.T(hir.Optional, value))
			line("METHODS get IMPORTING p0 TYPE " + key + " RETURNING VALUE(result) TYPE " + opt + ".")
			code := "READ TABLE entries WITH KEY k = p0 INTO DATA(row).\nIF sy-subrc = 0.\n"
			if value.IsRef() {
				code += "result = row-v.\n"
			} else {
				code += "result = NEW #( ).\nresult->has = abap_true.\nresult->value = row-v.\n"
			}
			method("get", code+"ENDIF.\n")
		}
		if t.Kind == hir.OrderedSet {
			line("METHODS copy IMPORTING p0 TYPE REF TO " + name + " RETURNING VALUE(result) TYPE REF TO " + name + ".")
			method("copy", "CREATE OBJECT result.\nAPPEND LINES OF p0->entries TO result->entries.\n")
		}
		op = "values"
		if t.Kind == hir.OrderedMap {
			op = "keys"
		}
		line("METHODS " + op + " RETURNING VALUE(result) TYPE " + e.typ(hir.T(hir.Array, t.Args[0])) + ".")
		method(op, "result = NEW #( ).\nLOOP AT entries INTO DATA(row).\nAPPEND row-k TO result->items.\nENDLOOP.\n")
	}
	e.files[name+".clas.abap"] = "CLASS " + name + " DEFINITION PUBLIC CREATE PUBLIC.\nPUBLIC SECTION.\n" + def.String() + "PROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\n" + impl.String() + "ENDCLASS.\n"
}
func (e *emitter) exception(t hir.Type) string {
	name := e.name("exception." + t.String())
	file := name + ".clas.abap"
	if _, ok := e.files[file]; ok {
		return name
	}
	e.files[file] = "CLASS " + name + " DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.\nPUBLIC SECTION.\nDATA payload TYPE " + e.typ(t) + ".\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\nENDCLASS.\n"
	return name
}

// support emits the phase-2 runtime and support classes, gated on use: the
// class-value descriptors (with the hidden classOf method's backing) and the
// factory, the builtin Error, the regex and dynamic boxes. Programs that do
// not use them keep their exact previous output.
func (e *emitter) support() {
	if e.descriptors {
		e.classvalueRuntime()
		if e.errorUsed {
			e.descriptorOf("builtin.Error")
		}
		e.descriptorShards()
	}
	if e.errorUsed {
		name := e.name("builtin.Error")
		opt := e.typ(hir.T(hir.Optional, hir.T(hir.String)))
		hidden, impl := "", ""
		if e.descriptors {
			hidden = "INTERFACES " + e.name("runtime.described") + ".\nMETHODS " + e.name("builtin.classOf") + " RETURNING VALUE(result) TYPE REF TO " + e.name("runtime.classvalue") + ".\n"
			impl = "METHOD " + e.name("builtin.classOf") + ".\nresult = " + e.descriptorOf("builtin.Error") + ".\nENDMETHOD.\nMETHOD " + e.name("runtime.described") + "~" + e.name("builtin.classOf") + ".\nresult = me->" + e.name("builtin.classOf") + "( ).\nENDMETHOD.\n"
		}
		e.files[name+".clas.abap"] = "CLASS " + name + " DEFINITION PUBLIC CREATE PUBLIC.\nPUBLIC SECTION.\n" + hidden + "DATA " + e.member("message") + " TYPE string.\nMETHODS constructor IMPORTING " + e.param("message") + " TYPE " + opt + " OPTIONAL.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\nMETHOD constructor.\nIF " + e.param("message") + " IS BOUND.\n" + e.member("message") + " = " + e.param("message") + "->value.\nENDIF.\nENDMETHOD.\n" + impl + "ENDCLASS.\n"
	}
	if e.regexpUsed {
		e.regexpRuntime()
	}
	if e.dynamicUsed {
		e.dynamicRuntime()
	}
}

// classvalueRuntime emits the descriptor class: the TS class name (for
// constructor.name), the emitted ABAP class name (for the factory), the
// parent descriptor (for instanceof with a dynamic operand) and the names of
// the static members (for reflection-style presence tests).
func (e *emitter) classvalueRuntime() {
	id := "runtime.classvalue"
	if e.types[id] {
		return
	}
	e.types[id] = true
	name := e.name(id)
	arr := e.typ(hir.T(hir.Array, hir.T(hir.String)))
	e.files[name+".clas.abap"] = "CLASS " + name + " DEFINITION PUBLIC CREATE PUBLIC.\nPUBLIC SECTION.\nDATA name TYPE string.\nDATA emitted TYPE string.\nDATA parent TYPE REF TO " + name + ".\nDATA static_names TYPE " + arr + ".\nMETHODS constructor IMPORTING name TYPE string emitted TYPE string parent TYPE REF TO " + name + " static_names TYPE " + arr + ".\nMETHODS has IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE abap_bool.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\nMETHOD constructor.\nme->name = name.\nme->emitted = emitted.\nme->parent = parent.\nme->static_names = static_names.\nENDMETHOD.\nMETHOD has.\nDATA s TYPE string.\nIF static_names IS NOT BOUND.\nRETURN.\nENDIF.\nLOOP AT static_names->items INTO s.\nIF s = p0.\nresult = abap_true.\nRETURN.\nENDIF.\nENDLOOP.\nENDMETHOD.\nENDCLASS.\n"
}

// descriptorOf returns the static descriptor reference expression for a class.
func (e *emitter) descriptorOf(className string) string {
	shard, field := e.descriptorShard(className)
	return shard + "=>" + field
}

type descRef struct{ shard, field string }

// descriptorShard returns (creating once) the shard class and field name of
// the descriptor singleton for className. Shards are keyed by the first
// character of the inheritance root so ancestry stays within one module.
func (e *emitter) descriptorShard(className string) (string, string) {
	if e.descIndex == nil {
		e.descIndex = map[string]descRef{}
		e.descOrder = map[string][]string{}
	}
	if r, ok := e.descIndex[className]; ok {
		return r.shard, r.field
	}
	root := className
	for e.superOf(root) != "" {
		root = e.superOf(root)
	}
	// Keep ancestry in one shard: eager class constructors on OSG-JS
	// cannot read a singleton from a module that has not registered yet.
	key := "runtime.descriptors." + shardKey(root)
	if _, ok := e.descCount[key]; !ok {
		e.descCount[key] = 0
	}
	n := e.descCount[key]
	e.descCount[key] = n + 1
	r := descRef{shard: e.name(key), field: e.member(fmt.Sprintf("d%d", n))}
	e.descIndex[className] = r
	e.descOrder[key] = append(e.descOrder[key], className)
	return r.shard, r.field
}

func shardKey(className string) string {
	for _, c := range className {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			return strings.ToLower(string(c))
		}
	}
	return "0"
}

// descriptorShards writes the shard classes, each with its descriptors'
// CLASS-DATA and a class_constructor that creates them with name, emitted
// name, parent descriptor and static member names.
func (e *emitter) descriptorShards() {
	arr := e.typ(hir.T(hir.Array, hir.T(hir.String)))
	for key, classes := range e.descOrder {
		shard := e.name(key)
		var def, init strings.Builder
		fmt.Fprintf(&def, "CLASS %s DEFINITION PUBLIC CREATE PUBLIC.\nPUBLIC SECTION.\n", shard)
		for i := range classes {
			fmt.Fprintf(&def, "CLASS-DATA %s TYPE REF TO %s.\n", e.member(fmt.Sprintf("d%d", i)), e.name("runtime.classvalue"))
		}
		def.WriteString("CLASS-METHODS class_constructor.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + shard + " IMPLEMENTATION.\nMETHOD class_constructor.\n")
		empty := "t_empty"
		fmt.Fprintf(&init, "DATA %s TYPE %s.\nCREATE OBJECT %s.\n", empty, arr, empty)
		for i, c := range classes {
			f := e.member(fmt.Sprintf("d%d", i))
			statics := ""
			var names []string
			for _, m := range e.staticsOf(c) {
				names = append(names, m)
			}
			if len(names) > 0 {
				tmp := fmt.Sprintf("t_s%d", i)
				fmt.Fprintf(&init, "DATA %s TYPE %s.\nCREATE OBJECT %s.\n", tmp, arr, tmp)
				for _, n := range names {
					fmt.Fprintf(&init, "APPEND `%s` TO %s->items.\n", strings.ReplaceAll(n, "`", "``"), tmp)
				}
				statics = tmp
			} else {
				statics = empty
			}
			parent := fmt.Sprintf("init_parent%d", i)
			fmt.Fprintf(&init, "DATA %s TYPE REF TO %s.\n", parent, e.name("runtime.classvalue"))
			fmt.Fprintf(&init, "CREATE OBJECT %s EXPORTING name = `%s` emitted = `%s` parent = %s static_names = %s.\n",
				f, strings.ReplaceAll(tsName(c), "`", "``"), strings.ReplaceAll(e.name(c), "`", "``"), parent, statics)
		}
		// Publish all singletons before linking ancestry within the shard.
		for i, c := range classes {
			if super := e.superOf(c); super != "" {
				fmt.Fprintf(&init, "%s->parent = %s.\n", e.member(fmt.Sprintf("d%d", i)), e.descriptorOf(super))
			}
		}
		def.WriteString(init.String() + "ENDMETHOD.\nENDCLASS.\n")
		e.files[shard+".clas.abap"] = def.String()
	}
	// The factory exposes known zero-argument constructors as explicit type
	// references so runtime closure discovery includes descriptor-only classes.
	// Constructors with required arguments retain the checked dynamic fallback.
	factory := e.name("runtime.classvalue.factory")
	var dispatch strings.Builder
	dispatch.WriteString("CASE class_name.\n")
	for _, c := range e.p.Classes {
		if c.Abstract {
			continue
		}
		ctor := e.p.Constructor(c.Name)
		callable := true
		if ctor != nil {
			for _, p := range ctor.Params {
				if p.Type.Kind != hir.Optional {
					callable = false
				}
			}
		}
		if !callable {
			continue
		}
		fmt.Fprintf(&dispatch, "WHEN `%s`.\nCREATE OBJECT result TYPE %s.\n", strings.ToUpper(e.name(c.Name)), e.name(c.Name))
	}
	dispatch.WriteString("WHEN OTHERS.\nCREATE OBJECT result TYPE (class_name).\nENDCASE.\n")
	e.files[factory+".clas.abap"] = "CLASS " + factory + " DEFINITION PUBLIC CREATE PUBLIC.\nPUBLIC SECTION.\nCLASS-METHODS new IMPORTING p0 TYPE REF TO " + e.name("runtime.classvalue") + " RETURNING VALUE(result) TYPE REF TO object.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + factory + " IMPLEMENTATION.\nMETHOD new.\nDATA class_name TYPE string.\nclass_name = p0->emitted.\nTRANSLATE class_name TO UPPER CASE.\n" + dispatch.String() + "ENDMETHOD.\nENDCLASS.\n"
}

// staticsOf lists the raw member names of the static members of a class.
func (e *emitter) staticsOf(className string) []string {
	c := e.classBy(className)
	if c == nil {
		return nil
	}
	var out []string
	for _, f := range c.Fields {
		if f.Static {
			out = append(out, f.Name)
		}
	}
	for _, m := range c.Methods {
		if m.Static && m.Name != "class_constructor" {
			out = append(out, m.Name)
		}
	}
	return out
}

func (e *emitter) superOf(className string) string {
	if c := e.classBy(className); c != nil {
		return c.Super
	}
	return ""
}

// tsName recovers the TypeScript class name from the qualified HIR name
// (the part after the last dot of the file path prefix).
func tsName(className string) string {
	if i := strings.LastIndex(className, "."); i >= 0 {
		return className[i+1:]
	}
	return className
}

// regexpRuntime emits the regex class: the pattern is stored as written (for
// source) and translated to POSIX classes at construction (see the stage-0
// probe: \w and \d differ between JavaScript and ABAP regexes).
func (e *emitter) regexpRuntime() {
	id := "runtime.regexp"
	if e.types[id] {
		return
	}
	e.types[id] = true
	name := e.name(id)
	e.files[name+".clas.abap"] = "CLASS " + name + ` DEFINITION PUBLIC CREATE PUBLIC.
PUBLIC SECTION.
DATA source TYPE string.
DATA posix TYPE string.
DATA excluded TYPE string.
DATA ignore_case TYPE abap_bool.
DATA global TYPE abap_bool.
METHODS constructor IMPORTING pattern TYPE string flags TYPE string excluded_pattern TYPE string OPTIONAL.
METHODS test IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE abap_bool.
METHODS replace IMPORTING p0 TYPE string p1 TYPE string RETURNING VALUE(result) TYPE string.
PROTECTED SECTION.
PRIVATE SECTION.
ENDCLASS.
CLASS ` + name + ` IMPLEMENTATION.
METHOD constructor.
DATA i TYPE i.
DATA n TYPE i.
DATA c TYPE string.
DATA in_class TYPE abap_bool.
source = pattern.
excluded = excluded_pattern.
n = strlen( pattern ).
WHILE i < n.
  c = pattern+i(1).
  IF c = '\'.
    i = i + 1.
    IF i >= n.
      EXIT.
    ENDIF.
    c = pattern+i(1).
    CASE c.
      WHEN 'w'.
        IF in_class = abap_true.
          CONCATENATE posix 'A-Za-z0-9_' INTO posix RESPECTING BLANKS.
        ELSE.
          CONCATENATE posix '[A-Za-z0-9_]' INTO posix RESPECTING BLANKS.
        ENDIF.
      WHEN 'W'.
        IF in_class = abap_true.
          CONCATENATE posix '\W' INTO posix RESPECTING BLANKS.
        ELSE.
          CONCATENATE posix '[^A-Za-z0-9_]' INTO posix RESPECTING BLANKS.
        ENDIF.
      WHEN 'd'.
        IF in_class = abap_true.
          CONCATENATE posix '0-9' INTO posix RESPECTING BLANKS.
        ELSE.
          CONCATENATE posix '[0-9]' INTO posix RESPECTING BLANKS.
        ENDIF.
      WHEN 'D'.
        IF in_class = abap_true.
          CONCATENATE posix '\D' INTO posix RESPECTING BLANKS.
        ELSE.
          CONCATENATE posix '[^0-9]' INTO posix RESPECTING BLANKS.
        ENDIF.
      WHEN OTHERS.
        CONCATENATE posix '\' c INTO posix RESPECTING BLANKS.
    ENDCASE.
  ELSE.
    IF c = '['.
      in_class = abap_true.
    ELSEIF c = ']'.
      in_class = abap_false.
    ENDIF.
    CONCATENATE posix c INTO posix RESPECTING BLANKS.
  ENDIF.
  i = i + 1.
ENDWHILE.
n = strlen( flags ).
i = 0.
WHILE i < n.
  c = flags+i(1).
  IF c = 'i'.
    ignore_case = abap_true.
  ELSEIF c = 'g'.
    global = abap_true.
  ENDIF.
  i = i + 1.
ENDWHILE.
ENDMETHOD.
METHOD test.
IF excluded IS NOT INITIAL.
  IF ignore_case = abap_true.
    FIND REGEX excluded IN p0 IGNORING CASE.
  ELSE.
    FIND REGEX excluded IN p0.
  ENDIF.
  IF sy-subrc = 0.
    RETURN.
  ENDIF.
ENDIF.
IF ignore_case = abap_true.
  FIND REGEX posix IN p0 IGNORING CASE.
ELSE.
  FIND REGEX posix IN p0.
ENDIF.
IF sy-subrc = 0.
  result = abap_true.
ENDIF.
ENDMETHOD.
METHOD replace.
result = p0.
IF excluded IS NOT INITIAL.
  IF ignore_case = abap_true.
    FIND REGEX excluded IN p0 IGNORING CASE.
  ELSE.
    FIND REGEX excluded IN p0.
  ENDIF.
  IF sy-subrc = 0.
    RETURN.
  ENDIF.
ENDIF.

IF global = abap_true.
  IF ignore_case = abap_true.
    REPLACE ALL OCCURRENCES OF REGEX posix IN result WITH p1 IGNORING CASE.
  ELSE.
    REPLACE ALL OCCURRENCES OF REGEX posix IN result WITH p1.
  ENDIF.
ELSE.
  IF ignore_case = abap_true.
    REPLACE REGEX posix IN result WITH p1 IGNORING CASE.
  ELSE.
    REPLACE REGEX posix IN result WITH p1.
  ENDIF.
ENDIF.
ENDMETHOD.
ENDCLASS.
`
}

// dynamicRuntime emits the tagged union box for Dynamic values.
func (e *emitter) dynamicRuntime() {
	id := "runtime.dynamic"
	if e.types[id] {
		return
	}
	e.types[id] = true
	name := e.name(id)
	e.files[name+".clas.abap"] = "CLASS " + name + ` DEFINITION PUBLIC CREATE PUBLIC.
PUBLIC SECTION.
CONSTANTS: tag_string TYPE i VALUE 1, tag_class TYPE i VALUE 2, tag_ref TYPE i VALUE 3.
DATA tag TYPE i.
DATA sval TYPE string.
DATA cval TYPE REF TO ` + e.name("runtime.classvalue") + `.
DATA oval TYPE REF TO object.
METHODS is_string RETURNING VALUE(result) TYPE abap_bool.
METHODS is_function RETURNING VALUE(result) TYPE abap_bool.
METHODS as_string RETURNING VALUE(result) TYPE string.
METHODS as_classvalue RETURNING VALUE(result) TYPE REF TO ` + e.name("runtime.classvalue") + `.
METHODS as_ref RETURNING VALUE(result) TYPE REF TO object.
PROTECTED SECTION.
PRIVATE SECTION.
ENDCLASS.
CLASS ` + name + ` IMPLEMENTATION.
METHOD is_string.
IF tag = tag_string.
  result = abap_true.
ENDIF.
ENDMETHOD.
METHOD is_function.
IF tag = tag_class.
  result = abap_true.
ENDIF.
ENDMETHOD.
METHOD as_string.
result = sval.
ENDMETHOD.
METHOD as_classvalue.
result = cval.
ENDMETHOD.
METHOD as_ref.
result = oval.
ENDMETHOD.
ENDCLASS.
`
}
