package abap

import (
	"fmt"
	"os"
	"sort"

	"github.com/oisee/abapiti/hir"
)

// inline runs the HIR inlining pass (ABAPITI_INLINE=0 disables it) and
// verifies its result. ABAPITI_INLINE_STATS=1 prints the inlined callees.
func inline(p *hir.Program) error {
	if os.Getenv("ABAPITI_INLINE") == "0" {
		return nil
	}
	n, stats := hir.InlineStats(p)
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
