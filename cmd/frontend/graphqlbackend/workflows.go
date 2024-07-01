package graphqlbackend

import (
	"context"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
)

func (s *schemaResolver) Workflows(ctx context.Context, args WorkflowsArgs) (*WorkflowConnectionResolver, error) {
	return s.WorkflowsResolver.Workflows(ctx, args)
}

type WorkflowsResolver interface {
	// Query
	Workflows(ctx context.Context, args WorkflowsArgs) (*WorkflowConnectionResolver, error)
	WorkflowByID(ctx context.Context, id graphql.ID) (WorkflowResolver, error)

	// Mutations
	CreateWorkflow(ctx context.Context, args *CreateWorkflowArgs) (WorkflowResolver, error)
	UpdateWorkflow(ctx context.Context, args *UpdateWorkflowArgs) (WorkflowResolver, error)
	DeleteWorkflow(ctx context.Context, args *DeleteWorkflowArgs) (*EmptyResponse, error)

	NodeResolvers() map[string]NodeByIDFunc
}

type WorkflowConnectionResolver = graphqlutil.ConnectionResolver[WorkflowResolver]

type WorkflowResolver interface {
	ID() graphql.ID
	Name() string
	Description() string
	Template() WorkflowPromptTemplateResolver
	Draft() bool
	Owner(context.Context) (*NamespaceResolver, error)
	NameWithOwner(context.Context) (string, error)
	CreatedBy(context.Context) (*UserResolver, error)
	CreatedAt() gqlutil.DateTime
	UpdatedBy(context.Context) (*UserResolver, error)
	UpdatedAt() gqlutil.DateTime
	ViewerCanAdminister(context.Context) (bool, error)
}

type WorkflowPromptTemplateResolver struct {
	Text_ string
}

func (r WorkflowPromptTemplateResolver) Text() string {
	return r.Text_
}

type WorkflowsArgs struct {
	graphqlutil.ConnectionResolverArgs
	Query              *string
	Owner              *graphql.ID
	ViewerIsAffiliated *bool
	IncludeDrafts      bool
}

type CreateWorkflowArgs struct {
	Input WorkflowInput
}

type WorkflowInput struct {
	Owner        graphql.ID
	Name         string
	Description  string
	TemplateText string
	Draft        bool
}

type UpdateWorkflowArgs struct {
	ID    graphql.ID
	Input WorkflowUpdateInput
}

type WorkflowUpdateInput struct {
	Name         string
	Description  string
	TemplateText string
	Draft        bool
}

type DeleteWorkflowArgs struct {
	ID graphql.ID
}
