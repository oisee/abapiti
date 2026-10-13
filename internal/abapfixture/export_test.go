package abapfixture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrackFinalMapAndIndependentEmissions(t *testing.T) {
	base := t.TempDir()
	t.Setenv("ABAPITI_ABAP_FIXTURE_DIR", base)
	t.Run("fixture", func(t *testing.T) {
		first := map[string]string{"zcl.clas.abap": "first"}
		Track(t, "probe", first)
		first["zcl.clas.testclasses.abap"] = "unit"
		Track(t, "probe", map[string]string{"zcl.clas.abap": "second"})
		Track(t, "probe", nil)
	})
	dir := filepath.Join(base, "probe", t.Name(), "fixture")
	for path, want := range map[string]string{
		"emit-001/zcl.clas.abap":             "first",
		"emit-001/zcl.clas.testclasses.abap": "unit",
		"emit-002/zcl.clas.abap":             "second",
	} {
		got, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil || string(got) != want {
			t.Fatalf("%s: %q, %v; want %q", path, got, err, want)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("nil emission exported: %v, %v", entries, err)
	}
}

func TestOutputDirOptInAndNestedUnits(t *testing.T) {
	base := t.TempDir()
	t.Setenv("ABAPITI_TEST_OUT", "")
	t.Setenv("ABAPITI_ABAP_FIXTURE_DIR", "")
	if got := OutputDir(t, "probe"); got != "" {
		t.Fatal("export enabled by default", got)
	}
	t.Setenv("ABAPITI_ABAP_FIXTURE_DIR", base)
	t.Run("fixture", func(t *testing.T) {
		dir := OutputDir(t, "probe")
		if dir != OutputDir(t, "probe") {
			t.Fatal("output directory changed within one test")
		}
		dir = filepath.Join(dir, "case0")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, src := range map[string]string{"zcl.clas.abap": "class", "zcl.clas.testclasses.abap": "unit", "evidence.json": "{}"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0644); err != nil {
				t.Fatal(err)
			}
		}
	})
	dir := filepath.Join(base, "probe", t.Name(), "fixture", "emit-001")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("collected %v: %v", entries, err)
	}
}
