package query

import (
	"sort"
	"strings"

	"github.com/go-enry/go-enry/v2/data" //nolint:depguard - FIXME: Expose needed APIs in codeintel/languages
	"github.com/grafana/regexp"

	"github.com/sourcegraph/sourcegraph/lib/codeintel/languages"
)

// UnionRegExps separates values with a | operator to create a string
// representing a union of regexp patterns.
func UnionRegExps(values []string) string {
	if len(values) == 0 {
		// As a regular expression, "()" and "" are equivalent so this
		// condition wouldn't ordinarily be needed to distinguish these
		// values. But, our internal search engine assumes that ""
		// implies "no regexp" (no values), while "()" implies "match
		// empty regexp" (all values) for file patterns.
		return ""
	}
	if len(values) == 1 {
		// Cosmetic format for regexp value, wherever this happens to be
		// pretty printed.
		return values[0]
	}
	return "(?:" + strings.Join(values, ")|(?:") + ")"
}

func CaseInsensitiveRegExp(value string) string {
	// Don't add (?i) if it is already present.
	if strings.HasPrefix(value, "(?i)") {
		return value
	}

	return `(?i)` + value
}

// filenamesFromLanguage is a map of language name to full filenames
// that are associated with it. This is different from extensions, because
// some languages (like Dockerfile) do not conventionally have an associated
// extension.
var filenamesFromLanguage = func() map[string][]string {
	res := make(map[string][]string, len(data.LanguagesByFilename))
	for filename, languages := range data.LanguagesByFilename {
		for _, language := range languages {
			res[language] = append(res[language], filename)
		}
	}
	for _, v := range res {
		sort.Strings(v)
	}
	return res
}()

// LangToFileRegexp converts a lang: parameter to its corresponding file
// patterns for file filters. The lang value must be valid, cf. validate.go
func LangToFileRegexp(lang string) string {
	lang, _ = languages.GetLanguageByNameOrAlias(lang) // Invariant: lang is valid.
	extensions := languages.GetLanguageExtensions(lang)
	patterns := make([]string, len(extensions))
	for i, e := range extensions {
		// Add `\.ext$` pattern to match files with the given extension.
		patterns[i] = regexp.QuoteMeta(e) + "$"
	}
	for _, filename := range filenamesFromLanguage[lang] {
		patterns = append(patterns, "(^|/)"+regexp.QuoteMeta(filename)+"$")
	}

	// We always treat lang filters as case insensitive.
	return CaseInsensitiveRegExp(UnionRegExps(patterns))
}

// ZoektScoreBoost is the scoring boost applied to exact phrases, used mainly in ExperimentalPhraseBoost.
const ZoektScoreBoost = 20
