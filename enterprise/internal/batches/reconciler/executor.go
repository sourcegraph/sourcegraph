package reconciler

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/state"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
)

// ExecutePlan executes the given reconciler plan.
func ExecutePlan(ctx context.Context, gitserverClient GitserverClient, sourcer repos.Sourcer, noSleepBeforeSync bool, tx *store.Store, plan *Plan) (err error) {
	e := &executor{
		gitserverClient:   gitserverClient,
		sourcer:           sourcer,
		noSleepBeforeSync: noSleepBeforeSync,
		tx:                tx,
		ch:                plan.Changeset,
		spec:              plan.ChangesetSpec,
		delta:             plan.Delta,
	}

	return e.Run(ctx, plan)
}

type executor struct {
	gitserverClient   GitserverClient
	sourcer           repos.Sourcer
	noSleepBeforeSync bool
	tx                *store.Store

	ccs repos.ChangesetSource

	repo   *types.Repo
	extSvc *types.ExternalService

	// au is nil if we want to use the global credentials stored in the external
	// service configuration.
	au auth.Authenticator

	ch    *batches.Changeset
	spec  *batches.ChangesetSpec
	delta *ChangesetSpecDelta
}

func (e *executor) Run(ctx context.Context, plan *Plan) (err error) {
	if plan.Ops.IsNone() {
		return nil
	}

	e.repo, err = e.tx.Repos().Get(ctx, e.ch.RepoID)
	if err != nil {
		return errors.Wrap(err, "failed to load repository")
	}

	esStore := e.tx.ExternalServices()

	e.extSvc, err = loadExternalService(ctx, esStore, e.repo)
	if err != nil {
		return errors.Wrap(err, "failed to load external service")
	}

	// Figure out which authenticator we should use to modify the changeset.
	e.au, err = e.loadAuthenticator(ctx)
	if err != nil {
		return err
	}

	// Set up a source with which we can modify the changeset.
	e.ccs, err = e.buildChangesetSource(e.repo, e.extSvc)
	if err != nil {
		return err
	}

	for _, op := range plan.Ops.ExecutionOrder() {
		switch op {
		case batches.ReconcilerOperationSync:
			err = e.syncChangeset(ctx)

		case batches.ReconcilerOperationImport:
			err = e.importChangeset(ctx)

		case batches.ReconcilerOperationPush:
			err = e.pushChangesetPatch(ctx)

		case batches.ReconcilerOperationPublish:
			err = e.publishChangeset(ctx, false)

		case batches.ReconcilerOperationPublishDraft:
			err = e.publishChangeset(ctx, true)

		case batches.ReconcilerOperationReopen:
			err = e.reopenChangeset(ctx)

		case batches.ReconcilerOperationUpdate:
			err = e.updateChangeset(ctx)

		case batches.ReconcilerOperationUndraft:
			err = e.undraftChangeset(ctx)

		case batches.ReconcilerOperationClose:
			err = e.closeChangeset(ctx)

		case batches.ReconcilerOperationSleep:
			e.sleep()

		case batches.ReconcilerOperationDetach:
			e.detachChangeset()

		default:
			err = fmt.Errorf("executor operation %q not implemented", op)
		}

		if err != nil {
			return err
		}
	}

	events := e.ch.Events()
	state.SetDerivedState(ctx, e.tx.Repos(), e.ch, events)

	if err := e.tx.UpsertChangesetEvents(ctx, events...); err != nil {
		log15.Error("UpsertChangesetEvents", "err", err)
		return err
	}

	return e.tx.UpdateChangeset(ctx, e.ch)
}

