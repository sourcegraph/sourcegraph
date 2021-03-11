package resolvers

import (
	"context"
	"strconv"
	"sync"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/search"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/service"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
)

func marshalBatchSpecRandID(id string) graphql.ID {
	return relay.MarshalID("BatchSpec", id)
}

func unmarshalBatchSpecID(id graphql.ID) (batchSpecRandID string, err error) {
	err = relay.UnmarshalSpec(id, &batchSpecRandID)
	return
}

var _ graphqlbackend.BatchSpecResolver = &batchSpecResolver{}

type batchSpecResolver struct {
	store *store.Store

	batchSpec          *batches.BatchSpec
	preloadedNamespace *graphqlbackend.NamespaceResolver

	// We cache the namespace on the resolver, since it's accessed more than once.
	namespaceOnce sync.Once
	namespace     *graphqlbackend.NamespaceResolver
	namespaceErr  error

	// TODO(campaigns-deprecation): This should be removed once we remove campaigns completely
	shouldActAsCampaignSpec bool
}

func (r *batchSpecResolver) ActAsCampaignSpec() bool {
	return r.shouldActAsCampaignSpec
}

func (r *batchSpecResolver) ID() graphql.ID {
	// 🚨 SECURITY: This needs to be the RandID! We can't expose the
	// sequential, guessable ID.
	return marshalBatchSpecRandID(r.batchSpec.RandID)
}

func (r *batchSpecResolver) OriginalInput() (string, error) {
	return r.batchSpec.RawSpec, nil
}

func (r *batchSpecResolver) ParsedInput() (graphqlbackend.JSONValue, error) {
	return graphqlbackend.JSONValue{Value: r.batchSpec.Spec}, nil
}

func (r *batchSpecResolver) ChangesetSpecs(ctx context.Context, args *graphqlbackend.ChangesetSpecsConnectionArgs) (graphqlbackend.ChangesetSpecConnectionResolver, error) {
	opts := store.ListChangesetSpecsOpts{}
	if err := validateFirstParamDefaults(args.First); err != nil {
		return nil, err
	}
	opts.Limit = int(args.First)
	if args.After != nil {
		id, err := strconv.Atoi(*args.After)
		if err != nil {
			return nil, err
		}
		opts.Cursor = int64(id)
	}

	return &changesetSpecConnectionResolver{
		store:       r.store,
		opts:        opts,
		batchSpecID: r.batchSpec.ID,
	}, nil
}

func (r *batchSpecResolver) ApplyPreview(ctx context.Context, args *graphqlbackend.ChangesetApplyPreviewConnectionArgs) (graphqlbackend.ChangesetApplyPreviewConnectionResolver, error) {
	if err := validateFirstParamDefaults(args.First); err != nil {
		return nil, err
	}
	opts := store.GetRewirerMappingsOpts{
		LimitOffset: &database.LimitOffset{
			Limit: int(args.First),
		},
		CurrentState: args.CurrentState,
	}
	if args.After != nil {
		id, err := strconv.Atoi(*args.After)
		if err != nil {
			return nil, err
		}
		opts.LimitOffset.Offset = id
	}
	if args.Search != nil {
		var err error
		opts.TextSearch, err = search.ParseTextSearch(*args.Search)
		if err != nil {
			return nil, errors.Wrap(err, "parsing search")
		}
	}

	return &changesetApplyPreviewConnectionResolver{
		store:       r.store,
		opts:        opts,
		action:      args.Action,
		batchSpecID: r.batchSpec.ID,
	}, nil
}

func (r *batchSpecResolver) Description() graphqlbackend.BatchChangeDescriptionResolver {
	return &batchChangeDescriptionResolver{
		name:        r.batchSpec.Spec.Name,
		description: r.batchSpec.Spec.Description,
	}
}

func (r *batchSpecResolver) Creator(ctx context.Context) (*graphqlbackend.UserResolver, error) {
	user, err := graphqlbackend.UserByIDInt32(ctx, r.store.DB(), r.batchSpec.UserID)
	if errcode.IsNotFound(err) {
		return nil, nil
	}
	return user, err
}

func (r *batchSpecResolver) Namespace(ctx context.Context) (*graphqlbackend.NamespaceResolver, error) {
	return r.computeNamespace(ctx)
}

func (r *batchSpecResolver) computeNamespace(ctx context.Context) (*graphqlbackend.NamespaceResolver, error) {
	r.namespaceOnce.Do(func() {
		if r.preloadedNamespace != nil {
			r.namespace = r.preloadedNamespace
			return
		}
		var (
			err error
			n   = &graphqlbackend.NamespaceResolver{}
		)

		if r.batchSpec.NamespaceUserID != 0 {
			n.Namespace, err = graphqlbackend.UserByIDInt32(ctx, r.store.DB(), r.batchSpec.NamespaceUserID)
		} else {
			n.Namespace, err = graphqlbackend.OrgByIDInt32(ctx, r.store.DB(), r.batchSpec.NamespaceOrgID)
		}

		if errcode.IsNotFound(err) {
			r.namespace = nil
			r.namespaceErr = errors.New("namespace of batch spec has been deleted")
			return
		}

		r.namespace = n
		r.namespaceErr = err
	})
	return r.namespace, r.namespaceErr
}

func (r *batchSpecResolver) ApplyURL(ctx context.Context) (string, error) {
	n, err := r.computeNamespace(ctx)
	if err != nil {
		return "", err
	}
	return batchChangesApplyURL(n, r), nil
}

