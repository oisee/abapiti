package abap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestCallableClassPredicate(t *testing.T) {
	for _, tc := range []struct {
		name       string
		implements []string
		want       bool
	}{
		{"closure.arrow", []string{"closure.function.test"}, true},
		{"closure.bound", []string{"closure.thunk"}, true},
		{"ordinary.call", []string{"closure.function.test"}, false},
		{"closure.data", nil, false},
		{"closure.unproven", []string{"ordinary.call"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &hir.Class{Name: tc.name, Implements: tc.implements, Methods: []*hir.Method{{Name: "call"}}}
			if got := callableClass(c); got != tc.want {
				t.Fatalf("callable = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTypeofMarkerConstantSize(t *testing.T) {
	var previous string
	for _, count := range []int{0, 1, 64} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			d := hir.T(hir.Dynamic)
			m := method("tag", str, hir.B(ret(rt("dynamic.typeofValue", local("x", d), str))))
			m.Static, m.Params = true, []hir.Param{{Name: "x", Type: d}}
			p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{m}}}}
			if count > 0 {
				p.Interfaces = []*hir.Interface{{Name: "closure.function.test"}}
				for i := 0; i < count; i++ {
					p.Classes = append(p.Classes, &hir.Class{Name: fmt.Sprintf("closure.test%d", i), Implements: []string{"closure.function.test"}})
				}
			}
			files, names, err := emitABAPNamed(t, p)
			if err != nil {
				t.Fatal(err)
			}
			marker := names.Get("runtime.callable")
			src := files[names.Get("Probe")+".clas.abap"]
			checks := strings.Count(src, "->oval IS INSTANCE OF ")
			if count == 0 {
				if checks != 0 || files[marker+".intf.abap"] != "" {
					t.Fatal("marker emitted without callable classes")
				}
				return
			}
			if checks != 1 || !strings.Contains(src, "->oval IS INSTANCE OF "+marker+".") {
				t.Fatal("typeof needs one marker check")
			}
			for _, c := range p.Classes[1:] {
				if !strings.Contains(files[names.Get(c.Name)+".clas.abap"], "INTERFACES "+marker+".") {
					t.Fatalf("missing marker on %s", c.Name)
				}
			}
			if previous != "" && src != previous {
				t.Fatal("typeof method grew with closure count")
			}
			previous = src
		})
	}
}
