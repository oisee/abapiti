package abap

import (
	"strings"
	"testing"
)

func TestInstanceOfInitialReference(t *testing.T) {
	p := fixtures()[2].p
	defaultFiles, err := Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	concat := func(files map[string]string) string {
		var all strings.Builder
		for _, src := range files {
			all.WriteString(src)
		}
		return all.String()
	}
	modern := concat(defaultFiles)
	if !strings.Contains(modern, "IS BOUND AND") || !strings.Contains(modern, "IS INSTANCE OF") || strings.Contains(modern, "narrowed ?=") {
		t.Fatal("default InstanceOf must reject initial references before the static-type test")
	}
}
