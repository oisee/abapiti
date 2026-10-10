package golang

import (
	"encoding/json"
	"github.com/oisee/abapiti/hir"
	"reflect"
	"testing"
)

func TestResultDebugDoesNotChangeNormalEmissionOrHIR(t *testing.T) {
	p := &hir.Program{Classes: []*hir.Class{{Name: "src/abap/2_statements/result.ts.Result", Fields: []hir.Field{{Name: "count", Type: hir.T(hir.I64)}}}}}
	before, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	normal, err := Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	debug, err := EmitResultDebug(p)
	if err != nil {
		t.Fatal(err)
	}
	if debug["result-debug-locals.json"] == "" || debug["result-debug-shapes.json"] == "" {
		t.Fatal("missing Go-only metadata")
	}
	after, err := Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	state, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(state) || !reflect.DeepEqual(normal, after) {
		t.Fatal("debug emission affected HIR or default emission")
	}
	if normal["result-debug-locals.json"] != "" || normal["result-debug-shapes.json"] != "" {
		t.Fatal("debug metadata leaked into normal build")
	}
}
