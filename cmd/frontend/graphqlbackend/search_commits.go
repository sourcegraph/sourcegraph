package graphqlbackend

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/xeonx/timeago"

	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/filter"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

// CommitSearchResultResolver is a resolver for the GraphQL type `CommitSearchResult`
type CommitSearchResultResolver struct {
	result.CommitMatch

	db dbutil.DB

	// gitCommitResolver should not be used directly since it may be uninitialized.
	// Use Commit() instead.
	gitCommitResolver *GitCommitResolver
	gitCommitOnce     sync.Once
}

func (r *CommitSearchResultResolver) Select(path filter.SelectPath) SearchResultResolver {
	switch path.Type {
	case filter.Repository:
		return r.Commit().Repository()
	case filter.Commit:
		return r
	}
	return nil
}

func (r *CommitSearchResultResolver) Commit() *GitCommitResolver {
	r.gitCommitOnce.Do(func() {
		if r.gitCommitResolver != nil {
			return
		}
		repoResolver := NewRepositoryResolver(r.db, r.RepoName.ToRepo())
		r.gitCommitResolver = toGitCommitResolver(repoResolver, r.db, r.CommitMatch.Commit.ID, &r.CommitMatch.Commit)
	})
	return r.gitCommitResolver
}

func (r *CommitSearchResultResolver) Refs() []*GitRefResolver {
	out := make([]*GitRefResolver, 0, len(r.CommitMatch.Refs))
	for _, ref := range r.CommitMatch.Refs {
		out = append(out, &GitRefResolver{
			repo: r.Commit().Repository(),
			name: ref,
		})
	}
	return out
}

func (r *CommitSearchResultResolver) SourceRefs() []*GitRefResolver {
	out := make([]*GitRefResolver, 0, len(r.CommitMatch.SourceRefs))
	for _, ref := range r.CommitMatch.SourceRefs {
		out = append(out, &GitRefResolver{
			repo: r.Commit().Repository(),
			name: ref,
		})
	}
	return out
}

func (r *CommitSearchResultResolver) MessagePreview() *highlightedStringResolver {
	if r.CommitMatch.MessagePreview == nil {
		return nil
	}
	return &highlightedStringResolver{*r.CommitMatch.MessagePreview}
}

func (r *CommitSearchResultResolver) DiffPreview() *highlightedStringResolver {
	if r.CommitMatch.DiffPreview == nil {
		return nil
	}
	return &highlightedStringResolver{*r.CommitMatch.DiffPreview}
}

func (r *CommitSearchResultResolver) Icon() string {
	return "data:image/svg+xml;base64,PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0iVVRGLTgiPz48IURPQ1RZUEUgc3ZnIFBVQkxJQyAiLS8vVzNDLy9EVEQgU1ZHIDEuMS8vRU4iICJodHRwOi8vd3d3LnczLm9yZy9HcmFwaGljcy9TVkcvMS4xL0RURC9zdmcxMS5kdGQiPjxzdmcgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIiB4bWxuczp4bGluaz0iaHR0cDovL3d3dy53My5vcmcvMTk5OS94bGluayIgdmVyc2lvbj0iMS4xIiB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCI+PHBhdGggZD0iTTE3LDEyQzE3LDE0LjQyIDE1LjI4LDE2LjQ0IDEzLDE2LjlWMjFIMTFWMTYuOUM4LjcyLDE2LjQ0IDcsMTQuNDIgNywxMkM3LDkuNTggOC43Miw3LjU2IDExLDcuMVYzSDEzVjcuMUMxNS4yOCw3LjU2IDE3LDkuNTggMTcsMTJNMTIsOUEzLDMgMCAwLDAgOSwxMkEzLDMgMCAwLDAgMTIsMTVBMywzIDAgMCwwIDE1LDEyQTMsMyAwIDAsMCAxMiw5WiIgLz48L3N2Zz4="
}

func (r *CommitSearchResultResolver) Label() Markdown {
	message := r.CommitMatch.Commit.Message.Subject()
	author := r.CommitMatch.Commit.Author.Name
	repoName := displayRepoName(r.Commit().Repository().Name())
	repoURL := r.Commit().Repository().URL()
	url := r.Commit().URL()

	label := fmt.Sprintf("[%s](%s) › [%s](%s): [%s](%s)", repoName, repoURL, author, url, message, url)
	return Markdown(label)
}

func (r *CommitSearchResultResolver) URL() string {
	return r.Commit().URL()
}

