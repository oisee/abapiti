package wasm

import (
	"fmt"
	"strings"
	"testing"
)

// buildMemoryModule uses the same binary section helpers as the other WASM tests.
func buildMemoryModule(min uint32, max bool) []byte {
	w := newWasmBuilder()
	types := []FuncType{{Params: []ValType{ValI32}, Results: []ValType{ValI32}}, {Results: []ValType{ValI32}}}
	w.addSection(1, buildTypeSection(types))
	if max {
		w.addSection(3, buildFuncSection([]int{0, 1, 1}))
	} else {
		w.addSection(3, buildFuncSection([]int{1}))
	}
	mem := []byte{1}
	if max {
		mem = append(mem, 1)
	} else {
		mem = append(mem, 0)
	}
	mem = append(mem, leb128u(min)...)
	if max {
		mem = append(mem, leb128u(2)...)
	}
	w.addSection(5, mem)
	if max {
		w.addSection(7, buildExportSection([]Export{{Name: "grow", Kind: 0, Index: 0}, {Name: "size", Kind: 0, Index: 1}, {Name: "storeload", Kind: 0, Index: 2}}))
		addr := append([]byte{OpI32Const}, leb128s(131071)...)
		store := append(append(append(append([]byte{}, addr...), OpI32Const), leb128s(123)...), 0x3a, 0, 0)
		store = append(store, addr...)
		store = append(store, 0x2d, 0, 0)
		w.addSection(10, buildCodeSection([][]byte{
			buildFuncBody(nil, []byte{OpLocalGet, 0, OpMemoryGrow, 0}),
			buildFuncBody(nil, []byte{OpMemorySize, 0}),
			buildFuncBody(nil, store),
		}))
	} else {
		w.addSection(7, buildExportSection([]Export{{Name: "size", Kind: 0, Index: 0}}))
		w.addSection(10, buildCodeSection([][]byte{buildFuncBody(nil, []byte{OpMemorySize, 0})}))
	}
	return w.bytes()
}

func TestMemoryGrowGeneration(t *testing.T) {
	for _, tc := range []struct {
		name  string
		min   uint32
		max   bool
		limit string
	}{
		{"bounded", 1, true, "2"}, {"zero", 0, false, "65536"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := buildMemoryModule(tc.min, tc.max)
			mod, err := Parse(bin)
			if err != nil {
				t.Fatal(err)
			}
			if mod.Memory.HasMax != tc.max {
				t.Fatalf("HasMax = %v", mod.Memory.HasMax)
			}
			if tc.max && mod.Memory.Max != 2 {
				t.Fatalf("Max = %d", mod.Memory.Max)
			}
			results := map[string]string{"class": mustCompile(t, mod, "zcl_test_mem")}
			for _, backend := range []BackendKind{BackendFUGR, BackendHybrid} {
				result := mustCompileWith(t, mod, "ztmem", backend, 10)
				var src strings.Builder
				for _, s := range result.Files {
					src.WriteString(s)
				}
				results[backend.String()] = src.String()
			}
			results["multi"] = mustCompileMultiClass(t, mod, "zcl_test_mem", 10).StateClass
			for name, src := range results {
				t.Run(name, func(t *testing.T) {
					for _, needle := range []string{"iv_pages < 0", "rv = -1.", "iv_pages > " + tc.limit + " -", "mem_zero_pages", "DO 8 TIMES.", "DO iv_pages TIMES."} {
						if !strings.Contains(src, needle) {
							t.Errorf("missing %q", needle)
						}
					}
					if strings.Contains(src, "* 256 TIMES") || strings.Contains(src, "alloc_mem(") {
						t.Error("slow allocator remains")
					}
					page := fmt.Sprintf("mv_mem_pages = %d.", tc.min)
					if name == "fugr" || name == "hybrid" {
						page = fmt.Sprintf("gv_mem_pages = %d.", tc.min)
					}
					if !strings.Contains(src, page) {
						t.Errorf("missing %q", page)
					}
				})
			}
		})
	}
}

func TestExplicitZeroMemoryMax(t *testing.T) {
	w := newWasmBuilder()
	w.addSection(5, []byte{1, 1, 0, 0}) // one memory, maximum present, min=0, max=0
	mod, err := Parse(w.bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !mod.Memory.HasMax || mod.Memory.Max != 0 {
		t.Fatalf("memory limits = %+v", mod.Memory)
	}
	if src := mustCompile(t, mod, "zcl_zero_max"); !strings.Contains(src, "iv_pages > 0 - mv_mem_pages") {
		t.Fatal("explicit maximum of zero was treated as unbounded")
	}
}

func TestMemoryGrowWazero(t *testing.T) {
	got := wazeroResults(t, buildMemoryModule(1, true), osdModules[2].cases)
	want := []int32{1, -1, -1, 2, 123}
	for i, v := range got {
		if v.trap || v.value != want[i] {
			t.Errorf("case %d = %+v, want %d", i, v, want[i])
		}
	}
	zero := wazeroResults(t, buildMemoryModule(0, false), osdModules[3].cases)
	if zero[0].trap || zero[0].value != 0 {
		t.Errorf("initial size = %+v", zero[0])
	}
}
