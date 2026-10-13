package abap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Chai has its own observation boundary. Never snapshot native references or
// infer enumerable JS keys from ABAP implementation slots.
func (b *body) chaiBox(n, a string, t hir.Type) {
	if t.Kind == hir.Optional {
		base := t.Args[0]
		test := a + " IS BOUND"
		if !base.IsRef() {
			test += " AND " + a + "->has = abap_true"
		}
		b.line("CLEAR " + n + ".\nIF " + test + ".")
		if !base.IsRef() {
			a += "->value"
		}
		b.chaiBox(n, a, base)
		b.line("ENDIF.")
		return
	}
	if t.IsRef() && t.Kind != hir.ClassValue {
		b.line(n + " = " + b.e.name("runtime.chai") + "=>box_ref( " + a + " ).")
		return
	}
	if !t.IsRef() && t.Kind != hir.String && t.Kind != hir.Number && t.Kind != hir.I32 && t.Kind != hir.Bool {
		b.line("RAISE EXCEPTION TYPE " + b.e.name("exception.chaiRefusal") + ".")
		return
	}
	b.boxDynamic(n, a, t)
	if t.Kind == hir.I32 {
		b.line(n + "->chai_zero_sign = 1.")
	}
}

// A statically certified thunk needs no erased-object dispatch. Discard its
// return value, but drain settled rejection exactly as the Go backend does.
func (b *body) chaiThrowTyped(n, a string, t hir.Type, expected string) {
	var call *hir.Method
	for _, iface := range b.e.p.Interfaces {
		if iface.Name == t.Name {
			for _, m := range iface.Methods {
				if m.Name == "call" {
					call = m
				}
			}
		}
	}
	if call == nil {
		b.line("RAISE EXCEPTION TYPE " + b.e.name("exception.chaiRefusal") + ".")
		return
	}
	error := b.rawTemp("REF TO cx_root")
	b.line("TRY.")
	invoke := a + "->" + b.e.member("call") + "( )"
	if call.Result.Kind == hir.Void {
		b.line(invoke + ".")
	} else {
		value := b.temp(call.Result)
		b.line(value + " = " + invoke + ".")
		if call.Result.Kind == hir.Promise {
			b.line("IF " + value + " IS NOT BOUND.\nRAISE EXCEPTION TYPE " + b.e.name("exception.chaiRefusal") + ".\nENDIF.\nIF " + value + "->rejection IS BOUND.\nRAISE EXCEPTION " + value + "->rejection.\nENDIF.")
		}
	}
	b.line("CATCH " + b.e.chaiCatchTypes() + " INTO " + error + ".")
	b.line(n + " = " + b.e.name("runtime.chai") + "=>match_error( p0 = " + error + " p1 = " + expected + " ).\nENDTRY.")
}

// Catch translated payloads only. Native faults and coverage refusals propagate
// without passing through the Chai message adapter.
func (e *emitter) chaiCatchTypes() string {
	if e.promiseUsed {
		return e.promiseExceptionBase()
	}
	types := []string{e.exception(hir.T(hir.String)), e.exception(hir.T(hir.Dynamic))}
	if e.errorUsed {
		types = append(types, e.exception(hir.Ref("builtin.Error")))
	}
	for _, c := range e.p.Classes {
		if c.Name == "builtin.Error" {
			continue
		}
		for parent := c.Super; parent != ""; parent = e.superOf(parent) {
			if parent == "builtin.Error" {
				types = append(types, e.exception(hir.Ref(c.Name)))
				break
			}
		}
	}
	return strings.Join(types, " ")
}

