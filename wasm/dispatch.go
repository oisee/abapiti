package wasm

import (
	"fmt"
	"sort"
)

func (c *compiler) dispatchTypes() []int {
	out := make([]int, 0, len(c.usedDispatch))
	for index := range c.usedDispatch {
		out = append(out, index)
	}
	sort.Ints(out)
	return out
}

func (c *compiler) emitDispatchDeclarations() {
	for _, index := range c.dispatchTypes() {
		if index >= len(c.mod.Types) {
			continue
		}
		ft := c.mod.Types[index]
		c.line("METHODS dispatch_t%d", index)
		c.indent++
		c.line("IMPORTING iv_idx TYPE i")
		for i, p := range ft.Params {
			c.line("p%d TYPE %s", i, p.ABAPType())
		}
		if len(ft.Results) > 0 {
			c.line("RETURNING VALUE(rv) TYPE %s.", ft.Results[0].ABAPType())
		} else {
			c.line(".")
		}
		c.indent--
	}
}

func (c *compiler) emitDispatchMethods() {
	tableTargets := make(map[int]bool)
	for _, elem := range c.mod.Elements {
		for _, index := range elem.FuncIndices {
			tableTargets[index] = true
		}
	}
	for _, index := range c.dispatchTypes() {
		if index >= len(c.mod.Types) {
			continue
		}
		ft := c.mod.Types[index]
		c.line("METHOD dispatch_t%d.", index)
		c.indent++
		c.line("CASE iv_idx.")
		c.indent++
		for i, f := range c.mod.Functions {
			if !tableTargets[f.Index] || f.TypeIndex != index {
				continue
			}
			c.line("WHEN %d.", f.Index)
			name := fmt.Sprintf("f%d", i)
			if f.ExportName != "" {
				name = sanitizeABAP(f.ExportName)
			}
			prefix := ""
			if len(ft.Results) > 0 {
				prefix = "rv = "
			}
			if len(ft.Params) == 0 {
				c.line("%s%s( ).", prefix, name)
				continue
			}
			c.line("%s%s(", prefix, name)
			c.indent++
			for j := range ft.Params {
				c.line("p%d = p%d", j, j)
			}
			c.indent--
			c.line(").")
		}
		c.line("WHEN OTHERS.")
		c.line("RAISE EXCEPTION TYPE cx_sy_program_error.")
		c.indent--
		c.line("ENDCASE.")
		c.indent--
		c.line("ENDMETHOD.")
	}
}
