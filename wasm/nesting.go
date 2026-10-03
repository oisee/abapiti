package wasm

import (
	"fmt"
	"strings"
)

const maxABAPNesting = 100
const flattenBlockThreshold = 64

func (c *compiler) emitFlatBlocks(f *Function, code []Instruction, start, count int, stack *virtualStack) int {
	ends := make([]int, count)
	depth := count
	for i := start + count; i < len(code); i++ {
		switch code[i].Op {
		case OpBlock, OpLoop, OpIf, OpTry:
			depth++
		case OpEnd:
			depth--
			if depth < count && ends[depth] == 0 {
				ends[depth] = i
			}
		}
		if depth == 0 {
			break
		}
	}
	for i := 0; i < count; i++ {
		bt := c.blockType(code[start+i].BlockType)
		c.blockStack = append(c.blockStack, blockEntry{kind: blockFLAT, savedDepth: max(0, stack.depth-len(bt.Params)), results: len(bt.Results), params: len(bt.Params), resultTypes: bt.Results})
	}
	segment := start + count
	for i := count - 1; i >= 0; i-- {
		c.line("DO 1 TIMES.")
		c.indent++
		c.line("IF %s = 0.", c.brVar())
		c.indent++
		c.emitInstructions(f, code[segment:ends[i]], stack, 0)
		entry := c.blockStack[len(c.blockStack)-1]
		c.copyLabelValues(entry.savedDepth, entry.results, stack)
		stack.depth = entry.savedDepth
		for _, typ := range entry.resultTypes {
			stack.next = typ
			stack.push()
		}
		c.indent--
		c.line("ENDIF.")
		c.indent--
		c.line("ENDDO.")
		c.blockStack = c.blockStack[:len(c.blockStack)-1]
		c.emitBrConsume()
		segment = ends[i] + 1
	}
	c.emitBrPropagate()
	return ends[0]
}

func checkABAPNesting(src string) error {
	depth := 0
	scope := "source"
	var statement strings.Builder
	var quote byte
	comment := false
	lineStart := true
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if ch == '\n' {
			comment = false
			lineStart = true
		}
		if comment {
			continue
		}
		if quote != 0 {
			if quote == '|' && ch == '\\' {
				i++
			} else if ch == quote {
				if i+1 < len(src) && src[i+1] == quote {
					i++
				} else {
					quote = 0
				}
			}
			continue
		}
		if ch == '"' || (lineStart && ch == '*') {
			comment = true
			continue
		}
		if ch != '\n' {
			lineStart = false
		}
		if ch == '\'' || ch == '`' || ch == '|' {
			quote = ch
			statement.WriteByte(' ')
			continue
		}
		if ch != '.' {
			statement.WriteByte(ch)
			continue
		}
		fields := strings.Fields(statement.String())
		statement.Reset()
		if len(fields) == 0 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "METHOD", "FORM", "FUNCTION":
			if len(fields) > 1 {
				scope = fields[1]
			}
			depth = 0
		case "IF", "DO", "WHILE", "CASE", "TRY", "LOOP":
			depth++
			if depth > maxABAPNesting {
				return fmt.Errorf("ABAP nesting depth %d exceeds limit %d in %s; control flow cannot be flattened", depth, maxABAPNesting, scope)
			}
		case "ENDIF", "ENDDO", "ENDWHILE", "ENDCASE", "ENDTRY", "ENDLOOP":
			depth--
		}
	}
	return nil
}
