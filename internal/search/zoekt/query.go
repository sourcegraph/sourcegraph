package zoekt

import (
	"regexp/syntax" //nolint:depguard // using the grafana fork of regexp clashes with zoekt, which uses the std regexp/syntax.

	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/zoektquery"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/languages"
	"github.com/sourcegraph/sourcegraph/lib/errors"

	zoekt "github.com/sourcegraph/zoekt/query"
)

func QueryToZoektQuery(b query.Basic, resultTypes result.Types, feat *search.Features, typ search.IndexedRequestType) (q zoekt.Q, err error) {
	isCaseSensitive := b.IsCaseSensitive()

	if b.Pattern != nil {
		q, err = toZoektPattern(
			b.Pattern,
			isCaseSensitive,
			resultTypes.Has(result.TypeFile),
			resultTypes.Has(result.TypePath),
			typ,
		)
		if err != nil {
			return nil, err
		}
	}

	var and []zoekt.Q
	if q != nil {
		and = append(and, q)
	}

	// Handle file: and -file: filters.
	filesInclude, filesExclude := b.IncludeExcludeValues(query.FieldFile)

	// Handle lang: and -lang: filters.
	// By default, languages are converted to file filters. When the 'search-content-based-lang-detection'
	// feature is enabled, we use Zoekt's native language filters, which are based on the actual language
	// of the file (as determined by go-enry).
	langInclude, langExclude := b.IncludeExcludeValues(query.FieldLang)
	if feat.ContentBasedLangFilters {
		for _, lang := range langInclude {
			and = append(and, toLangFilter(lang))
		}
		for _, lang := range langExclude {
			filter := toLangFilter(lang)
			and = append(and, &zoekt.Not{Child: filter})
		}
	} else {
		// By default, convert the language filters to file regexp patterns. Note: these
		// are always case-insensitive, to match broadly on extensions like file.C.
		langFilters, err := toFileFilters(
			mapSlice(langInclude, query.LangToFileRegexp),
			mapSlice(langExclude, query.LangToFileRegexp),
			false)
		if err != nil {
			return nil, err
		}
		and = append(and, langFilters...)
	}

	fileFilters, err := toFileFilters(filesInclude, filesExclude, isCaseSensitive)
	if err != nil {
		return nil, err
	}
	and = append(and, fileFilters...)

	var repoHasFilters []zoekt.Q
	for _, filter := range b.RepoHasFileContent() {
		repoHasFilters = append(repoHasFilters, QueryForFileContentArgs(filter, isCaseSensitive))
	}
	if len(repoHasFilters) > 0 {
		and = append(and, zoekt.NewAnd(repoHasFilters...))
	}

	return zoekt.Simplify(zoekt.NewAnd(and...)), nil
}

func toLangFilter(lang string) zoekt.Q {
	lang, _ = languages.GetLanguageByNameOrAlias(lang) // Invariant: lang is valid.
	return &zoekt.Language{Language: lang}
}

// toFileFilters converts a list of file regexp patterns to Zoekt file filters.
func toFileFilters(filesInclude []string, filesExclude []string, isCaseSensitive bool) ([]zoekt.Q, error) {
	var filters []zoekt.Q
	// TODO whitespace in file path patterns?
	for _, i := range filesInclude {
		q, err := zoektquery.FileRe(i, isCaseSensitive)
		if err != nil {
			return nil, err
		}
		filters = append(filters, q)
	}
	if len(filesExclude) > 0 {
		q, err := zoektquery.FileRe(query.UnionRegExps(filesExclude), isCaseSensitive)
		if err != nil {
			return nil, err
		}
		filters = append(filters, &zoekt.Not{Child: q})
	}
	return filters, nil
}

func QueryForFileContentArgs(opt query.RepoHasFileContentArgs, caseSensitive bool) zoekt.Q {
	var children []zoekt.Q
	if opt.Path != "" {
		re, err := syntax.Parse(opt.Path, syntax.Perl)
		if err != nil {
			panic(err)
		}
		children = append(children, &zoekt.Regexp{Regexp: re, FileName: true, CaseSensitive: caseSensitive})
	}
	if opt.Content != "" {
		re, err := syntax.Parse(opt.Content, syntax.Perl)
		if err != nil {
			panic(err)
		}
		children = append(children, &zoekt.Regexp{Regexp: re, Content: true, CaseSensitive: caseSensitive})
	}
	q := zoekt.NewAnd(children...)
	q = &zoekt.Type{Type: zoekt.TypeRepo, Child: q}
	if opt.Negated {
		q = &zoekt.Not{Child: q}
	}
	q = zoekt.Simplify(q)
	return q
}

func toZoektPattern(
	expression query.Node, isCaseSensitive, patternMatchesContent, patternMatchesPath bool, typ search.IndexedRequestType) (zoekt.Q, error) {
	var fold func(node query.Node) (zoekt.Q, error)
	fold = func(node query.Node) (zoekt.Q, error) {
		switch n := node.(type) {
		case query.Operator:
			children := make([]zoekt.Q, 0, len(n.Operands))
			for _, op := range n.Operands {
				child, err := fold(op)
				if err != nil {
					return nil, err
				}
				children = append(children, child)
			}
			switch n.Kind {
			case query.Or:
				return &zoekt.Or{Children: children}, nil
			case query.And:
				return &zoekt.And{Children: children}, nil
			default:
				// unreachable
				return nil, errors.Errorf("broken invariant: don't know what to do with node %T in toZoektPattern", node)
			}
		case query.Pattern:
			var q zoekt.Q
			var err error

			fileNameOnly := patternMatchesPath && !patternMatchesContent
			contentOnly := !patternMatchesPath && patternMatchesContent

			q, err = zoektquery.ParseRe(n.RegExpPattern(), fileNameOnly, contentOnly, isCaseSensitive)
			if err != nil {
				return nil, err
			}

			if typ == search.SymbolRequest && q != nil {
				// Tell zoekt q must match on symbols
				q = &zoekt.Symbol{
					Expr: q,
				}
			}

			if n.Negated {
				q = &zoekt.Not{Child: q}
			}

			if n.Annotation.Labels.IsSet(query.Boost) {
				q = &zoekt.Boost{Child: q, Boost: query.ZoektScoreBoost}
			}

			return q, nil
		}
		// unreachable
		return nil, errors.Errorf("broken invariant: don't know what to do with node %T in toZoektPattern", node)
	}

	q, err := fold(expression)
	if err != nil {
		return nil, err
	}

	return q, nil
}

func mapSlice(values []string, f func(string) string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = f(v)
	}
	return out
}
