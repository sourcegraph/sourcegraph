package reconciler

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"github.com/sourcegraph/log"

	bgql "github.com/sourcegraph/sourcegraph/internal/batches/graphql"
	"github.com/sourcegraph/sourcegraph/internal/batches/sources"
	"github.com/sourcegraph/sourcegraph/internal/batches/state"
	"github.com/sourcegraph/sourcegraph/internal/batches/store"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/batches/webhooks"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// executePlan executes the given reconciler plan.
func executePlan(ctx context.Context, logger log.Logger, cli httpcli.Doer, client gitserver.Client, sourcer sources.Sourcer, noSleepBeforeSync bool, tx *store.Store, plan *Plan) (afterDone func(store *store.Store), err error) {
	e := &executor{
		client:            client,
		logger:            logger.Scoped("executor"),
		cli:               cli,
		sourcer:           sourcer,
		noSleepBeforeSync: noSleepBeforeSync,
		tx:                tx,
		ch:                plan.Changeset,
		spec:              plan.ChangesetSpec,
	}

	return e.Run(ctx, plan)
}

type executor struct {
	client            gitserver.Client
	logger            log.Logger
	cli               httpcli.Doer
	sourcer           sources.Sourcer
	noSleepBeforeSync bool
	tx                *store.Store
	ch                *btypes.Changeset
	spec              *btypes.ChangesetSpec

	// targetRepo represents the repo where the changeset should be opened.
	targetRepo *types.Repo

	// css represents the changeset source, and must be accessed via the
	// changesetSource method.
	css     sources.ChangesetSource
	cssErr  error
	cssOnce sync.Once

	// remote represents the repo that should be pushed to, and must be accessed
	// via the remoteRepo method.
	remote     *types.Repo
	remoteErr  error
	remoteOnce sync.Once
}

func (e *executor) Run(ctx context.Context, plan *Plan) (afterDone func(store *store.Store), err error) {
	if plan.Ops.IsNone() {
		return nil, nil
	}

	// Load the target repo.
	//
	// Note that the remote repo is lazily set when a changeset source is
	// requested, since it isn't useful outside of that context.
	e.targetRepo, err = e.tx.Repos().Get(ctx, e.ch.RepoID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load repository")
	}

	// If we are only pushing, without publishing or updating, we want to be sure to
	// trigger a webhooks.ChangesetUpdate event for this operation as well.
	var triggerUpdateWebhook bool
	if plan.Ops.Contains(btypes.ReconcilerOperationPush) && !plan.Ops.Contains(btypes.ReconcilerOperationPublish) && !plan.Ops.Contains(btypes.ReconcilerOperationUpdate) {
		triggerUpdateWebhook = true
	}

	for _, op := range plan.Ops.ExecutionOrder() {
		switch op {
		case btypes.ReconcilerOperationSync:
			err = e.syncChangeset(ctx)

		case btypes.ReconcilerOperationImport:
			err = e.importChangeset(ctx)

		case btypes.ReconcilerOperationPush:
			afterDone, err = e.pushChangesetPatch(ctx, triggerUpdateWebhook)

		case btypes.ReconcilerOperationPublish:
			afterDone, err = e.publishChangeset(ctx, false)

		case btypes.ReconcilerOperationPublishDraft:
			afterDone, err = e.publishChangeset(ctx, true)

		case btypes.ReconcilerOperationReopen:
			afterDone, err = e.reopenChangeset(ctx)

		case btypes.ReconcilerOperationUpdate:
			afterDone, err = e.updateChangeset(ctx)

		case btypes.ReconcilerOperationUndraft:
			afterDone, err = e.undraftChangeset(ctx)

		case btypes.ReconcilerOperationClose:
			afterDone, err = e.closeChangeset(ctx)

		case btypes.ReconcilerOperationSleep:
			e.sleep()

		case btypes.ReconcilerOperationDetach:
			e.detachChangeset()

		case btypes.ReconcilerOperationArchive:
			e.archiveChangeset()

		case btypes.ReconcilerOperationReattach:
			e.reattachChangeset()

		default:
			err = errors.Errorf("executor operation %q not implemented", op)
		}

		if err != nil {
			return afterDone, err
		}
	}

	events, err := e.ch.Events()
	if err != nil {
		log15.Error("Events", "err", err)
		return afterDone, errcode.MakeNonRetryable(err)
	}
	state.SetDerivedState(ctx, e.tx.Repos(), e.client, e.ch, events)

	if err := e.tx.UpsertChangesetEvents(ctx, events...); err != nil {
		log15.Error("UpsertChangesetEvents", "err", err)
		return afterDone, err
	}

	e.ch.PreviousFailureMessage = nil

	return afterDone, e.tx.UpdateChangeset(ctx, e.ch)
}

