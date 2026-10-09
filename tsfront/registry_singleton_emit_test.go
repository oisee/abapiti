package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/internal/inlineoracle"
	"github.com/oisee/abapiti/tsfront/overrides"
)

func TestEmitRegistrySingletons(t *testing.T) {
	root := os.Getenv("REGISTRY_CLOSURE")
	if root == "" {
		root = "testdata/registrysingletons"
	}
	dir := t.TempDir()
	names := []string{"_abstract_type", "cgeneric_type", "clike_type", "pgeneric_type", "simple_type", "xgeneric_type"}
	var files []string
	for _, name := range names {
		file := "src/abap/types/basic/" + name + ".ts"
		raw, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(dir, file)
		if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(dest, raw, 0600); err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true},"include":["src/**/*.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := overrides.New(overrides.RegistryDeployment()...)
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithReachability(files, registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hasBlocking(diags) {
		t.Fatal(diags)
	}
	if errors := hir.Verify(prog); len(errors) > 0 {
		t.Fatal(errors, hir.Dump(prog))
	}
	inlineoracle.Check(t, prog)
	emitted, abapNames, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	classes := []string{"CGenericType", "CLikeType", "PGenericType", "SimpleType", "XGenericType"}
	raw, err := os.ReadFile("testdata/registryfeatures/singleton-oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle []struct {
		Name     string
		Identity bool
		Instance bool
		Text     string
		Generic  bool
	}
	if err = json.Unmarshal(raw, &oracle); err != nil {
		t.Fatal(err)
	}
	if len(oracle) != len(classes) {
		t.Fatal("incomplete original singleton oracle")
	}
	texts := make([]string, len(classes))
	for i, item := range oracle {
		if item.Name != classes[i] || !item.Identity || !item.Instance || !item.Generic {
			t.Fatal("unexpected original singleton observation", item)
		}
		texts[i] = item.Text
	}
	for i, class := range classes {
		native := abapNames.Get(files[i+1] + "." + class)
		get := abapNames.Get("member.get")
		body := fmt.Sprintf("CLASS ltcl_singleton DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS identity FOR TESTING.\nENDCLASS.\nCLASS ltcl_singleton IMPLEMENTATION.\nMETHOD identity.\nDATA(first) = %s=>%s( ).\nDATA(second) = %s=>%s( ).\ncl_abap_unit_assert=>assert_true( xsdbool( first = second ) ).\ncl_abap_unit_assert=>assert_true( xsdbool( first IS INSTANCE OF %s ) ).\ncl_abap_unit_assert=>assert_equals( act = first->%s( 0 ) exp = '%s' ).\ncl_abap_unit_assert=>assert_true( first->%s( ) ).\nENDMETHOD.\nENDCLASS.\n", native, get, native, get, native, abapNames.Get("member.toText"), texts[i], abapNames.Get("member.isGeneric"))
		emitted[native+".clas.testclasses.abap"] = body
	}
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		if err = os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for file, body := range emitted {
			if err = os.WriteFile(filepath.Join(out, file), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Every constructor dependency is pinned: altering the base must fail before lowering.
	base := filepath.Join(dir, files[0])
	raw, err = os.ReadFile(base)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), "this.data = input;", "this.data = undefined;", 1))
	if err = os.WriteFile(base, raw, 0600); err != nil {
		t.Fatal(err)
	}
	p, err = Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = p.LowerWithReachability(files, registry, nil); err == nil {
		t.Fatal("changed base constructor dependency was accepted")
	}
}
