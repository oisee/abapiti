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

// The union layout must test the array domain before using the boolean temp.
// F2 exposed body.line folding the whole IF block into CONV abap_bool(...).
func TestStructuralUnionArrayLayout(t *testing.T) {
	dyn := hir.T(hir.Dynamic)
	arr := hir.T(hir.Array, str)
	union := hir.Type{Kind: hir.InterfaceRef, Name: "ArrayPayload"}
	members := []hir.StructuralMember{}
	classes := []*hir.Class{}
	for _, name := range []string{"FirstPayload", "SecondPayload"} {
		fields := []hir.StructuralField{
			{Name: "kind", Type: str, Domains: []hir.StructuralDomain{{Kind: hir.String, Exact: true, Literal: name}}},
			{Name: "locals", Type: arr, Domains: []hir.StructuralDomain{{Kind: hir.Array}}},
		}
		members = append(members, hir.StructuralMember{Type: hir.Ref(name), Fields: fields})
		classes = append(classes, &hir.Class{Name: name, Structural: true, Implements: []string{union.Name}, Fields: []hir.Field{{Name: "kind", Type: str}, {Name: "locals", Type: arr}}})
	}
	container := hir.Ref("ArrayContainer")
	classes = append(classes, &hir.Class{Name: container.Name, Structural: true, Fields: []hir.Field{{Name: "payload", Type: union}}})
	view := method("view", container, ret(rt("dynamic.classView", local("input", dyn), container)))
	view.Static, view.Params = true, []hir.Param{{Name: "input", Type: dyn}}
	parse := method("parse", dyn, ret(rt("json.parseSubset", local("input", str), dyn)))
	parse.Static, parse.Params = true, []hir.Param{{Name: "input", Type: str}}
	box := method("box", dyn, ret(rt("dynamic.of", local("input", arr), dyn)))
	box.Static, box.Params = true, []hir.Param{{Name: "input", Type: arr}}
	classes = append(classes, &hir.Class{Name: "ArrayLayoutProbe", Methods: []*hir.Method{view, parse, box}})
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: classes, Interfaces: []*hir.Interface{{Name: union.Name, StructuralMembers: members}}})
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("ArrayLayoutProbe")
	layout := names.Get("view.union." + union.Name)
	src := files[layout+".clas.abap"]
	if !strings.Contains(src, "`locals`") || !strings.Contains(src, "IS INSTANCE OF "+names.Get("runtime.arraymark")) {
		t.Fatalf("array union layout was not emitted: %s", src)
	}
	for _, source := range files {
		if strings.Contains(source, "CONV abap_bool( abap_false.") {
			t.Fatal("array statement block embedded in CONV", source)
		}
	}
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf(`CLASS ltcl_array_layout DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS check FOR TESTING.
ENDCLASS.
CLASS ltcl_array_layout IMPLEMENTATION.
METHOD check.
DATA payload TYPE REF TO %s.
DATA native TYPE REF TO %s.
DATA accepted TYPE REF TO %s.
DATA refused TYPE abap_bool.
payload = %s=>parse( %s ).
accepted = %s=>decode( payload ).
cl_abap_unit_assert=>assert_bound( accepted ).
payload = %s=>parse( %s ).
native = NEW #( ).
payload->put( p0 = 'locals' p1 = %s=>%s( native ) ).
accepted = %s=>decode( payload ).
cl_abap_unit_assert=>assert_bound( accepted ).
payload = %s=>parse( %s ).
TRY.
accepted = %s=>decode( payload ).
CATCH %s.
refused = abap_true.
ENDTRY.
cl_abap_unit_assert=>assert_true( refused ).
ENDMETHOD.
ENDCLASS.
`, names.Get("runtime.dynamic"), names.Get("runtime."+arr.String()), names.Get(union.Name),
		names.Get("runtime.jsonSubset"), "`{\"kind\":\"FirstPayload\",\"locals\":[\"one\"]}`", layout,
		names.Get("runtime.jsonSubset"), "`{\"kind\":\"SecondPayload\"}`", owner, names.Get("member.box"), layout,
		names.Get("runtime.jsonSubset"), "`{\"kind\":\"FirstPayload\",\"locals\":{}}`", layout, names.Get("view.refusal"))
	// Enable the existing syntax and OSG harnesses together. Emission and the
	// compile-gate fixture collection still run without the optional checkout.
	if osg := os.Getenv("ABAPITI_OSG_HOME"); osg != "" {
		dir := t.TempDir()
		for name, source := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
		}
		// Use the same v750 parser_error/check_syntax rules as hir-lint.mjs.
		out, err := exec.Command("node", "--input-type=module", "-e", `
import {createRequire} from 'node:module';
import {readFileSync,readdirSync} from 'node:fs';
import {join,resolve} from 'node:path';
const [dir,osg] = process.argv.slice(1);
const lint = createRequire(join(resolve(osg),'package.json'))('@abaplint/core');
const config = lint.Config.getDefault();
config.get().syntax.version = lint.Version.v750;
config.get().syntax.errorNamespace = '.*';
config.get().rules = {parser_error:true,check_syntax:true};
const reg = new lint.Registry(config);
for (const name of readdirSync(dir)) reg.addFile(new lint.MemoryFile(name,readFileSync(join(dir,name),'utf8')));
function deps(dir) {
 for (const e of readdirSync(dir,{withFileTypes:true})) {
  const path = join(dir,e.name);
  if (e.isDirectory()) deps(path);
  else if (/\.(abap|xml)$/.test(e.name) && !e.name.includes('testclasses')) reg.addDependency(new lint.MemoryFile(path,readFileSync(path,'utf8')));
 }
}
deps(join(osg,'.local/lars/open-abap-core/src'));
reg.parse();
const issues = reg.findIssues();
for (const i of issues) console.error(i.getFilename()+':'+i.getStart().getRow()+': '+i.getMessage());
console.log('ABAP v750: '+issues.length+' issues');
process.exitCode = issues.length ? 1 : 0;
`, dir, osg).CombinedOutput()
		if err != nil {
			t.Fatalf("abaplint: %v\n%s", err, out)
		}
		t.Logf("%s", out)
	}
	storageHostFixture(t, files, owner)
}
