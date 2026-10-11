package jsregex

import _ "embed"

// RuntimeSource is the same implementation used to certify literal patterns.
// It is usage-gated so the frozen deployment runtime stays byte-identical.
//
//go:embed subset.go
var RuntimeSource string