var errCannotPushToArchivedRepo = errcode.MakeNonRetryable(errors.New("cannot push to an archived repo"))

// pushChangesetPatch creates the commits for the changeset on its codehost. If the option
// triggerUpdateWebhook is set, it will also enqueue an update webhook for the changeset.
func (e *executor) pushChangesetPatch(ctx context.Context, triggerUpdateWebhook bool) (afterDone func(store *store.Store), err error) {
	if triggerUpdateWebhook {
		afterDone = func(store *store.Store) { e.enqueueWebhook(ctx, store, webhooks.ChangesetUpdateError) }
	}

	existingSameBranch, err := e.tx.GetChangeset(ctx, store.GetChangesetOpts{
		ExternalServiceType: e.ch.ExternalServiceType,
		RepoID:              e.ch.RepoID,
		ExternalBranch:      e.spec.HeadRef,
		// TODO: Do we need to check whether it's published or not?
	})
	if err != nil && err != store.ErrNoResults {
		return afterDone, err
	}

	if existingSameBranch != nil && existingSameBranch.ID != e.ch.ID {
		return afterDone, errPublishSameBranch{}
	}

	// Create a commit and push it
	// Figure out which authenticator we should use to modify the changeset.
	css, err := e.changesetSource(ctx)

	if err != nil {
		return afterDone, err
	}
	remoteRepo, err := e.remoteRepo(ctx)
	if err != nil {
		return afterDone, err
	}

	// Short circuit any attempt to push to an archived repo, since we can save
	// gitserver the work (and it'll keep retrying).
	if remoteRepo.Archived {
		return afterDone, errCannotPushToArchivedRepo
	}

	pushConf, err := css.GitserverPushConfig(remoteRepo)
	if err != nil {
		return afterDone, err
	}
	opts := css.BuildCommitOpts(e.targetRepo, e.ch, e.spec, pushConf)
	resp, err := e.pushCommit(ctx, opts)
	if err != nil {
		var pce pushCommitError
		if errors.As(err, &pce) {
			if acss, ok := css.(sources.ArchivableChangesetSource); ok {
				if acss.IsArchivedPushError(pce.CombinedOutput) {
					if err := e.handleArchivedRepo(ctx); err != nil {
						return afterDone, errors.Wrap(err, "handling archived repo")
					}
					return afterDone, errCannotPushToArchivedRepo
				}
			}
			// do not wrap the error (pushCommitError), so it can be nicely displayed in the UI
			return afterDone, err
		}
		return afterDone, errors.Wrap(err, "pushing commit")
	}

	// update the changeset's external_id column if a changelist id is returned
	// because that's going to make it back to the UI so that the user can see the changelist id and take action on it
	if resp != nil && resp.ChangelistId != "" {
		e.ch.ExternalID = resp.ChangelistId
	}

	if err = e.runAfterCommit(ctx, css, resp, remoteRepo, opts); err != nil {
		return afterDone, errors.Wrap(err, "running after commit routine")
	}

	if triggerUpdateWebhook && err == nil {
		afterDone = func(store *store.Store) { e.enqueueWebhook(ctx, store, webhooks.ChangesetUpdate) }
	}
	return afterDone, err
}

