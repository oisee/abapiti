package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/oisee/abapiti/hir/abap"
	corpusabap "github.com/oisee/abapiti/tools/allbackends/corpus-abap"
	"github.com/oisee/abapiti/tsfront"
)

var abapObjectRef = regexp.MustCompile(`(?i)\b(?:zcl_lnt_|zif_lnt_|zcx_lnt_|zlnt_)[a-z0-9_]+\b`)
var a4hObjectName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,29}$`)
var abapQuoted = regexp.MustCompile("`(?:``|[^`])*`|'(?:''|[^'])*'|\"[^\n]*")

// Partition by the static dependency closure of ALL original core declarations,
// not a workload trace. The remaining objects are test-owned synthesized types,
// closures, mocha/chai, registrations and the driver. Reject reverse edges.
func splitTestPackages(files map[string]string) (map[string]string, map[string]string, error) {
	var names map[string]abap.SourceEntry
	if err := json.Unmarshal([]byte(files["names.json"]), &names); err != nil {
		return nil, nil, err
	}
	byObject := map[string]string{}
	for file := range files {
		if strings.HasSuffix(file, ".abap") {
			byObject[strings.ToUpper(strings.Split(file, ".")[0])] = file
		}
	}
	core := map[string]string{}
	tests := map[string]string{}
	queue := []string{}
	for _, file := range []string{"zcl_lnt_registry_a4h.clas.abap", "zlnt_registry_run.prog.abap", "zlnt_registry_clean.prog.abap", "zlnt_registry_neg.prog.abap"} {
		if source, ok := files[file]; ok {
			core[file] = source
			queue = append(queue, strings.ToUpper(strings.Split(file, ".")[0]))
		}
	}
	for name, entry := range names {
		if strings.HasPrefix(entry.ID, "src/") || strings.HasPrefix(entry.ID, "node_modules/") || strings.HasPrefix(entry.ID, "harness/") {
			if file := byObject[name]; file != "" {
				core[file] = files[file]
				queue = append(queue, name)
			}
		}
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		text := abapQuoted.ReplaceAllString(files[byObject[name]], " ")
		for _, ref := range abapObjectRef.FindAllString(text, -1) {
			ref = strings.ToUpper(ref)
			file := byObject[ref]
			if file == "" || core[file] != "" {
				continue
			}
			id := names[ref].ID
			if strings.HasPrefix(id, "test/") || strings.HasPrefix(id, "runtime/chai.ts") || strings.HasPrefix(id, "runtime/mocha.ts") || strings.Contains(id, "test/") || strings.HasPrefix(id, "closure.") && (strings.HasPrefix(names[ref].Source, "test/") || strings.HasPrefix(names[ref].Source, "runtime/")) || strings.HasPrefix(ref, "ZCL_LNT_TEST_") {
				return nil, nil, fmt.Errorf("core %s has a reverse dependency on test object %s (%s)", name, ref, id)
			}
			core[file] = files[file]
			queue = append(queue, ref)
		}
	}
	for file, source := range files {
		if strings.HasSuffix(file, ".abap") && core[file] == "" {
			tests[file] = source
		}
	}
	return core, tests, nil
}

// Keep the driver emitted with the union's name/shape table. A separately
// emitted plain CLI driver may use different names or method signatures.
func splitA4HRegistry(core map[string]string) (map[string]string, error) {
	registry := map[string]string{}
	for file, source := range core {
		name := strings.ToLower(strings.Split(file, ".")[0])
		switch name {
		case "zcl_lnt_registry_a4h", "zlnt_registry_run", "zlnt_registry_clean", "zlnt_registry_neg":
			registry[file] = source
			delete(core, file)
		}
	}
	for file, source := range core {
		text := strings.ToLower(file + "\n" + source)
		for _, forbidden := range []string{"zcl_lnt_registry_a4h", "zlnt_registry_run", "zlnt_registry_clean", "zlnt_registry_neg", "zcl_abapiti_"} {
			if strings.Contains(text, forbidden) {
				return nil, fmt.Errorf("release core %s references %s", file, forbidden)
			}
		}
	}
	entries, err := corpusabap.Runtime.ReadDir("src")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		raw, err := corpusabap.Runtime.ReadFile("src/" + entry.Name())
		if err != nil {
			return nil, err
		}
		if entry.Name() != "package.devc.xml" {
			registry[entry.Name()] = string(raw)
		}
	}
	bal, err := tsfront.A4HTestLogBAL()
	if err != nil {
		return nil, err
	}
	registry["zcl_lnt_test_log_bal.clas.abap"] = bal
	return registry, nil
}

func validateA4HSources(packages ...map[string]string) error {
	seen := map[string]bool{}
	for _, sources := range packages {
		local := map[string]bool{}
		for file, source := range sources {
			name := strings.Split(file, ".")[0]
			if !a4hObjectName.MatchString(name) {
				return fmt.Errorf("invalid A4H object name %s", name)
			}
			if seen[name] && !local[name] {
				return fmt.Errorf("duplicate A4H object across packages: %s", name)
			}
			seen[name] = true
			local[name] = true
			if strings.HasSuffix(file, ".abap") {
				for i, line := range strings.Split(source, "\n") {
					if len(line) > 255 {
						return fmt.Errorf("%s:%d exceeds 255 bytes", file, i+1)
					}
				}
			}
		}
	}
	return nil
}

