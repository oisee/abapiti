package abap

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// A module initializer above the chunk bound becomes consecutive chunk
// methods. The emitted ABAP Unit class checks the JavaScript semantics of a
// refusal in the middle: every earlier statement ran, no later one did, the
// exception is the located trap, and a later access rethrows that same
// failure without running the initializer again (InitFailure).
func TestModuleInitializerChunks(t *testing.T) {
	old := initChunkNodes
	initChunkNodes = 12
	defer func() { initChunkNodes = old }()
	static := func(name string) *hir.Expr {
		return &hir.Expr{Kind: hir.StaticGet, Owner: "Init", Name: name, Type: i32}
	}
	set := func(name string, x *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Assign, X: static(name), Y: x} }
	increment := func() *hir.Stmt { return set("count", binary("+", static("count"), lit(1), i32)) }
	var list []*hir.Stmt
	for i := 0; i < 14; i++ {
		list = append(list, increment())
	}
	// One local group: the declaration and its uses stay in one chunk.
	list = append(list, decl("t", i32, lit(100)), increment(), increment(), set("seen", local("t", i32)))
	list = append(list, &hir.Stmt{Kind: hir.Trap, Name: "lowering trap: node 7 (m.ts:3:1): refused"})
	for i := 0; i < 14; i++ {
		list = append(list, increment())
	}
	init := method("class_constructor", hir.T(hir.Void), hir.B(list...))
	init.Static = true
	init.InitFailure = true
	p := &hir.Program{Classes: []*hir.Class{{Name: "Init", Fields: []hir.Field{{Name: "count", Type: i32, Static: true}, {Name: "seen", Type: i32, Static: true}}, Methods: []*hir.Method{init}}}}
	files, names := abiSource(t, p)
	class := names.Get("Init")
	src := files[class+".clas.abap"]
	initialize := names.Get("builtin.initialize.Init")
	body := regexp.MustCompile(`(?s)METHOD ` + initialize + `\.\n(.*?)ENDMETHOD\.`).FindStringSubmatch(src)
	if body == nil {
		t.Fatal("no initializer method")
	}
	calls := regexp.MustCompile(`(\w+)\( \)\.`).FindAllStringSubmatch(body[1], -1)
	if len(calls) < 3 {
		t.Fatalf("initializer not chunked:\n%s", body[1])
	}
	for i, call := range calls {
		chunk := names.Get("builtin.initializeChunk.Init." + string(rune('1'+i)))
		if i < 9 && call[1] != chunk {
			t.Fatalf("call %d is %s, want %s", i, call[1], chunk)
		}
		if !strings.Contains(src, "CLASS-METHODS "+call[1]+".\n") || !strings.Contains(src, "METHOD "+call[1]+".\n") {
			t.Fatalf("chunk %s not declared and implemented", call[1])
		}
	}
	if strings.Contains(body[1], "+ 1") {
		t.Fatal("initializer statements left in the calling method")
	}
	failure := names.Get("builtin.initFailure.Init")
	for _, part := range []string{"CLASS-DATA " + failure + " TYPE REF TO cx_root.\n", "IF " + failure + " IS BOUND.\n", "RAISE EXCEPTION " + failure + ".\n", "CATCH cx_root INTO "} {
		if !strings.Contains(src, part) {
			t.Fatalf("rethrow part %q missing", part)
		}
	}
	local := false
	for _, chunk := range regexp.MustCompile(`(?s)METHOD (\w+)\.\n(.*?)ENDMETHOD\.`).FindAllStringSubmatch(src, -1) {
		if strings.Contains(chunk[2], "CONV i( 100 )") {
			local = strings.Contains(chunk[2], "=>"+names.Get("member.seen")+" = ")
		}
	}
	if !local {
		t.Fatal("local t split from its use")
	}
	again, _, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	if again[class+".clas.abap"] != src {
		t.Fatal("nondeterministic chunking")
	}
	unexecuted := names.Get("exception.unexecuted")
	files[class+".clas.testclasses.abap"] = "CLASS ltcl_init DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS stops_at_trap FOR TESTING.\nENDCLASS.\nCLASS ltcl_init IMPLEMENTATION.\nMETHOD stops_at_trap.\n" +
		"DATA trapped TYPE REF TO " + unexecuted + ".\nDATA again TYPE REF TO " + unexecuted + ".\nTRY.\n" + class + "=>" + initialize + "( ).\nCATCH " + unexecuted + " INTO trapped.\nENDTRY.\n" +
		"TRY.\n" + class + "=>" + initialize + "( ).\nCATCH " + unexecuted + " INTO again.\nENDTRY.\n" +
		"cl_abap_unit_assert=>assert_bound( trapped ).\n" +
		"cl_abap_unit_assert=>assert_true( xsdbool( again = trapped ) ).\n" +
		"cl_abap_unit_assert=>assert_equals( act = trapped->source_location exp = `lowering trap: node 7 (m.ts:3:1): refused` ).\n" +
		"cl_abap_unit_assert=>assert_equals( act = " + class + "=>" + names.Get("member.count") + " exp = 16 ).\n" +
		"cl_abap_unit_assert=>assert_equals( act = " + class + "=>" + names.Get("member.seen") + " exp = 100 ).\n" +
		"ENDMETHOD.\nENDCLASS.\n"
}

// Small initializers keep the single-method layout byte for byte.
func TestModuleInitializerBelowBoundIsOneMethod(t *testing.T) {
	init := method("class_constructor", hir.T(hir.Void), hir.B(
		&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.StaticGet, Owner: "Small", Name: "count", Type: i32}, Y: binary("+", &hir.Expr{Kind: hir.StaticGet, Owner: "Small", Name: "count", Type: i32}, lit(1), i32)}))
	init.Static = true
	p := &hir.Program{Classes: []*hir.Class{{Name: "Small", Fields: []hir.Field{{Name: "count", Type: i32, Static: true}}, Methods: []*hir.Method{init}}}}
	files, names := abiSource(t, p)
	if src := files[names.Get("Small")+".clas.abap"]; strings.Contains(src, "initializechunk") || strings.Count(src, "METHOD ") != 1 || strings.Contains(src, "CATCH") {
		t.Fatalf("small initializer was chunked:\n%s", src)
	}
}

func TestLocalGroups(t *testing.T) {
	s := func(names ...string) *hir.Stmt {
		var args []*hir.Expr
		for _, n := range names {
			args = append(args, local(n, i32))
		}
		return run(rt("noop", nil, i32, args...))
	}
	list := []*hir.Stmt{s(), decl("a", i32, lit(1)), s(), s("a"), s(), decl("b", i32, lit(2)), s("b"), decl("c", i32, local("b", i32)), s("c"), s()}
	var sizes []string
	for _, g := range localGroups(list) {
		sizes = append(sizes, strconv.Itoa(len(g)))
	}
	if got := strings.Join(sizes, ","); got != "1,3,1,4,1" {
		t.Fatalf("groups %s", got)
	}
}
