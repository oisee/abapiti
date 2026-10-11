package golang

// Emitted only when a program uses general repeat, keeping the established
// default runtime byte-identical. Main's repeatIndent remains unchanged.
const repeatRuntimeSource = `
// ToIntegerOrInfinity and UTF-16 code-unit copying, including lone surrogates.
func (s jsString) repeat(n float64) jsString {
 if math.IsNaN(n) { n = 0 }
 n = math.Trunc(n)
 if n < 0 || math.IsInf(n, 0) { panic(newRangeFault()) }
 if n == 0 || len(s) == 0 { return "" }
 if n > float64(2147483647 / s.length()) { panic(newRangeFault()) }
 return jsString(strings.Repeat(string(s), int(n)))
}
`
