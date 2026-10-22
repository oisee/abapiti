package abap

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

func (e *emitter) structuralUnion(t hir.Type) string {
	return e.structuralUnionProjection(t, "")
}

func (e *emitter) structuralUnionProjection(t hir.Type, member string) string {
	members := e.p.StructuralUnion(t.Name)
	if len(members) == 0 {
		return ""
	}
	result := t
	if member != "" {
		var selected []hir.StructuralMember
		for _, m := range members {
			if m.Type.Name == member {
				selected = append(selected, m)
				result = m.Type
			}
		}
		if len(selected) != 1 {
			return ""
		}
		members = selected
	}
	active := member != ""
	for _, m := range members {
		active = active || e.liveClasses[m.Type.Name]
	}
	if !active {
		return ""
	}
	key := "view.union." + t.Name
	if member != "" {
		key += "/" + member
	}
	name := e.name(key)
	file := name + ".clas.abap"
	if _, ok := e.files[file]; ok {
		return name
	}
	e.files[file] = ""
	e.liveRuntime()
	d, refusal := e.name("runtime.dynamic"), e.name("view.refusal")
	b := &body{e: e, m: &hir.Method{Result: result}, locals: map[string]string{}, wholeMethod: true}
	b.line("IF p0 IS NOT BOUND.\nRAISE EXCEPTION TYPE " + refusal + " EXPORTING reason = `structural union requires a record`.\nENDIF.")
	adapter := b.rawTemp("REF TO " + e.name("view.storage"))
	b.line("IF p0->tag <> " + d + "=>tag_object.\nIF p0->tag <> " + d + "=>tag_ref.\nRAISE EXCEPTION TYPE " + refusal + " EXPORTING reason = `structural union requires object storage`.\nENDIF.\nTRY.\n" + adapter + " ?= p0->oval.\nCATCH cx_sy_move_cast_error.\nRAISE EXCEPTION TYPE " + refusal + " EXPORTING reason = `structural union refuses nominal storage`.\nENDTRY.\nIF " + adapter + "->structural( ) <> abap_true.\nRAISE EXCEPTION TYPE " + refusal + " EXPORTING reason = `structural union refuses nominal storage`.\nENDIF.\nENDIF.")
	for _, m := range members {
		valid := b.temp(hir.T(hir.Bool))
		b.line(valid + " = abap_true.")
		for _, f := range m.Fields {
			b.structuralField(valid, adapter, f)
		}
		b.line("IF " + valid + " = abap_true.")
		if member != "" {
			b.line("result = p0.\nRETURN.\nENDIF.")
		} else {
			native := b.temp(m.Type)
			b.line("IF p0->tag = " + d + "=>tag_ref.\nTRY.\n" + native + " ?= p0->oval.\nCATCH cx_sy_move_cast_error.\nENDTRY.\nIF " + native + " IS BOUND.\nresult = " + native + ".\nRETURN.\nENDIF.\nENDIF.")
			row := b.temp(m.Type)
			if e.liveClasses[m.Type.Name] {
				b.liveView(&hir.Expr{Type: m.Type}, row, "p0")
			} else {
				b.line("RAISE EXCEPTION TYPE " + refusal + " EXPORTING reason = `structural union member has no live data layout`.")
			}
			b.line("result = " + row + ".\nRETURN.\nENDIF.")
		}
	}
	b.line("RAISE EXCEPTION TYPE " + refusal + " EXPORTING reason = `value is outside structural union members`.")
	method := func(name string, b *body) string {
		return "METHOD " + name + ".\n" + b.constantDeclarations() + b.code.String() + "ENDMETHOD.\n"
	}
	definition := "CLASS " + name + " DEFINITION PUBLIC FINAL CREATE PUBLIC.\nPUBLIC SECTION.\nCLASS-METHODS decode IMPORTING p0 TYPE REF TO " + d + " RETURNING VALUE(result) TYPE " + e.typ(result) + ".\nCLASS-METHODS convert IMPORTING p0 TYPE REF TO " + d + " RETURNING VALUE(result) TYPE " + e.typ(result) + ".\n"
	implementation := ""
	if member != "" {
		guard := e.name("view.guard")
		definition += "INTERFACES " + guard + ".\nCLASS-METHODS validate IMPORTING p0 TYPE REF TO " + d + " RETURNING VALUE(result) TYPE REF TO " + d + ".\n"
		implementation += method("validate", b)
		decode := &body{e: e, m: &hir.Method{Result: result}, locals: map[string]string{}, wholeMethod: true}
		checked := decode.temp(hir.T(hir.Dynamic))
		decode.line(checked + " = validate( p0 ).")
		decode.liveView(&hir.Expr{Type: result}, "result", checked, true)
		decode.line("result->" + e.name("view.guardSlot") + " = NEW " + name + "( ).")
		implementation += method("decode", decode)
		implementation += "METHOD " + guard + "~check.\nDATA checked TYPE REF TO " + d + ".\nchecked = validate( p0 ).\nENDMETHOD.\n"
	} else {
		implementation += method("decode", b)
	}
	implementation += "METHOD convert.\nTRY.\nresult = decode( p0 ).\nCATCH " + refusal + ".\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDTRY.\nENDMETHOD.\n"
	e.files[file] = definition + "ENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\n" + implementation + "ENDCLASS.\n"

	return name
}

