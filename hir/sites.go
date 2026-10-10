package hir

import (
	"fmt"
	"net/url"
)

// AssignSiteIDs stamps unstamped nodes in deterministic pre-order. Counters
// are local to an owning qualified method, source and kind; serial IDs are
// deliberately excluded. Call before transformations that copy nodes.
func AssignSiteIDs(p *Program) {
	stampOwner := func(owner string, visit func(func(*Node, string))) {
		counts := map[string]int{}
		visit(func(n *Node, kind string) {
			key := n.Source + "\x00" + kind
			ordinal := counts[key]
			counts[key]++
			if n.SiteID != "" {
				return
			}
			n.SiteOwner, n.SiteSource = owner, n.Source
			n.SiteID = fmt.Sprintf("%s|%s|%s|%d", url.QueryEscape(owner), url.QueryEscape(n.Source), kind, ordinal)
		})
	}
	method := func(owner string, m *Method) {
		if m == nil {
			return
		}
		stampOwner(owner+"."+m.Name, func(stamp func(*Node, string)) {
			stamp(&m.Node, "method")
			walk(m.Body, func(s *Stmt) { stamp(&s.Node, string(s.Kind)) }, func(x *Expr) { stamp(&x.Node, string(x.Kind)) })
		})
	}
	for _, c := range p.Classes {
		stampOwner(c.Name, func(stamp func(*Node, string)) {
			stamp(&c.Node, "class")
			for i := range c.Fields {
				stamp(&c.Fields[i].Node, "field_decl")
			}
		})
		method(c.Name, c.Ctor)
		for _, m := range c.Methods {
			method(c.Name, m)
		}
	}
	for _, c := range p.Interfaces {
		stampOwner(c.Name, func(stamp func(*Node, string)) { stamp(&c.Node, "interface") })
		for _, m := range c.Methods {
			method(c.Name, m)
		}
	}
}

// InlineNode preserves legacy diagnostics while carrying the original callee
// identity and a fresh caller-to-callee call-site chain. legacy is the node
// the inliner would have emitted before site metadata was introduced.
func InlineNode(original, call, legacy Node) Node {
	legacy.SiteID, legacy.SiteSource, legacy.SiteOwner = original.SiteID, original.SiteSource, original.SiteOwner
	legacy.InlinePath = append([]string{}, call.InlinePath...)
	if call.SiteID != "" {
		legacy.InlinePath = append(legacy.InlinePath, call.SiteID)
	}
	legacy.InlinePath = append(legacy.InlinePath, original.InlinePath...)
	return legacy
}

// SiteLocation is extensible to backend-specific columns. ABAP emitters can
// attach class/method/line through SiteMap.Add without changing site identity.
type SiteLocation struct {
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Class  string `json:"class,omitempty"`
	Method string `json:"method,omitempty"`
}
type Site struct {
	SiteID     string                  `json:"site_id"`
	Kind       string                  `json:"kind"`
	Source     string                  `json:"source"`
	Method     string                  `json:"method"`
	InlinePath []string                `json:"inline_path"`
	Locations  map[string]SiteLocation `json:"locations"`
}
type SiteMap struct {
	Schema string `json:"schema"`
	Sites  []Site `json:"sites"`
}

func NewSiteMap() *SiteMap { return &SiteMap{Schema: "sites/1", Sites: []Site{}} }

// Add records an emitted occurrence. Copies share SiteID, and InlinePath
// distinguishes their calling contexts; repeated emission may have locations.
func (m *SiteMap) Add(n Node, kind, backend string, location SiteLocation) {
	path := append([]string{}, n.InlinePath...)
	m.Sites = append(m.Sites, Site{n.SiteID, kind, n.SiteSource, n.SiteOwner, path, map[string]SiteLocation{backend: location}})
}
func SiteKind(k ExprKind) string {
	switch k {
	case DirectCall, SuperCall:
		return "call"
	case VirtualCall:
		return "virtual_call"
	case New:
		return "new"
	case RuntimeOp:
		return "runtime_op"
	}
	return ""
}
