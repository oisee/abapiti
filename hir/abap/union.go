package abap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Structural record families (hir.UnionDecl) use the Go ABI: a union
// reference is the boxed original record, REF TO runtime.dynamic, and every
// operation re-validates the live record. Payloads keep their structural
// classes; each payload class checks its own layout once, and one class per
// family dispatches on the actual tag. Programs without families never reach
// this file, so their output is unchanged.

// unionStorage stores union references as their dynamic envelope, so the
// containers of all families share one runtime class per shape.
func unionStorage(t hir.Type) hir.Type {
	if t.Kind == hir.UnionRef {
		return hir.T(hir.Dynamic)
	}
	if !mentionsUnion(t) {
		return t
	}
	args := make([]hir.Type, len(t.Args))
	for i, a := range t.Args {
		args[i] = unionStorage(a)
	}
	t.Args = args
	return t
}

func mentionsUnion(t hir.Type) bool {
	if t.Kind == hir.UnionRef {
		return true
	}
	for _, a := range t.Args {
		if mentionsUnion(a) {
			return true
		}
	}
	return false
}

func (e *emitter) unionPayload(c *hir.Class) bool {
	if len(e.p.Unions) == 0 {
		return false
	}
	if e.unionPayloads == nil {
		e.unionPayloads = map[string]bool{}
		for _, u := range e.p.Unions {
			for _, a := range u.Alternatives {
				e.unionPayloads[a.Payload.Name] = true
			}
		}
	}
	return e.unionPayloads[c.Name]
}

// unionRuntime holds the shared record helpers; runtime.dynamic is unchanged.
func (e *emitter) unionRuntime() string {
	name := e.name("union.runtime")
	file := name + ".clas.abap"
	if _, ok := e.files[file]; ok {
		return name
	}
	e.liveRuntime()
	d, storage := e.name("runtime.dynamic"), e.name("view.storage")
	array := false
	for _, c := range e.p.Classes {
		if !e.unionPayload(c) {
			continue
		}
		for _, f := range c.Fields {
			if t := unionFieldType(f.Type); !f.Static && t.Kind == hir.Array {
				array = true
			}
		}
	}
	var def, impl strings.Builder
	def.WriteString("CLASS " + name + " DEFINITION PUBLIC FINAL CREATE PUBLIC.\nPUBLIC SECTION.\n")
	def.WriteString("CLASS-METHODS box IMPORTING p0 TYPE REF TO object RETURNING VALUE(result) TYPE REF TO " + d + ".\n")
	def.WriteString("CLASS-METHODS record IMPORTING p0 TYPE REF TO " + d + " RETURNING VALUE(result) TYPE abap_bool.\n")
	def.WriteString("CLASS-METHODS has IMPORTING p0 TYPE REF TO " + d + " p1 TYPE string RETURNING VALUE(result) TYPE abap_bool.\n")
	def.WriteString("CLASS-METHODS literal IMPORTING p0 TYPE REF TO " + d + " p1 TYPE string p2 TYPE string RETURNING VALUE(result) TYPE abap_bool.\n")
	impl.WriteString("METHOD box.\nDATA adapter TYPE REF TO " + storage + ".\nIF p0 IS NOT BOUND.\nRETURN.\nENDIF.\nTRY.\nresult ?= p0.\nCATCH cx_sy_move_cast_error.\nENDTRY.\nIF result IS BOUND.\nRETURN.\nENDIF.\n")
	impl.WriteString("TRY.\nadapter ?= p0.\nCATCH cx_sy_move_cast_error.\nENDTRY.\nIF adapter IS BOUND.\nresult = adapter->source( ).\nENDIF.\nIF result IS NOT BOUND.\nCREATE OBJECT result.\nresult->tag = " + d + "=>tag_ref.\nresult->oval = p0.\nENDIF.\nENDMETHOD.\n")
	impl.WriteString("METHOD record.\nDATA adapter TYPE REF TO " + storage + ".\nIF p0 IS NOT BOUND.\nRETURN.\nENDIF.\nIF p0->tag = " + d + "=>tag_object.\nresult = abap_true.\nRETURN.\nENDIF.\nIF p0->tag <> " + d + "=>tag_ref.\nRETURN.\nENDIF.\n")
	impl.WriteString("TRY.\nadapter ?= p0->oval.\nCATCH cx_sy_move_cast_error.\nRETURN.\nENDTRY.\nresult = adapter->structural( ).\nENDMETHOD.\n")
	impl.WriteString("METHOD has.\nDATA adapter TYPE REF TO " + storage + ".\nIF p0->tag = " + d + "=>tag_object.\nREAD TABLE p0->entries WITH KEY k = p1 TRANSPORTING NO FIELDS.\nresult = xsdbool( sy-subrc = 0 ).\nRETURN.\nENDIF.\nadapter ?= p0->oval.\nresult = adapter->has( p1 ).\nENDMETHOD.\n")
	impl.WriteString("METHOD literal.\nDATA v TYPE REF TO " + d + ".\nv = p0->get( p1 ).\nIF v IS BOUND AND v->tag = " + d + "=>tag_string AND v->sval = p2.\nresult = abap_true.\nENDIF.\nENDMETHOD.\n")
	if array {
		e.isArrayUsed = true
		def.WriteString("CLASS-METHODS array IMPORTING p0 TYPE REF TO " + d + " RETURNING VALUE(result) TYPE abap_bool.\n")
		impl.WriteString("METHOD array.\nDATA mark TYPE REF TO " + e.name("runtime.arraymark") + ".\nIF p0->tag = " + d + "=>tag_array.\nresult = abap_true.\nELSEIF p0->tag = " + d + "=>tag_ref AND p0->oval IS BOUND.\nTRY.\nmark ?= p0->oval.\nresult = abap_true.\nCATCH cx_sy_move_cast_error.\nENDTRY.\nENDIF.\nENDMETHOD.\n")
	}
	def.WriteString("PROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\n")
	e.files[file] = def.String() + "CLASS " + name + " IMPLEMENTATION.\n" + impl.String() + "ENDCLASS.\n"
	return name
}

