package graphqlbackend

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/externallink"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	resolverstubs "github.com/sourcegraph/sourcegraph/internal/codeintel/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/phabricator"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/perforce"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

type RepositoryResolver struct {
	id   api.RepoID
	name api.RepoName

	db              database.DB
	gitserverClient gitserver.Client
	logger          log.Logger

	// Fields below this line should not be used directly.
	// Use the get* methods instead.

	repoOnce sync.Once
	repo     *types.Repo
	repoErr  error

	defaultBranchOnce sync.Once
	defaultBranch     *GitRefResolver
	defaultBranchErr  error

	linkerOnce sync.Once
	linker     externallink.RepositoryLinker
	linkerErr  error
}

// NewMinimalRepositoryResolver creates a new lazy resolver from the minimum necessary information: repo name and repo ID.
// If you have a fully resolved *types.Repo, use NewRepositoryResolver instead.
func NewMinimalRepositoryResolver(db database.DB, client gitserver.Client, id api.RepoID, name api.RepoName) *RepositoryResolver {
	return &RepositoryResolver{
		id:              id,
		name:            name,
		db:              db,
		gitserverClient: client,
		logger: log.Scoped("repositoryResolver").
			With(log.Object("repo",
				log.String("name", string(name)),
				log.Int32("id", int32(id)))),
	}
}

// NewRepositoryResolver creates a repository resolver from a fully resolved *types.Repo. Do not use this
// function with an incomplete *types.Repo. Instead, use NewMinimalRepositoryResolver, which will lazily
// fetch the *types.Repo if needed.
func NewRepositoryResolver(db database.DB, gs gitserver.Client, repo *types.Repo) *RepositoryResolver {
	// Protect against a nil repo
	//
	// TODO(@camdencheek): this shouldn't be necessary because it doesn't
	// make sense to construct a repository resolver from a nil repo,
	// but this was historically allowed, so tests fail without this.
	// We should do an audit of callsites to fix places where this could
	// be called with a nil repo.
	var id api.RepoID
	var name api.RepoName
	if repo != nil {
		name = repo.Name
		id = repo.ID
	}

	return &RepositoryResolver{
		id:              id,
		name:            name,
		db:              db,
		gitserverClient: gs,
		logger: log.Scoped("repositoryResolver").
			With(log.Object("repo",
				log.String("name", string(name)),
				log.Int32("id", int32(id)))),
		repo: repo,
	}
}

func (r *RepositoryResolver) ID() graphql.ID {
	return MarshalRepositoryID(r.IDInt32())
}

func (r *RepositoryResolver) IDInt32() api.RepoID {
	return r.id
}

func MarshalRepositoryID(repo api.RepoID) graphql.ID { return relay.MarshalID("Repository", repo) }

func MarshalRepositoryIDs(ids []api.RepoID) []graphql.ID {
	res := make([]graphql.ID, len(ids))
	for i, id := range ids {
		res[i] = MarshalRepositoryID(id)
	}
	return res
}

func UnmarshalRepositoryID(id graphql.ID) (repo api.RepoID, err error) {
	err = relay.UnmarshalSpec(id, &repo)
	return
}

func UnmarshalRepositoryIDs(ids []graphql.ID) ([]api.RepoID, error) {
	repoIDs := make([]api.RepoID, len(ids))
	for i, id := range ids {
		repoID, err := UnmarshalRepositoryID(id)
		if err != nil {
			return nil, err
		}
		repoIDs[i] = repoID
	}
	return repoIDs, nil
}

// repo makes sure the repo is hydrated before returning it.
func (r *RepositoryResolver) getRepo(ctx context.Context) (*types.Repo, error) {
	r.repoOnce.Do(func() {
		// Repositories with an empty creation date were created using RepoName.ToRepo(),
		// they only contain ID and name information.
		//
		// TODO(@camdencheek): We _shouldn't_ need to inspect the CreatedAt date,
		// but the API NewRepositoryResolver previously allowed passing in a *types.Repo
		// with only the Name and ID fields populated.
		if r.repo != nil && !r.repo.CreatedAt.IsZero() {
			return
		}
		r.logger.Debug("RepositoryResolver.hydrate", log.String("repo.ID", string(r.IDInt32())))
		r.repo, r.repoErr = r.db.Repos().Get(ctx, r.id)
	})
	return r.repo, r.repoErr
}

func (r *RepositoryResolver) RepoName() api.RepoName {
	return r.name
}

