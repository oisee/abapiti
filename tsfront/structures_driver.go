package tsfront

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/oisee/abapiti/hir"
)

type StructureCase struct {
	LexerSHA256      string `json:"lexerSHA256"`
	StatementsSHA256 string `json:"statementsSHA256"`
	Name             string `json:"name"`
	Abap             string `json:"abap"`
	Filename         string `json:"filename"`
	Dump             string `json:"dump"`
	Tokens           int    `json:"tokens"`
	Statements       int    `json:"statements"`
	Structures       int    `json:"structures"`
	Issues           int    `json:"issues"`
	SHA256           string `json:"sha256"`
}

// StructuresBenchmarkClass emits harness code, not a transformation of the
// upstream parser. Source data is base64-encoded and quoted by the same literal builder as the
// differential corpus and never interpreted as executable ABAP.
func StructuresBenchmarkClass(c StructureCase, lexerSHA, statementsSHA string, names *hir.Names) string {
	var b strings.Builder
	line := func(s string, args ...any) { fmt.Fprintf(&b, s+"\n", args...) }
	input := abapStringBuild("raw", "ch", base64.StdEncoding.EncodeToString([]byte(c.Abap)))
	var chunks [][]string
	for start := 0; start < len(input); {
		end := min(start+400, len(input))
		if end < len(input) && strings.HasPrefix(input[end-1], "ch = ") {
			end--
		}
		chunks = append(chunks, input[start:end])
		start = end
	}
	corpus := os.Getenv("STRUCTURES_BENCH_CORPUS") != ""
	if corpus {
		chunks = nil
	}
	parts := len(chunks)
	driver := names.Get("harness/structures_dump.ts.StructuresDump")
	line("CLASS zcl_phase3_benchmark DEFINITION PUBLIC FINAL CREATE PUBLIC.")
	line("PUBLIC SECTION.")
	line("CLASS-METHODS run IMPORTING print TYPE abap_bool DEFAULT abap_false EXPORTING report TYPE string RETURNING VALUE(ok) TYPE abap_bool.")
	line("PROTECTED SECTION.")
	line("PRIVATE SECTION.")
	for i := 0; i < parts; i++ {
		line("CLASS-METHODS input_%03d CHANGING raw TYPE string.", i)
	}
	line("ENDCLASS.")
	line("CLASS zcl_phase3_benchmark IMPLEMENTATION.")
	line("METHOD run.")
	for _, s := range []string{"raw", "dump", "lexer_hash", "statements_hash", "structures_hash", "verdict"} {
		line("DATA %s TYPE string.", s)
	}
	for _, s := range []string{"start", "stop", "lex_us", "statements_us", "structures_us", "dump_hash_us", "tokens", "statements", "structures", "issues"} {
		line("DATA %s TYPE i.", s)
	}
	line("DATA lexed TYPE REF TO %s.", names.Get("src/abap/1_lexer/lexer_result.ts.IABAPLexerResult"))
	line("DATA parsed TYPE REF TO %s.", names.Get("src/abap/2_statements/statement_result.ts.IStatementResult"))
	line("DATA result TYPE REF TO %s.", names.Get("src/abap/3_structures/structure_result.ts.IStructureResult"))
	if corpus {
		line("DATA(log) = zcl_abapiti_log=>start( subobject = 'BENCH' extnumber = `structures %s` ).", c.Filename)
		line("raw = zcl_abapiti_corpus=>get_text( setname = 'ZABAPGIT' name = `%s` ).", c.Filename)
		line("log->info( |input loaded: { strlen( raw ) } characters| ).")
	} else {
		for i := 0; i < parts; i++ {
			line("CALL METHOD input_%03d CHANGING raw = raw.", i)
		}
		line("raw = cl_http_utility=>decode_base64( raw ).")
	}
	stage := func(name, field string) {
		if corpus {
			line("log->stage( name = `%s` microseconds = CONV int8( %s ) ).", name, field)
		}
	}
	line("GET RUN TIME FIELD start.")
	line("CALL METHOD %s=>%s EXPORTING %s = raw %s = `%s` RECEIVING result = lexed.", driver, names.Get("member.lex"), names.Get("param.raw"), names.Get("param.filename"), c.Filename)
	line("GET RUN TIME FIELD stop.")
	line("lex_us = stop - start.")
	stage("lexer", "lex_us")
	line("tokens = lexed->%s->length( ).", names.Get("member.tokens"))
	line("GET RUN TIME FIELD start.")
	line("CALL METHOD %s=>%s EXPORTING %s = lexed RECEIVING result = parsed.", driver, names.Get("member.parseStatements"), names.Get("param.lexed"))
	line("GET RUN TIME FIELD stop.")
	line("statements_us = stop - start.")
	stage("statements", "statements_us")
	line("statements = parsed->%s->length( ).", names.Get("member.statements"))
	line("GET RUN TIME FIELD start.")
	line("CALL METHOD %s=>%s EXPORTING %s = parsed RECEIVING result = result.", driver, names.Get("member.parseStructures"), names.Get("param.input"))
	line("GET RUN TIME FIELD stop.")
	line("structures_us = stop - start.")
	stage("structures", "structures_us")
	line("issues = result->%s->length( ).", names.Get("member.issues"))
	line("GET RUN TIME FIELD start.")
	line("CALL METHOD %s=>%s EXPORTING %s = lexed RECEIVING result = dump.", driver, names.Get("member.dumpLexed"), names.Get("param.input"))
	benchmarkHash(&b, "lexer_hash")
	line("CALL METHOD %s=>%s EXPORTING %s = parsed RECEIVING result = dump.", driver, names.Get("member.dumpStatements"), names.Get("param.input"))
	benchmarkHash(&b, "statements_hash")
	line("CALL METHOD %s=>%s EXPORTING %s = parsed %s = result RECEIVING result = dump.", driver, names.Get("member.fromStructure"), names.Get("param.input"), names.Get("param.result"))
	benchmarkHash(&b, "structures_hash")
	line("GET RUN TIME FIELD stop.")
	line("dump_hash_us = stop - start.")
	line("structures = %s=>%s.", driver, names.Get("member.lastStructures"))
	line("IF tokens = %d AND statements = %d AND structures = %d AND issues = %d", c.Tokens, c.Statements, c.Structures, c.Issues)
	line("AND lexer_hash = `%s`", lexerSHA)
	line("AND statements_hash = `%s`", statementsSHA)
	line("AND structures_hash = `%s`.", c.SHA256)
	line("ok = abap_true.")
	line("verdict = `OK`.")
	line("ELSE.")
	line("verdict = `MISMATCH`.")
	line("ENDIF.")
	line("report = |STAGETIME { verdict } tokens { tokens } statements { statements } structures { structures } issues { issues } lex_us { lex_us } statements_us { statements_us } structures_us { structures_us } dump_hash_us { dump_hash_us }|.")
	if corpus {
		line("log->info( report ).")
	}
	line("IF print = abap_true.")
	line("WRITE: / report.")
	line("WRITE: / `lexer_sha256`, lexer_hash.")
	line("WRITE: / `statements_sha256`, statements_hash.")
	line("WRITE: / `structures_sha256`, structures_hash.")
	line("ENDIF.")
	line("ENDMETHOD.")
	for i := 0; i < parts; i++ {
		line("METHOD input_%03d.", i)

		for _, s := range chunks[i] {
			line("%s", s)
		}
		line("ENDMETHOD.")
	}
	line("ENDCLASS.")
	return b.String()
}

func benchmarkHash(b *strings.Builder, target string) {
	fmt.Fprintf(b, "cl_abap_message_digest=>calculate_hash_for_char( EXPORTING if_algorithm = `SHA256` if_data = dump IMPORTING ef_hashstring = %s ).\n", target)
	fmt.Fprintf(b, "%s = to_lower( %s ).\n", target, target)
}

func StructuresBenchmarkReport() string {
	return "REPORT zphase3_structures_bench.\nDATA ok TYPE abap_bool.\nDATA report TYPE string.\nok = zcl_phase3_benchmark=>run( EXPORTING print = abap_true IMPORTING report = report ).\n"
}

func StructuresBenchmarkTest() string {
	return `CLASS ltcl_benchmark DEFINITION FOR TESTING DURATION LONG RISK LEVEL HARMLESS.
PRIVATE SECTION.
METHODS benchmark FOR TESTING.
ENDCLASS.
CLASS ltcl_benchmark IMPLEMENTATION.
METHOD benchmark.
DATA ok TYPE abap_bool.
DATA report TYPE string.
ok = zcl_phase3_benchmark=>run( IMPORTING report = report ).
cl_abap_unit_assert=>fail( msg = report ).
ENDMETHOD.
ENDCLASS.
`
}
