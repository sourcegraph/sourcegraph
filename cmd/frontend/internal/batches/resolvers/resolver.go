package resolvers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"

	"github.com/graph-gophers/graphql-go"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/rbac"
	"github.com/sourcegraph/sourcegraph/internal/types"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/batches/search"
	"github.com/sourcegraph/sourcegraph/internal/batches/service"
	"github.com/sourcegraph/sourcegraph/internal/batches/sources"
	"github.com/sourcegraph/sourcegraph/internal/batches/store"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/deviceid"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	extsvcauth "github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	ghstore "github.com/sourcegraph/sourcegraph/internal/github_apps/store"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/usagestats"
	batcheslib "github.com/sourcegraph/sourcegraph/lib/batches"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// Resolver is the GraphQL resolver of all things related to batch changes.
type Resolver struct {
	store           *store.Store
	gitserverClient gitserver.Client
	db              database.DB
	logger          log.Logger
}

// New returns a new Resolver whose store uses the given database
func New(db database.DB, store *store.Store, gitserverClient gitserver.Client, logger log.Logger) graphqlbackend.BatchChangesResolver {
	return &Resolver{store: store, gitserverClient: gitserverClient, db: db, logger: logger}
}

// batchChangesCreateAccess returns true if the current user has batch changes enabled for
// them and can create batchChanges/changesetSpecs/batchSpecs.
func batchChangesCreateAccess(ctx context.Context, db database.DB) error {
	if err := enterprise.BatchChangesEnabledForUser(ctx, db); err != nil {
		return err
	}

	act := sgactor.FromContext(ctx)
	if !act.IsAuthenticated() {
		return auth.ErrNotAuthenticated
	}
	return nil
}

// checkLicense returns the current plan's configured Batch Changes feature.
// Returns a user-facing error if the batchChanges feature is not purchased
// with the current license or any error occurred while validating the license.
func checkLicense() (*licensing.FeatureBatchChanges, error) {
	bcFeature := &licensing.FeatureBatchChanges{}
	if err := licensing.Check(bcFeature); err != nil {
		return nil, err
	}

	return bcFeature, nil
}

type batchSpecCreatedArg struct {
	ChangesetSpecsCount int `json:"changeset_specs_count"`
}

type batchChangeEventArg struct {
	BatchChangeID int64 `json:"batch_change_id"`
}

// TODO: Use EventRecorder from internal/telemetryrecorder instead.
func logBackendEvent(ctx context.Context, db database.DB, name string, args any, publicArgs any) error {
	actor := sgactor.FromContext(ctx)
	jsonArg, err := json.Marshal(args)
	if err != nil {
		return err
	}
	jsonPublicArg, err := json.Marshal(publicArgs)
	if err != nil {
		return err
	}

	//lint:ignore SA1019 existing usage of deprecated functionality.
	return usagestats.LogBackendEvent(db, actor.UID, deviceid.FromContext(ctx), name, jsonArg, jsonPublicArg, featureflag.GetEvaluatedFlagSet(ctx), nil)
}

func (r *Resolver) NodeResolvers() map[string]graphqlbackend.NodeByIDFunc {
	return map[string]graphqlbackend.NodeByIDFunc{
		batchChangeIDKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
			return r.batchChangeByID(ctx, id)
		},
		batchSpecIDKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
			return r.batchSpecByID(ctx, id)
		},
		changesetSpecIDKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
			return r.changesetSpecByID(ctx, id)
		},
		changesetIDKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
			return r.changesetByID(ctx, id)
		},
		batchChangesCredentialIDKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
			return r.batchChangesCredentialByID(ctx, id)
		},
		bulkOperationIDKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
			return r.bulkOperationByID(ctx, id)
		},
		batchSpecWorkspaceIDKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
			return r.batchSpecWorkspaceByID(ctx, id)
		},
		workspaceFileIDKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
			return r.batchSpecWorkspaceFileByID(ctx, id)
		},
	}
}

