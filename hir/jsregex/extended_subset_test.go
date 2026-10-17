package jsregex

import "testing"

func TestExtendedSubsetTranslations(t *testing.T) {
	for _, c := range [][3]string{
		{`\n`, "g", `\n`},
		{`^'.*'$`, "s", `^'(?s:.)*'\z`},
		{`a|^b`, "", `a|\Ab`},
		{`(?:x|^y)[\t;]`, "", `(?:x|\Ay)[\t;]`},
		{`^W\s+'(.*)'$`, "is", `^[Ww]` + execSpace + `+'((?s:.)*)'\z`},
	} {
		got, ok := TranslateExtendedSubset(c[0], c[1])
		if !ok || got != c[2] {
			t.Errorf("/%s/%s: got %q, %v; want %q", c[0], c[1], got, ok, c[2])
		}
	}
}

func TestExtendedSubsetRefusals(t *testing.T) {
	for _, c := range [][2]string{
		{`a|^*b`, ""}, {`a|^b`, "y"}, {`a`, "ss"}, {`[\t-\r]`, ""}, {`[ -\n]`, ""}, {`\(^a`, ""}, {`[|^a]^`, ""}, {`\0`, ""}, {`a`, "m"},
	} {
		if got, ok := TranslateExtendedSubset(c[0], c[1]); ok {
			t.Errorf("accepted /%s/%s as %q", c[0], c[1], got)
		}
	}
}