func (r *CommitSearchResultResolver) Detail() Markdown {
	commitHash := r.CommitMatch.Commit.ID.Short()
	timeagoConfig := timeago.NoMax(timeago.English)
	detail := fmt.Sprintf("[`%v` %v](%v)", commitHash, timeagoConfig.Format(r.CommitMatch.Commit.Author.Date), r.Commit().URL())
	return Markdown(detail)
}

func (r *CommitSearchResultResolver) Matches() []*searchResultMatchResolver {
	match := &searchResultMatchResolver{
		body:       r.CommitMatch.Body.Value,
		highlights: r.CommitMatch.Body.Highlights,
		url:        r.Commit().URL(),
	}
	matches := []*searchResultMatchResolver{match}
	return matches
}

func (r *CommitSearchResultResolver) ToRepository() (*RepositoryResolver, bool) { return nil, false }
func (r *CommitSearchResultResolver) ToFileMatch() (*FileMatchResolver, bool)   { return nil, false }
func (r *CommitSearchResultResolver) ToCommitSearchResult() (*CommitSearchResultResolver, bool) {
	return r, true
}

func (r *CommitSearchResultResolver) ResultCount() int32 {
	return 1
}

func commitParametersToDiffParameters(ctx context.Context, op *search.CommitParameters) (*search.DiffParameters, error) {
	args := []string{
		"--no-prefix",
		"--max-count=" + strconv.Itoa(int(op.PatternInfo.FileMatchLimit)+1),
	}
	if op.Diff {
		args = append(args,
			"--unified=0",
		)
	}
	if op.PatternInfo.IsRegExp {
		args = append(args, "--extended-regexp")
	}
	if !op.Query.IsCaseSensitive() {
		args = append(args, "--regexp-ignore-case")
	}

	for _, rev := range op.RepoRevs.Revs {
		switch {
		case rev.RevSpec != "":
			if strings.HasPrefix(rev.RevSpec, "-") {
				// A revspec starting with "-" would be interpreted as a `git log` flag.
				// It would not be a security vulnerability because the flags are checked
				// against a allowlist, but it could cause unexpected errors by (e.g.)
				// changing the format of `git log` to a format that our parser doesn't
				// expect.
				return nil, fmt.Errorf("invalid revspec: %q", rev.RevSpec)
			}
			args = append(args, rev.RevSpec)

		case rev.RefGlob != "":
			args = append(args, "--glob="+rev.RefGlob)

		case rev.ExcludeRefGlob != "":
			args = append(args, "--exclude="+rev.ExcludeRefGlob)
		}
	}

	beforeValues, _ := op.Query.StringValues(query.FieldBefore)
	for _, s := range beforeValues {
		args = append(args, "--until="+s)
	}
	afterValues, _ := op.Query.StringValues(query.FieldAfter)
	for _, s := range afterValues {
		args = append(args, "--since="+s)
	}

	// Helper for adding git log flags --grep, --author, and --committer, which all behave similarly.
	var hasSeenGrepLikeFields, hasSeenInvertedGrepLikeFields bool
	addGrepLikeFlags := func(args *[]string, gitLogFlag string, field string, extraValues []string, expandUsernames bool) error {
		values, minusValues := op.Query.RegexpPatterns(field)
		values = append(values, extraValues...)

		if expandUsernames {
			var err error
			values, err = expandUsernamesToEmails(ctx, values)
			if err != nil {
				return errors.WithMessage(err, fmt.Sprintf("expanding usernames in field %s", field))
			}
			minusValues, err = expandUsernamesToEmails(ctx, minusValues)
			if err != nil {
				return errors.WithMessage(err, fmt.Sprintf("expanding usernames in field -%s", field))
			}
		}

		hasSeenGrepLikeFields = hasSeenGrepLikeFields || len(values) > 0
		hasSeenInvertedGrepLikeFields = hasSeenInvertedGrepLikeFields || len(minusValues) > 0

		if hasSeenGrepLikeFields && hasSeenInvertedGrepLikeFields {
			// TODO(sqs): this is a limitation of `git log` flags, but we could overcome this
			// with post-filtering
			return errors.New("query not supported: combining message:/author:/committer: and -message/-author:/-committer: filters")
		}
		if len(values) > 0 || len(minusValues) > 0 {
			// To be consistent with how other filters work, always treat additional
			// filters as further constraining the result set, not widening it.
			*args = append(*args, "--all-match")

			if len(minusValues) > 0 {
				*args = append(*args, "--invert-grep")
			}

			// Only one of these for-loops will have any values to iterate over.
			for _, s := range values {
				*args = append(*args, gitLogFlag+"="+s)
			}
			for _, s := range minusValues {
				*args = append(*args, gitLogFlag+"="+s)
			}
		}
		return nil
	}
	if err := addGrepLikeFlags(&args, "--grep", query.FieldMessage, op.ExtraMessageValues, false); err != nil {
		return nil, err
	}
	if err := addGrepLikeFlags(&args, "--author", query.FieldAuthor, nil, true); err != nil {
		return nil, err
	}
	if err := addGrepLikeFlags(&args, "--committer", query.FieldCommitter, nil, true); err != nil {
		return nil, err
	}

	textSearchOptions := git.TextSearchOptions{
		Pattern:         op.PatternInfo.Pattern,
		IsRegExp:        op.PatternInfo.IsRegExp,
		IsCaseSensitive: op.PatternInfo.IsCaseSensitive,
	}
	return &search.DiffParameters{
		Repo: op.RepoRevs.GitserverRepo(),
		Options: git.RawLogDiffSearchOptions{
			Query: textSearchOptions,
			Paths: git.PathOptions{
				IncludePatterns: op.PatternInfo.IncludePatterns,
				ExcludePattern:  op.PatternInfo.ExcludePattern,
				IsCaseSensitive: op.PatternInfo.PathPatternsAreCaseSensitive,
				IsRegExp:        op.PatternInfo.PathPatternsAreRegExps,
			},
			Diff:              op.Diff,
			OnlyMatchingHunks: true,
			Args:              args,
		},
	}, nil
}

