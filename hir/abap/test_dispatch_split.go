package abap

import (
	"fmt"
	"strings"
)

// splitTestDispatch bounds generated first-match dispatch without regrouping
// branches. A handled flag carries each original early RETURN to the caller.
func splitTestDispatch(source, method, prefix, importing, outputs string) string {
	marker := "METHOD " + method + ".\n"
	start := strings.Index(source, marker)
	if start < 0 {
		return source
	}
	start += len(marker)
	end := start + strings.Index(source[start:], "ENDMETHOD.")
	body := source[start:end]
	lines := strings.SplitAfter(body, "\n")
	var before, branch, after strings.Builder
	var blocks []string
	depth := 0
	for _, line := range lines {
		if depth == 0 && strings.HasPrefix(line, prefix) {
			depth = 1
			branch.WriteString(line)
			continue
		}
		if depth > 0 {
			if strings.HasPrefix(line, "IF ") {
				depth++
			}
			if line == "ENDIF.\n" {
				depth--
			}
			branch.WriteString(line)
			if depth == 0 {
				blocks = append(blocks, branch.String())
				branch.Reset()
			}
		} else if len(blocks) == 0 {
			before.WriteString(line)
		} else {
			after.WriteString(line)
		}
	}
	if strings.Count(strings.Join(blocks, ""), "\n") < 2000 {
		return source
	}
	var groups []string
	var group strings.Builder
	for _, block := range blocks {
		if group.Len() > 0 && strings.Count(group.String(), "\n")+strings.Count(block, "\n")+strings.Count(block, "RETURN.\n") > 200 {
			groups = append(groups, group.String())
			group.Reset()
		}
		group.WriteString(block)
	}
	if group.Len() > 0 {
		groups = append(groups, group.String())
	}
	var declarations, methods, calls strings.Builder
	for i, group := range groups {
		name := fmt.Sprintf("%s_part_%d", method, i)
		fmt.Fprintf(&declarations, "CLASS-METHODS %s IMPORTING %s EXPORTING handled TYPE abap_bool%s.\n", name, importing, outputs)
		actuals := "p0 = p0"
		if strings.Contains(importing, "key TYPE") {
			actuals += " key = key"
		}
		receive := "handled = matched"
		if outputs != "" {
			receive += " result = result"
		}
		fmt.Fprintf(&calls, "%s( EXPORTING %s IMPORTING %s ).\nIF matched = abap_true.\n%sRETURN.\nENDIF.\n", name, actuals, receive, func() string {
			if method == "run" {
				return "handled = abap_true.\n"
			}
			return ""
		}())
		if !strings.Contains(group, "handled = abap_true.") {
			group = strings.ReplaceAll(group, "RETURN.\n", "handled = abap_true.\nRETURN.\n")
		}
		fmt.Fprintf(&methods, "METHOD %s.\nCLEAR handled.\n%sENDMETHOD.\n", name, group)
	}
	// Helpers clear their match output; the caller propagates an early return.
	local := "DATA matched TYPE abap_bool.\n"
	source = source[:start] + local + before.String() + calls.String() + after.String() + source[end:]
	source = strings.Replace(source, "ENDCLASS.\n", declarations.String()+"ENDCLASS.\n", 1)
	at := strings.LastIndex(source, "ENDCLASS.")
	return source[:at] + methods.String() + source[at:]
}
