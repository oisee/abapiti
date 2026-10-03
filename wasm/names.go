package wasm

import (
	"fmt"
	"strings"
)

// abapNameAllocator allocates identifiers in a case-insensitive ABAP namespace.
// Traversal order is fixed by WASM function index; no map iteration assigns names.
type abapNameAllocator struct{ used map[string]bool }

func (a *abapNameAllocator) reserve(name string) {
	if a.used == nil {
		a.used = make(map[string]bool)
	}
	a.used[strings.ToLower(name)] = true
}

func (a *abapNameAllocator) allocate(raw string) string {
	base := sanitizeABAP(raw)
	name := base
	for counter := 1; a.used[strings.ToLower(name)]; counter++ {
		suffix := fmt.Sprintf("_%d", counter)
		name = base[:min(len(base), 30-len(suffix))] + suffix
	}
	a.reserve(name)
	return name
}

func sanitizeABAP(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	name = b.String()
	if name == "" {
		name = "wasm_func"
	}
	if name[0] >= '0' && name[0] <= '9' {
		name = "f_" + name
	}
	return name[:min(len(name), 30)]
}

func moduleFunctionNames(mod *Module) []string {
	var a abapNameAllocator
	for _, name := range []string{"constructor", "wasm_init", "mem_ld_i32", "mem_st_i32", "mem_ld_i32_8u", "mem_ld_i32_8s", "mem_ld_i32_16u", "mem_st_i32_8", "mem_st_i32_16", "mem_grow", "mem_zero_pages"} {
		a.reserve(name)
	}
	declarations, _ := runtimeTemplates()
	for name := range declarations {
		a.reserve(name)
	}
	for i := range mod.Types {
		a.reserve(fmt.Sprintf("dispatch_t%d", i))
	}
	names := make([]string, len(mod.Functions))
	for i, f := range mod.Functions {
		raw := f.ExportName
		if raw == "" {
			raw = fmt.Sprintf("f%d", i)
		}
		names[i] = a.allocate(raw)
	}
	return names
}

func (c *compiler) functionName(index int) string {
	if c.names == nil {
		c.names = moduleFunctionNames(c.mod)
	}
	return c.names[index]
}

func functionModuleNames(mod *Module, prefix string) []string {
	var a abapNameAllocator
	names := make([]string, len(mod.Functions))
	for i, f := range mod.Functions {
		if f.ExportName != "" && f.Type != nil {
			names[i] = strings.ToUpper(a.allocate(prefix + "_" + f.ExportName))
		}
	}
	return names
}

// CompileInterface emits the public WASM API with the same names as Compile.
func CompileInterface(mod *Module, name string) string {
	c := &compiler{mod: mod}
	c.line("INTERFACE %s PUBLIC.", name)
	c.indent++
	for i, f := range mod.Functions {
		if f.ExportName != "" && f.Type != nil {
			c.emitMethodSignature(c.functionName(i), f.Type, true)
		}
	}
	c.indent--
	c.line("ENDINTERFACE.")
	return wrapLongLines(c.sb.String())
}

// bodyName is the method or FORM that holds function index's body. With WASI
// export wrappers, an exported function's body lives under its internal name
// and the export name belongs to the wrapper.
func (c *compiler) bodyName(index int) string {
	if c.mod.Functions[index].ExportName != "" && c.wasiExportWrappers() {
		return fmt.Sprintf("f%d", index)
	}
	return c.functionName(index)
}