func (r *Resolver) changesetByID(ctx context.Context, id graphql.ID) (graphqlbackend.ChangesetResolver, error) {
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	changesetID, err := unmarshalChangesetID(id)
	if err != nil {
		return nil, err
	}

	if changesetID == 0 {
		return nil, ErrIDIsZero{}
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

	return NewChangesetResolver(r.store, r.gitserverClient, r.logger, changeset, repo), nil
}

func (r *Resolver) batchChangeByID(ctx context.Context, id graphql.ID) (graphqlbackend.BatchChangeResolver, error) {
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	batchChangeID, err := unmarshalBatchChangeID(id)
	if err != nil {
		return nil, err
	}

	if batchChangeID == 0 {
		return nil, ErrIDIsZero{}
	}

	batchChange, err := r.store.GetBatchChange(ctx, store.GetBatchChangeOpts{ID: batchChangeID})
	if err != nil {
		if err == store.ErrNoResults {
			return nil, nil
		}
		return nil, err
	}

	return &batchChangeResolver{store: r.store, gitserverClient: r.gitserverClient, batchChange: batchChange, logger: r.logger}, nil
}

func (r *Resolver) BatchChange(ctx context.Context, args *graphqlbackend.BatchChangeArgs) (graphqlbackend.BatchChangeResolver, error) {
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	opts := store.GetBatchChangeOpts{Name: args.Name}

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

	return &batchChangeResolver{store: r.store, gitserverClient: r.gitserverClient, batchChange: batchChange, logger: r.logger}, nil
}

func (r *Resolver) ResolveWorkspacesForBatchSpec(ctx context.Context, args *graphqlbackend.ResolveWorkspacesForBatchSpecArgs) ([]graphqlbackend.ResolvedBatchSpecWorkspaceResolver, error) {
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	// Parse the batch spec.
	evaluatableSpec, err := batcheslib.ParseBatchSpec([]byte(args.BatchSpec))
	if err != nil {
		return nil, err
	}

	// Verify the user is authenticated.
	act := sgactor.FromContext(ctx)
	if !act.IsAuthenticated() {
		return nil, auth.ErrNotAuthenticated
	}

	// Run the resolution.
	resolver := service.NewWorkspaceResolver(r.store)
	workspaces, err := resolver.ResolveWorkspacesForBatchSpec(ctx, evaluatableSpec)
	if err != nil {
		return nil, err
	}

	// Transform the result into resolvers.
	resolvers := make([]graphqlbackend.ResolvedBatchSpecWorkspaceResolver, 0, len(workspaces))
	for _, w := range workspaces {
		resolvers = append(resolvers, &resolvedBatchSpecWorkspaceResolver{store: r.store, workspace: w})
	}

	return resolvers, nil
}

func (r *Resolver) batchSpecByID(ctx context.Context, id graphql.ID) (graphqlbackend.BatchSpecResolver, error) {
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	batchSpecRandID, err := unmarshalBatchSpecID(id)
	if err != nil {
		return nil, err
	}

	if batchSpecRandID == "" {
		return nil, ErrIDIsZero{}
	}
	batchSpec, err := r.store.GetBatchSpec(ctx, store.GetBatchSpecOpts{RandID: batchSpecRandID})
	if err != nil {
		if err == store.ErrNoResults {
			return nil, nil
		}
		return nil, err
	}

	// Everyone can see a batch spec, if they have the rand ID. The batch specs won't be
	// enumerated to users other than their creators + admins, but they can be accessed
	// directly if shared, e.g. to share a preview link before applying a new batch spec.
	return &batchSpecResolver{store: r.store, logger: r.logger, batchSpec: batchSpec}, nil
}

func (r *Resolver) changesetSpecByID(ctx context.Context, id graphql.ID) (graphqlbackend.ChangesetSpecResolver, error) {
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	changesetSpecRandID, err := unmarshalChangesetSpecID(id)
	if err != nil {
		return nil, err
	}

	if changesetSpecRandID == "" {
		return nil, ErrIDIsZero{}
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

type batchChangesCredentialResolver interface {
	graphqlbackend.BatchChangesCredentialResolver
	authenticator(ctx context.Context) (extsvcauth.Authenticator, error)
}

func (r *Resolver) batchChangesCredentialByID(ctx context.Context, id graphql.ID) (batchChangesCredentialResolver, error) {
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	dbID, isSiteCredential, err := unmarshalBatchChangesCredentialID(id)
	if err != nil {
		return nil, err
	}

	if dbID == 0 {
		return nil, ErrIDIsZero{}
	}

	if isSiteCredential {
		return r.batchChangesSiteCredentialByID(ctx, dbID)
	}

	return r.batchChangesUserCredentialByID(ctx, dbID)
}

func (r *Resolver) batchChangesUserCredentialByID(ctx context.Context, id int64) (batchChangesCredentialResolver, error) {
	cred, err := r.store.UserCredentials().GetByID(ctx, id)
	if err != nil {
		if err == store.ErrNoResults {
			return nil, nil
		}
		return nil, err
	}

	if err := auth.CheckSiteAdminOrSameUser(ctx, r.store.DatabaseDB(), cred.UserID); err != nil {
		return nil, err
	}

	return &batchChangesUserCredentialResolver{credential: cred}, nil
}

func (r *Resolver) batchChangesSiteCredentialByID(ctx context.Context, id int64) (batchChangesCredentialResolver, error) {
	// Todo: Is this required? Should everyone be able to see there are _some_ credentials?
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	cred, err := r.store.GetSiteCredential(ctx, store.GetSiteCredentialOpts{ID: id})
	if err != nil {
		if err == store.ErrNoResults {
			return nil, nil
		}
		return nil, err
	}

	return &batchChangesSiteCredentialResolver{credential: cred}, nil
}

func (r *Resolver) bulkOperationByID(ctx context.Context, id graphql.ID) (graphqlbackend.BulkOperationResolver, error) {
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	dbID, err := unmarshalBulkOperationID(id)
	if err != nil {
		return nil, err
	}

	if dbID == "" {
		return nil, ErrIDIsZero{}
	}

	return r.bulkOperationByIDString(ctx, dbID)
}

func (r *Resolver) bulkOperationByIDString(ctx context.Context, id string) (graphqlbackend.BulkOperationResolver, error) {
	bulkOperation, err := r.store.GetBulkOperation(ctx, store.GetBulkOperationOpts{ID: id})
	if err != nil {
		if err == store.ErrNoResults {
			return nil, nil
		}
		return nil, err
	}
	return &bulkOperationResolver{store: r.store, gitserverClient: r.gitserverClient, bulkOperation: bulkOperation, logger: r.logger}, nil
}

func (r *Resolver) batchSpecWorkspaceByID(ctx context.Context, gqlID graphql.ID) (graphqlbackend.BatchSpecWorkspaceResolver, error) {
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	id, err := unmarshalBatchSpecWorkspaceID(gqlID)
	if err != nil {
		return nil, err
	}

	if id == 0 {
		return nil, ErrIDIsZero{}
	}

	w, err := r.store.GetBatchSpecWorkspace(ctx, store.GetBatchSpecWorkspaceOpts{ID: id})
	if err != nil {
		if err == store.ErrNoResults {
			return nil, nil
		}
		return nil, err
	}

	spec, err := r.store.GetBatchSpec(ctx, store.GetBatchSpecOpts{ID: w.BatchSpecID})
	if err != nil {
		return nil, err
	}

	ex, err := r.store.GetBatchSpecWorkspaceExecutionJob(ctx, store.GetBatchSpecWorkspaceExecutionJobOpts{BatchSpecWorkspaceID: w.ID})
	if err != nil && err != store.ErrNoResults {
		return nil, err
	}

	return newBatchSpecWorkspaceResolver(ctx, r.store, r.logger, w, ex, spec.Spec)
}

func (r *Resolver) CreateBatchChange(ctx context.Context, args *graphqlbackend.CreateBatchChangeArgs) (_ graphqlbackend.BatchChangeResolver, err error) {
	tr, _ := trace.New(ctx, "Resolver.CreateBatchChange", attribute.String("BatchSpec", string(args.BatchSpec)))
	defer tr.EndWithErr(&err)

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	opts := service.ApplyBatchChangeOpts{
		// This is what differentiates CreateBatchChange from ApplyBatchChange
		FailIfBatchChangeExists: true,
	}
	batchChange, err := r.applyOrCreateBatchChange(ctx, &graphqlbackend.ApplyBatchChangeArgs{
		BatchSpec:         args.BatchSpec,
		EnsureBatchChange: nil,
		PublicationStates: args.PublicationStates,
	}, opts)
	if err != nil {
		return nil, err
	}

	arg := &batchChangeEventArg{BatchChangeID: batchChange.ID}
	err = logBackendEvent(ctx, r.store.DatabaseDB(), "BatchChangeCreated", arg, arg)
	if err != nil {
		return nil, err
	}

	return &batchChangeResolver{store: r.store, gitserverClient: r.gitserverClient, batchChange: batchChange, logger: r.logger}, nil
}

func (r *Resolver) ApplyBatchChange(ctx context.Context, args *graphqlbackend.ApplyBatchChangeArgs) (_ graphqlbackend.BatchChangeResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.ApplyBatchChange", attribute.String("BatchSpec", string(args.BatchSpec)))
	defer tr.EndWithErr(&err)

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	batchChange, err := r.applyOrCreateBatchChange(ctx, args, service.ApplyBatchChangeOpts{})
	if err != nil {
		return nil, err
	}

	arg := &batchChangeEventArg{BatchChangeID: batchChange.ID}
	err = logBackendEvent(ctx, r.store.DatabaseDB(), "BatchChangeCreatedOrUpdated", arg, arg)
	if err != nil {
		return nil, err
	}

	return &batchChangeResolver{store: r.store, gitserverClient: r.gitserverClient, batchChange: batchChange, logger: r.logger}, nil
}

func addPublicationStatesToOptions(in *[]graphqlbackend.ChangesetSpecPublicationStateInput, opts *service.UiPublicationStates) error {
	var errs error

	if in != nil && *in != nil {
		for _, state := range *in {
			id, err := unmarshalChangesetSpecID(state.ChangesetSpec)
			if err != nil {
				return err
			}

			if err := opts.Add(id, state.PublicationState); err != nil {
				errs = errors.Append(errs, err)
			}
		}

	}

	return errs
}

func (r *Resolver) applyOrCreateBatchChange(ctx context.Context, args *graphqlbackend.ApplyBatchChangeArgs, opts service.ApplyBatchChangeOpts) (*btypes.BatchChange, error) {
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	var err error
	if opts.BatchSpecRandID, err = unmarshalBatchSpecID(args.BatchSpec); err != nil {
		return nil, err
	}

	if opts.BatchSpecRandID == "" {
		return nil, ErrIDIsZero{}
	}

	if batchChangesFeature, licenseErr := checkLicense(); licenseErr == nil {
		batchSpec, err := r.store.GetBatchSpec(ctx, store.GetBatchSpecOpts{
			RandID: opts.BatchSpecRandID,
		})
		if err != nil {
			return nil, err
		}
		count, err := r.store.CountChangesetSpecs(ctx, store.CountChangesetSpecsOpts{BatchSpecID: batchSpec.ID})
		if err != nil {
			return nil, err
		}
		if !batchChangesFeature.Unrestricted && count > batchChangesFeature.MaxNumChangesets {
			return nil, ErrBatchChangesOverLimit{errors.Newf("maximum number of changesets per batch change (%d) exceeded", batchChangesFeature.MaxNumChangesets)}
		}
	} else {
		return nil, ErrBatchChangesUnlicensed{licenseErr}
	}

	if args.EnsureBatchChange != nil {
		opts.EnsureBatchChangeID, err = unmarshalBatchChangeID(*args.EnsureBatchChange)
		if err != nil {
			return nil, err
		}
	}

	if err := addPublicationStatesToOptions(args.PublicationStates, &opts.PublicationStates); err != nil {
		return nil, err
	}

	svc := service.New(r.store)
	// 🚨 SECURITY: ApplyBatchChange checks whether the user has permission to
	// apply the batch spec.
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

	return batchChange, nil
}

func (r *Resolver) CreateBatchSpec(ctx context.Context, args *graphqlbackend.CreateBatchSpecArgs) (_ graphqlbackend.BatchSpecResolver, err error) {
	tr, ctx := trace.New(ctx, "CreateBatchSpec", attribute.String("namespace", string(args.Namespace)), attribute.String("batchSpec", string(args.BatchSpec)))
	defer tr.EndWithErr(&err)

	if err := batchChangesCreateAccess(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	if batchChangesFeature, err := checkLicense(); err == nil {
		if !batchChangesFeature.Unrestricted && len(args.ChangesetSpecs) > batchChangesFeature.MaxNumChangesets {
			return nil, ErrBatchChangesOverLimit{errors.Newf("maximum number of changesets per batch change (%d) exceeded", batchChangesFeature.MaxNumChangesets)}
		}
	} else {
		return nil, ErrBatchChangesUnlicensed{err}
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

	eventArg := &batchSpecCreatedArg{ChangesetSpecsCount: len(opts.ChangesetSpecRandIDs)}
	if err := logBackendEvent(ctx, r.store.DatabaseDB(), "BatchSpecCreated", eventArg, eventArg); err != nil {
		return nil, err
	}

	specResolver := &batchSpecResolver{
		store:     r.store,
		logger:    r.logger,
		batchSpec: batchSpec,
	}

	return specResolver, nil
}

func (r *Resolver) CreateChangesetSpec(ctx context.Context, args *graphqlbackend.CreateChangesetSpecArgs) (_ graphqlbackend.ChangesetSpecResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.CreateChangesetSpec")
	defer tr.EndWithErr(&err)

	if err := batchChangesCreateAccess(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	act := sgactor.FromContext(ctx)
	// Actor MUST be logged in at this stage, because batchChangesCreateAccess checks that already.
	// To be extra safe, we'll just do the cheap check again here so if anyone ever modifies
	// batchChangesCreateAccess, we still enforce it here.
	if !act.IsAuthenticated() {
		return nil, auth.ErrNotAuthenticated
	}

	svc := service.New(r.store)
	spec, err := svc.CreateChangesetSpec(ctx, args.ChangesetSpec, act.UID)
	if err != nil {
		return nil, err
	}

	return NewChangesetSpecResolver(ctx, r.store, spec)
}

func (r *Resolver) CreateChangesetSpecs(ctx context.Context, args *graphqlbackend.CreateChangesetSpecsArgs) (_ []graphqlbackend.ChangesetSpecResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.CreateChangesetSpecs")
	defer tr.EndWithErr(&err)

	if err := batchChangesCreateAccess(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	act := sgactor.FromContext(ctx)
	// Actor MUST be logged in at this stage, because batchChangesCreateAccess checks that already.
	// To be extra safe, we'll just do the cheap check again here so if anyone ever modifies
	// batchChangesCreateAccess, we still enforce it here.
	if !act.IsAuthenticated() {
		return nil, auth.ErrNotAuthenticated
	}

	svc := service.New(r.store)
	specs, err := svc.CreateChangesetSpecs(ctx, args.ChangesetSpecs, act.UID)
	if err != nil {
		return nil, err
	}

	resolvers := make([]graphqlbackend.ChangesetSpecResolver, len(specs))
	for i, spec := range specs {
		resolver, err := NewChangesetSpecResolver(ctx, r.store, spec)
		if err != nil {
			return nil, err
		}
		resolvers[i] = resolver
	}

	return resolvers, nil
}

func (r *Resolver) MoveBatchChange(ctx context.Context, args *graphqlbackend.MoveBatchChangeArgs) (_ graphqlbackend.BatchChangeResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.MoveBatchChange", attribute.String("batchChange", string(args.BatchChange)))
	defer tr.EndWithErr(&err)

	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
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

	return &batchChangeResolver{store: r.store, gitserverClient: r.gitserverClient, batchChange: batchChange, logger: r.logger}, nil
}

func (r *Resolver) DeleteBatchChange(ctx context.Context, args *graphqlbackend.DeleteBatchChangeArgs) (_ *graphqlbackend.EmptyResponse, err error) {
	tr, ctx := trace.New(ctx, "Resolver.DeleteBatchChange", attribute.String("batchChange", string(args.BatchChange)))
	defer tr.EndWithErr(&err)
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
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
	if err != nil {
		return nil, err
	}

	arg := &batchChangeEventArg{BatchChangeID: batchChangeID}
	if err := logBackendEvent(ctx, r.store.DatabaseDB(), "BatchChangeDeleted", arg, arg); err != nil {
		return nil, err
	}

	return &graphqlbackend.EmptyResponse{}, err
}

func (r *Resolver) BatchChanges(ctx context.Context, args *graphqlbackend.ListBatchChangesArgs) (graphqlbackend.BatchChangesConnectionResolver, error) {
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	opts := store.ListBatchChangesOpts{}

	state, err := parseBatchChangeState(args.State)
	if err != nil {
		return nil, err
	}
	if state != "" {
		opts.States = []btypes.BatchChangeState{state}
	}

	// If multiple `states` are provided, prefer them over `state`.
	if args.States != nil {
		states, err := parseBatchChangeStates(args.States)
		if err != nil {
			return nil, err
		}
		opts.States = states
	}

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

	authErr := auth.CheckCurrentUserIsSiteAdmin(ctx, r.store.DatabaseDB())
	if authErr != nil && authErr != auth.ErrMustBeSiteAdmin {
		return nil, authErr
	}
	isSiteAdmin := authErr != auth.ErrMustBeSiteAdmin
	if !isSiteAdmin {
		actor := sgactor.FromContext(ctx)
		if args.ViewerCanAdminister != nil && *args.ViewerCanAdminister {
			opts.OnlyAdministeredByUserID = actor.UID
		}

		// 🚨 SECURITY: If the user is not an admin, we don't want to include
		// unapplied (draft) BatchChanges except those that the user owns.
		opts.ExcludeDraftsNotOwnedByUserID = actor.UID
	}

	if args.Namespace != nil {
		err := graphqlbackend.UnmarshalNamespaceID(*args.Namespace, &opts.NamespaceUserID, &opts.NamespaceOrgID)
		if err != nil {
			return nil, err
		}
	}

	if args.Repo != nil {
		repoID, err := graphqlbackend.UnmarshalRepositoryID(*args.Repo)
		if err != nil {
			return nil, err
		}
		opts.RepoID = repoID
	}

	return &batchChangesConnectionResolver{
		store:           r.store,
		gitserverClient: r.gitserverClient,
		logger:          r.logger,
		opts:            opts,
	}, nil
}

func (r *Resolver) RepoChangesetsStats(ctx context.Context, repo *graphql.ID) (graphqlbackend.RepoChangesetsStatsResolver, error) {
	repoID, err := graphqlbackend.UnmarshalRepositoryID(*repo)
	if err != nil {
		return nil, err
	}

	stats, err := r.store.GetRepoChangesetsStats(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return &repoChangesetsStatsResolver{stats: *stats}, nil
}

func (r *Resolver) GlobalChangesetsStats(
	ctx context.Context,
) (graphqlbackend.GlobalChangesetsStatsResolver, error) {
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}
	stats, err := r.store.GetGlobalChangesetsStats(ctx)
	if err != nil {
		return nil, err
	}
	return &globalChangesetsStatsResolver{stats: *stats}, nil
}

func (r *Resolver) RepoDiffStat(ctx context.Context, repo *graphql.ID) (*graphqlbackend.DiffStat, error) {
	repoID, err := graphqlbackend.UnmarshalRepositoryID(*repo)
	if err != nil {
		return nil, err
	}

	diffStat, err := r.store.GetRepoDiffStat(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return graphqlbackend.NewDiffStat(*diffStat), nil
}

func (r *Resolver) BatchChangesCodeHosts(ctx context.Context, args *graphqlbackend.ListBatchChangesCodeHostsArgs) (graphqlbackend.BatchChangesCodeHostConnectionResolver, error) {
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if args.UserID != nil {
		// 🚨 SECURITY: Only viewable for self or by site admins.
		if err := auth.CheckSiteAdminOrSameUser(ctx, r.store.DatabaseDB(), *args.UserID); err != nil {
			return nil, err
		}
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

	return &batchChangesCodeHostConnectionResolver{userID: args.UserID, limitOffset: limitOffset, store: r.store, db: r.db, logger: r.logger}, nil
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

	if args.OnlyClosable != nil && *args.OnlyClosable {
		if args.State != nil {
			return opts, false, errors.New("invalid combination of state and onlyClosable")
		}

		opts.States = []btypes.ChangesetState{btypes.ChangesetStateOpen, btypes.ChangesetStateDraft}
	}

	if args.State != nil {
		state := btypes.ChangesetState(*args.State)
		if !state.Valid() {
			return opts, false, errors.New("changeset state not valid")
		}

		opts.States = []btypes.ChangesetState{state}
	}

	if args.ReviewState != nil {
		state := btypes.ChangesetReviewState(*args.ReviewState)
		if !state.Valid() {
			return opts, false, errors.New("changeset review state not valid")
		}
		opts.ExternalReviewState = &state
		// If the user filters by ReviewState we cannot include hidden
		// changesets, since that would leak information.
		safe = false
	}
	if args.CheckState != nil {
		state := btypes.ChangesetCheckState(*args.CheckState)
		if !state.Valid() {
			return opts, false, errors.New("changeset check state not valid")
		}
		opts.ExternalCheckState = &state
		// If the user filters by CheckState we cannot include hidden
		// changesets, since that would leak information.
		safe = false
	}
	if args.OnlyPublishedByThisBatchChange != nil {
		published := btypes.ChangesetPublicationStatePublished

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
	if args.OnlyArchived {
		opts.OnlyArchived = args.OnlyArchived
	}
	if args.Repo != nil {
		repoID, err := graphqlbackend.UnmarshalRepositoryID(*args.Repo)
		if err != nil {
			return opts, false, errors.Wrap(err, "unmarshalling repo id")
		}
		opts.RepoIDs = []api.RepoID{repoID}
	}

	return opts, safe, nil
}

func (r *Resolver) CloseBatchChange(ctx context.Context, args *graphqlbackend.CloseBatchChangeArgs) (_ graphqlbackend.BatchChangeResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.CloseBatchChange", attribute.String("batchChange", string(args.BatchChange)))
	defer tr.EndWithErr(&err)

	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
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

	arg := &batchChangeEventArg{BatchChangeID: batchChangeID}
	if err := logBackendEvent(ctx, r.store.DatabaseDB(), "BatchChangeClosed", arg, arg); err != nil {
		return nil, err
	}

	return &batchChangeResolver{store: r.store, gitserverClient: r.gitserverClient, batchChange: batchChange, logger: r.logger}, nil
}

func (r *Resolver) SyncChangeset(ctx context.Context, args *graphqlbackend.SyncChangesetArgs) (_ *graphqlbackend.EmptyResponse, err error) {
	tr, ctx := trace.New(ctx, "Resolver.SyncChangeset", attribute.String("changeset", string(args.Changeset)))
	defer tr.EndWithErr(&err)
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
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
	tr, ctx := trace.New(ctx, "Resolver.ReenqueueChangeset", attribute.String("changeset", string(args.Changeset)))
	defer tr.EndWithErr(&err)
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
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

	return NewChangesetResolver(r.store, r.gitserverClient, r.logger, changeset, repo), nil
}

func (r *Resolver) CreateBatchChangesCredential(ctx context.Context, args *graphqlbackend.CreateBatchChangesCredentialArgs) (_ graphqlbackend.BatchChangesCredentialResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.CreateBatchChangesCredential",
		attribute.String("externalServiceKind", args.ExternalServiceKind),
		attribute.String("externalServiceURL", args.ExternalServiceURL),
	)
	defer tr.EndWithErr(&err)
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	var userID int32
	if args.User != nil {
		userID, err = graphqlbackend.UnmarshalUserID(*args.User)
		if err != nil {
			return nil, err
		}

		if userID == 0 {
			return nil, ErrIDIsZero{}
		}
	}

	// Need to validate externalServiceKind, otherwise this'll panic.
	kind, valid := extsvc.ParseServiceKind(args.ExternalServiceKind)
	if !valid {
		return nil, errors.New("invalid external service kind")
	}

	if args.Credential == "" {
		return nil, errors.New("empty credential not allowed")
	}

	svc := service.New(r.store)

	if userID != 0 {
		cred, err := svc.CreateBatchChangesUserCredential(ctx, sources.AuthenticationStrategyUserCredential, service.CreateBatchChangesUserCredentialArgs{
			ExternalServiceURL:  args.ExternalServiceURL,
			ExternalServiceType: extsvc.KindToType(kind),
			UserID:              userID,
			Credential:          args.Credential,
			Username:            args.Username,
		})
		if err != nil {
			return nil, err
		}
		return &batchChangesUserCredentialResolver{credential: cred}, nil
	}

	cred, err := svc.CreateBatchChangesSiteCredential(ctx, args.ExternalServiceURL, extsvc.KindToType(kind), args.Credential, args.Username, sources.AuthenticationStrategyUserCredential)
	if err != nil {
		return nil, err
	}
	return &batchChangesSiteCredentialResolver{credential: cred}, nil
}

func (r *Resolver) DeleteBatchChangesCredential(ctx context.Context, args *graphqlbackend.DeleteBatchChangesCredentialArgs) (_ *graphqlbackend.EmptyResponse, err error) {
	tr, ctx := trace.New(ctx, "Resolver.DeleteBatchChangesCredential", attribute.String("credential", string(args.BatchChangesCredential)))
	defer tr.EndWithErr(&err)
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	dbID, isSiteCredential, err := unmarshalBatchChangesCredentialID(args.BatchChangesCredential)
	if err != nil {
		return nil, err
	}

	if dbID == 0 {
		return nil, ErrIDIsZero{}
	}

	if isSiteCredential {
		return r.deleteBatchChangesSiteCredential(ctx, dbID)
	}

	return r.deleteBatchChangesUserCredential(ctx, dbID)
}

func (r *Resolver) deleteBatchChangesUserCredential(ctx context.Context, credentialDBID int64) (*graphqlbackend.EmptyResponse, error) {
	// Get existing credential.
	cred, err := r.store.UserCredentials().GetByID(ctx, credentialDBID)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Check that the requesting user may delete the credential.
	if err := auth.CheckSiteAdminOrSameUser(ctx, r.store.DatabaseDB(), cred.UserID); err != nil {
		return nil, err
	}

	// This also fails if the credential was not found.
	if err := r.store.UserCredentials().Delete(ctx, credentialDBID); err != nil {
		return nil, err
	}

	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *Resolver) deleteBatchChangesSiteCredential(ctx context.Context, credentialDBID int64) (*graphqlbackend.EmptyResponse, error) {
	// 🚨 SECURITY: Check that the requesting user may delete the credential.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	// This also fails if the credential was not found.
	if err := r.store.DeleteSiteCredential(ctx, credentialDBID); err != nil {
		return nil, err
	}

	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *Resolver) DetachChangesets(ctx context.Context, args *graphqlbackend.DetachChangesetsArgs) (_ graphqlbackend.BulkOperationResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.DetachChangesets",
		attribute.String("batchChange", string(args.BatchChange)),
		attribute.Int("changesets.len", len(args.Changesets)))
	defer tr.EndWithErr(&err)
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	batchChangeID, changesetIDs, err := unmarshalBulkOperationBaseArgs(args.BulkOperationBaseArgs)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: CreateChangesetJobs checks whether current user is authorized.
	svc := service.New(r.store)
	bulkGroupID, err := svc.CreateChangesetJobs(
		ctx,
		batchChangeID,
		changesetIDs,
		btypes.ChangesetJobTypeDetach,
		&btypes.ChangesetJobDetachPayload{},
		store.ListChangesetsOpts{
			// Only allow to run this on archived changesets.
			OnlyArchived: true,
		},
	)
	if err != nil {
		return nil, err
	}

	return r.bulkOperationByIDString(ctx, bulkGroupID)
}

func (r *Resolver) CreateChangesetComments(ctx context.Context, args *graphqlbackend.CreateChangesetCommentsArgs) (_ graphqlbackend.BulkOperationResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.CreateChangesetComments",
		attribute.String("batchChange", string(args.BatchChange)),
		attribute.Int("changesets.len", len(args.Changesets)))
	defer tr.EndWithErr(&err)
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	if args.Body == "" {
		return nil, errors.New("empty comment body is not allowed")
	}

	batchChangeID, changesetIDs, err := unmarshalBulkOperationBaseArgs(args.BulkOperationBaseArgs)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: CreateChangesetJobs checks whether current user is authorized.
	svc := service.New(r.store)
	published := btypes.ChangesetPublicationStatePublished
	bulkGroupID, err := svc.CreateChangesetJobs(
		ctx,
		batchChangeID,
		changesetIDs,
		btypes.ChangesetJobTypeComment,
		&btypes.ChangesetJobCommentPayload{
			Message: args.Body,
		},
		store.ListChangesetsOpts{
			// Also include archived changesets, we allow commenting on them as well.
			IncludeArchived: true,
			// We can only comment on published changesets.
			PublicationState: &published,
		},
	)
	if err != nil {
		return nil, err
	}

	return r.bulkOperationByIDString(ctx, bulkGroupID)
}

func (r *Resolver) ReenqueueChangesets(ctx context.Context, args *graphqlbackend.ReenqueueChangesetsArgs) (_ graphqlbackend.BulkOperationResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.ReenqueueChangesets",
		attribute.String("batchChange", string(args.BatchChange)),
		attribute.Int("changesets.len", len(args.Changesets)))
	defer tr.EndWithErr(&err)
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	batchChangeID, changesetIDs, err := unmarshalBulkOperationBaseArgs(args.BulkOperationBaseArgs)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: CreateChangesetJobs checks whether current user is authorized.
	svc := service.New(r.store)
	bulkGroupID, err := svc.CreateChangesetJobs(
		ctx,
		batchChangeID,
		changesetIDs,
		btypes.ChangesetJobTypeReenqueue,
		&btypes.ChangesetJobReenqueuePayload{},
		store.ListChangesetsOpts{
			// Only allow to retry failed changesets.
			ReconcilerStates: []btypes.ReconcilerState{btypes.ReconcilerStateFailed},
		},
	)
	if err != nil {
		return nil, err
	}

	return r.bulkOperationByIDString(ctx, bulkGroupID)
}

func (r *Resolver) MergeChangesets(ctx context.Context, args *graphqlbackend.MergeChangesetsArgs) (_ graphqlbackend.BulkOperationResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.MergeChangesets",
		attribute.String("batchChange", string(args.BatchChange)),
		attribute.Int("changesets.len", len(args.Changesets)))
	defer tr.EndWithErr(&err)
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	batchChangeID, changesetIDs, err := unmarshalBulkOperationBaseArgs(args.BulkOperationBaseArgs)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: CreateChangesetJobs checks whether current user is authorized.
	svc := service.New(r.store)
	published := btypes.ChangesetPublicationStatePublished
	openState := btypes.ChangesetExternalStateOpen
	bulkGroupID, err := svc.CreateChangesetJobs(
		ctx,
		batchChangeID,
		changesetIDs,
		btypes.ChangesetJobTypeMerge,
		&btypes.ChangesetJobMergePayload{Squash: args.Squash},
		store.ListChangesetsOpts{
			PublicationState: &published,
			ReconcilerStates: []btypes.ReconcilerState{btypes.ReconcilerStateCompleted},
			ExternalStates:   []btypes.ChangesetExternalState{openState},
		},
	)
	if err != nil {
		return nil, err
	}

	return r.bulkOperationByIDString(ctx, bulkGroupID)
}

func (r *Resolver) CloseChangesets(ctx context.Context, args *graphqlbackend.CloseChangesetsArgs) (_ graphqlbackend.BulkOperationResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.CloseChangesets",
		attribute.String("batchChange", string(args.BatchChange)),
		attribute.Int("changesets.len", len(args.Changesets)))
	defer tr.EndWithErr(&err)
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	batchChangeID, changesetIDs, err := unmarshalBulkOperationBaseArgs(args.BulkOperationBaseArgs)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: CreateChangesetJobs checks whether current user is authorized.
	svc := service.New(r.store)
	published := btypes.ChangesetPublicationStatePublished
	bulkGroupID, err := svc.CreateChangesetJobs(
		ctx,
		batchChangeID,
		changesetIDs,
		btypes.ChangesetJobTypeClose,
		&btypes.ChangesetJobClosePayload{},
		store.ListChangesetsOpts{
			PublicationState: &published,
			ReconcilerStates: []btypes.ReconcilerState{btypes.ReconcilerStateCompleted},
			ExternalStates:   []btypes.ChangesetExternalState{btypes.ChangesetExternalStateOpen, btypes.ChangesetExternalStateDraft},
		},
	)
	if err != nil {
		return nil, err
	}

	return r.bulkOperationByIDString(ctx, bulkGroupID)
}

func (r *Resolver) PublishChangesets(ctx context.Context, args *graphqlbackend.PublishChangesetsArgs) (_ graphqlbackend.BulkOperationResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.PublishChangesets",
		attribute.String("batchChange", string(args.BatchChange)),
		attribute.Int("changesets.len", len(args.Changesets)))
	defer tr.EndWithErr(&err)
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	batchChangeID, changesetIDs, err := unmarshalBulkOperationBaseArgs(args.BulkOperationBaseArgs)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: CreateChangesetJobs checks whether current user is authorized.
	svc := service.New(r.store)
	bulkGroupID, err := svc.CreateChangesetJobs(
		ctx,
		batchChangeID,
		changesetIDs,
		btypes.ChangesetJobTypePublish,
		&btypes.ChangesetJobPublishPayload{Draft: args.Draft},
		store.ListChangesetsOpts{},
	)
	if err != nil {
		return nil, err
	}

	return r.bulkOperationByIDString(ctx, bulkGroupID)
}

func (r *Resolver) BatchSpecs(ctx context.Context, args *graphqlbackend.ListBatchSpecArgs) (_ graphqlbackend.BatchSpecConnectionResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.BatchSpecs",
		attribute.Int("first", int(args.First)),
		attribute.String("after", fmt.Sprintf("%v", args.After)))
	defer tr.EndWithErr(&err)

	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := validateFirstParamDefaults(args.First); err != nil {
		return nil, err
	}

	opts := store.ListBatchSpecsOpts{
		LimitOpts: store.LimitOpts{
			Limit: int(args.First),
		},
		NewestFirst: true,
	}

	if args.IncludeLocallyExecutedSpecs != nil {
		opts.IncludeLocallyExecutedSpecs = *args.IncludeLocallyExecutedSpecs
	}

	if args.ExcludeEmptySpecs != nil {
		opts.ExcludeEmptySpecs = *args.ExcludeEmptySpecs
	}

	// 🚨 SECURITY: If the user is not an admin, we don't want to include
	// BatchSpecs that were created with CreateBatchSpecFromRaw and not owned
	// by the user
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.store.DatabaseDB()); err != nil {
		opts.ExcludeCreatedFromRawNotOwnedByUser = sgactor.FromContext(ctx).UID
	}

	if args.After != nil {
		id, err := strconv.Atoi(*args.After)
		if err != nil {
			return nil, err
		}
		opts.Cursor = int64(id)
	}

	return &batchSpecConnectionResolver{store: r.store, logger: r.logger, opts: opts}, nil
}

func (r *Resolver) CreateEmptyBatchChange(ctx context.Context, args *graphqlbackend.CreateEmptyBatchChangeArgs) (_ graphqlbackend.BatchChangeResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.CreateEmptyBatchChange",
		attribute.String("namespace", string(args.Namespace)))
	defer tr.EndWithErr(&err)
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	svc := service.New(r.store)

	var uid, oid int32
	if err := graphqlbackend.UnmarshalNamespaceID(args.Namespace, &uid, &oid); err != nil {
		return nil, err
	}

	batchChange, err := svc.CreateEmptyBatchChange(ctx, service.CreateEmptyBatchChangeOpts{
		NamespaceUserID: uid,
		NamespaceOrgID:  oid,
		Name:            args.Name,
	})

	if err != nil {
		// Render pretty error.
		if err == store.ErrInvalidBatchChangeName {
			return nil, ErrBatchChangeInvalidName{}
		}
		return nil, err
	}

	return &batchChangeResolver{store: r.store, gitserverClient: r.gitserverClient, batchChange: batchChange, logger: r.logger}, nil
}

