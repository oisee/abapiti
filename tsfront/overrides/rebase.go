package overrides

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// ForFiles relocates package-relative adapters into an importing project's
// file identities. Fingerprints are still checked by Lookup. Multiple copies
// of an adapted package are ambiguous and must not silently share an adapter.
func (r *Registry) ForFiles(files []string) (*Registry, error) {
	roots := map[string]map[string]bool{}
	for _, e := range r.Inventory() {
		i := strings.IndexByte(e.Key.File, '/')
		if i < 0 {
			continue
		}
		root := e.Key.File[:i+1]
		for _, file := range files {
			if file == e.Key.File || strings.HasSuffix(file, "/"+e.Key.File) {
				if roots[root] == nil {
					roots[root] = map[string]bool{}
				}
				roots[root][strings.TrimSuffix(file, e.Key.File)] = true
			}
		}
	}
	prefixes := map[string]string{}
	var ordered []string
	for root, candidates := range roots {
		if len(candidates) > 1 {
			var paths []string
			for p := range candidates {
				paths = append(paths, p+root)
			}
			sort.Strings(paths)
			return nil, fmt.Errorf("ambiguous override package %s: %s", root, strings.Join(paths, ", "))
		}
		for prefix := range candidates {
			if prefix != "" {
				prefixes[root] = prefix + root
				ordered = append(ordered, root)
			}
		}
	}
	if len(prefixes) == 0 {
		return r, nil
	}
	sort.Strings(ordered)
	rename := func(name string) string {
		for _, root := range ordered {
			if strings.HasPrefix(name, root) {
				return prefixes[root] + strings.TrimPrefix(name, root)
			}
		}
		return name
	}
	var entries []Entry
	for _, e := range r.Inventory() {
		e.Key.File = rename(e.Key.File)
		e.References = append([]Key(nil), e.References...)
		for i := range e.References {
			e.References[i].File = rename(e.References[i].File)
		}
		if build := e.Result; build != nil {
			e.Result = func() hir.Type { return rebaseType(build(), rename) }
		}
		e.Types = rebaseTypes(e.Types, rename)
		e.Method = rebaseBuild(e.Method, rename)
		e.UnprunedMethod = rebaseBuild(e.UnprunedMethod, rename)
		e.Interface = rebaseBuild(e.Interface, rename)
		e.Expression = rebaseBuild(e.Expression, rename)
		e.Body = rebaseBuild(e.Body, rename)
		e.Expressions = rebaseBuilders(e.Expressions, rename)
		e.Statements = rebaseBuilders(e.Statements, rename)
		e.Patterns = rebasePatterns(e.Patterns, rename)
		e.UnprunedPatterns = rebasePatterns(e.UnprunedPatterns, rename)
		entries = append(entries, e)
	}
	return New(entries...)
}

func rebasePatterns(p *Patterns, rename func(string) string) *Patterns {
	if p == nil {
		return nil
	}
	out := *p
	out.Expressions = rebaseBuilders(p.Expressions, rename)
	out.Statements = rebaseBuilders(p.Statements, rename)
	out.Annotations = rebaseTypes(p.Annotations, rename)
	out.AssertionTypes = rebaseTypes(p.AssertionTypes, rename)
	return &out
}

func rebaseType(t hir.Type, rename func(string) string) hir.Type {
	t.Name = rename(t.Name)
	if t.Args != nil {
		t.Args = append([]hir.Type{}, t.Args...)
		for i := range t.Args {
			t.Args[i] = rebaseType(t.Args[i], rename)
		}
	}
	return t
}

func rebaseTypes(m map[string]hir.Type, rename func(string) string) map[string]hir.Type {
	if m == nil {
		return nil
	}
	out := map[string]hir.Type{}
	for key, t := range m {
		out[key] = rebaseType(t, rename)
	}
	return out
}

func rebaseBuilders[T any](m map[string]func() *T, rename func(string) string) map[string]func() *T {
	if m == nil {
		return nil
	}
	out := map[string]func() *T{}
	for span, build := range m {
		out[span] = rebaseBuild(build, rename)
	}
	return out
}

func rebaseBuild[T any](build func() *T, rename func(string) string) func() *T {
	if build == nil {
		return nil
	}
	return func() *T {
		node := build() // Registry builders return fresh nodes.
		var visit func(reflect.Value)
		visit = func(v reflect.Value) {
			if !v.IsValid() {
				return
			}
			if v.Kind() == reflect.Pointer {
				if !v.IsNil() {
					visit(v.Elem())
				}
				return
			}
			if v.Type() == reflect.TypeOf(hir.Type{}) {
				v.Set(reflect.ValueOf(rebaseType(v.Interface().(hir.Type), rename)))
				return
			}
			switch v.Kind() {
			case reflect.Struct:
				// Only qualified identities and source locations move. Method,
				// field and local names, and literal payloads, are JS data.
				switch v.Type() {
				case reflect.TypeOf(hir.Expr{}):
					owner := v.FieldByName("Owner")
					owner.SetString(rename(owner.String()))
				case reflect.TypeOf(hir.Interface{}):
					name := v.FieldByName("Name")
					name.SetString(rename(name.String()))
				case reflect.TypeOf(hir.Node{}):
					for _, key := range []string{"Source", "SiteSource", "SiteOwner"} {
						field := v.FieldByName(key)
						field.SetString(rename(field.String()))
					}
				}
				for i := 0; i < v.NumField(); i++ {
					visit(v.Field(i))
				}
			case reflect.Slice:
				for i := 0; i < v.Len(); i++ {
					visit(v.Index(i))
				}
			}
		}
		visit(reflect.ValueOf(node))
		return node
	}
}
