package abap

import (
	"github.com/oisee/abapiti/hir"
	"strings"
	"testing"
)

func TestMapInsertions(t *testing.T) {
	m := hir.T(hir.OrderedMap, str, str)
	src := u3Emit(t, rt("map.insertions", newObj(m), hir.T(hir.I32)), rt("map.set", newObj(m), m, hir.L(str, "a"), hir.L(str, "b")), rt("map.delete", newObj(m), boolean, hir.L(str, "a")), rt("map.clear", newObj(m), hir.T(hir.Void)))
	u3Want(t, src, "DATA inserts TYPE i.", "METHOD insertions.\nresult = inserts.", "<row>-v = p1.\nELSE.\ninserts = inserts + 1.\nAPPEND VALUE #( k = p0 v = p1 ) TO entries.", "CLEAR ")
	if strings.Count(src, "inserts = inserts + 1.") != 1 {
		t.Fatal("counter must only increment on a new key, never on overwrite/delete/clear")
	}
	plain := u3Emit(t, rt("map.size", newObj(m), hir.T(hir.I32)))
	if strings.Contains(plain, "inserts") || strings.Contains(plain, "METHOD insertions") {
		t.Fatal("unused counter emitted")
	}
}

func TestWeakIdentityKeys(t *testing.T) {
	dyn, obj := hir.T(hir.Dynamic), hir.Ref(hir.RootObject)
	for _, input := range []*hir.Expr{hir.L(str, "primitive"), hir.L(hir.T(hir.Number), 1), hir.L(boolean, true), hir.L(hir.T(hir.Optional, str), nil), newObj(hir.T(hir.Array, str))} {
		key := rt("dynamic.of", input, dyn)
		src := u3Emit(t, rt("weak.lookupKey", key, obj))
		u3Want(t, src, "->oval.", "->cval.", "tag_array.", "ENDCASE.\nENDIF.")
		if strings.Contains(src, "Invalid value used as weak map key") {
			t.Fatal("lookup must not throw for primitives or undefined")
		}
		src = u3Emit(t, rt("weak.writeKey", key, obj))
		u3Want(t, src, "Invalid value used as weak map key", "RAISE EXCEPTION ", "->payload = ")
	}
}
