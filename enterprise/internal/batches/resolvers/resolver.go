package resolvers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/graph-gophers/graphql-go"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/search"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/service"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/usagestats"
)

// Resolver is the GraphQL resolver of all things related to batch changes.
type Resolver struct {
	store *store.Store
}

// New returns a new Resolver whose store uses the given database
func New(store *store.Store) graphqlbackend.BatchChangesResolver {
	return &Resolver{store: store}
}

func batchChangesEnabled(ctx context.Context) error {
	// On Sourcegraph.com nobody can read/create batch changes entities
	if envvar.SourcegraphDotComMode() {
		return ErrBatchChangesDotcom{}
	}

	if enabled := conf.BatchChangesEnabled(); enabled {
		if conf.BatchChangesRestrictedToAdmins() && backend.CheckCurrentUserIsSiteAdmin(ctx) != nil {
			return ErrBatchChangesDisabledForUser{}
		}
		return nil
	}

	return ErrBatchChangesDisabled{}
}

// batchChangesCreateAccess returns true if the current user can create
// batchChanges/changesetSpecs/batchSpecs.
func batchChangesCreateAccess(ctx context.Context) error {
	// On Sourcegraph.com nobody can create batchChanges/patchsets/changesets
	if envvar.SourcegraphDotComMode() {
		return ErrBatchChangesDotcom{}
	}

	act := actor.FromContext(ctx)
	if !act.IsAuthenticated() {
		return backend.ErrNotAuthenticated
	}
	return nil
}

// checkLicense returns a user-facing error if the batchChanges feature is not purchased
// with the current license or any error occurred while validating the license.
func checkLicense() error {
	batchChangesErr := licensing.Check(licensing.FeatureBatchChanges)
	if batchChangesErr == nil {
		return nil
	}

	if licensing.IsFeatureNotActivated(batchChangesErr) {
		// Let's fallback and check whether (deprecated) campaigns are enabled:
		campaignsErr := licensing.Check(licensing.FeatureCampaigns)
		if campaignsErr == nil {
			return nil
		}
		return batchChangesErr
	}

	return errors.New("Unable to check license feature, please refer to logs for actual error message.")
}

// maxUnlicensedChangesets is the maximum number of changesets that can be
// attached to a batch change when Sourcegraph is unlicensed or the Batch
// Changes feature is disabled.
const maxUnlicensedChangesets = 5

func (r *Resolver) ChangesetByID(ctx context.Context, id graphql.ID) (graphqlbackend.ChangesetResolver, error) {
	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	changesetID, err := unmarshalChangesetID(id)
	if err != nil {
		return nil, err
	}

	if changesetID == 0 {
		return nil, nil
	}

	changeset, err := r.store.GetChangeset(ctx, store.GetChangesetOpts{ID: changesetID})
	if err != nil {
		if err == store.ErrNoResults {
			return nil, nil
		}
		return nil, err
	}

	// 🚨 SECURITY: database.Repos.Get uses the authzFilter under the hood and
	// filters out repositories that the user doesn't have access to.
	repo, err := r.store.Repos().Get(ctx, changeset.RepoID)
	if err != nil && !errcode.IsNotFound(err) {
		return nil, err
	}

	return NewChangesetResolver(r.store, changeset, repo), nil
}

func (r *Resolver) BatchChangeByID(ctx context.Context, id graphql.ID) (graphqlbackend.BatchChangeResolver, error) {
	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	batchChangeID, err := unmarshalBatchChangeID(id)
	if err != nil {
		return nil, err
	}

	if batchChangeID == 0 {
		return nil, nil
	}

	batchChange, err := r.store.GetBatchChange(ctx, store.CountBatchChangeOpts{ID: batchChangeID})
	if err != nil {
		if err == store.ErrNoResults {
			return nil, nil
		}
		return nil, err
	}

	return &batchChangeResolver{store: r.store, batchChange: batchChange}, nil
}

func (r *Resolver) BatchChange(ctx context.Context, args *graphqlbackend.BatchChangeArgs) (graphqlbackend.BatchChangeResolver, error) {
	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	opts := store.CountBatchChangeOpts{Name: args.Name}

	err := graphqlbackend.UnmarshalNamespaceID(graphql.ID(args.Namespace), &opts.NamespaceUserID, &opts.NamespaceOrgID)
	if err != nil {
		return nil, err
	}

	batchChange, err := r.store.GetBatchChange(ctx, opts)
	if err != nil {
		if err == store.ErrNoResults {
			return nil, nil
		}
		return nil, err
	}

	return &batchChangeResolver{store: r.store, batchChange: batchChange}, nil
}

