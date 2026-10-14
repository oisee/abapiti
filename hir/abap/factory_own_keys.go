package abap

import (
	"strings"

	"github.com/oisee/abapiti/hir"
)

// New.Op identifies a checked library factory. A native slot layout alone
// never proves own-key presence: ordinary New expressions with nil OwnKeys
// continue to refuse. These factories retain the Node creation order.
func (b *body) factoryOwnKeys(x *hir.Expr, n string) {
	if x.Op != "lsp.RenameFile.create" && x.Op != "lsp.Position.create" {
		return
	}
	c := b.e.classBy(x.Type.Name)
	expected := map[string]hir.Kind{"line": hir.Number, "character": hir.Number}
	keys := []string{"line", "character"}
	suffix := ".Position"
	if x.Op == "lsp.RenameFile.create" {
		expected = map[string]hir.Kind{"kind": hir.String, "oldUri": hir.String, "newUri": hir.String, "options": hir.Optional, "annotationId": hir.Optional}
		keys = []string{"kind", "oldUri", "newUri"}
		suffix = ".RenameFile"
	}
	fields := map[string]hir.Type{}
	for base := c; base != nil; base = b.e.classBy(base.Super) {
		for _, f := range base.Fields {
			if !f.Static {
				fields[f.Name] = f.Type
			}
		}
	}
	valid := c != nil && c.Structural && strings.Contains(c.Name, "node_modules/vscode-languageserver-types/") && strings.HasSuffix(c.Name, suffix)
	valid = valid && len(fields) == len(expected) && c.Ctor != nil && len(c.Ctor.Params) == len(x.Args)
	for key, kind := range expected {
		t, ok := fields[key]
		matches := t.Kind == kind
		if kind == hir.Number {
			matches = matches || t.Kind == hir.I32 || t.Kind == hir.I64
		}
		valid = valid && ok && matches
	}
	if x.Op == "lsp.RenameFile.create" {
		opt := fields["options"]
		annotation := fields["annotationId"]
		valid = valid && len(opt.Args) == 1 && opt.Args[0].Kind == hir.ClassRef && len(annotation.Args) == 1 && annotation.Args[0].Kind == hir.String
	}
	if !valid {
		b.line("RAISE EXCEPTION TYPE " + b.e.name("view.refusal") + " EXPORTING reason = `factory_own_keys.go: checked LSP factory layout does not match its ABI`.")
		return
	}
	bag := n + "->" + b.e.name("view.ownKeys")
	b.line("CREATE OBJECT " + bag + ".\n" + bag + "->tag = " + b.e.name("runtime.dynamic") + "=>tag_object.")
	for _, key := range keys {
		b.markOwnKey(n, key)
	}
	if x.Op != "lsp.RenameFile.create" {
		return
	}
	// Node uses !== undefined, so false option flags still install options.
	// Inspect the original option object through its presence-aware adapter.
	options := n + "->" + b.e.member("options")
	d := b.temp(hir.T(hir.Dynamic))
	overwrite := b.temp(hir.T(hir.Dynamic))
	ignore := b.temp(hir.T(hir.Dynamic))
	b.line("IF " + options + " IS BOUND.")
	b.boxDynamic(d, options, fields["options"])
	b.line("CLEAR " + ignore + ".\n" + overwrite + " = " + d + "->get( `overwrite` ).\nIF " + overwrite + " IS NOT BOUND.\n" + ignore + " = " + d + "->get( `ignoreIfExists` ).\nENDIF.\nIF " + overwrite + " IS BOUND OR " + ignore + " IS BOUND.")
	b.markOwnKey(n, "options")
	b.line("ELSE.\nCLEAR " + options + ".\nENDIF.\nENDIF.")
	annotation := n + "->" + b.e.member("annotationId")
	b.line("IF " + annotation + " IS BOUND AND " + annotation + "->has = abap_true.")
	b.markOwnKey(n, "annotationId")
	b.line("ENDIF.")
}
