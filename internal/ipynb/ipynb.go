package ipynb

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/bevzzz/nb"
	synth "github.com/bevzzz/nb-synth"
	"github.com/bevzzz/nb/extension"
	"github.com/bevzzz/nb/extension/adapter"
	jupyter "github.com/bevzzz/nb/extension/extra/goldmark-jupyter"
	"github.com/robert-nix/ansihtml"

	"github.com/sourcegraph/sourcegraph/internal/htmlutil"
	"github.com/sourcegraph/sourcegraph/internal/markdown"
)

// Render renders Jupyter Notebook file (.ipynb) to sanitized HTML that is safe to render anywhere.
func Render(content string) (string, error) {
	var buf bytes.Buffer
	c := notebook()
	if err := c.Convert(&buf, []byte(content)); err != nil {
		return "", fmt.Errorf("ipynb.Render: %w", err)
	}
	return htmlutil.SanitizeReader(&buf).String(), nil
}

var notebook = sync.OnceValue(func() *nb.Notebook {
	md := markdown.Goldmark()
	return nb.New(
		nb.WithExtensions(
			jupyter.Goldmark(md),
			synth.NewHighlighting(
				synth.WithFormatOptions(
					htmlutil.SyntaxHighlightingOptions()...,
				),
			),
			extension.NewStream(adapter.AnsiHtml(ansi2html)),
		),
	)
})

// ansi2html calls ansihtml.ConvertToHTMLWithClasses with empty class prefix.
func ansi2html(b []byte) []byte {
	return ansihtml.ConvertToHTMLWithClasses(b, "ansi-", false)
}
