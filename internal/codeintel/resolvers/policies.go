package resolvers

import (
	"context"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
)

type PoliciesServiceResolver interface {
	// Fetch policies
	CodeIntelligenceConfigurationPolicies(ctx context.Context, args *CodeIntelligenceConfigurationPoliciesArgs) (CodeIntelligenceConfigurationPolicyConnectionResolver, error)
	ConfigurationPolicyByID(ctx context.Context, id graphql.ID) (CodeIntelligenceConfigurationPolicyResolver, error)

	// Modify policies
	CreateCodeIntelligenceConfigurationPolicy(ctx context.Context, args *CreateCodeIntelligenceConfigurationPolicyArgs) (CodeIntelligenceConfigurationPolicyResolver, error)
	UpdateCodeIntelligenceConfigurationPolicy(ctx context.Context, args *UpdateCodeIntelligenceConfigurationPolicyArgs) (*EmptyResponse, error)
	DeleteCodeIntelligenceConfigurationPolicy(ctx context.Context, args *DeleteCodeIntelligenceConfigurationPolicyArgs) (*EmptyResponse, error)

	// Filter previews
	PreviewRepositoryFilter(ctx context.Context, args *PreviewRepositoryFilterArgs) (RepositoryFilterPreviewResolver, error)
	PreviewGitObjectFilter(ctx context.Context, id graphql.ID, args *PreviewGitObjectFilterArgs) (GitObjectFilterPreviewResolver, error)
}

type CodeIntelligenceConfigurationPoliciesArgs struct {
	PagedConnectionArgs
	Repository       *graphql.ID
	Query            *string
	ForDataRetention *bool
	ForIndexing      *bool
	Protected        *bool
}

type CreateCodeIntelligenceConfigurationPolicyArgs struct {
	Repository *graphql.ID
	CodeIntelConfigurationPolicy
}

type CodeIntelConfigurationPolicy struct {
	Name                      string
	RepositoryID              *int32
	RepositoryPatterns        *[]string
	Type                      GitObjectType
	Pattern                   string
	RetentionEnabled          bool
	RetentionDurationHours    *int32
	RetainIntermediateCommits bool
	IndexingEnabled           bool
	IndexCommitMaxAgeHours    *int32
	IndexIntermediateCommits  bool
}

type UpdateCodeIntelligenceConfigurationPolicyArgs struct {
	ID         graphql.ID
	Repository *graphql.ID
	CodeIntelConfigurationPolicy
}

type DeleteCodeIntelligenceConfigurationPolicyArgs struct {
	Policy graphql.ID
}

type PreviewRepositoryFilterArgs struct {
	ConnectionArgs
	Patterns []string
}

type PreviewGitObjectFilterArgs struct {
	ConnectionArgs
	Type                         GitObjectType
	Pattern                      string
	CountObjectsYoungerThanHours *int32
}

type (
	CodeIntelligenceConfigurationPolicyConnectionResolver = PagedConnectionWithTotalCountResolver[CodeIntelligenceConfigurationPolicyResolver]
)

type CodeIntelligenceConfigurationPolicyResolver interface {
	ID() graphql.ID
	Repository(ctx context.Context) (RepositoryResolver, error)
	RepositoryPatterns() *[]string
	Name() string
	Type() (GitObjectType, error)
	Pattern() string
	Protected() bool
	RetentionEnabled() bool
	RetentionDurationHours() *int32
	RetainIntermediateCommits() bool
	IndexingEnabled() bool
	IndexCommitMaxAgeHours() *int32
	IndexIntermediateCommits() bool
}

type RepositoryFilterPreviewResolver interface {
	Nodes() []RepositoryResolver
	TotalCount() int32
	Limit() *int32
	TotalMatches() int32
	MatchesAllRepos() bool
}

type GitObjectFilterPreviewResolver interface {
	Nodes() []CodeIntelGitObjectResolver
	TotalCount() int32
	TotalCountYoungerThanThreshold() *int32
}

type CodeIntelGitObjectResolver interface {
	Name() string
	Rev() string
	CommittedAt() gqlutil.DateTime
}

type GitObjectType string

func (GitObjectType) ImplementsGraphQLType(name string) bool { return name == "GitObjectType" }

const (
	GitObjectTypeCommit  GitObjectType = "GIT_COMMIT"
	GitObjectTypeTag     GitObjectType = "GIT_TAG"
	GitObjectTypeTree    GitObjectType = "GIT_TREE"
	GitObjectTypeBlob    GitObjectType = "GIT_BLOB"
	GitObjectTypeUnknown GitObjectType = "GIT_UNKNOWN"
)
