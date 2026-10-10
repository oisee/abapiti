package golang

import (
	"fmt"
	"go/scanner"
	"go/token"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
)

type emittedSite struct {
	node   hir.Node
	kind   string
	offset int
}

func (e *emitter) mark(n hir.Node, kind string) {
	e.sites = append(e.sites, emittedSite{n, kind, e.code.Len()})
}
func (e *emitter) profileIndex(n hir.Node, kind string) int {
	if i, ok := e.profileIndices[n.SiteID]; ok {
		return i
	}
	i := len(e.profileNodes)
	e.profileIndices[n.SiteID] = i
	e.profileNodes = append(e.profileNodes, hir.Site{SiteID: n.SiteID, Kind: kind})
	return i
}

// EmitWithSites emits the ordinary Go package and an opt-in profile_sites
// build variant. The default hir.go body is byte-identical to Emit output;
// the build constraint selects exactly one body. Sites describe default Go
// locations; go_profile locations identify the instrumented variant.
func EmitWithSites(p *hir.Program) (map[string]string, *hir.SiteMap, error) {
	files, e, err := emitPackage(p, "main", false)
	if err != nil {
		return nil, nil, err
	}
	sites := hir.NewSiteMap()
	if err := e.addLocations(sites, files["hir.go"], "go", "hir.go", 2); err != nil {
		return nil, nil, err
	}
	prof, pe, err := emitPackage(p, "main", true)
	if err != nil {
		return nil, nil, err
	}
	pm := hir.NewSiteMap()
	if err := pe.addLocations(pm, prof["hir.go"], "go_profile", "hir_profile.go", 2); err != nil {
		return nil, nil, err
	}
	// Emission order may differ because instrumentation introduces closures.
	// Join occurrences by original identity and inline context, not serial ID.
	byKey := map[string][]hir.SiteLocation{}
	key := func(s hir.Site) string { return s.SiteID + "\x00" + strings.Join(s.InlinePath, "\x00") }
	for _, s := range pm.Sites {
		k := key(s)
		byKey[k] = append(byKey[k], s.Locations["go_profile"])
	}
	for i := range sites.Sites {
		s := &sites.Sites[i]
		k := key(*s)
		if locs := byKey[k]; len(locs) > 0 {
			s.Locations["go_profile"] = locs[0]
			byKey[k] = locs[1:]
		}
	}
	files["hir.go"] = "//go:build !profile_sites\n\n" + files["hir.go"]
	files["hir_profile.go"] = "//go:build profile_sites\n\n" + prof["hir.go"]
	files["site_profile.go"] = pe.profileSource()
	files["site_profile_off.go"] = profileOffSource
	return files, sites, nil
}

type sourceToken struct {
	offset, line int
	token        token.Token
	literal      string
}

func sourceTokens(source string) []sourceToken {
	fs := token.NewFileSet()
	f := fs.AddFile("hir.go", -1, len(source))
	var scan scanner.Scanner
	scan.Init(f, []byte(source), nil, 0)
	var out []sourceToken
	for {
		p, t, l := scan.Scan()
		if t == token.EOF {
			break
		}
		if t == token.SEMICOLON {
			continue
		}
		out = append(out, sourceToken{f.Offset(p), fs.Position(p).Line, t, l})
	}
	return out
}
func (e *emitter) addLocations(m *hir.SiteMap, formatted, backend, file string, lineOffset int) error {
	raw := sourceTokens(e.code.String() + e.extra.String())
	pretty := sourceTokens(formatted)
	if len(raw) != len(pretty) {
		return fmt.Errorf("site map: Go formatting changed token count")
	}
	for i := range raw {
		if raw[i].token != pretty[i].token || raw[i].literal != pretty[i].literal {
			return fmt.Errorf("site map: Go formatting changed token %d", i)
		}
	}
	for _, s := range e.sites {
		i := sort.Search(len(raw), func(i int) bool { return raw[i].offset >= s.offset })
		if i == len(raw) {
			return fmt.Errorf("site map: no emitted token for %s", s.node.SiteID)
		}
		m.Add(s.node, s.kind, backend, hir.SiteLocation{File: file, Line: pretty[i].line + lineOffset})
	}
	return nil
}

// An invocation-local counter and deferred commit include zero trips and exits
// through return, break or panic. Existing HIR return-boundary machinery
// propagates returns through the profiling closure to the enclosing method.
func (b *body) profileLoop(s *hir.Stmt) (func(), string) {
	if !b.e.profile {
		return func() {}, ""
	}
	trips := b.fresh()
	b.line("{ %s:=uint64(0);func(){defer func(){siteLoop(%d,%s)}()", trips, b.e.profileIndex(s.Node, "loop"), trips)
	b.tryDepth++
	return func() { b.tryDepth--; b.line("}() }") }, trips
}

func (e *emitter) profileSource() string {
	var table strings.Builder
	table.WriteString("var siteDefinitions=[]struct{ID,Kind string}{\n")
	for _, n := range e.profileNodes {
		fmt.Fprintf(&table, "{%q,%q},\n", n.SiteID, n.Kind)
	}
	table.WriteString("}\nvar siteClasses=map[string]string{\n")
	for _, c := range e.p.Classes {
		fmt.Fprintf(&table, "reflect.TypeOf((*%s)(nil)).String():%s,\n", e.obj(c.Name), strconv.Quote(c.SiteOwner))
	}
	table.WriteString("}\n")
	return profileRuntimeSource + table.String()
}
