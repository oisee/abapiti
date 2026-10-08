package tsfront

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"github.com/oisee/abapiti/tsfront/overrides"
)

// The supplied original inventory is expected data only. Production parsing
// flows through tsgo -> pinned override -> HIR -> ABAP adapter/runtime.
func TestEmitRegistryXML(t *testing.T) {
	oracle := os.Getenv("REGISTRY_XML_ORACLE")
	if oracle == "" {
		t.Skip("set REGISTRY_XML_ORACLE to original inventory and REGISTRY_XML_INPUTS to input roots")
	}
	roots := filepath.SplitList(os.Getenv("REGISTRY_XML_INPUTS"))
	if len(roots) == 0 {
		t.Fatal("missing XML input roots")
	}
	raw, err := os.ReadFile(oracle)
	if err != nil {
		t.Fatal(err)
	}
	var inventory []struct {
		Name, Type  string
		Files       []string
		XML         any
		Description *string
	}
	if err := json.Unmarshal(raw, &inventory); err != nil {
		t.Fatal(err)
	}
	if len(inventory) != 188 {
		t.Fatal("expected all 188 original objects")
	}
	dir := t.TempDir()
	source, err := os.ReadFile("testdata/registryfeatures/xml.ts")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "xml.ts"), source, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true},"files":["xml.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	entry := overrides.Entry{ID: "xml-fixture", Key: overrides.Key{File: "xml.ts", Symbol: "XMLProbe.parse", Kind: "KindMethodDeclaration"}, SHA256: "2694df07e7fa742006984694df4043d989b10a7f950b48ff4757d765c2e2f821", Rationale: "pinned subset adapter differential", Method: func() *hir.Method {
		return &hir.Method{Name: "parse", Static: true, Result: hir.T(hir.Dynamic), Params: []hir.Param{{Name: "xml", Type: hir.T(hir.String)}}, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "xml.parseSubset", Type: hir.T(hir.Dynamic), X: hir.V("xml", hir.T(hir.String))}})}
	}}
	rawEntry := overrides.Entry{ID: "xml-raw-fixture", Key: overrides.Key{File: "xml.ts", Symbol: "XMLProbe.raw", Kind: "KindMethodDeclaration"}, SHA256: "6d6fb6c53a947a2115784f19a8fe3f13179a2b5c48aa0ae3294c0b1e2f8ee974", Rationale: "same missing/XML adapter boundary as AbstractObject.parseRaw2", Method: func() *hir.Method { return overrides.XMLRawMethod("xml.ts.XMLProbe", "raw") }}
	registry, err := overrides.New(entry, rawEntry)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.LowerWithReachability([]string{"xml.ts"}, registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hasBlocking(diags) {
		t.Fatal(diags)
	}
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs, hir.Dump(prog))
	}
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	class, dynamic := names.Get("xml.ts.XMLProbe"), names.Get("runtime.dynamic")
	parsed := 0
	for i, obj := range inventory {

		var xml string
		for _, name := range obj.Files {
			if !strings.HasSuffix(name, ".xml") {
				continue
			}
			for _, root := range roots {
				bytes, readErr := os.ReadFile(filepath.Join(root, name))
				if readErr == nil {
					xml = string(bytes)
					break
				}
			}
			if xml != "" {
				break
			}
		}
		if xml == "" && obj.XML != nil {
			t.Fatalf("XML input missing for %s", obj.Name)
		}
		var b strings.Builder
		line := func(f string, args ...any) { fmt.Fprintf(&b, f+"\n", args...) }
		driver := fmt.Sprintf("z_xml_case_%03d", i)
		line("CLASS %s DEFINITION PUBLIC CREATE PUBLIC.", driver)
		line("PUBLIC SECTION.")
		line("PROTECTED SECTION.")
		line("PRIVATE SECTION.")
		line("ENDCLASS.")
		line("CLASS %s IMPLEMENTATION.", driver)
		line("ENDCLASS.")
		files[driver+".clas.abap"] = b.String()
		b.Reset()
		line("CLASS ltcl_xml DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.")
		line("PRIVATE SECTION.")
		line("METHODS metadata FOR TESTING.")
		line("ENDCLASS.")
		line("CLASS ltcl_xml IMPLEMENTATION.")
		line("METHOD metadata.")
		line("DATA xml TYPE string.")
		line("DATA ch TYPE c LENGTH 1.")
		for _, s := range abapStringBuild("xml", "ch", xml) {
			line("%s", s)
		}
		line("DATA root TYPE REF TO %s.", dynamic)
		if obj.XML == nil {
			line("DATA(probe) = NEW %s( ).", class)
			line("root = probe->%s( ).", names.Get("member.raw"))
			line("cl_abap_unit_assert=>assert_true( xsdbool( root IS NOT BOUND ) ).")
		} else {
			line("DATA wrapped TYPE REF TO %s.", names.Get("runtime.optional<string>"))
			line("CREATE OBJECT wrapped.")
			line("wrapped->has = abap_true.")
			line("wrapped->value = xml.")
			line("DATA(probe) = NEW %s( wrapped ).", class)
			line("root = probe->%s( ).", names.Get("member.raw"))
		}
		// Independent recursive assertions compare every key, array length, leaf
		// value and node tag. Counts also reject unexpected fields/elements.
		serial := 0
		var check func(any, string)
		check = func(value any, parent string) {
			switch v := value.(type) {
			case string:
				line("cl_abap_unit_assert=>assert_equals( act = %s->tag exp = 1 ).", parent)
				line("CLEAR xml.")
				for _, s := range abapStringBuild("xml", "ch", v) {
					line("%s", s)
				}
				line("cl_abap_unit_assert=>assert_equals( act = %s->sval exp = xml ).", parent)
			case map[string]any:
				line("cl_abap_unit_assert=>assert_equals( act = %s->tag exp = 5 ).", parent)
				line("cl_abap_unit_assert=>assert_equals( act = lines( %s->entries ) exp = %d ).", parent, len(v))
				keys := make([]string, 0, len(v))
				for key := range v {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					child := v[key]
					serial++
					local := fmt.Sprintf("node%d", serial)
					line("DATA %s TYPE REF TO %s.", local, dynamic)
					line("%s = %s->get( `%s` ).", local, parent, strings.ReplaceAll(key, "`", "``"))
					line("cl_abap_unit_assert=>assert_bound( %s ).", local)
					check(child, local)
				}
			case []any:
				line("cl_abap_unit_assert=>assert_equals( act = %s->tag exp = 6 ).", parent)
				line("cl_abap_unit_assert=>assert_equals( act = lines( %s->items ) exp = %d ).", parent, len(v))
				for index, child := range v {
					serial++
					local := fmt.Sprintf("node%d", serial)
					line("DATA %s TYPE REF TO %s.", local, dynamic)
					line("READ TABLE %s->items INDEX %d INTO %s.", parent, index+1, local)
					check(child, local)
				}
			default:
				t.Fatalf("unexpected oracle XML leaf %T", value)
			}
		}
		if obj.XML != nil {
			check(obj.XML, "root")
			parsed++
		}
		if obj.Type == "CLAS" && obj.XML != nil && obj.Description != nil {
			line("DATA description TYPE string.")
			line("description = probe->%s( ).", names.Get("member.description"))
			line("CLEAR xml.")
			for _, s := range abapStringBuild("xml", "ch", *obj.Description) {
				line("%s", s)
			}
			line("cl_abap_unit_assert=>assert_equals( act = description exp = xml ).")
		}
		line("ENDMETHOD.")
		line("ENDCLASS.")
		files[driver+".clas.testclasses.abap"] = b.String()
	}
	// Failure contract: malformed nesting, unrecognized entity, CDATA and DTD
	// must raise the XML subset exception rather than yielding partial metadata.
	var b strings.Builder
	fmt.Fprintf(&b, "CLASS ltcl_reject DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS rejects FOR TESTING.\nMETHODS tagged FOR TESTING.\nENDCLASS.\nCLASS ltcl_reject IMPLEMENTATION.\nMETHOD rejects.\nDATA caught TYPE abap_bool.\nDATA value TYPE REF TO %s.\n", dynamic)
	fmt.Fprintf(&b, "DATA failure TYPE REF TO %s.\nDATA message TYPE string.\n", names.Get("exception.RegistryXMLSubsetError"))
	for _, text := range []string{"<a></b>", "<a>&unknown;</a>", "<a><![CDATA[x]]></a>", "<!DOCTYPE a><a/>"} {
		fmt.Fprintf(&b, "CLEAR caught.\nTRY.\nvalue = %s=>%s( `%s` ).\nCATCH %s INTO failure.\ncaught = abap_true.\nmessage = failure->get_text( ).\ncl_abap_unit_assert=>assert_true( xsdbool( strlen( message ) > 0 ) ).\nENDTRY.\ncl_abap_unit_assert=>assert_true( caught ).\n", class, names.Get("member.parse"), text, names.Get("exception.RegistryXMLSubsetError"))
	}
	// Include the structural lint canaries without affecting parser production.
	fmt.Fprintf(&b, "DATA(probe) = NEW %s( ).\ncl_abap_unit_assert=>assert_true( xsdbool( probe IS INSTANCE OF %s ) ).\nENDMETHOD.\nENDCLASS.\n", class, class)

	driver := strings.TrimSuffix(b.String(), "ENDCLASS.\n")
	driver += "METHOD tagged.\nDATA actual TYPE string.\n"
	taggedRaw, err := os.ReadFile("testdata/registryfeatures/tagged-oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var taggedCases []struct {
		Flag     bool
		Expected string
	}
	if err := json.Unmarshal(taggedRaw, &taggedCases); err != nil {
		t.Fatal(err)
	}
	if len(taggedCases) != 2 {
		t.Fatal("missing tagged original observations")
	}
	for _, c := range taggedCases {
		flag := "abap_false"
		if c.Flag {
			flag = "abap_true"
		}
		driver += fmt.Sprintf("actual = %s=>%s( %s ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = `%s` ).\n", class, names.Get("member.tagged"), flag, c.Expected)
	}
	driver += "ENDMETHOD.\nENDCLASS.\n"

	files[class+".clas.testclasses.abap"] = driver
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(out, name), []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Logf("%d original object observations: %d full XML trees, %d absent XML; %d emitted files", len(inventory), parsed, len(inventory)-parsed, len(files))
}
