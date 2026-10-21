package rewrite

import "testing"

func TestObjectBridgeEffects(t *testing.T) {
	e, ok := RuntimeEffects["dynamic.asObject"]
	if !ok || !e.Reads.Receiver || e.Reads.Args || e.Reads.Global || e.Writes != "None" || e.Allocates || e.MayRaise != "Catchable" || e.Aliases != "Receiver" {
		t.Fatal(e)
	}
	if RuntimeEffects["dynamic.asRef"].MayRaise != "Catchable" || RuntimeEffects["dynamic.classView"].MayRaise != "Trap" {
		t.Fatal("old reference contracts changed")
	}
}
