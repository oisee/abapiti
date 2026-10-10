package tsfront

import (
	"encoding/base64"
	"fmt"
	"os"
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
			calls = append(calls, fmt.Sprintf("input_%04d( CHANGING raw = raw ).", id))
		}
		calls = append(calls, "raw = decode( raw ).")
	}
	for _, f := range files {
		load(chunk(f.Raw))
		member := names.Get("member.addFile")
		if f.Dependency {
			member = names.Get("member.addDependency")
		}
		calls = append(calls, fmt.Sprintf("h->%s( %s = `%s` %s = raw ).", member, names.Get("param.filename"), strings.ReplaceAll(f.Name, "`", "``"), names.Get("param.raw")))
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
	for _, s := range []string{"raw", "cfg", "dump", "hash", "stages"} {
		line("DATA %s TYPE string.", s)
	}
	for _, s := range []string{"start", "stop", "load_us", "run_us"} {
		line("DATA %s TYPE i.", s)
	}
	line("GET RUN TIME FIELD start.")
	line("h = NEW #( ).")
	for _, c := range calls {
		line("%s", c)
	}
	line("GET RUN TIME FIELD stop.")
	line("load_us = stop - start.")
	line("GET RUN TIME FIELD start.")
	line("dump = h->%s( cfg ).", names.Get("member.run"))
	line("GET RUN TIME FIELD stop.")
	line("run_us = stop - start.")
	line("cl_abap_message_digest=>calculate_hash_for_char( EXPORTING if_algorithm = `SHA256` if_data = dump IMPORTING ef_hashstring = hash ).")
	line("hash = to_lower( hash ).")
	line("ok = xsdbool( hash = `%s` ).", wantSHA)
	line("stages = h->%s( ).", names.Get("member.timings"))
	line("report = |REGISTRY ok={ ok } want_issues=%d load_us={ load_us } run_us={ run_us } sha256={ hash } ms: { stages }|.", wantIssues)
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
	trapCatch := ""
	if trap != "" {
		trapCatch = "CATCH " + trap + " INTO DATA(trapped).\nreport = |TRAP { trapped->source_location }|.\n"
	}
	return "CLASS ltcl_registry DEFINITION FOR TESTING DURATION LONG RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS run FOR TESTING.\nENDCLASS.\nCLASS ltcl_registry IMPLEMENTATION.\nMETHOD run.\nDATA report TYPE string.\nDATA ok TYPE abap_bool.\nDATA error TYPE REF TO cx_root.\nTRY.\nok = " + class + "=>run( IMPORTING report = report ).\n" + trapCatch + "CATCH cx_root INTO error.\nreport = |RAISED { cl_abap_classdescr=>get_class_name( error ) } { error->get_text( ) }|.\nENDTRY.\ncl_abap_unit_assert=>fail( msg = report ).\nENDMETHOD.\nENDCLASS.\n"
}