func (e *executor) buildChangesetSource(repo *types.Repo, extSvc *types.ExternalService) (repos.ChangesetSource, error) {
	sources, err := e.sourcer(extSvc)
	if err != nil {
		return nil, err
	}
	if len(sources) != 1 {
		return nil, errors.New("invalid number of sources for external service")
	}
	src := sources[0]

	if e.au != nil {
		// If e.au == nil that means the user that applied that last
		// batch/changeset spec is a site-admin and we can fall back to the
		// global credentials stored in extSvc.
		ucs, ok := src.(repos.UserSource)
		if !ok {
			return nil, errors.Errorf("using user credentials on code host of repo %q is not implemented", repo.Name)
		}

		if src, err = ucs.WithAuthenticator(e.au); err != nil {
			return nil, errors.Wrapf(err, "unable to use this specific user credential on code host of repo %q", repo.Name)
		}
	}

	ccs, ok := src.(repos.ChangesetSource)
	if !ok {
		return nil, errors.Errorf("creating changesets on code host of repo %q is not implemented", repo.Name)
	}

	return ccs, nil
}

// loadAuthenticator determines the correct Authenticator to use when
// reconciling the current changeset. It will return nil, nil if the code host's
// global configuration should be used (ie the applying user is an admin and
// doesn't have a credential configured for the code host, or the changeset
// isn't owned by a batch change).
func (e *executor) loadAuthenticator(ctx context.Context) (auth.Authenticator, error) {
	if e.ch.OwnedByBatchChangeID == 0 {
		// Unowned changesets are imported, and therefore don't need to use a user
		// credential, since reconciliation isn't a mutating process.
		return nil, nil
	}

	// If the changeset is owned by a batch change, we want to reconcile using
	// the user's credentials, which means we need to know which user last
	// applied the owning batch change. Let's go find out.
	batchChange, err := loadBatchChange(ctx, e.tx, e.ch.OwnedByBatchChangeID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load owning batch change")
	}

	cred, err := e.tx.UserCredentials().GetByScope(ctx, database.UserCredentialScope{
		Domain:              database.UserCredentialDomainBatches,
		UserID:              batchChange.LastApplierID,
		ExternalServiceType: e.repo.ExternalRepo.ServiceType,
		ExternalServiceID:   e.repo.ExternalRepo.ServiceID,
	})
	if err != nil {
		if errcode.IsNotFound(err) {
			// We need to check if the user is an admin: if they are, then
			// we can use the nil return from loadUserCredential() to fall
			// back to the global credentials used for the code host. If
			// not, then we need to error out.
			user, err := database.UsersWith(e.tx).GetByID(ctx, batchChange.LastApplierID)
			if err != nil {
				return nil, errors.Wrap(err, "failed to load user applying the batch change")
			}

			if user.SiteAdmin {
				return nil, nil
			}

			return nil, ErrMissingCredentials{repo: string(e.repo.Name)}
		}
		return nil, errors.Wrap(err, "failed to load user credential")
	}

	return cred.Credential, nil
}

// pushChangesetPatch creates the commits for the changeset on its codehost.
func (e *executor) pushChangesetPatch(ctx context.Context) (err error) {
	existingSameBranch, err := e.tx.GetChangeset(ctx, store.GetChangesetOpts{
		ExternalServiceType: e.ch.ExternalServiceType,
		RepoID:              e.ch.RepoID,
		ExternalBranch:      e.spec.Spec.HeadRef,
		// TODO: Do we need to check whether it's published or not?
	})
	if err != nil && err != store.ErrNoResults {
		return err
	}

	if existingSameBranch != nil && existingSameBranch.ID != e.ch.ID {
		return ErrPublishSameBranch{}
	}

	// Create a commit and push it
	opts, err := buildCommitOpts(e.repo, e.extSvc, e.spec, e.au)
	if err != nil {
		return err
	}
	return e.pushCommit(ctx, opts)
}

