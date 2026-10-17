package jsregex

import _ "embed"

// RuntimeSource is the same implementation used to certify literal patterns.
// It is usage-gated so the frozen deployment runtime stays byte-identical.
//
//go:embed subset.go
var RuntimeSource string

// IteratorRuntimeSource extends the same checked grammar for unpruned programs.
//
//go:embed iterator_subset.go
var IteratorRuntimeSource string

// ExtendedRuntimeSource adds dotAll, control escapes and alternative-start
// anchors. It is emitted only when a program's literal needs it.
//
//go:embed extended_subset.go
var ExtendedRuntimeSource string