// publishChangeset creates the given changeset on its code host.
func (e *executor) publishChangeset(ctx context.Context, asDraft bool) (afterDone func(store *store.Store), err error) {
	afterDoneUpdate := func(store *store.Store) { e.enqueueWebhook(ctx, store, webhooks.ChangesetUpdateError) }
	afterDonePublish := func(store *store.Store) { e.enqueueWebhook(ctx, store, webhooks.ChangesetUpdateError) }

	// Depending on the changeset, we may want to add to the body (for example,
	// to add a backlink to Sourcegraph).
	body, err := e.decorateChangesetBody(ctx)
	if err != nil {
		// At this point in time, we haven't yet established if the changeset has already
		// been published or not. When in doubt, we record a more generic "update error"
		// event.
		return afterDoneUpdate, errors.Wrapf(err, "decorating body for changeset %d", e.ch.ID)
	}

	css, err := e.changesetSource(ctx)
	if err != nil {
		return afterDoneUpdate, err
	}

	remoteRepo, err := e.remoteRepo(ctx)
	if err != nil {
		return afterDoneUpdate, err
	}

	cs := &sources.Changeset{
		Title:      e.spec.Title,
		Body:       body,
		BaseRef:    e.spec.BaseRef,
		HeadRef:    e.spec.HeadRef,
		RemoteRepo: remoteRepo,
		TargetRepo: e.targetRepo,
		Changeset:  e.ch,
	}

	var exists, outdated bool
	if asDraft {
		// If the changeset shall be published in draft mode, make sure the changeset source implements DraftChangesetSource.
		draftCss, err := sources.ToDraftChangesetSource(css)
		if err != nil {
			return afterDoneUpdate, err
		}
		exists, err = draftCss.CreateDraftChangeset(ctx, cs)
		if err != nil {
			// For several code hosts, it's also impossible to tell if a changeset exists
			// already or not, yet. Since we're here *intending* to publish, we'll just
			// emit ChangesetPublish webhook events here.
			return afterDonePublish, errors.Wrap(err, "creating draft changeset")
		}
	} else {
		// If we're running this method a second time, because we failed due to an
		// ephemeral error, there's a race condition here.
		// It's possible that `CreateChangeset` doesn't return the newest head ref
		// commit yet, because the API of the codehost doesn't return it yet.
		exists, err = css.CreateChangeset(ctx, cs)
		if err != nil {
			// For several code hosts, it's also impossible to tell if a changeset exists
			// already or not, yet. Since we're here *intending* to publish, we'll just
			// emit ChangesetPublish webhook events here.
			return afterDonePublish, errors.Wrap(err, "creating changeset")
		}
	}

	// If the Changeset already exists and our source can update it, we try to update it
	if exists {
		outdated, err = cs.IsOutdated()
		if err != nil {
			return afterDonePublish, errors.Wrap(err, "could not determine whether changeset needs update")
		}

		// If the changeset is actually outdated, we can be reasonably sure it already
		// exists on the code host. Here, we'll emit a ChangesetUpdate webhook event.
		if outdated {
			if err := css.UpdateChangeset(ctx, cs); err != nil {
				return afterDoneUpdate, errors.Wrap(err, "updating changeset")
			}
		}
	}

	// Set the changeset to published.
	e.ch.PublicationState = btypes.ChangesetPublicationStatePublished

	// Enqueue the appropriate webhook.
	if exists && outdated {
		afterDone = func(store *store.Store) { e.enqueueWebhook(ctx, store, webhooks.ChangesetUpdate) }
	} else {
		afterDone = func(store *store.Store) { e.enqueueWebhook(ctx, store, webhooks.ChangesetPublish) }
	}

	return afterDone, nil
}

