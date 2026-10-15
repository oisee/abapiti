package golang

import (
	"strconv"

	"github.com/oisee/abapiti/hir"
)

func init() {
	for _, op := range []string{"clock.epochMilliseconds", "weak.lookupKey", "weak.writeKey"} {
		supportedOps[op] = true
	}
}

func (b *body) gogenRuntime(x *hir.Expr) (string, bool) {
	switch x.Op {
	case "clock.epochMilliseconds":
		return "epochMilliseconds()", true
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
