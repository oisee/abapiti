package abap

import "strings"

// Cast probes test with IS INSTANCE OF instead of TRY ... CATCH
// cx_sy_move_cast_error: OSG's gogen lowers every TRY to a closure with
// defer/recover, IS INSTANCE OF to a type assertion. Each form keeps the
// outcome of the TRY it replaces, including for an initial source, and the
// ?= after a successful test cannot fail.

// castProbe replaces TRY. dst ?= src. CATCH cx_sy_move_cast_error. ENDTRY.:
// an initial source clears dst, a foreign one leaves dst unchanged.
func castProbe(dst, src, class string) string {
	return "IF " + src + " IS NOT BOUND OR " + src + " IS INSTANCE OF " + class + ".\n" + dst + " ?= " + src + ".\nENDIF."
}

// clearedCastProbe is castProbe for an initial dst, which an initial source
// leaves unchanged.
func clearedCastProbe(dst, src, class string) string {
	return "IF " + src + " IS INSTANCE OF " + class + ".\n" + dst + " ?= " + src + ".\nENDIF."
}

// castOrElse replaces TRY. dst ?= src. CATCH cx_sy_move_cast_error. other
// ENDTRY.; other runs only for a bound source of a foreign class.
func castOrElse(dst, src, class, other string) string {
	return "IF " + src + " IS NOT BOUND OR " + src + " IS INSTANCE OF " + class + ".\n" + dst + " ?= " + src + ".\nELSE.\n" + other + "\nENDIF."
}

// castOrRaise is castOrElse for a handler that leaves the method (RAISE,
// RETURN), so the cast follows unconditionally.
func castOrRaise(dst, src, class, leave string) string {
	return "IF " + src + " IS BOUND AND " + src + " IS NOT INSTANCE OF " + class + ".\n" + leave + "\nENDIF.\n" + dst + " ?= " + src + "."
}

// refClass is the class or interface of a reference type from typ.
func refClass(typ string) string {
	class, ok := strings.CutPrefix(typ, "REF TO ")
	if !ok || strings.ContainsAny(class, " .") {
		panic("cast probe target is not an object reference: " + typ)
	}
	return class
}