type searchCommitsInRepoEvent struct {
	// Results are new commit results found.
	Results []*CommitSearchResultResolver

	// LimitHit is true if we stopped searching since we found FileMatchLimit
	// results.
	LimitHit bool

	// TimedOut is true when the results may have been parsed from only
	// partial output from the underlying git command (because, e.g., it timed
	// out during execution and only returned partial output).
	TimedOut bool

	// Error is non-nil if an error occurred. It will be the last event if
	// set.
	//
	// Note: Results will be empty if Error is set.
	Error error
}

// searchCommitsInRepoStream searchs for commits based on op.
//
// The returned channel must be read until closed, otherwise you may leak
// resources.
func searchCommitsInRepoStream(ctx context.Context, db dbutil.DB, op search.CommitParameters) chan searchCommitsInRepoEvent {
	c := make(chan searchCommitsInRepoEvent)
	go func() {
		defer close(c)
		_, _, _ = doSearchCommitsInRepoStream(ctx, db, op, c)
	}()

	return c
}

func doSearchCommitsInRepoStream(ctx context.Context, db dbutil.DB, op search.CommitParameters, c chan searchCommitsInRepoEvent) (limitHit, timedOut bool, err error) {
	resultCount := 0
	tr, ctx := trace.New(ctx, "searchCommitsInRepo", fmt.Sprintf("repoRevs: %v, pattern %+v", op.RepoRevs, op.PatternInfo))
	defer func() {
		tr.LazyPrintf("%d results, limitHit=%v, timedOut=%v", resultCount, limitHit, timedOut)
		tr.SetError(err)
		tr.Finish()
	}()

	// This defer will read the named return values. This is a convenient way
	// to send errors down the channel, since we only want to do this once.
	empty := true
	defer func() {
		// Send a final event if we had an error or if we hadn't sent down the
		// channel.
		if err != nil || empty {
			c <- searchCommitsInRepoEvent{
				LimitHit: limitHit,
				TimedOut: timedOut,
				Error:    err,
			}
		}
	}()

	diffParameters, err := commitParametersToDiffParameters(ctx, &op)
	if err != nil {
		return false, false, err
	}

	// Cancel context so we can stop RawLogDiffSearchOptions if we return
	// early.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start the commit search stream.
	events := git.RawLogDiffSearchStream(ctx, diffParameters.Repo, diffParameters.Options)

	// Ensure we drain events if we return early (limitHit or error).
	defer func() {
		cancel()
		for range events {
		}
	}()

	var repoName types.RepoName
	if op.RepoRevs.Repo != nil {
		repoName = *op.RepoRevs.Repo
	}

	for event := range events {
		// if the result is incomplete, git log timed out and the client
		// should be notified of that.
		timedOut = !event.Complete

		// Convert the results into resolvers and send them.
		results, err := logCommitSearchResultsToResolvers(ctx, db, &op, repoName, event.Results)
		if len(results) > 0 {
			empty = false
			resultCount += len(event.Results)
			limitHit = resultCount > int(op.PatternInfo.FileMatchLimit)
			c <- searchCommitsInRepoEvent{
				Results:  results,
				LimitHit: limitHit,
				TimedOut: timedOut,
			}
		}
		if err != nil {
			return limitHit, timedOut, err
		}

		// If we have hit the limit we stop (after we sent the above results).
		if limitHit {
			break
		}

		// If we have an error, stop and report it.
		if event.Error != nil {
			return limitHit, timedOut, event.Error
		}
	}

	return limitHit, timedOut, nil
}