func (r *Resolver) UpsertEmptyBatchChange(ctx context.Context, args *graphqlbackend.UpsertEmptyBatchChangeArgs) (_ graphqlbackend.BatchChangeResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.UpsertEmptyBatchChange",
		attribute.String("namespace", string(args.Namespace)))
	defer tr.EndWithErr(&err)
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	svc := service.New(r.store)

	var uid, oid int32
	if err := graphqlbackend.UnmarshalNamespaceID(args.Namespace, &uid, &oid); err != nil {
		return nil, err
	}

	batchChange, err := svc.UpsertEmptyBatchChange(ctx, service.UpsertEmptyBatchChangeOpts{
		NamespaceUserID: uid,
		NamespaceOrgID:  oid,
		Name:            args.Name,
	})

	if err != nil {
		return nil, err
	}

	return &batchChangeResolver{store: r.store, gitserverClient: r.gitserverClient, batchChange: batchChange, logger: r.logger}, nil
}

func (r *Resolver) CreateBatchSpecFromRaw(ctx context.Context, args *graphqlbackend.CreateBatchSpecFromRawArgs) (_ graphqlbackend.BatchSpecResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.CreateBatchSpecFromRaw",
		attribute.String("namespace", string(args.Namespace)))
	defer tr.EndWithErr(&err)

	if err := batchChangesCreateAccess(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	svc := service.New(r.store)

	var uid, oid int32
	if err := graphqlbackend.UnmarshalNamespaceID(args.Namespace, &uid, &oid); err != nil {
		return nil, err
	}

	bid, err := unmarshalBatchChangeID(args.BatchChange)
	if err != nil {
		return nil, err
	}

	batchSpec, err := svc.CreateBatchSpecFromRaw(ctx, service.CreateBatchSpecFromRawOpts{
		NamespaceUserID:  uid,
		NamespaceOrgID:   oid,
		RawSpec:          args.BatchSpec,
		AllowIgnored:     args.AllowIgnored,
		AllowUnsupported: args.AllowUnsupported,
		NoCache:          args.NoCache,
		BatchChange:      bid,
	})
	if err != nil {
		return nil, err
	}

	return &batchSpecResolver{store: r.store, logger: r.logger, batchSpec: batchSpec}, nil
}

