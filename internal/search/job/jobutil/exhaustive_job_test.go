package jobutil

import (
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/job/printer"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestNewExhaustive(t *testing.T) {
	// When reading the autogold expectations things we currently want to see:
	//
	//  - Simple jobs that are essentially just REPOPAGER and SEARCHERTEXTSEARCH
	//  - Only use searcher, not zoekt (indexed . false)
	//  - Avoid normal timeouts (useFullDeadline . true)
	//  - High limits (filematchlimit:1000000)
	repoRevs := &search.RepositoryRevisions{
		Repo: types.MinimalRepo{
			ID:   1,
			Name: "repo",
		},
		Revs: []string{"dev1"},
	}

	cases := []struct {
		Name      string
		Query     string
		WantPager autogold.Value
		WantJob   autogold.Value
	}{
		{
			Name:  "glob",
			Query: `type:file index:no repo:foo rev:*refs/heads/dev* content`,
			WantPager: autogold.Expect(`
(REPOPAGER
  (containsRefGlobs . true)
  (repoOpts.repoFilters . [foo@*refs/heads/dev*])
  (repoOpts.useIndex . no)
  (PARTIALREPOS
    (SEARCHERTEXTSEARCH
      (useFullDeadline . true)
      (patternInfo . TextPatternInfo{"content",nopath,filematchlimit:1000000})
      (numRepos . 0)
      (pathRegexps . [])
      (indexed . false))))
`),
			WantJob: autogold.Expect(`
(SEARCHERTEXTSEARCH
  (useFullDeadline . true)
  (patternInfo . TextPatternInfo{"content",nopath,filematchlimit:1000000})
  (numRepos . 1)
  (pathRegexps . [])
  (indexed . false))
`),
		},
		{
			Name:  "only pattern",
			Query: "type:file index:no content",
			WantPager: autogold.Expect(`
(REPOPAGER
  (containsRefGlobs . false)
  (repoOpts.useIndex . no)
  (PARTIALREPOS
    (SEARCHERTEXTSEARCH
      (useFullDeadline . true)
      (patternInfo . TextPatternInfo{"content",nopath,filematchlimit:1000000})
      (numRepos . 0)
      (pathRegexps . [])
      (indexed . false))))
`),
			WantJob: autogold.Expect(`
(SEARCHERTEXTSEARCH
  (useFullDeadline . true)
  (patternInfo . TextPatternInfo{"content",nopath,filematchlimit:1000000})
  (numRepos . 1)
  (pathRegexps . [])
  (indexed . false))
`),
		},
		{
			Name:  "keyword search",
			Query: "type:file index:no foo bar baz patterntype:keyword",
			WantPager: autogold.Expect(`
(REPOPAGER
  (containsRefGlobs . false)
  (repoOpts.useIndex . no)
  (PARTIALREPOS
    (SEARCHERTEXTSEARCH
      (useFullDeadline . true)
      (patternInfo . TextPatternInfo{"foo bar baz",nopath,filematchlimit:1000000})
      (numRepos . 0)
      (pathRegexps . [])
      (indexed . false))))
`),
			WantJob: autogold.Expect(`
(SEARCHERTEXTSEARCH
  (useFullDeadline . true)
  (patternInfo . TextPatternInfo{"foo bar baz",nopath,filematchlimit:1000000})
  (numRepos . 1)
  (pathRegexps . [])
  (indexed . false))
`),
		},
		{
			Name:  "boolean query",
			Query: "type:file index:no (foo OR bar) AND baz patterntype:keyword",
			WantPager: autogold.Expect(`
(REPOPAGER
  (containsRefGlobs . false)
  (repoOpts.useIndex . no)
  (PARTIALREPOS
    (SEARCHERTEXTSEARCH
      (useFullDeadline . true)
      (patternInfo . TextPatternInfo{(("foo" OR "bar") AND "baz"),nopath,filematchlimit:1000000})
      (numRepos . 0)
      (pathRegexps . [])
      (indexed . false))))
`),
			WantJob: autogold.Expect(`
(SEARCHERTEXTSEARCH
  (useFullDeadline . true)
  (patternInfo . TextPatternInfo{(("foo" OR "bar") AND "baz"),nopath,filematchlimit:1000000})
  (numRepos . 1)
  (pathRegexps . [])
  (indexed . false))
`),
		},
		{
			Name:  "regexp",
			Query: "type:file index:no foo.*bar patterntype:regexp",
			WantPager: autogold.Expect(`
(REPOPAGER
  (containsRefGlobs . false)
  (repoOpts.useIndex . no)
  (PARTIALREPOS
    (SEARCHERTEXTSEARCH
      (useFullDeadline . true)
      (patternInfo . TextPatternInfo{"foo.*bar",nopath,filematchlimit:1000000})
      (numRepos . 0)
      (pathRegexps . [])
      (indexed . false))))
`),
			WantJob: autogold.Expect(`
(SEARCHERTEXTSEARCH
  (useFullDeadline . true)
  (patternInfo . TextPatternInfo{"foo.*bar",nopath,filematchlimit:1000000})
  (numRepos . 1)
  (pathRegexps . [])
  (indexed . false))
`),
		},
		{
			Name:  "repo:has.file predicate",
			Query: "type:file index:no foo repo:has.file(go.mod)",
			WantPager: autogold.Expect(`
(REPOPAGER
  (containsRefGlobs . false)
  (repoOpts.useIndex . no)
  (repoOpts.hasFileContent[0].path . go.mod)
  (PARTIALREPOS
    (SEARCHERTEXTSEARCH
      (useFullDeadline . true)
      (patternInfo . TextPatternInfo{"foo",nopath,filematchlimit:1000000})
      (numRepos . 0)
      (pathRegexps . [])
      (indexed . false))))
`),
			WantJob: autogold.Expect(`
(SEARCHERTEXTSEARCH
  (useFullDeadline . true)
  (patternInfo . TextPatternInfo{"foo",nopath,filematchlimit:1000000})
  (numRepos . 1)
  (pathRegexps . [])
  (indexed . false))
`),
		},
		{
			Name:  "repo:has.meta predicate",
			Query: "type:file index:no foo repo:has.meta(cognition)",
			WantPager: autogold.Expect(`
(REPOPAGER
  (containsRefGlobs . false)
  (repoOpts.useIndex . no)
  (repoOpts.hasKVPs[0].key . cognition)
  (PARTIALREPOS
    (SEARCHERTEXTSEARCH
      (useFullDeadline . true)
      (patternInfo . TextPatternInfo{"foo",nopath,filematchlimit:1000000})
      (numRepos . 0)
      (pathRegexps . [])
      (indexed . false))))
`),
			WantJob: autogold.Expect(`
(SEARCHERTEXTSEARCH
  (useFullDeadline . true)
  (patternInfo . TextPatternInfo{"foo",nopath,filematchlimit:1000000})
  (numRepos . 1)
  (pathRegexps . [])
  (indexed . false))
`),
		},
		{
			Name:  "type:diff author:alice",
			Query: "index:no type:diff author:alice",
			WantPager: autogold.Expect(`
(REPOPAGER
  (containsRefGlobs . false)
  (repoOpts.useIndex . no)
  (repoOpts.onlyCloned . true)
  (PARTIALREPOS
    (DIFFSEARCH
      (includeModifiedFiles . false)
      (query . *protocol.AuthorMatches(alice))
      (diff . true)
      (limit . 1000000))))
`),
			WantJob: autogold.Expect(`
(DIFFSEARCH
  (includeModifiedFiles . false)
  (query . *protocol.AuthorMatches(alice))
  (diff . true)
  (limit . 1000000))
`),
		},
		{
			Name:  "type:commit author:alice",
			Query: "index:no type:commit author:alice",
			WantPager: autogold.Expect(`
(REPOPAGER
  (containsRefGlobs . false)
  (repoOpts.useIndex . no)
  (repoOpts.onlyCloned . true)
  (PARTIALREPOS
    (COMMITSEARCH
      (includeModifiedFiles . false)
      (query . *protocol.AuthorMatches(alice))
      (diff . false)
      (limit . 1000000))))
`),
			WantJob: autogold.Expect(`
(COMMITSEARCH
  (includeModifiedFiles . false)
  (query . *protocol.AuthorMatches(alice))
  (diff . false)
  (limit . 1000000))
`),
		},
		{
			Name:  "type:path f:search.go",
			Query: "index:no type:path f:search.go",
			WantPager: autogold.Expect(`
(REPOPAGER
  (containsRefGlobs . false)
  (repoOpts.useIndex . no)
  (PARTIALREPOS
    (SEARCHERTEXTSEARCH
      (useFullDeadline . true)
      (patternInfo . TextPatternInfo{//,nocontent,filematchlimit:1000000,f:"search.go"})
      (numRepos . 0)
      (pathRegexps . [(?i)search.go])
      (indexed . false))))
`),
			WantJob: autogold.Expect(`
(SEARCHERTEXTSEARCH
  (useFullDeadline . true)
  (patternInfo . TextPatternInfo{//,nocontent,filematchlimit:1000000,f:"search.go"})
  (numRepos . 1)
  (pathRegexps . [(?i)search.go])
  (indexed . false))
`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			searchType := query.SearchTypeLiteral
			plan, err := query.Pipeline(query.Init(tc.Query, searchType))
			require.NoError(t, err)

			inputs := &search.Inputs{
				Plan:         plan,
				Query:        plan.ToQ(),
				UserSettings: &schema.Settings{},
				PatternType:  searchType,
				Protocol:     search.Exhaustive,
				Features:     &search.Features{},
			}

			exhaustive, err := NewExhaustive(inputs)
			require.NoError(t, err)

			tc.WantPager.Equal(t, sPrintSexpMax(exhaustive.repoPagerJob))

			tc.WantJob.Equal(t, sPrintSexpMax(exhaustive.Job(repoRevs)))
		})
	}
}

