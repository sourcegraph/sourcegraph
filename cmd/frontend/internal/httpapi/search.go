package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/zoekt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	proto "github.com/sourcegraph/zoekt/cmd/zoekt-sourcegraph-indexserver/protos/sourcegraph/zoekt/configuration/v1"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/internal/api"
	citypes "github.com/sourcegraph/sourcegraph/internal/codeintel/types"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	searchbackend "github.com/sourcegraph/sourcegraph/internal/search/backend"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

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

type searchIndexerGRPCServer struct {
	server *searchIndexerServer
	proto.ZoektConfigurationServiceServer
}

func (s *searchIndexerGRPCServer) SearchConfiguration(ctx context.Context, request *proto.SearchConfigurationRequest) (*proto.SearchConfigurationResponse, error) {
	repoIDs := make([]api.RepoID, 0, len(request.GetRepoIds()))
	for _, repoID := range request.GetRepoIds() {
		repoIDs = append(repoIDs, api.RepoID(repoID))
	}

	var fingerprint searchbackend.ConfigFingerprint
	fingerprint.FromProto(request.GetFingerprint())

	parameters := searchConfigurationParameters{
		fingerprint: fingerprint,
		repoIDs:     repoIDs,
	}

	r, err := s.server.doSearchConfiguration(ctx, parameters)
	if err != nil {
		var parameterErr *parameterError
		if errors.As(err, &parameterErr) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		return nil, err
	}

	options := make([]*proto.ZoektIndexOptions, 0, len(r.options))
	for _, o := range r.options {
		options = append(options, o.ToProto())
	}

	return &proto.SearchConfigurationResponse{
		UpdatedOptions: options,
		Fingerprint:    r.fingerprint.ToProto(),
	}, nil
}

func (s *searchIndexerGRPCServer) List(ctx context.Context, r *proto.ListRequest) (*proto.ListResponse, error) {
	indexedIDs := make([]api.RepoID, 0, len(r.GetIndexedIds()))
	for _, repoID := range r.GetIndexedIds() {
		indexedIDs = append(indexedIDs, api.RepoID(repoID))
	}

	var parameters listParameters
	parameters.IndexedIDs = indexedIDs
	parameters.Hostname = r.GetHostname()

	repoIDs, err := s.server.doList(ctx, &parameters)
	if err != nil {
		return nil, err
	}

	var response proto.ListResponse
	response.RepoIds = make([]int32, 0, len(repoIDs))
	for _, repoID := range repoIDs {
		response.RepoIds = append(response.RepoIds, int32(repoID))
	}

	return &response, nil
}

func (s *searchIndexerGRPCServer) DocumentRanks(ctx context.Context, request *proto.DocumentRanksRequest) (*proto.DocumentRanksResponse, error) {
	ranks, err := s.server.Ranking.GetDocumentRanks(ctx, api.RepoName(request.Repository))
	if err != nil {
		if errcode.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, err.Error())
		}

		return nil, err
	}

	return repoPathRanksToProto(&ranks), nil
}

func (s *searchIndexerGRPCServer) UpdateIndexStatus(ctx context.Context, req *proto.UpdateIndexStatusRequest) (*proto.UpdateIndexStatusResponse, error) {
	var request indexStatusUpdateArgs
	request.FromProto(req)

	err := s.server.doIndexStatusUpdate(ctx, &request)
	if err != nil {
		return nil, err
	}

	return &proto.UpdateIndexStatusResponse{}, nil
}

var _ proto.ZoektConfigurationServiceServer = &searchIndexerGRPCServer{}

// searchIndexerServer has handlers that zoekt-sourcegraph-indexserver
// interacts with (search-indexer).
type searchIndexerServer struct {
	db     database.DB
	logger log.Logger

	gitserverClient gitserver.Client
	// ListIndexable returns the repositories to index.
	ListIndexable func(context.Context) ([]types.MinimalRepo, error)

	// RepoStore is a subset of database.RepoStore used by searchIndexerServer.
	RepoStore interface {
		List(context.Context, database.ReposListOptions) ([]*types.Repo, error)
		StreamMinimalRepos(context.Context, database.ReposListOptions, func(*types.MinimalRepo)) error
	}

	SearchContextsRepoRevs func(context.Context, []api.RepoID) (map[api.RepoID][]string, error)

	// Indexers is the subset of searchbackend.Indexers methods we
	// use. reposListServer is used by indexed-search to get the list of
	// repositories to index. These methods are used to return the correct
	// subset for horizontal indexed search. Declared as an interface for
	// testing.
	Indexers interface {
		// ReposSubset returns the subset of repoNames that hostname should
		// index.
		ReposSubset(ctx context.Context, hostname string, indexed zoekt.ReposMap, indexable []types.MinimalRepo) ([]types.MinimalRepo, error)
		// Enabled is true if horizontal indexed search is enabled.
		Enabled() bool
	}

	// Ranking is a service that provides ranking scores for various code objects.
	Ranking enterprise.RankingService

	// MinLastChangedDisabled is a feature flag for disabling more efficient
	// polling by zoekt. This can be removed after v3.34 is cut (Dec 2021).
	MinLastChangedDisabled bool
}

