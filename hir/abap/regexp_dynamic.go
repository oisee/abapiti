package abap

import (
	"fmt"
	"sort"
	"strings"
)

// Runtime strings must not bypass the reviewed JS regex language. The table
// is shared with Go; its regexes are valid PCRE as well as RE2.
func (e *emitter) regexpDynamicRuntime(name string) {
	file := name + ".clas.abap"
	text := e.files[file]
	text = strings.Replace(text, "excluded_pattern TYPE string OPTIONAL.", "excluded_pattern TYPE string OPTIONAL checked TYPE abap_bool OPTIONAL.", 1)
	quote := regexABAPString
	var code strings.Builder
	reviewed := e.regexpLiteralTable()
	keys := make([]string, 0, len(reviewed))
	for k := range reviewed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// WHEN takes only a literal or data object on a real kernel, so a key
	// that needs several literal chunks is compared in an IF chain first
	// (an expression is fine there); short keys stay in the CASE.
	var long, short strings.Builder
	for _, k := range keys {
		spec := reviewed[k]
		key := quote(k)
		body := "posix = " + quote(pcreRegexPattern(spec[0])) + ".\nexcluded = " + quote(pcreRegexPattern(spec[1])) + ".\nignore_case = abap_false.\n"
		if strings.Contains(key, "\n") {
			if long.Len() == 0 {
				long.WriteString("IF dispatch_key = " + key + ".\n" + body)
			} else {
				long.WriteString("ELSEIF dispatch_key = " + key + ".\n" + body)
			}
			continue
		}
		short.WriteString("WHEN " + key + ".\n" + body)
	}
	code.WriteString("IF checked = abap_true.\nDATA dispatch_key TYPE string.\ndispatch_key = pattern && `/` && flags.\n")
	if long.Len() > 0 {
		code.WriteString(long.String() + "ELSE.\n")
	}
	code.WriteString("CASE dispatch_key.\n" + short.String())
	code.WriteString(`WHEN OTHERS.
IF flags = 'g'.
  FIND PCRE '\A&[1-9][0-9]*\z' IN pattern.
ELSEIF flags = '' OR flags = 'i'.
  FIND PCRE '\A\^[A-Za-z0-9_|()?]+\$?\z' IN pattern.
ELSE.
  sy-subrc = 4.
ENDIF.
IF sy-subrc <> 0.
  RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.
ENDIF.
posix = pattern.
IF flags = 'i'.
  CLEAR posix.
  i = 0.
  WHILE i < strlen( pattern ).
    c = pattern+i(1).
    IF ( c >= 'A' AND c <= 'Z' ) OR ( c >= 'a' AND c <= 'z' ).
      posix = posix && '[' && to_upper( c ) && to_lower( c ) && ']'.
    ELSE.
      posix = posix && c.
    ENDIF.
    i = i + 1.
  ENDWHILE.
  ignore_case = abap_false.
ENDIF.
ENDCASE.
@LONG_END@posix = me->pcre_pattern( posix ).
TRY.
  FIND REGEX posix IN ' '.
CATCH cx_root.
  RAISE EXCEPTION TYPE @REGEX_ERROR@ EXPORTING pattern = pattern flags = flags.
ENDTRY.
ENDIF.
`)
	if e.execUsed {
		code.WriteString("exec_pattern = posix.\n")
	}
	longEnd := ""
	if long.Len() > 0 {
		longEnd = "ENDIF.\n"
	}
	text = strings.Replace(text, "ENDMETHOD.\nMETHOD test.", strings.Replace(code.String(), "@LONG_END@", longEnd, 1)+"ENDMETHOD.\nMETHOD test.", 1)
	trap := e.name("runtime.regex_error")
	text = strings.ReplaceAll(text, "@REGEX_ERROR@", trap)
	text = strings.Replace(text, "RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.", "RAISE EXCEPTION TYPE "+trap+" EXPORTING pattern = pattern flags = flags.", 1)
	e.files[trap+".clas.abap"] = "CLASS " + trap + ` DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.
PUBLIC SECTION.
DATA pattern TYPE string.
DATA flags TYPE string.
METHODS constructor IMPORTING pattern TYPE string flags TYPE string.
METHODS get_text REDEFINITION.
PROTECTED SECTION.
PRIVATE SECTION.
ENDCLASS.
CLASS ` + trap + ` IMPLEMENTATION.
METHOD constructor.
super->constructor( ).
me->pattern = pattern.
me->flags = flags.
ENDMETHOD.
METHOD get_text.
result = |unreviewed JavaScript regexp /{ pattern }/{ flags }|.
ENDMETHOD.
ENDCLASS.
`
	e.files[file] = text
}

// Preserve control characters and Unicode separators in generated ABAP source.
func regexABAPString(s string) string {
	if s == "" {
		return "``"
	}
	var parts []string
	var chunk []rune
	flush := func() {
		if len(chunk) > 0 {
			parts = append(parts, "`"+strings.ReplaceAll(string(chunk), "`", "``")+"`")
			chunk = nil
		}
	}
	for _, r := range s {
		if r < 32 || r == 127 || r == 0xfeff || r == 0x2028 || r == 0x2029 {
			flush()
			parts = append(parts, fmt.Sprintf("cl_abap_conv_in_ce=>uccpi( %d )", r))
		} else {
			chunk = append(chunk, r)
			if len(chunk) == 80 {
				flush()
			}
		}
	}
	flush()
	return strings.Join(parts, " &&\n")
}
