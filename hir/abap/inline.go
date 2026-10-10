package abap

import (
	"fmt"
	"os"
	"sort"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
)

// inline runs the HIR inlining pass and verifies its result. Grace's
// inlining rule is the default; ABAPITI_INLINE=classic runs hir.Inline, the
// oracle Grace is checked against (same output), and ABAPITI_INLINE=0
// disables the pass. ABAPITI_INLINE_STATS=1 prints the inlined callees.
func inline(p *hir.Program) error {
	// ABAPITI_SINGLETON=1: recv.m([e]) -> recv.m_one(e) before inlining
	// (experimental; see hir/singleton.go).
	if os.Getenv("ABAPITI_SINGLETON") == "1" {
		st := hir.Singleton(p)
		if os.Getenv("ABAPITI_INLINE_STATS") != "" {
			fmt.Fprintf(os.Stderr, "singleton: %d methods %v, %d clones, %d forwarding defaults, %d call sites\n", st.Methods, st.Names, st.Clones, st.Defaults, st.CallSites)
		}
		if errors := hir.Verify(p); len(errors) > 0 {
			return fmt.Errorf("singleton specialisation produced invalid HIR: %w", errors[0])
		}
	}
	var n int
	var stats map[string]int
	switch mode := os.Getenv("ABAPITI_INLINE"); mode {
	case "0":
		return nil
	case "classic":
		n, stats = hir.InlineStats(p)
	case "", "1", "grace":
		s, err := rewrite.Inline(p)
		if err != nil {
			return fmt.Errorf("Grace HIR inlining (ABAPITI_INLINE=classic runs hir.Inline): %w", err)
		}
		n, stats = s.CallSites, s.Callees
	default:
		return fmt.Errorf("ABAPITI_INLINE=%q: use grace (default), classic or 0", mode)
	}
	if os.Getenv("ABAPITI_INLINE_STATS") != "" {
		keys := make([]string, 0, len(stats))
		for k := range stats {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if stats[keys[i]] != stats[keys[j]] {
				return stats[keys[i]] > stats[keys[j]]
			}
			return keys[i] < keys[j]
		})
		fmt.Fprintf(os.Stderr, "inline: %d call sites, %d callees\n", n, len(keys))
		for _, k := range keys {
			fmt.Fprintf(os.Stderr, "inline: %6d %s\n", stats[k], k)
		}
	}
	if n == 0 {
		return nil
	}
	if errors := hir.Verify(p); len(errors) > 0 {
		return fmt.Errorf("HIR inlining produced invalid HIR (ABAPITI_INLINE=0 disables the pass): %w", errors[0])
	}
	return nil
}
