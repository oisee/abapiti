package rewrite_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/hir/golang"
	"github.com/oisee/abapiti/internal/hirclone"
)

func TestAccumulatorBackendFlag(t *testing.T) {
	p, _, _ := accumulatorFixture()
	t.Setenv("ABAPITI_INLINE", "0")
	for _, emit := range []struct {
		name string
		run  func(*hir.Program) (map[string]string, error)
	}{{"go", golang.Emit}, {"abap", abap.Emit}} {
		t.Run(emit.name, func(t *testing.T) {
			t.Setenv("ABAPITI_ACCUMULATOR", "")
			off, err := emit.run(hirclone.Clone(p))
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("ABAPITI_ACCUMULATOR", "0")
			explicit, err := emit.run(hirclone.Clone(p))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(off, explicit) {
				t.Fatal("default differs from OFF")
			}
			t.Setenv("ABAPITI_ACCUMULATOR", "1")
			on, err := emit.run(hirclone.Clone(p))
			if err != nil {
				t.Fatal(err)
			}
			if reflect.DeepEqual(off, on) {
				t.Fatal("ON did not reach shared pass")
			}
		})
	}
}

func TestAccumulatorGoExecution(t *testing.T) {
	p, _, c := accumulatorFixture()
	arr := c.Result
	n := arr.Args[0]
	p.Interfaces = []*hir.Interface{{Name: "I", Methods: []*hir.Method{{Name: "m", Virtual: true, Abstract: true, Result: arr}}}}
	p.Classes[0].Implements = []string{"I"}
	// The fallback returns a constructed array without the eligible body shape.
	lit := &hir.Expr{Kind: hir.Seq, Type: arr, Y: hir.V("tmp", arr), Stmt: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "tmp", Type: arr, X: &hir.Expr{Kind: hir.New, Type: arr}})}
	for _, v := range []int{7, 8} {
		lit.Stmt.List = append(lit.Stmt.List, &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: hir.T(hir.I32), X: hir.V("tmp", arr), Args: []*hir.Expr{hir.L(n, v)}}})
	}
	p.Classes = append(p.Classes, &hir.Class{Name: "D", Implements: []string{"I"}, Methods: []*hir.Method{{Name: "m", Virtual: true, Result: arr, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: lit})}}})
	c.Params[0].Type = hir.Type{Kind: hir.InterfaceRef, Name: "I"}
	c.Body.List[1].X.X.Type = c.Params[0].Type
	// A preexisting element verifies append order and reference identity.
	c.Body.List = append(c.Body.List[:1], append([]*hir.Stmt{{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: hir.T(hir.I32), X: hir.V("out", arr), Args: []*hir.Expr{hir.L(n, 0)}}}}, c.Body.List[1:]...)...)
	for _, flag := range []string{"0", "1"} {
		t.Run(flag, func(t *testing.T) {
			t.Setenv("ABAPITI_ACCUMULATOR", flag)
			files, err := golang.Emit(hirclone.Clone(p))
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			names := hir.NewNames()
			files["go.mod"] = "module probe\n\ngo 1.26.0\n"
			files["main.go"] = fmt.Sprintf("package main\nimport \"fmt\"\nfunc main(){fmt.Println(%s(%s()).Items,%s(%s()).Items)}\n", names.Get("body.C.caller"), names.Get("new.C"), names.Get("body.C.caller"), names.Get("new.D"))
			for name, src := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("go", "run", ".")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GOWORK=off")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			if strings.TrimSpace(string(out)) != "[0 1] [0 7 8]" {
				t.Fatal(string(out))
			}
		})
	}
	t.Setenv("ABAPITI_ACCUMULATOR", "1")
	t.Setenv("ABAPITI_INLINE", "0")
	files, err := abap.Emit(hirclone.Clone(p))
	if err != nil {
		t.Fatal(err)
	}
	into := hir.NewNames().Get("member.m_into")
	for name, src := range files {
		if strings.HasSuffix(name, ".intf.abap") {
			for _, line := range strings.Split(src, "\n") {
				if strings.Contains(line, into) && strings.Contains(line, "ABSTRACT") {
					t.Fatal(line)
				}
			}
		}
	}
}
