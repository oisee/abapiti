package golang

import (
	"encoding/json"
	"fmt"
	"github.com/oisee/abapiti/hir"
	"math/rand"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

func TestReviewedRegexNodeOracle(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node absent")
	}
	patterns := []string{}
	for p := range reviewedPatterns {
		patterns = append(patterns, p)
	}
	patterns = append(patterns, "&1/g", "&999/g", "^abs$/i", "^(CHAR|INT8|DECFLOAT34)$/i", "^LT?CL_/i")
	sort.Strings(patterns)
	inputs := []string{"", "a", "ZCL_FOO", "lcl_test", "/ABC/FOO", "/ab/abc", "*", "foo*", "FROM", "from", "DEPTH", "depth", "COUNT( * )", "123", "83", "84", "abc\n", "abc\r\n", "a\u2028c", "a\u2029c", "é", "K", "ſ", "😀", "\u00a0", "\ufeff", "'foo'", "`foo`", "a'\n'", "foo.clas.abap", "foo.screen_123.abap", "<foo>", "x/y"}
	alphabet := []rune("abcABC_0123/*~!%$#?&<>`'\n\r\téKſ😀\u00a0\ufeff")
	rng := rand.New(rand.NewSource(577875))
	for i := 0; i < 1000; i++ {
		var s strings.Builder
		for j, n := 0, rng.Intn(12); j < n; j++ {
			s.WriteRune(alphabet[rng.Intn(len(alphabet))])
		}
		inputs = append(inputs, s.String())
	}
	data, _ := json.Marshal(struct{ Patterns, Inputs []string }{patterns, inputs})
	script := `const {Patterns,Inputs}=JSON.parse(process.argv[1]);let out="";for(const p of Patterns){const i=p.lastIndexOf("/");const r=new RegExp(p.slice(0,i),p.slice(i+1));for(const s of Inputs){r.lastIndex=0;out+=r.test(s)?"1":"0"}}process.stdout.write(out)`
	want, err := exec.Command("node", "-e", script, string(data)).Output()
	if err != nil {
		t.Fatal(err)
	}
	var main strings.Builder
	main.WriteString("patterns:=[]string{")
	for _, p := range patterns {
		fmt.Fprintf(&main, "%q,", p)
	}
	main.WriteString("};inputs:=[]string{")
	for _, s := range inputs {
		fmt.Fprintf(&main, "%q,", s)
	}
	main.WriteString(`};for _,p:=range patterns{i:=strings.LastIndex(p,"/");r:=newRegExp(str(p[:i]),str(p[i+1:]));for _,s:=range inputs{r.LastIndex=0;if r.test(str(s)){fmt.Print("1")}else{fmt.Print("0")}}}`)
	// strings is already imported by the runtime file, but imports are per file.
	got := execute(t, &hir.Program{}, strings.ReplaceAll(main.String(), `strings.LastIndex(p,"/")`, `func()int{for i:=len(p)-1;i>=0;i--{if p[i]=='/'{return i}};panic("separator")}()`))
	if got != string(want) {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s input %q: Go %c Node %c", patterns[i/len(inputs)], inputs[i%len(inputs)], got[i], want[i])
			}
		}
		t.Fatal("length mismatch")
	}
	t.Logf("%d patterns x %d inputs equal to Node", len(patterns), len(inputs))
}
