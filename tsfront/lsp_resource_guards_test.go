package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestLSPResourceGuardsNodeOracle(t *testing.T) {
	source := `import {CreateFile,DeleteFile} from "vscode-languageserver-types";
 class Probe {
 static run(json:string):string {const x=JSON.parse(json);return (CreateFile.is(x)?"C":"-")+(DeleteFile.is(x)?"D":"-");}
 static value():boolean {return CreateFile.is(JSON.parse("null"));}
 }`
	p, ds := lspProbe(t, source)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	if es := hir.Verify(p); len(es) > 0 {
		t.Fatal(es)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	core := os.Getenv("TSFRONT_ABAPLINT")
	if core == "" {
		core = filepath.Join(home, "dev/abaplint/packages/core")
	}
	pkg := filepath.Join(core, "node_modules/vscode-languageserver-types")
	if _, err := os.Stat(filepath.Join(pkg, "package.json")); err != nil {
		pkg = filepath.Join(core, "../../node_modules/vscode-languageserver-types")
	}
	if _, err := os.Stat(filepath.Join(pkg, "package.json")); err != nil {
		t.Skip("pinned LSP oracle unavailable")
	}
	script := `if(require(process.argv[1]+"/package.json").version!=="3.18.0")throw Error("pin");const {CreateFile,DeleteFile}=require(process.argv[1]);const x=JSON.parse(process.argv[2]);console.log((CreateFile.is(x)?"C":"-")+(DeleteFile.is(x)?"D":"-"));`
	for _, json := range []string{`{"kind":"create","uri":"x"}`, `{"kind":"delete","uri":"x"}`, `{"kind":"rename","uri":"x"}`, `{"kind":"create"}`, `{"kind":"create","uri":3}`, `{"kind":"create","uri":"x","options":{"overwrite":true,"ignoreIfExists":false},"annotationId":"a"}`, `{"kind":"delete","uri":"x","options":{"recursive":false,"ignoreIfNotExists":true}}`, `{"kind":"delete","uri":"x","options":{"recursive":1}}`, `{"kind":"create","uri":"x","annotationId":1}`, `null`, `false`, `0`, `""`, `[]`} {
		out, err := exec.Command("node", "-e", script, pkg, json).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		got := runGoHIR(t, p, "fmt.Print("+goEntry("main.ts.Probe", "run")+"(str("+strconv.Quote(json)+")).String())")
		if got != strings.TrimSpace(string(out)) {
			t.Fatalf("%s: Go %q Node %q", json, got, out)
		}
	}
	out, err := exec.Command("node", "-e", `console.log(String(require(process.argv[1]).CreateFile.is(null)))`, pkg).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "null" {
		t.Fatal(err, string(out))
	}
	got := runGoHIR(t, p, "defer func(){if x:=recover();x!=nil {fmt.Print(x)}}();fmt.Print("+goEntry("main.ts.Probe", "value")+"())")
	if !strings.Contains(got, "non-boolean falsy value") {
		t.Fatal(got)
	}
}