func (r *Resolver) ExecuteBatchSpec(ctx context.Context, args *graphqlbackend.ExecuteBatchSpecArgs) (_ graphqlbackend.BatchSpecResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.ExecuteBatchSpec",
		attribute.String("batchSpec", string(args.BatchSpec)))
	defer tr.EndWithErr(&err)
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	batchSpecRandID, err := unmarshalBatchSpecID(args.BatchSpec)
	if err != nil {
		return nil, err
	}

	if batchSpecRandID == "" {
		return nil, ErrIDIsZero{}
	}

	// 🚨 SECURITY: ExecuteBatchSpec checks whether current user is authorized
	// and has access to namespace.
	// Right now we also only allow creating batch specs in a user-namespace,
	// so the check makes sure the current user is the creator of the batch
	// spec or an admin.
	svc := service.New(r.store)
	batchSpec, err := svc.ExecuteBatchSpec(ctx, service.ExecuteBatchSpecOpts{
		BatchSpecRandID: batchSpecRandID,
		// TODO: args not yet implemented: AutoApply
		NoCache: args.NoCache,
	})
	if err != nil {
		return nil, err
	}

	return &batchSpecResolver{store: r.store, logger: r.logger, batchSpec: batchSpec}, nil
}

func (r *Resolver) CancelBatchSpecExecution(ctx context.Context, args *graphqlbackend.CancelBatchSpecExecutionArgs) (_ graphqlbackend.BatchSpecResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.CancelBatchSpecExecution",
		attribute.String("batchSpec", string(args.BatchSpec)))
	defer tr.EndWithErr(&err)

	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	batchSpecRandID, err := unmarshalBatchSpecID(args.BatchSpec)
	if err != nil {
		return nil, err
	}

	if batchSpecRandID == "" {
		return nil, ErrIDIsZero{}
	}

	svc := service.New(r.store)
	batchSpec, err := svc.CancelBatchSpec(ctx, service.CancelBatchSpecOpts{
		BatchSpecRandID: batchSpecRandID,
	})
	if err != nil {
		return nil, err
	}

	return &batchSpecResolver{store: r.store, logger: r.logger, batchSpec: batchSpec}, nil
}

