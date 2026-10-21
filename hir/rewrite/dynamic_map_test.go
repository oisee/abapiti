package rewrite

import "testing"

func TestDynamicMapBridgeEffects(t *testing.T) {
	for _, op := range []string{"dynamic.mapView", "map.checkedGet", "map.checkedSet", "map.checkedValues"} {
		e, ok := RuntimeEffects[op]
		if !ok || !e.Reads.Receiver || !e.Allocates || e.MayRaise != "Catchable" || e.Aliases != "Receiver" {
			t.Fatal(op, e)
		}
	}
	for _, op := range []string{"map.get", "map.set", "map.values"} {
		if RuntimeEffects[op].MayRaise != "None" {
			t.Fatal("ordinary operation contract changed", op)
		}
	}
}
