package hir

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Readable names: ZCL_<P>_<CLASS>, ZIF_<P>_<INTERFACE>, ZCX_<P>_<EXCEPTION>
// for the program's classes and interfaces, UPPER_SNAKE member names and
// I_<NAME> parameters, with P the project code (LNT for abaplint). A name that
// two identities would share gets, for every one of them, the area tag of its
// path (ST_, SY_, ...), then its folder, then its file, then a 4-hex hash: the outcome does
// not depend on the order identities are met. Everything else (runtime,
// builtins, closures) keeps a hashed name under zlnt_. ABAP limits names to 30
// characters; the class part is shortened first, never the qualifiers.

const abapNameMax = 30

// NewReadableNames precomputes the names of p's classes, interfaces, members
// and parameters.
func NewReadableNames(p *Program, project string) *Names {
	n := NewNames()
	n.fixed = map[string]string{}
	n.fallback = "z" + strings.ToLower(project) + "_"
	exceptions := exceptionClasses(p)
	type decl struct{ id, prefix string }
	var decls []decl
	for _, c := range p.Classes {
		prefix := "ZCL_" + project + "_"
		if exceptions[c.Name] {
			prefix = "ZCX_" + project + "_"
		}
		decls = append(decls, decl{c.Name, prefix})
	}
	for _, i := range p.Interfaces {
		decls = append(decls, decl{i.Name, "ZIF_" + project + "_"})
	}
	sort.Slice(decls, func(a, b int) bool { return decls[a].id < decls[b].id })
	levels := []func(id, prefix string) string{
		func(id, prefix string) string { return fitName(prefix, "", simpleSnake(id), "") },
		func(id, prefix string) string { return fitName(prefix, "", keptSnake(id), "") },
		func(id, prefix string) string { return fitName(prefix, area(id), simpleSnake(id), "") },
		func(id, prefix string) string { return fitName(prefix, area(id)+folder(id), simpleSnake(id), "") },
		func(id, prefix string) string { return fitName(prefix, area(id)+file(id), simpleSnake(id), "") },
		func(id, prefix string) string { return fitName(prefix, area(id), simpleSnake(id), "_"+hash4(id)) },
	}
	// The harness objects around the translated classes (tsfront Drivers)
	// take these names; a translated class never does.
	taken := map[string]bool{}
	for _, r := range []string{"REGISTRY_A4H", "REGISTRY_OSG"} {
		taken["ZCL_"+project+"_"+r] = true
	}
	level := make([]int, len(decls))
	for round := 0; round < len(levels); round++ {
		count := map[string]int{}
		for i, d := range decls {
			count[levels[level[i]](d.id, d.prefix)]++
		}
		for name := range taken {
			count[name] += 2
		}
		changed := false
		for i, d := range decls {
			if count[levels[level[i]](d.id, d.prefix)] > 1 && level[i] < len(levels)-1 {
				level[i]++
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	for i, d := range decls {
		n.fixed[d.id] = strings.ToLower(levels[level[i]](d.id, d.prefix))
	}
	// Members and parameters: one ABAP name per TS name (member.* ids are
	// shared by every class, as before); two TS names with one spelling both
	// get a hash.
	members, params := map[string]bool{}, map[string]bool{}
	addMethod := func(m *Method) {
		members[m.Name] = true
		for _, a := range m.Params {
			params[a.Name] = true
		}
	}
	for _, c := range p.Classes {
		for _, f := range c.Fields {
			members[f.Name] = true
		}
		for _, m := range c.Methods {
			addMethod(m)
		}
		if c.Ctor != nil {
			for _, a := range c.Ctor.Params {
				params[a.Name] = true
			}
		}
	}
	for _, i := range p.Interfaces {
		for _, m := range i.Methods {
			addMethod(m)
		}
	}
	assign := func(kind string, names map[string]bool, render func(string, string) string) {
		keys := make([]string, 0, len(names))
		for k := range names {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		count := map[string]int{}
		for _, k := range keys {
			count[render(k, "")]++
		}
		for _, k := range keys {
			s := render(k, "")
			if count[s] > 1 || reservedComponent[s] {
				s = render(k, "_"+hash4(kind+k))
			}
			n.fixed[kind+k] = strings.ToLower(s)
		}
	}
	assign("member.", members, func(k, suffix string) string { return fitName("", "", snake(k), suffix) })
	assign("param.", params, func(k, suffix string) string { return fitName("I_", "", snake(k), suffix) })
	return n
}

// reservedComponent are names the emitter or ABAP itself uses inside a class.
var reservedComponent = map[string]bool{"ME": true, "SUPER": true, "RESULT": true, "CONSTRUCTOR": true, "CLASS_CONSTRUCTOR": true}

func exceptionClasses(p *Program) map[string]bool {
	super := map[string]string{}
	for _, c := range p.Classes {
		super[c.Name] = c.Super
	}
	out := map[string]bool{}
	for _, c := range p.Classes {
		for s, seen := c.Super, 0; s != "" && seen < 64; s, seen = super[s], seen+1 {
			if s == "builtin.Error" || strings.HasPrefix(s, "exception.") {
				out[c.Name] = true
				break
			}
		}
	}
	return out
}

// simpleSnake is the declaration's own name: after the last '.' of the id.
// A module class (a file's top-level code) is named after its file; a
// synthesized type without a source (union., shape., tuple., ...) after its
// kind and 8 hex of its identity.
func simpleSnake(id string) string { return declSnake(id, true) }

// keptSnake keeps TypeScript's I: the fallback when dropping it collides
// (IConfig next to a class Config).
func keptSnake(id string) string { return declSnake(id, false) }

func declSnake(id string, drop bool) string {
	if ts := strings.Index(id, ".ts."); ts >= 0 {
		if id[ts+4:] == "module" {
			file := id[:ts]
			if i := strings.LastIndexByte(file, '/'); i >= 0 {
				file = file[i+1:]
			}
			return snake(file) + "_MODULE"
		}
		name := id[ts+4:]
		if drop {
			name = dropInterfacePrefix(name)
		}
		return snake(name)
	}
	if i := strings.IndexByte(id, '.'); i > 0 {
		h := sha256.Sum256([]byte(id))
		return snake(id[:i]) + "_" + strings.ToUpper(fmt.Sprintf("%x", h[:4]))
	}
	return snake(id)
}

// dropInterfacePrefix removes TypeScript's I of IConfig / IABAPLexerResult:
// ABAP already says interface with ZIF_, and a data-shape interface lowered to
// a class needs no I either. Issue, Integer, Interface keep theirs.
func dropInterfacePrefix(name string) string {
	// I + Word (IConfig, ICandidate); IACBinaryData, IAMApp, IABAPLexerResult
	// keep the I: there it is part of an acronym (IAC, IAM) or ambiguous.
	if len(name) > 2 && name[0] == 'I' && name[1] >= 'A' && name[1] <= 'Z' && name[2] >= 'a' && name[2] <= 'z' {
		return name[1:]
	}
	return name
}

// area is a two-letter tag for the id's layer, from its path under src/.
func area(id string) string {
	parts := pathParts(id)
	if len(parts) == 0 {
		return ""
	}
	top := parts[0]
	if top == "abap" && len(parts) > 1 {
		top = parts[1]
	}
	tags := map[string]string{"1_lexer": "LX", "2_statements": "ST", "3_structures": "SR", "4_file_information": "FI", "5_syntax": "SY",
		"rules": "RL", "objects": "OB", "lsp": "LS", "utils": "UT", "cds": "CD", "pretty_printer": "PP", "files": "FL", "ddic": "DD", "nodes": "ND", "types": "TY"}
	if t, ok := tags[top]; ok {
		return t + "_"
	}
	s := snake(strings.TrimLeft(top, "0123456789_"))
	if len(s) > 2 {
		s = s[:2]
	}
	if s == "" {
		return ""
	}
	return s + "_"
}

// folder is the id's immediate folder, the second qualifier.
func folder(id string) string {
	parts := pathParts(id)
	if len(parts) < 2 {
		return ""
	}
	s := snake(strings.TrimLeft(parts[len(parts)-1], "0123456789_"))
	if len(s) > 6 {
		s = s[:6]
	}
	return s + "_"
}

// file is the id's file name, the third qualifier (one folder, several files
// declaring the same class name).
func file(id string) string {
	ts := strings.Index(id, ".ts.")
	if ts < 0 {
		return ""
	}
	f := id[:ts]
	if i := strings.LastIndexByte(f, '/'); i >= 0 {
		f = f[i+1:]
	}
	s := snake(strings.TrimLeft(f, "_"))
	if len(s) > 14 {
		s = s[:14]
	}
	return strings.Trim(s, "_") + "_"
}

// pathParts are the folders of a source-qualified id (src/abap/x/y.ts.Name).
func pathParts(id string) []string {
	ts := strings.Index(id, ".ts.")
	if ts < 0 {
		return nil
	}
	path := strings.TrimPrefix(id[:ts], "src/")
	parts := strings.Split(path, "/")
	return parts[:len(parts)-1]
}

// snake turns camelCase / PascalCase / snake into UPPER_SNAKE (ABAP charset).
func snake(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case unicode.IsUpper(r):
			if i > 0 && (unicode.IsLower(rs[i-1]) || unicode.IsDigit(rs[i-1]) || (i+1 < len(rs) && unicode.IsLower(rs[i+1]) && unicode.IsUpper(rs[i-1]))) {
				b.WriteByte('_')
			}
			b.WriteRune(r)
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(unicode.ToUpper(r))
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	for strings.Contains(out, "__") {
		out = strings.ReplaceAll(out, "__", "_")
	}
	if out == "" || out[0] >= '0' && out[0] <= '9' {
		out = "N" + out
	}
	return out
}

// fitName joins prefix, qualifier, base and suffix within 30 characters,
// shortening only the base: vowels inside words first, then the words' tails.
func fitName(prefix, qual, base, suffix string) string {
	room := abapNameMax - len(prefix) - len(qual) - len(suffix)
	if room < 1 {
		room = 1
	}
	if len(base) > room {
		base = dropVowels(base)
	}
	for len(base) > room {
		words := strings.Split(base, "_")
		longest := 0
		for i, w := range words {
			if len(w) > len(words[longest]) {
				longest = i
			}
		}
		if len(words[longest]) <= 1 {
			base = base[:room]
			break
		}
		words[longest] = words[longest][:len(words[longest])-1]
		base = strings.Join(words, "_")
	}
	return prefix + qual + strings.Trim(base, "_") + suffix
}

// dropVowels removes vowels after the first letter of each word.
func dropVowels(s string) string {
	words := strings.Split(s, "_")
	for i, w := range words {
		var b strings.Builder
		for j, r := range w {
			if j > 0 && strings.ContainsRune("AEIOU", r) {
				continue
			}
			b.WriteRune(r)
		}
		words[i] = b.String()
	}
	return strings.Join(words, "_")
}

func hash4(id string) string {
	h := sha256.Sum256([]byte(id))
	return strings.ToUpper(fmt.Sprintf("%x", h[:2]))
}
