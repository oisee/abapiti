package jsregex

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"regexp"
	"testing"
)

func TestLazyCapturesMatchNode(t *testing.T) {
	patterns := []string{`a*?`, `a+?`, `a??`, `a{1,3}?`, `(a??)a`, `(.+?)X`, `^(.+?)\s+IS\s+(?:NOT\s+)?INITIAL\s*$`}
	inputs := []string{"", "a", "aaaa", "aaXaaX", "abc IS INITIAL", "abc IS NOT INITIAL", "a IS INITIAL IS INITIAL", "a\u00a0IS INITIAL", "a\nIS INITIAL", "a\u2028IS INITIAL", "aaa IS INITIAL "}
	request, _ := json.Marshal(map[string]any{"patterns": patterns, "inputs": inputs})
	command := exec.Command("node", "-e", `const q=JSON.parse(process.argv[1]);console.log(JSON.stringify(q.patterns.map(p=>q.inputs.map(s=>new RegExp(p).exec(s)))))`, string(request))
	raw, err := command.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(raw))
	}
	var want [][][]string
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	for i, pattern := range patterns {
		translated, ok := TranslateSubset(pattern, "")
		if !ok {
			t.Fatalf("refused %s", pattern)
		}
		re, err := regexp.Compile(translated)
		if err != nil {
			t.Fatal(err)
		}
		for j, input := range inputs {
			if got := re.FindStringSubmatch(input); !reflect.DeepEqual(got, want[i][j]) {
				t.Fatalf("/%s/ on %q: Go %q Node %q", pattern, input, got, want[i][j])
			}
		}
	}
}
