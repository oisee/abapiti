package gracecheck

import (
	"testing"

	"github.com/oisee/abapiti/hir/rewrite"
)

func TestMethodFlowReference(t *testing.T) {
	for _, crossMethod := range []bool{false, true} {
		base := rewrite.NewDB()
		add := func(pred string, values ...string) {
			t.Helper()
			if err := base.Add(pred, values...); err != nil {
				t.Fatal(err)
			}
		}
		for _, m := range []string{"A", "B"} {
			for _, site := range []string{"entry", "read", "kill", "last"} {
				add("flow_node", m, m+"/"+site)
			}
			add("def", m+"/v", m+"/entry")
			add("use", m+"/v", m+"/read")
			add("observed", m+"/v", m+"/read")
			add("def", m+"/v", m+"/kill")
			add("use", m+"/v", m+"/last")
			add("observed", m+"/v", m+"/last")
			add("next", m+"/entry", m+"/read")
			add("next", m+"/read", m+"/kill")
			add("next", m+"/kill", m+"/last")
			add("next", m+"/last", m+"/read")
		}
		if crossMethod {
			// This graph is not partitionable; the generic reference must win.
			add("next", "A/read", "B/read")
		}
		want, _, err := Reference(base, localFlowRules, 1000000)
		if err != nil {
			t.Fatal(err)
		}
		got, _, err := ReferenceFull(base, localFlowRules, 1000000)
		if err != nil {
			t.Fatal(err)
		}
		Equal(t, got, want)
		if !got.Has("not_read_after", "A/v", "A/read") || got.Has("not_read_after", "A/v", "A/last") {
			t.Fatal("partition lost assignment kills or loop back-edge reads")
		}
	}
}