func (r *Resolver) BatchSpecByID(ctx context.Context, id graphql.ID) (graphqlbackend.BatchSpecResolver, error) {
	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	batchSpecRandID, err := unmarshalBatchSpecID(id)
	if err != nil {
		return nil, err
	}

	if batchSpecRandID == "" {
		return nil, nil
	}

	opts := store.GetBatchSpecOpts{RandID: batchSpecRandID}
	batchSpec, err := r.store.GetBatchSpec(ctx, opts)
	if err != nil {
		if err == store.ErrNoResults {
			return nil, nil
		}
		return nil, err
	}

	return &batchSpecResolver{store: r.store, batchSpec: batchSpec}, nil
}

func (r *Resolver) ChangesetSpecByID(ctx context.Context, id graphql.ID) (graphqlbackend.ChangesetSpecResolver, error) {
	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	changesetSpecRandID, err := unmarshalChangesetSpecID(id)
	if err != nil {
		return nil, err
	}

	if changesetSpecRandID == "" {
		return nil, nil
	}

	opts := store.GetChangesetSpecOpts{RandID: changesetSpecRandID}
	changesetSpec, err := r.store.GetChangesetSpec(ctx, opts)
	if err != nil {
		if err == store.ErrNoResults {
			return nil, nil
		}
		return nil, err
	}

	return NewChangesetSpecResolver(ctx, r.store, changesetSpec)
}