func logCommitSearchResultsToResolvers(ctx context.Context, db dbutil.DB, op *search.CommitParameters, repoName types.RepoName, rawResults []*git.LogCommitSearchResult) ([]*CommitSearchResultResolver, error) {
	if len(rawResults) == 0 {
		return nil, nil
	}

	results := make([]*CommitSearchResultResolver, len(rawResults))
	for i, rawResult := range rawResults {
		commit := rawResult.Commit

		var (
			diffPreview     *result.HighlightedString
			messagePreview  *result.HighlightedString
			matchBody       string
			matchHighlights []result.HighlightedRange
		)
		// TODO(sqs): properly combine message: and term values for type:commit searches
		if !op.Diff {
			if len(op.ExtraMessageValues) > 0 {
				patString := orderedFuzzyRegexp(op.ExtraMessageValues)
				if !op.Query.IsCaseSensitive() {
					patString = "(?i:" + patString + ")"
				}
				pat, err := regexp.Compile(patString)
				if err == nil {
					messagePreview = highlightMatches(pat, []byte(commit.Message))
					matchHighlights = messagePreview.Highlights
				}
			} else {
				messagePreview = &result.HighlightedString{Value: string(commit.Message)}
			}
			matchBody = "```COMMIT_EDITMSG\n" + string(rawResult.Commit.Message) + "\n```"
		}

		if rawResult.Diff != nil && op.Diff {
			diffPreview = &result.HighlightedString{
				Value:      rawResult.Diff.Raw,
				Highlights: fromVCSHighlights(rawResult.DiffHighlights),
			}
			matchBody, matchHighlights = cleanDiffPreview(fromVCSHighlights(rawResult.DiffHighlights), rawResult.Diff.Raw)
		}

		results[i] = &CommitSearchResultResolver{
			db: db,
			CommitMatch: result.CommitMatch{
				Commit:         rawResult.Commit,
				Refs:           rawResult.Refs,
				SourceRefs:     rawResult.SourceRefs,
				MessagePreview: messagePreview,
				DiffPreview:    diffPreview,
				Body: result.HighlightedString{
					Value:      matchBody,
					Highlights: matchHighlights,
				},
				RepoName: repoName,
			},
		}
	}

	return results, nil
}

func cleanDiffPreview(highlights []result.HighlightedRange, rawDiffResult string) (string, []result.HighlightedRange) {
	// A map of line number to number of lines that have been ignored before the particular line number.
	lineByCountIgnored := make(map[int]int32)
	// The line numbers of lines that were ignored.
	ignoredLineNumbers := make(map[int]bool)

	lines := strings.Split(rawDiffResult, "\n")
	var finalLines []string
	ignoreUntilAtAt := false
	var countIgnored int32
	for i, line := range lines {
		// ignore index, ---file, and +++file lines
		if ignoreUntilAtAt && !strings.HasPrefix(line, "@@ ") {
			ignoredLineNumbers[i] = true
			countIgnored++
			continue
		} else {
			ignoreUntilAtAt = false
		}
		if strings.HasPrefix(line, "diff ") {
			ignoreUntilAtAt = true
			lineByCountIgnored[i] = countIgnored
			l := strings.Replace(line, "diff --git ", "", 1)
			finalLines = append(finalLines, l)
		} else {
			lineByCountIgnored[i] = countIgnored
			finalLines = append(finalLines, line)
		}
	}

	for n := range highlights {
		// For each highlight, adjust the line number by the number of lines that were
		// ignored in the diff before.
		linesIgnored := lineByCountIgnored[int(highlights[n].Line)]
		if ignoredLineNumbers[int(highlights[n].Line)-1] {
			// Effectively remove highlights that were on ignored lines by setting
			// line to -1.
			highlights[n].Line = -1
		}
		if linesIgnored > 0 {
			highlights[n].Line = highlights[n].Line - linesIgnored
		}
	}

	body := fmt.Sprintf("```diff\n%v```", strings.Join(finalLines, "\n"))
	return body, highlights
}