// publishChangeset creates the given changeset on its code host.
func (e *executor) publishChangeset(ctx context.Context, asDraft bool) (err error) {
	cs := &repos.Changeset{
		Title:     e.spec.Spec.Title,
		Body:      e.spec.Spec.Body,
		BaseRef:   e.spec.Spec.BaseRef,
		HeadRef:   e.spec.Spec.HeadRef,
		Repo:      e.repo,
		Changeset: e.ch,
	}

	// Depending on the changeset, we may want to add to the body (for example,
	// to add a backlink to Sourcegraph).
	if err := decorateChangesetBody(ctx, e.tx, database.NamespacesWith(e.tx), cs); err != nil {
		return errors.Wrapf(err, "decorating body for changeset %d", e.ch.ID)
	}

	var exists bool
	if asDraft {
		// If the changeset shall be published in draft mode, make sure the changeset source implements DraftChangesetSource.
		draftCcs, ok := e.ccs.(repos.DraftChangesetSource)
		if !ok {
			return errors.New("changeset operation is publish-draft, but changeset source doesn't implement DraftChangesetSource")
		}
		exists, err = draftCcs.CreateDraftChangeset(ctx, cs)
	} else {
		// If we're running this method a second time, because we failed due to an
		// ephemeral error, there's a race condition here.
		// It's possible that `CreateChangeset` doesn't return the newest head ref
		// commit yet, because the API of the codehost doesn't return it yet.
		exists, err = e.ccs.CreateChangeset(ctx, cs)
	}
	if err != nil {
		return errors.Wrap(err, "creating changeset")
	}
	// If the Changeset already exists and our source can update it, we try to update it
	if exists {
		outdated, err := cs.IsOutdated()
		if err != nil {
			return errors.Wrap(err, "could not determine whether changeset needs update")
		}

		if outdated {
			if err := e.ccs.UpdateChangeset(ctx, cs); err != nil {
				return errors.Wrap(err, "updating changeset")
			}
		}
	}
	// Set the changeset to published.
	e.ch.PublicationState = batches.ChangesetPublicationStatePublished
	return nil
}

func (e *executor) syncChangeset(ctx context.Context) error {
	if err := e.loadChangeset(ctx); err != nil {
		_, ok := err.(repos.ChangesetNotFoundError)
		if !ok {
			return err
		}

		// If we're syncing a changeset and it can't be found anymore, we mark
		// it as deleted.
		if !e.ch.IsDeleted() {
			e.ch.SetDeleted()
		}
	}

	return nil
}

func (e *executor) importChangeset(ctx context.Context) error {
	if err := e.loadChangeset(ctx); err != nil {
		return err
	}

	// The changeset finished importing, so it is published now.
	e.ch.PublicationState = batches.ChangesetPublicationStatePublished

	return nil
}

func (e *executor) loadChangeset(ctx context.Context) error {
	repoChangeset := &repos.Changeset{Repo: e.repo, Changeset: e.ch}
	return e.ccs.LoadChangeset(ctx, repoChangeset)
}

// updateChangeset updates the given changeset's attribute on the code host
// according to its ChangesetSpec and the delta previously computed.
func (e *executor) updateChangeset(ctx context.Context) (err error) {
	cs := repos.Changeset{
		Title:     e.spec.Spec.Title,
		Body:      e.spec.Spec.Body,
		BaseRef:   e.spec.Spec.BaseRef,
		HeadRef:   e.spec.Spec.HeadRef,
		Repo:      e.repo,
		Changeset: e.ch,
	}

	// Depending on the changeset, we may want to add to the body (for example,
	// to add a backlink to Sourcegraph).
	if err := decorateChangesetBody(ctx, e.tx, database.NamespacesWith(e.tx), &cs); err != nil {
		return errors.Wrapf(err, "decorating body for changeset %d", e.ch.ID)
	}

	if err := e.ccs.UpdateChangeset(ctx, &cs); err != nil {
		return errors.Wrap(err, "updating changeset")
	}

	return nil
}

// reopenChangeset reopens the given changeset attribute on the code host.
func (e *executor) reopenChangeset(ctx context.Context) (err error) {
	cs := repos.Changeset{Repo: e.repo, Changeset: e.ch}
	if err := e.ccs.ReopenChangeset(ctx, &cs); err != nil {
		return errors.Wrap(err, "updating changeset")
	}
	return nil
}

func (e *executor) detachChangeset() {
	for _, assoc := range e.ch.BatchChanges {
		if assoc.Detach {
			e.ch.RemoveBatchChangeID(assoc.BatchChangeID)
		}
	}
}

