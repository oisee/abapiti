package tsfront

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// These probes execute the emitted ABAP on OSG when ABAPITI_OSG_HOME is set.
// The oracle deliberately uses Node's default localeCompare, independently
// of the development-time Intl.Collator table in hir/abap.
func TestLocaleCompareABAPNodeOracle(t *testing.T) {
	names := []string{
		"zcl_probe.clas.abap", "cx_root.clas.abap", "ZCL_PROBE.clas.abap", "Cx_Root.clas.abap",
		"a", "A", "aa", "Aa", "aA", "AA", "ab", "Ab", "aB", "AB",
		"a.clas.abap", "a.clas.xml", "a.clas.testclasses.abap", "a-b.clas.abap", "a_b.clas.abap",
		"a/b.clas.abap", "a b.clas.abap", "a#b.clas.abap", "a1.clas.abap", "a2.clas.abap", "a10.clas.abap",
		"1", "2", "10", "01", "_", "-", ".", "/", " ", "",
		"src", "src/a.clas.abap", "src/A.clas.abap", "src/a-b.clas.abap", "src/a_b.clas.abap",
		"package.devc.xml", "Package.devc.xml", "zcl_foo.clas.abap", "zcl_foo.clas.xml",
		"zif_foo.intf.abap", "ZIF_FOO.intf.abap", "#foo#bar.clas.abap", "README.md",
	}
	ascii := make([]string, 0, 95)
	for c := byte(32); c <= 126; c++ {
		ascii = append(ascii, string(c))
	}
	multi := []string{"", "a", "A", "aa", "aA", "Aa", "AA", "ab", "Ab", "aB", "AB", "a ", " a", "a_", "a-", "a.", "a/", "a0", "a1", "a9", "0a", "9A", " a", "a$", "a~", "~a", "[]", "{}", "a+", "a#", "`a", "a`"}
	for _, tc := range []struct{ name, source string }{
		{"regression", `class Probe {static run():string {const entries=[{name:"zcl_probe.clas.abap"},{name:"cx_root.clas.abap"}];return entries.sort((a,b)=>a.name.localeCompare(b.name)).map(e=>e.name).join("|");}}`},
		{"filenames", `class Probe {static run():string {const names:string[]=` + localeJSON(t, names) + `;return names.sort((a,b)=>a.localeCompare(b)).join("|");}}`},
		{"pairwise", `class Probe {
   static signs(xs:string[]):string {let out="";for(const a of xs){for(const b of xs){const n=a.localeCompare(b);out+=n<0?"-":n>0?"+":"0";}}return out;}
   static run():string {return Probe.signs(` + localeJSON(t, ascii) + `)+"|"+Probe.signs(` + localeJSON(t, multi) + `);}
  }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := strings.TrimSuffix(conversionOracle(t, tc.source), "\n")
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": tc.source}, []string{"probe.ts"})
			storageABAPProbe(t, p, "probe.ts.Probe", want, "")
		})
	}
}

func TestLocaleCompareABAPRefusesOutsideTable(t *testing.T) {
	// Invalid suffixes must be validated even after an earlier primary difference.
	for i, input := range []string{"zä", "z\u007f", "z\t", "z\u0000", "z😀"} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			source := `class Probe {static run():string{return "ok";}static invalid():number{return "a".localeCompare(` + localeJSON(t, input) + `);}}`
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
			storageABAPProbe(t, p, "probe.ts.Probe", "ok", "invalid", "cx_sy_range_out_of_bounds")
		})
	}
}

func localeJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