func displayRepoName(repoPath string) string {
	parts := strings.Split(repoPath, "/")
	if len(parts) >= 3 && strings.Contains(parts[0], ".") {
		parts = parts[1:] // remove hostname from repo path (reduce visual noise)
	}
	return strings.Join(parts, "/")
}

func highlightMatches(pattern *regexp.Regexp, data []byte) *result.HighlightedString {
	const maxMatchesPerLine = 25 // arbitrary

	var highlights []result.HighlightedRange
	for i, line := range bytes.Split(data, []byte("\n")) {
		for _, match := range pattern.FindAllIndex(line, maxMatchesPerLine) {
			highlights = append(highlights, result.HighlightedRange{
				Line:      int32(i + 1),
				Character: int32(utf8.RuneCount(line[:match[0]])),
				Length:    int32(utf8.RuneCount(line[:match[1]]) - utf8.RuneCount(line[:match[0]])),
			})
		}
	}
	return &result.HighlightedString{
		Value:      string(data),
		Highlights: highlights,
	}
}

// resolveCommitParameters creates parameters for commit search from tp. It
// will wait for the list of repos to be resolved.
func resolveCommitParameters(ctx context.Context, tp *search.TextParameters) (*search.TextParametersForCommitParameters, error) {
	old := tp.PatternInfo
	patternInfo := &search.CommitPatternInfo{
		Pattern:                      old.Pattern,
		IsRegExp:                     old.IsRegExp,
		IsCaseSensitive:              old.IsCaseSensitive,
		FileMatchLimit:               old.FileMatchLimit,
		IncludePatterns:              old.IncludePatterns,
		ExcludePattern:               old.ExcludePattern,
		PathPatternsAreRegExps:       true,
		PathPatternsAreCaseSensitive: old.PathPatternsAreCaseSensitive,
	}
	repos, err := getRepos(ctx, tp.RepoPromise)
	if err != nil {
		return nil, err
	}

	return &search.TextParametersForCommitParameters{
		PatternInfo: patternInfo,
		Repos:       repos,
		Query:       tp.Query,
	}, nil
}

type searchCommitsInReposParameters struct {
	TraceName string

	ErrorName string

	// CommitParams are the base commit parameters passed to
	// searchCommitsInRepoStream. For each repository revision this is copied
	// with the RepoRevs field set.
	CommitParams search.CommitParameters

	ResultChannel Sender
}

