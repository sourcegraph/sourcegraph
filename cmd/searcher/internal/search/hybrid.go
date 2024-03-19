package search

import (
	"bytes"
	"context"
	"regexp/syntax" //nolint:depguard // using the grafana fork of regexp clashes with zoekt, which uses the std regexp/syntax.
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/zoekt"
	zoektquery "github.com/sourcegraph/zoekt/query"

	"github.com/sourcegraph/sourcegraph/cmd/searcher/diff"
	"github.com/sourcegraph/sourcegraph/cmd/searcher/protocol"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	metricHybridRetry = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "searcher_hybrid_retry_total",
		Help: "Total number of times we retry zoekt indexed search for hybrid search.",
	}, []string{"reason"})
	metricHybridFinalState = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "searcher_hybrid_final_state_total",
		Help: "Total number of times a hybrid search ended in a specific state.",
	}, []string{"state"})
)

// hybrid search is a feature which will search zoekt only for the paths that
// are the same for p.Commit. unsearched is the paths that searcher needs to
// search on p.Commit. If ok is false, then the zoekt search failed in a way
// where we should fallback to a normal unindexed search on the whole commit.
//
// This only interacts with zoekt so that we can leverage the normal searcher
// code paths for the unindexed parts. IE unsearched is expected to be used to
// fetch a zip via the store and then do a normal unindexed search.
func (s *Service) hybrid(ctx context.Context, rootLogger log.Logger, p *protocol.Request, sender matchSender) (unsearched []string, ok bool, err error) {
	// recordHybridFinalState is a wrapper around metricHybridState to make the
	// callsites more succinct.
	finalState := "unknown"
	recordHybridFinalState := func(state string) {
		finalState = state
	}

	// We call out to external services in several places, and in each case
	// the most common error condition for those is searcher cancelling the
	// request. As such we centralize our observability to always take into
	// account the state of the ctx.
	defer func() {
		// We can downgrade error logs to rootLogger.Warn
		errorLogger := rootLogger.Error

		if err != nil {
			switch ctx.Err() {
			case context.Canceled:
				// We swallow the error since we only cancel requests once we
				// have hit limits or the RPC request has gone away.
				recordHybridFinalState("search-canceled")
				unsearched, ok, err = nil, true, nil
			case context.DeadlineExceeded:
				// We return the error because hitting a deadline should be
				// unexpected. We also don't need to run the normal searcher
				// path in this case.
				recordHybridFinalState("search-timeout")
				unsearched, ok = nil, true
				errorLogger = rootLogger.Warn
			}
		}

		if err != nil {
			errorLogger("hybrid search failed", log.String("state", finalState), log.Error(err))
		} else {
			rootLogger.Debug("hybrid search done", log.String("state", finalState), log.Bool("ok", ok), log.Int("unsearched.len", len(unsearched)))
		}
		metricHybridFinalState.WithLabelValues(finalState).Inc()
	}()

	client := s.Indexed

	// There is a race condition between asking zoekt what is indexed vs
	// actually searching since the index may update. If the index changes,
	// which files we search need to change. As such we keep retrying until we
	// know we have had a consistent list and search on zoekt.
	for try := range 5 {
		logger := rootLogger.With(log.Int("try", try))

		indexed, ok, err := zoektIndexedCommit(ctx, client, p.Repo)
		if err != nil {
			recordHybridFinalState("zoekt-list-error")
			return nil, false, errors.Wrapf(err, "failed to list indexed commits for %s", p.Repo)
		}
		if !ok {
			logger.Debug("failed to find indexed commit")
			recordHybridFinalState("zoekt-list-missing")
			return nil, false, nil
		}
		logger = logger.With(log.String("indexed", string(indexed)))

		// TODO if our store was more flexible we could cache just based on
		// indexed and p.Commit and avoid the need of running diff for each
		// search.
		out, err := s.GitDiffSymbols(ctx, p.Repo, indexed, p.Commit)
		if errcode.IsNotFound(err) {
			recordHybridFinalState("git-diff-not-found")
			logger.Debug("not doing hybrid search due to likely missing indexed commit on gitserver", log.Error(err))
			return nil, false, nil
		} else if err != nil {
			recordHybridFinalState("git-diff-error")
			return nil, false, errors.Wrapf(err, "failed to find changed files in %s between %s and %s", p.Repo, indexed, p.Commit)
		}

		indexedIgnore, unindexedSearch, err := diff.ParseGitDiffNameStatus(out)
		if err != nil {
			logger.Debug("parseGitDiffNameStatus failed",
				log.Binary("out", out),
				log.Error(err))
			recordHybridFinalState("git-diff-parse-error")
			return nil, false, errors.Wrapf(err, "failed to parse git diff output of changed files in %s between %s and %s", p.Repo, indexed, p.Commit)
		}

		totalLenIndexedIgnore := totalStringsLen(indexedIgnore)
		totalLenUnindexedSearch := totalStringsLen(unindexedSearch)

		logger = logger.With(
			log.Int("indexedIgnorePaths", len(indexedIgnore)),
			log.Int("totalLenIndexedIgnorePaths", totalLenIndexedIgnore),
			log.Int("unindexedSearchPaths", len(unindexedSearch)),
			log.Int("totalLenUnindexedSearchPaths", totalLenUnindexedSearch))

		if totalLenIndexedIgnore > s.MaxTotalPathsLength || totalLenUnindexedSearch > s.MaxTotalPathsLength {
			logger.Debug("not doing hybrid search due to changed file list exceeding MAX_TOTAL_PATHS_LENGTH",
				log.Int("MAX_TOTAL_PATHS_LENGTH", s.MaxTotalPathsLength))
			recordHybridFinalState("diff-too-large")
			return nil, false, nil
		}

		logger.Debug("starting zoekt search")

		retryReason, err := zoektSearchIgnorePaths(ctx, client, p, sender, indexed, indexedIgnore)
		if err != nil {
			recordHybridFinalState("zoekt-search-error")
			return nil, false, errors.Wrapf(err, "failed to search indexed commit %s@%s", p.Repo, indexed)
		} else if retryReason != "" {
			metricHybridRetry.WithLabelValues(retryReason).Inc()
			logger.Debug("retrying search since index changed while searching", log.String("retryReason", retryReason))
			continue
		}

		recordHybridFinalState("success")
		return unindexedSearch, true, nil
	}

	rootLogger.Warn("reached maximum try count, falling back to default unindexed search")
	recordHybridFinalState("max-retrys")
	return nil, false, nil
}

