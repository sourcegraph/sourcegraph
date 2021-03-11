package graphqlbackend

import (
	"context"
	"html/template"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/highlight"
)

func TestVirtualFile(t *testing.T) {
	fileContent := "# this is content"
	fileName := "dir/awesome_file.md"
	vfr := NewVirtualFileResolver(
		CreateFileInfo(fileName, false),
		func(ctx context.Context) (string, error) {
			return fileContent, nil
		},
	)
	t.Run("Path", func(t *testing.T) {
		if have, want := vfr.Path(), fileName; have != want {
			t.Fatalf("wrong path, want=%q have=%q", want, have)
		}
	})
	t.Run("Name", func(t *testing.T) {
		if have, want := vfr.Name(), path.Base(fileName); have != want {
			t.Fatalf("wrong name, want=%q have=%q", want, have)
		}
	})
	t.Run("IsDirectory", func(t *testing.T) {
		if have, want := vfr.IsDirectory(), false; have != want {
			t.Fatalf("wrong IsDirectory, want=%t have=%t", want, have)
		}
	})
	t.Run("Content", func(t *testing.T) {
		have, err := vfr.Content(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if want := fileContent; have != want {
			t.Fatalf("wrong Content, want=%q have=%q", want, have)
		}
	})
	t.Run("ByteSize", func(t *testing.T) {
		have, err := vfr.ByteSize(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if want := int32(len([]byte(fileContent))); have != want {
			t.Fatalf("wrong ByteSize, want=%q have=%q", want, have)
		}
	})
	t.Run("RichHTML", func(t *testing.T) {
		have, err := vfr.RichHTML(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		renderedMarkdown := `<h1><a name="this-is-content" class="anchor" href="#this-is-content" rel="nofollow" aria-hidden="true"><span></span></a>this is content</h1>
`
		if diff := cmp.Diff(have, renderedMarkdown); diff != "" {
			t.Fatalf("wrong RichHTML: %s", diff)
		}
	})
	t.Run("Binary", func(t *testing.T) {
		isBinary, err := vfr.Binary(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if isBinary {
			t.Fatalf("wrong Binary: %t", isBinary)
		}
	})
	t.Run("Highlight", func(t *testing.T) {
		testHighlight := func(aborted bool) {
			highlightedContent := template.HTML("highlight of the file")
			highlight.Mocks.Code = func(p highlight.Params) (template.HTML, bool, error) {
				return highlightedContent, aborted, nil
			}
			t.Cleanup(highlight.ResetMocks)
			highlightedFile, err := vfr.Highlight(context.Background(), &HighlightArgs{})
			if err != nil {
				t.Fatal(err)
			}
			if highlightedFile.Aborted() != aborted {
				t.Fatalf("wrong Aborted. want=%t have=%t", aborted, highlightedFile.Aborted())
			}
			if highlightedFile.HTML() != string(highlightedContent) {
				t.Fatalf("wrong HTML. want=%q have=%q", highlightedContent, highlightedFile.HTML())
			}
		}
		testHighlight(false)
		testHighlight(true)
	})
}
