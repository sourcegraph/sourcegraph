package routevar

import "testing"

// pairs converts map's keys and values to a slice of []string{key1,
// value1, key2, value2, ...}.
func pairs(m map[string]string) []string {
	pairs := make([]string, 0, len(m)*2)
	for k, v := range m {
		pairs = append(pairs, k, v)
	}
	return pairs
}

func TestNamedToNonCapturingGroups(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{``, ``},
		{`(?P<foo>bar)`, `(?:bar)`},
		{`(?P<foo>(?P<baz>bar))`, `(?:(?:bar))`},
		{`(?P<foo>qux(?P<baz>bar))`, `(?:qux(?:bar))`},
	}
	for _, test := range tests {
		got := namedToNonCapturingGroups(test.input)
		if got != test.want {
			t.Errorf("%q: got %q, want %q", test.input, got, test.want)
		}
	}
}
