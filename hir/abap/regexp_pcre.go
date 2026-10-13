package abap

import (
	"strconv"
	"strings"
)

// pcreRegexPattern uses syntax shared by PCRE and the pinned JS ABAP runtime.
// Absolute assertions also prevent that runtime's implicit m flag from
// changing JavaScript non-m anchors. The reviewed m pattern has no anchors.
func pcreRegexPattern(pattern string) string {
	var out strings.Builder
	inClass := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c == '\\' && i+1 < len(pattern) {
			if !inClass && (pattern[i+1] == 'A' || pattern[i+1] == 'z') {
				if pattern[i+1] == 'A' {
					out.WriteString(`(?<![\s\S])`)
				} else {
					out.WriteString(`(?![\s\S])`)
				}
				i++
				continue
			}
			if strings.HasPrefix(pattern[i:], `\x{`) {
				if end := strings.IndexByte(pattern[i+3:], '}'); end >= 0 {
					end += i + 3
					if cp, err := strconv.ParseUint(pattern[i+3:end], 16, 32); err == nil {
						writePCRECharacter(&out, rune(cp), inClass)
						i = end
						continue
					}
				}
			}
			if i == 0 && strings.HasPrefix(pattern, `\b[dD][eE][fF][iI][nN][eE]`) {
				out.WriteString(`(?<![A-Za-z0-9_])`)
				i++
				continue
			}
			out.WriteByte(c)
			i++
			out.WriteByte(pattern[i])
			continue
		}
		if c == '[' {
			inClass = true
		} else if c == ']' {
			inClass = false
		}
		switch {
		case !inClass && c == '^':
			out.WriteString(`(?<![\s\S])`)
		case !inClass && c == '$':
			out.WriteString(`(?![\s\S])`)
		case !inClass && c == ' ':
			// Character classes protect literals from PCRE's extended mode and
			// mean the same in JS/RE2, without the host's refused hex escapes.
			out.WriteString(`[ ]`)
		case !inClass && c == '#':
			out.WriteString(`[#]`)
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

// A reviewed hex escape denotes one character, not regex syntax. Emit that
// character literally, escaping metacharacters and protecting ASCII whitespace
// outside classes from PCRE extended mode. The pinned OSGO refuses all \\x
// escapes; unlike changing anchors, this spelling is also valid JavaScript.
func writePCRECharacter(out *strings.Builder, r rune, inClass bool) {
	metacharacters := `\[]^-`
	if !inClass {
		metacharacters = `\.*+?()|[]{}^$`
		if r == ' ' || r == '#' || (r >= '\t' && r <= '\r') {
			out.WriteByte('[')
			out.WriteRune(r)
			out.WriteByte(']')
			return
		}
	}
	if strings.ContainsRune(metacharacters, r) {
		out.WriteByte('\\')
	}
	out.WriteRune(r)
}

// FIND REGEX is POSIX longest-match, multiline/dotall, and rejects lazy
// quantifiers. The JS match/exec path needs PCRE (ABAP 7.55), not a guess at
// equivalent POSIX semantics. UTF-16 unit text makes regex offsets/captures
// independent of the regex engine's supplementary-character interpretation.
func (e *emitter) regexpPCRERuntime(name string) {
	file := name + ".clas.abap"
	text := e.files[file]
	text = strings.Replace(text, "METHODS test IMPORTING", "METHODS unit_text IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE string.\nMETHODS pcre_pattern IMPORTING p0 TYPE string RETURNING VALUE(result) TYPE string.\nMETHODS test IMPORTING", 1)
	text = strings.TrimSuffix(text, "ENDCLASS.\n") + `METHOD unit_text.
DATA i TYPE i.
DATA c TYPE string.
DATA cp TYPE i.
WHILE i < strlen( p0 ).
  c = p0+i(1).
  cp = cl_abap_conv_out_ce=>uccpi( c ).
  IF ( cp >= 55296 AND cp <= 57343 ) OR cp = 65533.
    c = cl_abap_conv_in_ce=>uccp( 'FFFD' ).
  ENDIF.
  result = result && c.
  i = i + 1.
ENDWHILE.
ENDMETHOD.
METHOD pcre_pattern.
DATA i TYPE i.
DATA c TYPE string.
DATA escaped TYPE abap_bool.
DATA in_class TYPE abap_bool.
WHILE i < strlen( p0 ).
  c = p0+i(1).
  IF escaped = abap_true.
    result = result && c.
    escaped = abap_false.
  ELSEIF c = '\'.
    result = result && c.
    escaped = abap_true.
  ELSEIF c = '['.
    in_class = abap_true.
    result = result && c.
  ELSEIF c = ']'.
    in_class = abap_false.
    result = result && c.
  ELSEIF in_class = abap_false AND c = '^'.
    result = result && '(?<![\s\S])'.
  ELSEIF in_class = abap_false AND c = '$'.
    result = result && '(?![\s\S])'.
  ELSE.
    result = result && c.
  ENDIF.
  i = i + 1.
ENDWHILE.
ENDMETHOD.
ENDCLASS.
`
	// Match result offsets refer to unit text, while returned captures are
	// sliced from the original input (including lone surrogate captures).
	text = strings.Replace(text, "METHOD match.\n", "METHOD match.\nDATA text TYPE string.\ntext = me->unit_text( p0 ).\n", 1)
	text = strings.ReplaceAll(text, "REGEX posix IN p0", "PCRE posix IN text")
	text = strings.ReplaceAll(text, "REGEX excluded IN p0", "PCRE excluded IN text")
	text = strings.ReplaceAll(text, "REGEX excluded IN text", "PCRE excluded IN text")
	text = strings.ReplaceAll(text, "REGEX search IN text", "PCRE search IN text")
	text = strings.ReplaceAll(text, "REGEX posix IN ' '", "PCRE posix IN ' '")
	if !e.execUsed {
		text = strings.Replace(text, "METHOD test.\n", "METHOD test.\nDATA text TYPE string.\ntext = me->unit_text( p0 ).\n", 1)
		text = strings.Replace(text, "METHOD match_test.\n", "METHOD match_test.\nDATA text TYPE string.\ntext = me->unit_text( p0 ).\n", 1)
	}
	if e.execUsed {
		text = strings.Replace(text, "METHOD match.\n", "METHOD match.\nIF global = abap_false.\nresult = me->exec( p0 ).\nRETURN.\nENDIF.\n", 1)
	}
	// Replacement strings are literal (dollar substitutions are refused). Use
	// match offsets and the original text rather than PCRE replacement syntax.
	start := strings.Index(text, "METHOD replace.\n")
	end := strings.Index(text, "METHOD match.\n")
	replacement := `METHOD replace.
DATA text TYPE string.
DATA last TYPE i.
DATA n TYPE i.
DATA matches TYPE match_result_tab.
DATA hit TYPE match_result.
IF p1 CS '$'.
  RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.
ENDIF.
text = me->unit_text( p0 ).
IF excluded IS NOT INITIAL.
  FIND PCRE excluded IN text.
  IF sy-subrc = 0.
    result = p0.
    RETURN.
  ENDIF.
ENDIF.
IF global = abap_true.
`
	if e.execUsed {
		replacement += "last_index = 0.\n"
	}
	replacement += `  FIND ALL OCCURRENCES OF PCRE posix IN text RESULTS matches.
ELSE.
  FIND FIRST OCCURRENCE OF PCRE posix IN text RESULTS hit.
  IF sy-subrc = 0.
    APPEND hit TO matches.
  ENDIF.
ENDIF.
LOOP AT matches INTO hit.
  n = hit-offset - last.
  result = result && p0+last(n) && p1.
  last = hit-offset + hit-length.
ENDLOOP.
n = strlen( p0 ) - last.
result = result && p0+last(n).
ENDMETHOD.
`
	if end < 0 {
		end = strings.Index(text, "METHOD unit_text.\n")
	}
	text = text[:start] + replacement + text[end:]
	e.files[file] = text
}
