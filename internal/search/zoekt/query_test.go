package zoekt

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hexops/autogold/v2"

	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
)

func TestQueryToZoektQuery(t *testing.T) {
	cases := []struct {
		Name            string
		Type            search.IndexedRequestType
		Query           string
		Features        search.Features
		WantZoektOutput string
	}{
		{
			Name:            "substr",
			Type:            search.TextRequest,
			Query:           `foo patterntype:regexp`,
			WantZoektOutput: `substr:"foo"`,
		},
		{
			Name:            "symbol substr",
			Type:            search.SymbolRequest,
			Query:           `foo patterntype:regexp type:symbol`,
			WantZoektOutput: `sym:substr:"foo"`,
		},
		{
			Name:            "regex",
			Type:            search.TextRequest,
			Query:           `(foo).*?(bar) patterntype:regexp`,
			WantZoektOutput: `regex:"(?-s:foo.*?bar)"`,
		},
		{
			Name:            "path",
			Type:            search.TextRequest,
			Query:           `foo file:\.go$ file:\.yaml$ -file:\bvendor\b patterntype:regexp`,
			WantZoektOutput: `(and substr:"foo" file_regex:"(?m:\\.go$)" file_regex:"(?m:\\.yaml$)" (not file_regex:"\\bvendor\\b"))`,
		},
		{
			Name:            "case",
			Type:            search.TextRequest,
			Query:           `foo case:yes patterntype:regexp file:\.go$ file:yaml`,
			WantZoektOutput: `(and case_substr:"foo" case_file_regex:"(?m:\\.go$)" case_file_substr:"yaml")`,
		},
		{
			Name:            "casepath",
			Type:            search.TextRequest,
			Query:           `foo case:yes file:\.go$ file:\.yaml$ -file:\bvendor\b patterntype:regexp`,
			WantZoektOutput: `(and case_substr:"foo" case_file_regex:"(?m:\\.go$)" case_file_regex:"(?m:\\.yaml$)" (not case_file_regex:"\\bvendor\\b"))`,
		},
		{
			Name:            "path matches only",
			Type:            search.TextRequest,
			Query:           `test type:path`,
			WantZoektOutput: `file_substr:"test"`,
		},
		{
			Name:            "content matches only",
			Type:            search.TextRequest,
			Query:           `test type:file patterntype:literal`,
			WantZoektOutput: `content_substr:"test"`,
		},
		{
			Name:            "content and path matches",
			Type:            search.TextRequest,
			Query:           `test`,
			WantZoektOutput: `substr:"test"`,
		},
		{
			Name:            "Just file",
			Type:            search.TextRequest,
			Query:           `file:\.go$`,
			WantZoektOutput: `file_regex:"(?m:\\.go$)"`,
		},
		{
			Name:            "Languages get passed as file filter",
			Type:            search.TextRequest,
			Query:           `file:\.go$ lang:go`,
			WantZoektOutput: `(and file_regex:"(?m:\\.go$)" file_regex:"(?im:\\.GO$)")`,
		},
		{
			Name:            "Languages still use case_insensitive in case sensitivity mode",
			Type:            search.TextRequest,
			Query:           `file:\.go$ lang:go case:true`,
			WantZoektOutput: `(and case_file_regex:"(?m:\\.go$)" case_file_regex:"(?im:\\.GO$)")`,
		},
		{
			Name:  "Language get passed as lang: query",
			Type:  search.TextRequest,
			Query: `lang:go`,
			Features: search.Features{
				ContentBasedLangFilters: true,
			},
			WantZoektOutput: `lang:Go`,
		},
		{
			Name:  "Multiple languages get passed as lang queries",
			Type:  search.TextRequest,
			Query: `lang:go lang:typescript`,
			Features: search.Features{
				ContentBasedLangFilters: true,
			},
			WantZoektOutput: `(and lang:Go lang:TypeScript)`,
		},
		{
			Name:  "Excluded languages get passed as lang: query",
			Type:  search.TextRequest,
			Query: `lang:go -lang:typescript -lang:markdown`,
			Features: search.Features{
				ContentBasedLangFilters: true,
			},
			WantZoektOutput: `(and lang:Go (not lang:TypeScript) (not lang:Markdown))`,
		},
		{
			Name:  "Mixed file and lang filters",
			Type:  search.TextRequest,
			Query: `file:\.go$ lang:go lang:typescript`,
			Features: search.Features{
				ContentBasedLangFilters: true,
			},
			WantZoektOutput: `(and lang:Go lang:TypeScript file_regex:"(?m:\\.go$)")`,
		},
	}
	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			sourceQuery, _ := query.ParseRegexp(tt.Query)
			b, _ := query.ToBasicQuery(sourceQuery)

			types, _ := b.ToParseTree().StringValues(query.FieldType)
			resultTypes := computeResultTypes(types, b, query.SearchTypeRegex)
			got, err := QueryToZoektQuery(b, resultTypes, &tt.Features, tt.Type)
			if err != nil {
				t.Fatal("QueryToZoektQuery failed:", err)
			}

			queryStr := got.String()
			if diff := cmp.Diff(tt.WantZoektOutput, queryStr); diff != "" {
				t.Errorf("mismatched queries during [%s] (-want +got):\n%s", tt.Name, diff)
			}
		})
	}
}

func Test_toZoektPattern(t *testing.T) {
	test := func(input string, searchType query.SearchType, typ search.IndexedRequestType) string {
		p, err := query.Pipeline(query.Init(input, searchType))
		if err != nil {
			return err.Error()
		}
		zoektQuery, err := toZoektPattern(p[0].Pattern, false, false, false, typ)
		if err != nil {
			return err.Error()
		}
		return zoektQuery.String()
	}

	autogold.Expect(`substr:"a"`).
		Equal(t, test(`a`, query.SearchTypeLiteral, search.TextRequest))

	autogold.Expect(`(or (and substr:"a" substr:"b" (not substr:"c")) substr:"d")`).
		Equal(t, test(`a and b and not c or d`, query.SearchTypeLiteral, search.TextRequest))

	autogold.Expect(`substr:"\"func main() {\\n\""`).
		Equal(t, test(`"func main() {\n"`, query.SearchTypeLiteral, search.TextRequest))

	autogold.Expect(`substr:"func main() {\n"`).
		Equal(t, test(`"func main() {\n"`, query.SearchTypeRegex, search.TextRequest))

	autogold.Expect(`(and sym:substr:"foo" (not sym:substr:"bar"))`).
		Equal(t, test(`type:symbol (foo and not bar)`, query.SearchTypeLiteral, search.SymbolRequest))
}

func computeResultTypes(types []string, b query.Basic, searchType query.SearchType) result.Types {
	var rts result.Types
	if searchType == query.SearchTypeStructural && !b.IsEmptyPattern() {
		rts = result.TypeStructural
	} else {
		if len(types) == 0 {
			rts = result.TypeFile | result.TypePath | result.TypeRepo
		} else {
			for _, t := range types {
				rts = rts.With(result.TypeFromString[t])
			}
		}
	}
	return rts
}
