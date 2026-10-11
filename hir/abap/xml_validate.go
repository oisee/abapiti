package abap

import (
	_ "embed"
	"strings"
)

// Default-options port of fast-xml-parser 5.10.1, matching the Go runtime.
// Copyright (c) 2017 Amit Kumar Gupta; MIT (golang/testdata/xml-validator/LICENSE).
//
//go:embed xml_validate.abap
var xmlValidationSource string

func (e *emitter) xmlValidationRuntime() {
	id := "runtime.xmlValidation"
	if e.types[id] {
		return
	}
	e.types[id] = true
	e.classvalueRuntime()
	e.dynamicRuntime()
	name := e.name(id)
	e.files[name+".clas.abap"] = strings.NewReplacer("$P", name, "$D", e.name("runtime.dynamic")).Replace(xmlValidationSource)
}