var a4hTestBALRef = regexp.MustCompile(`(?i)\b(?:bal_|balloghndl\b|symsgty\b)`)

// Tests layers must activate without the optional SAP BAL adapter.
func validateA4HTestSources(sources map[string]string) error {
	for file, source := range sources {
		if strings.HasPrefix(strings.ToLower(filepath.Base(file)), "zcl_lnt_test_log_bal.") {
			return fmt.Errorf("BAL adapter must be packaged only in the registry archive: %s", file)
		}
		if forbidden := a4hTestBALRef.FindString(source); forbidden != "" {
			return fmt.Errorf("tests object %s depends on SAP BAL: %s", file, forbidden)
		}
	}
	return nil
}

func buildAbaplintTestsA4H(dir, out string, lowering *tsfront.RegistryLowering) error {
	var manifest struct {
		Shards []struct {
			Name string `json:"name"`
		} `json:"shards"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "closure.json"))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return err
	}
	expected := strings.Split("abap,abap/expressions,abap/lexer,abap/nodes,abap/statements,abap/structures,abap/syntax,abap/types,root,cds,ddl,lsp,objects,objects/rename,pretty_printer,rules,utils", ",")
	seen := map[string]bool{}
	for _, s := range manifest.Shards {
		if seen[s.Name] {
			return fmt.Errorf("duplicate shard %s", s.Name)
		}
		seen[s.Name] = true
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("a4h requires the full 17-shard union")
	}
	for _, name := range expected {
		if !seen[name] {
			return fmt.Errorf("a4h missing shard %s", name)
		}
	}
	files, report, err := tsfront.EmitAbaplintTestsCore(dir, lowering, zabapgitRunSHA)
	if err != nil {
		return err
	}
	if err := writeSources(filepath.Join(out, "abap"), files); err != nil {
		return err
	}
	if err := writeSources(filepath.Join(out, "native"), map[string]string{"zabaplint.prog.abap": report}); err != nil {
		return err
	}
	core, tests, err := splitTestPackages(files)
	if err != nil {
		return err
	}
	registry, err := splitA4HRegistry(core)
	if err != nil {
		return err
	}
	if err := writeSources(filepath.Join(out, "registry"), registry); err != nil {
		return err
	}
	if err := writeSources(filepath.Join(out, "core"), core); err != nil {
		return err
	}
	if err := writeSources(filepath.Join(out, "native", "lib"), core); err != nil {
		return err
	}
	if err := writeSources(filepath.Join(out, "tests-osg"), tests); err != nil {
		return err
	}
	delete(tests, "zcl_abapiti_test_runner.clas.abap")
	runner := strings.ReplaceAll(files["zcl_abapiti_test_runner.clas.abap"], "zcl_abapiti_test_runner", "zcl_lnt_test_runner")
	runner = strings.Replace(runner, "METHOD checkpoint.\n", "METHOD checkpoint.\nzcl_lnt_test_log=>append( value ).\n", 1)
	runner, err = abap.Format(runner)
	if err != nil {
		return err
	}
	tests["zcl_lnt_test_runner.clas.abap"] = runner
	support, err := tsfront.A4HTestSupport()
	if err != nil {
		return err
	}
	for file, source := range support {
		tests[file] = source
	}
	if err := validateA4HTestSources(tests); err != nil {
		return err
	}
	if err := validateA4HSources(core, tests, registry); err != nil {
		return err
	}
	if err := writeSources(filepath.Join(out, "tests"), tests); err != nil {
		return err
	}
	for _, part := range []struct {
		name, pkg string
		files     map[string]string
	}{{"zlnt", "$ZLNT", core}, {"zlnt_tests", "$ZLNT_TESTS", tests}, {"zlnt_registry", "$ZLNT_A4H", registry}} {
		zip := filepath.Join(out, part.name+".zip")
		count, err := writeAbapGitZip(zip, part.pkg, "abaplint full test closure", part.files)
		if err != nil {
			return err
		}
		if err := writeAbapGitManifest(zip, part.pkg); err != nil {
			return err
		}
		fmt.Printf("%s: %d objects\n", part.name, count)
	}
	var entries map[string]abap.SourceEntry
	if err := json.Unmarshal([]byte(files["names.json"]), &entries); err != nil {
		return err
	}
	for _, name := range []string{"ZCL_LNT_REGISTRY_A4H", "ZLNT_REGISTRY_RUN", "ZLNT_REGISTRY_CLEAN", "ZLNT_REGISTRY_NEG", "ZCL_LNT_TEST_LOG_BAL", "ZCL_LNT_TEST_RUNNER", "ZCL_LNT_TEST_LOG", "ZLNT_TESTS_RUN", "ZLNT_TESTRUN"} {
		filePrefix := strings.ToLower(name) + "."
		present := false
		for _, sources := range []map[string]string{core, tests, registry} {
			for file := range sources {
				if strings.HasPrefix(file, filePrefix) {
					present = true
				}
			}
		}
		if !present {
			continue
		}
		entries[name] = abap.SourceEntry{ID: "generated." + name, Kind: "generated", Source: "tsfront/abaplint_tests_a4h.go"}
	}
	data, err := json.MarshalIndent(entries, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "names.json"), append(data, '\n'), 0644)
}
