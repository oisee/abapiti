package abap

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/abapfixture"
)

func deleteDispatchProgram() *hir.Program {
	remove := method("remove", boolean, ret(rt("dynamic.delete", local("obj", hir.T(hir.Dynamic)), boolean, local("key", str))))
	remove.Static = true
	remove.Params = []hir.Param{{Name: "obj", Type: hir.T(hir.Dynamic)}, {Name: "key", Type: str}}
	return &hir.Program{Classes: []*hir.Class{
		{Name: "DeleteBase", Fields: []hir.Field{{Name: "baseOptional", Type: hir.T(hir.Optional, str)}, {Name: "baseRequired", Type: str}, {Name: "toString", Type: hir.T(hir.Optional, str)}, {Name: "staticOnly", Type: str, Static: true}}},
		{Name: "DeleteMiddle", Super: "DeleteBase", Fields: []hir.Field{{Name: "middleOptional", Type: hir.T(hir.Optional, str)}, {Name: "middleRequired", Type: str}}},
		{Name: "DeleteLeaf", Super: "DeleteMiddle", Fields: []hir.Field{{Name: "leafDynamic", Type: hir.T(hir.Dynamic)}, {Name: "leafRequired", Type: str}, {Name: "__proto__", Type: hir.T(hir.Optional, str)}}},
		{Name: "DeleteCaller", Methods: []*hir.Method{remove}},
	}}
}

func deleteMethod(source, name string) string {
	start := strings.Index(source, "METHOD "+name+".")
	if start < 0 {
		return ""
	}
	source = source[start:]
	end := strings.Index(source, "ENDMETHOD.")
	if end < 0 {
		return ""
	}
	return source[:end+len("ENDMETHOD.")]
}

