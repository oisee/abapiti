package gracecheck

import (
	"github.com/oisee/abapiti/hir/rewrite"
	"testing"
)

// CheckRecursiveSeeds uses per-root reachability DFS, independently of the
// extractor's SCC algorithm and hierarchy adapter. Calls come from Grace's
// separately checked receiver and dispatch rules, including ensure_init edges.
func CheckRecursiveSeeds(t *testing.T, db *rewrite.DB) {
	t.Helper()
	graph := map[string][]string{}
	nodes := map[string]bool{}
	for _, r := range db.Facts("calls") {
		graph[r[0]] = append(graph[r[0]], r[1])
		nodes[r[0]] = true
		nodes[r[1]] = true
	}
	for _, r := range db.Facts("recursive_call") {
		nodes[r[0]] = true
	}
	for root := range nodes {
		seen := map[string]bool{}
		pending := append([]string{}, graph[root]...)
		cyclic := false
		for len(pending) > 0 {
			v := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if v == root {
				cyclic = true
				break
			}
			if seen[v] {
				continue
			}
			seen[v] = true
			pending = append(pending, graph[v]...)
		}
		if db.Has("recursive_call", root) != cyclic {
			t.Fatalf("recursive_call(%s) differs from independent resolved-call DFS: want %t", root, cyclic)
		}
	}
}
