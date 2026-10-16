package golang

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
)

// Only concrete representations whose Go equality already is SameValueZero
// may bypass normalization. Interfaces retain typed-nil handling; dynamic and
// optional Set values retain their unboxing and numeric canonicalization.
func (e *emitter) directCollectionKey(t hir.Type) bool {
	switch t.Kind {
	case hir.String, hir.Bool, hir.I32, hir.I64, hir.ClassValue, hir.RegExp, hir.Array, hir.OrderedMap, hir.OrderedSet:
		return true
	case hir.ClassRef:
		return t.Name != hir.RootObject && e.concreteClass(t.Name)
	case hir.Optional:
		return t.Args[0].IsRef() && e.directCollectionKey(t.Args[0])
	}
	return false
}

// Runtime adapters also instantiate collections (notably JSON's string maps).
// Rewrite type syntax, rather than text, so strings and comments are untouched.
// Alias discovery keeps generated concrete class pointers on the same path.
func directCollectionTypes(files map[string]string) error {
	type parsed struct {
		source    string
		tree      *ast.File
		positions *token.FileSet
	}
	parsedFiles := map[string]parsed{}
	aliases := map[string]ast.Expr{}
	for name, source := range files {
		positions := token.NewFileSet()
		tree, err := parser.ParseFile(positions, name, source, 0)
		if err != nil {
			return fmt.Errorf("collection types %s: %w", name, err)
		}
		parsedFiles[name] = parsed{source, tree, positions}
		for _, decl := range tree.Decls {
			if group, ok := decl.(*ast.GenDecl); ok {
				for _, spec := range group.Specs {
					if t, ok := spec.(*ast.TypeSpec); ok && t.Assign.IsValid() {
						aliases[t.Name.Name] = t.Type
					}
				}
			}
		}
	}
	var direct func(ast.Expr, int) bool
	direct = func(x ast.Expr, depth int) bool {
		if depth > 16 {
			return false
		}
		switch x := x.(type) {
		case *ast.StarExpr:
			id, ok := x.X.(*ast.Ident)
			return !ok || id.Name != "dynamic"
		case *ast.Ident:
			switch x.Name {
			case "jsString", "string", "bool", "int32", "int64":
				return true
			}
			if alias := aliases[x.Name]; alias != nil {
				return direct(alias, depth+1)
			}
		}
		return false
	}
	for name, p := range parsedFiles {
		type edit struct {
			start, end int
			value      string
		}
		var edits []edit
		ast.Inspect(p.tree, func(n ast.Node) bool {
			var base, key ast.Expr
			switch x := n.(type) {
			case *ast.IndexExpr:
				base, key = x.X, x.Index
			case *ast.IndexListExpr:
				base, key = x.X, x.Indices[0]
			default:
				return true
			}
			id, ok := base.(*ast.Ident)
			if ok && (id.Name == "orderedMap" || id.Name == "orderedSet") && direct(key, 0) {
				edits = append(edits, edit{p.positions.Position(id.Pos()).Offset, p.positions.Position(id.End()).Offset, "fast" + strings.ToUpper(id.Name[:1]) + id.Name[1:]})
			}
			return true
		})
		sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
		var out strings.Builder
		end := 0
		for _, edit := range edits {
			out.WriteString(p.source[end:edit.start])
			out.WriteString(edit.value)
			end = edit.end
		}
		out.WriteString(p.source[end:])
		files[name] = out.String()
	}
	return nil
}

