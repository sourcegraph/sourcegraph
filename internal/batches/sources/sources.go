package sources

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/batches/store"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	ghaauth "github.com/sourcegraph/sourcegraph/internal/github_apps/auth"
	ghastore "github.com/sourcegraph/sourcegraph/internal/github_apps/store"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// ErrExternalServiceNotGitHub is returned when authenticating a ChangesetSource for a
// changeset if the method is invoked with AuthenticationStrategyGitHubApp, but the
// external service that is loaded for the changeset repo is not a GitHub connection.
var ErrExternalServiceNotGitHub = errors.New("cannot use GitHub App authentication with non-GitHub external service")

// ErrNoGitHubAppConfigured is returned when authenticating a ChangesetSource for a
// changeset if the method is invoked with AuthenticationStrategyGitHubApp and the code
// host is GitHub, but there is no GitHub App configured for it for Batch Changes.
var ErrNoGitHubAppConfigured = errors.New("no batches GitHub App found that can authenticate to this code host")

// ErrNoGitHubAppInstallation is returned when authenticating a ChangesetSource for a
// changeset if the method is invoked with AuthenticationStrategyGitHubApp, the code host
// is GitHub, and a GitHub App is configured for it for Batch Changes, but there is no
// recorded installation of that app for provided account namepsace.
var ErrNoGitHubAppInstallation = errors.New("no installations of GitHub App found for this account namespace")

// ErrMissingCredentials is returned when authenticating a ChangesetSource for a changeset
// or a user, if no user or site credential can be found that can authenticate to the code
// host.
var ErrMissingCredentials = errors.New("no credential found that can authenticate to the code host")

// ErrNoPushCredentials is returned by gitserverPushConfig if the
// authenticator cannot be used by git to authenticate a `git push`.
type ErrNoPushCredentials struct{ CredentialsType string }

func (e ErrNoPushCredentials) Error() string {
	return "invalid authenticator provided to push commits"
}

// ErrNoSSHCredential is returned by gitserverPushConfig, if the
// clone URL of the repository uses the ssh:// scheme, but the authenticator
// doesn't support SSH pushes.
var ErrNoSSHCredential = errors.New("authenticator doesn't support SSH")

// AuthenticationStrategy defines the possible types of authentication strategy that can
// be used to authenticate a ChangesetSource for a changeset.
type AuthenticationStrategy string

const (
	// Authenticate using a traditional PAT configured by the user or site admin. This
	// should be used for all code host interactions unless another authentication
	// strategy is explicitly required.
	AuthenticationStrategyUserCredential AuthenticationStrategy = "USER_CREDENTIAL"
	// Authenticate using a GitHub App. This should only be used for GitHub code hosts for
	// commit signing interactions.
	AuthenticationStrategyGitHubApp AuthenticationStrategy = "GITHUB_APP"
)

type SourcerStore interface {
	DatabaseDB() database.DB
	GetBatchChange(ctx context.Context, opts store.GetBatchChangeOpts) (*btypes.BatchChange, error)
	GetSiteCredential(ctx context.Context, opts store.GetSiteCredentialOpts) (*btypes.SiteCredential, error)
	GetExternalServiceIDs(ctx context.Context, opts store.GetExternalServiceIDsOpts) ([]int64, error)
	Repos() database.RepoStore
	ExternalServices() database.ExternalServiceStore
	UserCredentials() database.UserCredentialsStore
	GitHubAppsStore() ghastore.GitHubAppsStore
	GetChangesetSpecByID(ctx context.Context, id int64) (*btypes.ChangesetSpec, error)
}

