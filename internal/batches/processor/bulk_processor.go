package processor

import (
	"context"
	"fmt"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/batches/global"
	bgql "github.com/sourcegraph/sourcegraph/internal/batches/graphql"
	"github.com/sourcegraph/sourcegraph/internal/batches/service"
	"github.com/sourcegraph/sourcegraph/internal/batches/sources"
	"github.com/sourcegraph/sourcegraph/internal/batches/state"
	"github.com/sourcegraph/sourcegraph/internal/batches/store"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/batches/webhooks"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// unknownJobTypeErr is returned when a ChangesetJob record is of an unknown type
// and hence cannot be executed.
type unknownJobTypeErr struct {
	jobType string
}

func (e unknownJobTypeErr) Error() string {
	return fmt.Sprintf("invalid job type %q", e.jobType)
}

func (e unknownJobTypeErr) NonRetryable() bool {
	return true
}

var changesetIsProcessingErr = errors.New("cannot update a changeset that is currently being processed; will retry")

func New(logger log.Logger, tx *store.Store, sourcer sources.Sourcer) BulkProcessor {
	return &bulkProcessor{
		tx:      tx,
		sourcer: sourcer,
		logger:  logger,
	}
}

type BulkProcessor interface {
	Process(ctx context.Context, job *btypes.ChangesetJob) (afterDone func(*store.Store), err error)
}

type bulkProcessor struct {
	tx      *store.Store
	sourcer sources.Sourcer
	logger  log.Logger

	css  sources.ChangesetSource
	repo *types.Repo
	ch   *btypes.Changeset
}

func (b *bulkProcessor) Process(ctx context.Context, job *btypes.ChangesetJob) (afterDone func(*store.Store), err error) {
	// Use the acting user for the operation to enforce repository permissions.
	ctx = actor.WithActor(ctx, actor.FromUser(job.UserID))

	// Load changeset.
	b.ch, err = b.tx.GetChangeset(ctx, store.GetChangesetOpts{ID: job.ChangesetID})
	if err != nil {
		return nil, errors.Wrap(err, "loading changeset")
	}

	// Changesets that are currently processing should be retried at a later stage.
	if b.ch.ReconcilerState == btypes.ReconcilerStateProcessing {
		return nil, changesetIsProcessingErr
	}

	// Load repo.
	b.repo, err = b.tx.Repos().Get(ctx, b.ch.RepoID)
	if err != nil {
		return nil, errors.Wrap(err, "loading repo")
	}

	// Construct changeset source.
	b.css, err = b.sourcer.ForUser(ctx, b.tx, job.UserID, b.repo)
	if err != nil {
		return nil, errors.Wrap(err, "loading ChangesetSource")
	}

	b.logger.Info("processing changeset job", log.String("type", string(job.JobType)))

	switch job.JobType {

	case btypes.ChangesetJobTypeComment:
		return nil, b.comment(ctx, job)
	case btypes.ChangesetJobTypeDetach:
		return nil, b.detach(ctx, job)
	case btypes.ChangesetJobTypeReenqueue:
		return nil, b.reenqueueChangeset(ctx)
	case btypes.ChangesetJobTypeMerge:
		return b.mergeChangeset(ctx, job)
	case btypes.ChangesetJobTypeClose:
		return b.closeChangeset(ctx)
	case btypes.ChangesetJobTypePublish:
		return nil, b.publishChangeset(ctx, job)

	default:
		return nil, &unknownJobTypeErr{jobType: string(job.JobType)}
	}
}

func (b *bulkProcessor) comment(ctx context.Context, job *btypes.ChangesetJob) error {
	typedPayload, ok := job.Payload.(*btypes.ChangesetJobCommentPayload)
	if !ok {
		return errors.Errorf("invalid payload type for changeset_job, want=%T have=%T", &btypes.ChangesetJobCommentPayload{}, job.Payload)
	}

	remoteRepo, err := sources.GetRemoteRepo(ctx, b.css, b.repo, b.ch, nil)
	if err != nil {
		return errors.Wrap(err, "loading remote repo")
	}

	cs := &sources.Changeset{
		Changeset:  b.ch,
		TargetRepo: b.repo,
		RemoteRepo: remoteRepo,
	}
	return b.css.CreateComment(ctx, cs, typedPayload.Message)
}

func (b *bulkProcessor) detach(ctx context.Context, job *btypes.ChangesetJob) error {
	// Try to detach the changeset from the batch change of the job.
	var detached bool
	for i, assoc := range b.ch.BatchChanges {
		if assoc.BatchChangeID == job.BatchChangeID {
			if !b.ch.BatchChanges[i].Detach {
				b.ch.BatchChanges[i].Detach = true
				detached = true
			}
		}
	}

	if !detached {
		return nil
	}

	// If we successfully marked the record as to-be-detached, we save the
	// updated associations and trigger a reconciler run with two `UPDATE`
	// queries:
	// 1. Update only the changeset's BatchChanges in the database, trying not
	//    to overwrite any other data.
	// 2. Updates only the worker/reconciler-related columns to enqueue the
	//    changeset.
	if err := b.tx.UpdateChangesetBatchChanges(ctx, b.ch); err != nil {
		return err
	}

	return b.tx.EnqueueChangeset(ctx, b.ch, global.DefaultReconcilerEnqueueState(), "")
}

func (b *bulkProcessor) reenqueueChangeset(ctx context.Context) error {
	svc := service.New(b.tx)
	_, _, err := svc.ReenqueueChangeset(ctx, b.ch.ID)
	return err
}

func (b *bulkProcessor) mergeChangeset(ctx context.Context, job *btypes.ChangesetJob) (afterDone func(*store.Store), err error) {
	typedPayload, ok := job.Payload.(*btypes.ChangesetJobMergePayload)
	if !ok {
		return nil, errors.Errorf("invalid payload type for changeset_job, want=%T have=%T", &btypes.ChangesetJobMergePayload{}, job.Payload)
	}

	remoteRepo, err := sources.GetRemoteRepo(ctx, b.css, b.repo, b.ch, nil)
	if err != nil {
		return nil, errors.Wrap(err, "loading remote repo")
	}

	cs := &sources.Changeset{
		Changeset:  b.ch,
		TargetRepo: b.repo,
		RemoteRepo: remoteRepo,
	}
	if err := b.css.MergeChangeset(ctx, cs, typedPayload.Squash); err != nil {
		return nil, err
	}

	events, err := cs.Changeset.Events()
	if err != nil {
		b.logger.Error("Events", log.Error(err))
		return nil, errcode.MakeNonRetryable(err)
	}
	state.SetDerivedState(ctx, b.tx.Repos(), gitserver.NewClient("batches.bulkprocessor.mergechangeset"), cs.Changeset, events)

	if err := b.tx.UpsertChangesetEvents(ctx, events...); err != nil {
		b.logger.Error("UpsertChangesetEvents", log.Error(err))
		return nil, errcode.MakeNonRetryable(err)
	}

	if err := b.tx.UpdateChangesetCodeHostState(ctx, cs.Changeset); err != nil {
		b.logger.Error("UpdateChangeset", log.Error(err))
		return nil, errcode.MakeNonRetryable(err)
	}

	afterDone = func(s *store.Store) { b.enqueueWebhook(ctx, s, webhooks.ChangesetClose) }
	return afterDone, nil
}

func (b *bulkProcessor) closeChangeset(ctx context.Context) (afterDone func(*store.Store), err error) {
	remoteRepo, err := sources.GetRemoteRepo(ctx, b.css, b.repo, b.ch, nil)
	if err != nil {
		return nil, errors.Wrap(err, "loading remote repo")
	}

	cs := &sources.Changeset{
		Changeset:  b.ch,
		TargetRepo: b.repo,
		RemoteRepo: remoteRepo,
	}
	if err := b.css.CloseChangeset(ctx, cs); err != nil {
		return nil, err
	}

	events, err := cs.Changeset.Events()
	if err != nil {
		b.logger.Error("Events", log.Error(err))
		return nil, errcode.MakeNonRetryable(err)
	}
	state.SetDerivedState(ctx, b.tx.Repos(), gitserver.NewClient("batches.bulkprocessor.closechangeset"), cs.Changeset, events)

	if err := b.tx.UpsertChangesetEvents(ctx, events...); err != nil {
		b.logger.Error("UpsertChangesetEvents", log.Error(err))
		return nil, errcode.MakeNonRetryable(err)
	}

	if err := b.tx.UpdateChangesetCodeHostState(ctx, cs.Changeset); err != nil {
		b.logger.Error("UpdateChangeset", log.Error(err))
		return nil, errcode.MakeNonRetryable(err)
	}

	afterDone = func(s *store.Store) { b.enqueueWebhook(ctx, s, webhooks.ChangesetClose) }
	return afterDone, nil
}

func (b *bulkProcessor) publishChangeset(ctx context.Context, job *btypes.ChangesetJob) (err error) {
	typedPayload, ok := job.Payload.(*btypes.ChangesetJobPublishPayload)
	if !ok {
		return errors.Errorf("invalid payload type for changeset_job, want=%T have=%T", &btypes.ChangesetJobPublishPayload{}, job.Payload)
	}

	// We can't publish an imported changeset.
	if b.ch.CurrentSpecID == 0 {
		return errcode.MakeNonRetryable(errors.New("cannot publish an imported changeset"))
	}

	// We can't publish a changeset with its publication state set in the spec.
	spec, err := b.tx.GetChangesetSpecByID(ctx, b.ch.CurrentSpecID)
	if err != nil {
		b.logger.Error("GetChangesetBySpecID", log.Error(err))
		return errcode.MakeNonRetryable(errors.Wrapf(err, "getting changeset spec for changeset %d", b.ch.ID))
	} else if spec == nil {
		return errcode.MakeNonRetryable(errors.Newf("no changeset spec for changeset %d", b.ch.ID))
	}

	if !spec.Published.Nil() {
		return errcode.MakeNonRetryable(errors.New("cannot publish a changeset that has a published value set in its changesetTemplate"))
	}

	// Set the desired UI publication state.
	if typedPayload.Draft {
		b.ch.UiPublicationState = &btypes.ChangesetUiPublicationStateDraft
	} else {
		b.ch.UiPublicationState = &btypes.ChangesetUiPublicationStatePublished
	}

	// We do two UPDATE queries here:
	// 1. Update only the changeset's UiPublicationState in the database, trying not
	//    to overwrite any other data.
	// 2. Updates only the worker/reconciler-related columns to enqueue the
	//    changeset.
	if err := b.tx.UpdateChangesetUiPublicationState(ctx, b.ch); err != nil {
		b.logger.Error("UpdateChangesetUiPublicationState", log.Error(err))
		return errcode.MakeNonRetryable(err)
	}

	if err := b.tx.EnqueueChangeset(ctx, b.ch, global.DefaultReconcilerEnqueueState(), ""); err != nil {
		b.logger.Error("EnqueueChangeset", log.Error(err))
		return errcode.MakeNonRetryable(err)
	}

	return nil
}

func (b *bulkProcessor) enqueueWebhook(ctx context.Context, store *store.Store, eventType string) {
	webhooks.EnqueueChangeset(ctx, b.logger, httpcli.InternalDoer, store, eventType, bgql.MarshalChangesetID(b.ch.ID))
}