func (r *batchSpecResolver) CreatedAt() graphqlbackend.DateTime {
	return graphqlbackend.DateTime{Time: r.batchSpec.CreatedAt}
}

func (r *batchSpecResolver) ExpiresAt() *graphqlbackend.DateTime {
	return &graphqlbackend.DateTime{Time: r.batchSpec.ExpiresAt()}
}

func (r *batchSpecResolver) ViewerCanAdminister(ctx context.Context) (bool, error) {
	return checkSiteAdminOrSameUser(ctx, r.batchSpec.UserID)
}

type batchChangeDescriptionResolver struct {
	name, description string
}

func (r *batchChangeDescriptionResolver) Name() string {
	return r.name
}

func (r *batchChangeDescriptionResolver) Description() string {
	return r.description
}

func (r *batchSpecResolver) DiffStat(ctx context.Context) (*graphqlbackend.DiffStat, error) {
	specsConnection := &changesetSpecConnectionResolver{
		store:       r.store,
		batchSpecID: r.batchSpec.ID,
	}

	specs, err := specsConnection.Nodes(ctx)
	if err != nil {
		return nil, err
	}

	totalStat := &graphqlbackend.DiffStat{}
	for _, spec := range specs {
		// If we can't convert it, that means it's hidden from the user and we
		// can simply skip it.
		if _, ok := spec.ToVisibleChangesetSpec(); !ok {
			continue
		}

		resolver, ok := spec.(*changesetSpecResolver)
		if !ok {
			// This should never happen.
			continue
		}

		stat := resolver.changesetSpec.DiffStat()
		totalStat.AddStat(stat)
	}

	return totalStat, nil
}

// TODO(campaigns-deprecation): This should be removed once we remove campaigns completely.
func (r *batchSpecResolver) AppliesToCampaign(ctx context.Context) (graphqlbackend.BatchChangeResolver, error) {
	return r.AppliesToBatchChange(ctx)
}

func (r *batchSpecResolver) AppliesToBatchChange(ctx context.Context) (graphqlbackend.BatchChangeResolver, error) {
	svc := service.New(r.store)
	batchChange, err := svc.GetBatchChangeMatchingBatchSpec(ctx, r.batchSpec)
	if err != nil {
		return nil, err
	}
	if batchChange == nil {
		return nil, nil
	}

	return &batchChangeResolver{
		store:       r.store,
		batchChange: batchChange,
	}, nil
}

// TODO(campaigns-deprecation): This should be removed once we remove campaigns completely.
func (r *batchSpecResolver) SupersedingCampaignSpec(ctx context.Context) (graphqlbackend.BatchSpecResolver, error) {
	return r.SupersedingBatchSpec(ctx)
}

func (r *batchSpecResolver) SupersedingBatchSpec(ctx context.Context) (graphqlbackend.BatchSpecResolver, error) {
	namespace, err := r.computeNamespace(ctx)
	if err != nil {
		return nil, err
	}

	svc := service.New(r.store)
	newest, err := svc.GetNewestBatchSpec(ctx, r.store, r.batchSpec, actor.FromContext(ctx).UID)
	if err != nil {
		return nil, err
	}

	// If this is the newest spec, then we can just return nil.
	if newest == nil || newest.ID == r.batchSpec.ID {
		return nil, nil
	}

	// If this spec and the new spec have different creators, we shouldn't
	// return this as a superseding spec.
	if newest.UserID != r.batchSpec.UserID {
		return nil, nil
	}

	// Create our new resolver, reusing as many fields as we can from this one.
	resolver := &batchSpecResolver{
		store:              r.store,
		batchSpec:          newest,
		preloadedNamespace: namespace,
	}

	return resolver, nil
}

func (r *batchSpecResolver) ViewerBatchChangesCodeHosts(ctx context.Context, args *graphqlbackend.ListViewerBatchChangesCodeHostsArgs) (graphqlbackend.BatchChangesCodeHostConnectionResolver, error) {
	actor := actor.FromContext(ctx)
	if !actor.IsAuthenticated() {
		return nil, backend.ErrNotAuthenticated
	}

	// Short path for site-admins when OnlyWithoutCredential is true: It will always be an empty list.
	if args.OnlyWithoutCredential {
		if authErr := backend.CheckCurrentUserIsSiteAdmin(ctx); authErr == nil {
			// For site-admins never return anything
			return &emptyBatchChangesCodeHostConnectionResolver{}, nil
		} else if authErr != nil && authErr != backend.ErrMustBeSiteAdmin {
			return nil, authErr
		}
	}

	specs, _, err := r.store.ListChangesetSpecs(ctx, store.ListChangesetSpecsOpts{BatchSpecID: r.batchSpec.ID})
	if err != nil {
		return nil, err
	}

	offset := 0
	if args.After != nil {
		offset, err = strconv.Atoi(*args.After)
		if err != nil {
			return nil, err
		}
	}

	return &batchChangesCodeHostConnectionResolver{
		userID:                actor.UID,
		onlyWithoutCredential: args.OnlyWithoutCredential,
		store:                 r.store,
		opts: store.ListCodeHostsOpts{
			RepoIDs: specs.RepoIDs(),
		},
		limitOffset: database.LimitOffset{
			Limit:  int(args.First),
			Offset: offset,
		},
	}, nil
}