func (h *searchIndexerServer) doSearchConfiguration(ctx context.Context, parameters searchConfigurationParameters) (*searchConfigurationResponse, error) {
	siteConfig := conf.Get().SiteConfiguration

	if len(parameters.repoIDs) == 0 {
		return nil, &parameterError{err: "at least one repoID required"}
	}

	var minLastChanged time.Time
	nextFingerPrint := parameters.fingerprint
	if !h.MinLastChangedDisabled {
		var err error
		fp, err := searchbackend.NewConfigFingerprint(&siteConfig)
		if err != nil {
			return nil, err
		}

		minLastChanged = parameters.fingerprint.ChangesSince(fp)
		nextFingerPrint = *fp
	}

	// Preload repos to support fast lookups by repo ID.
	repos, loadReposErr := h.RepoStore.List(ctx, database.ReposListOptions{
		IDs: parameters.repoIDs,
		// When minLastChanged is non-zero we will only return the
		// repositories that have changed since minLastChanged. This takes
		// into account repo metadata, repo content and search context
		// changes.
		MinLastChanged: minLastChanged,
		// Not needed here and expensive to compute for so many repos.
		ExcludeSources: true,
	})
	reposMap := make(map[api.RepoID]*types.Repo, len(repos))
	for _, repo := range repos {
		reposMap[repo.ID] = repo
	}

	// If we used MinLastChanged, we should only return information for the
	// repositories that we found from List.
	if !minLastChanged.IsZero() {
		filtered := parameters.repoIDs[:0]
		for _, id := range parameters.repoIDs {
			if _, ok := reposMap[id]; ok {
				filtered = append(filtered, id)
			}
		}
		parameters.repoIDs = filtered
	}

	rankingLastUpdatedAt, err := h.Ranking.LastUpdatedAt(ctx, parameters.repoIDs)
	if err != nil {
		h.logger.Warn("failed to get ranking last updated timestamps, falling back to no ranking",
			log.Int("repos", len(parameters.repoIDs)),
			log.Error(err),
		)
		rankingLastUpdatedAt = make(map[api.RepoID]time.Time)
	}

	getRepoIndexOptions := func(repoID api.RepoID) (*searchbackend.RepoIndexOptions, error) {
		if loadReposErr != nil {
			return nil, loadReposErr
		}
		// Replicate what database.Repos.GetByName would do here:
		repo, ok := reposMap[repoID]
		if !ok {
			return nil, &database.RepoNotFoundErr{ID: repoID}
		}

		getVersion := func(branch string) (string, error) {
			metricGetVersion.Inc()
			// Do not to trigger a repo-updater lookup since this is a batch job.
			commitID, err := h.gitserverClient.ResolveRevision(ctx, repo.Name, branch, gitserver.ResolveRevisionOptions{
				NoEnsureRevision: true,
			})
			if err != nil && errcode.HTTP(err) == http.StatusNotFound {
				// GetIndexOptions wants an empty rev for a missing rev or empty
				// repo.
				return "", nil
			}
			return string(commitID), err
		}

		priority := float64(repo.Stars) + repoRankFromConfig(siteConfig, string(repo.Name))

		var documentRanksVersion string
		if t, ok := rankingLastUpdatedAt[repoID]; ok {
			documentRanksVersion = t.String()
		}

		return &searchbackend.RepoIndexOptions{
			Name:       string(repo.Name),
			RepoID:     repo.ID,
			Public:     !repo.Private,
			Priority:   priority,
			Fork:       repo.Fork,
			Archived:   repo.Archived,
			GetVersion: getVersion,

			DocumentRanksVersion: documentRanksVersion,
		}, nil
	}

	revisionsForRepo, revisionsForRepoErr := h.SearchContextsRepoRevs(ctx, parameters.repoIDs)
	getSearchContextRevisions := func(repoID api.RepoID) ([]string, error) {
		if revisionsForRepoErr != nil {
			return nil, revisionsForRepoErr
		}
		return revisionsForRepo[repoID], nil
	}

	indexOptions := searchbackend.GetIndexOptions(
		h.logger,
		&siteConfig,
		getRepoIndexOptions,
		getSearchContextRevisions,
		parameters.repoIDs...,
	)

	return &searchConfigurationResponse{
		options:     indexOptions,
		fingerprint: nextFingerPrint,
	}, nil
}

type parameterError struct {
	err string
}

func (e *parameterError) Error() string { return e.err }

type searchConfigurationParameters struct {
	repoIDs     []api.RepoID
	fingerprint searchbackend.ConfigFingerprint
}

type searchConfigurationResponse struct {
	options     []searchbackend.ZoektIndexOptions
	fingerprint searchbackend.ConfigFingerprint
}

