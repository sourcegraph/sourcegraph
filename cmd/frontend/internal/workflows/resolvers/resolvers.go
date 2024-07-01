package resolvers

import (
	"context"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// NewResolver returns a new Resolver that uses the given database.
func NewResolver(logger log.Logger, db database.DB) graphqlbackend.WorkflowsResolver {
	return &Resolver{logger: logger, db: db}
}

type Resolver struct {
	logger log.Logger
	db     database.DB
}

func (r *Resolver) Now() time.Time {
	return r.db.CodeMonitors().Now()
}

const WorkflowKind = "Workflow"

func (r *Resolver) NodeResolvers() map[string]graphqlbackend.NodeByIDFunc {
	return map[string]graphqlbackend.NodeByIDFunc{
		WorkflowKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
			return r.WorkflowByID(ctx, id)
		},
	}
}

type workflowResolver struct {
	db database.DB
	s  types.Workflow
}

func marshalWorkflowID(workflowID int32) graphql.ID {
	return relay.MarshalID(WorkflowKind, workflowID)
}

func unmarshalWorkflowID(id graphql.ID) (workflowID int32, err error) {
	err = relay.UnmarshalSpec(id, &workflowID)
	return
}

func (r *Resolver) WorkflowByID(ctx context.Context, id graphql.ID) (graphqlbackend.WorkflowResolver, error) {
	intID, err := unmarshalWorkflowID(id)
	if err != nil {
		return nil, err
	}

	ss, err := r.db.Workflows().GetByID(ctx, intID)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Make sure the current user has permission to get the workflow.
	if err := graphqlbackend.CheckAuthorizedForNamespaceByIDs(ctx, r.db, ss.Owner); err != nil {
		return nil, err
	}

	workflow := &workflowResolver{
		db: r.db,
		s:  *ss,
	}
	return workflow, nil
}

func (r *workflowResolver) ID() graphql.ID {
	return marshalWorkflowID(r.s.ID)
}

func (r *workflowResolver) Name() string { return r.s.Name }

func (r *workflowResolver) Description() string { return r.s.Description }

func (r *workflowResolver) Template() graphqlbackend.WorkflowPromptTemplateResolver {
	return graphqlbackend.WorkflowPromptTemplateResolver{Text_: r.s.TemplateText}
}

func (r *workflowResolver) Draft() bool { return r.s.Draft }

func (r *workflowResolver) Owner(ctx context.Context) (*graphqlbackend.NamespaceResolver, error) {
	if r.s.Owner.User != nil {
		n, err := graphqlbackend.NamespaceByID(ctx, r.db, graphqlbackend.MarshalUserID(*r.s.Owner.User))
		if err != nil {
			return nil, err
		}
		return &graphqlbackend.NamespaceResolver{Namespace: n}, nil
	}
	if r.s.Owner.Org != nil {
		n, err := graphqlbackend.NamespaceByID(ctx, r.db, graphqlbackend.MarshalOrgID(*r.s.Owner.Org))
		if err != nil {
			return nil, err
		}
		return &graphqlbackend.NamespaceResolver{Namespace: n}, nil
	}
	return nil, errors.New("no owner")
}

func (r *workflowResolver) NameWithOwner(ctx context.Context) (string, error) {
	owner, err := r.Owner(ctx)
	if err != nil {
		return "", err
	}
	return owner.NamespaceName() + "/" + r.s.Name, nil
}

func (r *workflowResolver) CreatedBy(ctx context.Context) (*graphqlbackend.UserResolver, error) {
	if userID := r.s.CreatedByUser; userID != nil {
		return graphqlbackend.UserByIDInt32(ctx, r.db, *userID)
	}
	return nil, nil
}

func (r *workflowResolver) CreatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.s.CreatedAt}
}

func (r *workflowResolver) UpdatedBy(ctx context.Context) (*graphqlbackend.UserResolver, error) {
	if userID := r.s.UpdatedByUser; userID != nil {
		return graphqlbackend.UserByIDInt32(ctx, r.db, *userID)
	}
	return nil, nil
}

func (r *workflowResolver) UpdatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.s.UpdatedAt}
}

func (r *workflowResolver) ViewerCanAdminister(ctx context.Context) (bool, error) {
	// Right now, any user who can see a workflow can edit/administer it, but in the future we can
	// add more access levels.
	return true, nil
}

func (r *Resolver) toWorkflowResolver(entry types.Workflow) *workflowResolver {
	return &workflowResolver{db: r.db, s: entry}
}

