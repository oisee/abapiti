package abap

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestLSPGuardFixtures(t *testing.T) {
	dyn := hir.T(hir.Dynamic)
	methods := []*hir.Method{}
	for _, op := range []string{"isTextDocumentEdit", "isRenameFile", "isCreateFile", "isDeleteFile"} {
		args := []*hir.Expr{}
		if op != "isTextDocumentEdit" {
			args = append(args, local("condition", boolean))
		}
		m := method(op, boolean, hir.B(decl("d", dyn, rt("json.parseSubset", local("input", str), dyn)), ret(rt("lsp."+op, local("d", dyn), boolean, args...))))
		m.Static = true
		m.Params = []hir.Param{{Name: "input", Type: str}, {Name: "condition", Type: boolean}}
		methods = append(methods, m)
	}
	files, names, err := EmitNamed(&hir.Program{Classes: []*hir.Class{{Name: "LSPGuards", Methods: methods}}})
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("LSPGuards")
	var unit strings.Builder
	fmt.Fprint(&unit, "CLASS ltcl_lsp DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_lsp IMPLEMENTATION.\nMETHOD check.\nDATA result TYPE abap_bool.\nDATA refused TYPE abap_bool.\n")
	cases := []struct {
		op, input string
		want      bool
	}{
		{"isTextDocumentEdit", `{"textDocument":{"uri":"u","version":null},"edits":[]}`, true},
		{"isTextDocumentEdit", `{"textDocument":{"uri":"u","version":0.5},"edits":[]}`, true},
		{"isTextDocumentEdit", `{"textDocument":{"uri":"u","version":2147483648},"edits":[]}`, false},
		{"isTextDocumentEdit", `{"textDocument":{"uri":"u"},"edits":[]}`, false},
		{"isTextDocumentEdit", `{"textDocument":{"uri":3,"version":0},"edits":[]}`, false},
		{"isTextDocumentEdit", `{"textDocument":{"uri":"u","version":0},"edits":{}}`, false},
		{"isRenameFile", `{"kind":"rename","oldUri":"u","newUri":"v","options":{"overwrite":false},"annotationId":"a"}`, true},
		{"isRenameFile", `{"kind":"rename","oldUri":"u","newUri":"v","options":{"ignoreIfExists":1}}`, false},
		{"isRenameFile", `{"kind":"rename","oldUri":"u","newUri":"v","annotationId":null}`, false},
		{"isRenameFile", `{"kind":"create","oldUri":"u","newUri":"v"}`, false},
		{"isCreateFile", `{"kind":"create","uri":"u","options":{"overwrite":true,"ignoreIfExists":false}}`, true},
		{"isCreateFile", `{"kind":"create","uri":"u","options":{"overwrite":"yes"}}`, false},
		{"isDeleteFile", `{"kind":"delete","uri":"u","options":{"recursive":true,"ignoreIfNotExists":false}}`, true},
		{"isDeleteFile", `{"kind":"delete","uri":"u","options":{"ignoreIfNotExists":3}}`, false},
	}
	// Resolve the same pinned installation used by frontend oracle fixtures.
	home, _ := os.UserHomeDir()
	pkg := filepath.Join(home, "dev/abaplint/packages/core/node_modules/vscode-languageserver-types")
	if _, err := os.Stat(pkg); err != nil {
		pkg = filepath.Join(home, "dev/abaplint/node_modules/vscode-languageserver-types")
	}
	_, nodeRuntimeErr := os.Stat(filepath.Join(pkg, "lib/umd/main.js"))
	if nodeRuntimeErr != nil {
		t.Log("installed LSP Node runtime unavailable; ABAP expectations still exercised")
	}
	for _, c := range cases {
		if nodeRuntimeErr == nil {
			code := fmt.Sprintf("if(require(%q).version!=='3.18.0')throw Error('wrong LSP pin');const L=require(%q);console.log(L.%s.is(%s));", filepath.Join(pkg, "package.json"), pkg, strings.TrimPrefix(c.op, "is"), c.input)
			out, err := exec.Command("node", "-e", code).CombinedOutput()
			if err != nil || string(out) != fmt.Sprintln(c.want) {
				t.Fatalf("Node %s %s: %s %v", c.op, c.input, out, err)
			}
		}
		want := "abap_false"
		if c.want {
			want = "abap_true"
		}
		fmt.Fprintf(&unit, "result = %s=>%s( input = `%s` condition = abap_false ).\ncl_abap_unit_assert=>assert_equals( act = result exp = %s ).\n", owner, names.Get("member."+c.op), c.input, want)
	}
	for _, op := range []string{"isRenameFile", "isCreateFile", "isDeleteFile"} {
		for _, input := range []string{"null", "0", `""`} {
			fmt.Fprintf(&unit, "result = %s=>%s( input = `%s` condition = abap_true ).\ncl_abap_unit_assert=>assert_false( result ).\nCLEAR refused.\nTRY.\nresult = %s=>%s( input = `%s` condition = abap_false ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", owner, names.Get("member."+op), input, owner, names.Get("member."+op), input, names.Get("view.refusal"))
		}
		input, _ := json.Marshal(map[string]any{"kind": map[string]string{"isRenameFile": "rename", "isCreateFile": "create", "isDeleteFile": "delete"}[op], "uri": "u", "oldUri": "u", "newUri": "v", "options": nil})
		fmt.Fprintf(&unit, "CLEAR refused.\nTRY.\nresult = %s=>%s( input = `%s` condition = abap_false ).\nCATCH cx_sy_move_cast_error.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", owner, names.Get("member."+op), input)
	}
	fmt.Fprint(&unit, "ENDMETHOD.\nENDCLASS.\n")
	files[owner+".clas.testclasses.abap"] = strings.NewReplacer("input =", names.Get("param.input")+" =", "condition =", names.Get("param.condition")+" =").Replace(unit.String())
	storageHostFixture(t, files, owner)
}
