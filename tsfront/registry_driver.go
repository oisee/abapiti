package tsfront

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// RegistryFile is one input of the deployment harness.
type RegistryFile struct {
	Name, Raw  string
	Dependency bool
}

// RegistryRunClass emits harness code, not a transformation of upstream
// code: it feeds the files to the translated harness/registry_run.ts, runs
// it with the given config and compares the printed issues with the Node
// oracle's SHA-256. Inputs are base64 literals split into small methods.
func RegistryRunClass(class string, files []RegistryFile, config, wantSHA string, wantIssues int, names *hir.Names) string {
	var b strings.Builder
	line := func(s string, args ...any) { fmt.Fprintf(&b, s+"\n", args...) }
	type part struct{ lines []string }
	var parts []part
	var calls []string
	chunk := func(text string) []int {
		input := abapStringBuild("raw", "ch", base64.StdEncoding.EncodeToString([]byte(text)))
		var ids []int
		for start := 0; start < len(input); {
			end := min(start+400, len(input))
			if end < len(input) && strings.HasPrefix(input[end-1], "ch = ") {
				end--
			}
			ids = append(ids, len(parts))
			parts = append(parts, part{input[start:end]})
			start = end
		}
		return ids
	}
	harness := names.Get("harness/registry_run.ts.RegistryRun")
	load := func(ids []int) {
		calls = append(calls, "CLEAR raw.")
		for _, id := range ids {
			calls = append(calls, fmt.Sprintf("CALL METHOD input_%04d CHANGING raw = raw.", id))
		}
		calls = append(calls, "raw = cl_http_utility=>decode_base64( raw ).")
	}
	for _, f := range files {
		load(chunk(f.Raw))
		member := names.Get("member.addFile")
		if f.Dependency {
			member = names.Get("member.addDependency")
		}
		calls = append(calls, fmt.Sprintf("CALL METHOD h->%s EXPORTING %s = `%s` %s = raw.", member, names.Get("param.filename"), strings.ReplaceAll(f.Name, "`", "``"), names.Get("param.raw")))
	}
	load(chunk(config))
	calls = append(calls, "cfg = raw.")
	line("CLASS %s DEFINITION PUBLIC FINAL CREATE PUBLIC.", class)
	line("PUBLIC SECTION.")
	line("CLASS-METHODS run EXPORTING report TYPE string RETURNING VALUE(ok) TYPE abap_bool.")
	line("PROTECTED SECTION.")
	line("PRIVATE SECTION.")
	for i := range parts {
		line("CLASS-METHODS input_%04d CHANGING raw TYPE string.", i)
	}
	line("ENDCLASS.")
	line("CLASS %s IMPLEMENTATION.", class)
	line("METHOD run.")
	line("DATA h TYPE REF TO %s.", harness)
	for _, s := range []string{"raw", "cfg", "dump", "hash"} {
		line("DATA %s TYPE string.", s)
	}
	for _, s := range []string{"start", "stop", "load_us", "run_us"} {
		line("DATA %s TYPE i.", s)
	}
	line("GET RUN TIME FIELD start.")
	line("CREATE OBJECT h.")
	for _, c := range calls {
		line("%s", c)
	}
	line("GET RUN TIME FIELD stop.")
	line("load_us = stop - start.")
	line("GET RUN TIME FIELD start.")
	line("CALL METHOD h->%s EXPORTING %s = cfg RECEIVING result = dump.", names.Get("member.run"), names.Get("param.config"))
	line("GET RUN TIME FIELD stop.")
	line("run_us = stop - start.")
	line("cl_abap_message_digest=>calculate_hash_for_char( EXPORTING if_algorithm = `SHA256` if_data = dump IMPORTING ef_hashstring = hash ).")
	line("hash = to_lower( hash ).")
	line("ok = xsdbool( hash = `%s` ).", wantSHA)
	line("report = |REGISTRY ok={ ok } want_issues=%d load_us={ load_us } run_us={ run_us } sha256={ hash }|.", wantIssues)
	line("IF ok = abap_false.")
	line("report = report && | head={ substring( val = dump len = nmin( val1 = strlen( dump ) val2 = 200 ) ) }|.")
	line("ENDIF.")
	line("ENDMETHOD.")
	for i, p := range parts {
		line("METHOD input_%04d.", i)
		for _, s := range p.lines {
			line("%s", s)
		}
		line("ENDMETHOD.")
	}
	line("ENDCLASS.")
	return b.String()
}

// RegistryRunTest always fails with the driver's report line (verdict,
// timings, hash), the same convention as the structures benchmark: the
// runner's message carries the measurement; ok=X is the acceptance.
func RegistryRunTest(class string) string {
	return "CLASS ltcl_registry DEFINITION FOR TESTING DURATION LONG RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS run FOR TESTING.\nENDCLASS.\nCLASS ltcl_registry IMPLEMENTATION.\nMETHOD run.\nDATA report TYPE string.\nDATA ok TYPE abap_bool.\nCALL METHOD " + class + "=>run IMPORTING report = report RECEIVING ok = ok.\ncl_abap_unit_assert=>fail( msg = report ).\nENDMETHOD.\nENDCLASS.\n"
}
