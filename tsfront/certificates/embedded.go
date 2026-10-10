package certificates

import "embed"

//go:embed *.json attestations/*.json
var embedded embed.FS

// Embedded returns the versioned records and immutable attestation evidence.
func Embedded() embed.FS { return embedded }
