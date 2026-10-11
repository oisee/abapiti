package hir

import (
	"fmt"
	"os"
)

// ReadableNames reports the ABAP naming mode. The empty setting defaults to
// readable; hashed preserves the legacy identities.
func ReadableNames() (bool, error) {
	switch value := os.Getenv("ABAPITI_NAMES"); value {
	case "", "readable":
		return true, nil
	case "hashed":
		return false, nil
	default:
		return false, fmt.Errorf("invalid ABAPITI_NAMES %q: use readable or hashed", value)
	}
}