// closeChangeset closes the given changeset on its code host if its ExternalState is OPEN or DRAFT.
func (e *executor) closeChangeset(ctx context.Context) (err error) {
	e.ch.Closing = false

	if e.ch.ExternalState != batches.ChangesetExternalStateDraft && e.ch.ExternalState != batches.ChangesetExternalStateOpen {
		return nil
	}

	cs := &repos.Changeset{Changeset: e.ch, Repo: e.repo}

	if err := e.ccs.CloseChangeset(ctx, cs); err != nil {
		return errors.Wrap(err, "closing changeset")
	}
	return nil
}

// undraftChangeset marks the given changeset on its code host as ready for review.
func (e *executor) undraftChangeset(ctx context.Context) (err error) {
	draftCcs, ok := e.ccs.(repos.DraftChangesetSource)
	if !ok {
		return errors.New("changeset operation is undraft, but changeset source doesn't implement DraftChangesetSource")
	}

	cs := &repos.Changeset{
		Title:     e.spec.Spec.Title,
		Body:      e.spec.Spec.Body,
		BaseRef:   e.spec.Spec.BaseRef,
		HeadRef:   e.spec.Spec.HeadRef,
		Repo:      e.repo,
		Changeset: e.ch,
	}

	if err := draftCcs.UndraftChangeset(ctx, cs); err != nil {
		return errors.Wrap(err, "undrafting changeset")
	}
	return nil
}

// sleep sleeps for 3 seconds.
func (e *executor) sleep() {
	if !e.noSleepBeforeSync {
		time.Sleep(3 * time.Second)
	}
}

func (e *executor) pushCommit(ctx context.Context, opts protocol.CreateCommitFromPatchRequest) error {
	_, err := e.gitserverClient.CreateCommitFromPatch(ctx, opts)
	if err != nil {
		if diffErr, ok := err.(*protocol.CreateCommitFromPatchError); ok {
			return errors.Errorf(
				"creating commit from patch for repository %q: %s\n"+
					"```\n"+
					"$ %s\n"+
					"%s\n"+
					"```",
				diffErr.RepositoryName, diffErr.InternalError, diffErr.Command, strings.TrimSpace(diffErr.CombinedOutput))
		}
		return err
	}

	return nil
}

// ErrPublishSameBranch is returned by publish changeset if a changeset with
// the same external branch already exists in the database and is owned by
// another batch change.
// It is a terminal error that won't be fixed by retrying to publish the
// changeset with the same spec.
type ErrPublishSameBranch struct{}

func (e ErrPublishSameBranch) Error() string {
	return "cannot create changeset on the same branch in multiple batch changes"
}

func (e ErrPublishSameBranch) NonRetryable() bool { return true }

// ErrMissingCredentials is returned by loadAuthenticator if the user that
// applied the last batch  change/changeset spec doesn't have UserCredentials for
// the given repository and is not a site-admin (so no fallback to the global
// credentials is possible).
type ErrMissingCredentials struct{ repo string }

func (e ErrMissingCredentials) Error() string {
	return fmt.Sprintf("user does not have a valid credential for repository %q", e.repo)
}

func (e ErrMissingCredentials) NonRetryable() bool { return true }

// ErrNoPushCredentials is returned by buildCommitOpts if the credentials
// cannot be used by git to authenticate a `git push`.
type ErrNoPushCredentials struct{ credentialsType string }

func (e ErrNoPushCredentials) Error() string {
	return fmt.Sprintf("cannot use credentials of type %s to push commits", e.credentialsType)
}

func (e ErrNoPushCredentials) NonRetryable() bool { return true }