func sPrintSexpMax(j job.Describer) string {
	return "\n" + printer.SexpVerbose(j, job.VerbosityMax, true) + "\n"
}

// The queries are validated before they reach exhaustive search, hence we only
// have to worry about valid queries we don't want to process for now.
func TestNewExhaustive_negative(t *testing.T) {
	tc := []struct {
		query              string
		isPatterntypeRegex bool
	}{
		// multiple jobs needed
		{query: `type:file index:no (repo:repo1 or repo:repo2) content`},
		// catch-all regex
		{query: `type:file index:no r:.* .*`, isPatterntypeRegex: true},
		{query: `type:file index:no r:repo .*`, isPatterntypeRegex: true},
		// file predicates
		{query: `type:file index:no file:has.content(content)`},
		{query: `type:file index:no file:has.owner(owner)`},
		{query: `type:file index:no file:contains.content(content)`},
		{query: `type:file index:no file:has.contributor(contributor)`},
		// unsupported types
		{query: `index:no type:repo`},
		{query: `index:no type:symbol`},
		{query: `index:no foo select:file.owners`},
	}

	for _, c := range tc {
		t.Run("", func(t *testing.T) {
			patternType := query.SearchTypeStandard
			if c.isPatterntypeRegex {
				patternType = query.SearchTypeRegex
			}

			plan, err := query.Pipeline(query.Init(c.query, patternType))
			require.NoError(t, err)

			inputs := &search.Inputs{
				Plan:         plan,
				Query:        plan.ToQ(),
				UserSettings: &schema.Settings{},
				PatternType:  patternType,
				Protocol:     search.Exhaustive,
				Features:     &search.Features{},
			}

			_, err = NewExhaustive(inputs)
			require.Error(t, err, "failed query: %q", c.query)
		})
	}
}
