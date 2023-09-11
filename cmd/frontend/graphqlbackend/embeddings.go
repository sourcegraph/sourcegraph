package graphqlbackend

import (
	"context"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
)

type EmbeddingsResolver interface {
	EmbeddingsSearch(ctx context.Context, args EmbeddingsSearchInputArgs) (EmbeddingsSearchResultsResolver, error)
	EmbeddingsMultiSearch(ctx context.Context, args EmbeddingsMultiSearchInputArgs) (EmbeddingsSearchResultsResolver, error)
	IsContextRequiredForChatQuery(ctx context.Context, args IsContextRequiredForChatQueryInputArgs) (bool, error)
	RepoEmbeddingJobs(ctx context.Context, args ListRepoEmbeddingJobsArgs) (*graphqlutil.ConnectionResolver[RepoEmbeddingJobResolver], error)

	RepoEmbeddingSize(ctx context.Context, args RepoEmbeddingsSizeArgs) (*RepoEmbeddingsSizeResponse, error)

	ScheduleRepositoriesForEmbedding(ctx context.Context, args ScheduleRepositoriesForEmbeddingArgs) (*EmptyResponse, error)
	CancelRepoEmbeddingJob(ctx context.Context, args CancelRepoEmbeddingJobArgs) (*EmptyResponse, error)
}

type ScheduleRepositoriesForEmbeddingArgs struct {
	RepoNames []string
	Force     *bool
}

type RepoEmbeddingsSizeArgs struct {
	RepoNames []string
}
type RepoEmbeddingsSize struct {
	Repo  string
	Bytes int32
}

func (r *RepoEmbeddingsSize) Name() string {
	return r.Repo
}

func (r *RepoEmbeddingsSize) SizeBytes() int32 {
	return r.Bytes
}

type RepoEmbeddingsSizeResponse struct {
	RepoSizeBytes []*RepoEmbeddingsSize
}

type IsContextRequiredForChatQueryInputArgs struct {
	Query string
}

type EmbeddingsSearchInputArgs struct {
	Repo             graphql.ID
	Query            string
	CodeResultsCount int32
	TextResultsCount int32
}

type EmbeddingsMultiSearchInputArgs struct {
	Repos            []graphql.ID
	Query            string
	CodeResultsCount int32
	TextResultsCount int32
}

type EmbeddingsSearchResultsResolver interface {
	CodeResults(ctx context.Context) ([]EmbeddingsSearchResultResolver, error)
	TextResults(ctx context.Context) ([]EmbeddingsSearchResultResolver, error)
}

type EmbeddingsSearchResultResolver interface {
	RepoName(ctx context.Context) string
	Revision(ctx context.Context) string
	FileName(ctx context.Context) string
	StartLine(ctx context.Context) int32
	EndLine(ctx context.Context) int32
	Content(ctx context.Context) string
}

type ListRepoEmbeddingJobsArgs struct {
	graphqlutil.ConnectionResolverArgs
	Query *string
	State *string
	Repo  *graphql.ID
}

type CancelRepoEmbeddingJobArgs struct {
	Job graphql.ID
}

type RepoEmbeddingJobResolver interface {
	ID() graphql.ID
	State() string
	FailureMessage() *string
	QueuedAt() gqlutil.DateTime
	StartedAt() *gqlutil.DateTime
	FinishedAt() *gqlutil.DateTime
	ProcessAfter() *gqlutil.DateTime
	NumResets() int32
	NumFailures() int32
	LastHeartbeatAt() *gqlutil.DateTime
	WorkerHostname() string
	Cancel() bool
	Repo(ctx context.Context) (*RepositoryResolver, error)
	Revision(ctx context.Context) (*GitCommitResolver, error)
	Stats(context.Context) (RepoEmbeddingJobStatsResolver, error)
}

type RepoEmbeddingJobStatsResolver interface {
	FilesEmbedded() int32
	FilesScheduled() int32
	FilesSkipped() int32
}

func (r *RepoEmbeddingsSizeResponse) Sizes() []*RepoEmbeddingsSize {
	return r.RepoSizeBytes
}