// zoektSearchIgnorePaths will execute the search for p on zoekt and stream
// out results via sender. It will not search paths listed under ignoredPaths.
//
// If we did not search the correct commit or we don't know if we did, a
// non-empty retryReason is returned.
func zoektSearchIgnorePaths(ctx context.Context, client zoekt.Streamer, p *protocol.Request, sender matchSender, indexed api.CommitID, ignoredPaths []string) (retryReason string, err error) {
	qText, err := zoektCompile(&p.PatternInfo)
	if err != nil {
		return "", errors.Wrap(err, "failed to compile query for zoekt")
	}
	q := zoektquery.Simplify(zoektquery.NewAnd(
		zoektquery.NewSingleBranchesRepos("HEAD", uint32(p.RepoID)),
		qText,
		&zoektquery.Not{Child: zoektquery.NewFileNameSet(ignoredPaths...)},
	))

	opts := (&search.ZoektParameters{
		FileMatchLimit:  int32(p.Limit),
		NumContextLines: int(p.NumContextLines),
	}).ToSearchOptions(ctx)
	if deadline, ok := ctx.Deadline(); ok {
		opts.MaxWallTime = time.Until(deadline) - 100*time.Millisecond
	}

	// We only support chunk matches below.
	opts.ChunkMatches = true

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// We need to keep track of extra state to ensure we searched the correct
	// commit (there is a race between List and Search). We can only tell if
	// we searched the correct commit if we had a result since that contains
	// the commit searched.
	var wrongCommit, foundResults bool
	var crashes int

	err = client.StreamSearch(ctx, q, opts, senderFunc(func(res *zoekt.SearchResult) {
		crashes += res.Crashes
		for _, fm := range res.Files {
			// Unexpected commit searched, signal to retry.
			if fm.Version != string(indexed) {
				wrongCommit = true
				cancel()
				return
			}

			foundResults = true

			sender.Send(protocol.FileMatch{
				Path:         fm.FileName,
				Language:     fm.Language,
				ChunkMatches: zoektChunkMatches(fm.ChunkMatches),
			})
		}
	}))
	// we check wrongCommit first since that overrides err (especially since
	// err is likely context.Cancel when we want to retry)
	if wrongCommit {
		return "index-search-changed", nil
	}
	if err != nil {
		return "", err
	}

	// We found results and we got past wrongCommit, so we know what we have
	// streamed back is correct.
	if foundResults {
		return "", nil
	}

	// The zoekt containing the repo may have been unreachable, so we are
	// conservative and treat any backend being down as a reason to retry.
	if crashes > 0 {
		return "index-search-missing", nil
	}

	// we have no matches, so we don't know if we searched the correct commit
	newIndexed, ok, err := zoektIndexedCommit(ctx, client, p.Repo)
	if err != nil {
		return "", errors.Wrap(err, "failed to double check indexed commit")
	}
	if !ok {
		// let the retry logic handle the call to zoektIndexedCommit again
		return "index-list-missing", nil
	}
	if newIndexed != indexed {
		return "index-list-changed", nil
	}
	return "", nil
}

