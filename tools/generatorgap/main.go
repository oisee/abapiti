// generatorgap measures the pinned TS→HIR closure without reachability pruning.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oisee/abapiti/tsfront"
)

func main() {
	if len(os.Args) != 3 {
		panic("usage: generatorgap <core-source-dir> <diagnostics.json>")
	}
	dir := os.Args[1]
	for _, pkg := range tsfront.RegistryNodePackages() {
		if err := pkg.WriteEmbedded(filepath.Join(dir, "node_modules", pkg.Name)); err != nil {
			panic(err)
		}
	}
	config := `{"compilerOptions":{"module":"commonjs","target":"es2020","lib":["es2020"],"noEmit":true,"skipLibCheck":true,"strictNullChecks":true,"strictFunctionTypes":true,"noImplicitAny":true,"strictPropertyInitialization":false},"include":["src/**/*.ts","harness/**/*.ts"]}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(config), 0600); err != nil {
		panic(err)
	}
	harness := filepath.Join(dir, tsfront.RegistryHarnessPath)
	if err := os.MkdirAll(filepath.Dir(harness), 0755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(harness, tsfront.RegistryHarness(), 0600); err != nil {
		panic(err)
	}
	registry, err := tsfront.RegistryOverrides()
	if err != nil {
		panic(err)
	}
	os.Setenv("ABAPITI_ASSUME_INT", "")
	r, err := tsfront.LowerRegistry(dir, append(tsfront.EmbeddedRegistryClosure().Files(), tsfront.RegistryHarnessPath), registry, nil)
	if err != nil {
		panic(err)
	}
	b, err := json.MarshalIndent(r.Blocking, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(os.Args[2], append(b, '\n'), 0600); err != nil {
		panic(err)
	}
	routes := []tsfront.LowerDiagnostic{}
	for _, d := range r.Diagnostics {
		if d.Category == "note-generator-iterator" || d.Category == "unsupported-generator" {
			d.Loc = strings.TrimPrefix(d.Loc, dir+string(filepath.Separator))
			routes = append(routes, d)
		}
	}
	routeJSON, err := json.MarshalIndent(routes, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(os.Args[2], ".json")+"-generators.json", append(routeJSON, '\n'), 0600); err != nil {
		panic(err)
	}
	verify := []string{}
	for _, err := range r.Verification {
		verify = append(verify, strings.TrimPrefix(err.Error(), dir+string(filepath.Separator)))
	}
	if err := os.WriteFile(strings.TrimSuffix(os.Args[2], ".json")+"-verify.txt", []byte(strings.Join(verify, "\n")+"\n"), 0600); err != nil {
		panic(err)
	}
	counts := map[string]int{}
	for _, d := range r.Blocking {
		counts[d.Message]++
	}
	keys := []string{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("blocking=%d verification=%d\n", len(r.Blocking), len(r.Verification))
	for _, k := range keys {
		fmt.Printf("%d\t%s\n", counts[k], k)
	}
}
