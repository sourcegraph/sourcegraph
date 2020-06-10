package graphqlbackend

import (
	"context"
	"errors"

	graphql "github.com/graph-gophers/graphql-go"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/api"
)

type CodeIntelResolver interface {
	LSIFUploadByID(ctx context.Context, id graphql.ID) (LSIFUploadResolver, error)
	LSIFUploads(ctx context.Context, args *LSIFUploadsQueryArgs) (LSIFUploadConnectionResolver, error)
	LSIFUploadsByRepo(ctx context.Context, args *LSIFRepositoryUploadsQueryArgs) (LSIFUploadConnectionResolver, error)
	DeleteLSIFUpload(ctx context.Context, id graphql.ID) (*EmptyResponse, error)
	LSIFIndexByID(ctx context.Context, id graphql.ID) (LSIFIndexResolver, error)
	LSIFIndexes(ctx context.Context, args *LSIFIndexesQueryArgs) (LSIFIndexConnectionResolver, error)
	LSIFIndexesByRepo(ctx context.Context, args *LSIFRepositoryIndexesQueryArgs) (LSIFIndexConnectionResolver, error)
	DeleteLSIFIndex(ctx context.Context, id graphql.ID) (*EmptyResponse, error)
	GitBlobLSIFData(ctx context.Context, args *GitBlobLSIFDataArgs) (GitBlobLSIFDataResolver, error)
}

var codeIntelOnlyInEnterprise = errors.New("lsif uploads and queries are only available in enterprise")

type defaultCodeIntelResolver struct{}

var DefaultCodeIntelResolver CodeIntelResolver = defaultCodeIntelResolver{}

func (defaultCodeIntelResolver) LSIFUploadByID(ctx context.Context, id graphql.ID) (LSIFUploadResolver, error) {
	return nil, codeIntelOnlyInEnterprise
}

func (defaultCodeIntelResolver) LSIFUploads(ctx context.Context, args *LSIFUploadsQueryArgs) (LSIFUploadConnectionResolver, error) {
	return nil, codeIntelOnlyInEnterprise
}

func (defaultCodeIntelResolver) LSIFUploadsByRepo(ctx context.Context, args *LSIFRepositoryUploadsQueryArgs) (LSIFUploadConnectionResolver, error) {
	return nil, codeIntelOnlyInEnterprise
}

func (defaultCodeIntelResolver) DeleteLSIFUpload(ctx context.Context, id graphql.ID) (*EmptyResponse, error) {
	return nil, codeIntelOnlyInEnterprise
}

func (defaultCodeIntelResolver) LSIFIndexByID(ctx context.Context, id graphql.ID) (LSIFIndexResolver, error) {
	return nil, codeIntelOnlyInEnterprise
}

func (defaultCodeIntelResolver) LSIFIndexes(ctx context.Context, args *LSIFIndexesQueryArgs) (LSIFIndexConnectionResolver, error) {
	return nil, codeIntelOnlyInEnterprise
}

func (defaultCodeIntelResolver) LSIFIndexesByRepo(ctx context.Context, args *LSIFRepositoryIndexesQueryArgs) (LSIFIndexConnectionResolver, error) {
	return nil, codeIntelOnlyInEnterprise
}

func (defaultCodeIntelResolver) DeleteLSIFIndex(ctx context.Context, id graphql.ID) (*EmptyResponse, error) {
	return nil, codeIntelOnlyInEnterprise
}

func (defaultCodeIntelResolver) GitBlobLSIFData(ctx context.Context, args *GitBlobLSIFDataArgs) (GitBlobLSIFDataResolver, error) {
	return nil, codeIntelOnlyInEnterprise
}

func (r *schemaResolver) LSIFUploads(ctx context.Context, args *LSIFUploadsQueryArgs) (LSIFUploadConnectionResolver, error) {
	return r.CodeIntelResolver.LSIFUploads(ctx, args)
}

func (r *schemaResolver) LSIFIndexes(ctx context.Context, args *LSIFIndexesQueryArgs) (LSIFIndexConnectionResolver, error) {
	return r.CodeIntelResolver.LSIFIndexes(ctx, args)
}

