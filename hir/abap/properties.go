package abap

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Tables are nominal, but dispatch on the actual descriptor preserves fields
// introduced by subclasses. Constructor-initialized inherited fields are own.
func (e *emitter) propertyLookup(t hir.Type) string {
	id := "property.lookup." + t.Name
	name := e.name(id)
	if e.types[id] {
		return name
	}
	e.types[id] = true
	e.classvalueRuntime()
	e.dynamicRuntime()
	c := e.classBy(t.Name)
	type entry struct {
		key, owner string
		field      *hir.Field
		getter     *hir.Method
		tag        string
	}
	entries := []entry{}
	seen := map[string]bool{}
	for base := c; base != nil; base = e.classBy(base.Super) {
		for i := range base.Fields {
			f := &base.Fields[i]
			if f.Static || seen[f.Name] {
				continue
			}
			seen[f.Name] = true
			entries = append(entries, entry{key: f.Name, owner: base.Name, field: f})
		}
	}
	add := func(key, owner, tag string) {
		if !seen[key] {
			seen[key] = true
			entries = append(entries, entry{key: key, owner: owner, tag: tag})
		}
	}
	for base := c; base != nil; base = e.classBy(base.Super) {
		for _, m := range base.Methods {
			if m.Static {
				continue
			}
			if strings.HasPrefix(m.Name, "get.") {
				key := strings.TrimPrefix(m.Name, "get.")
				if !seen[key] {
					seen[key] = true
					entries = append(entries, entry{key: key, owner: base.Name, getter: m})
				}
			} else {
				add(m.Name, base.Name, "function")
			}
		}
	}
	add("constructor", c.Name, "constructor")
	add("__proto__", c.Name, "prototype")
	for _, key := range []string{"hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable", "toLocaleString", "toString", "valueOf", "__defineGetter__", "__defineSetter__", "__lookupGetter__", "__lookupSetter__"} {
		add(key, "Object.prototype", "function")
	}
	var s strings.Builder
	line := func(f string, args ...any) { fmt.Fprintf(&s, f+"\n", args...) }
	d := e.name("runtime.dynamic")
	line("CLASS %s DEFINITION PUBLIC CREATE PRIVATE.\nPUBLIC SECTION.", name)
	for _, m := range []string{"get", "own", "has"} {
		result := "abap_bool"
		if m == "get" {
			result = "REF TO " + d
		}
		line("CLASS-METHODS %s IMPORTING p0 TYPE REF TO %s key TYPE string RETURNING VALUE(result) TYPE %s.", m, e.name(c.Name), result)
	}
	line("PROTECTED SECTION.\nPRIVATE SECTION.\nCLASS-METHODS initialize.\nCLASS-DATA initialized TYPE abap_bool.")
	line("TYPES: BEGIN OF entry, key TYPE string, slot TYPE i, own TYPE abap_bool, END OF entry.")
	line("CLASS-DATA entries TYPE HASHED TABLE OF entry WITH UNIQUE KEY key.")
	for i, p := range entries {
		if p.field == nil && p.getter == nil {
			line("CLASS-DATA token%d TYPE REF TO %s.", i, d)
		}
	}
	line("ENDCLASS.\nCLASS %s IMPLEMENTATION.\nMETHOD initialize.\nDATA row TYPE entry.\nIF initialized = abap_true.\nRETURN.\nENDIF.\ninitialized = abap_true.", name)
	for i, p := range entries {
		own := "abap_false"
		if p.field != nil {
			own = "abap_true"
		}
		line("row-key = %s.\nrow-slot = %d.\nrow-own = %s.\nINSERT row INTO TABLE entries.", "`"+strings.ReplaceAll(p.key, "`", "``")+"`", i, own)
		if p.field == nil && p.getter == nil {
			line("CREATE OBJECT token%d.", i)
			switch p.tag {
			case "constructor":
				line("token%d->tag = %s=>tag_class.\ntoken%d->cval = %s.", i, d, i, e.descriptorOf(c.Name))
			case "function":
				line("token%d = %s=>get( ).", i, e.propertyToken(p.owner, p.key, true))
			default:
				line("token%d = %s=>get( ).", i, e.propertyToken(p.owner, p.key, false))
			}
		}
	}
	line("ENDMETHOD.")
	// Descriptor equality selects the actual class, including deeper descendants.
	var derived []*hir.Class
	var visit func(*hir.Class)
	visit = func(base *hir.Class) {
		for _, child := range e.p.Classes {
			if child.Super == base.Name {
				visit(child)
				derived = append(derived, child)
			}
		}
	}
	visit(c)
	for _, m := range []string{"get", "own", "has"} {
		line("METHOD %s.\nDATA row TYPE entry.", m)
		line("IF p0 IS NOT BOUND.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.\ninitialize( ).")
		if e.liveClasses[c.Name] {
			source := "p0->" + e.name("view.source")
			line("IF %s IS BOUND.", source)
			if m == "get" {
				line("IF %s->has( key ) = abap_false.", source)
				line("CASE key.")
				for _, key := range []string{"constructor", "__proto__", "hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable", "toLocaleString", "toString", "valueOf", "__defineGetter__", "__defineSetter__", "__lookupGetter__", "__lookupSetter__"} {
					line("WHEN `%s`.\nRAISE EXCEPTION TYPE %s EXPORTING reason = `properties.go: delete: required or prototype slot cannot retain absence`.", key, e.name("view.refusal"))
				}
				line("ENDCASE.\nENDIF.\nresult = %s->get( key ).", source)
			} else {
				line("result = %s->has( key ).", source)
				if m == "has" {
					line("IF result = abap_false.\nCASE key.")
					for _, key := range []string{"constructor", "__proto__", "hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable", "toLocaleString", "toString", "valueOf", "__defineGetter__", "__defineSetter__", "__lookupGetter__", "__lookupSetter__"} {
						line("WHEN `%s`.\nresult = abap_true.", key)
					}
					line("ENDCASE.\nENDIF.")
				}
			}
			line("RETURN.\nENDIF.")
		}

		for i, child := range derived {
			line("IF p0->%s( ) = %s.\nDATA child%d TYPE REF TO %s.\nchild%d ?= p0.", e.name("builtin.classOf"), e.descriptorOf(child.Name), i, e.name(child.Name), i)
			line("result = %s=>%s( p0 = child%d key = key ).\nRETURN.\nENDIF.", e.propertyLookup(hir.Ref(child.Name)), m, i)
		}
		if e.recordDelete {
			line("READ TABLE p0->%s WITH TABLE KEY table_line = key TRANSPORTING NO FIELDS.\nIF sy-subrc = 0.\nRETURN.\nENDIF.", e.name("record.deleted"))
		}
		line("READ TABLE entries WITH TABLE KEY key = key INTO row.\nIF sy-subrc <> 0.")
		for base := c; base != nil; base = e.classBy(base.Super) {
			if base.FieldsDropped {
				line("RAISE EXCEPTION TYPE %s EXPORTING reason = %s.", e.viewRefusal(), "`"+strings.ReplaceAll(fmt.Sprintf("node %d (%s): property lookup on a class whose fields were dropped by a lowering trap", base.ID, base.Source), "`", "``")+"`")
				break
			}
		}
		line("RETURN.\nENDIF.")
		if m == "own" {
			line("result = row-own.")
		} else if m == "has" {
			line("result = abap_true.")
		} else {
			line("CASE row-slot.")
			b := &body{e: e, c: c, locals: map[string]string{"p0": "p0"}}
			for i, p := range entries {
				line("WHEN %d.", i)
				if p.field == nil && p.getter == nil {
					line("result = token%d.", i)
					continue
				}
				b.code.Reset()
				value := "p0->" + e.member(p.key)
				boxType := hir.Type{}
				if p.getter != nil {
					boxType = p.getter.Result
					value = b.expr(&hir.Expr{Kind: hir.VirtualCall, Type: boxType, Name: p.getter.Name, X: hir.V("p0", t)})
				} else {
					boxType = p.field.Type
					if e.liveClasses[p.owner] {
						value = b.expr(&hir.Expr{Kind: hir.FieldGet, Type: boxType, Name: p.key, X: hir.V("p0", t)})
					}
				}
				if boxType.Kind == hir.I64 {
					boxType = hir.T(hir.Number)
				}
				if boxType.Kind == hir.Optional && boxType.Args[0].Kind == hir.I64 {
					boxType = hir.T(hir.Optional, hir.T(hir.Number))
				}
				b.boxDynamic("result", value, boxType)
				s.WriteString(b.code.String())
			}
			line("ENDCASE.")
		}
		line("ENDMETHOD.")
	}
	line("ENDCLASS.")
	e.files[name+".clas.abap"] = s.String()
	return name
}

