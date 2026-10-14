package abap

import (
	"regexp"
	"strings"
	"testing"
)

func TestRegexJSClasses(t *testing.T) {
	for _, flags := range []string{"", "i", "s", "si"} {
		for _, tt := range []struct {
			pattern, input string
			want           bool
		}{
			{`a.b`, "a\nb", strings.Contains(flags, "s")},
			{`a.b`, "a\rb", strings.Contains(flags, "s")},
			{`a.b`, "a\u2028b", strings.Contains(flags, "s")},
			{`a.b`, "a\u2029b", strings.Contains(flags, "s")},
			{`a.b`, "axb", true}, {`a\.b`, "a.b", true}, {`[.]`, "\n", false},
			{`^\s*$`, "\u3000\ufeff\r \u00a0", true}, {`^\s*$`, "x", false},
			{`\S`, "\u00a0", false}, {`\S`, "x", true},
			{`^[\s]+$`, "\u3000\r", true}, {`^[^\s]+$`, "x", true},
			{`^\s$`, "\u180e", false}, {`^\s$`, "\u0085", false},
		} {
			p, _ := regexPattern(tt.pattern, flags)
			if strings.Contains(flags, "s") {
				p = "(?s)" + p
			}
			if strings.Contains(flags, "i") {
				p = "(?i)" + p
			}
			re, err := regexp.Compile(p)
			if err != nil {
				t.Fatal(err)
			}
			if got := re.MatchString(tt.input); got != tt.want {
				t.Errorf("/%s/%s on %q: %v, want %v", tt.pattern, flags, tt.input, got, tt.want)
			}
			// Static expansion is fed through the runtime scanner again.
			if again := regexClasses(p, flags); again != p {
				t.Errorf("non-idempotent translation: %q -> %q", p, again)
			}
		}
	}
	for _, r := range "\t\n\v\f\r \u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeff" {
		if !regexp.MustCompile(regexClasses(`^\s$`)).MatchString(string(r)) {
			t.Errorf("missing whitespace U+%04X", r)
		}
	}
}

func TestRegexCharacterClassesAndBoundedExclusions(t *testing.T) {
	tests := []struct {
		pattern string
		ignore  bool
		inputs  map[string]bool
	}{
		{`^[\w\d_%$\*\~]+$`, false, map[string]bool{"foo": true, "A_0": true, "*": true, "a~b": true, "@": false, "[": false}},
		{`^(\/\w+\/)?(?!\*)[\w\d_\*\~%]+$`, false, map[string]bool{"foo": true, "A*": true, "*": false, "/ABC/F*": true, "/ABC/*": false, "~": true}},
		{`^(?!(?:FROM|INTO|WHERE)$)(\/\w+\/)?(\*?\w+~(\/\w+\/)?(\w+|\*)|\w+)$`, true, map[string]bool{"from": false, "WHERE": false, "FROMX": true, "a~*": true, "/ABC/from": true, "foo": true}},
	}
	for _, tt := range tests {
		main, reject := regexPattern(tt.pattern)
		if tt.ignore {
			main = "(?i)" + main
			if reject != "" {
				reject = "(?i)" + reject
			}
		}
		re, err := regexp.Compile(main)
		if err != nil {
			t.Fatal(err)
		}
		var excluded *regexp.Regexp
		if reject != "" {
			excluded, err = regexp.Compile(reject)
			if err != nil {
				t.Fatal(err)
			}
		}
		for input, want := range tt.inputs {
			got := re.MatchString(input) && (excluded == nil || !excluded.MatchString(input))
			if got != want {
				t.Errorf("%s input %q: %v, want %v", tt.pattern, input, got, want)
			}
		}
	}
}