func unionFieldType(t hir.Type) hir.Type {
	if t.Kind == hir.Optional {
		return t.Args[0]
	}
	return t
}

func abapString(s string) string { return "`" + strings.ReplaceAll(s, "`", "``") + "`" }

// unionLayoutDeclaration and unionLayoutImplementation add the payload's own
// layout check: required own keys, then the value category of every slot,
// read through the record's raw storage (never through guarded getters).
func (e *emitter) unionLayoutDeclaration(b *strings.Builder, c *hir.Class) {
	if !e.unionPayload(c) {
		return
	}
	fmt.Fprintf(b, "CLASS-METHODS %s IMPORTING p0 TYPE REF TO %s RETURNING VALUE(result) TYPE abap_bool.\n", e.name("union.layout"), e.name("runtime.dynamic"))
}

func (e *emitter) unionLayoutImplementation(out *strings.Builder, c *hir.Class) {
	if !e.unionPayload(c) {
		return
	}
	u, d := e.unionRuntime(), e.name("runtime.dynamic")
	fmt.Fprintf(out, "METHOD %s.\nDATA v TYPE REF TO %s.\n", e.name("union.layout"), d)
	var required []string
	for _, f := range c.Fields {
		if !f.Static && f.Type.Kind != hir.Optional {
			required = append(required, u+"=>has( p0 = p0 p1 = "+abapString(f.Name)+" ) = abap_false")
		}
	}
	if len(required) > 0 {
		out.WriteString("IF " + strings.Join(required, "\nOR ") + ".\nRETURN.\nENDIF.\n")
	}
	for _, f := range c.Fields {
		if f.Static {
			continue
		}
		t := unionFieldType(f.Type)
		var tags []string
		mismatch := ""
		switch t.Kind {
		case hir.String:
			tags = []string{"tag_string"}
		case hir.Number, hir.I32, hir.I64:
			tags = []string{"tag_number"}
		case hir.Bool:
			tags = []string{"tag_boolean"}
		case hir.Array:
			mismatch = u + "=>array( v ) = abap_false"
		case hir.ClassRef, hir.InterfaceRef, hir.UnionRef:
			tags = []string{"tag_ref", "tag_object"}
			if t.Kind == hir.ClassRef && t.Name == hir.RootObject {
				tags = append(tags, "tag_array", "tag_class")
			}
		case hir.OrderedMap, hir.OrderedSet:
			tags = []string{"tag_ref"}
		default:
			continue
		}
		if mismatch == "" {
			var parts []string
			for _, tag := range tags {
				parts = append(parts, "v->tag <> "+d+"=>"+tag)
			}
			mismatch = "( " + strings.Join(parts, " AND ") + " )"
		}
		condition := "v IS NOT BOUND OR " + mismatch
		if f.Type.Kind == hir.Optional {
			condition = "v IS BOUND AND " + mismatch
		}
		fmt.Fprintf(out, "v = p0->get( %s ).\nIF %s.\nRETURN.\nENDIF.\n", abapString(f.Name), condition)
	}
	out.WriteString("result = abap_true.\nENDMETHOD.\n")
}

// unionFamily registers a use of family; alternative names a checked
// admission (or, from car U2 on, a projection) that needs its own method.
func (e *emitter) unionFamily(family, alternative string) string {
	if e.unionFamilies == nil {
		e.unionFamilies = map[string]map[string]bool{}
	}
	if e.unionFamilies[family] == nil {
		e.unionFamilies[family] = map[string]bool{}
	}
	if alternative != "" {
		e.unionFamilies[family][alternative] = true
	}
	return e.name("union.family." + family)
}

func (e *emitter) unionCheck(family, alternative string) string {
	return e.unionFamily(family, alternative) + "=>" + e.name("union.check."+family+"/"+alternative)
}

func (e *emitter) unionFamilyClasses() {
	families := make([]string, 0, len(e.unionFamilies))
	for family := range e.unionFamilies {
		families = append(families, family)
	}
	sort.Strings(families)
	for _, family := range families {
		e.unionFamilyClass(e.p.Union(family), e.unionFamilies[family])
	}
}