func (r *RepositoryResolver) Name() string {
	return string(r.name)
}

func (r *RepositoryResolver) ExternalRepo(ctx context.Context) (*api.ExternalRepoSpec, error) {
	repo, err := r.getRepo(ctx)
	if err != nil {
		return nil, err
	}
	return &repo.ExternalRepo, err
}

func (r *RepositoryResolver) IsFork(ctx context.Context) (bool, error) {
	repo, err := r.getRepo(ctx)
	if err != nil {
		return false, err
	}
	return repo.Fork, err
}

func (r *RepositoryResolver) IsArchived(ctx context.Context) (bool, error) {
	repo, err := r.getRepo(ctx)
	if err != nil {
		return false, err
	}
	return repo.Archived, err
}

func (r *RepositoryResolver) IsPrivate(ctx context.Context) (bool, error) {
	repo, err := r.getRepo(ctx)
	if err != nil {
		return false, err
	}
	return repo.Private, err
}

func (r *RepositoryResolver) URI(ctx context.Context) (string, error) {
	repo, err := r.getRepo(ctx)
	if err != nil {
		return "", err
	}
	return repo.URI, err
}

func (r *RepositoryResolver) SourceType(ctx context.Context) (*SourceType, error) {
	repo, err := r.getRepo(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve innerRepo")
	}

	if repo.ExternalRepo.ServiceType == extsvc.TypePerforce {
		return &PerforceDepotSourceType, nil
	}

	return &GitRepositorySourceType, nil
}

func (r *RepositoryResolver) Description(ctx context.Context) (string, error) {
	repo, err := r.getRepo(ctx)
	if err != nil {
		return "", err
	}
	return repo.Description, err
}

func (r *RepositoryResolver) ViewerCanAdminister(ctx context.Context) (bool, error) {
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		if err == auth.ErrMustBeSiteAdmin || err == auth.ErrNotAuthenticated {
			return false, nil // not an error
		}
		return false, err
	}
	return true, nil
}

func (r *RepositoryResolver) CloneInProgress(ctx context.Context) (bool, error) {
	return r.MirrorInfo().CloneInProgress(ctx)
}

func (r *RepositoryResolver) DiskSizeBytes(ctx context.Context) (*BigInt, error) {
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		if err == auth.ErrMustBeSiteAdmin || err == auth.ErrNotAuthenticated {
			return nil, nil // not an error
		}
		return nil, err
	}
	repo, err := r.db.GitserverRepos().GetByID(ctx, r.IDInt32())
	if err != nil {
		return nil, err
	}
	size := BigInt(repo.RepoSizeBytes)
	return &size, nil
}

func (r *RepositoryResolver) BatchChanges(ctx context.Context, args *ListBatchChangesArgs) (BatchChangesConnectionResolver, error) {
	id := r.ID()
	args.Repo = &id
	return EnterpriseResolvers.batchChangesResolver.BatchChanges(ctx, args)
}

func (r *RepositoryResolver) ChangesetsStats(ctx context.Context) (RepoChangesetsStatsResolver, error) {
	id := r.ID()
	return EnterpriseResolvers.batchChangesResolver.RepoChangesetsStats(ctx, &id)
}

func (r *RepositoryResolver) BatchChangesDiffStat(ctx context.Context) (*DiffStat, error) {
	id := r.ID()
	return EnterpriseResolvers.batchChangesResolver.RepoDiffStat(ctx, &id)
}

type RepositoryCommitArgs struct {
	Rev          string
	InputRevspec *string
}

func (r *RepositoryResolver) Commit(ctx context.Context, args *RepositoryCommitArgs) (_ *GitCommitResolver, err error) {
	tr, ctx := trace.New(ctx, "RepositoryResolver.Commit",
		attribute.String("commit", args.Rev))
	defer tr.EndWithErr(&err)

	commitID, err := backend.NewRepos(r.logger, r.db, r.gitserverClient).ResolveRev(ctx, r.name, args.Rev)
	if err != nil {
		if errors.HasType[*gitdomain.RevisionNotFoundError](err) {
			return nil, nil
		}
		return nil, err
	}

	return r.CommitFromID(ctx, args, commitID)
}

type RepositoryChangelistArgs struct {
	CID string
}

