package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRulesConfigIndexedGoMatchesNode(t *testing.T) {
	source := `export abstract class BasicRuleConfig { severity?: string = "Error"; absent?: string = undefined; }
export abstract class NamingRuleConfig extends BasicRuleConfig { ignoreNames?: string[] = []; }
export class Config extends NamingRuleConfig { clas?: string = "^Z"; enabled: boolean = false; count: number = 0; }
export class Derived extends Config { extra?: string = "sub"; }
export class Probe {
 static calls: string = "";
 static receiver(config: Config): Config { Probe.calls += "r"; return config; }
 static key(key: string): string { Probe.calls += "k"; return key; }
 static read(config: Config, key: string): string {
  // @ts-ignore
  const value = Probe.receiver(config)[Probe.key(key)];
  return value === undefined ? "absent" : "" + value;
 }
 static has(config: Config, key: string): boolean { return true; }
 static own(config: Config, key: string): boolean { return config.hasOwnProperty(key); }
 static run(): string {
  const config = new Config();
  let out = Probe.read(config,"clas") + "/" + Probe.read(config,"severity") + "/" + Probe.read(config,"absent") + "/" + Probe.read(config,"unknown") + "/" + Probe.read(config,"enabled") + "/" + Probe.read(config,"count");
  config.clas = "";
  out += "/" + Probe.read(config,"clas");
  return out + "/" + Probe.read(new Derived(),"extra") + "/" + Probe.own(config,"absent") + "/" + Probe.own(config,"severity") + "/" + Probe.own(config,"toString") + "/" + Probe.own(config,"unknown") + "/" + Probe.has(config,"absent") + "/" + Probe.has(config,"toString") + "/" + Probe.has(config,"unknown") + "/" + Probe.calls;
 }
}`
	prog := lowerStatementsProbe(t, map[string]string{"input.ts": source}, []string{"input.ts"})
	// Exercise presence metadata through HIR without extending the separately
	// owned TS `in` operator.
	for _, c := range prog.Classes {
		if c.Name == "input.ts.Probe" {
			for _, m := range c.Methods {
				if m.Name == "has" {
					m.Body = hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "object.has", Type: hir.T(hir.Bool), X: hir.V("config", hir.Ref("input.ts.Config")), Args: []*hir.Expr{hir.V("key", hir.T(hir.String))}}})
				}
			}
		}
	}
	got := runGoHIR(t, prog, "fmt.Print("+goEntry("input.ts.Probe", "run")+"().String())")
	dir := t.TempDir()
	oracle := filepath.Join(dir, "oracle.ts")
	if err := os.WriteFile(oracle, []byte(strings.Replace(source, "static has(config: Config, key: string): boolean { return true; }", "static has(config: Config, key: string): boolean { return key in config; }", 1)+"\nconsole.log(Probe.run());\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", oracle).CombinedOutput()
	if err != nil {
		t.Fatalf("Node: %v: %s", err, out)
	}
	if want := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("Go %q, Node %q", got, want)
	}
}

func TestRulesConfigPrototypeMutationRefused(t *testing.T) {
	for _, mutation := range []string{
		`Object.setPrototypeOf(config, new Config());`,
		`Config.prototype.field = "changed";`,
		`config["__proto__"] = new Config();`,
	} {
		t.Run(mutation, func(t *testing.T) {
			source := `export abstract class BasicRuleConfig { field: string = "own"; }
export class Config extends BasicRuleConfig {}
export class Probe {static run(): void {const config=new Config();` + mutation + `}}`
			dir := sourceProbe(t, source)
			p, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			_, diags, err := p.LowerWithOverrides([]string{"input.ts"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !hasBlocking(diags) {
				t.Fatal("prototype mutation accepted")
			}
		})
	}
}
