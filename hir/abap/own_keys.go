package abap

import (
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Native enumeration requires constructor/declaration order to agree.
// Structural instances require the constructor expression's OwnKeys proof.
func (e *emitter) ownAdapter(c *hir.Class) bool {
	if !e.ownKeysUsed || c == nil {
		return false
	}
	if result, ok := e.ownAdapters[c.Name]; ok {
		return result
	}
	result := c.Structural || hir.OwnDataClass(e.p, c.Name) && (e.liveClasses[c.Name] || c.Ctor != nil && len(c.Ctor.Params) == 0)
	if e.ownAdapters == nil {
		e.ownAdapters = map[string]bool{}
	}
	e.ownAdapters[c.Name] = result
	return result
}

func (e *emitter) inheritsOwnAdapter(c *hir.Class) bool {
	for parent := e.classBy(c.Super); parent != nil; parent = e.classBy(parent.Super) {
		if e.ownAdapter(parent) {
			return true
		}
	}
	return false
}

// Uninitialized declarations, conditional writes, repeated writes and custom
// constructor actions do not establish an own-property order. Keep them closed.
func (e *emitter) nativeOwnKeysProven(c *hir.Class) bool {
	for ; c != nil; c = e.classBy(c.Super) {
		var declared, assigned []string
		for _, f := range c.Fields {
			if !f.Static {
				declared = append(declared, f.Name)
			}
		}
		if c.Ctor == nil {
			if len(declared) != 0 {
				return false
			}
			continue
		}
		if len(c.Ctor.Params) != 0 {
			return false
		}
		valid := true
		superCalls := 0
		var visit func(*hir.Stmt)
		visit = func(s *hir.Stmt) {
			if s == nil {
				return
			}
			switch s.Kind {
			case hir.Block:
				for _, child := range s.List {
					visit(child)
				}
			case hir.Assign:
				if s.X.Kind != hir.FieldGet || s.X.X.Kind != hir.This {
					valid = false
					return
				}
				// The nascent instance must not escape through initializer
				// values: a call receiving this could install additional keys.
				var value func(*hir.Expr)
				value = func(x *hir.Expr) {
					if x == nil {
						return
					}
					if x.Kind == hir.This {
						valid = false
					}
					value(x.X)
					value(x.Y)
					value(x.Z)
					for _, arg := range x.Args {
						value(arg)
					}
					e.walkStmt(x.Stmt, value)
				}
				value(s.Y)
				assigned = append(assigned, s.X.Name)
			case hir.ExprStmt:
				superCalls++
				if s.X.Kind != hir.SuperCall || len(s.X.Args) != 0 || superCalls > 1 || len(assigned) > 0 {
					valid = false
				}
			default:
				valid = false
			}
		}
		visit(c.Ctor.Body)
		if !valid || strings.Join(declared, "\x00") != strings.Join(assigned, "\x00") {
			return false
		}
	}
	return true
}

func (e *emitter) ownKeysRuntime() {
	if e.types["runtime.ownKeys"] {
		return
	}
	e.types["runtime.ownKeys"] = true
	e.isArrayUsed = true
	e.liveRuntime()
	d, helper, iface := e.name("runtime.dynamic"), e.name("runtime.ownKeys"), e.name("own.storage")
	arr := e.typ(hir.T(hir.Array, hir.T(hir.String)))
	e.files[iface+".intf.abap"] = fmt.Sprintf("INTERFACE %s PUBLIC.\nMETHODS keys RETURNING VALUE(result) TYPE %s.\nENDINTERFACE.\n", iface, arr)
	e.files[helper+".clas.abap"] = strings.NewReplacer("$H", helper, "$M", e.name("runtime.arraymark"), "$D", d, "$I", iface, "$A", arr, "$S", e.name("view.storage"), "$T", e.name("view.refusal")).Replace(`CLASS $H DEFINITION PUBLIC CREATE PUBLIC.
PUBLIC SECTION.
CLASS-METHODS keys IMPORTING p0 TYPE REF TO $D RETURNING VALUE(result) TYPE $A.
PROTECTED SECTION.
PRIVATE SECTION.
ENDCLASS.
CLASS $H IMPLEMENTATION.
METHOD keys.
IF p0 IS NOT BOUND.
RAISE EXCEPTION TYPE $T EXPORTING reason = 'own_keys.go: keys: undefined receiver'.
ENDIF.
IF p0->tag = $D=>tag_ref.
DATA storage TYPE REF TO $S.
DATA source TYPE REF TO $D.
TRY.
storage ?= p0->oval.
CATCH cx_sy_move_cast_error.
ENDTRY.
IF storage IS BOUND.
source = storage->source( ).
IF source IS BOUND.
result = keys( source ).
RETURN.
ENDIF.
ENDIF.
IF p0->oval IS INSTANCE OF $M AND storage IS BOUND.
DATA array_length TYPE REF TO $D.
DATA array_index TYPE i.
DATA array_key TYPE string.
array_length = storage->get( 'length' ).
CREATE OBJECT result.
WHILE array_index < array_length->nval.
array_key = |{ array_index }|.
IF storage->has( array_key ) = abap_true.
APPEND array_key TO result->items.
ENDIF.
array_index = array_index + 1.
ENDWHILE.
RETURN.
ENDIF.
DATA native TYPE REF TO $I.
TRY.
native ?= p0->oval.
CATCH cx_sy_move_cast_error.
RAISE EXCEPTION TYPE $T EXPORTING reason = 'own_keys.go: keys: reference lacks own-key adapter'.
ENDTRY.
result = native->keys( ).
RETURN.
ENDIF.
IF p0->tag = $D=>tag_array.
CREATE OBJECT result.
DATA graph_index TYPE i.
DATA graph_key TYPE string.
DATA graph_slot TYPE i.
WHILE graph_index < lines( p0->items ).
graph_slot = graph_index + 1.
READ TABLE p0->missing WITH TABLE KEY table_line = graph_slot TRANSPORTING NO FIELDS.
IF sy-subrc <> 0.
graph_key = |{ graph_index }|.
APPEND graph_key TO result->items.
ENDIF.
graph_index = graph_index + 1.
ENDWHILE.
RETURN.
ENDIF.
IF p0->tag <> $D=>tag_object.
RAISE EXCEPTION TYPE $T EXPORTING reason = 'own_keys.go: keys: primitive key enumeration unavailable'.
ENDIF.
TYPES: BEGIN OF ordered_key, k TYPE string, category TYPE i, number TYPE int8, position TYPE i, END OF ordered_key.
DATA ordered TYPE STANDARD TABLE OF ordered_key WITH DEFAULT KEY.
DATA item TYPE ordered_key.
DATA row TYPE $D=>entry.
DATA position TYPE i.
LOOP AT p0->entries INTO row.
CLEAR item.
position = position + 1.
item-k = row-k.
item-position = position.
item-category = 1.
IF row-k IS NOT INITIAL AND strlen( row-k ) <= 10 AND row-k CO '0123456789'.
IF strlen( row-k ) = 1 OR row-k+0(1) <> '0'.
item-number = row-k.
IF item-number < 4294967295.
item-category = 0.
ELSE.
CLEAR item-number.
ENDIF.
ENDIF.
ENDIF.
APPEND item TO ordered.
ENDLOOP.
SORT ordered BY category number position.
CREATE OBJECT result.
LOOP AT ordered INTO item.
APPEND item-k TO result->items.
ENDLOOP.
ENDMETHOD.
ENDCLASS.
`)
}

func (e *emitter) ownClassDeclaration(out *strings.Builder, c *hir.Class) {
	if !e.ownAdapter(c) {
		if e.ownKeysUsed && e.inheritsOwnAdapter(c) {
			fmt.Fprintf(out, "METHODS %s~keys REDEFINITION.\nMETHODS %s~get REDEFINITION.\nMETHODS %s~put REDEFINITION.\n", e.name("own.storage"), e.name("view.storage"), e.name("view.storage"))
			fmt.Fprintf(out, "METHODS %s~has REDEFINITION.\nMETHODS %s~delete REDEFINITION.\n", e.name("view.storage"), e.name("view.storage"))
		}
		return
	}
	parent := e.classBy(c.Super)
	iface, storage := e.name("own.storage"), e.name("view.storage")
	if e.ownAdapter(parent) {
		fmt.Fprintf(out, "METHODS %s~keys REDEFINITION.\n", iface)
	} else {
		fmt.Fprintf(out, "INTERFACES %s.\n", iface)
	}
	if e.ownAdapter(parent) || parent != nil && e.liveClasses[parent.Name] {
		fmt.Fprintf(out, "METHODS %s~get REDEFINITION.\nMETHODS %s~put REDEFINITION.\nMETHODS %s~has REDEFINITION.\nMETHODS %s~delete REDEFINITION.\n", storage, storage, storage, storage)
	} else if !e.liveRoot(c) {
		fmt.Fprintf(out, "INTERFACES %s.\n", storage)
	}

}

func (e *emitter) refuseInheritedOwnAdapter(out *strings.Builder, c *hir.Class) bool {
	if !e.ownKeysUsed || e.ownAdapter(c) || !e.inheritsOwnAdapter(c) {
		return false
	}
	for _, name := range []string{e.name("own.storage") + "~keys", e.name("view.storage") + "~get", e.name("view.storage") + "~put", e.name("view.storage") + "~has", e.name("view.storage") + "~delete"} {
		fmt.Fprintf(out, "METHOD %s.\nRAISE EXCEPTION TYPE %s EXPORTING reason = `own_keys.go: inheritedAdapter: child own-slot layout unavailable`.\nENDMETHOD.\n", name, e.name("view.refusal"))
	}
	return true
}

func (e *emitter) ownClassImplementation(out *strings.Builder, c *hir.Class) {
	d, storage := e.name("runtime.dynamic"), e.name("view.storage")
	makeBody := func(name string) *body {
		return &body{e: e, c: c, m: &hir.Method{Result: hir.T(hir.Void)}, implemented: name, locals: map[string]string{}, wholeMethod: true}
	}
	emit := func(name string, b *body) {
		fmt.Fprintf(out, "METHOD %s.\n%s%sENDMETHOD.\n", name, b.constantDeclarations(), foldTempInits(b.code.String()))
	}
	parent := e.classBy(c.Super)
	if !e.ownAdapter(parent) && !(parent != nil && e.liveClasses[parent.Name]) {
		structural := ""
		if c.Structural {
			structural = "result = abap_true.\n"
		}
		source := ""
		if e.liveClasses[c.Name] {
			source = "result = me->" + e.name("view.source") + ".\n"
		}
		fmt.Fprintf(out, "METHOD %s~structural.\n%sENDMETHOD.\nMETHOD %s~source.\n%sENDMETHOD.\n", storage, structural, storage, source)
	}
	if c.Structural {
		e.nativeStructuralPresence(out, c)
	} else {
		e.nativeOwnPresence(out, c)
	}
	var fields []hir.Field
	seen := map[string]bool{}
	var visit func(*hir.Class)
	visit = func(c *hir.Class) {
		if c == nil {
			return
		}
		visit(e.classBy(c.Super))
		for _, f := range c.Fields {
			if !f.Static && !f.PrivateSlot && !seen[f.Name] {
				seen[f.Name] = true
				fields = append(fields, f)
			}
		}
	}
	visit(c)
	name := e.name("own.storage") + "~keys"
	b := makeBody(name)
	graph := b.temp(hir.T(hir.Dynamic))
	if e.liveClasses[c.Name] {
		b.line("IF me->" + e.name("view.source") + " IS BOUND.\nresult = " + e.name("runtime.ownKeys") + "=>keys( me->" + e.name("view.source") + " ).\nRETURN.\nENDIF.")
	}
	if c.Structural {
		b.line(graph + " = me->" + e.name("view.ownKeys") + ".")
		b.line("IF " + graph + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `own_keys.go: keys: structural own-key metadata absent`.\nENDIF.")
	} else if !e.nativeOwnKeysProven(e.classBy(c.Name)) {
		b.line("RAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `own_keys.go: keys: native own-key order unproven`.")
	} else {
		b.line(graph + " = NEW #( ).\n" + graph + "->tag = " + d + "=>tag_object.")
		for _, f := range fields {
			if e.recordDelete {
				b.line("READ TABLE me->" + e.name("record.deleted") + " WITH KEY table_line = `" + strings.ReplaceAll(f.Name, "`", "``") + "` TRANSPORTING NO FIELDS.\nIF sy-subrc <> 0.")
			}
			b.line(graph + "->put( p0 = `" + strings.ReplaceAll(f.Name, "`", "``") + "` p1 = VALUE #( ) ).")
			if e.recordDelete {
				b.line("ENDIF.")
			}
		}
	}
	b.line("result = " + e.name("runtime.ownKeys") + "=>keys( " + graph + " ).")
	emit(name, b)
	for _, op := range []string{"get", "put"} {
		b = makeBody(storage + "~" + op)
		if op == "get" {
			var prototypeKeys []string
			for _, key := range []string{"constructor", "toString", "toLocaleString", "valueOf", "hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable", "__proto__", "__defineGetter__", "__defineSetter__", "__lookupGetter__", "__lookupSetter__"} {
				if !seen[key] {
					prototypeKeys = append(prototypeKeys, "p0 = `"+key+"`")
				}
			}
			if len(prototypeKeys) > 0 {
				b.line("IF " + strings.Join(prototypeKeys, " OR ") + ".")
				graph := ""
				if e.liveClasses[c.Name] {
					source := "me->" + e.name("view.source")
					b.line("IF " + source + " IS BOUND.")
					b.line("IF " + source + "->tag = " + d + "=>tag_object.\nREAD TABLE " + source + "->entries WITH KEY k = p0 TRANSPORTING NO FIELDS.\nIF sy-subrc <> 0.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `own_keys.go: get: inherited prototype value unavailable from graph`.\nENDIF.\nELSEIF " + source + "->tag <> " + d + "=>tag_ref.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `own_keys.go: get: primitive prototype lookup unavailable`.\nENDIF.\nELSE.")
				}
				if c.Structural {
					graph = "me->" + e.name("view.ownKeys")
				}
				if graph == "" {
					b.line("RAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `own_keys.go: get: native prototype lookup unavailable`.")
				} else {
					b.line("IF " + graph + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `own_keys.go: get: prototype presence metadata absent`.\nENDIF.\nREAD TABLE " + graph + "->entries WITH KEY k = p0 TRANSPORTING NO FIELDS.\nIF sy-subrc <> 0.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `own_keys.go: get: inherited prototype value unavailable from native slots`.\nENDIF.")
				}
				if e.liveClasses[c.Name] {
					b.line("ENDIF.")
				}
				b.line("ENDIF.")
			}
		}
		if e.liveClasses[c.Name] {
			source := "me->" + e.name("view.source")
			b.line("IF " + source + " IS BOUND.")
			if op == "get" {
				b.line("result = " + source + "->get( p0 ).")
			} else {
				b.line(source + "->put( p0 = p0 p1 = p1 ).")
			}
			b.line("RETURN.\nENDIF.")
		}
		if !c.Structural && !e.nativeOwnKeysProven(e.classBy(c.Name)) {
			// A native default slot cannot stand in for a missing JS property.
			// Source-backed views above keep their own presence-aware adapter.
			b.line("RAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `own_keys.go: slot: native own-key presence unproven`.")
		}
		if c.Structural {
			metadata := "me->" + e.name("view.ownKeys")
			b.line("IF " + metadata + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `own_keys.go: " + op + " " + strings.ReplaceAll(c.Name, "`", "``") + ": structural own-key metadata absent`.\nENDIF.")
			if op == "get" {
				b.line("READ TABLE " + metadata + "->entries WITH KEY k = p0 TRANSPORTING NO FIELDS.\nIF sy-subrc <> 0.\nRETURN.\nENDIF.")
			}
		}
		b.line("CASE p0.")
		for _, f := range fields {
			if op == "put" && !c.Structural {
				t := f.Type
				if t.Kind == hir.Optional {
					t = t.Args[0]
				}
				if t.Kind != hir.String && t.Kind != hir.Bool && t.Kind != hir.Number && t.Kind != hir.Dynamic {
					continue
				}
			}
			b.line("WHEN `" + strings.ReplaceAll(f.Name, "`", "``") + "`.")
			if op == "get" {
				base := f.Type
				if base.Kind == hir.Optional {
					base = base.Args[0]
				}
				if !base.IsRef() && base.Kind != hir.String && base.Kind != hir.Number && base.Kind != hir.I32 && base.Kind != hir.Bool {
					// Keep unsupported scalar boxing a field-read refusal;
					// key enumeration itself does not need the field value.
					b.line("RAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `own_keys.go: get: scalar field has no exact box ABI`.")
				} else {
					b.boxDynamic("result", "me->"+e.member(f.Name), f.Type)
				}
			} else {
				b.liveUnbox("me->"+e.member(f.Name), "p1", f.Type)
				if e.recordDelete {
					b.line("DELETE me->" + e.name("record.deleted") + " WHERE table_line = `" + strings.ReplaceAll(f.Name, "`", "``") + "`.")
				}
			}
		}
		if c.Structural {
			extras := "me->" + e.name("view.extras")
			if op == "get" {
				b.line("WHEN OTHERS.\nIF " + extras + " IS BOUND.\nresult = " + extras + "->get( p0 ).\nENDIF.")
			} else {
				b.line("WHEN OTHERS.\nIF p0 = `__proto__`.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `own_keys.go: put: prototype mutation unavailable`.\nENDIF.\nIF " + extras + " IS NOT BOUND.\n" + extras + " = NEW #( ).\n" + extras + "->tag = " + d + "=>tag_object.\nENDIF.\n" + extras + "->put( p0 = p0 p1 = p1 ).")
			}
		} else if op == "put" {
			b.line("WHEN OTHERS.\nRAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `own_keys.go: put: unknown nominal slot has no storage`.")
		}
		b.line("ENDCASE.")
		if op == "put" && c.Structural {
			b.line("IF me->" + e.name("view.ownKeys") + " IS BOUND.\nme->" + e.name("view.ownKeys") + "->put( p0 = p0 p1 = VALUE #( ) ).\nENDIF.")
		}
		if op == "put" && e.jsonOwnKeys && c.Structural && c.Super == "" {
			b.jsonRecordKey("me", "p0")
		}
		emit(storage+"~"+op, b)
	}
}

// Nominal own-key adapters share the live graph's presence/deletion ABI.
func (e *emitter) nativeOwnPresence(out *strings.Builder, c *hir.Class) {
	storage, trap := e.name("view.storage"), e.name("view.refusal")
	for _, op := range []string{"has", "delete"} {
		fmt.Fprintf(out, "METHOD %s~%s.\n", storage, op)
		if e.liveClasses[c.Name] {
			fmt.Fprintf(out, "IF me->%s IS BOUND.\nresult = me->%s->%s( p0 ).\nRETURN.\nENDIF.\n", e.name("view.source"), e.name("view.source"), op)
		}
		if op == "delete" && e.recordDelete {
			fmt.Fprintf(out, "result = %s=>run( p0 = me key = p0 ).\n", e.nativeRecordDeleteRuntime())
		} else if op == "has" && e.nativeOwnKeysProven(c) {
			out.WriteString("CASE p0.\n")
			for base := c; base != nil; base = e.classBy(base.Super) {
				for _, f := range base.Fields {
					if !f.Static && !f.PrivateSlot {
						fmt.Fprintf(out, "WHEN `%s`.\nresult = abap_true.\n", strings.ReplaceAll(f.Name, "`", "``"))
					}
				}
			}
			out.WriteString("ENDCASE.\n")
			if e.recordDelete {
				fmt.Fprintf(out, "READ TABLE me->%s WITH KEY table_line = p0 TRANSPORTING NO FIELDS.\nIF sy-subrc = 0.\nresult = abap_false.\nENDIF.\n", e.name("record.deleted"))
			}
		} else {
			fmt.Fprintf(out, "RAISE EXCEPTION TYPE %s EXPORTING reason = `own_keys.go: presence: native presence or deletion ABI unproven`.\n", trap)
		}
		out.WriteString("ENDMETHOD.\n")
	}
}