func (r *RepositoryResolver) Changelist(ctx context.Context, args *RepositoryChangelistArgs) (_ *PerforceChangelistResolver, err error) {
	tr, ctx := trace.New(ctx, "RepositoryResolver.Changelist",
		attribute.String("changelist", args.CID))
	defer tr.EndWithErr(&err)

	cid, err := strconv.ParseInt(args.CID, 10, 64)
	if err != nil {
		// NOTE: From the UI, the user may visit a URL like:
		// https://sourcegraph.com/github.com/sourcegraph/sourcegraph@e28429f899870db6f6cbf0fc2bf98de6e947b213/-/blob/README.md
		//
		// Or they may visit a URL like:
		//
		// https://sourcegraph.com/perforce.sgdev.test/test-depot@998765/-/blob/README.md
		//
		// To make things easier, we request both the `commit($revision)` and the `changelist($cid)`
		// nodes on the `repository`.
		//
		// If the revision in the URL is a changelist ID then the commit node will be null (the
		// commit will not resolve).
		//
		// But if the revision in the URL is a commit SHA, then the changelist node will be null.
		// Which means we will be inadvertenly trying to parse a commit SHA into a int each time a
		// commit is viewed. We don't want to return an error in these cases.
		r.logger.Debug("failed to parse args.CID into int", log.String("args.CID", args.CID), log.Error(err))
		return nil, nil
	}

	repo, err := r.getRepo(ctx)
	if err != nil {
		return nil, err
	}

	// Changelists only exist for perforce repos.
	if repo.ExternalRepo.ServiceType != extsvc.TypePerforce {
		return nil, nil
	}

	rc, err := r.db.RepoCommitsChangelists().GetRepoCommitChangelist(ctx, repo.ID, cid)
	if err != nil {
		// Not found could mean either the mapper has some bug, is currently
		// broken, or hasn't run yet.
		// We take a slow path here to check if we can find the commit in the repo.
		if errcode.IsNotFound(err) {
			r.logger.Debug("failed to find changelist ID - trying to find it via log", log.String("args.CID", args.CID))
			messageQuery := fmt.Sprintf("change = %d]", cid)
			cs, err := r.gitserverClient.Commits(ctx, r.name, gitserver.CommitsOptions{
				AllRefs:      true,
				MessageQuery: messageQuery,
				N:            1,
			})
			if err != nil {
				return nil, err
			}
			if len(cs) == 0 {
				return nil, nil
			}
			cid, err := perforce.GetP4ChangelistID(string(cs[0].Message))
			if err != nil {
				tr.AddEvent("failed to parse changelist", attribute.String("error", err.Error()))
				return nil, nil
			}
			return newPerforceChangelistResolver(
				r,
				cid,
				string(cs[0].ID),
			), nil
		}
		return nil, err
	}

	return newPerforceChangelistResolver(
		r,
		fmt.Sprintf("%d", rc.PerforceChangelistID),
		string(rc.CommitSHA),
	), nil
}

func (r *RepositoryResolver) FirstEverCommit(ctx context.Context) (_ *GitCommitResolver, err error) {
	tr, ctx := trace.New(ctx, "RepositoryResolver.FirstEverCommit")
	defer tr.EndWithErr(&err)

	repo, err := r.getRepo(ctx)
	if err != nil {
		return nil, err
	}

	commit, err := r.gitserverClient.FirstEverCommit(ctx, repo.Name)
	if err != nil {
		if errors.HasType[*gitdomain.RevisionNotFoundError](err) {
			return nil, nil
		}
		return nil, err
	}

	return r.CommitFromID(ctx, &RepositoryCommitArgs{}, commit.ID)
}

func (r *RepositoryResolver) CommitFromID(ctx context.Context, args *RepositoryCommitArgs, commitID api.CommitID) (*GitCommitResolver, error) {
	resolver := NewGitCommitResolver(r.db, r.gitserverClient, r, commitID, nil)
	if args.InputRevspec != nil {
		resolver.inputRev = args.InputRevspec
	} else {
		resolver.inputRev = &args.Rev
	}
	return resolver, nil
}

func (r *RepositoryResolver) DefaultBranch(ctx context.Context) (*GitRefResolver, error) {
	do := func() (*GitRefResolver, error) {
		refName, _, err := r.gitserverClient.GetDefaultBranch(ctx, r.RepoName(), false)
		if err != nil {
			return nil, err
		}
		if refName == "" {
			return nil, nil
		}
		return &GitRefResolver{repo: r, name: refName}, nil
	}
	r.defaultBranchOnce.Do(func() {
		r.defaultBranch, r.defaultBranchErr = do()
	})
	return r.defaultBranch, r.defaultBranchErr
}

