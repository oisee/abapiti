package golang

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// executeImports is execute for fixture mains that need runtime, strings or
// time to pin allocation counts and long-input facts, not just output bytes.
func executeImports(t *testing.T, p *hir.Program, imports []string, main string) string {
	t.Helper()
	files, err := Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Emit(p)
	if err != nil || !reflect.DeepEqual(files, again) {
		t.Fatal("nondeterministic emission", err)
	}
	dir := t.TempDir()
	files["go.mod"] = "module fixture\n\ngo 1.26.0\n"
	files["main.go"] = "package main\nimport (\n\"" + strings.Join(imports, "\"\n\"") + "\"\n)\nfunc main(){" + main + "}\n"
	goFiles := map[string]string{}
	for n, s := range files {
		if strings.HasSuffix(n, ".go") {
			goFiles[n] = s
		}
	}
	if err := directCollectionTypes(goFiles); err != nil {
		t.Fatal(err)
	}
	for n, s := range goFiles {
		files[n] = s
	}
	for n, s := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s\ngenerated module: %s", err, out, dir)
	}
	return string(out)
}

func TestConfigJSONQuoteBytes(t *testing.T) {
	// Pins the exact quoted bytes, including the UTF-16LE unit layout of every
	// emitted piece: escapes, raw control units, surrogate halves and pairs,
	// plus the joined array and object writers.
	main := `q:=func(s jsString) string {return jsonQuote(s).String()}
	fmt.Println(q(str("")));fmt.Println(q(str("hello world")));fmt.Println(q(str("\"")));fmt.Println(q(str("\\")))
	fmt.Println(q(str("\b\t\n\f\r")));fmt.Println(q(unit(0x0001)+unit(0x001f)));fmt.Println(q(str("😀")))
	fmt.Println(q(unit(0xd800)));fmt.Println(q(unit(0xdc00)));fmt.Println(q(unit(0xd83d)+str("x")))
	a:=&array[jsString]{};a.push(str("x\"y"));a.push(str("\b\t\n\f\r"));a.push(unit(0xdc00));a.push(str(""));a.push(unit(0xd83d)+unit(0xde00))
	fmt.Println(stringifyConfig(a).String())
	m:=&orderedMap[jsString,any]{};m.set(str("zz"),unit(0x0001)+unit(0x001f));m.set(str("10"),box(false));m.set(str("2"),box(int32(2)));m.set(str("01"),box(int32(1)));m.set(str("b\"c"),str("v"));m.set(str("gone"),nil)
	fmt.Println(stringifyConfig(m).String())
	chunk:=str("a\"b\\c\nd")+unit(0xd800)+str("😀");mixed:=jsString(strings.Repeat(string(chunk),16384));out:=jsonQuote(mixed)
	if out.length()!=2+18*16384 {panic("long mixed length")};if out.substring(1,19)!=jsonQuote(chunk).substring(1,19) {panic("long mixed head bytes")}
	big:=str(strings.Repeat("a",131072));if jsonQuote(big)!=str("\""+strings.Repeat("a",131072)+"\"") {panic("long ascii bytes")};fmt.Println(out.length(),jsonQuote(big).length())`
	want := "\"\"\n\"hello world\"\n\"\\\"\"\n\"\\\\\"\n\"\\b\\t\\n\\f\\r\"\n\"\\u0001\\u001f\"\n\"😀\"\n\"\\ud800\"\n\"\\udc00\"\n\"\\ud83dx\"\n" +
		"[\"x\\\"y\",\"\\b\\t\\n\\f\\r\",\"\\udc00\",\"\",\"😀\"]\n" +
		"{\"2\":2,\"10\":false,\"zz\":\"\\u0001\\u001f\",\"01\":1,\"b\\\"c\":\"v\"}\n" +
		"294914 131074\n"
	if got := executeImports(t, configJSONFixture(), []string{"fmt", "strings"}, main); got != want {
		t.Fatalf("got %q; want %q", got, want)
	}
}

func TestConfigJSONQuoteLinear(t *testing.T) {
	// Quoting a 100k-unit string must allocate a bounded number of times, not
	// once per unit: the writer is one buffer, so long configs stay linear.
	main := `var m0,m1 runtime.MemStats;big:=str(strings.Repeat("a",131072));last:=jsString("")
	runtime.GC();runtime.ReadMemStats(&m0);for i:=0;i<10;i++ {last=jsonQuote(big)};runtime.ReadMemStats(&m1)
	fmt.Println("allocs",m1.Mallocs-m0.Mallocs);if m1.Mallocs-m0.Mallocs>2000 {panic("jsonQuote allocates per unit")};fmt.Println(last.length())`
	got := executeImports(t, configJSONFixture(), []string{"fmt", "runtime", "strings"}, main)
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 || lines[1] != "131074" {
		t.Fatalf("got %q", got)
	}
	if n, err := strconv.Atoi(strings.TrimPrefix(lines[0], "allocs ")); err != nil || n > 2000 {
		t.Fatalf("allocations %q: jsonQuote is not linear", lines[0])
	}
}

func configJSONReplacerFixture() *hir.Program {
	p := configJSONFixture()
	absentFn := &hir.Expr{Kind: hir.Lit, Type: hir.T(hir.Optional, hir.T(hir.Dynamic))}
	absentKeys := &hir.Expr{Kind: hir.Lit, Type: hir.T(hir.Optional, hir.T(hir.Array, hir.T(hir.String)))}
	op := &hir.Expr{Kind: hir.RuntimeOp, Op: "json.stringifyReplacer", Type: hir.T(hir.Optional, hir.T(hir.String)), X: hir.L(hir.T(hir.String), ""), Args: []*hir.Expr{absentFn, absentKeys, hir.L(hir.T(hir.String), "")}}
	p.Classes[0].Methods = append(p.Classes[0].Methods, &hir.Method{Name: "repl", Static: true, Result: op.Type, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: op})})
	return p
}

func TestConfigJSONReplacer(t *testing.T) {
	// The replacer runtime patches the writer in place; both patched loops must
	// keep the exact bytes and the own-key order of the plain writer.
	main := `d:=parseJSON(str("{\"b\":\"x\",\"10\":10,\"2\":2}"));fn:=func(k jsString,v *dynamic)*dynamic {if k==str("b") {return nil};return v}
	r:=stringifyReplacer(d,fn,nil,str(""));if !r.Has {panic("replacer dropped the root")};fmt.Println(r.Value.String())
	keys:=&array[jsString]{};keys.push(str("2"));keys.push(str("b"))
	k:=stringifyReplacer(d,nil,keys,str(""));if !k.Has {panic("key list dropped the root")};fmt.Println(k.Value.String())`
	want := "{\"2\":2,\"10\":10}\n{\"2\":2,\"b\":\"x\"}\n"
	if got := execute(t, configJSONReplacerFixture(), main); got != want {
		t.Fatalf("got %q; want %q", got, want)
	}
}