func buildCommitOpts(repo *types.Repo, extSvc *types.ExternalService, spec *batches.ChangesetSpec, a auth.Authenticator) (protocol.CreateCommitFromPatchRequest, error) {
	var opts protocol.CreateCommitFromPatchRequest

	desc := spec.Spec

	diff, err := desc.Diff()
	if err != nil {
		return opts, err
	}

	commitMessage, err := desc.CommitMessage()
	if err != nil {
		return opts, err
	}

	commitAuthorName, err := desc.AuthorName()
	if err != nil {
		return opts, err
	}

	commitAuthorEmail, err := desc.AuthorEmail()
	if err != nil {
		return opts, err
	}

	source, ok := repo.Sources[extSvc.URN()]
	if !ok {
		return opts, errors.New("repository was not cloned through given external service")
	}

	pushConf, err := buildPushConfig(repo.ExternalRepo.ServiceType, source.CloneURL, a)
	if err != nil {
		return opts, err
	}

	opts = protocol.CreateCommitFromPatchRequest{
		Repo:       repo.Name,
		BaseCommit: api.CommitID(desc.BaseRev),
		// IMPORTANT: We add a trailing newline here, otherwise `git apply`
		// will fail with "corrupt patch at line <N>" where N is the last line.
		Patch:     diff + "\n",
		TargetRef: desc.HeadRef,

		// CAUTION: `UniqueRef` means that we'll push to the branch even if it
		// already exists.
		// So when we retry publishing a changeset, this will overwrite what we
		// pushed before.
		UniqueRef: false,

		CommitInfo: protocol.PatchCommitInfo{
			Message:     commitMessage,
			AuthorName:  commitAuthorName,
			AuthorEmail: commitAuthorEmail,
			Date:        spec.CreatedAt,
		},
		// We use unified diffs, not git diffs, which means they're missing the
		// `a/` and `b/` filename prefixes. `-p0` tells `git apply` to not
		// expect and strip prefixes.
		GitApplyArgs: []string{"-p0"},
		Push:         pushConf,
	}

	return opts, nil
}

// ErrNoSSHCredential is returned by buildPushConfig if the clone URL of the
// repository uses the ssh:// scheme, but the authenticator doesn't support SSH pushes.
type ErrNoSSHCredential struct{}

func (e ErrNoSSHCredential) Error() string {
	return "The used credential doesn't support SSH pushes, but the repo requires pushing over SSH."
}

func (e ErrNoSSHCredential) NonRetryable() bool { return true }

func buildPushConfig(extSvcType, cloneURL string, a auth.Authenticator) (*protocol.PushConfig, error) {
	if a == nil {
		// This is OK: we'll just send no key and gitserver will use
		// the keys installed locally for SSH and the token from the
		// clone URL for https.
		// This path is only triggered when `loadAuthenticator` returns
		// nil, which is only the case for site-admins currently.
		// We want to revisit this once we start disabling usage of global
		// credentials altogether in RFC312.
		return &protocol.PushConfig{RemoteURL: cloneURL}, nil
	}

	u, err := vcs.ParseURL(cloneURL)
	if err != nil {
		return nil, errors.Wrap(err, "parsing repository clone URL")
	}

	// If the repo is cloned using SSH, we need to pass along a private key and passphrase.
	if u.Scheme == "ssh" {
		sshA, ok := a.(auth.AuthenticatorWithSSH)
		if !ok {
			return nil, ErrNoSSHCredential{}
		}
		privateKey, passphrase := sshA.SSHPrivateKey()
		return &protocol.PushConfig{
			RemoteURL:  cloneURL,
			PrivateKey: privateKey,
			Passphrase: passphrase,
		}, nil
	}

	switch av := a.(type) {
	case *auth.OAuthBearerTokenWithSSH:
		if err := setOAuthTokenAuth(u, extSvcType, av.Token); err != nil {
			return nil, err
		}
	case *auth.OAuthBearerToken:
		if err := setOAuthTokenAuth(u, extSvcType, av.Token); err != nil {
			return nil, err
		}

	case *auth.BasicAuthWithSSH:
		if err := setBasicAuth(u, extSvcType, av.Username, av.Password); err != nil {
			return nil, err
		}
	case *auth.BasicAuth:
		if err := setBasicAuth(u, extSvcType, av.Username, av.Password); err != nil {
			return nil, err
		}
	default:
		return nil, ErrNoPushCredentials{credentialsType: fmt.Sprintf("%T", a)}
	}

	return &protocol.PushConfig{RemoteURL: u.String()}, nil
}