func (r *RepositoryResolver) Language(ctx context.Context) (string, error) {
	// The repository language is the most common language at the HEAD commit of the repository.
	// Note: the repository database field is no longer updated as of
	// https://github.com/sourcegraph/sourcegraph/issues/2586, so we do not use it anymore and
	// instead compute the language on the fly.
	repo, err := r.getRepo(ctx)
	if err != nil {
		return "", err
	}

	commitID, err := backend.NewRepos(r.logger, r.db, r.gitserverClient).ResolveRev(ctx, r.name, "")
	if err != nil {
		// Comment: Should we return a nil error?
		return "", err
	}

	inventory, err := backend.NewRepos(r.logger, r.db, r.gitserverClient).GetInventory(ctx, repo.Name, commitID, false)
	if err != nil {
		return "", err
	}
	if len(inventory.Languages) == 0 {
		return "", err
	}
	return inventory.Languages[0].Name, nil
}

func (r *RepositoryResolver) Enabled() bool { return true }

// CreatedAt is deprecated and will be removed in a future release.
// No clients that we know of read this field. Additionally on performance profiles
// the marshalling of timestamps is significant in our postgres client. So we
// deprecate the fields and return fake data for created_at.
// https://github.com/sourcegraph/sourcegraph/pull/4668
func (r *RepositoryResolver) CreatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: time.Now()}
}

func (r *RepositoryResolver) RawCreatedAt() string {
	// TODO(@camdencheek): this represents a race condition between
	// the sync.Once that populates r.repo and any callers of this method.
	// The risk is small, so I'm not worrying about it right now.
	if r.repo == nil {
		return ""
	}
	return r.repo.CreatedAt.Format(time.RFC3339)
}

func (r *RepositoryResolver) UpdatedAt() *gqlutil.DateTime {
	return nil
}

func (r *RepositoryResolver) URL() string {
	return r.url().String()
}

func (r *RepositoryResolver) url() *url.URL {
	return &url.URL{Path: "/" + string(r.name)}
}

func (r *RepositoryResolver) ExternalURLs(ctx context.Context) ([]*externallink.Resolver, error) {
	linker, err := r.getLinker(ctx)
	if err != nil {
		return nil, err
	}
	return linker.Repository(), nil
}

func (r *RepositoryResolver) Label() (Markdown, error) {
	text := "[" + r.Name() + "](" + r.URL() + ")"
	return Markdown(text), nil
}

func (r *RepositoryResolver) Detail() Markdown {
	return "Repository match"
}

func (r *RepositoryResolver) Matches() []*searchResultMatchResolver {
	return nil
}

func (r *RepositoryResolver) ToRepository() (*RepositoryResolver, bool) { return r, true }
func (r *RepositoryResolver) ToFileMatch() (*FileMatchResolver, bool)   { return nil, false }
func (r *RepositoryResolver) ToCommitSearchResult() (*CommitSearchResultResolver, bool) {
	return nil, false
}

func (r *RepositoryResolver) Stars(ctx context.Context) (int32, error) {
	repo, err := r.getRepo(ctx)
	if err != nil {
		return 0, err
	}
	return int32(repo.Stars), nil
}

// Deprecated: Use RepositoryResolver.Metadata instead.
func (r *RepositoryResolver) KeyValuePairs(ctx context.Context) ([]KeyValuePair, error) {
	return r.Metadata(ctx)
}

func (r *RepositoryResolver) Metadata(ctx context.Context) ([]KeyValuePair, error) {
	repo, err := r.getRepo(ctx)
	if err != nil {
		return nil, err
	}

	kvps := make([]KeyValuePair, 0, len(repo.KeyValuePairs))
	for k, v := range repo.KeyValuePairs {
		kvps = append(kvps, KeyValuePair{key: k, value: v})
	}
	return kvps, nil
}

func (r *RepositoryResolver) Topics(ctx context.Context) ([]string, error) {
	repo, err := r.getRepo(ctx)
	if err != nil {
		return nil, err
	}

	return repo.Topics, nil
}

func (r *RepositoryResolver) IndexConfiguration(ctx context.Context) (resolverstubs.IndexConfigurationResolver, error) {
	return EnterpriseResolvers.codeIntelResolver.IndexConfiguration(ctx, r.ID())
}

func (r *RepositoryResolver) CodeIntelligenceCommitGraph(ctx context.Context) (resolverstubs.CodeIntelligenceCommitGraphResolver, error) {
	return EnterpriseResolvers.codeIntelResolver.CommitGraph(ctx, r.ID())
}

