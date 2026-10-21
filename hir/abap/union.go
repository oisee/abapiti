package abap

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Structural record families (hir.UnionDecl) use representation C on ABAP:
// a union reference is the payload object itself, typed as REF TO one shared
// payload interface that every payload class implements once. kind is a
// program-wide payload class id, sig the id of the class's family membership
// signature, and tag reads a discriminant or literal field. A family is a set
// of signatures, so a membership test is a few integer comparisons; admission
// and widening are assignments and a payload projection is one cast. Only
// untyped values (JSON graphs, foreign records) are validated by layout, in
// from_dynamic, which views the record as its payload class. Go keeps its own
// boxed-record ABI. Programs without families never reach this file, so their
// output is unchanged.

// unionStorage stores union references of every family as one canonical
// union reference inside containers, so families share runtime classes.
func unionStorage(t hir.Type) hir.Type {
	if t.Kind == hir.UnionRef {
		return hir.Type{Kind: hir.UnionRef}
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
	_, ok := e.unionModel().kind[c.Name]
	return ok
}

// unionModel numbers payload classes and membership signatures once per
// program. Signatures are ordered by membership in the largest families
// first, so most families are one or a few signature intervals.
type unionModel struct {
	kind   map[string]int
	sig    map[string]int
	sigs   map[string][]int
	fields map[string][]string
	exact  map[string]bool
	same   map[string]string
}

func (e *emitter) unionModel() *unionModel {
	if e.unions != nil {
		return e.unions
	}
	m := &unionModel{kind: map[string]int{}, sig: map[string]int{}, sigs: map[string][]int{}, fields: map[string][]string{}, exact: map[string]bool{}, same: map[string]string{}}
	e.unions = m
	var families []*hir.UnionDecl
	members := map[string]map[string]bool{}
	for _, u := range e.p.Unions {
		if u == nil {
			continue
		}
		families = append(families, u)
		members[u.Name] = map[string]bool{}
		for _, a := range u.Alternatives {
			members[u.Name][a.Payload.Name] = true
		}
	}
	// Families with the same discriminant and alternatives (the same TS union
	// reached through two aliases) share one family class.
	canonical := map[string]string{}
	for _, u := range families {
		key, _ := json.Marshal(struct {
			D string
			A []hir.UnionAlternative
		}{u.Discriminant, u.Alternatives})
		if first, ok := canonical[string(key)]; !ok || u.Name < first {
			canonical[string(key)] = u.Name
		}
	}
	for _, u := range families {
		key, _ := json.Marshal(struct {
			D string
			A []hir.UnionAlternative
		}{u.Discriminant, u.Alternatives})
		m.same[u.Name] = canonical[string(key)]
	}
	order := append([]*hir.UnionDecl(nil), families...)
	sort.SliceStable(order, func(i, j int) bool {
		if len(members[order[i].Name]) != len(members[order[j].Name]) {
			return len(members[order[i].Name]) > len(members[order[j].Name])
		}
		return order[i].Name < order[j].Name
	})
	keys := map[string]string{}
	var payloads []string
	for _, u := range families {
		for _, a := range u.Alternatives {
			if _, ok := keys[a.Payload.Name]; !ok {
				keys[a.Payload.Name] = ""
				payloads = append(payloads, a.Payload.Name)
			}
		}
	}
	distinct := map[string]bool{}
	for _, c := range payloads {
		var key strings.Builder
		for _, u := range order {
			if members[u.Name][c] {
				key.WriteByte('1')
			} else {
				key.WriteByte('0')
			}
		}
		keys[c] = key.String()
		distinct[key.String()] = true
	}
	var signatures []string
	for key := range distinct {
		signatures = append(signatures, key)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(signatures)))
	id := map[string]int{}
	for i, key := range signatures {
		id[key] = i + 1
	}
	for _, c := range payloads {
		m.sig[c] = id[keys[c]]
	}
	sort.Slice(payloads, func(i, j int) bool {
		if m.sig[payloads[i]] != m.sig[payloads[j]] {
			return m.sig[payloads[i]] < m.sig[payloads[j]]
		}
		return payloads[i] < payloads[j]
	})
	for i, c := range payloads {
		m.kind[c] = i + 1
	}
	for _, u := range families {
		seen := map[int]bool{}
		for c := range members[u.Name] {
			if !seen[m.sig[c]] {
				seen[m.sig[c]] = true
				m.sigs[u.Name] = append(m.sigs[u.Name], m.sig[c])
			}
		}
		sort.Ints(m.sigs[u.Name])
	}
	fields := map[string]map[string]bool{}
	for _, u := range families {
		for _, a := range u.Alternatives {
			if fields[a.Payload.Name] == nil {
				fields[a.Payload.Name] = map[string]bool{}
			}
			fields[a.Payload.Name][u.Discriminant] = true
			for _, l := range a.Literals {
				fields[a.Payload.Name][l.Field] = true
			}
		}
	}
	for c, set := range fields {
		for f := range set {
			m.fields[c] = append(m.fields[c], f)
		}
		sort.Strings(m.fields[c])
	}
	// The domain of a tag or literal field over every alternative of its
	// class; an alternative that leaves the field open makes it unbounded.
	domain := map[string]map[string]bool{}
	open := map[string]bool{}
	for _, u := range families {
		for _, a := range u.Alternatives {
			for _, f := range m.fields[a.Payload.Name] {
				key := a.Payload.Name + "\x00" + f
				if domain[key] == nil {
					domain[key] = map[string]bool{}
				}
				values := []string(nil)
				if f == u.Discriminant {
					values = a.Tags
				}
				for _, l := range a.Literals {
					if l.Field == f {
						values = l.Values
					}
				}
				if values == nil {
					open[key] = true
				}
				for _, v := range values {
					domain[key][v] = true
				}
			}
		}
	}
	within := func(class, field string, values []string) bool {
		key := class + "\x00" + field
		if open[key] {
			return false
		}
		for v := range domain[key] {
			found := false
			for _, w := range values {
				found = found || v == w
			}
			if !found {
				return false
			}
		}
		return true
	}
	for _, u := range families {
		for _, a := range u.Alternatives {
			shared := 0
			for _, b := range u.Alternatives {
				if b.Payload.Equal(a.Payload) {
					shared++
				}
			}
			exact := shared == 1 && within(a.Payload.Name, u.Discriminant, a.Tags)
			for _, l := range a.Literals {
				if l.Field != u.Discriminant {
					exact = exact && within(a.Payload.Name, l.Field, l.Values)
				}
			}
			m.exact[u.Name+"/"+a.ID] = exact
		}
	}
	return m
}