func setOAuthTokenAuth(u *url.URL, extsvcType, token string) error {
	switch extsvcType {
	case extsvc.TypeGitHub:
		u.User = url.User(token)

	case extsvc.TypeGitLab:
		u.User = url.UserPassword("git", token)

	case extsvc.TypeBitbucketServer:
		return errors.New("require username/token to push commits to BitbucketServer")
	}
	return nil
}

func setBasicAuth(u *url.URL, extSvcType, username, password string) error {
	switch extSvcType {
	case extsvc.TypeGitHub, extsvc.TypeGitLab:
		return errors.New("need token to push commits to " + extSvcType)

	case extsvc.TypeBitbucketServer:
		u.User = url.UserPassword(username, password)
	}
	return nil
}

type getBatchChanger interface {
	GetBatchChange(ctx context.Context, opts store.CountBatchChangeOpts) (*batches.BatchChange, error)
}

func loadBatchChange(ctx context.Context, tx getBatchChanger, id int64) (*batches.BatchChange, error) {
	if id == 0 {
		return nil, errors.New("changeset has no owning batch change")
	}

	batchChange, err := tx.GetBatchChange(ctx, store.CountBatchChangeOpts{ID: id})
	if err != nil && err != store.ErrNoResults {
		return nil, errors.Wrapf(err, "retrieving owning batch change: %d", id)
	} else if batchChange == nil {
		return nil, errors.Errorf("batch change not found: %d", id)
	}

	return batchChange, nil
}

type getNamespacer interface {
	GetByID(ctx context.Context, orgID, userID int32) (*database.Namespace, error)
}

func decorateChangesetBody(ctx context.Context, tx getBatchChanger, nsStore getNamespacer, cs *repos.Changeset) error {
	batchChange, err := loadBatchChange(ctx, tx, cs.OwnedByBatchChangeID)
	if err != nil {
		return errors.Wrap(err, "failed to load batch change")
	}

	// We need to get the namespace, since external batch change URLs are
	// namespaced.
	ns, err := nsStore.GetByID(ctx, batchChange.NamespaceOrgID, batchChange.NamespaceUserID)
	if err != nil {
		return errors.Wrap(err, "retrieving namespace")
	}

	u, err := batchChangeURL(ctx, ns, batchChange)
	if err != nil {
		return errors.Wrap(err, "building URL")
	}

	cs.Body = fmt.Sprintf(
		"%s\n\n[_Created by Sourcegraph batch change `%s/%s`._](%s)",
		cs.Body, ns.Name, batchChange.Name, u,
	)

	return nil
}

// internalClient is here for mocking reasons.
var internalClient interface {
	ExternalURL(context.Context) (string, error)
} = api.InternalClient

func batchChangeURL(ctx context.Context, ns *database.Namespace, c *batches.BatchChange) (string, error) {
	// To build the absolute URL, we need to know where Sourcegraph is!
	extStr, err := internalClient.ExternalURL(ctx)
	if err != nil {
		return "", errors.Wrap(err, "getting external Sourcegraph URL")
	}

	extURL, err := url.Parse(extStr)
	if err != nil {
		return "", errors.Wrap(err, "parsing external Sourcegraph URL")
	}

	// This needs to be kept consistent with resolvers.batchChangeURL().
	// (Refactoring the resolver to use the same function is difficult due to
	// the different querying and caching behaviour in GraphQL resolvers, so we
	// simply replicate the logic here.)
	u := extURL.ResolveReference(&url.URL{Path: namespaceURL(ns) + "/batch-changes/" + c.Name})

	return u.String(), nil
}

func namespaceURL(ns *database.Namespace) string {
	prefix := "/users/"
	if ns.Organization != 0 {
		prefix = "/organizations/"
	}

	return prefix + ns.Name
}
