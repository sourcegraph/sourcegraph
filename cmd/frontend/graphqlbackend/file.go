package graphqlbackend

import (
	"context"
	"strings"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/externallink"
	"github.com/sourcegraph/sourcegraph/internal/highlight"
	"github.com/sourcegraph/sourcegraph/internal/ipynb"
	"github.com/sourcegraph/sourcegraph/internal/markdown"
)

type FileResolver interface {
	Path() string
	Name() string
	IsDirectory() bool
	Content(ctx context.Context, args *GitTreeContentPageArgs) (string, error)
	ByteSize(ctx context.Context) (int32, error)
	TotalLines(ctx context.Context) (int32, error)
	Binary(ctx context.Context) (bool, error)
	RichHTML(ctx context.Context, args *GitTreeContentPageArgs) (string, error)
	URL(ctx context.Context) (string, error)
	CanonicalURL() string
	ChangelistURL(ctx context.Context) (*string, error)
	ExternalURLs(ctx context.Context) ([]*externallink.Resolver, error)
	Highlight(ctx context.Context, args *HighlightArgs) (*HighlightedFileResolver, error)
	Languages(ctx context.Context) ([]string, error)

	ToGitBlob() (*GitTreeEntryResolver, bool)
	ToVirtualFile() (*VirtualFileResolver, bool)
	ToBatchSpecWorkspaceFile() (BatchWorkspaceFileResolver, bool)
}

func richHTML(content, ext string) (string, error) {
	switch strings.ToLower(ext) {
	case ".md", ".mdown", ".markdown", ".markdn":
		return markdown.Render(content)
	case ".ipynb":
		return ipynb.Render(content)
	}
	return "", nil
}

type markdownOptions struct {
	AlwaysNil *string
}

func (*schemaResolver) RenderMarkdown(args *struct {
	Markdown string
	Options  *markdownOptions
}) (string, error) {
	return markdown.Render(args.Markdown)
}

func (*schemaResolver) HighlightCode(ctx context.Context, args *struct {
	Code           string
	FuzzyLanguage  string
	DisableTimeout bool
	IsLightTheme   *bool
}) (string, error) {
	language := highlight.SyntectLanguageMap[strings.ToLower(args.FuzzyLanguage)]
	filePath := "file." + language
	response, _, err := highlight.Code(ctx, highlight.Params{
		Content:        []byte(args.Code),
		Filepath:       filePath,
		DisableTimeout: args.DisableTimeout,
	})
	if err != nil {
		return args.Code, err
	}

	html, err := response.HTML()
	if err != nil {
		return "", err
	}

	return string(html), err
}
