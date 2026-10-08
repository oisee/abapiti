package abap

import "github.com/oisee/abapiti/hir"

// JavaScript split retains empty final segments and all whitespace; native
// ABAP SPLIT omits the final empty segment and has different blank handling.
// Search explicit string sections and copy their exact lengths instead.
func (b *body) stringSplit(result, source, separator, length string) {
	start := b.temp(hir.T(hir.I32))
	found := b.temp(hir.T(hir.I32))
	count := b.temp(hir.T(hir.I32))
	sepLength := b.temp(hir.T(hir.I32))
	part := b.temp(hir.T(hir.String))
	b.line("CREATE OBJECT " + result + ".")
	b.line(sepLength + " = strlen( " + separator + " ).")
	b.line(start + " = 0.")
	b.line("IF " + sepLength + " = 0.")
	b.line("WHILE " + start + " < " + length + ".")
	b.line(part + " = " + source + "+" + start + "(1).")
	b.line("APPEND " + part + " TO " + result + "->items.")
	b.line(start + " = " + start + " + 1.")
	b.line("ENDWHILE.")
	b.line("ELSE.")
	b.line("WHILE " + start + " <= " + length + ".")
	b.line("FIND FIRST OCCURRENCE OF " + separator + " IN SECTION OFFSET " + start + " OF " + source + " MATCH OFFSET " + found + ".")
	b.line("IF sy-subrc = 0.")
	b.line(count + " = " + found + " - " + start + ".")
	b.line("ELSE.")
	b.line(found + " = -1.")
	b.line(count + " = " + length + " - " + start + ".")
	b.line("ENDIF.")
	b.line("CLEAR " + part + ".")
	b.line("IF " + count + " > 0.")
	b.line(part + " = " + source + "+" + start + "(" + count + ").")
	b.line("ENDIF.")
	b.line("APPEND " + part + " TO " + result + "->items.")
	b.line("IF " + found + " < 0.")
	b.line("EXIT.")
	b.line("ENDIF.")
	b.line(start + " = " + found + " + " + sepLength + ".")
	b.line("ENDWHILE.")
	b.line("ENDIF.")
}
