package casetransform

import (
	"testing"

	"regexp/syntax" //nolint:depguard // using the grafana fork of regexp clashes with zoekt, which uses the std regexp/syntax.
)

func TestLowerRegexpASCII(t *testing.T) {
	// The expected values are a bit volatile, since they come from
	// syntex.Regexp.String. So they may change between go versions. Just
	// ensure they make sense.
	cases := map[string]string{
		"foo":       "foo",
		"FoO":       "foo",
		"(?m:^foo)": "(?m:^foo)",
		"(?m:^FoO)": "(?m:^foo)",

		// Ranges for the characters can be tricky. So we include many
		// cases. Importantly user intention when they write [^A-Z] is would
		// expect [^a-z] to apply when ignoring case.
		"[A-Z]":  "[a-z]",
		"[^A-Z]": "[^A-Za-z]",
		"[A-M]":  "[a-m]",
		"[^A-M]": "[^A-Ma-m]",
		"[A]":    "a",
		"[^A]":   "[^Aa]",
		"[M]":    "m",
		"[^M]":   "[^Mm]",
		"[Z]":    "z",
		"[^Z]":   "[^Zz]",
		"[a-z]":  "[a-z]",
		"[^a-z]": "[^a-z]",
		"[a-m]":  "[a-m]",
		"[^a-m]": "[^a-m]",
		"[a]":    "a",
		"[^a]":   "[^a]",
		"[m]":    "m",
		"[^m]":   "[^m]",
		"[z]":    "z",
		"[^z]":   "[^z]",

		// @ is tricky since it is 1 value less than A
		"[^A-Z@]": "[^@-Za-z]",

		// full unicode range should just be a .
		"[\\x00-\\x{10ffff}]": "(?s:.)",

		"[abB-Z]":       "[b-zab]",
		"([abB-Z]|FoO)": "([b-zab]|foo)",
		`[@-\[]`:        `[@-\[a-z]`,    // original range includes A-Z but excludes a-z
		`\S`:            `[^\t\n\f\r ]`, // \S is shorthand for the expected
	}

	for expr, want := range cases {
		re, err := syntax.Parse(expr, syntax.Perl)
		if err != nil {
			t.Fatal(expr, err)
		}
		LowerRegexpASCII(re)
		got := re.String()
		if want != got {
			t.Errorf("LowerRegexpASCII(%q) == %q != %q", expr, got, want)
		}
	}
}
