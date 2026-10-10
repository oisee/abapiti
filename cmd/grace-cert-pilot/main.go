// grace-cert-pilot is analysis-only: it reports certificate-backed cache facts,
// never lowers ABAP or proves a parallel structures map.
package main

import (
	"fmt"
	"github.com/oisee/abapiti/tsfront"
	"os"
)

func main() {
	report, err := tsfront.PilotCertificateReport()
	if err != nil {
		fmt.Fprintln(os.Stderr, "refused:", err)
		os.Exit(1)
	}
	fmt.Print(report)
}