func (b *body) structuralField(valid, adapter string, f hir.StructuralField) {
	e := b.e
	key := "`" + strings.ReplaceAll(f.Name, "`", "``") + "`"
	d := e.name("runtime.dynamic") + "=>"
	if !f.Optional {
		b.line("IF p0->tag = " + d + "tag_object.\nREAD TABLE p0->entries WITH KEY k = " + key + " TRANSPORTING NO FIELDS.\nIF sy-subrc <> 0.\n" + valid + " = abap_false.\nENDIF.\nELSEIF " + adapter + "->has( " + key + " ) = abap_false.\n" + valid + " = abap_false.\nENDIF.")
	}
	v := b.temp(hir.T(hir.Dynamic))
	matches := b.temp(hir.T(hir.Bool))
	b.line(v + " = p0->get( " + key + " ).\n" + matches + " = abap_false.\nIF " + v + " IS NOT BOUND.")
	absent := f.Optional
	for _, domain := range f.Domains {
		absent = absent || domain.Kind == "undefined" || domain.Kind == hir.Dynamic
	}
	if absent {
		b.line(matches + " = abap_true.")
	}
	b.line("ELSE.")
	for _, domain := range f.Domains {
		condition := ""
		switch domain.Kind {
		case hir.Dynamic:
			condition = "abap_true = abap_true"
		case "null":
			condition = v + "->tag = " + d + "tag_null"
		case hir.String:
			condition = v + "->tag = " + d + "tag_string"
			if domain.Exact {
				condition += " AND " + v + "->sval = `" + strings.ReplaceAll(domain.Literal, "`", "``") + "`"
			}
		case hir.Number:
			condition = v + "->tag = " + d + "tag_number"
			if domain.Exact {
				condition += " AND " + v + "->nval = '" + domain.Literal + "'"
			}
		case hir.Bool:
			condition = v + "->tag = " + d + "tag_boolean"
			if domain.Exact {
				value := "abap_false"
				if domain.Literal == "true" {
					value = "abap_true"
				}
				condition += " AND " + v + "->bval = " + value
			}
		case hir.ClassRef:
			condition = v + "->tag = " + d + "tag_object OR " + v + "->tag = " + d + "tag_ref"
		case hir.ClassValue:
			condition = v + "->tag = " + d + "tag_class"
		case hir.Array:
			e.isArrayUsed = true
			array := b.temp(hir.T(hir.Bool))
			mark := b.rawTemp("REF TO " + e.name("runtime.arraymark"))
			b.line(array + " = abap_false.\nIF " + v + "->tag = " + d + "tag_array.\n" + array + " = abap_true.\nELSEIF " + v + "->tag = " + d + "tag_ref AND " + v + "->oval IS BOUND.\nTRY.\n" + mark + " ?= " + v + "->oval.\n" + array + " = abap_true.\nCATCH cx_sy_move_cast_error.\nENDTRY.\nENDIF.")
			condition = array + " = abap_true"
		}
		if condition != "" {
			b.line("IF " + condition + ".\n" + matches + " = abap_true.\nENDIF.")
		}
	}
	b.line("ENDIF.\nIF " + matches + " = abap_false.\n" + valid + " = abap_false.\nENDIF.")
}
