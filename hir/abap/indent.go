package abap

import "strings"

// Format is the final ABAP text pass. Run it only after optimizations that
// depend on column-zero statements. Wrapping includes the indentation budget.
func Format(src string) (string, error) {
	return wrap(indent(src))
}

// indent follows abaplint's pretty_printer/Indent with its default options:
// two spaces per block, four after TRY, and class sections/WHEN at the saved
// class/CASE level. Only statement starts affect the nesting; periods in
// literals, templates, comments and qualified names are not terminators.
func indent(src string) string {
	lines := strings.Split(src, "\n")
	depth := 0
	parentIsEvent := false
	stack := []int{}
	peek := func() int {
		if len(stack) == 0 {
			return 0
		}
		return stack[len(stack)-1]
	}
	pop := func() int {
		n := peek()
		if len(stack) > 0 {
			stack = stack[:len(stack)-1]
		}
		return n
	}
	indented := make(map[int]bool)
	var statement strings.Builder
	start := -1
	quote := byte(0)
	finish := func(end int) {
		text := strings.ToUpper(statement.String())
		words := strings.Fields(text)
		if len(words) == 0 {
			return
		}
		key := strings.TrimRight(words[0], ":.")
		class := key == "CLASS" && !strings.Contains(text, " DEFERRED") && !strings.Contains(text, " LOAD")
		catch := key == "CATCH" && !strings.HasPrefix(text, "CATCH SYSTEM-EXCEPTIONS")
		event := false
		switch key {
		case "START-OF-SELECTION", "END-OF-SELECTION", "INITIALIZATION", "LOAD-OF-PROGRAM", "TOP-OF-PAGE", "END-OF-PAGE", "PROCESS":
			event = true
		case "GET":
			event = len(words) > 1 && !strings.Contains(" RUN TIME PARAMETER REFERENCE CURSOR BIT DATASET LOCALE BADI ", " "+words[1]+" ")
		case "AT":
			event = strings.HasPrefix(text, "AT SELECTION-SCREEN") || strings.HasPrefix(text, "AT LINE-SELECTION") || strings.HasPrefix(text, "AT USER-COMMAND") || strings.HasPrefix(text, "AT PF")
		}
		switch {
		case event:
			depth = 0
			parentIsEvent = true
		case class || key == "FORM" || key == "MODULE" || (key == "INCLUDE" && parentIsEvent && !strings.HasPrefix(text, "INCLUDE TYPE")):
			depth = 0
			parentIsEvent = false
		case catch || key == "CLEANUP":
			depth = peek() - 2
		case key == "PUBLIC" || key == "PROTECTED" || key == "PRIVATE" || key == "WHEN":
			depth = peek()
		case key == "ENDTRY":
			depth = pop() - 4
		case key == "ENDCLASS" || key == "ENDCASE":
			depth = pop() - 2
		case key == "ELSE" || key == "ELSEIF" || strings.Contains(" ENDIF ENDWHILE ENDMODULE ENDSELECT ENDMETHOD ENDCHAIN ENDAT ENDEXEC END-OF-DEFINITION ENDLOOP END-TEST-INJECTION END-TEST-SEAM ENDFORM ENDCATCH ENDFUNCTION ENDINTERFACE ENDDO ", " "+key+" "):
			depth -= 2
		}
		if depth < 0 {
			depth = 0
		}
		startDepth := depth
		for row := start; row <= end; row++ {
			line := strings.TrimLeft(lines[row], " \t")
			if line == "" || strings.HasPrefix(lines[row], "*") || indented[row] {
				continue
			}
			lines[row] = strings.Repeat(" ", startDepth) + line
			indented[row] = true
		}
		switch {
		case key == "TRY":
			depth += 4
			stack = append(stack, depth)
		case class || key == "CASE":
			depth += 2
			stack = append(stack, depth)
		case key == "INTERFACE" && strings.Contains(text, " DEFERRED"):
			// A forward declaration does not open an interface block.
		case key == "AT" && strings.HasPrefix(text, "AT PF"):
			// abaplint resets these events but does not indent their contents.
		case event || strings.Contains(" IF WHILE DO LOOP METHOD INTERFACE FORM MODULE FUNCTION AT EXEC DEFINE WHEN CATCH CLEANUP ELSE ELSEIF CHAIN TEST-INJECTION TEST-SEAM PUBLIC PROTECTED PRIVATE ", " "+key+" "):
			depth += 2
		case key == "SELECT" && !strings.Contains(text, "SINGLE ") && !strings.Contains(text, " TABLE "):
			depth += 2
		}
		statement.Reset()
		start = -1
	}
	for row, original := range lines {
		if strings.HasPrefix(original, "*") || strings.HasPrefix(strings.TrimSpace(original), "\"") {
			continue
		}
		for i := 0; i < len(original); i++ {
			c := original[i]
			if quote == 0 && c == '"' {
				break
			}
			if start < 0 {
				if c == ' ' || c == '\t' {
					continue
				}
				start = row
			}
			statement.WriteByte(c)
			if quote != 0 {
				if quote == '|' && c == '\\' && i+1 < len(original) {
					i++
					statement.WriteByte(original[i])
				} else if c == quote {
					if quote != '|' && i+1 < len(original) && original[i+1] == quote {
						i++
						statement.WriteByte(original[i])
					} else {
						quote = 0
					}
				}
			} else if c == '\'' || c == '`' || c == '|' {
				quote = c
			} else if c == '.' && (i+1 == len(original) || original[i+1] == ' ' || original[i+1] == '\t' || original[i+1] == '"') {
				finish(row)
			}
		}
		if start >= 0 {
			statement.WriteByte(' ')
		}
	}
	return strings.Join(lines, "\n")
}