func (e *emitter) chaiRuntime() {
	d, name, trap := e.name("runtime.dynamic"), e.name("runtime.chai"), e.name("exception.chaiRefusal")
	file := d + ".clas.abap"
	e.files[file] = strings.Replace(e.files[file], "DATA nval TYPE f.", "DATA nval TYPE f.\nDATA chai_zero_sign TYPE i.", 1)
	jsonFile := e.name("runtime.jsonSubset") + ".clas.abap"
	if src, ok := e.files[jsonFile]; ok {
		e.files[jsonFile] = strings.Replace(src, "result->nval = token.", "result->nval = token.\nIF result->nval = 0.\nresult->chai_zero_sign = 1.\nIF token+0(1) = '-'.\nresult->chai_zero_sign = 2.\nENDIF.\nENDIF.", 1)
	}
	e.files[trap+".clas.abap"] = "CLASS " + trap + " DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.\nPUBLIC SECTION.\nPROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + trap + " IMPLEMENTATION.\nENDCLASS.\n"
	// Adapter methods are emitted after the program and ordinary runtime types.
	// Reading the original arrays on each invocation preserves aliases/mutations.
	var arrays, kind, entries, box, action strings.Builder
	arraySerial, entrySerial := 0, 0
	sort.Slice(e.chaiArrays, func(i, j int) bool { return e.chaiArrays[i].String() < e.chaiArrays[j].String() })
	for i, t := range e.chaiArrays {
		typ := e.name("runtime." + t.String())
		fmt.Fprintf(&kind, "IF p0->oval IS INSTANCE OF %s.\nresult = $D=>tag_array.\nRETURN.\nENDIF.\n", typ)
		v := fmt.Sprintf("a%d", i)
		fmt.Fprintf(&arrays, "IF p0->oval IS INSTANCE OF %s.\nDATA %s TYPE REF TO %s.\n%s ?= p0->oval.\n", typ, v, typ, v)
		b := &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}, serial: arraySerial}
		row := b.rawTemp(e.typ(t.Args[0]))
		val := b.temp(hir.T(hir.Dynamic))
		b.line("LOOP AT " + v + "->items INTO " + row + ".")
		b.chaiBox(val, row, t.Args[0])
		b.line("APPEND " + val + " TO result.\nENDLOOP.\nRETURN.")
		if e.jsRegexpUsed && t.Args[0].Kind == hir.String {
			fmt.Fprintf(&arrays, "IF %s->match_missing IS NOT INITIAL.\nRAISE EXCEPTION TYPE $T.\nENDIF.\n", v)
		}
		arrays.WriteString(b.code.String())
		arraySerial = b.serial
		arrays.WriteString("ENDIF.\n")
	}
	sort.Slice(e.chaiMaps, func(i, j int) bool { return e.chaiMaps[i].String() < e.chaiMaps[j].String() })
	for i, t := range e.chaiMaps {
		file := e.name("runtime."+t.String()) + ".clas.abap"
		e.files[file] = strings.Replace(e.files[file], "DATA entries TYPE entries_type.", "DATA entries TYPE entries_type.\nDATA chai_enumerable TYPE abap_bool.", 1)
		if t.Args[0].Kind != hir.String {
			continue
		}
		v := fmt.Sprintf("record%d", i)
		typ := e.name("runtime." + t.String())
		fmt.Fprintf(&entries, "IF p0->oval IS INSTANCE OF %s.\nDATA %s TYPE REF TO %s.\n%s ?= p0->oval.\nIF %s->chai_enumerable <> abap_true.\nRAISE EXCEPTION TYPE $T.\nENDIF.\n", typ, v, typ, v, v)
		b := &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}, serial: entrySerial}
		row := b.rawTemp(typ + "=>entry")
		val := b.temp(hir.T(hir.Dynamic))
		b.line("LOOP AT " + v + "->entries INTO " + row + ".")
		b.chaiBox(val, row+"-v", t.Args[1])
		b.line("APPEND VALUE #( k = " + row + "-k v = " + val + " ) TO result.\nENDLOOP.\nRETURN.\nENDIF.")
		entries.WriteString(b.code.String())
		entrySerial = b.serial
	}
	// Match Go's data-shape certification, not nominal class field reflection.
	for i, c := range e.p.Classes {
		proven := c.Super == "" && !c.Abstract && len(c.Methods) == 0 && c.Ctor != nil && len(c.Ctor.Params) == len(c.Fields)
		for _, f := range c.Fields {
			proven = proven && !f.Static && f.Type.Kind != hir.Optional
		}
		if !proven {
			continue
		}
		v := fmt.Sprintf("c%d", i)
		fmt.Fprintf(&entries, "IF p0->oval IS INSTANCE OF %s.\nDATA %s TYPE REF TO %s.\n%s ?= p0->oval.\n", e.name(c.Name), v, e.name(c.Name), v)
		// A subclass is not the certified shape, even if its base is one.
		for _, sub := range e.p.Classes {
			if sub.Super == c.Name {
				fmt.Fprintf(&entries, "IF p0->oval IS INSTANCE OF %s.\nRAISE EXCEPTION TYPE $T.\nENDIF.\n", e.name(sub.Name))
			}
		}
		b := &body{e: e, m: &hir.Method{Result: hir.T(hir.Void)}, locals: map[string]string{}, serial: entrySerial}
		for _, f := range c.Fields {
			val := b.temp(hir.T(hir.Dynamic))
			b.chaiBox(val, v+"->"+e.member(f.Name), f.Type)
			b.line("APPEND VALUE #( k = `" + strings.ReplaceAll(f.Name, "`", "``") + "` v = " + val + " ) TO result.")
		}
		entries.WriteString(b.code.String())
		entrySerial = b.serial
		entries.WriteString("RETURN.\nENDIF.\n")
	}
	for _, c := range e.p.Classes {
		if e.materialized[c.Name] {
			fmt.Fprintf(&box, "IF p0 IS INSTANCE OF %s.\nDATA snapshot_%d TYPE REF TO %s.\nsnapshot_%d ?= p0.\nIF snapshot_%d->%s IS BOUND.\nRAISE EXCEPTION TYPE $T.\nENDIF.\nENDIF.\n", e.name(c.Name), len(box.String()), e.name(c.Name), len(box.String()), len(box.String()), e.name("builtin.materializedSource"))
		}
	}
	if len(e.liveClasses) > 0 {
		box.WriteString("DATA adapter TYPE REF TO " + e.name("view.storage") + ".\nTRY.\nadapter ?= p0.\nCATCH cx_sy_move_cast_error.\nENDTRY.\nIF adapter IS BOUND AND adapter->source( ) IS BOUND.\nRAISE EXCEPTION TYPE $T.\nENDIF.\n")
	}
	for i, c := range e.p.Classes {
		if !strings.HasPrefix(c.Name, "closure.") {
			continue
		}
		callable, thunk := false, false
		for _, s := range c.Implements {
			thunk = thunk || s == "closure.thunk"
			callable = callable || s == "closure.thunk" || strings.HasPrefix(s, "closure.function.")
		}
		m, _ := e.method(c, "call")
		if !callable || m == nil || len(m.Params) != 0 || m.Result.Kind == hir.Promise && !thunk {
			continue
		}
		v := fmt.Sprintf("action%d", i)
		fmt.Fprintf(&action, "IF p0->oval IS INSTANCE OF %s.\nDATA %s TYPE REF TO %s.\n%s ?= p0->oval.\n", e.name(c.Name), v, e.name(c.Name), v)
		call := v + "->" + e.member("call") + "( )"
		if m.Result.Kind == hir.Void {
			action.WriteString(call + ".\n")
		} else {
			fmt.Fprintf(&action, "DATA value%d TYPE %s.\nvalue%d = %s.\n", i, e.typ(m.Result), i, call)
			if m.Result.Kind == hir.Promise {
				fmt.Fprintf(&action, "IF value%d IS NOT BOUND.\nRAISE EXCEPTION TYPE $T.\nENDIF.\nIF value%d->rejection IS BOUND.\nRAISE EXCEPTION value%d->rejection.\nENDIF.\n", i, i, i)
			}
		}
		action.WriteString("RETURN.\nENDIF.\n")
	}
	// Only translated payload exceptions are observed, never native faults/traps.
	var catches strings.Builder
	stringException := e.exception(hir.T(hir.String))
	fmt.Fprintf(&catches, "IF error IS INSTANCE OF %s.\nDATA text_error TYPE REF TO %s.\ntext_error ?= error.\nmessage = text_error->payload.\nELSE.\n", stringException, stringException)
	if e.promiseUsed {
		fmt.Fprintf(&catches, "IF error IS INSTANCE OF %s.\nDATA source_error TYPE REF TO %s.\nsource_error ?= error.\nmessage = throw_message( source_error->source_value ).\nELSE.\nRAISE EXCEPTION error.\nENDIF.\n", e.promiseExceptionBase(), e.promiseExceptionBase())
	} else {
		dynamicException := e.exception(hir.T(hir.Dynamic))
		fmt.Fprintf(&catches, "IF error IS INSTANCE OF %s.\nDATA dynamic_error TYPE REF TO %s.\ndynamic_error ?= error.\nmessage = throw_message( dynamic_error->payload ).\nELSE.\n", dynamicException, dynamicException)
		var nativeTypes []string
		if e.errorUsed {
			nativeTypes = append(nativeTypes, "builtin.Error")
		}
		for _, c := range e.p.Classes {
			if c.Name == "builtin.Error" {
				continue
			}
			for parent := c.Super; parent != ""; parent = e.superOf(parent) {
				if parent == "builtin.Error" {
					file := e.name("exception."+hir.Ref(c.Name).String()) + ".clas.abap"
					if _, exists := e.files[file]; exists {
						nativeTypes = append(nativeTypes, c.Name)
					}
					break
				}
			}
		}
		for i, typ := range nativeTypes {
			errorException := e.exception(hir.Ref(typ))
			v := fmt.Sprintf("native_error%d", i)
			fmt.Fprintf(&catches, "IF error IS INSTANCE OF %s.\nDATA %s TYPE REF TO %s.\n%s ?= error.\nIF %s->payload IS NOT BOUND.\nRAISE EXCEPTION TYPE $T.\nENDIF.\nmessage = %s->payload->%s.\nELSE.\n", errorException, v, errorException, v, v, v, e.member("message"))
		}
		catches.WriteString("RAISE EXCEPTION error.\n")
		for range nativeTypes {
			catches.WriteString("ENDIF.\n")
		}
		catches.WriteString("ENDIF.\n")
	}
	catches.WriteString("ENDIF.\n")
	errorMessage := "RAISE EXCEPTION TYPE $T.\n"
	if e.errorUsed {
		errorMessage = "IF p0->tag = $D=>tag_ref AND p0->oval IS INSTANCE OF " + e.name("builtin.Error") + ".\nDATA native TYPE REF TO " + e.name("builtin.Error") + ".\nnative ?= p0->oval.\nresult = native->" + e.member("message") + ".\nRETURN.\nENDIF.\n" + errorMessage
	}
	source := strings.NewReplacer("$ARRAYS", arrays.String(), "$KIND", kind.String(), "$ENTRIES", entries.String(), "$BOX", box.String(), "$ACTION", action.String(), "$CATCH_TYPES", e.chaiCatchTypes(), "$CATCH", catches.String(), "$MESSAGE", errorMessage).Replace(chaiRuntimeSource)
	source = strings.NewReplacer("$D", d, "$N", name, "$T", trap).Replace(source)
	e.files[name+".clas.abap"] = source
}

