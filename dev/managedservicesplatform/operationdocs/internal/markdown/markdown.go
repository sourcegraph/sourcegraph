// Package markdown is slightly modified copy-pasta from
// https://sourcegraph.sourcegraph.com/github.com/sourcegraph/controller/-/tree/internal/markdown
package markdown

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/olekukonko/tablewriter"
)

// Builder can be used to generate Markdown content piecewise. It implements the internal
// Renderer interface.
type Builder struct{ acc *strings.Builder }

// NewBuilder creates a Builder that can be used to generate Markdown content piecewise.
func NewBuilder() *Builder {
	return &Builder{
		acc: &strings.Builder{},
	}
}

// Headingf writes a heading at the desired level. It automatically adds newlines.
// Returns the sanitized anchor id for the heading and the link to this heading
func (b *Builder) Headingf(level int, title string, vars ...any) (id string, link string) {
	var content string
	id, link, content = Headingf(level, title, vars...)
	b.acc.WriteString(content)
	return id, link
}

// sanitizeHeadingID returns a sanitized anchor name for the given text.
// Taken from https://github.com/shurcooL/sanitized_anchor_name/blob/master/main.go#L14:1
// copy from https://sourcegraph.com/github.com/gomarkdown/markdown@663e2500819c19ed2d3f4bf955931b16fa9adf63/-/blob/parser/block.go?L83-104
func sanitizeHeadingID(text string) string {
	var anchorName []rune
	var futureDash = false
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			if futureDash && len(anchorName) > 0 {
				anchorName = append(anchorName, '-')
			}
			futureDash = false
			anchorName = append(anchorName, unicode.ToLower(r))
		default:
			futureDash = true
		}
	}
	if len(anchorName) == 0 {
		return "empty"
	}
	return string(anchorName)
}

type AdmonitionLevel string

const (
	AdmonitionWarning   AdmonitionLevel = "Warning"
	AdmonitionNote      AdmonitionLevel = "Note"
	AdmonitionImportant AdmonitionLevel = "Important"
)

// Admonitionf renders a blockquote admonition that is supported by GitHub:
// https://github.com/orgs/community/discussions/16925
//
// These render nicely as callouts for the reader to take note of based on the
// AdmonitionLevel.
func (b *Builder) Admonitionf(level AdmonitionLevel, content string, args ...any) {
	b.acc.WriteString("\n")
	switch level {
	case AdmonitionNote:
		b.acc.WriteString("> [!NOTE]\n")
	case AdmonitionWarning:
		b.acc.WriteString("> [!WARNING]\n")
	case AdmonitionImportant:
		b.acc.WriteString("> [!IMPORTANT]\n")
	default:
		panic(fmt.Sprintf("unknown admonition level %q", level))
	}
	for _, line := range strings.Split(strings.TrimSpace(fmt.Sprintf(content, args...)), "\n") {
		if line == "" {
			b.acc.WriteString(">") // don't write unnecessary whitespace
		} else {
			b.acc.WriteString("> " + line)
		}
		b.acc.WriteString("\n")
	}
}

// Paragraphf adds a new paragraph with the given text. It automatically adds newlines.
func (b *Builder) Paragraphf(content string, vars ...any) {
	b.acc.WriteString("\n")
	b.acc.WriteString(strings.TrimSpace(fmt.Sprintf(content, vars...)))
	b.acc.WriteString("\n")
}

// List adds the given lines as list items. It automatically adds newlines.
// It supports arbitrary nesting of lists of string, and each sub-list will be indented.
func (b *Builder) List(lines any) {
	b.acc.WriteString("\n")
	b.acc.WriteString(renderList(lines, -1))
	// already has trailing newline
}

// CodeBlock adds a new code block in the given language with the given text. It
// automatically adds newlines.
func (b *Builder) CodeBlock(lang string, content string) {
	b.acc.WriteString(fmt.Sprintf("\n```%s\n", lang))
	b.acc.WriteString(strings.ReplaceAll(strings.TrimSpace(content), "\t", "  "))
	b.acc.WriteString("\n```\n")
}

// CodeBlockf adds a new code block in the given language with the given text. It
// automatically adds newlines.
func (b *Builder) CodeBlockf(lang string, content string, vars ...any) {
	b.acc.WriteString(fmt.Sprintf("\n```%s\n", lang))
	b.acc.WriteString(strings.ReplaceAll(strings.TrimSpace(fmt.Sprintf(content, vars...)), "\t", "  "))
	b.acc.WriteString("\n```\n")
}

// EscapedCodeBlock adds a new code block in the given language with the given text. It
// automatically adds newlines.
// It uses four backticks instead of three, so that you can have nested code blocks.
func (b *Builder) EscapedCodeBlock(lang string, content string) {
	b.acc.WriteString(fmt.Sprintf("\n````%s\n", lang))
	b.acc.WriteString(strings.ReplaceAll(strings.TrimSpace(content), "\t", "  "))
	b.acc.WriteString("\n````\n")
}

// Table adds a new table with the given headers and data. It automatically adds new
// lines.
func (b *Builder) Table(headers []string, data [][]string) {
	b.acc.WriteString("\n")

	table := tablewriter.NewWriter(b.acc)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.SetAutoWrapText(false) // avoid test wrapping spanning multiple rows, it looks weird
	table.SetHeader(headers)
	table.AppendBulk(data)

	table.Render() // adds a new line
}

// Quotef creates a quote for the given text. It automatically adds newlines.
func (b *Builder) Quotef(content string, vars ...any) {
	b.acc.WriteString("\n")
	b.acc.WriteString("> ")
	b.acc.WriteString(strings.TrimSpace(fmt.Sprintf(content, vars...)))
	b.acc.WriteString("\n")
}

// Commentf inserts a Markdown comment - these do not get rendered in most
// Markdown viewers.
func (b *Builder) Commentf(content string, vars ...any) {
	b.acc.WriteString("\n")
	b.acc.WriteString("<!--\n")
	b.acc.WriteString(strings.TrimSpace(fmt.Sprintf(content, vars...)))
	b.acc.WriteString("\n-->\n")
}

// String returns the accumulated content as a raw Markdown string.
func (b *Builder) String() string {
	return strings.TrimPrefix(b.acc.String(), "\n")
}