func (r *Resolver) RetryBatchSpecWorkspaceExecution(ctx context.Context, args *graphqlbackend.RetryBatchSpecWorkspaceExecutionArgs) (_ *graphqlbackend.EmptyResponse, err error) {
	tr, ctx := trace.New(ctx, "Resolver.RetryBatchSpecWorkspaceExecution",
		attribute.String("workspaces", fmt.Sprintf("%+v", args.BatchSpecWorkspaces)))
	defer tr.EndWithErr(&err)

	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	var workspaceIDs []int64
	for _, raw := range args.BatchSpecWorkspaces {
		id, err := unmarshalBatchSpecWorkspaceID(raw)
		if err != nil {
			return nil, err
		}

		if id == 0 {
			return nil, ErrIDIsZero{}
		}

		workspaceIDs = append(workspaceIDs, id)
	}

	// 🚨 SECURITY: RetryBatchSpecWorkspaces checks whether current user is authorized
	// and has access to namespace.
	// Right now we also only allow creating batch specs in a user-namespace,
	// so the check makes sure the current user is the creator of the batch
	// spec or an admin.
	svc := service.New(r.store)
	err = svc.RetryBatchSpecWorkspaces(ctx, workspaceIDs)
	if err != nil {
		return nil, err
	}

	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *Resolver) ReplaceBatchSpecInput(ctx context.Context, args *graphqlbackend.ReplaceBatchSpecInputArgs) (_ graphqlbackend.BatchSpecResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.ReplaceBatchSpecInput",
		attribute.String("batchSpec", string(args.BatchSpec)))
	defer tr.EndWithErr(&err)

	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	batchSpecRandID, err := unmarshalBatchSpecID(args.PreviousSpec)
	if err != nil {
		return nil, err
	}

	if batchSpecRandID == "" {
		return nil, ErrIDIsZero{}
	}

	// 🚨 SECURITY: ReplaceBatchSpecInput checks whether current user is authorized
	// and has access to namespace.
	// Right now we also only allow creating batch specs in a user-namespace,
	// so the check makes sure the current user is the creator of the batch
	// spec or an admin.
	svc := service.New(r.store)
	batchSpec, err := svc.ReplaceBatchSpecInput(ctx, service.ReplaceBatchSpecInputOpts{
		BatchSpecRandID:  batchSpecRandID,
		RawSpec:          args.BatchSpec,
		AllowIgnored:     args.AllowIgnored,
		AllowUnsupported: args.AllowUnsupported,
		NoCache:          args.NoCache,
	})
	if err != nil {
		return nil, err
	}

	return &batchSpecResolver{store: r.store, logger: r.logger, batchSpec: batchSpec}, nil
}

