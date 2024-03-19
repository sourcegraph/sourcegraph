package streaming

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/grafana/regexp"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
)

// SearchFilters computes the filters to show a user based on results.
//
// Note: it currently lives in graphqlbackend. However, once we have a non
// resolver based SearchResult type it can be extracted. It lives in its own
// file to make that more obvious. We already have the filter type extracted
// (Filter).
type SearchFilters struct {
	filters filters

	// Dirty is true if SearchFilters has changed since the last call to Compute()
	Dirty bool
}

// commonFileFilters are common filters used. It is used by SearchFilters to
// propose them if they match shown results.
var commonFileFilters = []struct {
	label       string
	regexp      *lazyregexp.Regexp
	regexFilter string
	globFilter  string
}{
	{
		label:       "Exclude Go tests",
		regexp:      lazyregexp.New(`_test\.go$`),
		regexFilter: `-file:_test\.go$`,
		globFilter:  `-file:**_test.go`,
	},
	{
		label:       "Exclude Go vendor",
		regexp:      lazyregexp.New(`(^|/)vendor/`),
		regexFilter: `-file:(^|/)vendor/`,
		globFilter:  `-file:vendor/** -file:**/vendor/**`,
	},
	{
		label:       "Exclude node_modules",
		regexp:      lazyregexp.New(`(^|/)node_modules/`),
		regexFilter: `-file:(^|/)node_modules/`,
		globFilter:  `-file:node_modules/** -file:**/node_modules/**`,
	},
	{
		label:       "Exclude minified JavaScript",
		regexp:      lazyregexp.New(`\.min\.js$`),
		regexFilter: `-file:\.min\.js$`,
		globFilter:  `-file:**.min.js`,
	},
	{
		label:       "Exclude JavaScript maps",
		regexp:      lazyregexp.New(`\.js\.map$`),
		regexFilter: `-file:\.js\.map$`,
		globFilter:  `-file:**.js.map`,
	},
}

const (
	// After/Before Filters
	AFTER  = "after"
	BEFORE = "before"

	// After/Before Values
	YESTERDAY     = "yesterday"
	ONE_WEEK_AGO  = `"1 week ago"`
	ONE_MONTH_AGO = `"1 month ago"`
)

type dateFilterInfo struct {
	Timeframe string
	Value     string
	Label     string
}

func determineTimeframe(date time.Time) dateFilterInfo {
	now := time.Now()

	switch {
	case date.After(now.Add(-25 * time.Hour)):
		return dateFilterInfo{
			Timeframe: AFTER,
			Value:     YESTERDAY,
			Label:     "Last 24 hours",
		}
	case date.After(now.Add(-8 * 24 * time.Hour)):
		return dateFilterInfo{
			Timeframe: BEFORE,
			Value:     ONE_WEEK_AGO,
			Label:     "Last week",
		}
	case date.After(now.Add(-31 * 24 * time.Hour)):
		return dateFilterInfo{
			Timeframe: BEFORE,
			Value:     ONE_MONTH_AGO,
			Label:     "Last month",
		}

	default:
		return dateFilterInfo{
			Timeframe: BEFORE,
			Value:     `"2 months ago"`,
			Label:     "Older than 2 months",
		}
	}
}

