package embed

import (
	"fmt"
	"path"
	"strings"
)

var commentLinePrefix = map[string]string{
	"py":       "#",
	"pl":       "#",
	"rb":       "#",
	"r":        "#",
	"exs":      "#",
	"ex":       "#",
	"jl":       "#",
	"sh":       "#",
	"yml":      "#",
	"yaml":     "#",
	"sql":      "--",
	"hs":       "--",
	"lua":      "--",
	"md":       "<!--",
	"markdown": "<!--",
	"html":     "<!--",
	"ml":       "(*",
	"mli":      "(*",
	"lisp":     ";;",
	"el":       ";;",
	"clj":      ";;",
	"m":        "%",
	"erl":      "%",
	"txt":      "",
}

var commentLineSuffix = map[string]string{
	"md":       "-->",
	"markdown": "-->",
	"html":     "-->",
	"ml":       "*)",
	"mli":      "*)",
}

func getMetadataHeader(repoName, filePath string) string {
	extension := strings.ToLower(strings.TrimPrefix(path.Ext(filePath), "."))
	prefix, prefixOk := commentLinePrefix[extension]
	if !prefixOk {
		prefix = "//"
	}
	suffix := commentLineSuffix[extension]
	return strings.TrimSpace(fmt.Sprintf("%s %s %s %s", prefix, repoName, filePath, suffix))
}

func addMetadataHeader(code string, repoName string, filePath string) string {
	return getMetadataHeader(repoName, filePath) + "\n" + code
}
