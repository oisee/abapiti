package tsfront

import (
	_ "embed"
	"strings"

	"github.com/oisee/abapiti/hir"
)

//go:embed golang_library.go.txt
var registryGoLibrary string

// RegistryGoLibrary exposes the pinned generated core without publishing its
// internal symbols. It is emitted beside hir.go, never patched into generated code.
// pkg and names must come from the same golang.EmitPackageWithNames call.
func RegistryGoLibrary(pkg string, names *hir.Names) string {
	return registryGoLibrarySymbols(pkg, names).Replace(registryGoLibrary)
}

func registryGoLibrarySymbols(pkg string, n *hir.Names) *strings.Replacer {
	pairs := []string{"@package@", pkg}
	for _, name := range []string{"parsePrivate", "run", "addFile", "addDependency", "parse", "getFileByName", "findObjectForFile", "findIssuesObject", "getFiles", "getObjects", "next", "close", "done", "value", "getRaw", "getObjectName", "getName", "setDirty", "isDirty", "updateFile", "removeFile", "getKey", "getSeverity", "getFilename", "getMessage", "getStart", "getEnd", "getRow", "getCol", "getObject", "getType", "getABAPFiles", "getMainABAPFile", "getSequencedFiles", "getStructure", "getStatements", "get", "getChildren", "getFirstStatement", "getFirstToken", "getLastToken", "getTokens", "getStr", "getTree", "getSQLViewName", "concatTokens", "parseType", "listKeys", "getComponents", "name"} {
		pairs = append(pairs, "@"+name+"@", n.Get("member."+name))
	}
	for marker, id := range map[string]string{
		"ExpressionNode":          "ref.src/abap/nodes/expression_node.ts.ExpressionNode",
		"DataDefinition":          "ref.src/objects/data_definition.ts.DataDefinition",
		"Table":                   "ref.src/objects/table.ts.Table",
		"View":                    "ref.src/objects/view.ts.View",
		"StructureType":           "ref.src/abap/types/basic/structure_type.ts.StructureType",
		"AbstractType":            "ref.src/abap/types/basic/_abstract_type.ts.AbstractType",
		"Component":               "ref.src/abap/types/basic/structure_type.ts.IStructureComponent",
		"Registry":                "ref.src/registry.ts.Registry",
		"ABAPObject":              "ref.src/objects/_abap_object.ts.ABAPObject",
		"ABAPFile":                "ref.src/abap/abap_file.ts.ABAPFile",
		"StatementNode":           "ref.src/abap/nodes/statement_node.ts.StatementNode",
		"StructureNode":           "ref.src/abap/nodes/structure_node.ts.StructureNode",
		"AbstractToken":           "ref.src/abap/1_lexer/tokens/abstract_token.ts.AbstractToken",
		"Position":                "ref.src/position.ts.Position",
		"PragmaDescriptor":        "descriptor.src/abap/1_lexer/tokens/pragma.ts.Pragma",
		"Object":                  "ref.src/objects/_iobject.ts.IObject",
		"File":                    "ref.src/files/_ifile.ts.IFile",
		"Issue":                   "ref.src/issue.ts.Issue",
		"clearParsingPerformance": "body.src/registry.ts.ParsingPerformance.clear",
		"newCrossIncludeMacros":   "new.src/abap/cross_include_macros.ts.CrossIncludeMacros",
		"newRegistry":             "new.src/registry.ts.Registry",
		"newConfig":               "new.src/config.ts.Config",
		"newRun":                  "new.harness/registry_run.ts.RegistryRun",
		"newFile":                 "new.src/files/memory_file.ts.MemoryFile",
	} {
		pairs = append(pairs, "@"+marker+"@", n.Get(id))
	}
	return strings.NewReplacer(pairs...)
}
