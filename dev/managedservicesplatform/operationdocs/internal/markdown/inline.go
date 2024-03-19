package markdown

import (
	"fmt"
	neturl "net/url"
	"reflect"
	"strings"
)

// HeadingLink generates a link to a heading that has not been written yet.
// A Heading must be added with the returned heading eventually, or this link
// will point to nothing.
func HeadingLinkf(title string, vars ...any) (link, heading string) {
	heading = fmt.Sprintf(title, vars...)
	id := sanitizeHeadingID(heading)
	return Linkf(heading, "#%s", id), heading
}

// Headingf writes a heading at the desired level. It automatically adds newlines.
// Returns the sanitized anchor id for the heading and the link to this heading
func Headingf(level int, title string, vars ...any) (id string, link string, content string) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n%s ", strings.Repeat("#", level)))
	link, heading := HeadingLinkf(title, vars...)
	b.WriteString(heading)
	b.WriteString("\n")
	return id, link, b.String()
}

func Code(v string) string {
	return fmt.Sprintf("`%s`", v)
}

func Codef(v string, vars ...any) string {
	return Code(fmt.Sprintf(v, vars...))
}

func Bold(v string) string {
	return fmt.Sprintf("**%s**", v)
}

func Boldf(v string, vars ...any) string {
	return Bold(fmt.Sprintf(v, vars...))
}

// Link generates a Markdown link.
func Link(text, url string) string {
	// some urls params are not escaped properly, let's fix that magically
	parsedUrl, err := neturl.Parse(url)
	// we are intentionally ignoring the error here
	// if the user supplied an invalid url, it may or may not be intentional
	// either way, not this method's problem
	if err == nil {
		params := neturl.Values{}
		for k, v := range parsedUrl.Query() {
			for _, vv := range v {
				params.Add(k, vv)
			}
		}
		parsedUrl.RawQuery = params.Encode()
		url = parsedUrl.String()
	}
	return fmt.Sprintf("[%s](%s)", text, url)
}

// Linkf generates a Markdown link. Format arguments only apply to the URL.
func Linkf(text, url string, vars ...any) string {
	return Link(text, fmt.Sprintf(url, vars...))
}

// List generates a Markdown list.
// It supports arbitrary nesting of lists of string, and each sub-list will be indented.
func List(lines any) string {
	return renderList(lines, -1)
}

// renderList is a helper method to render unknown depth list of strings.
// depth has to start from -1 because the first line is not indented.
//
// You should use the exported List() or List() from Builder struct instead.
func renderList(lines any, depth int) string {
	var buffer strings.Builder

	s := reflect.ValueOf(lines)
	t := reflect.TypeOf(s.Interface())
	v := reflect.ValueOf(s.Interface())
	switch t.Kind() {
	case reflect.String:
		buffer.WriteString(indent(depth))
		buffer.WriteString(fmt.Sprintf("- %v\n", v))
	case reflect.Slice:
		for i := range v.Len() {
			buffer.WriteString(renderList(v.Index(i).Interface(), depth+1))
		}
	default:
		buffer.WriteString(indent(depth))
		buffer.WriteString(fmt.Sprintf("- unknown type: %T\n", v.Interface()))
	}

	return buffer.String()
}

func indent(d int) string {
	return strings.Repeat("  ", d)
}