func deleteUnindent(source string) string {
	lines := strings.Split(strings.TrimSpace(source), "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.Join(lines, "\n")
}

func TestNativeDeleteVirtualMatchesLegacy(t *testing.T) {
	testNativeDeleteVirtualMatchesLegacy(t, deleteDispatchProgram())
}

func TestNativeDeleteVirtualRuntimeAncestor(t *testing.T) {
	for _, fields := range []bool{false, true} {
		for _, leaf := range []bool{false, true} {
			t.Run(fmt.Sprintf("fields_%t/leaf_%t", fields, leaf), func(t *testing.T) {
				p := deleteDispatchProgram()
				p.Classes[0] = &hir.Class{Name: "builtin.Error"}
				if fields {
					p.Classes[0].Fields = []hir.Field{{Name: "message", Type: str}}
				}
				p.Classes[1].Super = "builtin.Error"
				// A reference activates the runtime replacement of builtin.Error.
				p.Classes[3].Methods[0].Params = append(p.Classes[3].Methods[0].Params, hir.Param{Name: "error", Type: hir.Ref("builtin.Error")})
				if !leaf {
					p.Classes = append(p.Classes[:2], p.Classes[3])
				}
				testNativeDeleteVirtualMatchesLegacy(t, p)
			})
		}
	}
}

func testNativeDeleteVirtualMatchesLegacy(t *testing.T, p *hir.Program) {
	t.Helper()
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	e := &emitter{p: p, names: names, files: files, types: map[string]bool{}}
	legacy := e.legacyNativeDeletePart(false)
	legacyCode := files[legacy+".clas.abap"]
	iface := names.Get("runtime.nativeDeletable")
	for i, c := range e.classesByDepth() {
		if c.Name == "builtin.Error" {
			if strings.Contains(files[names.Get(c.Name)+".clas.abap"], iface) {
				t.Fatal("runtime ancestor unexpectedly implements delete interface")
			}
			continue
		}
		start := strings.Index(legacyCode, "native"+fmt.Sprint(i)+" ?= p0.\n")
		if start < 0 {
			t.Fatal("missing legacy arm", c.Name)
		}
		arm := legacyCode[start+len("native"+fmt.Sprint(i)+" ?= p0.\n"):]
		arm = arm[:strings.Index(arm, "RETURN.\nENDIF.")]
		arm = strings.ReplaceAll(arm, fmt.Sprintf("native%d->", i), "me->")
		expected, err := Format("METHOD " + iface + "~delete_key.\n" + arm + "ENDMETHOD.\n")
		if err != nil {
			t.Fatal(err)
		}
		got := deleteMethod(files[names.Get(c.Name)+".clas.abap"], iface+"~delete_key")
		if deleteUnindent(expected) != deleteUnindent(got) {
			t.Fatalf("%s differs from legacy arm\nwant:\n%s\ngot:\n%s", c.Name, expected, got)
		}
		decl := "INTERFACES " + iface + "."
		if c.Super != "" && c.Super != "builtin.Error" {
			decl = "METHODS " + iface + "~delete_key REDEFINITION."
		}
		if !strings.Contains(files[names.Get(c.Name)+".clas.abap"], decl) {
			t.Fatal("missing virtual declaration", c.Name)
		}
	}
	run := names.Get("runtime.nativeDelete")
	var unit strings.Builder
	unit.WriteString("CLASS ltcl_delete DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_delete IMPLEMENTATION.\nMETHOD check.\nDATA a TYPE REF TO object.\nDATA b TYPE REF TO object.\nDATA new_result TYPE abap_bool.\nDATA old_result TYPE abap_bool.\nDATA new_refused TYPE abap_bool.\nDATA old_refused TYPE abap_bool.\n")
	for ci, c := range p.Classes {
		if c.Name == "builtin.Error" {
			continue
		}
		n := fmt.Sprintf("c%d", ci)
		typ := names.Get(c.Name)
		fmt.Fprintf(&unit, "DATA new_%s TYPE REF TO %s.\nDATA old_%s TYPE REF TO %s.\n", n, typ, n, typ)
		for _, key := range []string{"baseOptional", "middleOptional", "leafDynamic", "baseRequired", "middleRequired", "leafRequired", "toString", "__proto__", "absent", "staticOnly", "message"} {
			fmt.Fprintf(&unit, "CREATE OBJECT new_%s.\nCREATE OBJECT old_%s.\na = new_%s.\nb = old_%s.\n", n, n, n, n)
			var optional []string
			for base := c; base != nil; base = e.classBy(base.Super) {
				for _, f := range base.Fields {
					if !f.Static && (f.Type.Kind == hir.Optional || f.Type.Kind == hir.Dynamic) {
						member := names.Get("member." + f.Name)
						optional = append(optional, member)
						fmt.Fprintf(&unit, "CREATE OBJECT new_%s->%s.\nold_%s->%s = new_%s->%s.\n", n, member, n, member, n, member)
					}
				}
			}
			unit.WriteString("DO 2 TIMES.\nCLEAR new_refused.\nCLEAR old_refused.\n")
			for _, v := range []struct{ prefix, receiver, runtime string }{{"new", "a", run}, {"old", "b", legacy}} {
				fmt.Fprintf(&unit, "TRY.\n%s_result = %s=>run( p0 = %s key = `%s` ).\nCATCH %s.\n%s_refused = abap_true.\nENDTRY.\n", v.prefix, v.runtime, v.receiver, key, names.Get("view.refusal"), v.prefix)
			}
			unit.WriteString("cl_abap_unit_assert=>assert_equals( exp = old_refused act = new_refused ).\ncl_abap_unit_assert=>assert_equals( exp = old_result act = new_result ).\n")
			mask := names.Get("record.deleted")
			fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( exp = old_%s->%s act = new_%s->%s ).\n", n, mask, n, mask)
			for _, member := range optional {
				fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( exp = old_%s->%s act = new_%s->%s ).\n", n, member, n, member)
			}
			unit.WriteString("ENDDO.\n")
		}
	}
	for _, setup := range []string{"CLEAR a.", "a = NEW " + names.Get("runtime.dynamic") + "( )."} {
		unit.WriteString(setup + "\nCLEAR new_refused.\nCLEAR old_refused.\n")
		for _, v := range []struct{ prefix, runtime string }{{"new", run}, {"old", legacy}} {
			fmt.Fprintf(&unit, "TRY.\n%s_result = %s=>run( p0 = a key = `absent` ).\nCATCH %s.\n%s_refused = abap_true.\nENDTRY.\n", v.prefix, v.runtime, names.Get("view.refusal"), v.prefix)
		}
		unit.WriteString("cl_abap_unit_assert=>assert_true( new_refused ).\ncl_abap_unit_assert=>assert_equals( exp = old_refused act = new_refused ).\n")
	}
	unit.WriteString("ENDMETHOD.\nENDCLASS.\n")
	owner := names.Get("DeleteCaller")
	files[owner+".clas.testclasses.abap"] = unit.String()
	if issues := abapfixture.KernelViolations(files); len(issues) != 0 {
		t.Fatal(issues)
	}
	nativeDeleteSyntaxFixture(t, files)
	storageHostFixture(t, files, owner)
	t.Logf("hierarchy fixture: legacy run %d lines, virtual run %d lines", strings.Count(deleteMethod(legacyCode, "run"), "\n")+1, strings.Count(deleteMethod(files[run+".clas.abap"], "run"), "\n")+1)
}

func TestNativeDeleteVirtualSize(t *testing.T) {
	p := deleteDispatchProgram()
	p.Classes = p.Classes[3:]
	for i := 0; i < 512; i++ {
		c := &hir.Class{Name: fmt.Sprintf("DeleteSize%d", i)}
		for j := 0; j < 4; j++ {
			c.Fields = append(c.Fields, hir.Field{Name: fmt.Sprintf("field%d", j), Type: hir.T(hir.Optional, str)})
		}
		p.Classes = append(p.Classes, c)
	}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	run := deleteMethod(files[names.Get("runtime.nativeDelete")+".clas.abap"], "run")
	if lines := strings.Count(run, "\n") + 1; lines >= 40 {
		t.Fatalf("run: %d lines", lines)
	}
	maxLines := 0
	for file, source := range files {
		for _, line := range strings.Split(source, "\n") {
			if len(line) > 255 {
				t.Fatalf("%s: line exceeds 255", file)
			}
		}
		for _, method := range regexp.MustCompile(`(?ms)^\s*METHOD [^\n]+\n.*?ENDMETHOD\.`).FindAllString(source, -1) {
			lines := strings.Count(strings.TrimSpace(method), "\n") + 1
			if lines > maxLines {
				maxLines = lines
			}
			if lines > 200 {
				t.Fatalf("%s: method has %d lines", file, lines)
			}
		}
	}
	e := &emitter{p: p, names: names, files: map[string]string{}, types: map[string]bool{}}
	legacy := e.legacyNativeDeletePart(false)
	before, _ := Format(e.files[legacy+".clas.abap"])
	t.Logf("512 x 4 fixture: legacy run %d lines, virtual run %d lines, largest emitted method %d lines", strings.Count(deleteMethod(before, "run"), "\n")+1, strings.Count(run, "\n")+1, maxLines)
}

func TestNativeDeleteVirtualUsageGate(t *testing.T) {
	p := deleteDispatchProgram()
	p.Classes = p.Classes[:3]
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	for file, source := range files {
		if strings.Contains(source, "delete_key") || file == names.Get("runtime.nativeDeletable")+".intf.abap" {
			t.Fatal("unused delete support", file)
		}
	}
}

func TestNativeDeleteVirtualTestPackages(t *testing.T) {
	for _, runtimeRoot := range []bool{false, true} {
		t.Run(fmt.Sprintf("runtimeRoot_%t", runtimeRoot), func(t *testing.T) {
			testNativeDeleteVirtualTestPackages(t, runtimeRoot)
		})
	}
}

func testNativeDeleteVirtualTestPackages(t *testing.T, runtimeRoot bool) {
	p := deleteDispatchProgram()
	if runtimeRoot {
		p.Classes = append(p.Classes, &hir.Class{Name: "builtin.Error", Fields: []hir.Field{{Name: "message", Type: str}}})
		p.Classes = append(p.Classes,
			&hir.Class{Name: "test/RuntimeMiddle", Super: "builtin.Error", Fields: []hir.Field{{Name: "runtimeOptional", Type: hir.T(hir.Optional, str)}}},
			&hir.Class{Name: "test/RuntimeLeaf", Super: "test/RuntimeMiddle", Fields: []hir.Field{{Name: "runtimeDynamic", Type: hir.T(hir.Dynamic)}}},
		)
		p.Classes[0].Super = "builtin.Error"
		p.Classes[3].Methods[0].Params = append(p.Classes[3].Methods[0].Params, hir.Param{Name: "error", Type: hir.Ref("builtin.Error")})
	}
	p.Classes[1].Name = "test/DeleteMiddle"
	p.Classes[2].Name = "test/DeleteLeaf"
	p.Classes[2].Super = p.Classes[1].Name
	make := method("make", hir.Ref(hir.RootObject), ret(rt("classvalue.newArgs", local("ctor", hir.T(hir.ClassValue)), hir.Ref(hir.RootObject), local("args", hir.T(hir.Array, hir.T(hir.Dynamic))))))
	make.Static = true
	make.Params = []hir.Param{{Name: "ctor", Type: hir.T(hir.ClassValue)}, {Name: "args", Type: hir.T(hir.Array, hir.T(hir.Dynamic))}}
	p.Classes[3].Methods = append(p.Classes[3].Methods, make)
	files, names, err := EmitTestPackageNamed(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	coreInterface := names.Get("runtime.nativeDeletable")
	for _, c := range p.Classes {
		if c.Name == "builtin.Error" {
			continue
		}
		iface := coreInterface
		if testDeclaration(c) {
			iface = "zif_lnt_test_delete"
		}
		source := files[names.Get(c.Name)+".clas.abap"]
		if deleteMethod(source, iface+"~delete_key") == "" {
			t.Fatal("missing partition implementation", c.Name)
		}
		decl := "INTERFACES " + iface + "."
		if c.Name == "test/DeleteLeaf" || c.Name == "test/RuntimeLeaf" {
			decl = "METHODS " + iface + "~delete_key REDEFINITION."
		}
		if !strings.Contains(source, decl) {
			t.Fatal("missing partition declaration", c.Name, decl)
		}
	}
	core := names.Get("runtime.nativeDelete")
	for _, c := range p.Classes[:3] {
		if strings.Contains(files[core+".clas.abap"], names.Get(c.Name)) {
			t.Fatal("class probe in dispatcher", c.Name)
		}
	}
	if strings.Contains(files[core+".clas.abap"], "zif_lnt_test_delete") {
		t.Fatal("core depends on test interface")
	}
	testCode := files["zcl_lnt_test_delete.clas.abap"]
	if !strings.Contains(testCode, "IS INSTANCE OF zif_lnt_test_delete") {
		t.Fatal("missing test dispatch")
	}
	e := &emitter{p: p, names: names, files: files, types: map[string]bool{}, testPackages: true}
	legacy := e.legacyNativeDeletePart(false)
	legacyTest := e.legacyNativeDeletePart(true)
	owner := names.Get("DeleteCaller")
	leaf := names.Get("test/DeleteLeaf")
	base := names.Get("DeleteBase")
	optional := names.Get("member.middleOptional")
	mask := names.Get("record.deleted")
	factory := names.Get("runtime.classvalue.factory")
	var unit strings.Builder
	unit.WriteString("CLASS ltcl_delete DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_delete IMPLEMENTATION.\nMETHOD check.\nDATA new_result TYPE abap_bool.\nDATA old_result TYPE abap_bool.\nDATA new_handled TYPE abap_bool.\nDATA old_handled TYPE abap_bool.\n")
	fmt.Fprintf(&unit, "DATA current TYPE REF TO %s.\nDATA previous TYPE REF TO %s.\nDATA core TYPE REF TO %s.\nCREATE OBJECT core.\nCREATE OBJECT current.\nCREATE OBJECT previous.\nCREATE OBJECT current->%s.\nprevious->%s = current->%s.\nCLEAR %s=>extension.\n", leaf, leaf, base, optional, optional, optional, factory)
	for _, key := range []string{"middleOptional", "leafRequired", "absent"} {
		fmt.Fprintf(&unit, "new_result = %s=>run( p0 = current key = `%s` ).\nold_result = %s=>run( p0 = previous key = `%s` ).\ncl_abap_unit_assert=>assert_equals( exp = old_result act = new_result ).\n", core, key, legacy, key)
	}
	fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_bound( current->%s ).\ncl_abap_unit_assert=>assert_equals( exp = previous->%s act = current->%s ).\n", optional, mask, mask)
	for _, receiver := range []string{"current", "core"} {
		fmt.Fprintf(&unit, "CLEAR new_handled.\nCLEAR old_handled.\nzcl_lnt_test_delete=>run( EXPORTING p0 = %s key = `middleOptional` IMPORTING handled = new_handled result = new_result ).\n%s=>run( EXPORTING p0 = %s key = `middleOptional` IMPORTING handled = old_handled result = old_result ).\ncl_abap_unit_assert=>assert_equals( exp = old_handled act = new_handled ).\ncl_abap_unit_assert=>assert_equals( exp = old_result act = new_result ).\n", receiver, legacyTest, strings.ReplaceAll(receiver, "current", "previous"))
	}
	if runtimeRoot {
		unit.WriteString("DATA new_refused_runtime TYPE abap_bool.\nDATA old_refused_runtime TYPE abap_bool.\n")
		for _, class := range []string{"test/RuntimeMiddle", "test/RuntimeLeaf"} {
			n := names.Get(class)
			fmt.Fprintf(&unit, "DATA current_%s TYPE REF TO %s.\nDATA previous_%s TYPE REF TO %s.\nCREATE OBJECT current_%s.\nCREATE OBJECT previous_%s.\n", strings.TrimPrefix(class, "test/"), n, strings.TrimPrefix(class, "test/"), n, strings.TrimPrefix(class, "test/"), strings.TrimPrefix(class, "test/"))
			receiver := "current_" + strings.TrimPrefix(class, "test/")
			previous := "previous_" + strings.TrimPrefix(class, "test/")
			for _, key := range []string{"runtimeOptional", "runtimeDynamic", "message", "absent"} {
				unit.WriteString("CLEAR new_refused_runtime.\nCLEAR old_refused_runtime.\n")
				for _, v := range []struct{ prefix, receiver, runtime string }{{"new", receiver, "zcl_lnt_test_delete"}, {"old", previous, legacyTest}} {
					fmt.Fprintf(&unit, "TRY.\n%s=>run( EXPORTING p0 = %s key = `%s` IMPORTING handled = %s_handled result = %s_result ).\nCATCH %s.\n%s_refused_runtime = abap_true.\nENDTRY.\n", v.runtime, v.receiver, key, v.prefix, v.prefix, names.Get("view.refusal"), v.prefix)
				}
				unit.WriteString("cl_abap_unit_assert=>assert_equals( exp = old_refused_runtime act = new_refused_runtime ).\ncl_abap_unit_assert=>assert_equals( exp = old_result act = new_result ).\n")
				fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( exp = %s->%s act = %s->%s ).\n", previous, mask, receiver, mask)
			}
		}
	}
	unit.WriteString("DATA new_refused TYPE abap_bool.\nDATA old_refused TYPE abap_bool.\nCLEAR new_handled.\nCLEAR old_handled.\n")
	for _, v := range []struct{ prefix, receiver, runtime string }{{"new", "current", "zcl_lnt_test_delete"}, {"old", "previous", legacyTest}} {
		fmt.Fprintf(&unit, "TRY.\n%s=>run( EXPORTING p0 = %s key = `baseRequired` IMPORTING handled = %s_handled result = %s_result ).\nCATCH %s.\n%s_refused = abap_true.\nENDTRY.\n", v.runtime, v.receiver, v.prefix, v.prefix, names.Get("view.refusal"), v.prefix)
	}
	unit.WriteString("cl_abap_unit_assert=>assert_true( new_refused ).\ncl_abap_unit_assert=>assert_equals( exp = old_refused act = new_refused ).\ncl_abap_unit_assert=>assert_equals( exp = old_handled act = new_handled ).\n")
	fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_false( xsdbool( current->%s IS BOUND ) ).\ncl_abap_unit_assert=>assert_equals( exp = previous->%s act = current->%s ).\n%s=>extension = NEW zcl_lnt_test_extension( ).\ncl_abap_unit_assert=>assert_true( %s=>run( p0 = current key = `absent` ) ).\nCLEAR %s=>extension.\nENDMETHOD.\nENDCLASS.\n", optional, mask, mask, factory, core, factory)
	files[owner+".clas.testclasses.abap"] = unit.String()
	nativeDeleteSyntaxFixture(t, files)
	storageHostFixture(t, files, owner)
}

// Use the same abaplint syntax rules and library dependencies as OSG, but
// check every emitted class: gogen can otherwise omit a broken unused class.
func nativeDeleteSyntaxFixture(t *testing.T, files map[string]string) {
	t.Helper()
	osg := os.Getenv("ABAPITI_OSG_HOME")
	if osg == "" {
		return // Like storageHostFixture, external frontend checks are opt-in.
	}
	const script = `
const {createRequire} = require("node:module");
const {readFileSync, readdirSync} = require("node:fs");
const {join} = require("node:path");
const osg = process.argv[1];
const lint = createRequire(join(osg, "package.json"))("@abaplint/core");
const config = lint.Config.getDefault().get();
config.syntax = {...config.syntax, version: "v758", errorNamespace: "."};
config.rules = {check_syntax: true, parser_error: true};
const reg = new lint.Registry(new lint.Config(JSON.stringify(config)));
for (const [name, source] of Object.entries(JSON.parse(readFileSync(0, "utf8")))) {
  reg.addFile(new lint.MemoryFile(name, source));
}
function dependencies(dir) {
  for (const entry of readdirSync(dir, {withFileTypes: true})) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) dependencies(path);
    else if (/\.(abap|xml)$/.test(entry.name) && !entry.name.includes("testclasses")) {
      reg.addDependency(new lint.MemoryFile(path, readFileSync(path, "utf8")));
    }
  }
}
dependencies(join(process.env.OSD_LIB_OPEN_ABAP_CORE || join(osg, ".local/lars/open-abap-core"), "src"));
reg.parse();
const issues = reg.findIssues();
for (const issue of issues) console.error(issue.getFilename() + ": " + issue.getMessage());

process.exitCode = issues.length ? 1 : 0;
`
	input, err := json.Marshal(files)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "-e", script, osg)
	cmd.Stdin = strings.NewReader(string(input))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("abaplint syntax: %v\n%s", err, out)
	}
	t.Logf("abaplint syntax: %d files, 0 issues", len(files))
}
