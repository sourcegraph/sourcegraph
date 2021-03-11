package graphqlbackend

import (
	"context"
	"strings"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/externallink"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/highlight"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/markdown"
)

type FileResolver interface {
	Path() string
	Name() string
	IsDirectory() bool
	Content(ctx context.Context) (string, error)
	ByteSize(ctx context.Context) (int32, error)
	Binary(ctx context.Context) (bool, error)
	RichHTML(ctx context.Context) (string, error)
	URL(ctx context.Context) (string, error)
	CanonicalURL() (string, error)
	ExternalURLs(ctx context.Context) ([]*externallink.Resolver, error)
	Highlight(ctx context.Context, args *HighlightArgs) (*highlightedFileResolver, error)

	ToGitBlob() (*GitTreeEntryResolver, bool)
	ToVirtualFile() (*virtualFileResolver, bool)
}

func richHTML(content, ext string) (string, error) {
	switch ext {
	case ".md", ".mdown", ".markdown", ".markdn":
		break
	default:
		return "", nil
	}
	return markdown.Render(content), nil
}

type markdownOptions struct {
	AlwaysNil *string
}

func (*schemaResolver) RenderMarkdown(args *struct {
	Markdown string
	Options  *markdownOptions
}) string {
	return markdown.Render(args.Markdown)
}

func (*schemaResolver) HighlightCode(ctx context.Context, args *struct {
	Code           string
	FuzzyLanguage  string
	DisableTimeout bool
	IsLightTheme   bool
}) (string, error) {
	language := highlight.SyntectLanguageMap[strings.ToLower(args.FuzzyLanguage)]
	filePath := "file." + language
	html, _, err := highlight.Code(ctx, highlight.Params{
		Content:        []byte(args.Code),
		Filepath:       filePath,
		DisableTimeout: args.DisableTimeout,
		IsLightTheme:   args.IsLightTheme,
	})
	if err != nil {
		return args.Code, err
	}
	return string(html), nil
}