func (r *RepositoryResolver) CodeIntelSummary(ctx context.Context) (resolverstubs.CodeIntelRepositorySummaryResolver, error) {
	return EnterpriseResolvers.codeIntelResolver.RepositorySummary(ctx, r.ID())
}

func (r *RepositoryResolver) PreviewGitObjectFilter(ctx context.Context, args *resolverstubs.PreviewGitObjectFilterArgs) (resolverstubs.GitObjectFilterPreviewResolver, error) {
	return EnterpriseResolvers.codeIntelResolver.PreviewGitObjectFilter(ctx, r.ID(), args)
}

type AuthorizedUserArgs struct {
	RepositoryID graphql.ID
	Permission   string
	First        int32
	After        *string
}

type RepoAuthorizedUserArgs struct {
	RepositoryID graphql.ID
	*AuthorizedUserArgs
}

func (r *RepositoryResolver) AuthorizedUsers(ctx context.Context, args *AuthorizedUserArgs) (UserConnectionResolver, error) {
	return EnterpriseResolvers.authzResolver.AuthorizedUsers(ctx, &RepoAuthorizedUserArgs{
		RepositoryID:       r.ID(),
		AuthorizedUserArgs: args,
	})
}

func (r *RepositoryResolver) PermissionsInfo(ctx context.Context) (PermissionsInfoResolver, error) {
	return EnterpriseResolvers.authzResolver.RepositoryPermissionsInfo(ctx, r.ID())
}

func (r *schemaResolver) AddPhabricatorRepo(ctx context.Context, args *struct {
	Callsign string
	Name     *string
	// TODO(chris): Remove URI in favor of Name.
	URI *string
	URL string
},
) (*EmptyResponse, error) {
	if args.Name != nil {
		args.URI = args.Name
	}

	_, err := r.db.Phabricator().CreateIfNotExists(ctx, args.Callsign, api.RepoName(*args.URI), args.URL)
	if err != nil {
		r.logger.Error("adding phabricator repo", log.String("callsign", args.Callsign), log.Stringp("name", args.URI), log.String("url", args.URL))
	}
	return nil, err
}

func (r *schemaResolver) ResolvePhabricatorDiff(ctx context.Context, args *struct {
	RepoName    string
	DiffID      int32
	BaseRev     string
	Patch       *string
	AuthorName  *string
	AuthorEmail *string
	Description *string
	Date        *string
},
) (*GitCommitResolver, error) {
	db := r.db
	repo, err := db.Repos().GetByName(ctx, api.RepoName(args.RepoName))
	if err != nil {
		return nil, err
	}
	targetRef := fmt.Sprintf("phabricator/diff/%d", args.DiffID)
	getCommit := func() (*GitCommitResolver, error) {
		// We first check via the vcsrepo api so that we can toggle
		// NoEnsureRevision. We do this, otherwise RepositoryResolver.Commit
		// will try and fetch it from the remote host. However, this is not on
		// the remote host since we created it.
		_, err = r.gitserverClient.ResolveRevision(ctx, repo.Name, targetRef, gitserver.ResolveRevisionOptions{
			EnsureRevision: false,
		})
		if err != nil {
			return nil, err
		}
		r := NewRepositoryResolver(db, r.gitserverClient, repo)
		return r.Commit(ctx, &RepositoryCommitArgs{Rev: targetRef})
	}

	// If we already created the commit
	if commit, err := getCommit(); commit != nil || (err != nil && !errors.HasType[*gitdomain.RevisionNotFoundError](err)) {
		return commit, err
	}

	origin := ""
	if phabRepo, err := db.Phabricator().GetByName(ctx, api.RepoName(args.RepoName)); err == nil {
		origin = phabRepo.URL
	}

	if origin == "" {
		return nil, errors.New("unable to resolve the origin of the phabricator instance")
	}

	client, clientErr := makePhabClientForOrigin(ctx, r.logger, db, origin)

	patch := ""
	if args.Patch != nil {
		patch = *args.Patch
	} else if client == nil {
		return nil, clientErr
	} else {
		diff, err := client.GetRawDiff(ctx, int(args.DiffID))
		// No diff contents were given and we couldn't fetch them
		if err != nil {
			return nil, err
		}

		patch = diff
	}

	var info *phabricator.DiffInfo
	if client != nil && (args.AuthorEmail == nil || args.AuthorName == nil || args.Date == nil) {
		info, err = client.GetDiffInfo(ctx, int(args.DiffID))
		// Not all the information was given and we couldn't fetch it
		if err != nil {
			return nil, err
		}
	} else {
		var description, authorName, authorEmail string
		if args.Description != nil {
			description = *args.Description
		}
		if args.AuthorName != nil {
			authorName = *args.AuthorName
		}
		if args.AuthorEmail != nil {
			authorEmail = *args.AuthorEmail
		}
		date, err := phabricator.ParseDate(*args.Date)
		if err != nil {
			return nil, err
		}

		info = &phabricator.DiffInfo{
			AuthorName:  authorName,
			AuthorEmail: authorEmail,
			Message:     description,
			Date:        *date,
		}
	}

	_, err = r.gitserverClient.CreateCommitFromPatch(ctx, protocol.CreateCommitFromPatchRequest{
		Repo:       api.RepoName(args.RepoName),
		BaseCommit: api.CommitID(args.BaseRev),
		TargetRef:  targetRef,
		Patch:      []byte(patch),
		CommitInfo: protocol.PatchCommitInfo{
			AuthorName:  info.AuthorName,
			AuthorEmail: info.AuthorEmail,
			Messages:    []string{info.Message},
			Date:        info.Date,
		},
	})
	if err != nil {
		return nil, err
	}

	return getCommit()
}