// Shared prototype tokens retain identity across nominal lookup tables.
func (e *emitter) propertyToken(owner, key string, function bool) string {
	id := "property.token." + owner + "." + key
	name := e.name(id)
	if e.types[id] {
		return name
	}
	e.types[id] = true
	d := e.name("runtime.dynamic")
	var init string
	if function {
		init = fmt.Sprintf("value->tag = %s=>tag_class.\nDATA parent TYPE REF TO %s.\nDATA names TYPE %s.\nCREATE OBJECT value->cval EXPORTING name = '' emitted = '' parent = parent static_names = names.\n", d, e.name("runtime.classvalue"), e.typ(hir.T(hir.Array, hir.T(hir.String))))
	} else {
		init = fmt.Sprintf("value->tag = %s=>tag_object.\n", d)
	}
	e.files[name+".clas.abap"] = fmt.Sprintf("CLASS %s DEFINITION PUBLIC CREATE PRIVATE.\nPUBLIC SECTION.\nCLASS-METHODS get RETURNING VALUE(result) TYPE REF TO %s.\nPROTECTED SECTION.\nPRIVATE SECTION.\nCLASS-DATA value TYPE REF TO %s.\nENDCLASS.\nCLASS %s IMPLEMENTATION.\nMETHOD get.\nIF value IS NOT BOUND.\nCREATE OBJECT value.\n%sENDIF.\nresult = value.\nENDMETHOD.\nENDCLASS.\n", name, d, d, name, init)
	return name
}
