package main

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/hirclone"
	"os"
	"runtime"
	"time"
)

// measureInline times the existing production pass on independent full-closure
// copies. Lowering/loading, cloning and GC are outside the pass interval.
func measureInline(p *hir.Program) error {
	for i := 0; i < 3; i++ {
		q := hirclone.Clone(p)
		runtime.GC()
		start := time.Now()
		stats, err := rewrite.Inline(q)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "inline trial=%d seconds=%.9f sites=%d callees=%d\n", i+1, time.Since(start).Seconds(), stats.CallSites, len(stats.Callees))
	}
	return nil
}