func (r *Resolver) BatchChangesCredentialByID(ctx context.Context, id graphql.ID) (graphqlbackend.BatchChangesCredentialResolver, error) {
	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	dbID, err := unmarshalBatchChangesCredentialID(id)
	if err != nil {
		return nil, err
	}

	if dbID == 0 {
		return nil, nil
	}

	cred, err := r.store.UserCredentials().GetByID(ctx, dbID)
	if err != nil {
		if errcode.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	if err := backend.CheckSiteAdminOrSameUser(ctx, cred.UserID); err != nil {
		return nil, err
	}

	return &batchChangesCredentialResolver{credential: cred}, nil
}

func (r *Resolver) CreateBatchChange(ctx context.Context, args *graphqlbackend.CreateBatchChangeArgs) (graphqlbackend.BatchChangeResolver, error) {
	var err error
	tr, _ := trace.New(ctx, "Resolver.CreateBatchChange", fmt.Sprintf("BatchSpec %s", args.BatchSpec))
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	opts := service.ApplyBatchChangeOpts{
		// This is what differentiates CreateBatchChange from ApplyBatchChange
		FailIfBatchChangeExists: true,
	}

	opts.BatchSpecRandID, err = unmarshalBatchSpecID(args.BatchSpec)
	if err != nil {
		return nil, err
	}

	if opts.BatchSpecRandID == "" {
		return nil, ErrIDIsZero{}
	}

	svc := service.New(r.store)
	batchChange, err := svc.ApplyBatchChange(ctx, opts)
	if err != nil {
		if err == service.ErrEnsureBatchChangeFailed {
			return nil, ErrEnsureBatchChangeFailed{}
		} else if err == service.ErrApplyClosedBatchChange {
			return nil, ErrApplyClosedBatchChange{}
		} else if err == service.ErrMatchingBatchChangeExists {
			return nil, ErrMatchingBatchChangeExists{}
		}
		return nil, err
	}

	return &batchChangeResolver{store: r.store, batchChange: batchChange}, nil
}

func (r *Resolver) ApplyBatchChange(ctx context.Context, args *graphqlbackend.ApplyBatchChangeArgs) (graphqlbackend.BatchChangeResolver, error) {
	var err error
	tr, ctx := trace.New(ctx, "Resolver.ApplyBatchChange", fmt.Sprintf("BatchSpec %s", args.BatchSpec))
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	opts := service.ApplyBatchChangeOpts{}

	opts.BatchSpecRandID, err = unmarshalBatchSpecID(args.BatchSpec)
	if err != nil {
		return nil, err
	}

	if opts.BatchSpecRandID == "" {
		return nil, ErrIDIsZero{}
	}

	if args.EnsureBatchChange != nil {
		opts.EnsureBatchChangeID, err = unmarshalBatchChangeID(*args.EnsureBatchChange)
		if err != nil {
			return nil, err
		}
	}

	svc := service.New(r.store)
	// 🚨 SECURITY: ApplyBatchChange checks whether the user has permission to
	// apply the batch spec
	batchChange, err := svc.ApplyBatchChange(ctx, opts)
	if err != nil {
		if err == service.ErrEnsureBatchChangeFailed {
			return nil, ErrEnsureBatchChangeFailed{}
		} else if err == service.ErrApplyClosedBatchChange {
			return nil, ErrApplyClosedBatchChange{}
		} else if err == service.ErrMatchingBatchChangeExists {
			return nil, ErrMatchingBatchChangeExists{}
		}
		return nil, err
	}

	return &batchChangeResolver{store: r.store, batchChange: batchChange}, nil
}

func (r *Resolver) CreateBatchSpec(ctx context.Context, args *graphqlbackend.CreateBatchSpecArgs) (graphqlbackend.BatchSpecResolver, error) {
	var err error
	tr, ctx := trace.New(ctx, "CreateBatchSpec", fmt.Sprintf("Resolver.CreateBatchspace %s, Spec %q", args.Namespace, args.BatchSpec))
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	if err := batchChangesCreateAccess(ctx); err != nil {
		return nil, err
	}

	if err := checkLicense(); err != nil {
		if licensing.IsFeatureNotActivated(err) {
			if len(args.ChangesetSpecs) > maxUnlicensedChangesets {
				return nil, ErrBatchChangesUnlicensed{err}
			}
		} else {
			return nil, err
		}
	}

	opts := service.CreateBatchSpecOpts{RawSpec: args.BatchSpec}

	err = graphqlbackend.UnmarshalNamespaceID(args.Namespace, &opts.NamespaceUserID, &opts.NamespaceOrgID)
	if err != nil {
		return nil, err
	}

	for _, graphqlID := range args.ChangesetSpecs {
		randID, err := unmarshalChangesetSpecID(graphqlID)
		if err != nil {
			return nil, err
		}
		opts.ChangesetSpecRandIDs = append(opts.ChangesetSpecRandIDs, randID)
	}

	svc := service.New(r.store)
	batchSpec, err := svc.CreateBatchSpec(ctx, opts)
	if err != nil {
		return nil, err
	}

	if err := logBatchSpecCreated(ctx, r.store.DB(), &opts); err != nil {
		return nil, err
	}

	specResolver := &batchSpecResolver{
		store:     r.store,
		batchSpec: batchSpec,
	}

	return specResolver, nil
}

func logBatchSpecCreated(ctx context.Context, db dbutil.DB, opts *service.CreateBatchSpecOpts) error {
	// Log an analytics event when a BatchSpec has been created.
	// See internal/usagestats/batches.go.
	actor := actor.FromContext(ctx)

	type eventArg struct {
		ChangesetSpecsCount int `json:"changeset_specs_count"`
	}
	arg := eventArg{ChangesetSpecsCount: len(opts.ChangesetSpecRandIDs)}

	jsonArg, err := json.Marshal(arg)
	if err != nil {
		return err
	}

	return usagestats.LogBackendEvent(db, actor.UID, "CampaignSpecCreated", json.RawMessage(jsonArg))
}

func (r *Resolver) CreateChangesetSpec(ctx context.Context, args *graphqlbackend.CreateChangesetSpecArgs) (graphqlbackend.ChangesetSpecResolver, error) {
	var err error
	tr, ctx := trace.New(ctx, "Resolver.CreateChangesetSpec", "")
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	if err := batchChangesCreateAccess(ctx); err != nil {
		return nil, err
	}

	act := actor.FromContext(ctx)
	// Actor MUST be logged in at this stage, because batchChangesCreateAccess checks that already.
	// To be extra safe, we'll just do the cheap check again here so if anyone ever modifies
	// batchChangesCreateAccess, we still enforce it here.
	if !act.IsAuthenticated() {
		return nil, backend.ErrNotAuthenticated
	}

	svc := service.New(r.store)
	spec, err := svc.CreateChangesetSpec(ctx, args.ChangesetSpec, act.UID)
	if err != nil {
		return nil, err
	}

	return NewChangesetSpecResolver(ctx, r.store, spec)
}

func (r *Resolver) MoveBatchChange(ctx context.Context, args *graphqlbackend.MoveBatchChangeArgs) (graphqlbackend.BatchChangeResolver, error) {
	var err error
	tr, ctx := trace.New(ctx, "Resolver.MoveBatchChange", fmt.Sprintf("BatchChange %s", args.BatchChange))
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	batchChangeID, err := unmarshalBatchChangeID(args.BatchChange)
	if err != nil {
		return nil, err
	}

	if batchChangeID == 0 {
		return nil, ErrIDIsZero{}
	}

	opts := service.MoveBatchChangeOpts{
		BatchChangeID: batchChangeID,
	}

	if args.NewName != nil {
		opts.NewName = *args.NewName
	}

	if args.NewNamespace != nil {
		err := graphqlbackend.UnmarshalNamespaceID(*args.NewNamespace, &opts.NewNamespaceUserID, &opts.NewNamespaceOrgID)
		if err != nil {
			return nil, err
		}
	}

	svc := service.New(r.store)
	// 🚨 SECURITY: MoveBatchChange checks whether the current user is authorized.
	batchChange, err := svc.MoveBatchChange(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &batchChangeResolver{store: r.store, batchChange: batchChange}, nil
}

func (r *Resolver) DeleteBatchChange(ctx context.Context, args *graphqlbackend.DeleteBatchChangeArgs) (_ *graphqlbackend.EmptyResponse, err error) {
	tr, ctx := trace.New(ctx, "Resolver.DeleteBatchChange", fmt.Sprintf("BatchChange: %q", args.BatchChange))
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()
	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	batchChangeID, err := unmarshalBatchChangeID(args.BatchChange)
	if err != nil {
		return nil, err
	}

	if batchChangeID == 0 {
		return nil, ErrIDIsZero{}
	}

	svc := service.New(r.store)
	// 🚨 SECURITY: DeleteBatchChange checks whether current user is authorized.
	err = svc.DeleteBatchChange(ctx, batchChangeID)
	return &graphqlbackend.EmptyResponse{}, err
}

func (r *Resolver) BatchChanges(ctx context.Context, args *graphqlbackend.ListBatchChangesArgs) (graphqlbackend.BatchChangesConnectionResolver, error) {
	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	opts := store.ListBatchChangesOpts{}

	state, err := parseBatchChangeState(args.State)
	if err != nil {
		return nil, err
	}
	opts.State = state
	if err := validateFirstParamDefaults(args.First); err != nil {
		return nil, err
	}
	opts.Limit = int(args.First)
	if args.After != nil {
		cursor, err := strconv.ParseInt(*args.After, 10, 32)
		if err != nil {
			return nil, err
		}
		opts.Cursor = cursor
	}

	authErr := backend.CheckCurrentUserIsSiteAdmin(ctx)
	if authErr != nil && authErr != backend.ErrMustBeSiteAdmin {
		return nil, authErr
	}
	isSiteAdmin := authErr != backend.ErrMustBeSiteAdmin
	if !isSiteAdmin {
		if args.ViewerCanAdminister != nil && *args.ViewerCanAdminister {
			actor := actor.FromContext(ctx)
			opts.InitialApplierID = actor.UID
		}
	}

	if args.Namespace != nil {
		err := graphqlbackend.UnmarshalNamespaceID(*args.Namespace, &opts.NamespaceUserID, &opts.NamespaceOrgID)
		if err != nil {
			return nil, err
		}
	}

	return &batchChangesConnectionResolver{
		store: r.store,
		opts:  opts,
	}, nil
}

func (r *Resolver) BatchChangesCodeHosts(ctx context.Context, args *graphqlbackend.ListBatchChangesCodeHostsArgs) (graphqlbackend.BatchChangesCodeHostConnectionResolver, error) {
	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Only viewable for self or by site admins.
	if err := backend.CheckSiteAdminOrSameUser(ctx, args.UserID); err != nil {
		return nil, err
	}

	if err := validateFirstParamDefaults(args.First); err != nil {
		return nil, err
	}
	limitOffset := database.LimitOffset{
		Limit: int(args.First),
	}
	if args.After != nil {
		cursor, err := strconv.ParseInt(*args.After, 10, 32)
		if err != nil {
			return nil, err
		}
		limitOffset.Offset = int(cursor)
	}

	return &batchChangesCodeHostConnectionResolver{userID: args.UserID, limitOffset: limitOffset, store: r.store}, nil
}

// listChangesetOptsFromArgs turns the graphqlbackend.ListChangesetsArgs into
// ListChangesetsOpts.
// If the args do not include a filter that would reveal sensitive information
// about a changeset the user doesn't have access to, the second return value
// is false.
func listChangesetOptsFromArgs(args *graphqlbackend.ListChangesetsArgs, batchChangeID int64) (opts store.ListChangesetsOpts, optsSafe bool, err error) {
	if args == nil {
		return opts, true, nil
	}

	safe := true

	// TODO: This _could_ become problematic if a user has a batch change with > 10000 changesets, once
	// we use cursor based pagination in the frontend for ChangesetConnections this problem will disappear.
	// Currently we cannot enable it, though, because we want to re-fetch the whole list periodically to
	// check for a change in the changeset states.
	if err := validateFirstParamDefaults(args.First); err != nil {
		return opts, false, err
	}
	opts.Limit = int(args.First)

	if args.After != nil {
		cursor, err := strconv.ParseInt(*args.After, 10, 32)
		if err != nil {
			return opts, false, errors.Wrap(err, "parsing after cursor")
		}
		opts.Cursor = cursor
	}

	if args.State != nil {
		state := *args.State
		if !state.Valid() {
			return opts, false, errors.New("changeset state not valid")
		}

		switch state {
		case batches.ChangesetStateOpen:
			externalState := batches.ChangesetExternalStateOpen
			publicationState := batches.ChangesetPublicationStatePublished
			opts.ExternalState = &externalState
			opts.ReconcilerStates = []batches.ReconcilerState{batches.ReconcilerStateCompleted}
			opts.PublicationState = &publicationState
		case batches.ChangesetStateDraft:
			externalState := batches.ChangesetExternalStateDraft
			publicationState := batches.ChangesetPublicationStatePublished
			opts.ExternalState = &externalState
			opts.ReconcilerStates = []batches.ReconcilerState{batches.ReconcilerStateCompleted}
			opts.PublicationState = &publicationState
		case batches.ChangesetStateClosed:
			externalState := batches.ChangesetExternalStateClosed
			publicationState := batches.ChangesetPublicationStatePublished
			opts.ExternalState = &externalState
			opts.ReconcilerStates = []batches.ReconcilerState{batches.ReconcilerStateCompleted}
			opts.PublicationState = &publicationState
		case batches.ChangesetStateMerged:
			externalState := batches.ChangesetExternalStateMerged
			publicationState := batches.ChangesetPublicationStatePublished
			opts.ExternalState = &externalState
			opts.ReconcilerStates = []batches.ReconcilerState{batches.ReconcilerStateCompleted}
			opts.PublicationState = &publicationState
		case batches.ChangesetStateDeleted:
			externalState := batches.ChangesetExternalStateDeleted
			publicationState := batches.ChangesetPublicationStatePublished
			opts.ExternalState = &externalState
			opts.ReconcilerStates = []batches.ReconcilerState{batches.ReconcilerStateCompleted}
			opts.PublicationState = &publicationState
		case batches.ChangesetStateUnpublished:
			publicationState := batches.ChangesetPublicationStateUnpublished
			opts.ReconcilerStates = []batches.ReconcilerState{batches.ReconcilerStateCompleted}
			opts.PublicationState = &publicationState
		case batches.ChangesetStateProcessing:
			opts.ReconcilerStates = []batches.ReconcilerState{batches.ReconcilerStateQueued, batches.ReconcilerStateProcessing}
		case batches.ChangesetStateRetrying:
			opts.ReconcilerStates = []batches.ReconcilerState{batches.ReconcilerStateErrored}
		case batches.ChangesetStateFailed:
			opts.ReconcilerStates = []batches.ReconcilerState{batches.ReconcilerStateFailed}
		default:
			return opts, false, errors.Errorf("changeset state %q not supported in filtering", state)
		}
	}

	if args.ReviewState != nil {
		state := *args.ReviewState
		if !state.Valid() {
			return opts, false, errors.New("changeset review state not valid")
		}
		opts.ExternalReviewState = &state
		// If the user filters by ReviewState we cannot include hidden
		// changesets, since that would leak information.
		safe = false
	}
	if args.CheckState != nil {
		state := *args.CheckState
		if !state.Valid() {
			return opts, false, errors.New("changeset check state not valid")
		}
		opts.ExternalCheckState = &state
		// If the user filters by CheckState we cannot include hidden
		// changesets, since that would leak information.
		safe = false
	}
	if args.OnlyPublishedByThisCampaign != nil || args.OnlyPublishedByThisBatchChange != nil {
		published := batches.ChangesetPublicationStatePublished

		opts.OwnedByBatchChangeID = batchChangeID
		opts.PublicationState = &published
	}
	if args.Search != nil {
		var err error
		opts.TextSearch, err = search.ParseTextSearch(*args.Search)
		if err != nil {
			return opts, false, errors.Wrap(err, "parsing search")
		}
		// Since we search for the repository name in text searches, the
		// presence or absence of results may leak information about hidden
		// repositories.
		safe = false
	}

	return opts, safe, nil
}

func (r *Resolver) CloseBatchChange(ctx context.Context, args *graphqlbackend.CloseBatchChangeArgs) (_ graphqlbackend.BatchChangeResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.CloseBatchChange", fmt.Sprintf("BatchChange: %q", args.BatchChange))
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	batchChangeID, err := unmarshalBatchChangeID(args.BatchChange)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshaling batch change id")
	}

	if batchChangeID == 0 {
		return nil, ErrIDIsZero{}
	}

	svc := service.New(r.store)
	// 🚨 SECURITY: CloseBatchChange checks whether current user is authorized.
	batchChange, err := svc.CloseBatchChange(ctx, batchChangeID, args.CloseChangesets)
	if err != nil {
		return nil, errors.Wrap(err, "closing batch change")
	}

	return &batchChangeResolver{store: r.store, batchChange: batchChange}, nil
}

func (r *Resolver) SyncChangeset(ctx context.Context, args *graphqlbackend.SyncChangesetArgs) (_ *graphqlbackend.EmptyResponse, err error) {
	tr, ctx := trace.New(ctx, "Resolver.SyncChangeset", fmt.Sprintf("Changeset: %q", args.Changeset))
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()
	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	changesetID, err := unmarshalChangesetID(args.Changeset)
	if err != nil {
		return nil, err
	}

	if changesetID == 0 {
		return nil, ErrIDIsZero{}
	}

	// 🚨 SECURITY: EnqueueChangesetSync checks whether current user is authorized.
	svc := service.New(r.store)
	if err = svc.EnqueueChangesetSync(ctx, changesetID); err != nil {
		return nil, err
	}

	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *Resolver) ReenqueueChangeset(ctx context.Context, args *graphqlbackend.ReenqueueChangesetArgs) (_ graphqlbackend.ChangesetResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.ReenqueueChangeset", fmt.Sprintf("Changeset: %q", args.Changeset))
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()
	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	changesetID, err := unmarshalChangesetID(args.Changeset)
	if err != nil {
		return nil, err
	}

	if changesetID == 0 {
		return nil, ErrIDIsZero{}
	}

	// 🚨 SECURITY: ReenqueueChangeset checks whether the current user is authorized and can administer the changeset.
	svc := service.New(r.store)
	changeset, repo, err := svc.ReenqueueChangeset(ctx, changesetID)
	if err != nil {
		return nil, err
	}

	return NewChangesetResolver(r.store, changeset, repo), nil
}

func (r *Resolver) CreateBatchChangesCredential(ctx context.Context, args *graphqlbackend.CreateBatchChangesCredentialArgs) (_ graphqlbackend.BatchChangesCredentialResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.CreateBatchChangesCredential", fmt.Sprintf("%q (%q)", args.ExternalServiceKind, args.ExternalServiceURL))
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()
	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	userID, err := graphqlbackend.UnmarshalUserID(args.User)
	if err != nil {
		return nil, err
	}

	if userID == 0 {
		return nil, ErrIDIsZero{}
	}

	// 🚨 SECURITY: Check that the requesting user can create the credential.
	if err := backend.CheckSiteAdminOrSameUser(ctx, userID); err != nil {
		return nil, err
	}

	// Need to validate externalServiceKind, otherwise this'll panic.
	kind, valid := extsvc.ParseServiceKind(args.ExternalServiceKind)
	if !valid {
		return nil, errors.New("invalid external service kind")
	}

	// TODO: Do we want to validate the URL, or even if such an external service exists? Or better, would the DB have a constraint?

	if args.Credential == "" {
		return nil, errors.New("empty credential not allowed")
	}

	scope := database.UserCredentialScope{
		Domain:              database.UserCredentialDomainBatches,
		ExternalServiceID:   args.ExternalServiceURL,
		ExternalServiceType: extsvc.KindToType(kind),
		UserID:              userID,
	}

	// Throw error documented in schema.graphql.
	existing, err := r.store.UserCredentials().GetByScope(ctx, scope)
	if err != nil && !errcode.IsNotFound(err) {
		return nil, err
	}
	if existing != nil {
		return nil, ErrDuplicateCredential{}
	}

	keypair, err := encryption.GenerateRSAKey()
	if err != nil {
		return nil, err
	}

	var a auth.Authenticator
	if kind == extsvc.KindBitbucketServer {
		svc := service.New(r.store)
		username, err := svc.FetchUsernameForBitbucketServerToken(ctx, args.ExternalServiceURL, extsvc.KindToType(kind), args.Credential)
		if err != nil {
			if bitbucketserver.IsUnauthorized(err) {
				return nil, &ErrVerifyCredentialFailed{SourceErr: err}
			}
			return nil, err
		}
		a = &auth.BasicAuthWithSSH{
			BasicAuth:  auth.BasicAuth{Username: username, Password: args.Credential},
			PrivateKey: keypair.PrivateKey,
			PublicKey:  keypair.PublicKey,
			Passphrase: keypair.Passphrase,
		}
	} else {
		a = &auth.OAuthBearerTokenWithSSH{
			OAuthBearerToken: auth.OAuthBearerToken{Token: args.Credential},
			PrivateKey:       keypair.PrivateKey,
			PublicKey:        keypair.PublicKey,
			Passphrase:       keypair.Passphrase,
		}
	}

	cred, err := r.store.UserCredentials().Create(ctx, scope, a)
	if err != nil {
		return nil, err
	}

	return &batchChangesCredentialResolver{credential: cred}, nil
}

func (r *Resolver) DeleteBatchChangesCredential(ctx context.Context, args *graphqlbackend.DeleteBatchChangesCredentialArgs) (_ *graphqlbackend.EmptyResponse, err error) {
	tr, ctx := trace.New(ctx, "Resolver.DeleteBatchChangesCredential", fmt.Sprintf("Credential: %q", args.BatchChangesCredential))
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()
	if err := batchChangesEnabled(ctx); err != nil {
		return nil, err
	}

	dbID, err := unmarshalBatchChangesCredentialID(args.BatchChangesCredential)
	if err != nil {
		return nil, err
	}

	if dbID == 0 {
		return nil, ErrIDIsZero{}
	}

	// Get existing credential.
	cred, err := r.store.UserCredentials().GetByID(ctx, dbID)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Check that the requesting user may delete the credential.
	if err := backend.CheckSiteAdminOrSameUser(ctx, cred.UserID); err != nil {
		return nil, err
	}

	// This also fails if the credential was not found.
	if err := r.store.UserCredentials().Delete(ctx, dbID); err != nil {
		return nil, err
	}

	return &graphqlbackend.EmptyResponse{}, nil
}

func parseBatchChangeState(s *string) (batches.BatchChangeState, error) {
	if s == nil {
		return batches.BatchChangeStateAny, nil
	}
	switch *s {
	case "OPEN":
		return batches.BatchChangeStateOpen, nil
	case "CLOSED":
		return batches.BatchChangeStateClosed, nil
	default:
		return batches.BatchChangeStateAny, fmt.Errorf("unknown state %q", *s)
	}
}

func checkSiteAdminOrSameUser(ctx context.Context, userID int32) (bool, error) {
	// 🚨 SECURITY: Only site admins or the authors of a batch change have batch change
	// admin rights.
	if err := backend.CheckSiteAdminOrSameUser(ctx, userID); err != nil {
		if _, ok := err.(*backend.InsufficientAuthorizationError); ok {
			return false, nil
		}

		return false, err
	}
	return true, nil
}

type ErrInvalidFirstParameter struct {
	Min, Max, First int
}

func (e ErrInvalidFirstParameter) Error() string {
	return fmt.Sprintf("first param %d is out of range (min=%d, max=%d)", e.First, e.Min, e.Max)
}

func validateFirstParam(first int32, max int) error {
	if first < 0 || first > int32(max) {
		return ErrInvalidFirstParameter{Min: 0, Max: max, First: int(first)}
	}
	return nil
}

const defaultMaxFirstParam = 10000

func validateFirstParamDefaults(first int32) error {
	return validateFirstParam(first, defaultMaxFirstParam)
}
