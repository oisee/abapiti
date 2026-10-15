package tsfront

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var runnerMethod = regexp.MustCompile(`(?ms)^\s*METHOD (\w+)\.\n(.*?)^\s*ENDMETHOD\.`)
var runnerChunkCall = regexp.MustCompile(`(?m)^\s*(init_\w+)\( skip_init = skip_init \)\.\n`)

func flattenedShard(t *testing.T, source, shard string) string {
	t.Helper()
	methods := map[string]string{}
	for _, m := range runnerMethod.FindAllStringSubmatch(source, -1) {
		methods[m[1]] = m[2]
	}
	var flatten func(string) string
	flatten = func(name string) string {
		body, ok := methods[name]
		if !ok {
			t.Fatalf("missing helper %s", name)
		}
		return runnerChunkCall.ReplaceAllStringFunc(body, func(call string) string { return flatten(runnerChunkCall.FindStringSubmatch(call)[1]) })
	}
	return flatten(testShardMethod(shard))
}

func TestAbaplintTestShardSplitEquivalence(t *testing.T) {
	for _, shard := range []string{"abap", "abap/statements", "objects/rename", "UPPER", strings.Repeat("x", 80)} {
		method := testShardMethod(shard)
		if len(method) > 30 || !testShardIdentifier.MatchString(method) {
			t.Fatal(method)
		}
		var blocks []string
		// 4,962 ordered registration boundaries, including literals with periods
		// and declaration names which must not be rewritten by splitting.
		for i := 1; i <= 4962; i++ {
			blocks = append(blocks, fmt.Sprintf("IF skip_init NS ',%d,'.\nTRY.\nDATA(host_message_%d) = host_init_%d( ).\ncheckpoint( `test.%d` ).\nCATCH cx_root INTO init_error.\nerror_text = init_error->get_text( ).\nENDTRY.\nENDIF.\n", i, i, i, i))
		}
		decl, methods := testShardMethods(method, len(blocks), blocks)
		want := fmt.Sprintf("checkpoint( '{\"kind\":\"plan\",\"initializations\":%d}' ).\n", len(blocks)) + strings.Join(blocks, "")
		if got := flattenedShard(t, methods, shard); got != want {
			t.Fatalf("%s: flattened statement sequence differs", shard)
		}
		for _, m := range runnerMethod.FindAllStringSubmatch(methods, -1) {
			if strings.Count(m[2], "\n") > 5000 {
				t.Fatalf("overlarge method %s", m[1])
			}
			if !strings.Contains(decl, "CLASS-METHODS "+m[1]+" ") {
				t.Fatal("missing declaration", m[1])
			}
		}
	}
}
