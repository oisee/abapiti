package tsfront

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// Compile the actual emitted CLI with a small harness that exposes what it read.
// This keeps the host I/O contract test independent of the large translation.
func TestRegistryGoReadPolicy(t *testing.T) {
	dir := t.TempDir()
	names := hir.NewNames()
	stub := strings.NewReplacer("@new@", names.Get("new.harness/registry_run.ts.RegistryRun"), "@addFile@", names.Get("member.addFile"), "@addDependency@", names.Get("member.addDependency"), "@parse@", names.Get("member.parse"), "@report@", names.Get("member.report"), "@timings@", names.Get("member.timings")).Replace(`package main
 type value string
 func str(s string) value { return value(s) }
 func (v value) String() string { return string(v) }
 type harness struct { data string }
 func @new@() *harness { return &harness{} }
 func (h *harness) @addFile@(name, raw value) { h.data += string(raw) }
 func (h *harness) @addDependency@(name, raw value) { h.data += string(raw) }
 func (h *harness) @parse@(cfg value) value { return value(h.data)+cfg }
 func (h *harness) @report@(v value) value { return v }
 func (h *harness) @timings@() value { return "" }
 `)
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "main.go"), RegistryGoCLI())
	write(filepath.Join(dir, "stub.go"), stub)
	write(filepath.Join(dir, "go.mod"), "module readpolicy\n\ngo 1.26.0\n")
	write(filepath.Join(dir, "policy_test.go"), `package main
 import("os";"path/filepath";"strings";"testing")
 func TestPolicy(t *testing.T) {
  base:=t.TempDir();inside:=filepath.Join(base,"inside");outside:=filepath.Join(base,"inside-sibling")
  for _,d:=range []string{inside,outside}{if err:=os.Mkdir(d,0755);err!=nil{t.Fatal(err)}}
  for _,d:=range []string{inside,outside}{if err:=os.WriteFile(filepath.Join(d,"file"),[]byte("ok"),0644);err!=nil{t.Fatal(err)}}
  var p readPolicy;if err:=p.allow(inside);err!=nil{t.Fatal(err)}
  if _,err:=p.read(filepath.Join(inside,"file"));err!=nil{t.Fatal(err)}
  // Sibling-prefix confusion and parent traversal must both be refused.
  for _,path:=range []string{filepath.Join(outside,"file"),inside+"/../inside-sibling/file",filepath.Join(outside,"missing")}{
   if _,err:=p.read(path);err==nil||!strings.Contains(err.Error(),"-allow-read")||!strings.Contains(err.Error(),path){t.Fatalf("read %s: %v",path,err)}
  }
  link:=filepath.Join(inside,"link")
  if err:=os.Symlink(outside,link);err!=nil{t.Logf("symlink unavailable: %v",err)}else if _,err:=p.read(filepath.Join(link,"file"));err==nil{t.Fatal("symlink escaped")}
  if err:=p.allow(outside);err!=nil{t.Fatal(err)}
  if _,err:=p.read(filepath.Join(outside,"file"));err!=nil{t.Fatal(err)}
 }
 `)
	runGo := func(args ...string) {
		t.Helper()
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-buildvcs=false")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %v: %v\n%s", args, err, out)
		}
	}
	runGo("test", "-v", ".")
	binary := filepath.Join(dir, "zabaplint-go")
	runGo("build", "-o", binary, ".")
	file := filepath.Join(dir, "input", "source.abap")
	config := filepath.Join(dir, "config", "abaplint.json")
	deps := filepath.Join(dir, "lists", "deps.txt")
	write(file, "file|")
	write(config, "config|")
	write(filepath.Join(dir, "external", "dep.abap"), "dep|")
	write(deps, "../external/dep.abap\r\n")
	cwd := filepath.Join(dir, "other-cwd")
	if err := os.Mkdir(cwd, 0755); err != nil {
		t.Fatal(err)
	}
	runCLI := func(args ...string) (string, error) {
		cmd := exec.Command(binary, args...)
		cmd.Dir = cwd
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	args := []string{"--file", file, "--config", config, "--deps", deps}
	for _, extra := range [][]string{nil, {"-allow-read", filepath.Join(dir, "external"), "--allow-read", filepath.Join(dir, "input")}} {
		out, err := runCLI(append(append([]string{}, args...), extra...)...)
		if err != nil || out != "file|dep|config|\n" {
			t.Fatalf("CLI: %v\n%s", err, out)
		}
	}
	// An explicitly named file's directory grants no authority to its symlink target.
	link := filepath.Join(dir, "input", "linked.abap")
	if err := os.Symlink(filepath.Join(dir, "external", "dep.abap"), link); err != nil {
		t.Logf("symlink unavailable: %v", err)
		return
	}
	out, err := runCLI("--file", link, "--config", config)
	if err == nil || !strings.Contains(out, "refused: read access denied for") || !strings.Contains(out, "-allow-read") || strings.Contains(out, "panic:") {
		t.Fatalf("refusal: %v\n%s", err, out)
	}
	t.Log("refusal:", strings.TrimSpace(out))
	out, err = runCLI("--file", link, "--config", config, "-allow-read", filepath.Join(dir, "external"))
	if err != nil || out != "dep|config|\n" {
		t.Fatalf("explicit allow: %v\n%s", err, out)
	}
}