func makePhabClientForOrigin(ctx context.Context, logger log.Logger, db database.DB, origin string) (*phabricator.Client, error) {
	opt := database.ExternalServicesListOptions{
		Kinds: []string{extsvc.KindPhabricator},
		LimitOffset: &database.LimitOffset{
			Limit: 500, // The number is randomly chosen
		},
	}
	for {
		svcs, err := db.ExternalServices().List(ctx, opt)
		if err != nil {
			return nil, errors.Wrap(err, "list")
		}
		if len(svcs) == 0 {
			break // No more results, exiting
		}
		opt.AfterID = svcs[len(svcs)-1].ID // Advance the cursor

		for _, svc := range svcs {
			cfg, err := extsvc.ParseEncryptableConfig(ctx, svc.Kind, svc.Config)
			if err != nil {
				return nil, errors.Wrap(err, "parse config")
			}

			var conn *schema.PhabricatorConnection
			switch c := cfg.(type) {
			case *schema.PhabricatorConnection:
				conn = c
			default:
				err := errors.Errorf("want *schema.PhabricatorConnection but got %T", cfg)
				logger.Error("makePhabClientForOrigin", log.Error(err))
				continue
			}

			if conn.Url != origin {
				continue
			}

			if conn.Token == "" {
				return nil, errors.Errorf("no phabricator token was given for: %s", origin)
			}

			return phabricator.NewClient(ctx, conn.Url, conn.Token, nil)
		}

		if len(svcs) < opt.Limit {
			break // Less results than limit means we've reached end
		}
	}

	return nil, errors.Errorf("no phabricator was configured for: %s", origin)
}

func (r *RepositoryResolver) IngestedCodeowners(ctx context.Context) (CodeownersIngestedFileResolver, error) {
	return EnterpriseResolvers.ownResolver.RepoIngestedCodeowners(ctx, r.IDInt32())
}

// isPerforceDepot is a helper to avoid the repetitive error handling of calling r.SourceType, and
// where we want to only take a custom action if this function returns true. For false we want to
// ignore and continue on the default behaviour.
func (r *RepositoryResolver) isPerforceDepot(ctx context.Context) bool {
	s, err := r.SourceType(ctx)
	if err != nil {
		r.logger.Error("failed to retrieve sourceType of repository", log.Error(err))
		return false
	}

	return s == &PerforceDepotSourceType
}

func (r *RepositoryResolver) getLinker(ctx context.Context) (externallink.RepositoryLinker, error) {
	r.linkerOnce.Do(func() {
		defaultBranchResolver, err := r.DefaultBranch(ctx)
		if err != nil {
			r.linkerErr = err
			return
		}

		defaultBranch := ""
		if defaultBranchResolver != nil {
			defaultBranch = defaultBranchResolver.Name()
		}

		repo, err := r.getRepo(ctx)
		if err != nil {
			r.linkerErr = err
			return
		}
		r.linker = externallink.NewRepositoryLinker(ctx, r.db, repo, defaultBranch)
	})
	return r.linker, r.linkerErr
}