func (r *Resolver) UpsertBatchSpecInput(ctx context.Context, args *graphqlbackend.UpsertBatchSpecInputArgs) (_ graphqlbackend.BatchSpecResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.UpsertBatchSpecInput", attribute.String("batchSpec", string(args.BatchSpec)))
	defer tr.EndWithErr(&err)

	if err := batchChangesCreateAccess(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	svc := service.New(r.store)

	var uid, oid int32
	if err := graphqlbackend.UnmarshalNamespaceID(args.Namespace, &uid, &oid); err != nil {
		return nil, err
	}

	// 🚨 SECURITY: UpsertBatchSpecInput checks whether current user is
	// authorised and has access to the namespace.
	//
	// Right now we also only allow creating batch specs in a user namespace, so
	// the check makes sure the current user is the creator of the batch spec or
	// an admin.
	batchSpec, err := svc.UpsertBatchSpecInput(ctx, service.UpsertBatchSpecInputOpts{
		NamespaceUserID:  uid,
		NamespaceOrgID:   oid,
		RawSpec:          args.BatchSpec,
		AllowIgnored:     args.AllowIgnored,
		AllowUnsupported: args.AllowUnsupported,
		NoCache:          args.NoCache,
	})
	if err != nil {
		return nil, err
	}

	return &batchSpecResolver{store: r.store, logger: r.logger, batchSpec: batchSpec}, nil
}

func (r *Resolver) CancelBatchSpecWorkspaceExecution(ctx context.Context, args *graphqlbackend.CancelBatchSpecWorkspaceExecutionArgs) (*graphqlbackend.EmptyResponse, error) {
	// TODO(ssbc): currently admin only.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}
	// TODO(ssbc): not implemented
	return nil, errors.New("not implemented yet")
}

func (r *Resolver) RetryBatchSpecExecution(ctx context.Context, args *graphqlbackend.RetryBatchSpecExecutionArgs) (_ graphqlbackend.BatchSpecResolver, err error) {
	tr, ctx := trace.New(ctx, "Resolver.RetryBatchSpecExecution", attribute.String("batchSpec", string(args.BatchSpec)))
	defer tr.EndWithErr(&err)

	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}

	batchSpecRandID, err := unmarshalBatchSpecID(args.BatchSpec)
	if err != nil {
		return nil, err
	}

	if batchSpecRandID == "" {
		return nil, ErrIDIsZero{}
	}

	// 🚨 SECURITY: RetryBatchSpecExecution checks whether current user is authorized
	// and has access to namespace.
	svc := service.New(r.store)
	if err = svc.RetryBatchSpecExecution(ctx, service.RetryBatchSpecExecutionOpts{
		BatchSpecRandID:  batchSpecRandID,
		IncludeCompleted: args.IncludeCompleted,
	}); err != nil {
		return nil, err
	}

	return r.batchSpecByID(ctx, args.BatchSpec)
}

