package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/internal/abapfixture"
)

// testdata/extern-abap is also the OSGo smoke program
// (tools/extern-abap-osg-smoke.sh); here only emission is checked.
func TestEmitABAPInitializeExterns(t *testing.T) {
	project, err := filepath.Abs("testdata/extern-abap")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	emit := newEmitABAPCmd()
	emit.SetArgs([]string{"--tsconfig", filepath.Join(project, "tsconfig.json"), "--externs", filepath.Join(project, "externs.json"),
		"--initialize", filepath.Join(project, "initialization.json"), "-o", out, "main.ts", "state.ts", "host.ts"})
	if err = emit.Execute(); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(out, "src")
	host, err := os.ReadFile("../../extern/hostio/abap/zcl_abapiti_hostio.clas.abap")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(src, "zcl_abapiti_hostio.clas.abap"), host, 0644); err != nil {
		t.Fatal(err)
	}
	abapfixture.CollectDir(t, "cmd/abapiti", src)
	main, err := os.ReadFile(filepath.Join(src, "zcl_abapiti_main.clas.abap"))
	if err != nil {
		t.Fatal(err)
	}
	// host.ts has no module state; state.ts initializes before main.ts.
	want := "CLASS zcl_abapiti_main DEFINITION PUBLIC FINAL CREATE PUBLIC.\n  PUBLIC SECTION.\n    CLASS-METHODS run.\nENDCLASS.\nCLASS zcl_abapiti_main IMPLEMENTATION.\n  METHOD run.\n" +
		"    z_state_ts_mod_eb5af01b30085e=>z_builtin_init_9016f21c683b49( ).\n    z_main_ts_modu_50921dc30544a5=>z_builtin_init_1003c5810dfa2d( ).\n  ENDMETHOD.\nENDCLASS.\n"
	if string(main) != want {
		t.Fatalf("main class:\n%s", main)
	}
	names, err := os.ReadFile(filepath.Join(out, "evidence", "names.json"))
	if err != nil || !strings.Contains(string(names), `"ZCL_ABAPITI_MAIN"`) {
		t.Fatalf("names.json lacks the entry class: %v", err)
	}
	traps, err := os.ReadFile(filepath.Join(out, "evidence", "trap-sites.json"))
	if err != nil || strings.TrimSpace(string(traps)) != "null" {
		t.Fatalf("unexpected traps: %v %s", err, traps)
	}
	for _, call := range []string{"zcl_abapiti_hostio=>env( name = ", "zcl_abapiti_hostio=>list_entries( dir = ", "zcl_abapiti_hostio=>write_stdout( text = "} {
		found := false
		for _, file := range []string{"z_state_ts_mod_eb5af01b30085e.clas.abap", "z_main_ts_modu_50921dc30544a5.clas.abap"} {
			b, _ := os.ReadFile(filepath.Join(src, file))
			found = found || strings.Contains(string(b), call)
		}
		if !found {
			t.Errorf("missing host call %q", call)
		}
	}
	bad := filepath.Join(t.TempDir(), "order.json")
	if err = os.WriteFile(bad, []byte(`["other.ts"]`), 0600); err != nil {
		t.Fatal(err)
	}
	emit = newEmitABAPCmd()
	emit.SetArgs([]string{"--tsconfig", filepath.Join(project, "tsconfig.json"), "--externs", filepath.Join(project, "externs.json"),
		"--initialize", bad, "-o", t.TempDir(), "main.ts", "state.ts", "host.ts"})
	emit.SilenceUsage, emit.SilenceErrors = true, true
	if err = emit.Execute(); err == nil || !strings.Contains(err.Error(), "outside selected closure") {
		t.Fatalf("order outside the closure: %v", err)
	}
}