func (h *searchIndexerServer) doList(ctx context.Context, parameters *listParameters) (repoIDS []api.RepoID, err error) {
	indexable, err := h.ListIndexable(ctx)
	if err != nil {
		return nil, err
	}

	if h.Indexers.Enabled() {
		indexed := make(zoekt.ReposMap, len(parameters.IndexedIDs))
		add := func(r *types.MinimalRepo) { indexed[uint32(r.ID)] = zoekt.MinimalRepoListEntry{} }
		if len(parameters.IndexedIDs) > 0 {
			opts := database.ReposListOptions{IDs: parameters.IndexedIDs}
			err = h.RepoStore.StreamMinimalRepos(ctx, opts, add)
			if err != nil {
				return nil, err
			}
		}

		indexable, err = h.Indexers.ReposSubset(ctx, parameters.Hostname, indexed, indexable)
		if err != nil {
			return nil, err
		}
	}

	// TODO: Avoid batching up so much in memory by:
	// 1. Changing the schema from object of arrays to array of objects.
	// 2. Stream out each object marshalled rather than marshall the full list in memory.

	ids := make([]api.RepoID, 0, len(indexable))
	for _, r := range indexable {
		ids = append(ids, r.ID)
	}

	return ids, nil
}

type listParameters struct {
	// Hostname is used to determine the subset of repos to return
	Hostname string
	// IndexedIDs are the repository IDs of indexed repos by Hostname.
	IndexedIDs []api.RepoID
}

var metricGetVersion = promauto.NewCounter(prometheus.CounterOpts{
	Name: "src_search_get_version_total",
	Help: "The total number of times we poll gitserver for the version of a indexable branch.",
})

func (h *searchIndexerServer) doIndexStatusUpdate(ctx context.Context, args *indexStatusUpdateArgs) error {
	var (
		ids     = make([]int32, len(args.Repositories))
		minimal = make(zoekt.ReposMap, len(args.Repositories))
	)

	for i, repo := range args.Repositories {
		ids[i] = int32(repo.RepoID)
		minimal[repo.RepoID] = zoekt.MinimalRepoListEntry{Branches: repo.Branches, IndexTimeUnix: repo.IndexTimeUnix}
	}

	h.logger.Info("updating index status", log.Int32s("repositories", ids))
	return h.db.ZoektRepos().UpdateIndexStatuses(ctx, minimal)
}

type indexStatusUpdateArgs struct {
	Repositories []indexStatusUpdateRepository
}

type indexStatusUpdateRepository struct {
	RepoID        uint32
	Branches      []zoekt.RepositoryBranch
	IndexTimeUnix int64
}

func (a *indexStatusUpdateArgs) FromProto(req *proto.UpdateIndexStatusRequest) {
	a.Repositories = make([]indexStatusUpdateRepository, 0, len(req.Repositories))

	for _, repo := range req.Repositories {
		branches := make([]zoekt.RepositoryBranch, 0, len(repo.Branches))
		for _, b := range repo.Branches {
			branches = append(branches, zoekt.RepositoryBranch{
				Name:    b.Name,
				Version: b.Version,
			})
		}

		a.Repositories = append(a.Repositories, indexStatusUpdateRepository{
			RepoID:        repo.RepoId,
			Branches:      branches,
			IndexTimeUnix: repo.GetIndexTimeUnix(),
		})
	}
}

func (a *indexStatusUpdateArgs) ToProto() *proto.UpdateIndexStatusRequest {
	repos := make([]*proto.UpdateIndexStatusRequest_Repository, 0, len(a.Repositories))

	for _, repo := range a.Repositories {
		branches := make([]*proto.ZoektRepositoryBranch, 0, len(repo.Branches))
		for _, b := range repo.Branches {
			branches = append(branches, &proto.ZoektRepositoryBranch{
				Name:    b.Name,
				Version: b.Version,
			})
		}

		repos = append(repos, &proto.UpdateIndexStatusRequest_Repository{
			RepoId:        repo.RepoID,
			Branches:      branches,
			IndexTimeUnix: repo.IndexTimeUnix,
		})
	}

	return &proto.UpdateIndexStatusRequest{
		Repositories: repos,
	}
}

func repoPathRanksToProto(r *citypes.RepoPathRanks) *proto.DocumentRanksResponse {
	paths := make(map[string]float64, len(r.Paths))
	for path, counts := range r.Paths {
		paths[path] = counts
	}

	return &proto.DocumentRanksResponse{
		Paths:    paths,
		MeanRank: r.MeanRank,
	}
}

func repoPathRanksFromProto(x *proto.DocumentRanksResponse) *citypes.RepoPathRanks {
	protoPaths := x.GetPaths()

	paths := make(map[string]float64, len(protoPaths))
	for path, counts := range protoPaths {
		paths[path] = counts
	}

	return &citypes.RepoPathRanks{
		Paths:    paths,
		MeanRank: x.MeanRank,
	}
}
