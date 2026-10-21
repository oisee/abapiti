package golang

import (
	"regexp"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// A drifted needle must fail loudly instead of silently skipping its patch:
// after the join rewrite (486f6cc7) the sparse join patch stopped matching and
// [1,,3].join(",") produced "1,0,3" with no error.
func TestRuntimePatchNeedleDriftPanics(t *testing.T) {
	brokenJoin := strings.Replace(runtimeSource, "b.WriteString(string(primitiveString(v)))", "b.WriteString(primitiveString(v))", 1)
	expectPatchPanic(t, func() { sparseRuntime(brokenJoin) })

	brokenPush := strings.Replace(runtimeSource, "func (a *array[T]) push(v T) int32 {", "func (a *array[T]) push(v T) int64 {", 1)
	expectPatchPanic(t, func() { liveArrayRuntime(sparseRuntime(brokenPush)) })
}

func expectPatchPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		msg, ok := recover().(string)
		if !ok || !strings.Contains(msg, "runtime patch target moved") {
			t.Fatalf("patcher panicked with %v, want a moved-target panic", msg)
		}
	}()
	f()
}

// Live views always run after sparseRuntime, so the dense one-line signatures
// are already rewritten and the chaiLength needle is absent without chai.
func TestLiveArrayRuntimeSparseOnly(t *testing.T) {
	r := liveArrayRuntime(sparseRuntime(runtimeSource))
	if strings.Contains(r, "chaiLength() int {if a.isLive()") {
		t.Fatal("chaiLength guard applied although the chai runtime is absent")
	}
	if !strings.Contains(r, "func (a *array[T]) push(v T) int32 {if a.isLive()") {
		t.Fatal("live push guard missing")
	}
}

// needleGuardProgram enables every runtime that patches base runtime text:
// live views, sparse holes and match groups (arrayExtraProgram's features),
// chai deep equality, readonly containers with map proxies, the collection
// probe, set iteration, map insertion counting and nonfinite numbers.
func needleGuardProgram() *hir.Program {
	dyn := hir.T(hir.Dynamic)
	strs := hir.T(hir.Array, str)
	sets := hir.T(hir.OrderedSet, str)
	maps := hir.T(hir.OrderedMap, str, i32)
	nums := hir.T(hir.Number)
	static := func(name string, result hir.Type, params []hir.Param, body *hir.Stmt) *hir.Method {
		return &hir.Method{Name: name, Static: true, Result: result, Params: params, Body: body}
	}
	return &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{
		static("parse", dyn, []hir.Param{{Name: "text", Type: str}},
			hir.B(ret(rt("xml.parseSubset", hir.V("text", str), dyn)))),
		static("groups", dyn, []hir.Param{{Name: "m", Type: strs}},
			hir.B(ret(rt("array.matchGroups", hir.V("m", strs), dyn)))),
		static("values", hir.T(hir.Void), []hir.Param{{Name: "d", Type: dyn}},
			hir.B(run(rt("dynamic.values", hir.V("d", dyn), hir.T(hir.Array, dyn))))),
		static("deep", boolean, []hir.Param{{Name: "a", Type: dyn}, {Name: "b", Type: dyn}},
			hir.B(ret(rt("chai.deepEqual", hir.V("a", dyn), boolean, hir.V("b", dyn))))),
		static("has", boolean, []hir.Param{{Name: "s", Type: sets}, {Name: "k", Type: dyn}},
			hir.B(ret(rt("collection.has", hir.V("s", sets), boolean, hir.V("k", dyn))))),
		static("iteration", strs, []hir.Param{{Name: "s", Type: sets}},
			hir.B(ret(rt("set.iterationValues", hir.V("s", sets), strs)))),
		static("insertions", i32, []hir.Param{{Name: "m", Type: maps}},
			hir.B(ret(rt("map.insertions", hir.V("m", maps), i32)))),
		static("nan", boolean, []hir.Param{{Name: "x", Type: nums}},
			hir.B(ret(rt("number.isNaN", hir.V("x", nums), boolean)))),
	}}}}
}

// Every patcher must run over the full-feature runtime without a panic, and
// the patches must actually be present in the emitted bytes.
func TestRuntimePatchersFullConfig(t *testing.T) {
	files, err := Emit(needleGuardProgram())
	if err != nil {
		t.Fatal(err)
	}
	runtime := files["runtime.go"]
	// Struct-field needles are regexps: gofmt splits the patched one-line
	// struct declarations and aligns their field columns.
	for _, want := range []string{
		`\n\tlive +liveArrayStorage\n`,
		`\n\tholes +map\[int\]bool\n`,
		`\n\tMatchGroups\s+\*dynamic\n`,
		`chaiLength\(\) int \{\s*if a\.isLive\(\)`,                         // live chaiLength guard (optional needle, present with chai)
		"enumerableRecord bool",                                            // chai deep records
		`readProxy \*setReadProxy\[T\]`,                                    // readonly containers
		`proxy\s+\*mapProxy`,                                               // map views
		`iteration \*array\[T\]`,                                           // set iteration
		`\n\tinserts\s+int32\n`,                                            // map insertion counting
		`func numberString\(x float64\) jsString \{\s*if math\.IsNaN\(x\)`, // nonfinite numbers
		`jsMap\s+bool`,                                                     // collection probe
		`return dynEqual\(box\(a\), box\(b\)\)`,                            // live equal dispatch
		`if a\.has\(int32\(i\)\) \{\s*b\.WriteString\(string\(primitiveString\(v\)\)\)`, // sparse join
	} {
		if !regexp.MustCompile(want).MatchString(runtime) {
			t.Fatalf("full-feature runtime lacks %q", want)
		}
	}
}
