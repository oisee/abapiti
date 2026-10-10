package tsfront

import "github.com/oisee/abapiti/hir/abap"

// The driver APIs return strings, not errors. An unwrappable generated token
// is a generator bug, just as an invalid driver template is.
func formatABAP(source *string) {
	formatted, err := abap.Format(*source)
	if err != nil {
		panic(err)
	}
	*source = formatted
}
