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
// oracle's SHA-256. Inputs are base64 literals split into small methods,
// decoded by the driver itself (pinned osgo has no decode_base64).
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
		calls = append(calls, "raw = decode( raw ).")
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
	line("CLASS-METHODS decode IMPORTING b64 TYPE string RETURNING VALUE(text) TYPE string.")
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
	// A plain base64 decoder (pinned osgo has no cl_http_utility=>decode_base64).
	line("METHOD decode.")
	line("CONSTANTS alphabet TYPE string VALUE `ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/`.")
	line("CONSTANTS digits TYPE string VALUE `0123456789ABCDEF`.")
	line("DATA parts TYPE string_table.")
	line("DATA chunk TYPE string.")
	line("DATA hex TYPE string.")
	line("DATA bytes TYPE xstring.")
	line("DATA acc TYPE i.")
	line("DATA bits TYPE i.")
	line("DATA pos TYPE i.")
	line("DATA idx TYPE i.")
	line("DATA v TYPE i.")
	line("DATA hi TYPE i.")
	line("DATA lo TYPE i.")
	line("DATA len TYPE i.")
	line("len = strlen( b64 ).")
	line("WHILE pos < len.")
	line("IF b64+pos(1) = `=`.")
	line("EXIT.")
	line("ENDIF.")
	line("idx = find( val = alphabet sub = b64+pos(1) ).")
	line("acc = acc * 64 + idx.")
	line("bits = bits + 6.")
	line("IF bits >= 8.")
	line("bits = bits - 8.")
	line("v = acc DIV ipow( base = 2 exp = bits ).")
	line("acc = acc MOD ipow( base = 2 exp = bits ).")
	line("hi = v DIV 16.")
	line("lo = v MOD 16.")
	line("chunk = chunk && digits+hi(1) && digits+lo(1).")
	line("IF strlen( chunk ) >= 8192.")
	line("APPEND chunk TO parts.")
	line("CLEAR chunk.")
	line("ENDIF.")
	line("ENDIF.")
	line("pos = pos + 1.")
	line("ENDWHILE.")
	line("APPEND chunk TO parts.")
	line("CONCATENATE LINES OF parts INTO hex.")
	line("bytes = hex.")
	line("text = cl_abap_codepage=>convert_from( bytes ).")
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
func RegistryRunTest(class, trap string) string {
	return "CLASS ltcl_registry DEFINITION FOR TESTING DURATION LONG RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS run FOR TESTING.\nENDCLASS.\nCLASS ltcl_registry IMPLEMENTATION.\nMETHOD run.\nDATA report TYPE string.\nDATA ok TYPE abap_bool.\nDATA error TYPE REF TO cx_root.\nTRY.\nCALL METHOD " + class + "=>run IMPORTING report = report RECEIVING ok = ok.\nCATCH cx_root INTO error.\nreport = |RAISED { cl_abap_classdescr=>get_class_name( error ) } { error->get_text( ) }|.\nDATA trapped TYPE REF TO " + trap + ".\nTRY.\ntrapped ?= error.\nreport = report && | at { trapped->source_location }|.\nCATCH cx_sy_move_cast_error.\nENDTRY.\nENDTRY.\ncl_abap_unit_assert=>fail( msg = report ).\nENDMETHOD.\nENDCLASS.\n"
}

