package abapfixture

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// KernelViolations catches constructor and CASE forms accepted by the OSG
// parser but rejected by an ABAP kernel. Inspect statements, not lines: the
// emitter wraps calls and quoted text can contain apparent control keywords.
func KernelViolations(files map[string]string) []string {
	var issues []string
	tokens := regexp.MustCompile("(?m)^\\s*\\*[^\\n]*|\"[^\\n]*|'(?:''|[^'])*'|`(?:``|[^`])*`|\\|(?:\\\\.|[^|])*\\||[a-zA-Z_][a-zA-Z_0-9~]*|->|[.]")
	for file, source := range files {
		if !strings.HasSuffix(file, ".abap") {
			continue
		}
		var statement []string
		var blocks []string
		var cases []bool
		constructor, returned := false, false
		calls := 0
		report := func(reason string) { issues = append(issues, file+": "+reason) }
		for _, token := range tokens.FindAllString(source, -1) {
			if strings.HasPrefix(token, "\"") || strings.HasPrefix(strings.TrimSpace(token), "*") {
				continue
			}
			if token != "." {
				statement = append(statement, strings.ToLower(token))
				continue
			}
			if len(statement) == 0 {
				continue
			}
			word := statement[0]
			if word == "method" {
				constructor = len(statement) > 1 && statement[1] == "constructor"
				blocks, cases = nil, nil
				returned, calls = false, 0
			}
			if constructor {
				for i := 0; i+2 < len(statement); i++ {
					if statement[i] == "super" && statement[i+1] == "->" && statement[i+2] == "constructor" {
						calls++
						if len(blocks) != 0 {
							report("super->constructor inside " + strings.Join(blocks, "/"))
						}
						if returned {
							report("RETURN before super->constructor")
						}
					}
				}
				if word == "return" {
					returned = true
				}
			}
			switch word {
			case "if", "case", "try", "loop", "do", "while", "select":
				// Single SELECT does not open a block. Emitted loop SELECTs use ENDSELECT.
				if word != "select" {
					blocks = append(blocks, word)
				}
			case "endif", "endcase", "endtry", "endloop", "enddo", "endwhile":
				if len(blocks) > 0 {
					blocks = blocks[:len(blocks)-1]
				}
			}
			switch word {
			case "case":
				cases = append(cases, false)
			case "when":
				if len(cases) > 0 {
					cases[len(cases)-1] = true
				}
			case "endcase":
				if len(cases) > 0 {
					if !cases[len(cases)-1] {
						report("CASE without WHEN")
					}
					cases = cases[:len(cases)-1]
				}
			case "endmethod":
				if constructor && calls > 1 {
					report(fmt.Sprintf("constructor calls super %d times", calls))
				}
				constructor = false
			}
			statement = nil
		}
	}
	sort.Strings(issues)
	return issues
}