// Sourcer exposes methods to get a ChangesetSource based on a changeset, repo or
// external service.
type Sourcer interface {
	// ForChangeset returns a ChangesetSource for the given changeset. The changeset.RepoID
	// is used to find the matching code host.
	//
	// It authenticates the given ChangesetSource with a credential appropriate to sync or
	// reconcile the given changeset based on the AuthenticationStrategy. Under most
	// conditions, the AuthenticationStrategy should be
	// AuthenticationStrategyUserCredential. When this strategy is used, if the changeset
	// was created by a batch change, then authentication will be based on the first
	// available option of:
	//
	// 1. The last applying user's credentials.
	// 2. Any available site credential matching the changesets repo.
	//
	// If the changeset was not created by a batch change, then a site credential will be
	// used. If another AuthenticationStrategy is specified, then it will be used.
	ForChangeset(ctx context.Context, tx SourcerStore, ch *btypes.Changeset, as AuthenticationStrategy, repo *types.Repo) (ChangesetSource, error)
	// ForUser returns a ChangesetSource for changesets on the given repo.
	// It will be authenticated with the given authenticator.
	ForUser(ctx context.Context, tx SourcerStore, uid int32, repo *types.Repo) (ChangesetSource, error)
	// ForExternalService returns a ChangesetSource based on the provided external service opts.
	// It will be authenticated with the given authenticator.
	ForExternalService(ctx context.Context, tx SourcerStore, au auth.Authenticator, opts ForExternalServiceOpts, as AuthenticationStrategy) (ChangesetSource, error)
}

// NewSourcer returns a new Sourcer to be used in Batch Changes.
func NewSourcer(cf *httpcli.Factory) Sourcer {
	return newSourcer(cf, loadBatchesSource)
}

type changesetSourceFactory func(ctx context.Context, tx SourcerStore, cf *httpcli.Factory, extSvc *types.ExternalService) (ChangesetSource, error)

type sourcer struct {
	logger    log.Logger
	cf        *httpcli.Factory
	newSource changesetSourceFactory
}

func newSourcer(cf *httpcli.Factory, csf changesetSourceFactory) Sourcer {
	return &sourcer{
		logger:    log.Scoped("sourcer"),
		cf:        cf,
		newSource: csf,
	}
}

