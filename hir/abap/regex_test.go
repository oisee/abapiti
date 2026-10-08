package abap

import (
	"regexp"
	"testing"
)

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