// Only integral boxes, source literals and parsed JSON certify zero's sign.
// Unproven zero/zero refuses rather than confusing SameValue with strict equality.
const chaiRuntimeSource = `CLASS $N DEFINITION PUBLIC CREATE PUBLIC.
PUBLIC SECTION.
CLASS-METHODS box_ref IMPORTING p0 TYPE REF TO object RETURNING VALUE(result) TYPE REF TO $D.
CLASS-METHODS deepEqual IMPORTING p0 TYPE REF TO $D p1 TYPE REF TO $D RETURNING VALUE(result) TYPE abap_bool.
CLASS-METHODS deepInclude IMPORTING p0 TYPE REF TO $D p1 TYPE REF TO $D RETURNING VALUE(result) TYPE i.
CLASS-METHODS arrayInclude IMPORTING p0 TYPE REF TO $D p1 TYPE REF TO $D RETURNING VALUE(result) TYPE abap_bool.
CLASS-METHODS members IMPORTING p0 TYPE REF TO $D p1 TYPE REF TO $D p2 TYPE abap_bool RETURNING VALUE(result) TYPE abap_bool.
CLASS-METHODS throwMatch IMPORTING p0 TYPE REF TO $D p1 TYPE string RETURNING VALUE(result) TYPE abap_bool.
CLASS-METHODS match_error IMPORTING p0 TYPE REF TO cx_root p1 TYPE string RETURNING VALUE(result) TYPE abap_bool.
PROTECTED SECTION.
PRIVATE SECTION.
TYPES: BEGIN OF pair, a TYPE REF TO object, b TYPE REF TO object, END OF pair.
TYPES pairs TYPE STANDARD TABLE OF pair WITH DEFAULT KEY.
CLASS-METHODS kind IMPORTING p0 TYPE REF TO $D RETURNING VALUE(result) TYPE i.
CLASS-METHODS identity IMPORTING p0 TYPE REF TO $D RETURNING VALUE(result) TYPE REF TO object.
CLASS-METHODS equal IMPORTING p0 TYPE REF TO $D p1 TYPE REF TO $D CHANGING seen TYPE pairs RETURNING VALUE(result) TYPE abap_bool.
CLASS-METHODS strict IMPORTING p0 TYPE REF TO $D p1 TYPE REF TO $D RETURNING VALUE(result) TYPE abap_bool.
CLASS-METHODS array_items IMPORTING p0 TYPE REF TO $D RETURNING VALUE(result) TYPE $D=>items_type.
CLASS-METHODS entries IMPORTING p0 TYPE REF TO $D RETURNING VALUE(result) TYPE $D=>entries_type.
CLASS-METHODS invoke IMPORTING p0 TYPE REF TO $D.
CLASS-METHODS throw_message IMPORTING p0 TYPE REF TO $D RETURNING VALUE(result) TYPE string.
ENDCLASS.
CLASS $N IMPLEMENTATION.
METHOD box_ref.
IF p0 IS NOT BOUND.
RETURN.
ENDIF.
IF p0 IS INSTANCE OF $D.
result ?= p0.
RETURN.
ENDIF.
$BOX
CREATE OBJECT result.
result->tag = $D=>tag_ref.
result->oval = p0.
ENDMETHOD.
METHOD kind.
IF p0 IS NOT BOUND.
RETURN.
ENDIF.
result = p0->tag.
IF result = $D=>tag_ref.
$KIND
result = $D=>tag_object.
ENDIF.
ENDMETHOD.
METHOD identity.
IF p0 IS BOUND.
IF p0->tag = $D=>tag_ref.
result = p0->oval.
ELSE.
result = p0.
ENDIF.
ENDIF.
ENDMETHOD.
METHOD strict.
IF p0 IS NOT BOUND.
result = xsdbool( p1 IS NOT BOUND ).
ELSE.
result = p0->strict_equals( p1 ).
ENDIF.
ENDMETHOD.
METHOD array_items.
IF p0 IS NOT BOUND.
RAISE EXCEPTION TYPE $T.
ENDIF.
IF p0->tag = $D=>tag_array.
result = p0->items.
RETURN.
ENDIF.
IF p0->tag = $D=>tag_ref.
$ARRAYS
ENDIF.
RAISE EXCEPTION TYPE $T.
ENDMETHOD.
METHOD entries.
IF p0 IS NOT BOUND.
RAISE EXCEPTION TYPE $T.
ENDIF.
IF p0->tag = $D=>tag_object.
result = p0->entries.
RETURN.
ENDIF.
IF p0->tag = $D=>tag_ref.
$ENTRIES
ENDIF.
RAISE EXCEPTION TYPE $T.
ENDMETHOD.
METHOD deepEqual.
DATA seen TYPE pairs.
result = equal( EXPORTING p0 = p0 p1 = p1 CHANGING seen = seen ).
ENDMETHOD.
METHOD equal.
DATA ak TYPE i.
DATA bk TYPE i.
ak = kind( p0 ).
bk = kind( p1 ).
IF ak <> bk.
RETURN.
ENDIF.
CASE ak.
WHEN 0 OR $D=>tag_null.
result = abap_true.
WHEN $D=>tag_number.
IF p0->nval = 0 AND p1->nval = 0.
IF p0->chai_zero_sign = 0 OR p1->chai_zero_sign = 0.
RAISE EXCEPTION TYPE $T.
ENDIF.
result = xsdbool( p0->chai_zero_sign = p1->chai_zero_sign ).
RETURN.
ENDIF.
result = xsdbool( p0->nval = p1->nval ).
WHEN $D=>tag_string OR $D=>tag_boolean OR $D=>tag_class.
result = strict( p0 = p0 p1 = p1 ).
WHEN $D=>tag_object OR $D=>tag_array.
IF strict( p0 = p0 p1 = p1 ) = abap_true.
result = abap_true.
RETURN.
ENDIF.
DATA a TYPE REF TO object.
DATA b TYPE REF TO object.
a = identity( p0 ).
b = identity( p1 ).
READ TABLE seen WITH KEY a = a b = b TRANSPORTING NO FIELDS.
IF sy-subrc = 0.
result = abap_true.
RETURN.
ENDIF.
APPEND VALUE #( a = a b = b ) TO seen.
IF ak = $D=>tag_array.
DATA x TYPE $D=>items_type.
DATA y TYPE $D=>items_type.
x = array_items( p0 ).
y = array_items( p1 ).
IF lines( x ) <> lines( y ).
RETURN.
ENDIF.
DATA v TYPE REF TO $D.
DATA w TYPE REF TO $D.
DATA idx TYPE i.
LOOP AT x INTO v.
idx = sy-tabix.
READ TABLE y INDEX idx INTO w.
IF equal( EXPORTING p0 = v p1 = w CHANGING seen = seen ) = abap_false.
RETURN.
ENDIF.
ENDLOOP.
ELSE.
DATA props TYPE $D=>entries_type.
DATA other TYPE $D=>entries_type.
DATA prop TYPE $D=>entry.
DATA found TYPE $D=>entry.
props = entries( p0 ).
other = entries( p1 ).
IF lines( props ) <> lines( other ).
RETURN.
ENDIF.
LOOP AT props INTO prop.
READ TABLE other WITH KEY k = prop-k INTO found.
IF sy-subrc <> 0.
RETURN.
ENDIF.
IF equal( EXPORTING p0 = prop-v p1 = found-v CHANGING seen = seen ) = abap_false.
RETURN.
ENDIF.
ENDLOOP.
ENDIF.
result = abap_true.
WHEN OTHERS.
RAISE EXCEPTION TYPE $T.
ENDCASE.
ENDMETHOD.
METHOD arrayInclude.
IF kind( p0 ) <> $D=>tag_array.
RAISE EXCEPTION TYPE $T.
ENDIF.
DATA items TYPE $D=>items_type.
DATA v TYPE REF TO $D.
items = array_items( p0 ).
LOOP AT items INTO v.
IF strict( p0 = v p1 = p1 ) = abap_true.
result = abap_true.
RETURN.
ENDIF.
ENDLOOP.
ENDMETHOD.
METHOD members.
IF kind( p0 ) <> $D=>tag_array OR kind( p1 ) <> $D=>tag_array.
RAISE EXCEPTION TYPE $T.
ENDIF.
DATA x TYPE $D=>items_type.
DATA y TYPE $D=>items_type.
DATA v TYPE REF TO $D.
DATA w TYPE REF TO $D.
DATA idx TYPE i.
DATA used TYPE STANDARD TABLE OF i WITH DEFAULT KEY.
DATA matched TYPE abap_bool.
DATA same TYPE abap_bool.
x = array_items( p0 ).
y = array_items( p1 ).
IF lines( x ) <> lines( y ).
RETURN.
ENDIF.
LOOP AT y INTO w.
matched = abap_false.
LOOP AT x INTO v.
idx = sy-tabix.
READ TABLE used WITH KEY table_line = idx TRANSPORTING NO FIELDS.
IF sy-subrc = 0.
CONTINUE.
ENDIF.
IF p2 = abap_true.
same = deepEqual( p0 = v p1 = w ).
ELSE.
same = strict( p0 = v p1 = w ).
ENDIF.
IF same = abap_true.
APPEND idx TO used.
matched = abap_true.
EXIT.
ENDIF.
ENDLOOP.
IF matched = abap_false.
RETURN.
ENDIF.
ENDLOOP.
result = abap_true.
ENDMETHOD.
METHOD deepInclude.
DATA v TYPE REF TO $D.
CASE kind( p0 ).
WHEN $D=>tag_string.
IF kind( p1 ) <> $D=>tag_string.
RAISE EXCEPTION TYPE $T.
ENDIF.
IF p1->sval IS INITIAL.
result = 1.
RETURN.
ENDIF.
FIND FIRST OCCURRENCE OF p1->sval IN p0->sval.
IF sy-subrc = 0.
result = 1.
ENDIF.
WHEN $D=>tag_array.
DATA items TYPE $D=>items_type.
items = array_items( p0 ).
LOOP AT items INTO v.
IF deepEqual( p0 = v p1 = p1 ) = abap_true.
result = 1.
RETURN.
ENDIF.
ENDLOOP.
WHEN $D=>tag_object.
DATA props TYPE $D=>entries_type.
DATA actual TYPE $D=>entries_type.
DATA prop TYPE $D=>entry.
DATA found TYPE $D=>entry.
props = entries( p1 ).
IF props IS INITIAL.
result = 2.
RETURN.
ENDIF.
actual = entries( p0 ).
LOOP AT props INTO prop.
READ TABLE actual WITH KEY k = prop-k INTO found.
IF sy-subrc <> 0.
CASE prop-k.
WHEN 'constructor' OR 'toString' OR 'valueOf' OR 'hasOwnProperty' OR 'isPrototypeOf' OR 'propertyIsEnumerable' OR 'toLocaleString' OR '__proto__' OR '__defineGetter__' OR '__defineSetter__' OR '__lookupGetter__' OR '__lookupSetter__'.
RAISE EXCEPTION TYPE $T.
ENDCASE.
RETURN.
ENDIF.
IF deepEqual( p0 = found-v p1 = prop-v ) = abap_false.
RETURN.
ENDIF.
ENDLOOP.
result = 1.
WHEN OTHERS.
RAISE EXCEPTION TYPE $T.
ENDCASE.
ENDMETHOD.
METHOD invoke.
IF p0 IS NOT BOUND OR p0->tag <> $D=>tag_ref.
RAISE EXCEPTION TYPE $T.
ENDIF.
$ACTION
RAISE EXCEPTION TYPE $T.
ENDMETHOD.
METHOD throw_message.
IF p0 IS NOT BOUND.
RAISE EXCEPTION TYPE $T.
ENDIF.
IF p0->tag = $D=>tag_string.
result = p0->sval.
RETURN.
ENDIF.
$MESSAGE
ENDMETHOD.
METHOD throwMatch.
TRY.
invoke( p0 ).
CATCH $CATCH_TYPES INTO DATA(error).
result = match_error( p0 = error p1 = p1 ).
ENDTRY.
ENDMETHOD.
METHOD match_error.
DATA error TYPE REF TO cx_root.
DATA message TYPE string.
error = p0.
$CATCH
IF p1 IS INITIAL.
result = abap_true.
RETURN.
ENDIF.
FIND FIRST OCCURRENCE OF p1 IN message.
result = xsdbool( sy-subrc = 0 ).
ENDMETHOD.
ENDCLASS.
`
