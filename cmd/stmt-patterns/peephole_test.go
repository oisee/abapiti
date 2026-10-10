package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPeepholeExportFullCorpusAndSourceMap(t *testing.T) {
	input, output := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(input, "classes"), 0755); err != nil {
		t.Fatal(err)
	}
	names := `{"Z_RUNTIME":{"id":"runtime.test","kind":"runtime"},"Z_SCOPE":{"id":"src/scope.ts.CurrentScope","kind":"class","source":"src/scope.ts:1:1"},"GET":{"id":"member.get","kind":"member","declared":["src/scope.ts.CurrentScope.get@src/scope.ts:8:3"]}}`
	body := "CLASS z_scope IMPLEMENTATION.\nMETHOD get.\nLOOP AT tab INTO row.\nDATA(t1) = CONV string(\n |a.b| ).\nresult = t1.\nENDLOOP.\nENDMETHOD.\nENDCLASS.\n"
	for p, s := range map[string]string{"names.json": names, "classes/z_scope.clas.abap": body, "classes/z_runtime.clas.abap": "CLASS z_runtime IMPLEMENTATION. METHOD get. result = 1. ENDMETHOD. ENDCLASS."} {
		if err := os.WriteFile(filepath.Join(input, p), []byte(s), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := exportPeepholes(input, output); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(output, "statements.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ms []miningMethod
	if err = json.Unmarshal(raw, &ms); err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[0].TS != "runtime.test.get" || ms[0].Source != "" || ms[1].TS != "src/scope.ts.CurrentScope.get" || ms[1].Source != "src/scope.ts:8:3" {
		t.Fatalf("source mapping: %+v", ms)
	}
	ss := ms[1].Statements
	if len(ss) != 4 || ss[1].Line != 4 || ss[1].Depth != 1 || ss[1].Normalized != "DATA(TMP) = CONV STRING( LIT )." {
		t.Fatalf("statements: %+v", ss)
	}
}
