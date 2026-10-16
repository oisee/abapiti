package golang

import (
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

func init() {
	for _, op := range []string{"clock.epochMilliseconds", "weak.lookupKey", "weak.writeKey", "map.insertions"} {
		supportedOps[op] = true
	}
}

func (b *body) gogenRuntime(x *hir.Expr) (string, bool) {
	if code, ok := b.nodePlatformRuntime(x); ok {
		return code, true
	}
	switch x.Op {
	case "clock.epochMilliseconds":
		return "epochMilliseconds()", true
	case "map.insertions":
		if b.e.usesMapViews() {
			return b.expr(x.X) + ".mapInsertions()", true
		}
		return b.expr(x.X) + ".inserts", true
	case "weak.lookupKey", "weak.writeKey":
		fail := "func() {}"
		if x.Op == "weak.writeKey" {
			b.e.usesTypedCatch = true
			t := hir.Ref("builtin.TypeError")
			fail = "func(){panic(payload[" + b.e.typ(t) + "]{Type:" + strconv.Quote(t.String()) + ",Value:" + b.e.name("new."+t.Name) + "(present(str(\"Invalid value used as weak map key\")))})}"
		}
		return "weakIdentityKey(" + b.expr(x.X) + "," + fail + ")", true
	}
	return "", false
}

// mapInsertionsRuntime counts new keys in both map representations, only for
// programs that read the counter: pinned generated sources stay byte-identical.
func (e *emitter) mapInsertionsRuntime(runtime string) string {
	if _, ok := hir.UsedRuntimeOps(e.p)["map.insertions"]; !ok {
		return runtime
	}
	for _, patch := range [][2]string{
		{"type orderedMap[K comparable, V any] struct {", "type orderedMap[K comparable, V any] struct { inserts int32;"},
		{" m.index[key]=len(m.Entries)\n", " m.inserts++;m.index[key]=len(m.Entries)\n"},
	} {
		if !strings.Contains(runtime, patch[0]) {
			panic("map insertion runtime patch target moved: " + patch[0])
		}
		runtime = strings.Replace(runtime, patch[0], patch[1], 1)
	}
	return runtime
}

// Append only for programs using weak collections: pinned generated sources
// retain their byte-for-byte runtime.
func (e *emitter) gogenRuntimeSource() string {
	ops := hir.UsedRuntimeOps(e.p)
	clock := ""
	if _, ok := ops["clock.epochMilliseconds"]; ok {
		clock = "\nfunc epochMilliseconds() float64 { return float64(time.Now().UnixMilli()) }\n"
	}
	if _, ok := ops["weak.lookupKey"]; !ok {
		if _, ok = ops["weak.writeKey"]; !ok {
			return clock
		}
	}
	return clock + `
func weakIdentityKey(d *dynamic, fail func()) any {
 if d == nil || d.Tag != 2 && d.Tag != tagArray && d.Tag != 10 || nilRef(d.Value) { fail(); return nil }
 return d.Value
}
`
}
