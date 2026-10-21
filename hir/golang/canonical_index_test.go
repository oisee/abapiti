package golang

import (
	"strings"
	"testing"
)

// The canonical-index digit precheck must keep the exact accept set of the
// ParseUint + FormatUint round-trips it replaces. The reference closures below
// are the original site decisions; the table pins both of them, the seeded
// fuzz needs old and new to agree, and the allocation probe needs one million
// non-numeric lookups to allocate nothing. Everything runs on the emitted
// runtime, so jsonIndex exercises the rewritten config-JSON site itself.
func TestCanonicalIndexPrecheck(t *testing.T) {
	main := `
	old31 := func(s string) (uint64, bool) {n, err := strconv.ParseUint(s, 10, 31); return n, err == nil && s != "" && strconv.FormatUint(n, 10) == s}
	old32 := func(s string) (uint64, bool) {n, err := strconv.ParseUint(s, 10, 32); return n, err == nil && n < 4294967295 && strconv.FormatUint(n, 10) == s}
	new31 := func(s string) (uint64, bool) {return parseCanonicalIndex(s, 31)}
	new32 := func(s string) (uint64, bool) {n, ok := parseCanonicalIndex(s, 32); return n, ok && n < 4294967295}
	site32 := func(s string) (uint64, bool) {return jsonIndex(str(s))}
	check := func(s string, old, new func(string) (uint64, bool), label string) {
		on, ook := old(s)
		nn, nok := new(s)
		if ook != nok || ook && on != nn {
			panic("index decision changed for " + strconv.Quote(s) + " (" + label + "): old " + fmt.Sprint(on, ook) + " new " + fmt.Sprint(nn, nok))
		}
	}
	type tc struct {
		s          string
		want31     bool
		want32     bool
	}
	cases := []tc{
		{"", false, false},
		{"0", true, true},
		{"00", false, false},
		{"01", false, false},
		{"1", true, true},
		{"4294967294", false, true},
		{"4294967295", false, false},
		{"4294967296", false, false},
		{"2147483647", true, true},
		{"2147483648", false, true},
		{"-1", false, false},
		{"+1", false, false},
		{" 1", false, false},
		{"1 ", false, false},
		{"1e3", false, false},
		{"0x1", false, false},
		{"length", false, false},
		{"١", false, false},
		{"111111111111111111111111111111", false, false},
	}
	for _, c := range cases {
		if _, ok := old31(c.s); ok != c.want31 {
			panic("31-bit reference disagrees for " + strconv.Quote(c.s))
		}
		if _, ok := old32(c.s); ok != c.want32 {
			panic("32-bit reference disagrees for " + strconv.Quote(c.s))
		}
		check(c.s, old31, new31, "31-bit")
		check(c.s, old32, new32, "32-bit")
		check(c.s, old32, site32, "jsonIndex")
	}
	state := uint64(0x9e3779b97f4a7c15)
	next := func() uint64 {state = state*6364136223846793005 + 1442695040888963407; return state}
	alpha := "0123456789-+ ex"
	buf := make([]byte, 12)
	for iter := 0; iter < 100000; iter++ {
		n := int(next() % 13)
		for i := 0; i < n; i++ {
			buf[i] = alpha[next()%14]
		}
		s := string(buf[:n])
		check(s, old31, new31, "31-bit")
		check(s, old32, new32, "32-bit")
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	keys := []string{"length", "name", "constructor", "push"}
	for i := 0; i < 1000000; i++ {
		if _, ok := parseCanonicalIndex(keys[i&3], 31); ok {
			panic("non-numeric key accepted")
		}
	}
	runtime.ReadMemStats(&after)
	// Mallocs is process-wide: background runtime work may add a few, while an
	// allocating precheck would add at least one per lookup (1,000,000).
	if after.Mallocs-before.Mallocs > 1000 {
		panic("non-numeric index precheck allocated")
	}
	fmt.Println("ok")
`
	if got := executeImports(t, configJSONFixture(), []string{"fmt", "runtime", "strconv"}, main); got != "ok\n" {
		t.Fatal(got)
	}
}

func TestCanonicalIndexSites(t *testing.T) {
	if !strings.Contains(runtimeSource, "func parseCanonicalIndex(s string, bits int) (uint64, bool) {") {
		t.Error("parseCanonicalIndex definition missing from the emitted runtime")
	}
	sources := []struct {
		name string
		src  string
		want int
	}{
		{"runtimeSource", runtimeSource, 2}, // definition + dynamic get on tagArray
		{"liveArraySource", liveArraySource, 2},
		{"ownKeysSource", ownKeysSource, 1},
		{"configJSONRuntimeSource", configJSONRuntimeSource, 1},
	}
	for _, s := range sources {
		if got := strings.Count(s.src, "parseCanonicalIndex("); got != s.want {
			t.Errorf("%s: %d parseCanonicalIndex references, want %d", s.name, got, s.want)
		}
	}
}
