package tsfront

import (
	_ "embed"
	"github.com/oisee/abapiti/hir"
	"strings"
)

//go:embed golang_library.go.txt
var registryGoLibrary string

// RegistryGoLibrary exposes the pinned generated core without publishing its
// internal symbols. It is emitted beside hir.go, never patched into generated code.
// pkg must have been validated by golang.EmitPackage.
func RegistryGoLibrary(pkg string) string {
	return registryGoLibrarySymbols(pkg).Replace(registryGoLibrary)
}

func registryGoLibrarySymbols(pkg string) *strings.Replacer {
	n := hir.NewNames()
	pairs := []string{"@package@", pkg}
	for _, name := range []string{"addFile", "addDependency", "parse", "getFileByName", "findObjectForFile", "findIssuesObject", "getFiles", "getObjects", "next", "close", "done", "value", "getRaw", "getObjectName", "getName", "setDirty", "isDirty", "updateFile", "removeFile", "getKey", "getSeverity", "getFilename", "getMessage", "getStart", "getEnd", "getRow", "getCol"} {
		pairs = append(pairs, "@"+name+"@", n.Get("member."+name))
	}
	for marker, id := range map[string]string{
		"Registry": "ref.src/registry.ts.Registry",
		"Object":   "ref.src/objects/_iobject.ts.IObject",
		"File":     "ref.src/files/_ifile.ts.IFile",
		"Issue":    "ref.src/issue.ts.Issue",
		"newRun":   "new.harness/registry_run.ts.RegistryRun",
		"newFile":  "new.src/files/memory_file.ts.MemoryFile",
	} {
		pairs = append(pairs, "@"+marker+"@", n.Get(id))
	}
	return strings.NewReplacer(pairs...)
}
