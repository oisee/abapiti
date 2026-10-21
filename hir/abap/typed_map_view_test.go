package abap

import (
	"github.com/oisee/abapiti/hir"
	"regexp"
	"strings"
	"testing"
)

func TestMapBoxedKeyView(t *testing.T) {
	srcType := hir.T(hir.OrderedMap, str, str)
	dst := hir.T(hir.OrderedMap, hir.T(hir.Dynamic), str)
	view := rt("map.view", newObj(srcType), dst)
	src := u3Emit(t, rt("map.get", view, hir.T(hir.Optional, str), rt("dynamic.of", hir.L(str, "a"), hir.T(hir.Dynamic))), rt("map.has", view, boolean, rt("dynamic.of", hir.L(hir.T(hir.Number), 1), hir.T(hir.Dynamic))), rt("map.keys", view, hir.T(hir.Array, hir.T(hir.Dynamic))), rt("map.insertions", view, hir.T(hir.I32)), rt("map.clear", view, hir.T(hir.Void)), rt("map.delete", view, boolean, rt("dynamic.of", hir.L(hir.T(hir.Optional, str), nil), hir.T(hir.Dynamic))), rt("map.set", view, dst, rt("dynamic.of", hir.L(str, "a"), hir.T(hir.Dynamic)), hir.L(str, "v")))

	checkViewLocals(t, src)
	u3Want(t, src, "->view_source ?=", "->view_box =", "IF view_source IS BOUND.", "view_source->get(", "= view_source->has(", "= view_source->keys( ).", "= view_source->insertions( ).", "view_source->clear( ).", "view_source->remove(", "IF p0 IS NOT BOUND.\nRETURN.", "writes through a boxed string-key Map view are unsupported", "->tag <> ")
}

func TestMapStructuralValueView(t *testing.T) {
	a, b := hir.Ref("A"), hir.Ref("B")
	for _, key := range []hir.Type{str, boolean, hir.T(hir.I32), hir.T(hir.Number), hir.Ref(hir.RootObject), hir.T(hir.Array, str), hir.T(hir.ClassValue)} {
		from, to := hir.T(hir.OrderedMap, key, a), hir.T(hir.OrderedMap, key, b)
		m := method("read", hir.T(hir.Optional, b), ret(rt("map.get", rt("map.view", local("input", from), to), hir.T(hir.Optional, b), local("key", key))))
		m.Static = true
		m.Params = []hir.Param{{Name: "input", Type: from}, {Name: "key", Type: key}}
		p := &hir.Program{Classes: []*hir.Class{{Name: "A", Structural: true, Fields: []hir.Field{{Name: "name", Type: str}}}, {Name: "B", Structural: true, Fields: []hir.Field{{Name: "name", Type: str}}}, {Name: "Probe", Methods: []*hir.Method{m}}}}
		files, err := emitABAP(t, p)
		if err != nil {
			t.Fatal(err)
		}
		var out string
		for _, s := range files {
			checkViewLocals(t, s)
			out += s
			for _, line := range strings.Split(s, "\n") {
				if len(line) > 255 {
					t.Fatal(line)
				}
			}
		}
		u3Want(t, out, "DATA view_source TYPE REF TO", "IF view_source IS BOUND.", "view_source->get(", "view_source->put(", "view_source->remove(", "view_source->clear( ).", "LOOP AT", "->view_box IS BOUND.")
	}
}

func TestSetReferenceView(t *testing.T) {
	from, to := hir.T(hir.OrderedSet, hir.Ref(hir.RootObject)), hir.T(hir.OrderedSet, hir.Ref("Value"))
	m := method("view", to, ret(rt("set.view", local("input", from), to)))
	m.Static = true
	m.Params = []hir.Param{{Name: "input", Type: from}}
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{{Name: "Value", Structural: true, Fields: []hir.Field{{Name: "name", Type: str}}}, {Name: "Probe", Methods: []*hir.Method{m}}}})
	if err != nil {
		t.Fatal(err)
	}
	var runtime string
	for _, source := range files {
		checkViewLocals(t, source)
		runtime += source
	}
	u3Want(t, runtime, "->as_ref( ).", "READ TABLE entries WITH KEY k =", "APPEND VALUE #( k =", "METHOD has.", "METHOD delete.")
	out := files[names.Get("Probe")+".clas.abap"]
	if strings.Contains(out, "CREATE OBJECT") {
		t.Fatal("Set views must preserve original storage reference")
	}
	u3Want(t, out, "TYPE REF TO "+names.Get("runtime."+from.String()))
}

func TestUnusedMapViewRuntime(t *testing.T) {
	src := u3Emit(t, rt("map.size", newObj(hir.T(hir.OrderedMap, str, str)), hir.T(hir.I32)))
	if strings.Contains(src, "view_source") || strings.Contains(src, "view_box") {
		t.Fatal("unused Map view runtime emitted")
	}
}

// Protocol guards are composed into existing runtime methods. Their locals
// must not shadow declarations in the older string-map guard.
func checkViewLocals(t *testing.T, source string) {
	t.Helper()
	declaration := regexp.MustCompile(`^(?:DATA(?:\(|\s+)|CONSTANTS\s+)([a-zA-Z_0-9]+)`)
	var seen map[string]bool
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "METHOD ") {
			seen = map[string]bool{}
		}
		if line == "ENDMETHOD." {
			seen = nil
		}
		if seen == nil {
			continue
		}
		match := declaration.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if seen[match[1]] {
			t.Errorf("duplicate runtime local %s", match[1])
		}
		seen[match[1]] = true
	}
}
