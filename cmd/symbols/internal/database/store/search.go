package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/grafana/regexp/syntax"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func scanSymbols(rows *sql.Rows, queryErr error) (symbols []result.Symbol, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	for rows.Next() {
		var symbol result.Symbol
		if err := rows.Scan(
			&symbol.Name,
			&symbol.Path,
			&symbol.Line,
			&symbol.Character,
			&symbol.Kind,
			&symbol.Language,
			&symbol.Parent,
			&symbol.ParentKind,
			&symbol.Signature,
			&symbol.FileLimited,
		); err != nil {
			return nil, err
		}

		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// MaxSymbolLimit prevents users from accidentally running a query that returns an
// extremely large number of results. It is arbitrary, but it should be at least
// as high as the default limit frontend sends to the symbol service.
const MaxSymbolLimit = 50_000

func (s *store) Search(ctx context.Context, args search.SymbolsParameters) ([]result.Symbol, bool, error) {
	if args.First < 0 {
		return nil, false, errors.New("limit must be greater than or equal to 0")
	}

	limit := args.First
	outOfBounds := false
	// We check against limit + 1 because frontend will ask for limit + 1. This way
	// we can communicate a nicer number to the user.
	if limit > MaxSymbolLimit+1 {
		outOfBounds = true
		limit = MaxSymbolLimit
	}

	res, err := scanSymbols(s.Query(ctx, sqlf.Sprintf(
		`
			SELECT
				name,
				path,
				line,
				character,
				kind,
				language,
				parent,
				parentkind,
				signature,
				filelimited
			FROM symbols
			WHERE %s
			LIMIT %s
		`,
		sqlf.Join(makeSearchConditions(args), "AND"),
		limit,
	)))
	if err != nil {
		return nil, false, err
	}

	limitHit := outOfBounds && len(res) == limit
	return res, limitHit, nil
}

func makeSearchConditions(args search.SymbolsParameters) []*sqlf.Query {
	conditions := make([]*sqlf.Query, 0, 2+len(args.IncludePatterns))
	conditions = append(conditions, makeSearchCondition("name", args.Query, args.IsCaseSensitive))
	conditions = append(conditions, negate(makeSearchCondition("path", args.ExcludePattern, args.IsCaseSensitive)))
	for _, includePattern := range args.IncludePatterns {
		conditions = append(conditions, makeSearchCondition("path", includePattern, args.IsCaseSensitive))
	}

	for _, includeLang := range args.IncludeLangs {
		conditions = append(conditions, makeLangCondition(includeLang))
	}
	for _, excludeLang := range args.ExcludeLangs {
		conditions = append(conditions, negate(makeLangCondition(excludeLang)))
	}

	filtered := conditions[:0]
	for _, condition := range conditions {
		if condition != nil {
			filtered = append(filtered, condition)
		}
	}

	if len(filtered) == 0 {
		// Ensure we have at least one condition
		filtered = append(filtered, sqlf.Sprintf("TRUE"))
	}

	return filtered
}

func makeSearchCondition(column string, regex string, isCaseSensitive bool) *sqlf.Query {
	if regex == "" {
		return nil
	}

	// Exact match
	if symbolName, isExact, err := isLiteralEquality(regex); err == nil && isExact {
		if isCaseSensitive {
			return sqlf.Sprintf(column+" = %s", symbolName)
		} else {
			return sqlf.Sprintf(column+"lowercase = %s", strings.ToLower(symbolName))
		}
	}

	// Prefix match
	if symbolName, isExact, err := isLiteralPrefix(regex); err == nil && isExact {
		if isCaseSensitive {
			return sqlf.Sprintf(column+" GLOB %s", globEscape(symbolName)+"*")
		} else {
			return sqlf.Sprintf(column+"lowercase GLOB %s", strings.ToLower(globEscape(symbolName))+"*")
		}
	}

	// Regex match
	if !isCaseSensitive {
		regex = "(?i:" + regex + ")"
	}
	return sqlf.Sprintf(column+" REGEXP %s", regex)
}

func makeLangCondition(lang string) *sqlf.Query {
	// We need to convert the lang since language filters use normalized enry names, but
	// the database stores ctags languages.
	ctagsLangs := convertEnryToCTagsLangs(lang)
	var queries []*sqlf.Query
	for _, ctagsLang := range ctagsLangs {
		queries = append(queries, sqlf.Sprintf("%s", ctagsLang))
	}
	return sqlf.Sprintf("lower(language) IN (%s) ", sqlf.Join(queries, ", "))
}

// convertEnryToCTagsLangs performs a best-effort mapping from normalized go-enry
// language names to ctags names. This list is currently not exhaustive.
func convertEnryToCTagsLangs(enryLang string) []string {
	lower := strings.ToLower(enryLang)
	switch lower {
	case "protocol buffer":
		return []string{"protobuf"}
	case "objective-c":
		return []string{"objectivec"}
	case "assembly":
		return []string{"asm"}
	case "raku":
		return []string{"perl6"}
	case "shell":
		return []string{"sh", "zsh"}
	}
	return []string{lower}
}

// isLiteralEquality returns true if the given regex matches literal strings exactly.
// If so, this function returns true along with the literal search query. If not, this
// function returns false.
func isLiteralEquality(expr string) (string, bool, error) {
	regexp, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		return "", false, errors.Wrap(err, "regexp/syntax.Parse")
	}

	// want a concat of size 3 which is [begin, literal, end]
	if regexp.Op == syntax.OpConcat && len(regexp.Sub) == 3 {
		// starts with ^
		if regexp.Sub[0].Op == syntax.OpBeginLine || regexp.Sub[0].Op == syntax.OpBeginText {
			// is a literal
			if regexp.Sub[1].Op == syntax.OpLiteral {
				// ends with $
				if regexp.Sub[2].Op == syntax.OpEndLine || regexp.Sub[2].Op == syntax.OpEndText {
					return string(regexp.Sub[1].Rune), true, nil
				}
			}
		}
	}

	return "", false, nil
}

// isLiteralPrefix returns true if the given regex matches literal strings by prefix.
// If so, this function returns true along with the literal search query. If not, this
// function returns false.
func isLiteralPrefix(expr string) (string, bool, error) {
	regexp, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		return "", false, errors.Wrap(err, "regexp/syntax.Parse")
	}

	// want a concat of size 2 which is [begin, literal]
	if regexp.Op == syntax.OpConcat && len(regexp.Sub) == 2 {
		// starts with ^
		if regexp.Sub[0].Op == syntax.OpBeginLine || regexp.Sub[0].Op == syntax.OpBeginText {
			// is a literal
			if regexp.Sub[1].Op == syntax.OpLiteral {
				return string(regexp.Sub[1].Rune), true, nil
			}
		}
	}

	return "", false, nil
}

func negate(query *sqlf.Query) *sqlf.Query {
	if query == nil {
		return nil
	}

	return sqlf.Sprintf("NOT %s", query)
}

func globEscape(str string) string {
	var out strings.Builder

	specials := `[]*?`

	for _, c := range str {
		if strings.ContainsRune(specials, c) {
			fmt.Fprintf(&out, "[%c]", c)
		} else {
			fmt.Fprintf(&out, "%c", c)
		}
	}

	return out.String()
}