func searchCommitsInRepos(ctx context.Context, db dbutil.DB, args *search.TextParametersForCommitParameters, params searchCommitsInReposParameters) (err error) {
	tr, ctx := trace.New(ctx, params.TraceName, fmt.Sprintf("query: %+v, numRepoRevs: %d", args.PatternInfo, len(args.Repos)))
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	repoSearch := func(ctx context.Context, repoRev *search.RepositoryRevisions) (limitHit, timedOut bool, err error) {
		commitParams := params.CommitParams
		commitParams.RepoRevs = repoRev

		// We use the stream so we can optionally send down resultChannel.
		for event := range searchCommitsInRepoStream(ctx, db, commitParams) {
			if params.ResultChannel != nil {
				var stats streaming.Stats
				var status search.RepoStatus
				if event.LimitHit {
					stats.IsLimitHit = true
					status = status & search.RepoStatusLimitHit
				}
				if event.TimedOut {
					status = status & search.RepoStatusTimedout
				}
				// Only write if we have something to report back
				if len(event.Results) > 0 || status != 0 {
					stats.Status = search.RepoStatusSingleton(repoRev.Repo.ID, status)
					params.ResultChannel.Send(SearchEvent{
						Results: commitSearchResultsToSearchResults(event.Results),
						Stats:   stats,
					})
				}
			}

			limitHit = limitHit || event.LimitHit
			timedOut = timedOut || event.TimedOut
			if event.Error != nil {
				err = event.Error
			}
		}

		return
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, repoRev := range args.Repos {
		// Skip the repo if no revisions were resolved for it
		if len(repoRev.Revs) == 0 {
			continue
		}

		wg.Add(1)
		go func(repoRev *search.RepositoryRevisions) {
			defer wg.Done()
			repoLimitHit, repoTimedOut, searchErr := repoSearch(ctx, repoRev)
			if ctx.Err() == context.Canceled {
				// Our request has been canceled (either because another one of args.repos had a
				// fatal error, or otherwise), so we can just ignore these results.
				return
			}
			repoTimedOut = repoTimedOut || ctx.Err() == context.DeadlineExceeded
			if searchErr != nil {
				tr.LogFields(otlog.String("repo", string(repoRev.Repo.Name)), otlog.String("searchErr", searchErr.Error()), otlog.Bool("timeout", errcode.IsTimeout(searchErr)), otlog.Bool("temporary", errcode.IsTemporary(searchErr)))
			}
			mu.Lock()
			defer mu.Unlock()
			repoCommon, fatalErr := handleRepoSearchResult(repoRev, repoLimitHit, repoTimedOut, searchErr)
			if fatalErr != nil {
				err = errors.Wrapf(searchErr, "failed to search commit %s %s", params.ErrorName, repoRev.String())
				cancel()
			}
			params.ResultChannel.Send(SearchEvent{
				Stats: repoCommon,
			})
		}(repoRev)
	}
	wg.Wait()

	return err
}

// searchCommitDiffsInRepos searches a set of repos for matching commit diffs.
func searchCommitDiffsInRepos(ctx context.Context, db dbutil.DB, args *search.TextParametersForCommitParameters, resultChannel Sender) error {
	return searchCommitsInRepos(ctx, db, args, searchCommitsInReposParameters{
		TraceName:     "searchCommitDiffsInRepos",
		ErrorName:     "diffs",
		ResultChannel: resultChannel,
		CommitParams: search.CommitParameters{
			PatternInfo: args.PatternInfo,
			Query:       args.Query,
			Diff:        true,
		},
	})
}

// searchCommitLogInRepos searches a set of repos for matching commits.
func searchCommitLogInRepos(ctx context.Context, db dbutil.DB, args *search.TextParametersForCommitParameters, resultChannel Sender) error {
	var terms []string
	if args.PatternInfo.Pattern != "" {
		terms = append(terms, args.PatternInfo.Pattern)
	}

	return searchCommitsInRepos(ctx, db, args, searchCommitsInReposParameters{
		TraceName:     "searchCommitLogsInRepos",
		ErrorName:     "commits",
		ResultChannel: resultChannel,
		CommitParams: search.CommitParameters{
			PatternInfo:        args.PatternInfo,
			Query:              args.Query,
			Diff:               false,
			ExtraMessageValues: terms,
		},
	})
}

func commitSearchResultsToSearchResults(results []*CommitSearchResultResolver) []SearchResultResolver {
	if len(results) == 0 {
		return nil
	}

	// Show most recent commits first.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Commit().commit.Author.Date.After(results[j].Commit().commit.Author.Date)
	})

	results2 := make([]SearchResultResolver, len(results))
	for i, result := range results {
		results2[i] = result
	}
	return results2
}

// expandUsernamesToEmails expands references to usernames to mention all possible (known and
// verified) email addresses for the user.
//
// For example, given a list ["foo", "@alice"] where the user "alice" has 2 email addresses
// "alice@example.com" and "alice@example.org", it would return ["foo", "alice@example\\.com",
// "alice@example\\.org"].
func expandUsernamesToEmails(ctx context.Context, values []string) (expandedValues []string, err error) {
	expandOne := func(ctx context.Context, value string) ([]string, error) {
		if isPossibleUsernameReference := strings.HasPrefix(value, "@"); !isPossibleUsernameReference {
			return nil, nil
		}

		user, err := database.GlobalUsers.GetByUsername(ctx, strings.TrimPrefix(value, "@"))
		if errcode.IsNotFound(err) {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
		emails, err := database.GlobalUserEmails.ListByUser(ctx, database.UserEmailsListOptions{
			UserID: user.ID,
		})
		if err != nil {
			return nil, err
		}
		values := make([]string, 0, len(emails))
		for _, email := range emails {
			if email.VerifiedAt != nil {
				values = append(values, regexp.QuoteMeta(email.Email))
			}
		}
		return values, nil
	}

	expandedValues = make([]string, 0, len(values))
	for _, v := range values {
		x, err := expandOne(ctx, v)
		if err != nil {
			return nil, err
		}
		if x == nil {
			expandedValues = append(expandedValues, v) // not a username or couldn't expand
		} else {
			expandedValues = append(expandedValues, x...)
		}
	}
	return expandedValues, nil
}
