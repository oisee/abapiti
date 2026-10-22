package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Shape of the @types/node declarations the lowering relies on: ambient
// modules in a declaration file (the probe has no node_modules).
const nodePlatformStub = `declare module "node:fs" {
 export class Dirent<Name extends string = string> { name: Name; isDirectory(): boolean; }
 export function readFileSync(path: string, encoding: "utf8"): string;
 export function readdirSync(path: string): string[];
 export function readdirSync(path: string, options: {withFileTypes: true}): Dirent[];
}
declare module "node:url" { export function fileURLToPath(url: string): string; }
declare module "node:path" { export function basename(path: string, suffix?: string): string; }
declare module "node:crypto" {
 export interface Hash { update(data: string): Hash; digest(encoding: "hex"): string; }
 export function createHash(algorithm: string): Hash;
}
`

func nodePlatformOracle(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "probe.mts")
	if err := os.WriteFile(file, []byte(source+"\nconsole.log(Probe.run());\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", "--disable-warning=ExperimentalWarning", file).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	return string(out)
}

func nodePlatformCheck(t *testing.T, source string, implemented ...bool) {
	t.Helper()
	want := nodePlatformOracle(t, source)
	files := map[string]string{"probe.ts": source, "node.d.ts": nodePlatformStub}
	p := lowerStatementsProbe(t, files, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q\nNode %q", got, want)
	}
	if len(implemented) > 0 && implemented[0] {
		storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSuffix(want, "\n"), "")
		return
	}
	// Filesystem operations still require a host ABI: located refusal.
	if _, err := emitABAP(t, lowerStatementsProbe(t, files, []string{"probe.ts"})); err == nil || !strings.Contains(err.Error(), "probe.ts:") {
		t.Fatalf("wanted located ABAP refusal, got %v", err)
	}
}

func TestNodePlatformHashNodeOracle(t *testing.T) {
	nodePlatformCheck(t, `import {createHash as digestOf} from "node:crypto";
import * as crypto from "node:crypto";
class Probe {
 static calls = 0;
 static once(s: string): string { Probe.calls++; return s; }
 static named(s: string): string { return digestOf("sha256").update(Probe.once(s)).digest("hex"); }
 static spaced(s: string): string { return crypto.createHash("sha256").update(s).digest("hex"); }
 static legacy(s: string): string { return crypto.createHash("sha1").update(s).digest("hex"); }
 static untyped(s: any): string { return digestOf("sha256").update(s).digest("hex"); }
 static run(): string {
  return [Probe.named(""), Probe.named("abc"), Probe.named("æ中😀"), Probe.spaced("a\0b"),
   Probe.legacy("abc"), Probe.untyped(JSON.stringify({k: [1, "x"]})), String(Probe.calls)].join("|");
 }
}`, true)
}

func TestNodePlatformBasenameNodeOracle(t *testing.T) {
	nodePlatformCheck(t, `import {basename} from "node:path";
import * as path from "node:path";
class Probe {
 static run(): string {
  const cases = ["", "/", "//", "a", "a/b", "a/b/", "/a//", "/x/y.txt", "x\\y/z.txt", "dir\\file.abap", "/é/😀", "./.."] ;
  return cases.map((c) => "[" + basename(c.replaceAll("\\", "/")) + "," + path.basename(c) + "]").join("");
 }
}`, true)
}

func TestNodePlatformFSNodeOracle(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("text.txt", []byte("\xef\xbb\xbfhi\r\n\xe2\x82x\xff\xed\xa0\x80é😀"))
	write("\xffodd.bin", nil)
	write("empty.txt", nil)
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("sub", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", filepath.Join(dir, "dangling")); err != nil {
		t.Fatal(err)
	}
	root := strings.ReplaceAll(dir, `\`, `\\`)
	nodePlatformCheck(t, `import {readFileSync, readdirSync} from "node:fs";
class Probe {
 static readText(file: string): string { return readFileSync(file, "utf8"); }
 static listDir(dir: string): string[] { return readdirSync(dir); }
 static listEntries(dir: string): [string, boolean][] { return readdirSync(dir, {withFileTypes: true}).map((entry) => [entry.name, entry.isDirectory()]); }
 static attempt(f: () => string): string { try { return f(); } catch (e) { return "error " + (e as Error).message; } }
 static run(): string {
  const root = "`+root+`";
  const out: string[] = [];
  out.push(JSON.stringify(Probe.readText(root + "/text.txt")), "[" + Probe.readText(root + "/empty.txt") + "]");
  out.push(Probe.listDir(root).join(","));
  for (const [name, directory] of Probe.listEntries(root)) out.push(name + "=" + directory);
  out.push(Probe.attempt(() => Probe.readText(root + "/missing.txt")));
  out.push(Probe.attempt(() => Probe.readText(root + "/sub")));
  out.push(Probe.attempt(() => Probe.listDir(root + "/text.txt").join()));
  out.push(Probe.attempt(() => Probe.listDir(root + "/none").join()));
  out.push(Probe.attempt(() => String(Probe.listEntries(root + "/none").length)));
  out.push(Probe.listDir(root + "/sub").length + "");
  return out.join("\n");
 }
}`)
}

func TestNodePlatformRefusals(t *testing.T) {
	for _, c := range []struct{ source, want string }{
		{`import {basename} from "node:path"; class Probe { static run(): string { return basename("a.txt", ".txt"); } }`, "suffix"},
		{`import {readFileSync} from "node:fs"; class Probe { static run(): string { return String(readFileSync("x")); } }`, "utf8"},
		{`import {readdirSync} from "node:fs"; class Probe { static run(): number { return readdirSync("x", {withFileTypes: false}).length; } }`, "withFileTypes"},
		{`import {readFileSync} from "node:fs"; class Probe { static run(): string { return readFileSync(new URL("file:///x") as unknown as string & {u: 1}, "utf8"); } }`, "string paths"},
		{`import {fileURLToPath} from "node:url"; class Probe { static run(): string { return fileURLToPath("file:///x"); } }`, "module locations"},
		{`import {createHash} from "node:crypto"; class Probe { static run(): string { return createHash("md4").update("x").digest("hex"); } }`, "sha256"},
	} {
		p, err := Load(writeProbe(t, map[string]string{"probe.ts": c.source, "node.d.ts": nodePlatformStub}))
		if err != nil {
			t.Fatal(err)
		}
		_, diags, err := p.Lower([]string{"probe.ts"})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range diags {
			found = found || strings.Contains(d.String(), "probe.ts:") && strings.Contains(d.String(), c.want)
		}
		if !found {
			t.Errorf("%s: no located %q diagnostic in %v", c.source, c.want, diags)
		}
	}
}

func writeProbe(t *testing.T, sources map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, source := range sources {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	config := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(config, []byte(`{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	return config
}