func (r *Resolver) EnqueueBatchSpecWorkspaceExecution(ctx context.Context, args *graphqlbackend.EnqueueBatchSpecWorkspaceExecutionArgs) (*graphqlbackend.EmptyResponse, error) {
	// TODO(ssbc): currently admin only.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}
	// TODO(ssbc): not implemented
	return nil, errors.New("not implemented yet")
}

func (r *Resolver) ToggleBatchSpecAutoApply(ctx context.Context, args *graphqlbackend.ToggleBatchSpecAutoApplyArgs) (graphqlbackend.BatchSpecResolver, error) {
	// TODO(ssbc): currently admin only.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}
	// TODO(ssbc): not implemented
	return nil, errors.New("not implemented yet")
}

func (r *Resolver) DeleteBatchSpec(ctx context.Context, args *graphqlbackend.DeleteBatchSpecArgs) (*graphqlbackend.EmptyResponse, error) {
	// TODO(ssbc): currently admin only.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if err := rbac.CheckCurrentUserHasPermission(ctx, r.store.DatabaseDB(), rbac.BatchChangesWritePermission); err != nil {
		return nil, err
	}
	// TODO(ssbc): not implemented
	return nil, errors.New("not implemented yet")
}

func (r *Resolver) batchSpecWorkspaceFileByID(ctx context.Context, gqlID graphql.ID) (_ graphqlbackend.BatchWorkspaceFileResolver, err error) {
	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	batchWorkspaceFileRandID, err := unmarshalWorkspaceFileRandID(gqlID)
	if err != nil {
		return nil, err
	}

	if batchWorkspaceFileRandID == "" {
		return nil, ErrIDIsZero{}
	}

	file, err := r.store.GetBatchSpecWorkspaceFile(ctx, store.GetBatchSpecWorkspaceFileOpts{RandID: batchWorkspaceFileRandID})
	if err != nil {
		if err == store.ErrNoResults {
			return nil, nil
		}
		return nil, err
	}

	spec, err := r.store.GetBatchSpec(ctx, store.GetBatchSpecOpts{ID: file.BatchSpecID})
	if err != nil {
		if err == store.ErrNoResults {
			return nil, nil
		}
		return nil, err
	}

	return newBatchSpecWorkspaceFileResolver(spec.RandID, file), nil
}

