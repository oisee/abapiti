package abap

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestStructuralInterfaceAdmissionPairs(t *testing.T) {
	t.Setenv("ABAPITI_INLINE", "0")
	iface := &hir.Interface{Name: "Reader", Methods: []*hir.Method{{Name: "read", Params: []hir.Param{{Name: "label", Type: str}}, Result: str}}}
	read := method("read", str, ret(local("value", str)))
	read.Virtual = true
	read.Params = []hir.Param{{Name: "value", Type: str}}
	base := &hir.Class{Name: "Base", Methods: []*hir.Method{read}}
	child := &hir.Class{Name: "Child", Super: "Base"}
	unused := &hir.Class{Name: "Unused", Super: "Base"}
	it := hir.Type{Kind: hir.InterfaceRef, Name: iface.Name}
	cast := func(c string) *hir.Expr { return &hir.Expr{Kind: hir.Cast, Type: it, X: newObj(hir.Ref(c))} }
	runMethod := method("run", hir.T(hir.Void), hir.B(run(cast(child.Name)), run(cast(child.Name))))
	runMethod.Static = true
	p := &hir.Program{Interfaces: []*hir.Interface{iface}, Classes: []*hir.Class{base, child, unused, {Name: "Probe", Methods: []*hir.Method{runMethod}}}}
	before, _ := json.Marshal(p)
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(p)
	if string(before) != string(after) {
		t.Fatal("ABAP admission changed shared HIR")
	}
	decl := "INTERFACES " + names.Get(iface.Name) + "."
	source := files[names.Get(child.Name)+".clas.abap"]
	if strings.Count(source, decl) != 1 || !strings.Contains(source, names.Get("param.value")+" = "+names.Get("param.label")) {
		t.Fatal("inherited implementation must map parameters by position and declare the pair once", source)
	}
	for _, c := range []*hir.Class{base, unused} {
		if strings.Contains(files[names.Get(c.Name)+".clas.abap"], decl) {
			t.Fatalf("unused pair %s was admitted", c.Name)
		}
	}
}

func TestStructuralInterfaceSignatureRefusalIsLocated(t *testing.T) {
	t.Setenv("ABAPITI_INLINE", "0")
	iface := &hir.Interface{Name: "Reader", Methods: []*hir.Method{{Name: "read", Result: str}}}
	read := method("read", i32, ret(lit(1)))
	c := &hir.Class{Name: "ReaderImpl", Methods: []*hir.Method{read}}
	node := hir.Node{ID: 81, Source: "probe.ts:9:4"}
	cast := &hir.Expr{Node: node, Kind: hir.Cast, Type: hir.Type{Kind: hir.InterfaceRef, Name: iface.Name}, X: newObj(hir.Ref(c.Name))}
	m := method("run", hir.T(hir.Void), hir.B(run(cast)))
	m.Static = true
	p := &hir.Program{Interfaces: []*hir.Interface{iface}, Classes: []*hir.Class{c, {Name: "Probe", Methods: []*hir.Method{m}}}}
	_, err := emitABAP(t, p)
	if err == nil || !strings.Contains(err.Error(), node.Source) || !strings.Contains(err.Error(), "cannot preserve the source value") {
		t.Fatalf("missing located refusal: %v", err)
	}
	refusals := Audit(p)
	var refusal Refusal
	if len(refusals) != 1 || !errors.As(refusals[0], &refusal) || refusal.Method != m || refusal.Node.Source != node.Source {
		t.Fatalf("refusal lost owning body: %#v", refusals)
	}
}

func TestStructuralInterfaceKeyIdentityRefusal(t *testing.T) {
	t.Setenv("ABAPITI_INLINE", "0")
	iface := &hir.Interface{Name: "Reader", Methods: []*hir.Method{{Name: "read", Result: hir.T(hir.Dynamic)}}}
	read := method("read", str, ret(hir.L(str, "value")))
	c := &hir.Class{Name: "ReaderImpl", Methods: []*hir.Method{read}}
	it := hir.Type{Kind: hir.InterfaceRef, Name: iface.Name}
	mt := hir.T(hir.OrderedMap, it, str)
	cast := &hir.Expr{Kind: hir.Cast, Type: it, X: newObj(hir.Ref(c.Name))}
	put := rt("map.set", local("map", mt), mt, cast, hir.L(str, "value"))
	put.Node = hir.Node{ID: 90, Source: "probe.ts:12:3"}
	m := method("run", hir.T(hir.Void), hir.B(decl("map", mt, newObj(mt)), run(put)))
	m.Static = true
	p := &hir.Program{Interfaces: []*hir.Interface{iface}, Classes: []*hir.Class{c, {Name: "Probe", Methods: []*hir.Method{m}}}}
	_, err := emitABAP(t, p)
	if err == nil || !strings.Contains(err.Error(), put.Source) || !strings.Contains(err.Error(), "canonical identity") {
		t.Fatalf("missing collection identity refusal: %v", err)
	}
}

func TestStructuralInterfaceIntegerBoxABAPHost(t *testing.T) {
	iface := &hir.Interface{Name: "Reader", Methods: []*hir.Method{{Name: "read", Result: hir.T(hir.Dynamic)}}}
	read := method("read", hir.T(hir.I64), ret(hir.L(hir.T(hir.I64), 42)))
	c := &hir.Class{Name: "ReaderImpl", Methods: []*hir.Method{read}}
	it := hir.Type{Kind: hir.InterfaceRef, Name: iface.Name}
	cast := &hir.Expr{Kind: hir.Cast, Type: it, X: newObj(hir.Ref(c.Name))}
	m := method("run", hir.T(hir.Dynamic), ret(call(hir.VirtualCall, cast, "", "read", hir.T(hir.Dynamic))))
	m.Static = true
	p := &hir.Program{Interfaces: []*hir.Interface{iface}, Classes: []*hir.Class{c, {Name: "Probe", Methods: []*hir.Method{m}}}}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner, dynamic := names.Get("Probe"), names.Get("runtime.dynamic")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_probe DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_probe IMPLEMENTATION.\nMETHOD check.\nDATA actual TYPE REF TO %s.\nDATA expected TYPE f.\nexpected = 42.\nactual = %s=>%s( ).\ncl_abap_unit_assert=>assert_equals( act = actual->tag exp = %s=>tag_number ).\ncl_abap_unit_assert=>assert_equals( act = actual->nval exp = expected ).\nENDMETHOD.\nENDCLASS.\n", dynamic, owner, names.Get("member.run"), dynamic)
	storageHostFixture(t, files, owner)
}
