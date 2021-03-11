package pathexistence

import (
	"testing"
)

func TestDirWithoutDot(t *testing.T) {
	testCases := []struct {
		actual   string
		expected string
	}{
		{dirWithoutDot("foo.txt"), ""},
		{dirWithoutDot("foo/bar.txt"), "foo"},
		{dirWithoutDot("foo/baz"), "foo"},
	}

	for _, testCase := range testCases {
		if testCase.actual != testCase.expected {
			t.Errorf("unexpected dirname: want=%s got=%s", testCase.expected, testCase.actual)
		}
	}
}
