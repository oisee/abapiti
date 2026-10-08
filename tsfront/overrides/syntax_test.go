package overrides

import "testing"

// Every syntax-closure override is a valid registry entry with exactly one
// builder, a rationale and a full fingerprint; the inventory lists them all.
func TestSyntaxOverrides(t *testing.T) {
	entries := Syntax()
	if len(entries) != 10 {
		t.Fatalf("expected 10 syntax overrides, got %d", len(entries))
	}
	r, err := New(entries...)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range r.Inventory() {
		if e.Method != nil {
			if m := e.Method(); m == nil || m.Name == "" {
				t.Fatalf("%s: method builder returned no method", e.ID)
			}
		}
		if e.Interface != nil {
			if i := e.Interface(); i == nil || len(i.Methods) == 0 {
				t.Fatalf("%s: interface builder returned no members", e.ID)
			}
		}
		for span, build := range e.Expressions {
			if build() == nil {
				t.Fatalf("%s: expression builder for %q returned nil", e.ID, span)
			}
		}
	}
	if _, err := New(append(entries, Abaplint().Inventory()...)...); err != nil {
		t.Fatalf("syntax overrides collide with the abaplint set: %v", err)
	}
}
