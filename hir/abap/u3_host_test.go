package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
	"testing"
)

func TestU3HostFallbacks(t *testing.T) {
	src := u3Emit(t, rt("host.getenv", hir.L(str, "HOME"), hir.T(hir.Optional, str)), rt("host.stderrWrite", hir.L(str, "diagnostic"), hir.T(hir.Void)))
	u3Want(t, src, "HOME", "diagnostic", "CLEAR ")
	if strings.Contains(src, "=>env(") || strings.Contains(src, "WRITE ") {
		t.Fatal("fallback must not access a host or print diagnostics to stdout")
	}
}
func TestU3HostExternCapabilities(t *testing.T) {
	opt := hir.T(hir.Optional, str)
	env := method("envProbe", opt, ret(rt("host.getenv", hir.L(str, "EMPTY"), opt)))
	env.Static = true
	stderr := method("stderrProbe", hir.T(hir.Void), hir.B(run(rt("host.stderrWrite", hir.L(str, "message"), hir.T(hir.Void)))))
	stderr.Static = true
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{env, stderr}}}, Externs: []hir.ExternBinding{
		{Module: "github.com/oisee/abapiti/extern/hostio", Name: "env", Result: opt, Signature: hir.ExternSignature{ABAPClass: "zcl_abapiti_hostio", Params: []hir.Type{str}, ParamNames: []string{"name"}}},
		{Module: "example/host", Name: "writeStderr", Result: hir.T(hir.Void), Signature: hir.ExternSignature{ABAPClass: "zcl_probe_host", Params: []hir.Type{str}, ParamNames: []string{"text"}}},
	}}
	files, err := emitABAP(t, p)
	if err != nil {
		t.Fatal(err)
	}
	var src string
	for _, v := range files {
		src += v
	}
	u3Want(t, src, "zcl_abapiti_hostio=>env( name = ", "zcl_probe_host=>write_stderr( text = ", "IS BOUND.", "->* TO <", "->has = abap_true.")
	if strings.Contains(src, "CAST string(") {
		t.Fatal("host optional string must be read as a data reference, not cast to an object")
	}
}
