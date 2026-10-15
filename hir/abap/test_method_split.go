package abap

import (
	"fmt"
	"strings"
)

// splitTestInitializer is restricted to generated, static registration actions.
// A private frame retains all temporaries across chunks and is cleared on every
// entry. Initializer exceptions still escape through the original host boundary.
func (e *emitter) splitTestInitializer(b *body, method string) string {
	typ := e.name(b.c.Name + "." + b.m.Name + ".state.type")
	defs := "TYPES: BEGIN OF " + typ + ",\n" + strings.Join(b.splitTypes, ",\n") + ",\nEND OF " + typ + ".\nCLASS-DATA " + b.splitState + " TYPE " + typ + ".\n"
	var code strings.Builder
	for _, line := range strings.Split(b.code.String(), "\n") {
		if strings.HasPrefix(line, "DATA("+b.splitState+"-") {
			line = strings.TrimPrefix(line, "DATA(")
			line = strings.Replace(line, ") = ", " = ", 1)
		} else if strings.HasPrefix(line, "DATA "+b.splitState+"-") {
			continue // the frame was initialized before the first executable statement
		}
		if line != "" {
			code.WriteString(line + "\n")
		}
	}
	chunks := generatedStatementChunks(code.String(), 200)
	var methods, calls strings.Builder
	fmt.Fprintf(&calls, "CLEAR %s.\n", b.splitState)
	for i, chunk := range chunks {
		name := e.name(b.c.Name + "." + b.m.Name + fmt.Sprintf(".chunk.%d", i))
		defs += "CLASS-METHODS " + name + ".\n"
		fmt.Fprintf(&calls, "%s( ).\n", name)
		fmt.Fprintf(&methods, "METHOD %s.\n%sENDMETHOD.\n", name, b.constantDeclarations()+chunk)
	}
	if e.splitDeclarations == nil {
		e.splitDeclarations = map[string]string{}
	}
	e.splitDeclarations[b.c.Name] += defs
	return "METHOD " + method + ".\n" + calls.String() + "ENDMETHOD.\n" + methods.String()
}

// Input is emitter-owned, unformatted ABAP: one statement per line, literals
// escaped by the emitter. Never cut through a control block or a wrapped line.
func generatedStatementChunks(code string, budget int) []string {
	var result []string
	var chunk strings.Builder
	depth, count := 0, 0
	for _, line := range strings.SplitAfter(code, "\n") {
		if line == "" {
			continue
		}
		key := strings.Fields(line)
		if len(key) == 0 {
			continue
		}
		switch strings.TrimSuffix(key[0], ".") {
		case "IF", "CASE", "TRY", "LOOP", "WHILE", "DO":
			depth++
		case "ENDIF", "ENDCASE", "ENDTRY", "ENDLOOP", "ENDWHILE", "ENDDO":
			depth--
		}
		chunk.WriteString(line)
		count++
		if depth == 0 && count >= budget && strings.HasSuffix(strings.TrimSpace(line), ".") {
			result = append(result, chunk.String())
			chunk.Reset()
			count = 0
		}
	}
	if chunk.Len() > 0 {
		result = append(result, chunk.String())
	}
	return result
}
