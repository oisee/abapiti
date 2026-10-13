package tsfront

import (
	"strings"
	"testing"
)

func TestAsyncInheritedBridgeABAPAdoptsSettledPromise(t *testing.T) {
	p, ds := asyncProbe(t, `
class Base { async make():Promise<Base> { return this; } }
class Child extends Base { async make():Promise<Child> { return this; } }
export class Probe { static async run():Promise<Base> { const x:Base=new Child(); return await x.make(); } }
`)
	if hasBlocking(ds) {
		t.Fatal(ds)
	}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	src := files[names.Get("input.ts.Child")+".clas.abap"]
	start := strings.Index(src, "METHOD "+names.Get("member.make")+".")
	if start < 0 {
		t.Fatal("missing inherited slot", src)
	}
	body := src[start:]
	end := strings.Index(body, "ENDMETHOD.")
	if end < 0 {
		t.Fatal("missing slot body", body)
	}
	body = body[:end]
	for _, part := range []string{"->rejection IS BOUND.", "RAISE EXCEPTION", "->value", "CATCH"} {
		if !strings.Contains(body, part) {
			t.Fatalf("async bridge lost settled adoption (%s):\n%s", part, body)
		}
	}
}
