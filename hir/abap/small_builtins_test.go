package abap

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestSmallBuiltinEmission(t *testing.T) {
	for _, op := range []string{"string.trimStart", "string.trimEnd", "map.delete"} {
		t.Run(op, func(t *testing.T) {
			recv := str
			result := str
			var args []*hir.Expr
			if op == "map.delete" {
				recv = hir.T(hir.OrderedMap, str, i32)
				result = boolean
				args = []*hir.Expr{hir.L(str, "key")}
			}
			m := method("run", result, hir.B(ret(rt(op, local("value", recv), result, args...))))
			m.Static = true
			m.Params = []hir.Param{{Name: "value", Type: recv}}
			p := &hir.Program{Classes: []*hir.Class{{Name: "Small", Methods: []*hir.Method{m}}}}
			if errs := hir.Verify(p); len(errs) > 0 {
				t.Fatal(errs)
			}
			files, err := emitABAP(t, p)
			if err != nil {
				t.Fatal(err)
			}
			all := ""
			for _, s := range files {
				all += s
			}
			if op == "map.delete" {
				if !strings.Contains(all, "DELETE ") || !strings.Contains(all, "xsdbool( sy-subrc = 0 )") {
					t.Fatal("Map.delete must return presence before deletion")
				}
			} else {
				if strings.Count(all, "WHILE ") != 1 {
					t.Fatal("one-sided trim must scan only one end")
				}
				if !strings.Contains(all, "65279") {
					t.Fatal("trim must recognize ECMAScript BOM whitespace")
				}
			}
		})
	}
}
