package context

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/scip/bindings/go/scip"
)

var testContents = `
package example

import "strings"

var prefixes = []string{
	"foo:",
	"bar:",
	"baz:",
}

func sanitize(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i, line := range lines {
		for _, prefix := range prefixes {
			lines[i] = strings.TrimPrefix(line, prefix)
		}
	}

	return strings.Join(lines, "\n")
}

// Some other random utility function
func random() int { return 4 }
`

const testExpectedDefinition = `
func sanitize(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i, line := range lines {
		for _, prefix := range prefixes {
			lines[i] = strings.TrimPrefix(line, prefix)
		}
	}

	return strings.Join(lines, "\n")
}
`

func TestExtractSnippet(t *testing.T) {
	testCases := []struct {
		Name     string
		Content  string
		Range    []int32
		Expected string
	}{
		{
			Name:     "singleton full",
			Content:  "just a single line",
			Range:    []int32{0, 0, 0, 18},
			Expected: "just a single line",
		},
		{
			Name:     "singleton partial",
			Content:  "just a single line",
			Range:    []int32{0, 7, 0, 13},
			Expected: "single",
		},
		{
			Name:     "single line full",
			Content:  "first\nsecond\nthird\nfourth\nfifth\nsixth\n",
			Range:    []int32{3, 0, 3, 6},
			Expected: "fourth",
		},
		{
			Name:     "single line partial",
			Content:  "first\nsecond\nthird\nfourth\nfifth\nsixth\n",
			Range:    []int32{1, 1, 1, 5},
			Expected: "econ",
		},
		{
			Name:     "multi-line full",
			Content:  "first\nsecond\nthird\nfourth\nfifth\nsixth\n",
			Range:    []int32{1, 0, 3, 6},
			Expected: "second\nthird\nfourth",
		},
		{
			Name:     "multi-line partial",
			Content:  "first\nsecond\nthird\nfourth\nfifth\nsixth\n",
			Range:    []int32{1, 2, 4, 4},
			Expected: "cond\nthird\nfourth\nfift",
		},
		{
			Name:     "defensive end offset",
			Content:  testContents[1:],           // trim leading newline
			Range:    []int32{10, 0, 20, 3},      // end char exceeds end line length
			Expected: testExpectedDefinition[1:], // trim leading newline
		},
		{
			Name:     "defensive start offset",
			Content:  testContents[1:],           // trim leading newline
			Range:    []int32{9, 100, 20, 3},     // start char exceeds start line length
			Expected: testExpectedDefinition[1:], // trim leading newline
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			if diff := cmp.Diff(testCase.Expected, NewDocumentAndText(testCase.Content, nil).Extract(scip.NewRange(testCase.Range))); diff != "" {
				t.Errorf("unexpected snippet (-want +got):\n%s", diff)
			}
		})
	}
}
