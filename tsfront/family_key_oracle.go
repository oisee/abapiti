package tsfront

import (
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// The oracle compares the equal/unequal relation of SCC ids with #186 ids.
// ABAPITI_FAMILY_KEY_ORACLE=<file> appends "scc-id<TAB>186-id|skip<TAB>acyclic|cyclic<TAB>type"
// for every outermost key; the #186 side gets a time budget per key.
type familyKeyOracleLog struct {
	mu     sync.Mutex
	budget time.Duration
	file   *os.File
	pairs  map[familyKeyID][2]string
	rows   [][2]string
	// Acyclic: #186 produced no back-reference; ids must be byte-identical.
	acyclic           int
	acyclicMismatches []string
}

var familyKeyOracle = func() *familyKeyOracleLog {
	path := os.Getenv("ABAPITI_FAMILY_KEY_ORACLE")
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "family key oracle:", err)
		return nil
	}
	return &familyKeyOracleLog{budget: 2 * time.Second, file: f, pairs: map[familyKeyID][2]string{}}
}()

type familyKeyOracleTimeout struct{}

func (l *lowerer) familyKeyOracleRecord(t *checker.Type, named bool, key string) {
	o := familyKeyOracle
	id := familyKeyID{t, named}
	o.mu.Lock()
	_, done := o.pairs[id]
	o.mu.Unlock()
	if done {
		return
	}
	backs := 0
	old := func() (key string) {
		defer func() {
			if p := recover(); p != nil {
				if _, ok := p.(familyKeyOracleTimeout); !ok {
					panic(p)
				}
				key = "skip"
			}
		}()
		return l.familyTypeKeyMerkle186(t, named, map[*checker.Type]bool{}, time.Now().Add(o.budget), &backs)
	}()
	shape := "cyclic"
	if backs == 0 && old != "skip" {
		shape = "acyclic"
		o.acyclic++
		if old != key {
			o.acyclicMismatches = append(o.acyclicMismatches, l.ck.TypeToString(t))
		}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pairs[id] = [2]string{key, old}
	o.rows = append(o.rows, [2]string{key, old})
	if o.file != nil {
		text := l.ck.TypeToString(t)
		if len(text) > 200 {
			text = text[:200] + "..."
		}
		fmt.Fprintf(o.file, "%s\t%s\t%s\t%s\n", key, old, shape, strings.ReplaceAll(text, "\n", " "))
	}
}

// familyKeyOracleCheck reports pairs whose equality differs between the two
// id schemes, the number of compared and the number of skipped keys.
func familyKeyOracleCheck(rows [][2]string) (violations []string, compared, skipped int) {
	byNew, byOld := map[string]map[string]bool{}, map[string]map[string]bool{}
	for _, r := range rows {
		if r[1] == "skip" {
			skipped++
			continue
		}
		compared++
		if byNew[r[0]] == nil {
			byNew[r[0]] = map[string]bool{}
		}
		if byOld[r[1]] == nil {
			byOld[r[1]] = map[string]bool{}
		}
		byNew[r[0]][r[1]] = true
		byOld[r[1]][r[0]] = true
	}
	for _, group := range []map[string]map[string]bool{byNew, byOld} {
		for k, set := range group {
			if len(set) > 1 {
				var others []string
				for o := range set {
					others = append(others, o)
				}
				sort.Strings(others)
				violations = append(violations, k+" ~ "+strings.Join(others, ","))
			}
		}
	}
	sort.Strings(violations)
	return violations, compared, skipped
}

// familyTypeKeyMerkle186 is the #186 id without memo: the reference for
// acyclic ids and for the equivalence oracle.
func (l *lowerer) familyTypeKeyMerkle186(t *checker.Type, named bool, visiting map[*checker.Type]bool, deadline time.Time, backs *int) (key string) {
	if time.Now().After(deadline) {
		panic(familyKeyOracleTimeout{})
	}
	defer func() {
		if !strings.HasPrefix(key, "back:") {
			sum := sha256.Sum256([]byte(key))
			key = fmt.Sprintf("%x", sum[:16])
		}
	}()
	if t == nil {
		return "missing"
	}
	if named {
		if t.Alias() != nil {
			if name := l.familySymbolKey(t.Alias().Symbol()); name != "" {
				for _, a := range t.Alias().TypeArguments() {
					name += "[" + l.familyTypeKeyMerkle186(a, true, map[*checker.Type]bool{}, deadline, backs) + "]"
				}
				return "alias:" + name
			}
		}
		if t.Symbol() != nil && t.Symbol().Flags&(ast.SymbolFlagsInterface|ast.SymbolFlagsClass) != 0 {
			name := l.familySymbolKey(t.Symbol())
			for _, a := range l.familyArguments(t) {
				name += "[" + l.familyTypeKeyMerkle186(a, true, visiting, deadline, backs) + "]"
			}
			return "decl:" + name
		}
	}
	if visiting[t] {
		*backs++
		return "back:" + l.familySymbolKey(t.Symbol())
	}
	visiting[t] = true
	defer delete(visiting, t)
	if t.IsUnion() || t.Flags()&checker.TypeFlagsIntersection != 0 {
		parts := []string{}
		for _, p := range t.AsUnionOrIntersectionType().Types() {
			parts = append(parts, l.familyTypeKeyMerkle186(p, true, visiting, deadline, backs))
		}
		sort.Strings(parts)
		return fmt.Sprintf("%d:%s", t.Flags(), strings.Join(parts, "|"))
	}
	if t.Flags()&checker.TypeFlagsObject != 0 {
		if args := l.familyArguments(t); len(args) > 0 {
			parts := []string{}
			for _, a := range args {
				parts = append(parts, l.familyTypeKeyMerkle186(a, true, visiting, deadline, backs))
			}
			name := "array"
			if t.Symbol() != nil {
				name = t.Symbol().Name
			}
			return name + "<" + strings.Join(parts, ",") + ">"
		}
		parts := []string{}
		for _, p := range l.ck.GetPropertiesOfType(t) {
			parts = append(parts, fmt.Sprintf("%q:%t:%t:%s", p.Name, p.Flags&ast.SymbolFlagsOptional != 0, shapePropertyReadonly(p), l.familyTypeKeyMerkle186(l.ck.GetTypeOfPropertyOfType(t, p.Name), true, visiting, deadline, backs)))
		}
		sort.Strings(parts)
		return "{" + strings.Join(parts, ";") + "}"
	}
	return l.ck.TypeToString(t)
}
