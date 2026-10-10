package main

import "testing"

func TestABAPStatementMeasurement(t *testing.T) {
	src := "* comment.\nMETHOD test.\nDATA x TYPE string.\nx = 'a.b''c.d'. \" ignored.\nx = `e.f`.\nx = |g.h|.\nENDMETHOD.\n"
	if got := abapStatements(src); got != 6 {
		t.Fatalf("got %d", got)
	}
	if got := methodStatements(src, "test"); got != 6 {
		t.Fatalf("method got %d", got)
	}
}
