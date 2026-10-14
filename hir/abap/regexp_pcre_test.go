package abap

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir/jsregex"
)

func TestPCREReviewedPatternSyntax(t *testing.T) {
	cases := map[string]string{
		`^a$`:                                 `(?<![\s\S])a(?![\s\S])`,
		`\Aa\z`:                               `(?<![\s\S])a(?![\s\S])`,
		`\\z`:                                 `\\z`,
		`[^^$]\^\$`:                           `[^^$]\^\$`,
		`[\x{0009}-\x{000D}\x{00A0}\x{2028}]`: "[\t-\r\u00a0\u2028]",
		`\b[dD][eE][fF][iI][nN][eE]`:          `\b[dD][eE][fF][iI][nN][eE]`,
		`\\x{2028}`:                           `\\x{2028}`,
		`a b#c[ #]`:                           `a[ ]b[#]c[ #]`,
		`\x{0020}\x{0023}\x{000B}`:            "[ ][#][\v]",
		`\x{005B}\x{005C}\x{0024}`:            `\[\\\$`,
		`[\x{005D}\x{002D}\x{005E}]`:          `[\]\-\^]`,
	}
	for in, want := range cases {
		if got := pcreRegexPattern(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

// Native PCRE/RE2 boundaries must retain JS non-u ASCII membership. Check
// both the subset translation and the reviewed DDL pattern's leading boundary.
func TestPCREWordBoundariesNodeOracle(t *testing.T) {
	type probe struct {
		JS, Flags, PCRE, Input string
	}
	probes := []probe{}
	for _, source := range []string{`\bdefine\s+behavior`, `\Bdefine\s+behavior`} {
		translated, ok := jsregex.TranslateSubset(source, "i")
		if !ok {
			t.Fatalf("subset refused %q", source)
		}
		pattern := pcreRegexPattern(translated)
		if !strings.HasPrefix(pattern, source[:2]) || strings.Contains(pattern, "(?<") {
			t.Fatalf("boundary is not native: %q", pattern)
		}
		for _, input := range []string{"define behavior", "xdefine behavior", "é define behavior", "_define behavior", "édefine behavior", "DEFINE BEHAVIOR", "Kdefine behavior", "ſdefine behavior"} {
			probes = append(probes, probe{source, "i", pattern, input})
		}
	}
	key := `\bdefine\s+(?:\w+\s+)*?behavior\s+for\s+([\w/]+)(?:\s+alias\s+(\w+))?/gi`
	for _, input := range []string{"define behavior for foo", "xdefine behavior for foo", "édefine behavior for foo", "_define behavior for foo"} {
		probes = append(probes, probe{strings.TrimSuffix(key, "/gi"), "i", pcreRegexPattern(jsregex.Reviewed[key][0]), input})
	}
	raw, err := json.Marshal(probes)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", "-e", `console.log(JSON.stringify(JSON.parse(process.argv[1]).map(p=>{const a=new RegExp(p.JS,p.Flags).test(p.Input);const b=new RegExp(p.PCRE).test(p.Input);if(a!==b)throw Error(JSON.stringify(p));return a})))`, string(raw)).CombinedOutput()
	if err != nil {
		t.Fatalf("Node oracle: %v\n%s", err, out)
	}
	var want []bool
	if err := json.Unmarshal(out, &want); err != nil {
		t.Fatal(err)
	}
	for i, p := range probes {
		re, err := regexp.Compile(p.PCRE)
		if err != nil {
			t.Fatal(err)
		}
		if got := re.MatchString(p.Input); got != want[i] {
			t.Errorf("/%s/%s on %q: RE2 %v != Node %v", p.JS, p.Flags, p.Input, got, want[i])
		}
	}
}

// These are the CDS lexer's normalization patterns and DDL compactTokens'
// whitespace language. Compare both the emitted spelling and RE2 with Node,
// including every JS whitespace codepoint and punctuation near hex escapes.
func TestPCRECharactersNodeOracle(t *testing.T) {
	input := "a\r\n\t\v\f \u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeffb\u0085c[]^-\\.$#"
	type probe struct {
		JS, PCRE, Input string
	}
	probes := []probe{}
	for _, key := range []string{`\u00a0/g`, `\u000b/g`, `\s+/g`} {
		pattern := pcreRegexPattern(jsregex.Reviewed[key][0])
		if strings.Contains(pattern, `\x`) {
			t.Fatalf("host-refused hex escape remains in %q", pattern)
		}
		probes = append(probes, probe{strings.TrimSuffix(key, "/g"), pattern, input})
	}
	probes = append(probes,
		probe{`[\x5d\x2d\x5e]`, pcreRegexPattern(`[\x{005D}\x{002D}\x{005E}]`), input},
		probe{`\x5b\x5d\x5e\x2d\\\.\$#`, pcreRegexPattern(`\x{005B}\x{005D}\x{005E}\x{002D}\x{005C}\x{002E}\x{0024}#`), "[]^-\\.$#"},
	)
	raw, err := json.Marshal(probes)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", "-e", `console.log(JSON.stringify(JSON.parse(process.argv[1]).map(p=>{const a=p.Input.match(new RegExp(p.JS,"g"))||[];const b=p.Input.match(new RegExp(p.PCRE,"g"))||[];if(JSON.stringify(a)!==JSON.stringify(b))throw Error(JSON.stringify(p));return a})))`, string(raw)).CombinedOutput()
	if err != nil {
		t.Fatalf("Node oracle: %v\n%s", err, out)
	}
	var want [][]string
	if err := json.Unmarshal(out, &want); err != nil {
		t.Fatal(err)
	}
	for i, p := range probes {
		re, err := regexp.Compile(p.PCRE)
		if err != nil {
			t.Fatal(err)
		}
		got := re.FindAllString(p.Input, -1)
		if !reflect.DeepEqual(got, want[i]) {
			t.Errorf("%q: RE2 %q != Node %q", p.JS, got, want[i])
		}
	}
}
