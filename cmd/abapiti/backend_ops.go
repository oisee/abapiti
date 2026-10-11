package main

import (
	"errors"
	"github.com/oisee/abapiti/hir"
	abaphir "github.com/oisee/abapiti/hir/abap"
	gohir "github.com/oisee/abapiti/hir/golang"
)

func checkTargetRuntimeOps(p *hir.Program, targets map[string]bool) error {
	var failures []error
	if targets["go"] {
		failures = append(failures, hir.CheckRuntimeOps(p, "go", gohir.SupportsRuntimeOp)...)
	}
	for _, target := range []string{"a4h", "osg", "native"} {
		if targets[target] {
			failures = append(failures, hir.CheckRuntimeOps(p, target, abaphir.SupportsRuntimeOp)...)
		}
	}
	return errors.Join(failures...)
}