// RegistryRunCorpusClass is the A4H variant: inputs come from the permanent
// ZABAPITI_CORPUS table (sets ZABAPGIT and DEPS, SHA-256 verified on load),
// progress goes to SLG1 through ZCL_ABAPITI_LOG. Dependency members are
// stored as src/<path>; Node names them relative to deps/src.
func RegistryRunCorpusClass(class, wantSHA string, negative *RegistryNegative, names *hir.Names) string {
	var b strings.Builder
	line := func(s string, args ...any) { fmt.Fprintf(&b, s+"\n", args...) }
	harness := names.Get("harness/registry_run.ts.RegistryRun")
	line("CLASS %s DEFINITION PUBLIC FINAL CREATE PUBLIC.", class)
	line("PUBLIC SECTION.")
	line("CLASS-METHODS run IMPORTING negative TYPE abap_bool DEFAULT abap_false EXPORTING report TYPE string RETURNING VALUE(ok) TYPE abap_bool.")
	line("PROTECTED SECTION.")
	line("PRIVATE SECTION.")
	line("ENDCLASS.")
	line("CLASS %s IMPLEMENTATION.", class)
	line("METHOD run.")
	line("DATA h TYPE REF TO %s.", harness)
	line("DATA set TYPE REF TO zif_abapiti_corpus.")
	line("DATA ch TYPE string.")
	line("DATA want TYPE string.")
	line("DATA log TYPE REF TO zcl_abapiti_log.")
	for _, s := range []string{"name", "raw", "cfg", "dump", "hash", "stages"} {
		line("DATA %s TYPE string.", s)
	}
	line("DATA i TYPE i.")
	line("DATA files TYPE i.")
	for _, s := range []string{"start", "stop", "load_us", "parse_us", "report_us", "run_us"} {
		line("DATA %s TYPE i.", s)
	}
	line("DATA reg TYPE REF TO %s.", names.Get("src/registry.ts.Registry"))
	line("log = zcl_abapiti_log=>start( subobject = 'BENCH' extnumber = 'REGISTRY zabapgit' ).")
	line("want = `%s`.", wantSHA)
	line("GET RUN TIME FIELD start.")
	line("h = NEW #( ).")
	if negative != nil {
		// Node adds the files in name order; the extra files sort first.
		line("IF negative = abap_true.")
		line("want = `%s`.", negative.SHA)
		for _, f := range negative.Extra {
			line("CLEAR raw.")
			for _, s := range abapStringBuild("raw", "ch", f.Raw) {
				line("%s", s)
			}
			line("h->%s( %s = `%s` %s = raw ).", names.Get("member.addFile"), names.Get("param.filename"), strings.ReplaceAll(f.Name, "`", "``"), names.Get("param.raw"))
			line("files = files + 1.")
		}
		line("ENDIF.")
	}
	line("set = NEW zcl_abapiti_corpus( 'ZABAPGIT' ).")
	line("DO set->count( ) TIMES.")
	line("i = sy-index.")
	line("name = set->name( i ).")
	line("IF name = `abaplint.json`.")
	line("cfg = set->get( i ).")
	line("ELSE.")
	line("raw = set->get( i ).")
	if negative != nil {
		for _, f := range negative.Append {
			line("IF negative = abap_true AND name = `%s`.", strings.ReplaceAll(f.Name, "`", "``"))
			for _, s := range abapStringBuild("raw", "ch", f.Raw) {
				line("%s", s)
			}
			line("ENDIF.")
		}
	}
	line("h->%s( %s = name %s = raw ).", names.Get("member.addFile"), names.Get("param.filename"), names.Get("param.raw"))
	line("files = files + 1.")
	line("ENDIF.")
	line("ENDDO.")
	line("set = NEW zcl_abapiti_corpus( 'DEPS' ).")
	line("DO set->count( ) TIMES.")
	line("i = sy-index.")
	line("name = set->name( i ).")
	line("IF strlen( name ) > 4 AND name(4) = `src/`.")
	line("name = substring( val = name off = 4 ).")
	line("h->%s( %s = name %s = set->get( i ) ).", names.Get("member.addDependency"), names.Get("param.filename"), names.Get("param.raw"))
	line("files = files + 1.")
	line("ENDIF.")
	line("ENDDO.")
	line("GET RUN TIME FIELD stop.")
	line("load_us = stop - start.")
	line("log->info( |loaded { files } files, config { strlen( cfg ) } chars, { load_us } us| ).")
	line("GET RUN TIME FIELD start.")
	line("reg = h->%s( cfg ).", names.Get("member.parse"))
	line("GET RUN TIME FIELD stop.")
	line("parse_us = stop - start.")
	line("log->info( |parsed: { parse_us } us| ).")
	line("GET RUN TIME FIELD start.")
	line("dump = h->%s( reg ).", names.Get("member.report"))
	line("GET RUN TIME FIELD stop.")
	line("report_us = stop - start.")
	line("run_us = parse_us + report_us.")
	line("cl_abap_message_digest=>calculate_hash_for_char( EXPORTING if_algorithm = `SHA256` if_data = dump IMPORTING ef_hashstring = hash ).")
	line("hash = to_lower( hash ).")
	line("ok = xsdbool( hash = want ).")
	line("stages = h->%s( ).", names.Get("member.timings"))
	line("report = |REGISTRY negative={ negative } ok={ ok } files={ files } load_us={ load_us } parse_us={ parse_us } report_us={ report_us } run_us={ run_us } sha256={ hash } ms: { stages }|.")
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
func RegistryRunReport(program, class string, negative bool) string {
	flag := "abap_false"
	if negative {
		flag = "abap_true"
	}
	return "REPORT " + program + ".\nDATA report TYPE string.\nDATA ok TYPE abap_bool.\nok = " + class + "=>run( EXPORTING negative = " + flag + " IMPORTING report = report ).\nDATA off TYPE i.\nDATA part TYPE string.\nWHILE off < strlen( report ).\npart = substring( val = report off = off len = nmin( val1 = 200 val2 = strlen( report ) - off ) ).\nWRITE: / part.\noff = off + 200.\nENDWHILE.\n"
}

// RegistryNegative is a seeded variant of the corpus run: Extra inputs are
// added before the corpus set, Append texts are appended to the named set
// members, and SHA is the Node oracle's hash of the printed issues.
type RegistryNegative struct {
	SHA    string         `json:"sha"`
	Extra  []RegistryFile `json:"extra"`
	Append []RegistryFile `json:"append"`
}

// RegistryCLIReport is a report for open-steamgate's native build (osabap):
// its selection screen becomes the command line, so the translated abaplint
// checks a file from disk with a given abaplint.json and optional
// dependencies (a text file listing one path per line).
func RegistryCLIReport(program string, names *hir.Names) string {
	var b strings.Builder
	line := func(s string, args ...any) { fmt.Fprintf(&b, s+"\n", args...) }
	addFile := func(target, path, name string) {
		line("CLEAR bytes.")
		line("OPEN DATASET %s FOR INPUT IN BINARY MODE MESSAGE msg.", path)
		line("IF sy-subrc <> 0.")
		line("WRITE: / |cannot read { %s }: { msg }|.", path)
		line("RETURN.")
		line("ENDIF.")
		line("READ DATASET %s INTO bytes.", path)
		line("CLOSE DATASET %s.", path)
		line("%s = cl_abap_codepage=>convert_from( bytes ).", target)
		if name != "" {
			line("name = %s.", path)
			line("REPLACE ALL OCCURRENCES OF `\\` IN name WITH `/`.")
			line("SPLIT name AT `/` INTO TABLE parts.")
			line("READ TABLE parts INDEX lines( parts ) INTO name.")
		}
	}
	line("REPORT %s LINE-SIZE 1023.", program)
	line("PARAMETERS p_file TYPE string LOWER CASE.")
	line("PARAMETERS p_config TYPE string LOWER CASE.")
	line("PARAMETERS p_deps TYPE string LOWER CASE.")
	line("PARAMETERS p_times AS CHECKBOX.")
	line("DATA h TYPE REF TO %s.", names.Get("harness/registry_run.ts.RegistryRun"))
	line("DATA reg TYPE REF TO %s.", names.Get("src/registry.ts.Registry"))
	line("DATA bytes TYPE xstring.")
	line("DATA msg TYPE string.")
	for _, v := range []string{"name", "raw", "cfg", "list", "path", "dump", "stages"} {
		line("DATA %s TYPE string.", v)
	}
	line("DATA parts TYPE STANDARD TABLE OF string WITH DEFAULT KEY.")
	line("DATA paths TYPE STANDARD TABLE OF string WITH DEFAULT KEY.")
	line("DATA out TYPE STANDARD TABLE OF string WITH DEFAULT KEY.")
	line("START-OF-SELECTION.")
	line("IF p_file IS INITIAL OR p_config IS INITIAL.")
	line("WRITE: / `usage: --file FILE --config ABAPLINT_JSON [--deps LIST_OF_DEPENDENCY_PATHS] [--times] -allow-read DIR`.")
	line("RETURN.")
	line("ENDIF.")
	line("TRY.")
	line("h = NEW #( ).")
	addFile("cfg", "p_config", "")
	addFile("raw", "p_file", "name")
	line("h->%s( %s = name %s = raw ).", names.Get("member.addFile"), names.Get("param.filename"), names.Get("param.raw"))
	line("IF p_deps IS NOT INITIAL.")
	addFile("list", "p_deps", "")
	line("REPLACE ALL OCCURRENCES OF cl_abap_char_utilities=>cr_lf(1) IN list WITH ``.")
	line("SPLIT list AT cl_abap_char_utilities=>newline INTO TABLE paths.")
	line("LOOP AT paths INTO path.")
	line("IF path IS INITIAL.")
	line("CONTINUE.")
	line("ENDIF.")
	addFile("raw", "path", "name")
	line("h->%s( %s = name %s = raw ).", names.Get("member.addDependency"), names.Get("param.filename"), names.Get("param.raw"))
	line("ENDLOOP.")
	line("ENDIF.")
	line("reg = h->%s( cfg ).", names.Get("member.parse"))
	line("dump = h->%s( reg ).", names.Get("member.report"))
	line("SPLIT dump AT cl_abap_char_utilities=>newline INTO TABLE out.")
	line("LOOP AT out INTO raw.")
	line("WRITE: / raw.")
	line("ENDLOOP.")
	line("IF p_times = abap_true.")
	line("stages = h->%s( ).", names.Get("member.timings"))
	line("WRITE: / |ms: { stages }|.")
	line("ENDIF.")
	// Bodies the zabapgit workload never executes are traps in this build:
	// say which TypeScript location the check reached instead of a bare dump.
	line("CATCH %s INTO DATA(trapped).", names.Get("exception.unexecuted"))
	line("WRITE: / |refused: this build has no code for { trapped->source_location }, which checking zabapgit_standalone never runs|.")
	line("CATCH cx_root INTO DATA(error).")
	line("WRITE: / |refused: { cl_abap_classdescr=>get_class_name( error ) } { error->get_text( ) }|.")
	line("ENDTRY.")
	return b.String()
}

// DriverNames are the harness objects around the translated classes. They
// follow the class names: ABAPITI_NAMES=readable puts them under the project
// code (ZCL_LNT_*, ZLNT_*), so both builds can live on one system. The SLG1
// log class and the corpus table stay shared (ZCL_ABAPITI_LOG, ZABAPITI_CORPUS).
type DriverNames struct {
	OSGRun, A4HClass, RunReport, CleanReport, NegReport string
}

func Drivers() DriverNames {
	if os.Getenv("ABAPITI_NAMES") == "readable" {
		return DriverNames{"zcl_lnt_registry_osg", "zcl_lnt_registry_a4h", "zlnt_registry_run", "zlnt_registry_clean", "zlnt_registry_neg"}
	}
	return DriverNames{"zcl_abapiti_registry_run", "zcl_abapiti_registry_a4h", "zabapiti_registry_run", "zabapiti_registry_clean", "zabapiti_registry_neg"}
}
