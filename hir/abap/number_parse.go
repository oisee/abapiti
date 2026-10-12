package abap

import (
	"math"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// finiteParseIntLiteral proves the ABAP subset without publishing a value for
// NaN, Infinity, unsafe integers or a dynamic argument. General runtime parsing
// and its nonfinite Number values remain available on Go.
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
