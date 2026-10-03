package wasm

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const progsDir = "testdata/progs"

type progsCheck struct {
	prog string
	c    osdCase
}

func progsChecks(t *testing.T) []progsCheck {
	t.Helper()
	f, err := os.Open(filepath.Join(progsDir, "cases.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var checks []progsCheck
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) < 2 {
			continue
		}
		var args []int32
		for _, a := range fs[2:] {
			v, err := strconv.ParseInt(a, 10, 32)
			if err != nil {
				t.Fatalf("cases.txt: %q: %v", sc.Text(), err)
			}
			args = append(args, int32(v))
		}
		checks = append(checks, progsCheck{fs[0], osdCase{fs[1], args}})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return checks
}

func progsNative(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(progsDir, "native-results.txt"))
	if err != nil {
		t.Fatal(err)
	}
	native := map[string]string{}
	for _, l := range strings.Split(string(data), "\n") {
		if i := strings.Index(l, " = "); i > 0 {
			native[l[:i]] = l[i+3:]
		}
	}
	return native
}

// TestProgsCorpus runs the committed C corpus in wazero against the native
// answers. With ABAPITI_TEST_OUT set it exports each program as a module, a
// generated class and an ABAP Unit test class expecting the native answers.
func TestProgsCorpus(t *testing.T) {
	checks := progsChecks(t)
	native := progsNative(t)
	if len(checks) != 38 {
		t.Fatalf("cases.txt has %d checks, want 38", len(checks))
	}
	byProg := map[string][]osdCase{}
	var order []string
	for _, c := range checks {
		if _, ok := byProg[c.prog]; !ok {
			order = append(order, c.prog)
		}
		byProg[c.prog] = append(byProg[c.prog], c.c)
	}
	out := os.Getenv("ABAPITI_TEST_OUT")
	if out != "" {
		out = filepath.Join(out, t.Name())
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range order {
		bin, err := os.ReadFile(filepath.Join(progsDir, "wasm", p+".wasm"))
		if err != nil {
			t.Fatal(err)
		}
		cases := byProg[p]
		got := wazeroResults(t, bin, cases)
		for i, c := range cases {
			key := p + " " + c.fn
			for _, a := range c.args {
				key += fmt.Sprintf(" %d", a)
			}
			want, ok := native[key]
			if !ok {
				t.Errorf("%s: no native answer", key)
				continue
			}
			if got[i].trap || fmt.Sprint(got[i].value) != want {
				t.Errorf("%s: wazero %v, native %s", key, got[i], want)
			}
		}
		mod, err := Parse(bin)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		cls := "zcl_abapiti_t_" + p
		src := mustCompile(t, mod, cls)
		if out == "" {
			continue
		}
		files := map[string][]byte{
			p + ".wasm":                    bin,
			cls + ".clas.abap":             []byte(src),
			cls + ".clas.testclasses.abap": []byte(osdTestClass(cls, cases, got)),
		}
		for name, data := range files {
			if err := os.WriteFile(filepath.Join(out, name), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