func (r *Resolver) AvailableBulkOperations(ctx context.Context, args *graphqlbackend.AvailableBulkOperationsArgs) (availableBulkOperations []string, err error) {
	tr, ctx := trace.New(ctx, "Resolver.AvailableBulkOperations",
		attribute.String("batchChange", string(args.BatchChange)),
		attribute.Int("changesets.len", len(args.Changesets)))
	defer tr.EndWithErr(&err)

	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if len(args.Changesets) == 0 {
		return nil, errors.New("no changesets provided")
	}

	batchChangeID, changesetIDs, err := unmarshalBulkOperationBaseArgs(args.BulkOperationBaseArgs)
	if err != nil {
		return nil, err
	}

	svc := service.New(r.store)
	availableBulkOperations, err = svc.GetAvailableBulkOperations(ctx, service.GetAvailableBulkOperationsOpts{
		BatchChange: batchChangeID,
		Changesets:  changesetIDs,
	})

	if err != nil {
		return nil, err
	}

	return availableBulkOperations, nil
}

func (r *Resolver) CheckBatchChangesCredential(ctx context.Context, args *graphqlbackend.CheckBatchChangesCredentialArgs) (_ *graphqlbackend.EmptyResponse, err error) {
	tr, ctx := trace.New(ctx, "Resolver.CheckBatchChangesCredential",
		attribute.String("credential", string(args.BatchChangesCredential)))
	defer tr.EndWithErr(&err)

	cred, err := r.batchChangesCredentialByID(ctx, args.BatchChangesCredential)
	if err != nil {
		return nil, err
	}
	if cred == nil {
		return nil, ErrIDIsZero{}
	}

	validateArgs := service.ValidateAuthenticatorArgs{
		ExternalServiceID:   cred.ExternalServiceURL(),
		ExternalServiceType: extsvc.KindToType(cred.ExternalServiceKind()),
	}
	as := sources.AuthenticationStrategyUserCredential
	if cred.IsGitHubApp() {
		as = sources.AuthenticationStrategyGitHubApp

		ghApp, err := r.db.GitHubApps().GetByID(ctx, cred.GitHubAppID())
		if err != nil {
			return nil, err
		}

		if ghApp == nil {
			return nil, ghstore.ErrNoGitHubAppFound{}
		}

		installs, err := r.db.GitHubApps().GetInstallations(ctx, ghApp.ID)
		if err != nil {
			return nil, err
		}

		if len(installs) == 0 {
			return nil, ghstore.ErrNoGitHubAppFound{}
		}

		validateArgs.GitHubAppKind = &ghApp.Kind
		validateArgs.Username = pointers.Ptr(installs[0].AccountLogin)
	}

	a, err := cred.authenticator(ctx)
	if err != nil {
		return nil, err
	}

	fmt.Println("cred auth", a == nil)

	svc := service.New(r.store)
	if err := svc.ValidateAuthenticator(ctx, a, as, validateArgs); err != nil {
		return nil, &service.ErrVerifyCredentialFailed{SourceErr: err}
	}

	return &graphqlbackend.EmptyResponse{}, nil
}

// Realistically, we don't care about this field if an instance _is_ licensed. However, at
// present there's no way to directly query the license details over GraphQL, so we just
// return an arbitrarily high number if an instance is licensed and unrestricted.
func (r *Resolver) MaxUnlicensedChangesets(ctx context.Context) int32 {
	if bcFeature, err := checkLicense(); err == nil {
		if bcFeature.Unrestricted {
			return 999999
		} else {
			return int32(bcFeature.MaxNumChangesets)
		}
	} else {
		// The license could not be checked.
		return 0
	}
}

func (r *Resolver) GetChangesetsByIDs(ctx context.Context, args *graphqlbackend.GetChangesetsByIDsArgs) (_ graphqlutil.SliceConnectionResolver[graphqlbackend.ChangesetResolver], err error) {
	tr, ctx := trace.New(ctx, "Resolver.GetChangesetsByIDs",
		attribute.String("batchChange", string(args.BatchChange)),
		attribute.Int("changesets.len", len(args.Changesets)))
	defer tr.EndWithErr(&err)

	if err := enterprise.BatchChangesEnabledForUser(ctx, r.store.DatabaseDB()); err != nil {
		return nil, err
	}

	if len(args.Changesets) == 0 {
		return nil, errors.New("no changesets provided")
	}

	batchChangeID, changesetIDs, err := unmarshalBulkOperationBaseArgs(args.BulkOperationBaseArgs)
	if err != nil {
		return nil, err
	}

	svc := service.New(r.store)
	changesets, err := svc.GetChangesetsByIDs(ctx, batchChangeID, changesetIDs)
	if err != nil {
		return nil, err
	}

	// We store a map of repos to avoid duplicate DB queries to fetch a changeset's repository.
	// This is simply a lazy cache for this query.
	reposMap := map[api.RepoID]*types.Repo{}

	cs := make([]graphqlbackend.ChangesetResolver, len(changesets))
	for i, changeset := range changesets {
		repo, ok := reposMap[changeset.RepoID]
		if !ok {
			repo, err = r.store.Repos().Get(ctx, changeset.RepoID)
			if err != nil {
				return nil, err
			}
			reposMap[repo.ID] = repo
		}

		cs[i] = NewChangesetResolver(r.store, r.gitserverClient, r.logger, changeset, repo)
	}

	return graphqlutil.NewSliceConnectionResolver(cs, len(cs), len(cs)), nil
}

func parseBatchChangeStates(ss *[]string) ([]btypes.BatchChangeState, error) {
	states := []btypes.BatchChangeState{}
	if ss == nil || len(*ss) == 0 {
		return states, nil
	}
	for _, s := range *ss {
		state, err := parseBatchChangeState(&s)
		if err != nil {
			return nil, err
		}
		if state != "" {
			states = append(states, state)
		}
	}
	return states, nil
}

func parseBatchChangeState(s *string) (btypes.BatchChangeState, error) {
	if s == nil {
		return "", nil
	}
	switch *s {
	case "OPEN":
		return btypes.BatchChangeStateOpen, nil
	case "CLOSED":
		return btypes.BatchChangeStateClosed, nil
	case "DRAFT":
		return btypes.BatchChangeStateDraft, nil
	default:
		return "", errors.Errorf("unknown state %q", *s)
	}
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

func unmarshalBulkOperationBaseArgs(args graphqlbackend.BulkOperationBaseArgs) (batchChangeID int64, changesetIDs []int64, err error) {
	batchChangeID, err = unmarshalBatchChangeID(args.BatchChange)
	if err != nil {
		return 0, nil, err
	}

	if batchChangeID == 0 {
		return 0, nil, ErrIDIsZero{}
	}

	for _, raw := range args.Changesets {
		id, err := unmarshalChangesetID(raw)
		if err != nil {
			return 0, nil, err
		}

		if id == 0 {
			return 0, nil, ErrIDIsZero{}
		}

		changesetIDs = append(changesetIDs, id)
	}

	if len(changesetIDs) == 0 {
		return 0, nil, errors.New("specify at least one changeset")
	}

	return batchChangeID, changesetIDs, nil
}
