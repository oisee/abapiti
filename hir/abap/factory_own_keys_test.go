package abap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestLSPFactoryPresence(t *testing.T) {
	const prefix = "node_modules/vscode-languageserver-types/lib/umd/main.d.ts."
	opts := hir.Ref(prefix + "RenameFileOptions")
	rename := hir.Ref(prefix + "RenameFile")
	position := hir.Ref(prefix + "Position")
	ob := hir.T(hir.Optional, boolean)
	annotationType := hir.T(hir.Optional, str)
	oo := hir.T(hir.Optional, opts)
	dyn := hir.T(hir.Dynamic)
	shape := func(typ hir.Type, fields []hir.Field) *hir.Class {
		c := &hir.Class{Name: typ.Name, Structural: true, Fields: fields}
		ctor := method("constructor", hir.T(hir.Void), hir.B())
		for _, f := range fields {
			ctor.Params = append(ctor.Params, hir.Param{Name: f.Name, Type: f.Type})
			ctor.Body.List = append(ctor.Body.List, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: f.Type, Name: f.Name, X: &hir.Expr{Kind: hir.This, Type: typ}}, Y: local(f.Name, f.Type)})
		}
		c.Ctor = ctor
		return c
	}
	optClass := shape(opts, []hir.Field{{Name: "overwrite", Type: ob}, {Name: "ignoreIfExists", Type: ob}})
	renameClass := shape(rename, []hir.Field{{Name: "kind", Type: str}, {Name: "oldUri", Type: str}, {Name: "newUri", Type: str}, {Name: "options", Type: oo}, {Name: "annotationId", Type: annotationType}})
	posClass := shape(position, []hir.Field{{Name: "line", Type: hir.T(hir.Number)}, {Name: "character", Type: hir.T(hir.Number)}})
	create := method("create", dyn, ret(rt("dynamic.of", &hir.Expr{Kind: hir.New, Type: rename, Op: "lsp.RenameFile.create", Args: []*hir.Expr{hir.L(str, "rename"), hir.L(str, "old"), hir.L(str, "new"), local("options", oo), local("annotation", annotationType)}}, dyn)))
	create.Static = true
	create.Params = []hir.Param{{Name: "options", Type: oo}, {Name: "annotation", Type: annotationType}}
	emptyKeys := []string{}
	flagKeys := []string{"overwrite"}
	empty := method("empty", opts, ret(&hir.Expr{Kind: hir.New, Type: opts, OwnKeys: &emptyKeys, Args: []*hir.Expr{hir.L(ob, nil), hir.L(ob, nil)}}))
	empty.Static = true
	flag := method("flag", opts, ret(&hir.Expr{Kind: hir.New, Type: opts, OwnKeys: &flagKeys, Args: []*hir.Expr{hir.L(ob, false), hir.L(ob, nil)}}))
	flag.Static = true
	ignoreKeys := []string{"ignoreIfExists"}
	ignore := method("ignore", opts, ret(&hir.Expr{Kind: hir.New, Type: opts, OwnKeys: &ignoreKeys, Args: []*hir.Expr{hir.L(ob, nil), hir.L(ob, false)}}))
	ignore.Static = true
	pos := method("position", dyn, ret(rt("dynamic.of", &hir.Expr{Kind: hir.New, Type: position, Op: "lsp.Position.create", Args: []*hir.Expr{hir.L(hir.T(hir.Number), float64(2)), hir.L(hir.T(hir.Number), float64(3))}}, dyn)))
	pos.Static = true
	unknown := method("unknown", dyn, ret(rt("dynamic.of", &hir.Expr{Kind: hir.New, Type: rename, Args: []*hir.Expr{hir.L(str, "rename"), hir.L(str, "old"), hir.L(str, "new"), hir.L(oo, nil), hir.L(annotationType, nil)}}, dyn)))
	unknown.Static = true
	keys := method("keys", str, ret(rt("array.join", rt("dynamic.keys", local("input", dyn), hir.T(hir.Array, str)), str, hir.L(str, ","))))
	keys.Static = true
	keys.Params = []hir.Param{{Name: "input", Type: dyn}}
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{optClass, renameClass, posClass, {Name: "FactoryProbe", Methods: []*hir.Method{create, empty, flag, ignore, pos, unknown, keys}}}})
	if err != nil {
		t.Fatal(err)
	}
	root := os.Getenv("ABAPITI_TESTS_UPSTREAM")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		root = filepath.Join(home, "dev", "abaplint")
	}
	pkg := filepath.Join(root, "packages", "core", "node_modules", "vscode-languageserver-types")
	if _, err := os.Stat(filepath.Join(pkg, "lib", "umd", "main.js")); err != nil {
		pkg = filepath.Join(root, "node_modules", "vscode-languageserver-types")
	}
	if _, err := os.Stat(filepath.Join(pkg, "lib", "umd", "main.js")); err == nil {
		script := `if(require(process.argv[2]).version!=='3.18.0')throw Error('wrong LSP pin');const L=require(process.argv[1]);for(const [o,a] of [[undefined,undefined],[{},undefined],[{overwrite:false},''],[{ignoreIfExists:false},'id']]){const r=L.RenameFile.create('old','new',o,a);console.log(Object.keys(r).join(','));}console.log(Object.keys(L.Range.create(1,2,3,4).start).join(','));`
		out, err := exec.Command("node", "-e", script, pkg, filepath.Join(pkg, "package.json")).CombinedOutput()
		expected := "kind,oldUri,newUri\nkind,oldUri,newUri\nkind,oldUri,newUri,options,annotationId\nkind,oldUri,newUri,options,annotationId\nline,character\n"
		if err != nil || string(out) != expected {
			t.Fatalf("Node: %s %v", out, err)
		}
	} else {
		t.Log("installed LSP Node runtime unavailable; emitted ABAP still exercised")
	}
	owner := names.Get("FactoryProbe")
	d := names.Get("runtime.dynamic")
	var unit string
	unit = fmt.Sprintf(`CLASS ltcl_factory DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS check FOR TESTING.
ENDCLASS.
CLASS ltcl_factory IMPLEMENTATION.
METHOD check.
DATA input TYPE REF TO %s.
DATA options TYPE REF TO %s.
DATA annotation TYPE REF TO %s.
DATA value TYPE REF TO %s.
DATA refused TYPE abap_bool.
input = %s=>%s( options = options annotation = annotation ).
cl_abap_unit_assert=>assert_equals( act = %s=>%s( input ) exp = 'kind,oldUri,newUri' ).
cl_abap_unit_assert=>assert_false( input->has( 'options' ) ).
value = input->get( 'textDocument' ).
cl_abap_unit_assert=>assert_false( xsdbool( value IS BOUND ) ).
value = input->get( 'oldUri' ).
cl_abap_unit_assert=>assert_equals( act = value->as_string( ) exp = 'old' ).
options = %s=>%s( ).
input = %s=>%s( options = options annotation = annotation ).
cl_abap_unit_assert=>assert_false( input->has( 'options' ) ).
value = input->get( 'options' ).
cl_abap_unit_assert=>assert_false( xsdbool( value IS BOUND ) ).
options = %s=>%s( ).
annotation = NEW #( ).
annotation->has = abap_true.
input = %s=>%s( options = options annotation = annotation ).
cl_abap_unit_assert=>assert_equals( act = %s=>%s( input ) exp = 'kind,oldUri,newUri,options,annotationId' ).
value = input->get( 'options' ).
cl_abap_unit_assert=>assert_true( xsdbool( value->oval = options ) ).
value = value->get( 'overwrite' ).
cl_abap_unit_assert=>assert_true( xsdbool( value IS BOUND ) ).
cl_abap_unit_assert=>assert_false( value->as_boolean( ) ).
value = input->get( 'annotationId' ).
cl_abap_unit_assert=>assert_equals( act = value->as_string( ) exp = '' ).
options = %s=>%s( ).
annotation->value = 'id'.
input = %s=>%s( options = options annotation = annotation ).
cl_abap_unit_assert=>assert_equals( act = %s=>%s( input ) exp = 'kind,oldUri,newUri,options,annotationId' ).
value = input->get( 'options' ).
value = value->get( 'ignoreIfExists' ).
cl_abap_unit_assert=>assert_true( xsdbool( value IS BOUND ) ).
cl_abap_unit_assert=>assert_false( value->as_boolean( ) ).
input = %s=>%s( ).
cl_abap_unit_assert=>assert_equals( act = %s=>%s( input ) exp = 'line,character' ).
value = input->get( 'character' ).
cl_abap_unit_assert=>assert_equals( act = value->as_number( ) exp = 3 ).
input = %s=>%s( ).
TRY.
value = input->get( 'oldUri' ).
CATCH %s.
refused = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_true( refused ).
ENDMETHOD.
ENDCLASS.
`, d, names.Get(prefix+"RenameFileOptions"), names.Get("runtime."+annotationType.String()), d, owner, names.Get("member.create"), owner, names.Get("member.keys"), owner, names.Get("member.empty"), owner, names.Get("member.create"), owner, names.Get("member.flag"), owner, names.Get("member.create"), owner, names.Get("member.keys"), owner, names.Get("member.ignore"), owner, names.Get("member.create"), owner, names.Get("member.keys"), owner, names.Get("member.position"), owner, names.Get("member.keys"), owner, names.Get("member.unknown"), names.Get("view.refusal"))
	files[owner+".clas.testclasses.abap"] = strings.ReplaceAll(unit, "options = options annotation = annotation", names.Get("param.options")+" = options "+names.Get("param.annotation")+" = annotation")
	storageHostFixture(t, files, owner)
}
