package abap

import (
	"fmt"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// A missed lookup is only undefined when the class layout is known. Omitted
// fields and unsupported carriers must retain an explicit, located refusal.
func TestInterfacePropertyLayoutRefusals(t *testing.T) {
	it := hir.Type{Kind: hir.InterfaceRef, Name: "Marker"}
	dyn := hir.T(hir.Dynamic)
	marker := &hir.Interface{Name: it.Name}
	dropped := &hir.Class{Node: hir.Node{ID: 81, Source: "probe.ts:3:1"}, Name: "Dropped", Implements: []string{it.Name}, FieldsDropped: true}
	unsupported := &hir.Class{Name: "Unsupported", Structural: true, Implements: []string{it.Name}}
	node := hir.Node{ID: 404, Source: "probe.ts:9:7"}
	read := method("read", dyn, ret(&hir.Expr{Node: node, Kind: hir.RuntimeOp, Op: "object.get", Type: dyn, X: local("input", it), Args: []*hir.Expr{hir.L(str, "unknown")}}))
	read.Static = true
	read.Params = []hir.Param{{Name: "input", Type: it}}
	p := &hir.Program{Interfaces: []*hir.Interface{marker}, Classes: []*hir.Class{dropped, unsupported, {Name: "Probe", Methods: []*hir.Method{read}}}}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("Probe")
	unit := "CLASS ltcl_layout DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_layout IMPLEMENTATION.\nMETHOD check.\nDATA value TYPE REF TO " + names.Get("runtime.dynamic") + ".\nDATA refused TYPE abap_bool.\nDATA text TYPE string.\n"
	for i, c := range []string{"Dropped", "Unsupported"} {
		unit += fmt.Sprintf("CLEAR refused.\nTRY.\nvalue = %s=>%s( NEW %s( ) ).\nCATCH %s INTO DATA(error%d).\nrefused = abap_true.\ntext = error%d->get_text( ).\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", owner, names.Get("member.read"), names.Get(c), names.Get("view.refusal"), i, i)
		location := map[string]string{"Dropped": "node 81 (probe.ts:3:1)", "Unsupported": "node 404 (probe.ts:9:7)"}[c]
		unit += fmt.Sprintf("cl_abap_unit_assert=>assert_true( xsdbool( text CS `%s` ) ).\n", location)
	}
	unit += "ENDMETHOD.\nENDCLASS.\n"
	files[owner+".clas.testclasses.abap"] = unit
	storageHostFixture(t, files, owner)
}

func TestInterfacePropertyAdmissionAdapter(t *testing.T) {
	dyn := hir.T(hir.Dynamic)
	iface := &hir.Interface{Name: "Reader", Methods: []*hir.Method{{Name: "read", Result: dyn}}}
	it := hir.Type{Kind: hir.InterfaceRef, Name: iface.Name}
	c := &hir.Class{Name: "NativeReader", Fields: []hir.Field{{Name: "value", Type: str}}, Methods: []*hir.Method{method("read", str, ret(hir.L(str, "method")))}}
	c.Ctor = method("constructor", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: str, Name: "value", X: &hir.Expr{Kind: hir.This, Type: hir.Ref(c.Name)}}, Y: hir.L(str, "native")}))
	cast := &hir.Expr{Kind: hir.Cast, Type: it, X: newObj(hir.Ref(c.Name))}
	m := method("run", str, ret(rt("dynamic.asString", rt("object.get", cast, dyn, hir.L(str, "value")), str)))
	m.Static = true
	files, names, err := emitABAPNamed(t, &hir.Program{Interfaces: []*hir.Interface{iface}, Classes: []*hir.Class{c, {Name: "Probe", Methods: []*hir.Method{m}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files[names.Get("view.interface.NativeReader.Reader")+".clas.abap"]; !ok {
		t.Fatal("probe did not create an interface admission adapter")
	}
	owner := names.Get("Probe")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_adapter DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_adapter IMPLEMENTATION.\nMETHOD check.\nDATA actual TYPE string.\nactual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = `native` ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.run"))
	storageHostFixture(t, files, owner)
}
