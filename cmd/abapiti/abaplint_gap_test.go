package main

import (
	"errors"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/tsfront"
)

func TestGapSummary(t *testing.T) {
	r := &tsfront.RegistryLowering{
		Blocking: []tsfront.LowerDiagnostic{
			{Category: "unsupported-call", Loc: "/tmp/work/src/abap/5_syntax/foo.ts:2:3", Message: "method trim on string is not lowered"},
			{Category: "unsupported-call", Loc: "src/abap/5_syntax/foo.ts:3:3", Message: "method padStart on string is not lowered"},
			{Category: "unsupported-expr", Loc: "src/rules/foo.ts:4:1", Message: "identifier failed is not a lowered local or module value", Cascade: true},
			{Category: "unsupported-type", Loc: "/tmp/work/node_modules/@scope/pkg/a.d.ts:1:1", Message: `checker type "Foo" is not a lowered declaration`},
		},
		Verification: []error{hir.Error{Node: hir.Node{Source: "src/rules/foo.ts:9:2"}}, hir.Error{Node: hir.Node{Source: "/tmp/work/src/objects/obj.ts:1:1"}}, errors.New("synthetic")},
	}
	s := summarizeGaps(r)
	if s.Blocking != 4 || s.RootBlocking != 3 || s.CascadeBlocking != 1 || s.HIRErrors != 3 || s.Files != 4 {
		t.Fatal(s)
	}
	if s.ByArea["abap/5_syntax"] != 2 || s.ByArea["node_modules/@scope/pkg"] != 1 || s.ByArea["rules"] != 1 {
		t.Fatal(s.ByArea)
	}
	if s.ByRootMessage[0].Message != "method X on string is not lowered" || s.ByRootMessage[0].Count != 2 {
		t.Fatal(s.ByRootMessage)
	}
	for _, pair := range [][2]string{
		{"identifier foo is not a lowered local or module value", "identifier bar is not a lowered local or module value"},
		{"assignment to foo is not lowered", "assignment to bar is not lowered"},
		{`checker type "Foo" is not lowered`, `checker type "Bar" is not lowered`},
	} {
		if normalizeGapMessage(pair[0]) != normalizeGapMessage(pair[1]) {
			t.Fatal(pair)
		}
	}
}