// RegistryRunCorpusClass is the A4H variant: inputs come from the permanent
// ZABAPITI_CORPUS table (sets ZABAPGIT and DEPS, SHA-256 verified on load),
// progress goes to SLG1 through ZCL_ABAPITI_LOG. Dependency members are
// stored as src/<path>; Node names them relative to deps/src.
func RegistryRunCorpusClass(class, wantSHA string, names *hir.Names) string {
	var b strings.Builder
	line := func(s string, args ...any) { fmt.Fprintf(&b, s+"\n", args...) }
	harness := names.Get("harness/registry_run.ts.RegistryRun")
	line("CLASS %s DEFINITION PUBLIC FINAL CREATE PUBLIC.", class)
	line("PUBLIC SECTION.")
	line("CLASS-METHODS run EXPORTING report TYPE string RETURNING VALUE(ok) TYPE abap_bool.")
	line("PROTECTED SECTION.")
	line("PRIVATE SECTION.")
	line("ENDCLASS.")
	line("CLASS %s IMPLEMENTATION.", class)
	line("METHOD run.")
	line("DATA h TYPE REF TO %s.", harness)
	line("DATA set TYPE REF TO zif_abapiti_corpus.")
	line("DATA log TYPE REF TO zcl_abapiti_log.")
	for _, s := range []string{"name", "raw", "cfg", "dump", "hash"} {
		line("DATA %s TYPE string.", s)
	}
	line("DATA i TYPE i.")
	line("DATA files TYPE i.")
	for _, s := range []string{"start", "stop", "load_us", "run_us"} {
		line("DATA %s TYPE i.", s)
	}
	line("log = zcl_abapiti_log=>start( subobject = 'BENCH' extnumber = 'REGISTRY zabapgit' ).")
	line("GET RUN TIME FIELD start.")
	line("CREATE OBJECT h.")
	line("set = NEW zcl_abapiti_corpus( 'ZABAPGIT' ).")
	line("DO set->count( ) TIMES.")
	line("i = sy-index.")
	line("name = set->name( i ).")
	line("IF name = `abaplint.json`.")
	line("cfg = set->get( i ).")
	line("ELSE.")
	line("CALL METHOD h->%s EXPORTING %s = name %s = set->get( i ).", names.Get("member.addFile"), names.Get("param.filename"), names.Get("param.raw"))
	line("files = files + 1.")
	line("ENDIF.")
	line("ENDDO.")
	line("set = NEW zcl_abapiti_corpus( 'DEPS' ).")
	line("DO set->count( ) TIMES.")
	line("i = sy-index.")
	line("name = set->name( i ).")
	line("IF strlen( name ) > 4 AND name(4) = `src/`.")
	line("name = substring( val = name off = 4 ).")
	line("CALL METHOD h->%s EXPORTING %s = name %s = set->get( i ).", names.Get("member.addDependency"), names.Get("param.filename"), names.Get("param.raw"))
	line("files = files + 1.")
	line("ENDIF.")
	line("ENDDO.")
	line("GET RUN TIME FIELD stop.")
	line("load_us = stop - start.")
	line("log->info( |loaded { files } files, config { strlen( cfg ) } chars, { load_us } us| ).")
	line("GET RUN TIME FIELD start.")
	line("CALL METHOD h->%s EXPORTING %s = cfg RECEIVING result = dump.", names.Get("member.run"), names.Get("param.config"))
	line("GET RUN TIME FIELD stop.")
	line("run_us = stop - start.")
	line("cl_abap_message_digest=>calculate_hash_for_char( EXPORTING if_algorithm = `SHA256` if_data = dump IMPORTING ef_hashstring = hash ).")
	line("hash = to_lower( hash ).")
	line("ok = xsdbool( hash = `%s` ).", wantSHA)
	line("report = |REGISTRY ok={ ok } files={ files } load_us={ load_us } run_us={ run_us } sha256={ hash }|.")
	line("IF ok = abap_false.")
	line("report = report && | head={ substring( val = dump len = nmin( val1 = strlen( dump ) val2 = 200 ) ) }|.")
	line("ENDIF.")
	line("log->info( report ).")
	line("ENDMETHOD.")
	line("ENDCLASS.")
	return b.String()
}

// RegistryRunReport runs the corpus driver as a background job and prints
// its report line to the spool.
func RegistryRunReport(program, class string) string {
	return "REPORT " + program + ".\nDATA report TYPE string.\nDATA ok TYPE abap_bool.\nCALL METHOD " + class + "=>run IMPORTING report = report RECEIVING ok = ok.\nWRITE: / report.\n"
}
