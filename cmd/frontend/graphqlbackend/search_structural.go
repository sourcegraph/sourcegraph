package graphqlbackend

import (
	"context"
	"regexp/syntax"
	"time"

	"github.com/google/zoekt"
	zoektquery "github.com/google/zoekt/query"

	searcherzoekt "github.com/sourcegraph/sourcegraph/cmd/searcher/search"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/comby"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	zoektutil "github.com/sourcegraph/sourcegraph/internal/search/zoekt"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func buildQuery(args *search.TextParameters, repos *indexedRepoRevs, filePathPatterns zoektquery.Q, shortcircuit bool) (zoektquery.Q, error) {
	regexString := comby.StructuralPatToRegexpQuery(args.PatternInfo.Pattern, shortcircuit)
	if len(regexString) == 0 {
		return &zoektquery.Const{Value: true}, nil
	}
	re, err := syntax.Parse(regexString, syntax.ClassNL|syntax.PerlX|syntax.UnicodeGroups)
	if err != nil {
		return nil, err
	}
	return zoektquery.NewAnd(
		&zoektquery.RepoBranches{Set: repos.repoBranches},
		filePathPatterns,
		&zoektquery.Regexp{
			Regexp:        re,
			CaseSensitive: true,
			Content:       true,
		},
	), nil
}

// zoektSearchHEADOnlyFiles searches repositories using zoekt, returning only the file paths containing
// content matching the given pattern.
//
// Timeouts are reported through the context, and as a special case errNoResultsInTimeout
// is returned if no results are found in the given timeout (instead of the more common
// case of finding partial or full results in the given timeout).
func zoektSearchHEADOnlyFiles(ctx context.Context, db dbutil.DB, args *search.TextParameters, repos *indexedRepoRevs, _ indexedRequestType, since func(t time.Time) time.Duration, c Sender) error {
	var (
		err       error
		limitHit  bool
		partial   map[api.RepoID]struct{}
		statusMap search.RepoStatusMap
	)

	if len(repos.repoRevs) == 0 {
		return nil
	}

	k := zoektutil.ResultCountFactor(len(repos.repoBranches), args.PatternInfo.FileMatchLimit, args.Mode == search.ZoektGlobalSearch)
	searchOpts := zoektutil.SearchOpts(ctx, k, args.PatternInfo)

	if args.UseFullDeadline {
		// If the user manually specified a timeout, allow zoekt to use all of the remaining timeout.
		deadline, _ := ctx.Deadline()
		searchOpts.MaxWallTime = time.Until(deadline)

		// We don't want our context's deadline to cut off zoekt so that we can get the results
		// found before the deadline.
		//
		// We'll create a new context that gets cancelled if the other context is cancelled for any
		// reason other than the deadline being exceeded. This essentially means the deadline for the new context
		// will be `deadline + time for zoekt to cancel + network latency`.
		var cancel context.CancelFunc
		ctx, cancel = contextWithoutDeadline(ctx)
		defer cancel()
	}

	filePathPatterns, err := searcherzoekt.HandleFilePathPatterns(args.PatternInfo)
	if err != nil {
		return err
	}

	t0 := time.Now()
	q, err := buildQuery(args, repos, filePathPatterns, true)
	if err != nil {
		return err
	}
	resp, err := args.Zoekt.Client.Search(ctx, q, &searchOpts)
	if err != nil {
		return err
	}

	mkStatusMap := func(mask search.RepoStatus) search.RepoStatusMap {
		var statusMap search.RepoStatusMap
		for _, r := range repos.repoRevs {
			statusMap.Update(r.Repo.ID, mask)
		}
		return statusMap
	}

	// Set all repos to "timed out"
	if since(t0) >= searchOpts.MaxWallTime {
		c.Send(SearchEvent{Stats: streaming.Stats{Status: mkStatusMap(search.RepoStatusTimedout)}})
	}

	// We always return approximate results (limitHit true) unless we run the branch to perform a more complete search.
	limitHit = true
	// If the previous indexed search did not return a substantial number of matching file candidates or count was
	// manually specified, run a more complete and expensive search.
	if resp.FileCount < 10 || args.PatternInfo.FileMatchLimit != defaultMaxSearchResults {
		q, err = buildQuery(args, repos, filePathPatterns, false)
		if err != nil {
			return err
		}
		resp, err = args.Zoekt.Client.Search(ctx, q, &searchOpts)
		if err != nil {
			return err
		}
		if since(t0) >= searchOpts.MaxWallTime {
			c.Send(SearchEvent{Stats: streaming.Stats{Status: mkStatusMap(search.RepoStatusTimedout)}})
		}
		// This is the only place limitHit can be set false, meaning we covered everything.
		limitHit = resp.FilesSkipped+resp.ShardsSkipped > 0
	}

	if len(resp.Files) == 0 {
		return nil
	}

	matchLimiter := zoektutil.MatchLimiter{Limit: int(args.PatternInfo.FileMatchLimit)}
	repoRevFunc := func(file *zoekt.FileMatch) (repo *types.RepoName, revs []string, ok bool) {
		repo, inputRevs := repos.GetRepoInputRev(file)
		return repo, inputRevs, true
	}

	var files []zoekt.FileMatch
	partial, files = matchLimiter.Slice(resp.Files, repoRevFunc)
	// Partial is populated with repositories we may have not fully
	// searched due to limits.
	for r := range partial {
		statusMap.Update(r, search.RepoStatusLimitHit)
	}

	limitHit = limitHit || len(partial) > 0
	resp.Files = files

	maxLineMatches := 25 + k
	matches := make([]SearchResultResolver, len(resp.Files))
	repoResolvers := make(RepositoryResolverCache)
	for i, file := range resp.Files {
		fileLimitHit := false
		if len(file.LineMatches) > maxLineMatches {
			file.LineMatches = file.LineMatches[:maxLineMatches]
			fileLimitHit = true
			limitHit = true
		}
		repoRev := repos.repoRevs[file.Repository]
		if repoResolvers[repoRev.Repo.Name] == nil {
			repoResolvers[repoRev.Repo.Name] = NewRepositoryResolver(db, repoRev.Repo.ToRepo())
		}
		matches[i] = &FileMatchResolver{
			db: db,
			FileMatch: result.FileMatch{
				Path:     file.FileName,
				LimitHit: fileLimitHit,
				URI:      fileMatchURI(repoRev.Repo.Name, "", file.FileName),
				Repo:     repoRev.Repo,
				CommitID: api.CommitID(file.Version),
			},
			RepoResolver: repoResolvers[repoRev.Repo.Name],
		}
	}

	c.Send(SearchEvent{
		Results: matches,
		Stats: streaming.Stats{
			Status:     statusMap,
			IsLimitHit: limitHit,
		},
	})

	return nil
}
