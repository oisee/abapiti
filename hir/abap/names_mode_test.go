package abap

import (
	"reflect"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestEmitNamesMode(t *testing.T) {
	t.Setenv("ABAPITI_NAMES", "")
	defaults, defaultNames, err := EmitNamed(fixtures()[0].p)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ABAPITI_NAMES", "readable")
	readable, _, err := EmitNamed(fixtures()[0].p)
	if err != nil || !reflect.DeepEqual(defaults, readable) {
		t.Fatalf("default differs from readable: %v", err)
	}
	class := fixtures()[0].p.Classes[0].Name
	if !strings.HasPrefix(defaultNames.Get(class), "zcl_lnt_") {
		t.Fatalf("default class name: %s", defaultNames.Get(class))
	}
	t.Setenv("ABAPITI_NAMES", "hashed")
	hashed, names, err := EmitNamed(fixtures()[0].p)
	if err != nil || names.Get(class) != hir.NewNames().Get(class) || reflect.DeepEqual(defaults, hashed) {
		t.Fatalf("legacy hashed mode not preserved: %v", err)
	}
	t.Setenv("ABAPITI_NAMES", "typo")
	if _, _, err := EmitNamed(fixtures()[0].p); err == nil || !strings.Contains(err.Error(), "ABAPITI_NAMES") {
		t.Fatalf("invalid naming mode: %v", err)
	}
}