func (r *schemaResolver) DeleteLSIFUpload(ctx context.Context, args *struct{ ID graphql.ID }) (*EmptyResponse, error) {
	return r.CodeIntelResolver.DeleteLSIFUpload(ctx, args.ID)
}

func (r *schemaResolver) DeleteLSIFIndex(ctx context.Context, args *struct{ ID graphql.ID }) (*EmptyResponse, error) {
	return r.CodeIntelResolver.DeleteLSIFIndex(ctx, args.ID)
}

type LSIFUploadsQueryArgs struct {
	graphqlutil.ConnectionArgs
	Query           *string
	State           *string
	IsLatestForRepo *bool
	After           *string
}

type LSIFRepositoryUploadsQueryArgs struct {
	*LSIFUploadsQueryArgs
	RepositoryID graphql.ID
}

type LSIFUploadResolver interface {
	ID() graphql.ID
	ProjectRoot(ctx context.Context) (*GitTreeEntryResolver, error)
	InputCommit() string
	InputRoot() string
	InputIndexer() string
	State() string
	UploadedAt() DateTime
	StartedAt() *DateTime
	FinishedAt() *DateTime
	Failure() *string
	IsLatestForRepo() bool
	PlaceInQueue() *int32
}

type LSIFUploadConnectionResolver interface {
	Nodes(ctx context.Context) ([]LSIFUploadResolver, error)
	TotalCount(ctx context.Context) (*int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

type LSIFIndexesQueryArgs struct {
	graphqlutil.ConnectionArgs
	Query *string
	State *string
	After *string
}

type LSIFRepositoryIndexesQueryArgs struct {
	*LSIFIndexesQueryArgs
	RepositoryID graphql.ID
}

type LSIFIndexResolver interface {
	ID() graphql.ID
	ProjectRoot(ctx context.Context) (*GitTreeEntryResolver, error)
	InputCommit() string
	State() string
	QueuedAt() DateTime
	StartedAt() *DateTime
	FinishedAt() *DateTime
	Failure() *string
	PlaceInQueue() *int32
}

type LSIFIndexConnectionResolver interface {
	Nodes(ctx context.Context) ([]LSIFIndexResolver, error)
	TotalCount(ctx context.Context) (*int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

type GitTreeLSIFDataResolver interface {
	Diagnostics(ctx context.Context, args *LSIFDiagnosticsArgs) (DiagnosticConnectionResolver, error)
}

type GitBlobLSIFDataResolver interface {
	GitTreeLSIFDataResolver
	ToGitTreeLSIFData() (GitTreeLSIFDataResolver, bool)
	ToGitBlobLSIFData() (GitBlobLSIFDataResolver, bool)

	Definitions(ctx context.Context, args *LSIFQueryPositionArgs) (LocationConnectionResolver, error)
	References(ctx context.Context, args *LSIFPagedQueryPositionArgs) (LocationConnectionResolver, error)
	Hover(ctx context.Context, args *LSIFQueryPositionArgs) (HoverResolver, error)
}

type GitBlobLSIFDataArgs struct {
	Repository *RepositoryResolver
	Commit     api.CommitID
	Path       string
	ExactPath  bool
	ToolName   string
	UploadID   int64
}

type LSIFQueryPositionArgs struct {
	Line      int32
	Character int32
}

type LSIFPagedQueryPositionArgs struct {
	LSIFQueryPositionArgs
	graphqlutil.ConnectionArgs
	After *string
}

type LSIFDiagnosticsArgs struct {
	graphqlutil.ConnectionArgs
}

type LocationConnectionResolver interface {
	Nodes(ctx context.Context) ([]LocationResolver, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

type HoverResolver interface {
	Markdown() MarkdownResolver
	Range() RangeResolver
}

type DiagnosticConnectionResolver interface {
	Nodes(ctx context.Context) ([]DiagnosticResolver, error)
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

type DiagnosticResolver interface {
	Location(ctx context.Context) (LocationResolver, error)
	Severity(ctx context.Context) (*string, error)
	Code(ctx context.Context) (*string, error)
	Source(ctx context.Context) (*string, error)
	Message(ctx context.Context) (*string, error)
}