// Copy non-index methods after all runtime extensions have been assembled, so
// record, live-array and JSON adapters stay identical for both representations.
func fastCollectionRuntime(source string) (string, error) {
	positions := token.NewFileSet()
	tree, err := parser.ParseFile(positions, "runtime.go", source, 0)
	if err != nil {
		return "", err
	}
	var extra strings.Builder
	fast := fastCollectionSource
	if strings.Contains(source, " inserts int32;") {
		if !strings.Contains(fast, " m.index[k]=len(m.Entries);") {
			panic("map insertion fast-path patch target moved")
		}
		fast = strings.Replace(fast, " m.index[k]=len(m.Entries);", " m.inserts++;m.index[k]=len(m.Entries);", 1)
	}
	extra.WriteString(fast)
	rename := strings.NewReplacer("orderedMap[", "fastOrderedMap[", "orderedSet[", "fastOrderedSet[")
	for _, decl := range tree.Decls {
		if group, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range group.Specs {
				if t, ok := spec.(*ast.TypeSpec); ok && t.Name.Name == "orderedMap" {
					text := source[positions.Position(group.Pos()).Offset:positions.Position(group.End()).Offset]
					text = rename.Replace(text)
					text = strings.ReplaceAll(text, "map[collectionIndexKey[K]]int", "map[K]int")
					extra.WriteString("\n" + text + "\n")
				}
			}
		}
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil {
			continue
		}
		recv, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		var base ast.Expr
		switch x := recv.X.(type) {
		case *ast.IndexExpr:
			base = x.X
		case *ast.IndexListExpr:
			base = x.X
		default:
			continue
		}
		id, ok := base.(*ast.Ident)
		if !ok || (id.Name != "orderedMap" && id.Name != "orderedSet") {
			continue
		}
		switch fn.Name.Name {
		case "ensureIndex", "set", "get", "has", "add", "delete":
			continue
		}
		extra.WriteString("\n" + rename.Replace(source[positions.Position(fn.Pos()).Offset:positions.Position(fn.End()).Offset]) + "\n")
	}
	return source + extra.String(), nil
}

const fastCollectionSource = `
// Concrete keys use Go's native typed indexes without boxing or normalization.
func (m *fastOrderedMap[K,V]) ensureIndex() {
 if m.index!=nil {return};m.index=make(map[K]int,len(m.Entries))
 for i,e:=range m.Entries {m.index[e.Key]=i}
}
func (m *fastOrderedMap[K,V]) set(k K,v V) *fastOrderedMap[K,V] {
 m.ensureIndex();if i,ok:=m.index[k];ok {m.Entries[i].Value=v;return m}
 m.index[k]=len(m.Entries);m.Entries=append(m.Entries,entry[K,V]{k,v});return m
}
func (m *fastOrderedMap[K,V]) get(k K) optional[V] {
 m.ensureIndex();if i,ok:=m.index[k];ok {return present(m.Entries[i].Value)};return optional[V]{}
}
func (m *fastOrderedMap[K,V]) has(k K) bool {m.ensureIndex();_,ok:=m.index[k];return ok}
func (m *fastOrderedMap[K,V]) delete(k K) bool {
 m.ensureIndex();i,ok:=m.index[k];if !ok {return false};delete(m.index,k)
 copy(m.Entries[i:],m.Entries[i+1:]);var zero entry[K,V];m.Entries[len(m.Entries)-1]=zero
 m.Entries=m.Entries[:len(m.Entries)-1];for j:=i;j<len(m.Entries);j++ {m.index[m.Entries[j].Key]=j};return true
}
type fastOrderedSet[T comparable] struct {Items []T;index map[T]int}
func (s *fastOrderedSet[T]) ensureIndex() {
 if s.index!=nil {return};s.index=make(map[T]int,len(s.Items));for i,v:=range s.Items {s.index[v]=i}
}
func (s *fastOrderedSet[T]) has(v T) bool {
 if len(s.Items)<=16 {for _,x:=range s.Items {if x==v {return true}};return false}
 s.ensureIndex();_,ok:=s.index[v];return ok
}
func (s *fastOrderedSet[T]) add(v T) *fastOrderedSet[T] {
 s.ensureIndex();if _,ok:=s.index[v];!ok {s.index[v]=len(s.Items);s.Items=append(s.Items,v)};return s
}
func (s *fastOrderedSet[T]) delete(v T) bool {
 s.ensureIndex();i,ok:=s.index[v];if !ok {return false};delete(s.index,v)
 copy(s.Items[i:],s.Items[i+1:]);var zero T;s.Items[len(s.Items)-1]=zero;s.Items=s.Items[:len(s.Items)-1]
 for j:=i;j<len(s.Items);j++ {s.index[s.Items[j]]=j};return true
}
func referenceFastSetFromArray[T comparable](s *fastOrderedSet[T],a *array[any]) *fastOrderedSet[T] {
 for _,v:=range a.Items {s.add(castRef[T](v))};return s
}
`
