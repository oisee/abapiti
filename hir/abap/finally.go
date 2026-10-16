package abap

import "github.com/oisee/abapiti/hir"

// Completion exceptions are private control transport, outside the TS throw
// hierarchy. A source catch must never consume a pending return or loop exit.
func (b *body) completionClass(kind hir.StmtKind) string {
	key := "completion." + string(kind)
	if kind == hir.Return {
		key += "." + b.m.Result.String()
	}
	name := b.e.name(key)
	file := name + ".clas.abap"
	if _, ok := b.e.files[file]; !ok {
		field := ""
		if kind == hir.Return && b.m.Result.Kind != hir.Void {
			field = "DATA value TYPE " + b.e.typ(b.m.Result) + ".\n"
		}
		b.e.files[file] = "CLASS " + name + " DEFINITION PUBLIC INHERITING FROM cx_no_check CREATE PUBLIC.\nPUBLIC SECTION.\n" + field + "PROTECTED SECTION.\nPRIVATE SECTION.\nENDCLASS.\nCLASS " + name + " IMPLEMENTATION.\nENDCLASS.\n"
	}
	return name
}

func (b *body) complete(kind hir.StmtKind, value string) {
	crossing := len(b.finalizers) > 0 && (kind == hir.Return || b.loopDepth <= b.finalizers[len(b.finalizers)-1])
	if crossing {
		name := b.completionClass(kind)
		signal := b.rawTemp("REF TO " + name)
		b.line(signal + " = NEW #( ).")
		if value != "" {
			b.line(signal + "->value = " + value + ".")
		}
		b.line("RAISE EXCEPTION " + signal + ".")
		return
	}
	switch kind {
	case hir.Return:
		if value != "" {
			b.line("result = " + value + ".")
		}
		b.line("RETURN.")
	case hir.Break:
		b.line("EXIT.")
	case hir.Continue:
		b.line("CONTINUE.")
	}
}

// Only control that leaves this region needs transport. Local nested-loop
// breaks and continues remain native; returns always leave the region.
func finallyCompletions(s *hir.Stmt, depth int, kinds map[hir.StmtKind]bool) {
	if s == nil {
		return
	}
	switch s.Kind {
	case hir.Return:
		kinds[hir.Return] = true
	case hir.Break, hir.Continue:
		if depth == 0 {
			kinds[s.Kind] = true
		}
	case hir.While, hir.ForEach:
		depth++
	}
	for _, child := range s.List {
		finallyCompletions(child, depth, kinds)
	}
	finallyCompletions(s.Body, depth, kinds)
	finallyCompletions(s.Else, depth, kinds)
}

func (b *body) finally(s *hir.Stmt) {
	kinds := map[hir.StmtKind]bool{}
	finallyCompletions(s.Body, 0, kinds)
	finallyCompletions(s.Else, 0, kinds)
	pending := b.rawTemp("REF TO cx_root")
	if len(kinds) == 0 {
		// Preserve the compact exception-only lowering when nothing can escape.
		b.line("TRY.")
		b.stmt(s.Body)
		b.line("CATCH cx_root INTO " + pending + ".")
		b.stmt(s.Else)
		b.line("RAISE EXCEPTION " + pending + ".\nENDTRY.")
		b.stmt(s.Else)
		return
	}
	b.line("CLEAR " + pending + ".\nTRY.")
	b.finalizers = append(b.finalizers, b.loopDepth)
	b.stmt(s.Body)
	b.line("CATCH cx_root INTO " + pending + ".\nENDTRY.\nTRY.")
	b.stmt(s.Else)
	b.finalizers = b.finalizers[:len(b.finalizers)-1]
	// Catch a replacement completion separately so the finalizer runs once.
	b.line("CATCH cx_root INTO " + pending + ".\nENDTRY.\nIF " + pending + " IS BOUND.\nTRY.\nRAISE EXCEPTION " + pending + ".")
	for _, kind := range []hir.StmtKind{hir.Return, hir.Break, hir.Continue} {
		if !kinds[kind] {
			continue
		}
		name := b.completionClass(kind)
		signal := b.rawTemp("REF TO " + name)
		b.line("CATCH " + name + " INTO " + signal + ".")
		value := ""
		if kind == hir.Return && b.m.Result.Kind != hir.Void {
			value = signal + "->value"
		}
		b.complete(kind, value)
	}
	b.line("CATCH cx_root.\nRAISE EXCEPTION " + pending + ".\nENDTRY.\nENDIF.")
}