// zoektCompile builds a text search zoekt query for p.
//
// This function should support the same features as the "compile" function,
// but return a zoektquery instead of a regexMatchTree.
//
// Note: This is used by hybrid search and not structural search.
func zoektCompile(p *protocol.PatternInfo) (zoektquery.Q, error) {
	var parts []zoektquery.Q
	// we are redoing work here, but ensures we generate the same regex and it
	// feels nicer than passing in a regexMatchTree since handle path directly.
	if m, err := toMatchTree(p.Query, p.IsCaseSensitive); err != nil {
		return nil, err
	} else {
		q, err := m.ToZoektQuery(p.PatternMatchesContent, p.PatternMatchesPath)
		if err != nil {
			return nil, err
		}
		parts = append(parts, q)
	}

	for _, pat := range p.IncludePaths {
		re, err := syntax.Parse(pat, syntax.Perl)
		if err != nil {
			return nil, err
		}
		parts = append(parts, &zoektquery.Regexp{
			Regexp:        re,
			FileName:      true,
			CaseSensitive: p.PathPatternsAreCaseSensitive,
		})
	}

	if p.ExcludePaths != "" {
		re, err := syntax.Parse(p.ExcludePaths, syntax.Perl)
		if err != nil {
			return nil, err
		}
		parts = append(parts, &zoektquery.Not{Child: &zoektquery.Regexp{
			Regexp:        re,
			FileName:      true,
			CaseSensitive: p.PathPatternsAreCaseSensitive,
		}})
	}

	for _, lang := range p.IncludeLangs {
		parts = append(parts, &zoektquery.Language{
			Language: lang,
		})
	}

	for _, lang := range p.ExcludeLangs {
		parts = append(parts, &zoektquery.Not{Child: &zoektquery.Language{
			Language: lang,
		}})
	}

	return zoektquery.Simplify(zoektquery.NewAnd(parts...)), nil
}

// zoektIndexedCommit returns the default indexed commit for a repository.
func zoektIndexedCommit(ctx context.Context, client zoekt.Streamer, repo api.RepoName) (api.CommitID, bool, error) {
	// TODO check we are using the most efficient way to List. I tested with
	// NewSingleBranchesRepos and it went through a slow path.
	q := zoektquery.NewRepoSet(string(repo))

	resp, err := client.List(ctx, q, &zoekt.ListOptions{Field: zoekt.RepoListFieldReposMap})
	if err != nil {
		return "", false, err
	}

	for _, v := range resp.ReposMap {
		return api.CommitID(v.Branches[0].Version), true, nil
	}

	return "", false, nil
}

func zoektChunkMatches(chunkMatches []zoekt.ChunkMatch) []protocol.ChunkMatch {
	cms := make([]protocol.ChunkMatch, 0, len(chunkMatches))
	for _, cm := range chunkMatches {
		if cm.FileName {
			continue
		}

		ranges := make([]protocol.Range, 0, len(cm.Ranges))
		for _, r := range cm.Ranges {
			ranges = append(ranges, protocol.Range{
				Start: protocol.Location{
					Offset: int32(r.Start.ByteOffset),
					Line:   int32(r.Start.LineNumber - 1),
					Column: int32(r.Start.Column - 1),
				},
				End: protocol.Location{
					Offset: int32(r.End.ByteOffset),
					Line:   int32(r.End.LineNumber - 1),
					Column: int32(r.End.Column - 1),
				},
			})
		}

		cms = append(cms, protocol.ChunkMatch{
			Content: string(bytes.ToValidUTF8(cm.Content, []byte("�"))),
			ContentStart: protocol.Location{
				Offset: int32(cm.ContentStart.ByteOffset),
				Line:   int32(cm.ContentStart.LineNumber) - 1,
				Column: int32(cm.ContentStart.Column) - 1,
			},
			Ranges: ranges,
		})
	}
	return cms
}

type senderFunc func(result *zoekt.SearchResult)

func (f senderFunc) Send(result *zoekt.SearchResult) {
	f(result)
}

func totalStringsLen(ss []string) int {
	sum := 0
	for _, s := range ss {
		sum += len(s)
	}
	return sum
}

// logWithTrace is a helper which returns l.WithTrace if there is a
// TraceContext associated with ctx.
func logWithTrace(ctx context.Context, l log.Logger) log.Logger {
	return l.WithTrace(trace.Context(ctx))
}