func (e *emitter) unionInterface() string {
	name := e.name("union.payload")
	file := name + ".intf.abap"
	if _, ok := e.files[file]; !ok {
		e.files[file] = "INTERFACE " + name + " PUBLIC.\nMETHODS kind RETURNING VALUE(result) TYPE i.\nMETHODS sig RETURNING VALUE(result) TYPE i.\nMETHODS tag IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE string.\nENDINTERFACE.\n"
	}
	return name
}

// unionRuntime holds the record helpers of from_dynamic; runtime.dynamic is
// unchanged.
func (e *emitter) unionRuntime() string {
	name := e.name("union.runtime")
	file := name + ".clas.abap"
	if _, ok := e.files[file]; ok {
		return name
	}
	e.liveRuntime()
	d, storage, iface := e.name("runtime.dynamic"), e.name("view.storage"), e.unionInterface()
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
	def.WriteString("CLASS-METHODS record IMPORTING p0 TYPE REF TO " + d + " RETURNING VALUE(result) TYPE abap_bool.\n")
	def.WriteString("CLASS-METHODS has IMPORTING p0 TYPE REF TO " + d + " p1 TYPE string RETURNING VALUE(result) TYPE abap_bool.\n")
	def.WriteString("CLASS-METHODS literal IMPORTING p0 TYPE REF TO " + d + " p1 TYPE string p2 TYPE string RETURNING VALUE(result) TYPE abap_bool.\n")
	def.WriteString("CLASS-METHODS payload IMPORTING p0 TYPE REF TO " + d + " RETURNING VALUE(result) TYPE REF TO " + iface + ".\n")
	impl.WriteString("METHOD record.\nDATA adapter TYPE REF TO " + storage + ".\nDATA native TYPE REF TO object.\nIF p0 IS NOT BOUND.\nRETURN.\nENDIF.\nIF p0->tag = " + d + "=>tag_object.\nresult = abap_true.\nRETURN.\nENDIF.\nIF p0->tag <> " + d + "=>tag_ref.\nRETURN.\nENDIF.\n")
	impl.WriteString("native = p0->oval.\nIF native IS NOT INSTANCE OF " + storage + ".\nRETURN.\nENDIF.\nadapter ?= native.\nresult = adapter->structural( ).\nENDMETHOD.\n")
	impl.WriteString("METHOD has.\nDATA adapter TYPE REF TO " + storage + ".\nIF p0->tag = " + d + "=>tag_object.\nREAD TABLE p0->entries WITH KEY k = p1 TRANSPORTING NO FIELDS.\nresult = xsdbool( sy-subrc = 0 ).\nRETURN.\nENDIF.\nadapter ?= p0->oval.\nresult = adapter->has( p1 ).\nENDMETHOD.\n")
	impl.WriteString("METHOD literal.\nDATA v TYPE REF TO " + d + ".\nv = p0->get( p1 ).\nIF v IS BOUND AND v->tag = " + d + "=>tag_string AND v->sval = p2.\nresult = abap_true.\nENDIF.\nENDMETHOD.\n")
	impl.WriteString("METHOD payload.\nDATA native TYPE REF TO object.\nIF p0 IS NOT BOUND.\nRETURN.\nENDIF.\nIF p0->tag = " + d + "=>tag_ref.\nnative = p0->oval.\nIF native IS INSTANCE OF " + iface + ".\nresult ?= native.\nRETURN.\nENDIF.\nENDIF.\n")
	impl.WriteString("RAISE EXCEPTION TYPE " + e.name("view.refusal") + " EXPORTING reason = `union.go: payload: container slot of an unknown family needs a payload object`.\nENDMETHOD.\n")
	if array {
		e.isArrayUsed = true
		def.WriteString("CLASS-METHODS array IMPORTING p0 TYPE REF TO " + d + " RETURNING VALUE(result) TYPE abap_bool.\n")
		impl.WriteString("METHOD array.\nDATA native TYPE REF TO object.\nIF p0->tag = " + d + "=>tag_array.\nresult = abap_true.\nELSEIF p0->tag = " + d + "=>tag_ref AND p0->oval IS BOUND.\nnative = p0->oval.\nIF native IS INSTANCE OF " + e.name("runtime.arraymark") + ".\nresult = abap_true.\nENDIF.\nENDIF.\nENDMETHOD.\n")
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

// unionPayloadDeclaration and unionPayloadImplementation add the payload
// interface (kind, sig, tag) and the layout check that admits an untyped
// record: required own keys, then the value category of every slot, read
// through the record's raw storage (never through guarded getters).
func (e *emitter) unionPayloadDeclaration(b *strings.Builder, c *hir.Class) {
	if !e.unionPayload(c) {
		return
	}
	fmt.Fprintf(b, "INTERFACES %s.\n", e.unionInterface())
	fmt.Fprintf(b, "CLASS-METHODS %s IMPORTING p0 TYPE REF TO %s RETURNING VALUE(result) TYPE abap_bool.\n", e.name("union.layout"), e.name("runtime.dynamic"))
}

func (e *emitter) unionFieldRead(c *hir.Class, receiver, field string) string {
	if owner := e.fieldOwner(c, field); owner != nil && e.liveClasses[owner.Name] {
		return receiver + "->" + e.name("view.read."+owner.Name+"."+field) + "( )"
	}
	return receiver + "->" + e.member(field)
}

func (e *emitter) unionPayloadImplementation(out *strings.Builder, c *hir.Class) {
	if !e.unionPayload(c) {
		return
	}
	m, iface := e.unionModel(), e.unionInterface()
	fmt.Fprintf(out, "METHOD %s~kind.\nresult = %d.\nENDMETHOD.\nMETHOD %s~sig.\nresult = %d.\nENDMETHOD.\nMETHOD %s~tag.\n", iface, m.kind[c.Name], iface, m.sig[c.Name], iface)
	if fields := m.fields[c.Name]; len(fields) == 1 {
		fmt.Fprintf(out, "result = %s.\n", e.unionFieldRead(c, "me", fields[0]))
	} else {
		out.WriteString("CASE p0.\n")
		for _, f := range fields {
			fmt.Fprintf(out, "WHEN %s.\nresult = %s.\n", abapString(f), e.unionFieldRead(c, "me", f))
		}
		out.WriteString("ENDCASE.\n")
	}
	out.WriteString("ENDMETHOD.\n")
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

// unionUse registers a use of a family class and, optionally, of one of its
// on-demand methods: from_dynamic, from_object, alt:<id> (the value test of
// an alternative that its class alone does not prove) or read:<field>.
func (e *emitter) unionUse(family, method string) string {
	family = e.unionModel().same[family]
	if e.unionUses == nil {
		e.unionUses = map[string]map[string]bool{}
	}
	if e.unionUses[family] == nil {
		e.unionUses[family] = map[string]bool{}
	}
	if method != "" {
		e.unionUses[family][method] = true
	}
	if method == "from_object" {
		e.unionUses[family]["from_dynamic"] = true
	}
	return e.name("union.family." + family)
}

func (e *emitter) unionAlternative(family, alternative string) string {
	family = e.unionModel().same[family]
	return e.unionUse(family, "alt:"+alternative) + "=>" + e.name("union.check."+family+"/"+alternative)
}

func unionReadName(family, field string, t hir.Type) string {
	return "union.read." + family + "." + field + "." + t.String()
}

// unionFamilyClasses emits the used family classes; emitting one may use
// another family's methods, so it repeats until the uses are stable.
func (e *emitter) unionFamilyClasses() {
	emitted := map[string]string{}
	for {
		families := make([]string, 0, len(e.unionUses))
		for family := range e.unionUses {
			families = append(families, family)
		}
		sort.Strings(families)
		changed := false
		for _, family := range families {
			var methods []string
			for method := range e.unionUses[family] {
				methods = append(methods, method)
			}
			sort.Strings(methods)
			key := strings.Join(methods, "\n")
			if done, ok := emitted[family]; ok && done == key {
				continue
			}
			emitted[family] = key
			changed = true
			e.unionFamilyClass(e.p.Union(family), methods)
		}
		if !changed {
			return
		}
	}
}

// unionTagTest is the value test of an alternative on a typed payload.
func unionTagTest(u *hir.UnionDecl, a hir.UnionAlternative) string {
	var tags []string
	for _, tag := range a.Tags {
		tags = append(tags, "p0->tag( "+abapString(u.Discriminant)+" ) = "+abapString(tag))
	}
	conditions := []string{"( " + strings.Join(tags, "\nOR ") + " )"}
	for _, l := range a.Literals {
		if l.Field == u.Discriminant {
			continue
		}
		var values []string
		for _, v := range l.Values {
			values = append(values, "p0->tag( "+abapString(l.Field)+" ) = "+abapString(v))
		}
		conditions = append(conditions, "( "+strings.Join(values, "\nOR ")+" )")
	}
	return strings.Join(conditions, "\nAND ")
}

// unionMembership tests s against the family's signatures: up to four
// intervals as comparisons, otherwise one CASE over the signature ids.
func unionMembership(sigs []int) string {
	type interval struct{ lo, hi int }
	var intervals []interval
	for _, s := range sigs {
		if n := len(intervals); n > 0 && intervals[n-1].hi+1 == s {
			intervals[n-1].hi = s
			continue
		}
		intervals = append(intervals, interval{s, s})
	}
	if len(intervals) > 4 {
		var values []string
		for _, s := range sigs {
			values = append(values, fmt.Sprint(s))
		}
		return "CASE s.\nWHEN " + strings.Join(values, "\nOR ") + ".\nresult = abap_true.\nENDCASE.\n"
	}
	var conditions []string
	for _, i := range intervals {
		if i.lo == i.hi {
			conditions = append(conditions, fmt.Sprintf("s = %d", i.lo))
		} else {
			conditions = append(conditions, fmt.Sprintf("( s >= %d AND s <= %d )", i.lo, i.hi))
		}
	}
	return "IF " + strings.Join(conditions, "\nOR ") + ".\nresult = abap_true.\nENDIF.\n"
}

func (e *emitter) unionFamilyClass(u *hir.UnionDecl, methods []string) {
	m := e.unionModel()
	name, iface, d := e.name("union.family."+u.Name), e.unionInterface(), e.name("runtime.dynamic")
	ref := "REF TO " + iface
	var def, impl strings.Builder
	def.WriteString("CLASS " + name + " DEFINITION PUBLIC FINAL CREATE PUBLIC.\nPUBLIC SECTION.\n")
	def.WriteString("CLASS-METHODS member IMPORTING p0 TYPE " + ref + " RETURNING VALUE(result) TYPE abap_bool.\n")
	def.WriteString("CLASS-METHODS check IMPORTING p0 TYPE " + ref + " RETURNING VALUE(result) TYPE " + ref + ".\n")
	impl.WriteString("METHOD member.\nDATA s TYPE i.\nIF p0 IS NOT BOUND.\nRETURN.\nENDIF.\ns = p0->sig( ).\n" + unionMembership(m.sigs[u.Name]))
	var classes []string
	alternatives := map[string][]hir.UnionAlternative{}
	for _, a := range u.Alternatives {
		if alternatives[a.Payload.Name] == nil {
			classes = append(classes, a.Payload.Name)
		}
		alternatives[a.Payload.Name] = append(alternatives[a.Payload.Name], a)
	}
	var tested strings.Builder
	for _, c := range classes {
		exact := true
		var tests []string
		for _, a := range alternatives[c] {
			exact = exact && m.exact[u.Name+"/"+a.ID]
			tests = append(tests, "( "+unionTagTest(u, a)+" )")
		}
		if !exact {
			fmt.Fprintf(&tested, "WHEN %d.\nresult = abap_false.\nIF %s.\nresult = abap_true.\nENDIF.\n", m.kind[c], strings.Join(tests, "\nOR "))
		}
	}
	if tested.Len() > 0 {
		impl.WriteString("IF result = abap_true.\nCASE p0->kind( ).\n" + tested.String() + "ENDCASE.\nENDIF.\n")
	}
	impl.WriteString("ENDMETHOD.\n")
	impl.WriteString("METHOD check.\nIF member( p0 ) = abap_false.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.\nresult = p0.\nENDMETHOD.\n")
	private := ""
	makeBody := func(method string) *body {
		return &body{e: e, c: &hir.Class{Name: u.Name}, m: &hir.Method{Result: hir.T(hir.Void)}, implemented: method, locals: map[string]string{}}
	}
	emit := func(method string, b *body) {
		fmt.Fprintf(&impl, "METHOD %s.\n%s%sENDMETHOD.\n", method, b.constantDeclarations(), foldTempInits(b.code.String()))
	}
	for _, method := range methods {
		switch {
		case strings.HasPrefix(method, "alt:"):
			a := u.Alternative(strings.TrimPrefix(method, "alt:"))
			check := e.name("union.check." + u.Name + "/" + a.ID)
			def.WriteString("CLASS-METHODS " + check + " IMPORTING p0 TYPE " + ref + " RETURNING VALUE(result) TYPE " + ref + ".\n")
			fmt.Fprintf(&impl, "METHOD %s.\nIF p0 IS BOUND.\nIF p0->kind( ) = %d.\nIF %s.\nresult = p0.\nRETURN.\nENDIF.\nENDIF.\nENDIF.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDMETHOD.\n", check, m.kind[a.Payload.Name], unionTagTest(u, *a))
		case method == "from_dynamic":
			def.WriteString("CLASS-METHODS from_dynamic IMPORTING p0 TYPE REF TO " + d + " RETURNING VALUE(result) TYPE " + ref + ".\n")
			private = "CLASS-METHODS tag IMPORTING p0 TYPE REF TO " + d + " RETURNING VALUE(result) TYPE string.\n"
			runtime := e.unionRuntime()
			impl.WriteString("METHOD tag.\nDATA t TYPE REF TO " + d + ".\nIF " + runtime + "=>record( p0 ) = abap_false.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.\n")
			impl.WriteString("t = p0->get( " + abapString(u.Discriminant) + " ).\nIF t IS NOT BOUND OR t->tag <> " + d + "=>tag_string.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDIF.\nresult = t->sval.\nENDMETHOD.\n")
			b := makeBody("from_dynamic")
			b.line("DATA p TYPE " + ref + ".\nDATA native TYPE REF TO object.\nDATA t TYPE string.")
			b.line("IF p0 IS BOUND AND p0->tag = " + d + "=>tag_ref.\nnative = p0->oval.\nIF native IS INSTANCE OF " + iface + ".\np ?= native.\nIF member( p ) = abap_true.\nresult = p.\nRETURN.\nENDIF.\nENDIF.\nENDIF.")
			b.line("t = tag( p0 ).")
			for _, a := range u.Alternatives {
				var tags []string
				for _, tag := range a.Tags {
					tags = append(tags, "t = "+abapString(tag))
				}
				conditions := []string{"( " + strings.Join(tags, "\nOR ") + " )", e.name(a.Payload.Name) + "=>" + e.name("union.layout") + "( p0 ) = abap_true"}
				for _, l := range a.Literals {
					if l.Field == u.Discriminant {
						continue
					}
					var values []string
					for _, v := range l.Values {
						values = append(values, runtime+"=>literal( p0 = p0 p1 = "+abapString(l.Field)+" p2 = "+abapString(v)+" ) = abap_true")
					}
					conditions = append(conditions, "( "+strings.Join(values, "\nOR ")+" )")
				}
				b.line("IF " + strings.Join(conditions, "\nAND ") + ".")
				c := e.classBy(a.Payload.Name)
				if e.wideShape(c) {
					e.refuse(fmt.Errorf("union family %s: payload %s has no view constructor", u.Name, c.Name))
				}
				args := []string{"view_source = p0"}
				if c.Ctor != nil {
					for _, p := range c.Ctor.Params {
						args = append(args, e.param(p.Name)+" = "+b.temp(p.Type))
					}
				}
				view := b.temp(a.Payload)
				b.line(view + " = NEW " + e.name(c.Name) + "( " + strings.Join(args, " ") + " ).")
				b.line("result = " + view + ".\nRETURN.\nENDIF.")
			}
			b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
			emit("from_dynamic", b)
		case method == "from_object":
			storage := e.name("view.storage")
			def.WriteString("CLASS-METHODS from_object IMPORTING p0 TYPE REF TO object RETURNING VALUE(result) TYPE " + ref + ".\n")
			impl.WriteString("METHOD from_object.\nDATA p TYPE " + ref + ".\nDATA d TYPE REF TO " + d + ".\nDATA adapter TYPE REF TO " + storage + ".\n")
			impl.WriteString("IF p0 IS INSTANCE OF " + iface + ".\np ?= p0.\nIF member( p ) = abap_true.\nresult = p.\nRETURN.\nENDIF.\nENDIF.\n")
			impl.WriteString("IF p0 IS INSTANCE OF " + d + ".\nd ?= p0.\nELSEIF p0 IS INSTANCE OF " + storage + ".\nadapter ?= p0.\nd = adapter->source( ).\nENDIF.\n")
			impl.WriteString("IF d IS NOT BOUND AND p0 IS BOUND.\nCREATE OBJECT d.\nd->tag = " + d + "=>tag_ref.\nd->oval = p0.\nENDIF.\nresult = from_dynamic( d ).\nENDMETHOD.\n")
		case strings.HasPrefix(method, "read:"):
			parts := strings.SplitN(strings.TrimPrefix(method, "read:"), "\x00", 2)
			field := parts[0]
			var t hir.Type
			for _, r := range e.unionReads[u.Name] {
				if r.field == field && r.t.String() == parts[1] {
					t = r.t
				}
			}
			read := e.name(unionReadName(u.Name, field, t))
			def.WriteString("CLASS-METHODS " + read + " IMPORTING p0 TYPE " + ref + " RETURNING VALUE(result) TYPE " + e.typ(t) + ".\n")
			b := makeBody(read)
			b.line("CASE p0->kind( ).")
			for _, c := range classes {
				class := e.classBy(c)
				b.line(fmt.Sprintf("WHEN %d.", m.kind[c]))
				var slot *hir.Field
				for k := class; k != nil && slot == nil; k = e.classBy(k.Super) {
					for i := range k.Fields {
						if k.Fields[i].Name == field && !k.Fields[i].Static {
							slot = &k.Fields[i]
						}
					}
				}
				if slot == nil {
					continue
				}
				payload := b.temp(hir.Ref(c))
				b.line(payload + " ?= p0.")
				value := b.temp(slot.Type)
				b.line(value + " = " + e.unionFieldRead(class, payload, field) + ".")
				b.line("result = " + b.convert(value, slot.Type, t) + ".")
			}
			b.line("WHEN OTHERS.\nRAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.\nENDCASE.")
			emit(read, b)
		}
	}
	def.WriteString("PROTECTED SECTION.\nPRIVATE SECTION.\n" + private + "ENDCLASS.\n")
	e.files[name+".clas.abap"] = def.String() + "CLASS " + name + " IMPLEMENTATION.\n" + impl.String() + "ENDCLASS.\n"
}

type unionRead struct {
	field string
	t     hir.Type
}

// unionFromDynamic admits an untyped value into a family (or, for the
// canonical container slot, into any family).
func (b *body) unionFromDynamic(n, d string, t hir.Type) {
	if t.Name == "" {
		b.line(n + " = " + b.e.unionRuntime() + "=>payload( " + d + " ).")
		return
	}
	b.line(n + " = " + b.e.unionUse(t.Name, "from_dynamic") + "=>from_dynamic( " + d + " ).")
}

// unionFromObject admits an erased array slot: the payload object itself,
// or a foreign record that from_dynamic validates.
func (b *body) unionFromObject(n, row string, t hir.Type) {
	if t.Name == "" {
		b.line(n + " ?= " + row + ".")
		return
	}
	b.line(n + " = " + b.e.unionUse(t.Name, "from_object") + "=>from_object( " + row + " ).")
}

// unionOperation emits the union operations of representation C.
func (b *body) unionOperation(x *hir.Expr, n string) {
	e := b.e
	if n == "" {
		n = b.temp(x.Type)
	}
	m := e.unionModel()
	switch {
	case x.Op == "union.admit":
		a := b.expr(x.X)
		b.line(n + " = " + a + ".")
		if !m.exact[x.Owner+"/"+x.Name] {
			b.line(n + " = " + e.unionAlternative(x.Owner, x.Name) + "( " + n + " ).")
		}
	case x.Op == "union.check" || x.Op == "union.project" && x.Type.Kind == hir.UnionRef && x.X.Type.Kind == hir.Dynamic:
		b.unionFromDynamic(n, b.expr(x.X), x.Type)
	case x.Op == "union.widen":
		b.line(n + " = " + b.expr(x.X) + ".")
	case x.Op == "union.project" && x.Type.Kind == hir.UnionRef:
		a := b.expr(x.X)
		if hir.UnionIncludes(e.p.Union(x.Type.Name), e.p.Union(x.X.Type.Name)) {
			b.line(n + " = " + a + ".")
		} else {
			b.line(n + " = " + e.unionUse(x.Type.Name, "") + "=>check( " + a + " ).")
		}
	case x.Op == "union.project":
		a := b.expr(x.X)
		if !m.exact[x.Owner+"/"+x.Name] {
			checked := b.temp(x.X.Type)
			b.line(checked + " = " + e.unionAlternative(x.Owner, x.Name) + "( " + a + " ).")
			a = checked
		}
		b.line(n + " ?= " + a + ".")
	case x.Op == "union.tag":
		b.line(n + " = " + b.expr(x.X) + "->tag( " + abapString(x.Name) + " ).")
	case x.Op == "union.read":
		a := b.expr(x.X)
		family := m.same[x.X.Type.Name]
		if e.unionReads == nil {
			e.unionReads = map[string][]unionRead{}
		}
		known := false
		for _, r := range e.unionReads[family] {
			known = known || r.field == x.Name && r.t.Equal(x.Type)
		}
		if !known {
			e.unionReads[family] = append(e.unionReads[family], unionRead{x.Name, x.Type})
		}
		class := e.unionUse(family, "read:"+x.Name+"\x00"+x.Type.String())
		b.line(n + " = " + class + "=>" + e.name(unionReadName(family, x.Name, x.Type)) + "( " + a + " ).")
	default:
		e.refuse(fmt.Errorf("node %d (%s): unsupported ABAP runtime op %s", x.ID, x.Source, x.Op))
	}
}

// Union references are payload objects; Map and Set keys over them keep
// their refusal until keyed identity is reviewed against Go's hash keys.
func unionKeyed(x *hir.Expr) bool {
	if x.X == nil || x.X.Type.Kind != hir.OrderedMap && x.X.Type.Kind != hir.OrderedSet || len(x.X.Type.Args) == 0 {
		return false
	}
	return mentionsUnion(x.X.Type.Args[0])
}
