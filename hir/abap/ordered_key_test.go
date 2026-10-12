package abap

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestOrderedCollectionsHashedSecondaryKey(t *testing.T) {
	for _, typ := range []hir.Type{
		hir.T(hir.OrderedMap, hir.T(hir.String), hir.T(hir.I64)),
		hir.T(hir.OrderedSet, hir.T(hir.I64)),
		hir.T(hir.OrderedMap, hir.Ref("Key"), hir.T(hir.I64)),
		hir.T(hir.OrderedSet, hir.Ref("Key")),
	} {
		t.Run(typ.String(), func(t *testing.T) {
			e := &emitter{types: map[string]bool{}, files: map[string]string{}, names: hir.NewNames()}
			e.runtime(typ)
			text := e.files[e.name("runtime."+typ.String())+".clas.abap"]
			if typ.Args[0].IsRef() {
				if strings.Contains(text, "HASHED KEY") || !strings.Contains(text, "READ TABLE entries WITH KEY k = p0") {
					t.Error("reference keys must retain identity lookups without an unsupported secondary key")
				}
				if typ.Kind == hir.OrderedSet && !strings.Contains(text, "DELETE entries INDEX sy-tabix.") {
					t.Error("linear reference-key reads must delete the matching index")
				}
				return
			}
			for _, want := range []string{
				"WITH UNIQUE HASHED KEY by_k COMPONENTS k.",
				"READ TABLE entries WITH KEY by_k COMPONENTS k = p0 TRANSPORTING NO FIELDS.",
				"READ TABLE entries WITH KEY by_k COMPONENTS k = p0 ASSIGNING",
			} {
				if !strings.Contains(text, want) {
					t.Errorf("missing %q", want)
				}
			}
			if typ.Kind == hir.OrderedSet {
				if !strings.Contains(text, "DELETE TABLE entries WITH TABLE KEY by_k COMPONENTS k = p0.") || strings.Contains(text, "DELETE entries INDEX sy-tabix.") {
					t.Error("hashed reads set sy-tabix to zero; delete must use the key")
				}
			}
		})
	}
}
