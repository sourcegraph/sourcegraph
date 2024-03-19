package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches/service"
	"github.com/sourcegraph/sourcegraph/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/batches/store/author"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	batcheslib "github.com/sourcegraph/sourcegraph/lib/batches"
	"github.com/sourcegraph/sourcegraph/lib/batches/execution"
	"github.com/sourcegraph/sourcegraph/lib/batches/execution/cache"
	"github.com/sourcegraph/sourcegraph/lib/batches/template"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// batchSpecWorkspaceCreator takes in BatchSpecs, resolves them into
// RepoWorkspaces and then persists those as pending BatchSpecWorkspaces.
type batchSpecWorkspaceCreator struct {
	store  *store.Store
	logger log.Logger
}

// HandlerFunc returns a workerutil.HandlerFunc that can be passed to a
// workerutil.Worker to process queued changesets.
func (r *batchSpecWorkspaceCreator) HandlerFunc() workerutil.HandlerFunc[*btypes.BatchSpecResolutionJob] {
	return func(ctx context.Context, logger log.Logger, job *btypes.BatchSpecResolutionJob) (err error) {
		// Run the resolution job as the user, so that only secrets and workspaces
		// that are visible to the user are returned.
		ctx = actor.WithActor(ctx, actor.FromUser(job.InitiatorID))

		return r.process(ctx, service.NewWorkspaceResolver, job)
	}
}

type stepCacheKey struct {
	index int
	key   string
}

type workspaceCacheKey struct {
	dbWorkspace   *btypes.BatchSpecWorkspace
	repo          batcheslib.Repository
	stepCacheKeys []stepCacheKey
	skippedSteps  map[int]struct{}
}

