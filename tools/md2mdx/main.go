package main

// This code is a hack, it could be improved a lot, but ideally we would fix the docsite rather than
// having to do this. For example, the docsite could simply support normal markdown along mdx, as
// generated docs are not going to have fancy components.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		println("Usage: md2mdx <path>")
		os.Exit(1)
	}

	path := os.Args[1]

	if filepath.Ext(path) != ".md" {
		println("Must pass a markdown file")
		os.Exit(1)
	}

	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}

	// Because of the index page issue when linking, we need the path to the file,
	// so we can work our way to compensate the bug with the relative urls.
	out, err := convert(f, path)
	if err != nil {
		panic(err)
	}

	if err := f.Close(); err != nil {
		panic(err)
	}

	if _, err := os.Stdout.Write([]byte(out)); err != nil {
		panic(err)
	}
}

// htmlCommentsRe matches HTML comments and outputs the comments that MDX wants.
var htmlCommentsRe = regexp.MustCompile(`<!-- (.*) -->`)

func convert(r io.Reader, path string) (string, error) {
	res := []string{}
	s := bufio.NewScanner(r)
	_, parentFolder := filepath.Split(filepath.Dir(path))

	isIndex := strings.HasSuffix(path, "index.md")

	var withinTripleBackticks bool

	for s.Scan() {
		line := s.Text()

		if withinTripleBackticks && strings.Contains(line, "```") {
			withinTripleBackticks = false
			goto APPEND
		}

		if withinTripleBackticks {
			goto APPEND
		}

		// Yes, the first part of the predicate is useless, but it's easier to read.
		if !withinTripleBackticks && strings.Contains(line, "```") {
			withinTripleBackticks = true
			goto APPEND
		}

		line = htmlCommentsRe.ReplaceAllString(line, `{/* $1 */}`)

		line = replaceTagsOrLtGt(line)
		line = replaceLinks(line, parentFolder, isIndex)
		line = replaceCurlies(line)
		line = replacePipes(line)

	APPEND:
		res = append(res, line)
	}

	if err := s.Err(); err != nil {
		return "", err
	}

	return strings.Join(res, "\n"), nil
}

var relativeLinksRe = regexp.MustCompile(`\[(.*)\]\((.*\.md[#\w-_]*)\)`)

// replaceLinks does some magic to handle some issues which should be fixed on
// the new docsite. In particular, because the new docsite renders index pages by removing
// the trailing slash, but do not take that in account when rendering the link, we have
// to artificially fix the rendered link by prefixing the destination with the enclosing folder
// of the index.md
//
// If we're not linking from an index.md, relative links work properly.
//
// Along the way, we have to drop the `.md` extension, and we don't inject the `.mdx` extension
// because it's not supported by the new docsite yet.
func replaceLinks(s string, parentFolder string, isIndex bool) string {
	matches := relativeLinksRe.FindStringSubmatch(s)
	if len(matches) == 0 {
		return s
	}

	content := matches[0]
	text := matches[1]
	link := matches[2]

	link = strings.TrimPrefix(link, "./")

	var anchor string
	if strings.Contains(link, "#") {
		r := regexp.MustCompile(`.*\#(.*)$`)
		matches = r.FindStringSubmatch(link)
		anchor = matches[1]
		link = strings.Replace(link, "#"+anchor, "", 1)
	}

	if strings.HasSuffix(link, "/index.md") {
		link = strings.TrimSuffix(link, "/index.md")
	} else {
		link = strings.TrimSuffix(link, ".md")
	}

	if anchor != "" {
		link = link + "#" + anchor
	}

	if isIndex {
		return strings.Replace(s, content, fmt.Sprintf("[%s](%s/%s)", text, parentFolder, link), -1)
	} else {
		return strings.Replace(s, content, fmt.Sprintf("[%s](%s)", text, link), -1)
	}
}

var tagsRe = regexp.MustCompile(`<([^>]+)>`)

// replaceTagsOrLtGt replaces all tags with their HTML entity equivalents, and all < with &lt; and all > with &gt;.
// This is quite fragile, and it's surely not going to work if you mix both.
//
// We also have a special case for <extID> from src-cli (see tests).
func replaceTagsOrLtGt(line string) string {
	if !tagsRe.MatchString(line) || strings.Contains(line, "<extID>") {
		line = strings.ReplaceAll(line, "<", "&lt;")
		line = strings.ReplaceAll(line, ">", "&gt;")
	}
	return line
}

// replaceCurlies replaces all curlies when they should be replaced, which follows
// some strange rules.
func replaceCurlies(s string) string {
	singleQuoteRe := regexp.MustCompile(`'([^']+)'`)
	doubleQuoteRe := regexp.MustCompile(`"([^']+)"`)
	rawDoubleCurlies := regexp.MustCompile("[^`]{{([^{}]+)}}[^`]?")

	replace := func(s string, r *regexp.Regexp) string {
		chunks := r.FindAllString(s, -1)
		if len(chunks) == 0 {
			return s
		}
		for _, content := range chunks {
			newContent := content
			newContent = strings.Replace(newContent, "{", "\\{", -1)
			newContent = strings.Replace(newContent, "}", "\\}", -1)
			newContent = strings.Replace(newContent, "|", "\\|", -1)

			s = strings.ReplaceAll(s, content, newContent)
		}
		return s
	}

	for _, r := range []*regexp.Regexp{singleQuoteRe, doubleQuoteRe, rawDoubleCurlies} {
		s = replace(s, r)
	}

	return s
}

// replacePipes replaces all pipes when they're inside backticks, because apparently,
// they should be escaped in there.
func replacePipes(s string) string {
	backticksRe := regexp.MustCompile("`([^`]+)`")

	chunks := backticksRe.FindAllString(s, -1)
	if len(chunks) == 0 {
		return s
	}
	for _, content := range chunks {
		newContent := content
		newContent = strings.Replace(newContent, "|", "\\|", -1)

		s = strings.ReplaceAll(s, content, newContent)
	}
	return s
}
