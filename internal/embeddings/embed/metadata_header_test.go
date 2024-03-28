package embed

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddMetadataHeader(t *testing.T) {
	tests := []struct {
		code         string
		fileName     string
		expectedCode string
	}{
		{
			code:         "a line of code",
			fileName:     "some/file.go",
			expectedCode: "// github.com/sourcegraph/sourcegraph some/file.go\na line of code",
		},
		{
			code:         "a line of html",
			fileName:     "some/file.html",
			expectedCode: "<!-- github.com/sourcegraph/sourcegraph some/file.html -->\na line of html",
		},
		{
			code:         "a line of text",
			fileName:     "some/extensionless/file",
			expectedCode: "// github.com/sourcegraph/sourcegraph some/extensionless/file\na line of text",
		},
		{
			code:         "a line of text",
			fileName:     "some/text/file.txt",
			expectedCode: "github.com/sourcegraph/sourcegraph some/text/file.txt\na line of text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			codeWithHeader := addMetadataHeader(tt.code, "github.com/sourcegraph/sourcegraph", tt.fileName)
			require.Equal(t, tt.expectedCode, codeWithHeader)
		})
	}
}