// process runs one workspace creation run for the given job utilizing the given
// workspace resolver to find the workspaces. It creates a database transaction
// to store all the entities in one transaction after the resolution process,
// to prevent long running transactions.
func (r *batchSpecWorkspaceCreator) process(
	ctx context.Context,
	newResolver service.WorkspaceResolverBuilder,
	job *btypes.BatchSpecResolutionJob,
) error {
	spec, err := r.store.GetBatchSpec(ctx, store.GetBatchSpecOpts{ID: job.BatchSpecID})
	if err != nil {
		return err
	}

	evaluatableSpec, err := batcheslib.ParseBatchSpec([]byte(spec.RawSpec))
	if err != nil {
		return err
	}

	// Next, we fetch all secrets that are requested by the spec.
	rk := spec.Spec.RequiredEnvVars()
	var secrets []*database.ExecutorSecret
	if len(rk) > 0 {
		esStore := r.store.DatabaseDB().ExecutorSecrets(keyring.Default().ExecutorSecretKey)
		secrets, _, err = esStore.List(ctx, database.ExecutorSecretScopeBatches, database.ExecutorSecretsListOpts{
			NamespaceUserID: spec.NamespaceUserID,
			NamespaceOrgID:  spec.NamespaceOrgID,
			Keys:            rk,
		})
		if err != nil {
			return errors.Wrap(err, "fetching secrets")
		}
	}

	esalStore := r.store.DatabaseDB().ExecutorSecretAccessLogs()
	envVars := make([]string, len(secrets))
	for i, secret := range secrets {
		// This will create an audit log event in the name of the initiating user.
		val, err := secret.Value(ctx, esalStore)
		if err != nil {
			return errors.Wrap(err, "getting value for secret")
		}
		envVars[i] = fmt.Sprintf("%s=%s", secret.Key, val)
	}

	resolver := newResolver(r.store)
	workspaces, err := resolver.ResolveWorkspacesForBatchSpec(ctx, evaluatableSpec)
	if err != nil {
		return err
	}

	r.logger.Info("resolved workspaces for batch spec", log.Int64("job", job.ID), log.Int64("spec", spec.ID), log.Int("workspaces", len(workspaces)))

	// Build DB workspaces and check for cache entries.
	ws := make([]*btypes.BatchSpecWorkspace, 0, len(workspaces))
	// Collect all cache keys so we can look them up in a single query.
	cacheKeyWorkspaces := make([]workspaceCacheKey, 0, len(workspaces))
	allStepCacheKeys := make([]string, 0, len(workspaces))
	// load the mounts from the DB up front to avoid duplicate calls with no difference in data
	mounts, err := listBatchSpecMounts(ctx, r.store, spec.ID)
	if err != nil {
		return err
	}
	retriever := &remoteFileMetadataRetriever{mounts: mounts}

	// Build workspaces DB objects.
	for _, w := range workspaces {
		workspace := &btypes.BatchSpecWorkspace{
			BatchSpecID:      spec.ID,
			ChangesetSpecIDs: []int64{},

			RepoID:             w.Repo.ID,
			Branch:             w.Branch,
			Commit:             string(w.Commit),
			Path:               w.Path,
			FileMatches:        w.FileMatches,
			OnlyFetchWorkspace: w.OnlyFetchWorkspace,

			Unsupported: w.Unsupported,
			Ignored:     w.Ignored,
		}

		ws = append(ws, workspace)

		if !spec.AllowIgnored && w.Ignored {
			continue
		}
		if !spec.AllowUnsupported && w.Unsupported {
			continue
		}

		repo := batcheslib.Repository{
			ID:          string(marshalRepositoryID(w.Repo.ID)),
			Name:        string(w.Repo.Name),
			BaseRef:     w.Branch,
			BaseRev:     string(w.Commit),
			FileMatches: w.FileMatches,
		}

		skippedSteps, err := batcheslib.SkippedStepsForRepo(spec.Spec, string(w.Repo.Name), w.FileMatches)
		if err != nil {
			return err
		}

		stepCacheKeys := make([]stepCacheKey, 0, len(spec.Spec.Steps))
		// Generate cache keys for all the steps.
		for i := range len(spec.Spec.Steps) {
			if _, ok := skippedSteps[i]; ok {
				continue
			}

			key := cache.KeyForWorkspace(
				&template.BatchChangeAttributes{
					Name:        spec.Spec.Name,
					Description: spec.Spec.Description,
				},
				repo,
				w.Path,
				envVars,
				w.OnlyFetchWorkspace,
				spec.Spec.Steps,
				i,
				retriever,
			)

			rawStepKey, err := key.Key()
			if err != nil {
				return err
			}

			stepCacheKeys = append(stepCacheKeys, stepCacheKey{index: i, key: rawStepKey})
			allStepCacheKeys = append(allStepCacheKeys, rawStepKey)
		}

		cacheKeyWorkspaces = append(cacheKeyWorkspaces, workspaceCacheKey{
			dbWorkspace:   workspace,
			repo:          repo,
			stepCacheKeys: stepCacheKeys,
			skippedSteps:  skippedSteps,
		})
	}

	stepEntriesByCacheKey := make(map[string]*btypes.BatchSpecExecutionCacheEntry, len(allStepCacheKeys))
	if len(allStepCacheKeys) > 0 {
		entries, err := r.store.ListBatchSpecExecutionCacheEntries(ctx, store.ListBatchSpecExecutionCacheEntriesOpts{
			UserID: spec.UserID,
			Keys:   allStepCacheKeys,
		})
		if err != nil {
			return err
		}
		for _, entry := range entries {
			stepEntriesByCacheKey[entry.Key] = entry
		}
	}

	// All changeset specs to be created.
	cs := []*btypes.ChangesetSpec{}
	// Collect all IDs of used cache entries to mark them as recently used later.
	usedCacheEntries := []int64{}
	changesetsByWorkspace := make(map[*btypes.BatchSpecWorkspace][]*btypes.ChangesetSpec)

	changesetAuthor, err := author.GetChangesetAuthorForUser(ctx, database.UsersWith(r.logger, r.store), spec.UserID)
	if err != nil {
		return err
	}

	// Check for an existing cache entry for each of the workspaces.
	for _, workspace := range cacheKeyWorkspaces {
		for _, ck := range workspace.stepCacheKeys {
			key := ck.key
			idx := ck.index
			if c, ok := stepEntriesByCacheKey[key]; ok {
				var res execution.AfterStepResult
				if err := json.Unmarshal([]byte(c.Value), &res); err != nil {
					return err
				}
				workspace.dbWorkspace.SetStepCacheResult(idx+1, btypes.StepCacheResult{Key: key, Value: &res})

				// Mark the cache entry as used.
				usedCacheEntries = append(usedCacheEntries, c.ID)
			} else {
				// Only add cache entries up until we don't have the cache entry
				// for the previous step anymore.
				break
			}
		}

		// Validate there is anything to run. If not, we skip execution.
		// TODO: In the future, move this to a separate field, so we can
		// tell the two cases apart.
		if len(spec.Spec.Steps) == len(workspace.skippedSteps) {
			// TODO: Doesn't this mean we don't build changeset specs?
			workspace.dbWorkspace.CachedResultFound = true
			continue
		}

		// Find the latest step that is not statically skipped.
		latestStepIdx := -1
		for i := len(spec.Spec.Steps) - 1; i >= 0; i-- {
			// Keep skipping steps until the first one is hit that we do want to run.
			if _, ok := workspace.skippedSteps[i]; ok {
				continue
			}
			latestStepIdx = i
			break
		}
		if latestStepIdx == -1 {
			continue
		}

		// TODO: Should we also do dynamic evaluation, instead of just static?
		// We have everything that's needed at this point, including the latest
		// execution step result.
		res, found := workspace.dbWorkspace.StepCacheResult(latestStepIdx + 1)
		if !found {
			// There is no cache result available, proceed.
			continue
		}

		workspace.dbWorkspace.CachedResultFound = true

		rawSpecs, err := cache.ChangesetSpecsFromCache(spec.Spec, workspace.repo, *res.Value, workspace.dbWorkspace.Path, true, changesetAuthor)
		if err != nil {
			return err
		}

		var specs []*btypes.ChangesetSpec
		for _, s := range rawSpecs {
			changesetSpec, err := btypes.NewChangesetSpecFromSpec(s)
			if err != nil {
				return err
			}
			changesetSpec.BatchSpecID = spec.ID
			changesetSpec.BaseRepoID = workspace.dbWorkspace.RepoID
			changesetSpec.UserID = spec.UserID

			specs = append(specs, changesetSpec)
		}

		cs = append(cs, specs...)
		changesetsByWorkspace[workspace.dbWorkspace] = specs
	}

	// If there are "importChangesets" statements in the spec we evaluate
	// them now and create ChangesetSpecs for them.
	im, err := changesetSpecsForImports(ctx, r.store, evaluatableSpec.ImportChangesets, spec.ID, spec.UserID)
	if err != nil {
		return err
	}
	cs = append(cs, im...)

	tx, err := r.store.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	// Mark all used cache entries as recently used for cache eviction purposes.
	if err := tx.MarkUsedBatchSpecExecutionCacheEntries(ctx, usedCacheEntries); err != nil {
		return err
	}

	if err = tx.CreateChangesetSpec(ctx, cs...); err != nil {
		return err
	}

	// Associate the changeset specs with the workspace now that they have IDs.
	for workspace, changesetSpecs := range changesetsByWorkspace {
		for _, spec := range changesetSpecs {
			workspace.ChangesetSpecIDs = append(workspace.ChangesetSpecIDs, spec.ID)
		}
	}

	return tx.CreateBatchSpecWorkspace(ctx, ws...)
}

