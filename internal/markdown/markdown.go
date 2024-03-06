package markdown

import (
	"bytes"
	"fmt"
	"sync"

	jupyter "github.com/bevzzz/nb/extension/extra/goldmark-jupyter"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/sourcegraph/sourcegraph/internal/htmlutil"
)

// Render renders Markdown content into sanitized HTML that is safe to render anywhere.
func Render(content string) (string, error) {
	var buf bytes.Buffer
	if err := Goldmark().Convert([]byte(content), &buf); err != nil {
		return "", fmt.Errorf("markdown.Render: %w", err)
	}
	return htmlutil.SanitizeReader(&buf).String(), nil
}

// Goldmark returns a preconfigured Markdown renderer.
var Goldmark = sync.OnceValue(func() goldmark.Markdown {
	html.LinkAttributeFilter.Add([]byte("aria-hidden"))
	html.LinkAttributeFilter.Add([]byte("name"))

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithFormatOptions(
					htmlutil.SyntaxHighlightingOptions()...,
				),
			),
			jupyter.Attachments(),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(util.Prioritized(mdTransformFunc(mdLinkHeaders), 1)),
		),
		goldmark.WithRendererOptions(
			// HTML sanitization is handled by htmlutil
			html.WithUnsafe(),
		),
	)
	return md
})

type mdTransformFunc func(*ast.Document, text.Reader, parser.Context)

var _ parser.ASTTransformer = new(mdTransformFunc)

func (f mdTransformFunc) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	f(node, reader, pc)
}

func mdLinkHeaders(doc *ast.Document, _ text.Reader, _ parser.Context) {
	mdWalk(doc)
}

func mdWalk(n ast.Node) {
	switch n := n.(type) {
	case *ast.Heading:
		id, ok := n.AttributeString("id")
		if !ok {
			return
		}

		var idStr string
		switch id := id.(type) {
		case []byte:
			idStr = string(id)
		case string:
			idStr = id
		default:
			return
		}

		anchorLink := ast.NewLink()
		anchorLink.Destination = []byte("#" + idStr)
		anchorLink.SetAttributeString("class", []byte("anchor"))
		anchorLink.SetAttributeString("rel", []byte("nofollow"))
		anchorLink.SetAttributeString("aria-hidden", []byte("true"))
		anchorLink.SetAttributeString("name", id)

		n.InsertBefore(n, n.FirstChild(), anchorLink)
		return
	}
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		mdWalk(child)
	}
}
