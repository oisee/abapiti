package grace_test

import (
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/oisee/abapiti/grace"
)

// This adapter owns only toy function identities, with no dependency on HIR.
func TestToyAnalysisAdapter(t *testing.T) {
	source, err := os.ReadFile("../hir/rewrite/rules/analysis.grace")
	if err != nil {
		t.Fatal(err)
	}
	_, rules, err := grace.ParseRules(string(source))
	if err != nil {
		t.Fatal(err)
	}
	db := grace.NewDB()
	type function struct {
		name   string
		calls  []string
		raises bool
	}
	ir := []function{{"entry", []string{"read"}, false}, {"read", nil, true}, {"idle", nil, false}}
	for _, f := range ir {
		for _, pred := range []string{"method", "defined"} {
			if err := db.Add(pred, f.name); err != nil {
				t.Fatal(err)
			}
		}
		for _, callee := range f.calls {
			if err := db.Add("calls", f.name, callee, f.name+"/call"); err != nil {
				t.Fatal(err)
			}
		}
		if f.raises {
			if err := db.Add("unknown_effect", f.name); err != nil {
				t.Fatal(err)
			}
			if err := db.Add("throws", f.name, "IO"); err != nil {
				t.Fatal(err)
			}
		}
	}
	selected, needed := grace.SelectDemand([]*grace.Rules{rules}, []string{"may_throw", "pure"})
	if !needed["impure"] || !needed["calls"] {
		t.Fatal("missing negative/transitive dependency")
	}
	if err := grace.Evaluate(db, selected); err != nil {
		t.Fatal(err)
	}
	if got := db.Facts("may_throw"); !reflect.DeepEqual(got, []grace.Tuple{{"entry"}, {"read"}}) {
		t.Fatal(got)
	}
	if got := db.Facts("pure"); !reflect.DeepEqual(got, []grace.Tuple{{"idle"}}) {
		t.Fatal(got)
	}
	before := grace.Report(db)
	if err := grace.Evaluate(db, selected); err != nil {
		t.Fatal(err)
	}
	if got := grace.Report(db); got != before {
		t.Fatal("report changed after reevaluation")
	}
}

func TestNoHIRDependency(t *testing.T) {
	// Inspect the transitive production dependency graph, not only source imports.
	cmd := exec.Command("go", "list", "-deps", "github.com/oisee/abapiti/grace")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, output)
	}
	for _, dep := range strings.Fields(string(output)) {
		if dep == "github.com/oisee/abapiti/hir" || strings.HasPrefix(dep, "github.com/oisee/abapiti/hir/") {
			t.Fatalf("grace depends on %s", dep)
		}
	}
}