func listBatchSpecMounts(ctx context.Context, s *store.Store, batchSpecID int64) ([]*btypes.BatchSpecWorkspaceFile, error) {
	mounts, _, err := s.ListBatchSpecWorkspaceFiles(ctx, store.ListBatchSpecWorkspaceFileOpts{BatchSpecID: batchSpecID})
	if err != nil {
		return nil, err
	}
	return mounts, nil
}

type remoteFileMetadataRetriever struct {
	mounts []*btypes.BatchSpecWorkspaceFile
}

func (r *remoteFileMetadataRetriever) Get(steps []batcheslib.Step) ([]cache.MountMetadata, error) {
	var mountsMetadata []cache.MountMetadata
	for _, step := range steps {
		for _, stepMount := range step.Mount {
			dir, file := filepath.Split(stepMount.Path)
			dir = strings.TrimSuffix(dir, string(filepath.Separator))
			dir = strings.TrimPrefix(dir, fmt.Sprintf(".%s", string(filepath.Separator)))

			mountPath := filepath.Join(dir, file)
			var metadata cache.MountMetadata
			for _, mount := range r.mounts {
				if filepath.Join(mount.Path, mount.FileName) == mountPath {
					metadata = cache.MountMetadata{Path: mountPath, Size: mount.Size, Modified: mount.ModifiedAt}
				}
			}
			if metadata.Path != "" {
				mountsMetadata = append(mountsMetadata, metadata)
			} else {
				// It is probably a directory
				for _, mount := range r.mounts {
					mountsMetadata = append(mountsMetadata, cache.MountMetadata{Path: filepath.Join(mount.Path, mount.FileName), Size: mount.Size, Modified: mount.ModifiedAt})
				}
			}

		}
	}
	return mountsMetadata, nil
}

func changesetSpecsForImports(ctx context.Context, s *store.Store, importChangesets []batcheslib.ImportChangeset, batchSpecID int64, userID int32) ([]*btypes.ChangesetSpec, error) {
	cs := []*btypes.ChangesetSpec{}

	reposStore := s.Repos()

	specs, err := batcheslib.BuildImportChangesetSpecs(ctx, importChangesets, func(ctx context.Context, repoNames []string) (map[string]string, error) {
		if len(repoNames) == 0 {
			return map[string]string{}, nil
		}

		// 🚨 SECURITY: We use database.Repos.List to get the ID and also to check
		// whether the user has access to the repository or not.
		repos, err := reposStore.List(ctx, database.ReposListOptions{Names: repoNames})
		if err != nil {
			return nil, err
		}

		repoNameIDs := make(map[string]string, len(repos))
		for _, r := range repos {
			repoNameIDs[string(r.Name)] = string(marshalRepositoryID(r.ID))
		}
		return repoNameIDs, nil
	})
	if err != nil {
		return nil, err
	}
	for _, c := range specs {
		var repoID api.RepoID
		err = relay.UnmarshalSpec(graphql.ID(c.BaseRepository), &repoID)
		if err != nil {
			return nil, err
		}

		changesetSpec, err := btypes.NewChangesetSpecFromSpec(c)
		if err != nil {
			return nil, err
		}
		changesetSpec.UserID = userID
		changesetSpec.BaseRepoID = repoID
		changesetSpec.BatchSpecID = batchSpecID

		cs = append(cs, changesetSpec)
	}
	return cs, nil
}

func marshalRepositoryID(id api.RepoID) graphql.ID {
	return relay.MarshalID("Repository", id)
}