func (e *emitter) unionFamilyClass(u *hir.UnionDecl, used map[string]bool) {
	name, runtime, d := e.name("union.family."+u.Name), e.unionRuntime(), e.name("runtime.dynamic")
	signature := " IMPORTING p0 TYPE REF TO " + d + " RETURNING VALUE(result) TYPE REF TO " + d + ".\n"
	var def, impl strings.Builder
	def.WriteString("CLASS " + name + " DEFINITION PUBLIC FINAL CREATE PUBLIC.\nPUBLIC SECTION.\nCLASS-METHODS check" + signature)
	impl.WriteString("METHOD tag.\nDATA t TYPE REF TO " + d + ".\nIF " + runtime + "=>record( p0 ) = abap_false.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.\n")
	impl.WriteString("t = p0->get( " + abapString(u.Discriminant) + " ).\nIF t IS NOT BOUND OR t->tag <> " + d + "=>tag_string.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.\nresult = t->sval.\nENDMETHOD.\n")
	accept := func(a hir.UnionAlternative) string {
		var tags []string
		for _, tag := range a.Tags {
			tags = append(tags, "t = "+abapString(tag))
		}
		conditions := []string{"( " + strings.Join(tags, "\nOR ") + " )", e.name(a.Payload.Name) + "=>" + e.name("union.layout") + "( p0 ) = abap_true"}
		for _, f := range a.Literals {
			if f.Field == u.Discriminant {
				continue
			}
			var values []string
			for _, v := range f.Values {
				values = append(values, runtime+"=>literal( p0 = p0 p1 = "+abapString(f.Field)+" p2 = "+abapString(v)+" ) = abap_true")
			}
			conditions = append(conditions, "( "+strings.Join(values, "\nOR ")+" )")
		}
		return "IF " + strings.Join(conditions, "\nAND ") + ".\nresult = p0.\nRETURN.\nENDIF.\n"
	}
	impl.WriteString("METHOD check.\nDATA t TYPE string.\nt = tag( p0 ).\n")
	for _, a := range u.Alternatives {
		impl.WriteString(accept(a))
	}
	impl.WriteString("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDMETHOD.\n")
	for _, a := range u.Alternatives {
		if !used[a.ID] {
			continue
		}
		method := e.name("union.check." + u.Name + "/" + a.ID)
		def.WriteString("CLASS-METHODS " + method + signature)
		impl.WriteString("METHOD " + method + ".\nDATA t TYPE string.\nt = tag( p0 ).\n" + accept(a) + "RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDMETHOD.\n")
	}
	def.WriteString("PROTECTED SECTION.\nPRIVATE SECTION.\nCLASS-METHODS tag IMPORTING p0 TYPE REF TO " + d + " RETURNING VALUE(result) TYPE string.\nENDCLASS.\n")
	e.files[name+".clas.abap"] = def.String() + "CLASS " + name + " IMPLEMENTATION.\n" + impl.String() + "ENDCLASS.\n"
}

// unionOperation emits the car U1 operations. Payload projection and common
// field reads need guarded payload views (car U2) and refuse at their site.
func (b *body) unionOperation(x *hir.Expr, n string) {
	e := b.e
	if n == "" {
		n = b.temp(x.Type)
	}
	switch {
	case x.Op == "union.admit":
		a := b.expr(x.X)
		boxed := b.temp(hir.T(hir.Dynamic))
		b.line(boxed + " = " + e.unionRuntime() + "=>box( " + a + " ).")
		b.line(n + " = " + e.unionCheck(x.Owner, x.Name) + "( " + boxed + " ).")
	case x.Op == "union.check" || x.Op == "union.widen" || x.Op == "union.project" && x.Type.Kind == hir.UnionRef:
		a := b.expr(x.X)
		b.line(n + " = " + e.unionFamily(x.Type.Name, "") + "=>check( " + a + " ).")
	case x.Op == "union.tag":
		a := b.expr(x.X)
		checked := b.temp(hir.T(hir.Dynamic))
		b.line(checked + " = " + e.unionFamily(x.X.Type.Name, "") + "=>check( " + a + " ).")
		tag := b.temp(hir.T(hir.Dynamic))
		b.line(tag + " = " + checked + "->get( " + abapString(x.Name) + " ).")
		b.line(n + " = " + tag + "->as_string( ).")
	case x.Op == "union.project":
		e.refuse(fmt.Errorf("node %d (%s): unsupported ABAP runtime op union.project to payload %s (guarded payload views are not implemented)", x.ID, x.Source, x.Type))
	default:
		e.refuse(fmt.Errorf("node %d (%s): unsupported ABAP runtime op %s", x.ID, x.Source, x.Op))
	}
}

// Union references are boxes, not values: Map/Set keys over them would need
// record identity rather than box identity.
func unionKeyed(x *hir.Expr) bool {
	if x.X == nil || x.X.Type.Kind != hir.OrderedMap && x.X.Type.Kind != hir.OrderedSet || len(x.X.Type.Args) == 0 {
		return false
	}
	return mentionsUnion(x.X.Type.Args[0])
}
