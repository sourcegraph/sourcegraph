package ranking

import (
	"context"
	"math"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/ranking/internal/lsifstore"
	internalshared "github.com/sourcegraph/sourcegraph/internal/codeintel/ranking/internal/shared"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/ranking/internal/store"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/ranking/shared"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/types"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/schema"
)

type Service struct {
	store      store.Store
	lsifstore  lsifstore.Store
	getConf    conftypes.SiteConfigQuerier
	operations *operations
	logger     log.Logger
}

func newService(
	observationCtx *observation.Context,
	store store.Store,
	lsifStore lsifstore.Store,
	getConf conftypes.SiteConfigQuerier,
) *Service {
	return &Service{
		store:      store,
		lsifstore:  lsifStore,
		getConf:    getConf,
		operations: newOperations(observationCtx),
		logger:     observationCtx.Logger,
	}
}

// GetRepoRank returns a rank vector for the given repository. Repositories are assumed to
// be ordered by each pairwise component of the resulting vector, higher ranks coming earlier.
// We currently rank first by user-defined scores, then by GitHub star count.
func (s *Service) GetRepoRank(ctx context.Context, repoName api.RepoName) (_ []float64, err error) {
	_, _, endObservation := s.operations.getRepoRank.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	userRank := repoRankFromConfig(s.getConf.SiteConfig(), string(repoName))

	starRank, err := s.store.GetStarRank(ctx, repoName)
	if err != nil {
		return nil, err
	}

	return []float64{squashRange(userRank), starRank}, nil
}

// copy pasta
// https://github.com/sourcegraph/sourcegraph/blob/942c417363b07c9e0a6377456f1d6a80a94efb99/cmd/frontend/internal/httpapi/search.go#L172
func repoRankFromConfig(siteConfig schema.SiteConfiguration, repoName string) float64 {
	val := 0.0
	if siteConfig.ExperimentalFeatures == nil || siteConfig.ExperimentalFeatures.Ranking == nil {
		return val
	}
	scores := siteConfig.ExperimentalFeatures.Ranking.RepoScores
	if len(scores) == 0 {
		return val
	}
	// try every "directory" in the repo name to assign it a value, so a repoName like
	// "github.com/sourcegraph/zoekt" will have "github.com", "github.com/sourcegraph",
	// and "github.com/sourcegraph/zoekt" tested.
	for i := range len(repoName) {
		if repoName[i] == '/' {
			val += scores[repoName[:i]]
		}
	}
	val += scores[repoName]
	return val
}

// squashRange maps a value in the range [0, inf) to a value in the range
// [0, 1) monotonically (i.e., (a < b) <-> (squashRange(a) < squashRange(b))).
func squashRange(j float64) float64 {
	return j / (1 + j)
}

// GetDocumentRank returns a map from paths within the given repo to their reference count.
func (s *Service) GetDocumentRanks(ctx context.Context, repoName api.RepoName) (_ types.RepoPathRanks, err error) {
	_, _, endObservation := s.operations.getDocumentRanks.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	documentRanks, ok, err := s.store.GetDocumentRanks(ctx, repoName)
	if err != nil {
		return types.RepoPathRanks{}, err
	}
	if !ok {
		return types.RepoPathRanks{}, nil
	}

	logmean, err := s.store.GetReferenceCountStatistics(ctx)
	if err != nil {
		return types.RepoPathRanks{}, err
	}

	paths := map[string]float64{}
	for path, rank := range documentRanks {
		if rank == 0 {
			paths[path] = 0
		} else {
			paths[path] = math.Log2(rank)
		}
	}

	return types.RepoPathRanks{
		MeanRank: logmean,
		Paths:    paths,
	}, nil
}

func (s *Service) Summaries(ctx context.Context) ([]shared.Summary, error) {
	return s.store.Summaries(ctx)
}

func (s *Service) DerivativeGraphKey(ctx context.Context) (string, bool, error) {
	derivativeGraphKeyPrefix, _, ok, err := s.store.DerivativeGraphKey(ctx)
	return internalshared.DerivativeGraphKeyFromPrefix(derivativeGraphKeyPrefix), ok, err
}

func (s *Service) BumpDerivativeGraphKey(ctx context.Context) error {
	return s.store.BumpDerivativeGraphKey(ctx)
}

func (s *Service) DeleteRankingProgress(ctx context.Context, graphKey string) error {
	return s.store.DeleteRankingProgress(ctx, graphKey)
}

func (s *Service) CoverageCounts(ctx context.Context, graphKey string) (shared.CoverageCounts, error) {
	return s.store.CoverageCounts(ctx, graphKey)
}

func (s *Service) LastUpdatedAt(ctx context.Context, repoIDs []api.RepoID) (map[api.RepoID]time.Time, error) {
	return s.store.LastUpdatedAt(ctx, repoIDs)
}

func (s *Service) NextJobStartsAt(ctx context.Context) (time.Time, bool, error) {
	expr, err := conf.CodeIntelRankingDocumentReferenceCountsCronExpression()
	if err != nil {
		return time.Time{}, false, err
	}

	_, previous, ok, err := s.store.DerivativeGraphKey(ctx)
	if err != nil {
		return time.Time{}, false, err
	}
	if !ok {
		return time.Time{}, false, nil
	}

	return expr.Next(previous), true, nil
}
