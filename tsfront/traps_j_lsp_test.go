package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestLSPErasedInNodeOracle(t *testing.T) {
	source := `import {TextEdit,TextDocumentEdit,Range,RenameFile} from "vscode-languageserver-types";
 class Probe {
 static has(x:object,key:string):boolean {return key in x;}
 static run():string {const edit=TextEdit.replace(Range.create(0,0,0,0),"");const doc=TextDocumentEdit.create({uri:"file",version:1},[edit]);return Probe.has(edit,"newText")+":"+Probe.has(doc,"newText")+":"+Probe.has(doc,"textDocument")+":"+Probe.has(edit,"toString");}
 static invalid():void {Probe.has(RenameFile.create("old","new"),"options");}
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
	if _, err := os.Stat(filepath.Join(pkg, "lib/umd/main.js")); err != nil {
		pkg = filepath.Join(core, "../../node_modules/vscode-languageserver-types")
		if _, err := os.Stat(filepath.Join(pkg, "lib/umd/main.js")); err != nil {
			t.Skip("live pinned LSP oracle requires installed upstream npm dependencies")
		}
	}
	body := source[strings.Index(source, "\n")+1:]
	out, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `if(require(process.argv[1]+"/package.json").version!=="3.18.0")throw Error("wrong LSP pin");const {TextEdit,TextDocumentEdit,Range,RenameFile}=require(process.argv[1]);if("options" in RenameFile.create("old","new"))throw Error("Node optional field presence");eval(require("node:module").stripTypeScriptTypes(process.argv[2])+"\nconsole.log(Probe.run())")`, pkg, body).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	got := runGoHIR(t, p, "fmt.Println("+goEntry("main.ts.Probe", "run")+"())")
	if got != string(out) {
		t.Fatalf("Go %q Node %q", got, out)
	}
	if got := runGoHIR(t, p, "defer func(){if _,ok:=recover().(trap);!ok{panic(\"unknown factory presence escaped\")};fmt.Println(\"refused\")}();"+goEntry("main.ts.Probe", "invalid")+"()"); got != "refused\n" {
		t.Fatal(got)
	}
	storageABAPProbe(t, p, "main.ts.Probe", strings.TrimSpace(string(out)), "invalid")
}