func (e *executor) syncChangeset(ctx context.Context) error {
	if err := e.loadChangeset(ctx); err != nil {
		if !errors.HasType(err, sources.ChangesetNotFoundError{}) {
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
	e.ch.PublicationState = btypes.ChangesetPublicationStatePublished

	return nil
}

func (e *executor) loadChangeset(ctx context.Context) error {
	css, err := e.changesetSource(ctx)
	if err != nil {
		return err
	}

	remoteRepo, err := e.remoteRepo(ctx)
	if err != nil {
		return err
	}

	repoChangeset := &sources.Changeset{
		RemoteRepo: remoteRepo,
		TargetRepo: e.targetRepo,
		Changeset:  e.ch,
	}
	return css.LoadChangeset(ctx, repoChangeset)
}

// updateChangeset updates the given changeset's attribute on the code host
// according to its ChangesetSpec and the delta previously computed.
func (e *executor) updateChangeset(ctx context.Context) (afterDone func(store *store.Store), err error) {
	afterDone = func(store *store.Store) { e.enqueueWebhook(ctx, store, webhooks.ChangesetUpdateError) }
	// Depending on the changeset, we may want to add to the body (for example,
	// to add a backlink to Sourcegraph).
	body, err := e.decorateChangesetBody(ctx)
	if err != nil {
		return afterDone, errors.Wrapf(err, "decorating body for changeset %d", e.ch.ID)
	}

	css, err := e.changesetSource(ctx)
	if err != nil {
		return afterDone, err
	}

	remoteRepo, err := e.remoteRepo(ctx)
	if err != nil {
		return afterDone, err
	}

	// We must construct the sources.Changeset after invoking changesetSource,
	// since that may change the remoteRepo.
	cs := sources.Changeset{
		Title:      e.spec.Title,
		Body:       body,
		BaseRef:    e.spec.BaseRef,
		HeadRef:    e.spec.HeadRef,
		RemoteRepo: remoteRepo,
		TargetRepo: e.targetRepo,
		Changeset:  e.ch,
	}

	if err := css.UpdateChangeset(ctx, &cs); err != nil {
		if errcode.IsArchived(err) {
			if err := e.handleArchivedRepo(ctx); err != nil {
				return afterDone, err
			}
		} else {
			return afterDone, errors.Wrap(err, "updating changeset")
		}
	}

	afterDone = func(store *store.Store) { e.enqueueWebhook(ctx, store, webhooks.ChangesetUpdate) }
	return afterDone, nil
}

// reopenChangeset reopens the given changeset attribute on the code host.
func (e *executor) reopenChangeset(ctx context.Context) (afterDone func(store *store.Store), err error) {
	afterDone = func(store *store.Store) { e.enqueueWebhook(ctx, store, webhooks.ChangesetUpdateError) }

	css, err := e.changesetSource(ctx)
	if err != nil {
		return afterDone, err
	}

	remoteRepo, err := e.remoteRepo(ctx)
	if err != nil {
		return afterDone, err
	}

	cs := sources.Changeset{
		Title:      e.spec.Title,
		Body:       e.spec.Body,
		BaseRef:    e.spec.BaseRef,
		HeadRef:    e.spec.HeadRef,
		RemoteRepo: remoteRepo,
		TargetRepo: e.targetRepo,
		Changeset:  e.ch,
	}
	if err := css.ReopenChangeset(ctx, &cs); err != nil {
		return afterDone, errors.Wrap(err, "reopening changeset")
	}

	afterDone = func(store *store.Store) { e.enqueueWebhook(ctx, store, webhooks.ChangesetUpdate) }
	return afterDone, nil
}

func (e *executor) detachChangeset() {
	for _, assoc := range e.ch.BatchChanges {
		if assoc.Detach {
			e.ch.RemoveBatchChangeID(assoc.BatchChangeID)
		}
	}
	// A changeset can be associated with multiple batch changes. Only set the detached_at field when the changeset is
	// no longer associated with any batch changes.
	if len(e.ch.BatchChanges) == 0 {
		e.ch.DetachedAt = time.Now()
	}
}

// archiveChangeset sets all associations to archived that are marked as "to-be-archived".
func (e *executor) archiveChangeset() {
	for i, assoc := range e.ch.BatchChanges {
		if assoc.Archive {
			e.ch.BatchChanges[i].IsArchived = true
			e.ch.BatchChanges[i].Archive = false
		}
	}
}

// reattachChangeset resets detached_at to zero.
func (e *executor) reattachChangeset() {
	if !e.ch.DetachedAt.IsZero() {
		e.ch.DetachedAt = time.Time{}
	}
}

// closeChangeset closes the given changeset on its code host if its ExternalState is OPEN or DRAFT.
func (e *executor) closeChangeset(ctx context.Context) (afterDone func(store *store.Store), err error) {
	afterDone = func(store *store.Store) { e.enqueueWebhook(ctx, store, webhooks.ChangesetUpdateError) }

	e.ch.Closing = false

	if e.ch.ExternalState != btypes.ChangesetExternalStateDraft && e.ch.ExternalState != btypes.ChangesetExternalStateOpen {
		// no-op
		return nil, nil
	}

	css, err := e.changesetSource(ctx)
	if err != nil {
		return afterDone, err
	}

	remoteRepo, err := e.remoteRepo(ctx)
	if err != nil {
		return afterDone, err
	}

	cs := &sources.Changeset{
		Changeset:  e.ch,
		RemoteRepo: remoteRepo,
		TargetRepo: e.targetRepo,
	}

	if err := css.CloseChangeset(ctx, cs); err != nil {
		return afterDone, errors.Wrap(err, "closing changeset")
	}

	afterDone = func(store *store.Store) { e.enqueueWebhook(ctx, store, webhooks.ChangesetClose) }
	return afterDone, nil
}

// undraftChangeset marks the given changeset on its code host as ready for review.
func (e *executor) undraftChangeset(ctx context.Context) (afterDone func(store *store.Store), err error) {
	afterDone = func(store *store.Store) { e.enqueueWebhook(ctx, store, webhooks.ChangesetUpdateError) }

	css, err := e.changesetSource(ctx)
	if err != nil {
		return afterDone, err
	}

	draftCss, err := sources.ToDraftChangesetSource(css)
	if err != nil {
		return afterDone, err
	}

	remoteRepo, err := e.remoteRepo(ctx)
	if err != nil {
		return afterDone, nil
	}

	cs := &sources.Changeset{
		Title:      e.spec.Title,
		Body:       e.spec.Body,
		BaseRef:    e.spec.BaseRef,
		HeadRef:    e.spec.HeadRef,
		RemoteRepo: remoteRepo,
		TargetRepo: e.targetRepo,
		Changeset:  e.ch,
	}

	if err := draftCss.UndraftChangeset(ctx, cs); err != nil {
		return afterDone, errors.Wrap(err, "undrafting changeset")
	}

	afterDone = func(store *store.Store) { e.enqueueWebhook(ctx, store, webhooks.ChangesetUpdate) }
	return afterDone, nil
}

// sleep sleeps for 3 seconds.
func (e *executor) sleep() {
	if !e.noSleepBeforeSync {
		time.Sleep(3 * time.Second)
	}
}

func (e *executor) changesetSource(ctx context.Context) (sources.ChangesetSource, error) {
	e.cssOnce.Do(func() {
		e.css, e.cssErr = loadChangesetSource(ctx, e.tx, e.sourcer, e.ch, e.targetRepo)
		if e.cssErr != nil {
			return
		}
	})

	return e.css, e.cssErr
}

func (e *executor) remoteRepo(ctx context.Context) (*types.Repo, error) {
	e.remoteOnce.Do(func() {
		css, err := e.changesetSource(ctx)
		if err != nil {
			e.remoteErr = errors.Wrap(err, "getting changeset source")
			return
		}

		// Set the remote repo, which may not be the same as the target repo if
		// forking is enabled.
		e.remote, e.remoteErr = sources.GetRemoteRepo(ctx, css, e.targetRepo, e.ch, e.spec)
	})

	return e.remote, e.remoteErr
}

func (e *executor) decorateChangesetBody(ctx context.Context) (string, error) {
	return decorateChangesetBody(ctx, e.tx, database.NamespacesWith(e.tx), e.ch, e.spec.Body)
}

func loadChangesetSource(ctx context.Context, s *store.Store, sourcer sources.Sourcer, ch *btypes.Changeset, repo *types.Repo) (sources.ChangesetSource, error) {
	css, err := sourcer.ForChangeset(ctx, s, ch, sources.AuthenticationStrategyUserCredential, repo)
	if err != nil {
		switch err {
		case sources.ErrMissingCredentials:
			return nil, &errMissingCredentials{repo: string(repo.Name)}
		case sources.ErrNoSSHCredential:
			return nil, &errNoSSHCredential{}
		default:
			var e sources.ErrNoPushCredentials
			if errors.As(err, &e) {
				return nil, &errNoPushCredentials{credentialsType: e.CredentialsType}
			}
			return nil, err
		}
	}

	return css, nil
}

type pushCommitError struct {
	*protocol.CreateCommitFromPatchError
}

func (e pushCommitError) Error() string {
	return fmt.Sprintf(
		"creating commit from patch for repository %q: %s\n"+
			"```\n"+
			"$ %s\n"+
			"%s\n"+
			"```",
		e.RepositoryName, e.InternalError, e.Command, strings.TrimSpace(e.CombinedOutput))
}

func (e *executor) pushCommit(ctx context.Context, opts protocol.CreateCommitFromPatchRequest) (*protocol.CreateCommitFromPatchResponse, error) {
	res, err := e.client.CreateCommitFromPatch(ctx, opts)
	if err != nil {
		var e *protocol.CreateCommitFromPatchError
		if errors.As(err, &e) {
			// Make "patch does not apply" errors a fatal error. Retrying the changeset
			// rollout won't help here and just causes noise.
			if strings.Contains(e.CombinedOutput, "patch does not apply") {
				return nil, errcode.MakeNonRetryable(pushCommitError{e})
			}
			return nil, pushCommitError{e}
		}
		return nil, err
	}

	return res, nil
}

func (e *executor) runAfterCommit(ctx context.Context, css sources.ChangesetSource, resp *protocol.CreateCommitFromPatchResponse, remoteRepo *types.Repo, opts protocol.CreateCommitFromPatchRequest) (err error) {
	rejectUnverifiedCommit := conf.RejectUnverifiedCommit()

	// If we're pushing to a GitHub code host, we should check if a GitHub App is
	// configured for Batch Changes to sign commits on this code host with.
	if _, ok := css.(*sources.GitHubSource); ok {
		// Attempt to get a ChangesetSource authenticated with a GitHub App.
		css, err = e.sourcer.ForChangeset(ctx, e.tx, e.ch, sources.AuthenticationStrategyGitHubApp, e.remote)
		if err != nil {
			switch err {
			case sources.ErrNoGitHubAppConfigured:
				if rejectUnverifiedCommit {
					return errors.New("no GitHub App configured to sign commit, rejecting unverified commit")
				}
				// If we didn't find any GitHub Apps configured for this code host, it's a
				// noop; commit signing is not set up for this code host.
			default:
				if rejectUnverifiedCommit {
					return errors.Wrap(err, "failed to get GitHub App for commit verification")
				}
				log15.Error("Failed to get GitHub App authenticated ChangesetSource", "err", err)
			}
		} else {
			// We found a GitHub App configured for Batch Changes; we should try to use it
			// to sign the commit.
			gcss, ok := css.(*sources.GitHubSource)
			if !ok {
				return errors.Wrap(err, "got non-GitHubSource for ChangesetSource when using GitHub App authentication strategy")
			}
			// Find the revision from the response from CreateCommitFromPatch.
			if resp == nil {
				return errors.New("no response from CreateCommitFromPatch")
			}
			rev := resp.Rev
			// We use the existing commit as the basis for the new commit, duplicating it
			// over the REST API in order to produce a signed version of it to replace the
			// original one with.
			newCommit, err := gcss.DuplicateCommit(ctx, opts, remoteRepo, rev)
			if err != nil {
				return errors.Wrap(err, "failed to duplicate commit")
			}
			if newCommit.Verification.Verified {
				err = e.tx.UpdateChangesetCommitVerification(ctx, e.ch, newCommit)
				if err != nil {
					return errors.Wrap(err, "failed to update changeset with commit verification")
				}
			} else {
				if rejectUnverifiedCommit {
					return errors.Wrap(err, "commit created with GitHub App was not signed, rejecting unverified commit")
				}
				log15.Warn("Commit created with GitHub App was not signed", "changeset", e.ch.ID, "commit", newCommit.SHA)
			}
		}
	}
	return nil
}

// handleArchivedRepo updates the changeset and repo once it has been
// determined that the repo has been archived.
func (e *executor) handleArchivedRepo(ctx context.Context) error {
	repo, err := e.remoteRepo(ctx)
	if err != nil {
		return errors.Wrap(err, "getting the archived remote repo")
	}

	return handleArchivedRepo(
		ctx,
		repos.NewStore(e.logger, e.tx.DatabaseDB()),
		repo,
		e.ch,
	)
}

func handleArchivedRepo(
	ctx context.Context,
	store repos.Store,
	repo *types.Repo,
	ch *btypes.Changeset,
) error {
	// We need to mark the repo as archived so that the later check for whether
	// the repo is still archived isn't confused.
	repo.Archived = true
	if _, err := store.UpdateRepo(ctx, repo); err != nil {
		return errors.Wrapf(err, "updating archived status of repo %d", int(repo.ID))
	}

	// Now we can set the ExternalState, and SetDerivedState will do the rest
	// later with that and the updated repo.
	ch.ExternalState = btypes.ChangesetExternalStateReadOnly

	return nil
}

func (e *executor) enqueueWebhook(ctx context.Context, store *store.Store, eventType string) {
	webhooks.EnqueueChangeset(ctx, e.logger, e.cli, store, eventType, bgql.MarshalChangesetID(e.ch.ID))
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

type getNamespacer interface {
	GetByID(ctx context.Context, orgID, userID int32) (*database.Namespace, error)
}

func decorateChangesetBody(ctx context.Context, tx getBatchChanger, nsStore getNamespacer, cs *btypes.Changeset, body string) (string, error) {
	batchChange, err := loadBatchChange(ctx, tx, cs.OwnedByBatchChangeID)
	if err != nil {
		return "", errors.Wrap(err, "failed to load batch change")
	}

	// We need to get the namespace, since external batch change URLs are
	// namespaced.
	ns, err := nsStore.GetByID(ctx, batchChange.NamespaceOrgID, batchChange.NamespaceUserID)
	if err != nil {
		return "", errors.Wrap(err, "retrieving namespace")
	}

	u, err := batchChange.URL(ctx, ns.Name)
	if err != nil {
		return "", errors.Wrap(err, "building URL")
	}

	bcl := fmt.Sprintf("[_Created by Sourcegraph batch change `%s/%s`._](%s)", ns.Name, batchChange.Name, u)

	// Check if the batch change link template variable is present in the changeset
	// template body.
	if strings.Contains(body, "batch_change_link") {
		// Since we already ran this template before, `cs.Body` should only contain valid templates for `batch_change_link` at this point.
		t, err := template.New("changeset_template").Delims("${{", "}}").Funcs(template.FuncMap{"batch_change_link": func() string { return bcl }}).Parse(body)
		if err != nil {
			return "", errors.Wrap(err, "handling batch_change_link: parsing changeset template")
		}

		var out bytes.Buffer
		if err := t.Execute(&out, nil); err != nil {
			return "", errors.Wrap(err, "handling batch_change_link: executing changeset template")
		}

		return out.String(), nil
	}

	// Otherwise, append to the end of the body.
	return fmt.Sprintf("%s\n\n%s", body, bcl), nil
}

// errPublishSameBranch is returned by publish changeset if a changeset with
// the same external branch already exists in the database and is owned by
// another batch change.
// It is a terminal error that won't be fixed by retrying to publish the
// changeset with the same spec.
type errPublishSameBranch struct{}

func (e errPublishSameBranch) Error() string {
	return "cannot create changeset on the same branch in multiple batch changes"
}

func (e errPublishSameBranch) NonRetryable() bool { return true }

// errNoSSHCredential is returned, if the  clone URL of the repository uses the
// ssh:// scheme, but the authenticator doesn't support SSH pushes.
type errNoSSHCredential struct{}

func (e errNoSSHCredential) Error() string {
	return "The used credential doesn't support SSH pushes, but the repo requires pushing over SSH."
}

func (e errNoSSHCredential) NonRetryable() bool { return true }

// errMissingCredentials is returned if the user that applied the last batch change
// /changeset spec doesn't have a user credential for the given repository and is
// not a site-admin (so no fallback to the global credentials is possible).
type errMissingCredentials struct{ repo string }

func (e errMissingCredentials) Error() string {
	return fmt.Sprintf("user does not have a valid credential for repository %q", e.repo)
}

func (e errMissingCredentials) NonRetryable() bool { return true }

func (e errMissingCredentials) Is(target error) bool {
	if t, ok := target.(errMissingCredentials); ok && t.repo == e.repo {
		return true
	}
	return false
}

// errNoPushCredentials is returned if the authenticator cannot be used by git to
// authenticate a `git push`.
type errNoPushCredentials struct{ credentialsType string }

func (e errNoPushCredentials) Error() string {
	return fmt.Sprintf("cannot use credentials of type %s to push commits", e.credentialsType)
}

func (e errNoPushCredentials) NonRetryable() bool { return true }
