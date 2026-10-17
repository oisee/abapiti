package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// An unsupported initializer traps its enclosing function. Its later closures
// still need the declared local type, even when another module exports an
// unrelated global with the same name. The executed path is a Node oracle;
// the retained failed path must not acquire a second, spurious type trap.
func TestStructuralKeysFrontendProgramRecoveryNodeOracle(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"types.ts": `export type Mutable<T>={-readonly [K in keyof T]:T[K]};
export interface Program {readonly rows:readonly string[]}
export interface FrontendProgram extends Omit<Mutable<Program>,'rows'> {rows:number[];extra?:unknown}
export const program:Program={rows:['driver']};`,
		"probe.mjs": `import {program as driverProgram} from './types.ts';
/** @param {import('./types.ts').FrontendProgram} program @returns {string} */ function finish(program){return ''+program.rows[0];}
function bad(){
 /** @type {import('./types.ts').FrontendProgram} */ const program={rows:[7],extra:new WeakRef({})};
 return ()=>finish(program);
}
function good(){
 /** @type {import('./types.ts').FrontendProgram} */ const program={rows:[7]};
 return finish(program);
}
function readGlobal(){return driverProgram.rows[0];}
export class Probe {static run(){return good();}}
`,
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022","module":"nodenext","allowImportingTsExtensions":true,"noEmit":true,"allowJs":true},"include":["*.mjs","*.ts"]}`,
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := exec.Command("node", "--input-type=module", "-e", `import {Probe} from `+fmt.Sprintf("%q", filepath.Join(dir, "probe.mjs"))+`;console.log(Probe.run())`).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	project, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := project.LowerWithOptions([]string{"types.ts", "probe.mjs"}, LowerOptions{TrapOnFailure: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Error(d)
		}
	}
	if errors := hir.Verify(prog); len(errors) != 0 {
		t.Fatal(errors)
	}
	got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.mjs.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	dump := hir.Dump(prog)
	if !strings.Contains(dump, "WeakRef") {
		t.Fatal("missing expected unsupported initializer trap")
	}
	if strings.Contains(dump, "call argument type mismatch") {
		t.Fatal("failed local initializer resolved its closure to an unrelated module global")
	}
	if _, err := emitABAP(t, prog); err != nil {
		t.Fatal(err)
	}
}
