package abap

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// finiteParseIntLiteral proves the ABAP subset without publishing a value for
// NaN, Infinity, unsafe integers or a dynamic argument. General runtime parsing
// uses the same safe-integer subset below; general Number values remain on Go.
func finiteParseIntLiteral(x *hir.Expr) (float64, bool) {
	if x == nil || x.Kind != hir.Lit || x.Type.Kind != hir.String {
		return 0, false
	}
	text, ok := x.Value.(string)
	if !ok {
		return 0, false
	}
	text = strings.TrimLeftFunc(text, func(r rune) bool {
		if r >= 0x2000 && r <= 0x200a {
			return true
		}
		for _, c := range jsWhiteSpace {
			if r == rune(c) {
				return true
			}
		}
		return false
	})
	end := 0
	if strings.HasPrefix(text, "+") || strings.HasPrefix(text, "-") {
		end++
	}
	start := end
	for end < len(text) && text[end] >= '0' && text[end] <= '9' {
		end++
	}
	if end == start {
		return 0, false
	}
	value, err := strconv.ParseFloat(text[:end], 64)
	return value, err == nil && !math.IsInf(value, 0) && !math.IsNaN(value) && math.Abs(value) <= 9007199254740991 && !(value == 0 && math.Signbit(value))
}

// parseIntNumber accepts only exact decimal safe integers. Accumulation uses
// int8 and checks before multiplication, so arbitrarily long inputs cannot
// overflow or round back into the accepted domain. NaN and negative zero raise.
func (b *body) parseIntNumber(n, a string) {
	lo := b.temp(hir.T(hir.I32))
	hi := b.temp(hir.T(hir.I32))
	ch := b.temp(hir.T(hir.String))
	negative := b.temp(hir.T(hir.Bool))
	hasDigit := b.temp(hir.T(hir.Bool))
	value := b.temp(hir.T(hir.I64))
	digit := b.temp(hir.T(hir.I32))
	threshold := b.constant("parse_safe_prefix", "int8", "900719925474099")
	b.line(lo + " = 0.")
	b.line(value + " = 0.")
	b.line(negative + " = abap_false.")
	b.line(hasDigit + " = abap_false.")
	b.line(hi + " = strlen( " + a + " ).")
	b.trimLoop(a, lo, hi, true)
	b.line("IF " + lo + " < " + hi + ".")
	b.line(ch + " = " + a + "+" + lo + "(1).")
	b.line("IF " + ch + " = '-' OR " + ch + " = '+'.")
	b.line(negative + " = xsdbool( " + ch + " = '-' ).")
	b.line(lo + " = " + lo + " + 1.")
	b.line("ENDIF.")
	b.line("ENDIF.")
	b.line("WHILE " + lo + " < " + hi + ".")
	b.line(ch + " = " + a + "+" + lo + "(1).")
	b.line("IF " + ch + " CN '0123456789' OR " + ch + " = ` `.")
	b.line("EXIT.")
	b.line("ENDIF.")
	b.line(digit + " = " + ch + ".")
	b.line("IF " + value + " > " + threshold + " OR ( " + value + " = " + threshold + " AND " + digit + " > 1 ).")
	b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
	b.line("ENDIF.")
	b.line(value + " = " + value + " * 10 + " + digit + ".")
	b.line(hasDigit + " = abap_true.")
	b.line(lo + " = " + lo + " + 1.")
	b.line("ENDWHILE.")
	b.line("IF " + hasDigit + " = abap_false OR ( " + negative + " = abap_true AND " + value + " = 0 ).")
	b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
	b.line("ENDIF.")
	b.line("IF " + negative + " = abap_true.")
	b.line(value + " = 0 - " + value + ".")
	b.line("ENDIF.")
	b.line(n + " = " + value + ".")
}

// numberCeil uses truncation only in the range where binary64 has fractional
// values. At magnitude >= 2^52 every finite binary64 is already integral.
// A dynamic zero may be -0; the ABI cannot preserve its sign portably.
func (b *body) numberCeil(x *hir.Expr, n, a string) {
	if x.X.Kind == hir.Lit {
		f, err := strconv.ParseFloat(fmt.Sprint(x.X.Value), 64)
		if err == nil && f == 0 {
			if math.Signbit(f) {
				b.e.refuse(fmt.Errorf("node %d (%s): ABAP ceil cannot preserve negative-zero Number", x.ID, x.Source))
			} else {
				b.line(n + " = 0.")
			}
			return
		}
	}
	finite := b.constant("ceil_finite", "f", "1.7976931348623157E+308")
	b.line("IF " + a + " <> " + a + " OR " + a + " > " + finite + " OR " + a + " < 0 - " + finite + ".")
	b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
	b.line("ENDIF.")
	b.line("IF " + a + " > -1 AND " + a + " <= 0.")
	b.line("RAISE EXCEPTION TYPE cx_sy_range_out_of_bounds.")
	b.line("ENDIF.")
	limit := b.constant("ceil_integral", "f", "4503599627370496")
	b.line("IF " + a + " >= " + limit + " OR " + a + " <= 0 - " + limit + ".")
	b.line(n + " = " + a + ".")
	b.line("ELSE.")
	b.line(n + " = trunc( " + a + " ).")
	b.line("IF " + a + " > " + n + ".")
	b.line(n + " = " + n + " + 1.")
	b.line("ENDIF.")
	b.line("ENDIF.")
}