func (s *sourcer) ForChangeset(ctx context.Context, tx SourcerStore, ch *btypes.Changeset, as AuthenticationStrategy, targetRepo *types.Repo) (ChangesetSource, error) {
	repo, err := tx.Repos().Get(ctx, ch.RepoID)
	if err != nil {
		return nil, errors.Wrap(err, "loading changeset repo")
	}

	// Consider all available external services for this repo.
	extSvc, err := loadExternalService(ctx, tx.ExternalServices(), database.ExternalServicesListOptions{
		IDs: repo.ExternalServiceIDs(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "loading external service")
	}

	if as == AuthenticationStrategyGitHubApp && extSvc.Kind != extsvc.KindGitHub {
		return nil, ErrExternalServiceNotGitHub
	}

	css, err := s.newSource(ctx, tx, s.cf, extSvc)
	if err != nil {
		return nil, err
	}

	if as == AuthenticationStrategyGitHubApp {
		cs, err := tx.GetChangesetSpecByID(ctx, ch.CurrentSpecID)
		if err != nil {
			return nil, errors.Wrap(err, "getting changeset spec")
		}

		var owner string
		// We check if the changeset is meant to be pushed to a fork.
		// If yes, then we try to figure out the user namespace and get a github app for the user namespace.
		if cs.IsFork() {
			// forkNamespace is nil returns a non-nil value if the fork namespace is explicitly defined
			// e.g sourcegraph.
			// if it isn't then we assume the changeset will be forked into the current user's namespace
			forkNamespace := cs.GetForkNamespace()
			if forkNamespace != nil {
				owner = *forkNamespace
			} else {
				u, err := getCloneURL(targetRepo)
				if err != nil {
					return nil, errors.Wrap(err, "getting url for forked changeset")
				}

				owner, _, err = github.SplitRepositoryNameWithOwner(strings.TrimPrefix(u.Path, "/"))
				if err != nil {
					return nil, errors.Wrap(err, "getting owner from repo name")
				}
			}
		} else {
			// Get owner from repo metadata. We expect repo.Metadata to be a github.Repository because the
			// authentication strategy `AuthenticationStrategyGitHubApp` only applies to GitHub repositories.
			// so this is a safe type cast.
			repoMetadata := repo.Metadata.(*github.Repository)
			owner, _, err = github.SplitRepositoryNameWithOwner(repoMetadata.NameWithOwner)
			if err != nil {
				return nil, errors.Wrap(err, "getting owner from repo name")
			}
		}

		return withGitHubAppAuthenticator(ctx, tx, css, extSvc, owner, nil)
	}

	if ch.OwnedByBatchChangeID != 0 {
		batchChange, err := loadBatchChange(ctx, tx, ch.OwnedByBatchChangeID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load owning batch change")
		}

		return withAuthenticatorForUser(ctx, tx, css, batchChange.LastApplierID, repo)
	}

	return withSiteAuthenticator(ctx, tx, css, repo)
}

func (s *sourcer) ForUser(ctx context.Context, tx SourcerStore, uid int32, repo *types.Repo) (ChangesetSource, error) {
	// Consider all available external services for this repo.
	extSvc, err := loadExternalService(ctx, tx.ExternalServices(), database.ExternalServicesListOptions{
		IDs: repo.ExternalServiceIDs(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "loading external service")
	}
	css, err := s.newSource(ctx, tx, s.cf, extSvc)
	if err != nil {
		return nil, err
	}
	return withAuthenticatorForUser(ctx, tx, css, uid, repo)
}

type ForExternalServiceOpts struct {
	ExternalServiceID   string
	ExternalServiceType string
	GitHubAppAccount    string
	GitHubAppKind       *types.GitHubAppKind
}

func (s *sourcer) ForExternalService(ctx context.Context, tx SourcerStore, au auth.Authenticator, opts ForExternalServiceOpts, as AuthenticationStrategy) (ChangesetSource, error) {
	// Empty authenticators are not for non-GitHubApp authentication strategies.
	if au == nil {
		return nil, ErrMissingCredentials
	}

	extSvcIDs, err := tx.GetExternalServiceIDs(ctx, store.GetExternalServiceIDsOpts{
		ExternalServiceType: opts.ExternalServiceType,
		ExternalServiceID:   opts.ExternalServiceID,
	})
	if err != nil {
		return nil, errors.Wrap(err, "loading external service IDs")
	}

	extSvc, err := loadExternalService(ctx, tx.ExternalServices(), database.ExternalServicesListOptions{
		IDs: extSvcIDs,
	})
	if err != nil {
		return nil, errors.Wrap(err, "loading external service")
	}

	css, err := s.newSource(ctx, tx, s.cf, extSvc)
	if err != nil {
		return nil, err
	}
	if as == AuthenticationStrategyGitHubApp {
		return withGitHubAppAuthenticator(ctx, tx, css, extSvc, opts.GitHubAppAccount, opts.GitHubAppKind)
	}
	return css.WithAuthenticator(au)
}

func loadBatchesSource(ctx context.Context, tx SourcerStore, cf *httpcli.Factory, extSvc *types.ExternalService) (ChangesetSource, error) {
	css, err := buildChangesetSource(ctx, tx, cf, extSvc)
	if err != nil {
		return nil, errors.Wrap(err, "building changeset source")
	}
	return css, nil
}

// GitserverPushConfig creates a push configuration given a repo and an
// authenticator. This function is only public for testing purposes, and should
// not be used otherwise.
func GitserverPushConfig(repo *types.Repo, au auth.Authenticator) (*protocol.PushConfig, error) {
	// Empty authenticators are not allowed.
	if au == nil {
		return nil, ErrNoPushCredentials{}
	}

	cloneURL, err := getCloneURL(repo)
	if err != nil {
		return nil, errors.Wrap(err, "getting clone URL")
	}

	// If the repo is cloned using SSH, we need to pass along a private key and passphrase.
	if cloneURL.IsSSH() {
		sshA, ok := au.(auth.AuthenticatorWithSSH)
		if !ok {
			return nil, ErrNoSSHCredential
		}
		privateKey, passphrase := sshA.SSHPrivateKey()
		return &protocol.PushConfig{
			RemoteURL:  cloneURL.String(),
			PrivateKey: privateKey,
			Passphrase: passphrase,
		}, nil
	}

	extSvcType := repo.ExternalRepo.ServiceType
	switch av := au.(type) {
	case *auth.OAuthBearerTokenWithSSH:
		if err := setOAuthTokenAuth(cloneURL, extSvcType, av.Token); err != nil {
			return nil, err
		}
	case *auth.OAuthBearerToken:
		if err := setOAuthTokenAuth(cloneURL, extSvcType, av.Token); err != nil {
			return nil, err
		}

	case *auth.BasicAuthWithSSH:
		if err := setBasicAuth(cloneURL, extSvcType, av.Username, av.Password); err != nil {
			return nil, err
		}
	case *auth.BasicAuth:
		if err := setBasicAuth(cloneURL, extSvcType, av.Username, av.Password); err != nil {
			return nil, err
		}
	default:
		return nil, ErrNoPushCredentials{CredentialsType: fmt.Sprintf("%T", au)}
	}

	return &protocol.PushConfig{RemoteURL: cloneURL.String()}, nil
}

// ToDraftChangesetSource returns a DraftChangesetSource, if the underlying
// source supports it. Returns an error if not.
func ToDraftChangesetSource(css ChangesetSource) (DraftChangesetSource, error) {
	draftCss, ok := css.(DraftChangesetSource)
	if !ok {
		return nil, errors.New("changeset source doesn't implement DraftChangesetSource")
	}
	return draftCss, nil
}

type getBatchChanger interface {
	GetBatchChange(ctx context.Context, opts store.GetBatchChangeOpts) (*btypes.BatchChange, error)
}

func loadBatchChange(ctx context.Context, tx getBatchChanger, id int64) (*btypes.BatchChange, error) {
	if id == 0 {
		return nil, errors.New("changeset has no owning batch change")
	}

	batchChange, err := tx.GetBatchChange(ctx, store.GetBatchChangeOpts{ID: id})
	if err != nil && err != store.ErrNoResults {
		return nil, errors.Wrapf(err, "retrieving owning batch change: %d", id)
	} else if batchChange == nil {
		return nil, errors.Errorf("batch change not found: %d", id)
	}

	return batchChange, nil
}

// withGitHubAppAuthenticator authenticates the given ChangesetSource with a GitHub App
// installation token, if the external service is a GitHub connection and a GitHub App has
// been configured for it for use with Batch Changes in the provided account namespace. If
// the external service is not a GitHub connection, ErrExternalServiceNotGitHub is
// returned. If the external service is a GitHub connection, but no batches domain GitHub
// App has been configured for it, ErrNoGitHubAppConfigured is returned. If a batches
// domain GitHub App has been configured, but no installation exists for the given
// account, ErrNoGitHubAppInstallation is returned.
func withGitHubAppAuthenticator(ctx context.Context, tx SourcerStore, css ChangesetSource, extSvc *types.ExternalService, account string, kind *types.GitHubAppKind) (ChangesetSource, error) {
	if extSvc.Kind != extsvc.KindGitHub {
		return nil, ErrExternalServiceNotGitHub
	}

	cfg, err := extSvc.Configuration(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "loading external service configuration")
	}
	config, ok := cfg.(*schema.GitHubConnection)
	if !ok {
		return nil, errors.Wrap(err, "invalid configuration type")
	}

	baseURL, err := url.Parse(config.Url)
	if err != nil {
		return nil, errors.Wrap(err, "parsing GitHub connection URL")
	}
	baseURL = extsvc.NormalizeBaseURL(baseURL)

	fmt.Println(baseURL.String())
	app, err := tx.GitHubAppsStore().GetByDomainAndKind(ctx, types.BatchesGitHubAppDomain, baseURL.String(), *kind)
	if err != nil {
		return nil, ErrNoGitHubAppConfigured
	}

	fmt.Println("app: ", app.AppID, "account: ", account)
	installID, err := tx.GitHubAppsStore().GetInstallID(ctx, app.AppID, account)
	if err != nil || installID == 0 {
		return nil, ErrNoGitHubAppInstallation
	}

	appAuther, err := ghaauth.NewGitHubAppAuthenticator(app.AppID, []byte(app.PrivateKey))
	if err != nil {
		return nil, errors.Wrap(err, "creating GitHub App authenticator")
	}

	baseURL, err = url.Parse(app.BaseURL)
	if err != nil {
		return nil, errors.Wrap(err, "parsing GitHub App base URL")
	}
	// Unfortunately as of today (2023-05-26), the GitHub REST API only supports signing
	// commits with a GitHub App when it authenticates as an installation, rather than
	// when it authenticates on behalf of a user. This means that commits will be authored
	// by the GitHub App installation bot account, rather than by the user who authored
	// the batch change. If GitHub adds support to their REST API for signing commits with
	// a GitHub App authenticated on behalf of a user, we should switch to using that
	// access token here. See here for more details:
	// https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/about-authentication-with-a-github-app
	installationAuther := ghaauth.NewInstallationAccessToken(baseURL, installID, appAuther, keyring.Default().GitHubAppKey)

	return css.WithAuthenticator(installationAuther)
}

// withAuthenticatorForUser authenticates the given ChangesetSource with a credential
// usable by the given user with userID. User credentials are preferred, with a
// fallback to site credentials. If none of these exist, ErrMissingCredentials
// is returned.
func withAuthenticatorForUser(ctx context.Context, tx SourcerStore, css ChangesetSource, userID int32, repo *types.Repo) (ChangesetSource, error) {
	cred, err := loadUserCredential(ctx, tx, userID, repo)
	if err != nil {
		return nil, errors.Wrap(err, "loading user credential")
	}
	if cred != nil {
		return css.WithAuthenticator(cred)
	}

	// Fall back to site credentials.
	return withSiteAuthenticator(ctx, tx, css, repo)
}

// withSiteAuthenticator uses the site credential of the code host of the passed-in repo.
// If no credential is found, the original source is returned and uses the external service
// config.
func withSiteAuthenticator(ctx context.Context, tx SourcerStore, css ChangesetSource, repo *types.Repo) (ChangesetSource, error) {
	cred, err := loadSiteCredential(ctx, tx, store.GetSiteCredentialOpts{
		ExternalServiceType: repo.ExternalRepo.ServiceType,
		ExternalServiceID:   repo.ExternalRepo.ServiceID,
	})
	if err != nil {
		return nil, errors.Wrap(err, "loading site credential")
	}
	if cred != nil {
		return css.WithAuthenticator(cred)
	}
	return nil, ErrMissingCredentials
}

// loadExternalService looks up all external services that are connected to the
// given repo and returns the first one ordered by id descending. If no external
// service matching the given criteria is found, an error is returned.
func loadExternalService(ctx context.Context, s database.ExternalServiceStore, opts database.ExternalServicesListOptions) (*types.ExternalService, error) {
	es, err := s.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, e := range es {
		cfg, err := e.Configuration(ctx)
		if err != nil {
			return nil, err
		}

		switch cfg.(type) {
		case *schema.GitHubConnection,
			*schema.BitbucketServerConnection,
			*schema.GitLabConnection,
			*schema.BitbucketCloudConnection,
			*schema.AzureDevOpsConnection,
			*schema.GerritConnection,
			*schema.PerforceConnection:
			return e, nil
		}
	}

	return nil, errors.New("no external services found")
}

// buildChangesetSource builds a ChangesetSource for the given repo to load the
// changeset state from.
func buildChangesetSource(ctx context.Context, tx SourcerStore, cf *httpcli.Factory, externalService *types.ExternalService) (ChangesetSource, error) {
	switch externalService.Kind {
	case extsvc.KindGitHub:
		return NewGitHubSource(ctx, tx.DatabaseDB(), externalService, cf)
	case extsvc.KindGitLab:
		return NewGitLabSource(ctx, externalService, cf)
	case extsvc.KindBitbucketServer:
		return NewBitbucketServerSource(ctx, externalService, cf)
	case extsvc.KindBitbucketCloud:
		return NewBitbucketCloudSource(ctx, externalService, cf)
	case extsvc.KindAzureDevOps:
		return NewAzureDevOpsSource(ctx, externalService, cf)
	case extsvc.KindGerrit:
		return NewGerritSource(ctx, externalService, cf)
	case extsvc.KindPerforce:
		return NewPerforceSource(ctx, gitserver.NewClient("batches.perforcesource"), externalService, cf)
	default:
		return nil, errors.Errorf("unsupported external service type %q", extsvc.KindToType(externalService.Kind))
	}
}

// loadUserCredential attempts to find a user credential for the given repo.
// When no credential is found, nil is returned.
func loadUserCredential(ctx context.Context, s SourcerStore, userID int32, repo *types.Repo) (auth.Authenticator, error) {
	cred, err := s.UserCredentials().GetByScope(ctx, database.UserCredentialScope{
		Domain:              database.UserCredentialDomainBatches,
		UserID:              userID,
		ExternalServiceType: repo.ExternalRepo.ServiceType,
		ExternalServiceID:   repo.ExternalRepo.ServiceID,
	})
	if err != nil && !errcode.IsNotFound(err) {
		return nil, err
	}
	if cred != nil {
		return cred.Authenticator(ctx, s)
	}
	return nil, nil
}

// loadSiteCredential attempts to find a site credential for the given repo.
// When no credential is found, nil is returned.
func loadSiteCredential(ctx context.Context, s SourcerStore, opts store.GetSiteCredentialOpts) (auth.Authenticator, error) {
	cred, err := s.GetSiteCredential(ctx, opts)
	if err != nil && err != store.ErrNoResults {
		return nil, err
	}
	if cred != nil {
		return cred.Authenticator(ctx)
	}
	return nil, nil
}

// setOAuthTokenAuth sets the user part of the given URL to use the provided OAuth token,
// with the specific quirks per code host.
func setOAuthTokenAuth(u *vcs.URL, extSvcType, token string) error {
	switch extSvcType {
	case extsvc.TypeGitHub:
		u.User = url.User(token)

	case extsvc.TypeGitLab:
		u.User = url.UserPassword("git", token)

	case extsvc.TypeBitbucketServer:
		return errors.New("require username/token to push commits to BitbucketServer")

	default:
		panic(fmt.Sprintf("setOAuthTokenAuth: invalid external service type %q", extSvcType))
	}
	return nil
}

// setBasicAuth sets the user part of the given URL to use the provided username/
// password combination, with the specific quirks per code host.
func setBasicAuth(u *vcs.URL, extSvcType, username, password string) error {
	switch extSvcType {
	case extsvc.TypeGitHub, extsvc.TypeGitLab:
		return errors.New("need token to push commits to " + extSvcType)
	case extsvc.TypeBitbucketServer, extsvc.TypeBitbucketCloud, extsvc.TypeAzureDevOps, extsvc.TypeGerrit:
		u.User = url.UserPassword(username, password)

	default:
		panic(fmt.Sprintf("setBasicAuth: invalid external service type %q", extSvcType))
	}
	return nil
}

// getCloneURL returns a remote URL for the provided *types.Repo from its Sources,
// preferring HTTPS over SSH.
func getCloneURL(repo *types.Repo) (*vcs.URL, error) {
	cloneURLs := repo.CloneURLs()

	if len(cloneURLs) == 0 {
		return nil, errors.New("no clone URLs found for repo")
	}

	parsedURLs := make([]*vcs.URL, 0, len(cloneURLs))
	for _, cloneURL := range cloneURLs {
		parsedURL, err := vcs.ParseURL(cloneURL)
		if err != nil {
			return nil, err
		}
		parsedURLs = append(parsedURLs, parsedURL)
	}

	sort.SliceStable(parsedURLs, func(i, j int) bool {
		return !parsedURLs[i].IsSSH() && parsedURLs[j].IsSSH()
	})

	return parsedURLs[0], nil
}

var ErrChangesetSourceCannotFork = errors.New("forking is enabled, but the changeset source does not support forks")

// GetRemoteRepo returns the remote that should be pushed to for a given
// changeset, changeset source, and target repo. The changeset spec may
// optionally be provided, and is required if the repo will be pushed to.
func GetRemoteRepo(
	ctx context.Context,
	css ChangesetSource,
	targetRepo *types.Repo,
	ch *btypes.Changeset,
	spec *btypes.ChangesetSpec,
) (*types.Repo, error) {
	// If the changeset spec doesn't expect a fork _and_ we're not updating a
	// changeset that was previously created using a fork, then we don't need to
	// even check if the changeset source is forkable, let alone set up the
	// remote repo: we can just return the target repo and be done with it.
	if ch.ExternalForkNamespace == "" && (spec == nil || !spec.IsFork()) {
		return targetRepo, nil
	}

	fss, ok := css.(ForkableChangesetSource)
	if !ok {
		return nil, ErrChangesetSourceCannotFork
	}

	var repo *types.Repo
	var err error

	// ExternalForkNamespace and ExternalForkName will only be set once a changeset has
	// been published.
	if ch.ExternalForkNamespace != "" {
		// If we're updating an existing changeset, we should push/modify the same fork it
		// was created on, even if the user credential would now fork into a different
		// namespace.
		repo, err = fss.GetFork(ctx, targetRepo, &ch.ExternalForkNamespace, &ch.ExternalForkName)
		if err != nil {
			return nil, errors.Wrap(err, "getting fork for changeset")
		}
		return repo, nil
	}

	// If we're creating a new changeset, we should fork into the namespace specified by
	// the changeset spec, if any.
	namespace := spec.GetForkNamespace()
	repo, err = fss.GetFork(ctx, targetRepo, namespace, nil)
	if err != nil {
		return nil, errors.Wrap(err, "getting fork for changeset spec")
	}
	return repo, nil
}

// DefaultForkName returns the default name assigned when creating a new fork of a
// repository originally from the given namespace and with the given name.
func DefaultForkName(namespace string, name string) string {
	return fmt.Sprintf("%s-%s", namespace, name)
}

// CopyRepoAsFork takes a *types.Repo and returns a copy of it where each
// *types.SourceInfo.CloneURL on its Sources has been updated from nameAndOwner to
// forkNameAndOwner and its Metadata is updated to the provided metadata. This is useful
// because a fork repo that is created by Batch Changes is not necessarily indexed by
// Sourcegraph, but we still need to create a legitimate-seeming *types.Repo for it with
// the right clone URLs, so that we know where to push commits and publish the changeset.
func CopyRepoAsFork(repo *types.Repo, metadata any, nameAndOwner, forkNameAndOwner string) (*types.Repo, error) {
	forkRepo := *repo

	if repo.Sources == nil || len(repo.Sources) == 0 {
		return nil, errors.New("repo has no sources")
	}

	forkSources := map[string]*types.SourceInfo{}

	for urn, src := range repo.Sources {
		if src != nil && src.CloneURL != "" {
			forkURL := strings.Replace(
				strings.ToLower(src.CloneURL),
				strings.ToLower(nameAndOwner),
				strings.ToLower(forkNameAndOwner),
				1,
			)
			forkSources[urn] = &types.SourceInfo{
				ID:       src.ID,
				CloneURL: forkURL,
			}
		}
	}

	forkRepo.Sources = forkSources
	forkRepo.Metadata = metadata

	return &forkRepo, nil
}
