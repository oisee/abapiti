package rewrite

import (
	"fmt"
	"sort"
	"strings"
)

// Report prints sorted relation counts, method sets, every virtual site's
// receiver classes, and unique direct static (method,class,field) writes.
func Report(d *DB) string {
	var b strings.Builder
	fmt.Fprintln(&b, "fact counts")
	predicates := map[string]bool{}
	for _, p := range strings.Fields("class extends implements method static final calls virtual_call new reads_field writes_field reads_static writes_static throws trap try local assign returns param runtime_op receivers may_throw pure escapes writes_static_transitive memo_store lazy_init counter_store") {
		predicates[p] = true
	}
	for _, p := range d.Predicates() {
		predicates[p] = true
	}
	var ordered []string
	for p := range predicates {
		ordered = append(ordered, p)
	}
	sort.Strings(ordered)
	for _, p := range ordered {
		fmt.Fprintf(&b, "  %s %d\n", p, d.Count(p))
	}
	for _, p := range []string{"may_throw", "pure"} {
		fmt.Fprintln(&b, p)
		for _, r := range d.Facts(p) {
			fmt.Fprintf(&b, "  %s\n", r[0])
		}
	}
	fmt.Fprintln(&b, "receivers")
	sites := map[string][]string{}
	for _, r := range d.Facts("virtual_call") {
		sites[r[3]] = nil
	}
	for _, r := range d.Facts("receivers") {
		sites[r[0]] = append(sites[r[0]], r[1])
	}
	names := []string{}
	one := 0
	for s, cs := range sites {
		names = append(names, s)
		if len(cs) == 1 {
			one++
		}
	}
	sort.Strings(names)
	for _, s := range names {
		fmt.Fprintf(&b, "  %s: %s\n", s, strings.Join(sites[s], ", "))
	}
	fmt.Fprintln(&b, "static writes (unique method/class/field)")
	counts := map[string][3]int{}
	total := [3]int{}
	for _, r := range d.Facts("writes_static") {
		kind, idx := "other", 2
		if d.Has("memo_store", r...) {
			kind, idx = "memo", 0
		} else if d.Has("counter_store", r...) {
			kind, idx = "counter", 1
		}
		n := counts[r[1]]
		n[idx]++
		counts[r[1]] = n
		total[idx]++
		fmt.Fprintf(&b, "  %s %s.%s %s\n", r[0], r[1], r[2], kind)
	}
	fmt.Fprintln(&b, "summary")
	fmt.Fprintf(&b, "  methods pure=%d may_throw=%d defined=%d\n", d.Count("pure"), d.Count("may_throw"), d.Count("defined"))
	fmt.Fprintf(&b, "  virtual sites single=%d total=%d\n", one, len(sites))
	fmt.Fprintf(&b, "  static writes memo=%d counter=%d other=%d\n", total[0], total[1], total[2])
	names = nil
	for c := range counts {
		names = append(names, c)
	}
	sort.Strings(names)
	for _, c := range names {
		n := counts[c]
		fmt.Fprintf(&b, "  %s memo=%d counter=%d other=%d\n", c, n[0], n[1], n[2])
	}
	return b.String()
}
