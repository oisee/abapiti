package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// frontend.mjs compileProgram's walk sorts host-io-compat readdirEntries()
// by `x.name`, a member of host-io.ts's exported DirectoryEntry interface.
// Reading a property of the caller's own object is not a use of an extern
// export; only the module's top-level functions are extern calls.
func TestStep4ExternInterfaceMemberNodeOracle(t *testing.T) {
	host, err := os.ReadFile("testdata/extern/host.ts")
	if err != nil {
		t.Fatal(err)
	}
	hostTS := string(host) + "export interface DirectoryEntry { name: string; isDirectory: () => boolean }\n"
	source := `import {join, listEntries, DirectoryEntry} from "./host";
function entries(names: string[]): DirectoryEntry[] {
  return names.map((name, i) => { const entry: DirectoryEntry = {name, isDirectory: () => i % 2 === 0}; return entry; });
}
class Probe {
  static run(): string {
    const fake = entries(["b", "c", "a"]);
    return fake.filter((x) => x.name !== "c").map((e) => join("d", e.name) + ":" + String(e.isDirectory())).join(",") + "|" + String(listEntries(".").length);
  }
}
`
	r, modules, _ := externProject(t, source, hostTS)
	bin, _ := buildExternProbe(t, r, modules)
	work := t.TempDir()
	cmd := exec.Command("nice", bin)
	cmd.Dir = work
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v %s", err, got)
	}
	nodeHost, err := filepath.Abs("testdata/extern/host.cjs")
	if err != nil {
		t.Fatal(err)
	}
	js := strings.Replace(source, `import {join, listEntries, DirectoryEntry} from "./host";`, `const {join, listEntries}=require(`+fmt.Sprintf("%q", nodeHost)+`);`, 1)
	cmd = exec.Command("nice", "node", "--disable-warning=ExperimentalWarning", "-e", `eval(require('node:module').stripTypeScriptTypes(process.argv[1])+'\nconsole.log(Probe.run())')`, js)
	cmd.Dir = work
	node, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Node: %v %s", err, node)
	}
	if string(got) != string(node) {
		t.Fatalf("Go %q Node %q", got, node)
	}
}