func (r *Resolver) Workflows(ctx context.Context, args graphqlbackend.WorkflowsArgs) (*graphqlbackend.WorkflowConnectionResolver, error) {
	connectionStore := &workflowsConnectionStore{db: r.db}

	if args.Query != nil {
		connectionStore.listArgs.Query = *args.Query
	}
	connectionStore.listArgs.HideDrafts = !args.IncludeDrafts

	if args.Owner != nil {
		// 🚨 SECURITY: Make sure the current user has permission to view workflows of the specified
		// owner.
		owner, err := graphqlbackend.CheckAuthorizedForNamespace(ctx, r.db, *args.Owner)
		if err != nil {
			return nil, err
		}
		connectionStore.listArgs.Owner = owner
	}

	if args.ViewerIsAffiliated != nil && *args.ViewerIsAffiliated {
		currentUser, err := auth.CurrentUser(ctx, r.db)
		if err != nil {
			return nil, err
		}
		connectionStore.listArgs.AffiliatedUser = &currentUser.ID
	}

	// 🚨 SECURITY: Only site admins can list workflows owned by other users or orgs that they are
	// not a member of.
	if connectionStore.listArgs.Owner == nil && connectionStore.listArgs.AffiliatedUser == nil {
		if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
			return nil, errors.Wrap(err, "must specify owner or viewerIsAffiliated args")
		}
	}

	return graphqlutil.NewConnectionResolver[graphqlbackend.WorkflowResolver](connectionStore, &args.ConnectionResolverArgs, nil)
}

type workflowsConnectionStore struct {
	db       database.DB
	listArgs database.WorkflowListArgs
}

func (s *workflowsConnectionStore) MarshalCursor(node graphqlbackend.WorkflowResolver, _ database.OrderBy) (*string, error) {
	cursor := string(node.ID())

	return &cursor, nil
}

func (s *workflowsConnectionStore) UnmarshalCursor(cursor string, _ database.OrderBy) ([]any, error) {
	nodeID, err := unmarshalWorkflowID(graphql.ID(cursor))
	if err != nil {
		return nil, err
	}

	return []any{nodeID}, nil
}

func (s *workflowsConnectionStore) ComputeTotal(ctx context.Context) (int32, error) {
	count, err := s.db.Workflows().Count(ctx, s.listArgs)
	return int32(count), err
}

func (s *workflowsConnectionStore) ComputeNodes(ctx context.Context, pgArgs *database.PaginationArgs) ([]graphqlbackend.WorkflowResolver, error) {
	dbResults, err := s.db.Workflows().List(ctx, s.listArgs, pgArgs)
	if err != nil {
		return nil, err
	}

	var results []graphqlbackend.WorkflowResolver
	for _, workflow := range dbResults {
		results = append(results, &workflowResolver{db: s.db, s: *workflow})
	}

	return results, nil
}

func (r *Resolver) CreateWorkflow(ctx context.Context, args *graphqlbackend.CreateWorkflowArgs) (graphqlbackend.WorkflowResolver, error) {
	// 🚨 SECURITY: Make sure the current user has permission to create a workflow in the
	// specified owner namespace.
	namespace, err := graphqlbackend.CheckAuthorizedForNamespace(ctx, r.db, args.Input.Owner)
	if err != nil {
		return nil, err
	}

	ss, err := r.db.Workflows().Create(ctx, &types.Workflow{
		Name:         args.Input.Name,
		Description:  args.Input.Description,
		TemplateText: args.Input.TemplateText,
		Draft:        args.Input.Draft,
		Owner:        *namespace,
	}, actor.FromContext(ctx).UID)
	if err != nil {
		return nil, err
	}

	return r.toWorkflowResolver(*ss), nil
}

func (r *Resolver) UpdateWorkflow(ctx context.Context, args *graphqlbackend.UpdateWorkflowArgs) (graphqlbackend.WorkflowResolver, error) {
	id, err := unmarshalWorkflowID(args.ID)
	if err != nil {
		return nil, err
	}

	old, err := r.db.Workflows().GetByID(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, "get existing workflow")
	}

	// 🚨 SECURITY: Make sure the current user has permission to update a workflow for the
	// specified owner namespace.
	if err := graphqlbackend.CheckAuthorizedForNamespaceByIDs(ctx, r.db, old.Owner); err != nil {
		return nil, err
	}

	ss, err := r.db.Workflows().Update(ctx, &types.Workflow{
		ID:           id,
		Name:         args.Input.Name,
		Description:  args.Input.Description,
		TemplateText: args.Input.TemplateText,
		Draft:        args.Input.Draft,
		Owner:        old.Owner,
	}, actor.FromContext(ctx).UID)
	if err != nil {
		return nil, err
	}

	return r.toWorkflowResolver(*ss), nil
}

func (r *Resolver) DeleteWorkflow(ctx context.Context, args *graphqlbackend.DeleteWorkflowArgs) (*graphqlbackend.EmptyResponse, error) {
	id, err := unmarshalWorkflowID(args.ID)
	if err != nil {
		return nil, err
	}
	ss, err := r.db.Workflows().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Make sure the current user has permission to delete a workflow for the
	// specified owner namespace.
	if err := graphqlbackend.CheckAuthorizedForNamespaceByIDs(ctx, r.db, ss.Owner); err != nil {
		return nil, err
	}

	if err := r.db.Workflows().Delete(ctx, id); err != nil {
		return nil, err
	}
	return &graphqlbackend.EmptyResponse{}, nil
}
