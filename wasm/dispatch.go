package wasm

import (
	"fmt"
	"sort"
)

// callIndirectTrap is raised when call_indirect traps (index out of range,
// null slot, signature mismatch). CX_SY_PROGRAM_ERROR does not exist on SAP
// or OSD; CX_SY_DYN_CALL_ILLEGAL_METHOD does on both, is instantiable, and
// says what happened: a dynamic call to a method that cannot be called.
const callIndirectTrap = "RAISE EXCEPTION TYPE cx_sy_dyn_call_illegal_method."

// nullFuncRef marks a table slot no element segment initialises. No function
// has this index, so dispatching it reaches WHEN OTHERS and traps.
const nullFuncRef = -1

// elementTables lays the active element segments out per table: slot k holds
// the function index stored at table offset k, or nullFuncRef. A segment
// starts at its Offset (clang places the first function at 1), and a later
// segment overwrites an earlier one, as instantiation does.
func elementTables(mod *Module) (map[int][]int, []int) {
	tables := make(map[int][]int)
	for _, elem := range mod.Elements {
		if elem.Offset < 0 {
			continue
		}
		slots := tables[elem.TableIndex]
		for len(slots) < elem.Offset+len(elem.FuncIndices) {
			slots = append(slots, nullFuncRef)
		}
		copy(slots[elem.Offset:], elem.FuncIndices)
		tables[elem.TableIndex] = slots
	}
	indices := make([]int, 0, len(tables))
	for index := range tables {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	return tables, indices
}

// sameFuncType reports whether two function types are structurally equal,
// which is what call_indirect checks.
func sameFuncType(a, b *FuncType) bool {
	if a == nil || b == nil || len(a.Params) != len(b.Params) || len(a.Results) != len(b.Results) {
		return false
	}
	for i := range a.Params {
		if a.Params[i] != b.Params[i] {
			return false
		}
	}
	for i := range a.Results {
		if a.Results[i] != b.Results[i] {
			return false
		}
	}
	return true
}

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
			if !tableTargets[f.Index] || !sameFuncType(f.Type, &ft) {
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
		c.line("%s", callIndirectTrap)
		c.indent--
		c.line("ENDCASE.")
		c.indent--
		c.line("ENDMETHOD.")
	}
}