// Update internal state for the results in event.
func (s *SearchFilters) Update(event SearchEvent) {
	// Initialize state on first call.
	if s.filters == nil {
		s.filters = make(filters)
	}

	addRepoFilter := func(repoName api.RepoName, rev string, lineMatchCount int32) {
		filter := fmt.Sprintf(`repo:^%s$`, regexp.QuoteMeta(string(repoName)))
		if rev != "" {
			// We don't need to quote rev. The only special characters we interpret
			// are @ and :, both of which are disallowed in git refs
			filter = filter + fmt.Sprintf(`@%s`, rev)
		}
		s.filters.Add(filter, string(repoName), lineMatchCount, FilterKindRepo)
	}

	addFileFilter := func(fileMatchPath string, lineMatchCount int32) {
		for _, ff := range commonFileFilters {
			// use regexp to match file paths unconditionally, whether globbing is enabled or not,
			// since we have no native library call to match `**` for globs.
			if ff.regexp.MatchString(fileMatchPath) {
				s.filters.Add(ff.regexFilter, ff.label, lineMatchCount, FilterKindFile)
			}
		}
	}

	addLangFilter := func(rawLanguage string, lineMatchCount int32) {
		if rawLanguage == "" {
			return
		}
		language := strings.ToLower(rawLanguage)
		if strings.Contains(language, " ") {
			language = strconv.Quote(language)
		}
		value := fmt.Sprintf(`lang:%s`, language)
		s.filters.Add(value, rawLanguage, lineMatchCount, FilterKindLang)
	}

	addSymbolFilter := func(symbols []*result.SymbolMatch) {
		for _, sym := range symbols {
			selectKind, ok := result.ToSelectKind[strings.ToLower(sym.Symbol.Kind)]
			if !ok {
				// Skip any symbols we don't know how to select
				// TODO(@camdencheek): figure out which symbols are missing from result.ToSelectKind
				continue
			}
			filter := fmt.Sprintf(`select:symbol.%s`, selectKind)
			s.filters.Add(filter, cases.Title(language.English, cases.Compact).String(selectKind), 1, FilterKindSymbolType)
		}
	}

	addCommitAuthorFilter := func(commit gitdomain.Commit) {
		filter := fmt.Sprintf(`author:%s`, regexp.QuoteMeta(commit.Author.Email))
		s.filters.Add(filter, commit.Author.Name, 1, FilterKindAuthor)
	}

	addCommitDateFilter := func(commit gitdomain.Commit) {
		var cd time.Time
		if commit.Committer != nil {
			cd = commit.Committer.Date
		} else {
			cd = commit.Author.Date
		}

		df := determineTimeframe(cd)
		filter := fmt.Sprintf("%s:%s", df.Timeframe, df.Value)
		s.filters.Add(filter, df.Label, 1, FilterKindCommitDate)
	}

	addTypeFilter := func(value, label string, count int32) {
		if count == 0 {
			return
		}
		s.filters.Add(value, label, count, FilterKindType)
		s.filters.MarkImportant(value)
	}

	if event.Stats.ExcludedForks > 0 {
		s.filters.Add("fork:yes", "Include forked repos", int32(event.Stats.ExcludedForks), FilterKindUtility)
		s.filters.MarkImportant("fork:yes")
		s.Dirty = true
	}
	if event.Stats.ExcludedArchived > 0 {
		s.filters.Add("archived:yes", "Include archived repos", int32(event.Stats.ExcludedArchived), FilterKindUtility)
		s.filters.MarkImportant("archived:yes")
		s.Dirty = true
	}

	for _, match := range event.Results {
		switch v := match.(type) {
		case *result.FileMatch:
			rev := ""
			if v.InputRev != nil {
				rev = *v.InputRev
			}
			lines := int32(v.ResultCount())

			addRepoFilter(v.Repo.Name, rev, lines)
			addLangFilter(v.MostLikelyLanguage(), lines)
			addFileFilter(v.Path, lines)
			addSymbolFilter(v.Symbols)
			addTypeFilter("type:file", "Code", int32(v.ChunkMatches.MatchCount()))
			addTypeFilter("type:symbol", "Symbols", int32(len(v.Symbols)))
			if len(v.Symbols) == 0 && len(v.ChunkMatches) == 0 && len(v.PathMatches) == 0 {
				// If we have no highlights, we still have a match on the file itself,
				// so count that as a "path match".
				addTypeFilter("type:path", "Paths", 1)
			} else {
				addTypeFilter("type:path", "Paths", int32(len(v.PathMatches)))
			}
			s.Dirty = true
		case *result.RepoMatch:
			addTypeFilter("type:repo", "Repositories", 1)
			s.Dirty = true
		case *result.CommitMatch:
			// We leave "rev" empty, instead of using "CommitMatch.Commit.ID". This way we
			// get 1 filter per repo instead of 1 filter per sha in the side-bar.
			addRepoFilter(v.Repo.Name, "", int32(v.ResultCount()))
			addCommitAuthorFilter(v.Commit)
			addCommitDateFilter(v.Commit)
			if v.DiffPreview != nil {
				addTypeFilter("type:diff", "Diffs", int32(v.ResultCount()))
			} else {
				addTypeFilter("type:commit", "Commits", int32(v.ResultCount()))
			}
			s.Dirty = true

			// =========== TODO: Jason Repo Metadata filters ============
			// file paths are in v.ModifiedFiles which is a []string
		}
	}
}

// Compute returns an ordered slice of Filters to present to the user based on
// events passed to Next.
func (s *SearchFilters) Compute() []*Filter {
	s.Dirty = false
	return s.filters.Compute(computeOpts{
		MaxRepos: 1000,
		MaxOther: 1000,
	})
}
