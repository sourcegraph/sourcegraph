package gitserver

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
)

// convertGRPCErrorToGitDomainError translates a GRPC error to a gitdomain error.
// If the error is not a GRPC error, it is returned as-is.
func convertGRPCErrorToGitDomainError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	// TODO: Is this needed? Seems like we handle context cancellation errors properly
	// in gRPC interceptors: https://sourcegraph.com/-/editor?remote_url=github.com%2Fsourcegraph%2Fsourcegraph&branch=es%2Fget-commit-rpc&file=internal%2Fgrpc%2Fcontextconv%2Fconversion.go&editor=VSCode&version=2.2.16&start_row=56&start_col=5&end_row=56&end_col=6
	if st.Code() == codes.Canceled {
		return context.Canceled
	}

	if st.Code() == codes.DeadlineExceeded {
		return context.DeadlineExceeded
	}

	for _, detail := range st.Details() {
		switch payload := detail.(type) {

		case *proto.ExecStatusPayload:
			return &CommandStatusError{
				Message:    st.Message(),
				Stderr:     payload.GetStderr(),
				StatusCode: payload.GetStatusCode(),
			}

		case *proto.RepoNotFoundPayload:
			return &gitdomain.RepoNotExistError{
				Repo:            api.RepoName(payload.GetRepo()),
				CloneInProgress: payload.GetCloneInProgress(),
				CloneProgress:   payload.GetCloneProgress(),
			}

		case *proto.RevisionNotFoundPayload:
			return &gitdomain.RevisionNotFoundError{
				Repo: api.RepoName(payload.GetRepo()),
				Spec: payload.GetSpec(),
			}
		}
	}

	return err
}

type CommandStatusError struct {
	Message    string
	StatusCode int32
	Stderr     string
}

func (c *CommandStatusError) Error() string {
	stderr := c.Stderr
	if len(stderr) > 100 {
		stderr = stderr[:100] + "... (truncated)"
	}
	if c.Message != "" {
		return fmt.Sprintf("%s (stderr: %q)", c.Message, stderr)
	}
	if c.StatusCode != 0 {
		return fmt.Sprintf("non-zero exit status: %d (stderr: %q)", c.StatusCode, stderr)
	}
	return stderr
}

// errorTranslatingClient is a convenience wrapper around a base proto.GitserverServiceClient that automatically
// converts well-known gRPC errors into our own error type.
type errorTranslatingClient struct {
	base proto.GitserverServiceClient
}

func (r *errorTranslatingClient) P4Exec(ctx context.Context, in *proto.P4ExecRequest, opts ...grpc.CallOption) (proto.GitserverService_P4ExecClient, error) {
	cc, err := r.base.P4Exec(ctx, in, opts...) //nolint:SA1019
	if err != nil {
		return nil, convertGRPCErrorToGitDomainError(err)
	}
	return &errorTranslatingP4ExecClient{cc}, nil
}

type errorTranslatingP4ExecClient struct {
	proto.GitserverService_P4ExecClient
}

func (r *errorTranslatingP4ExecClient) Recv() (*proto.P4ExecResponse, error) {
	res, err := r.GitserverService_P4ExecClient.Recv()
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) RepoDelete(ctx context.Context, in *proto.RepoDeleteRequest, opts ...grpc.CallOption) (*proto.RepoDeleteResponse, error) {
	res, err := r.base.RepoDelete(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) CreateCommitFromPatchBinary(ctx context.Context, opts ...grpc.CallOption) (proto.GitserverService_CreateCommitFromPatchBinaryClient, error) {
	cc, err := r.base.CreateCommitFromPatchBinary(ctx, opts...)
	if err != nil {
		return nil, convertGRPCErrorToGitDomainError(err)
	}
	return &errorTranslatingCreateCommitFromPatchBinaryClient{cc}, nil
}

type errorTranslatingCreateCommitFromPatchBinaryClient struct {
	proto.GitserverService_CreateCommitFromPatchBinaryClient
}

func (r *errorTranslatingCreateCommitFromPatchBinaryClient) Send(m *proto.CreateCommitFromPatchBinaryRequest) error {
	err := r.GitserverService_CreateCommitFromPatchBinaryClient.Send(m)
	return convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingCreateCommitFromPatchBinaryClient) CloseAndRecv() (*proto.CreateCommitFromPatchBinaryResponse, error) {
	res, err := r.GitserverService_CreateCommitFromPatchBinaryClient.CloseAndRecv()
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) Exec(ctx context.Context, in *proto.ExecRequest, opts ...grpc.CallOption) (proto.GitserverService_ExecClient, error) {
	cc, err := r.base.Exec(ctx, in, opts...)
	if err != nil {
		return nil, convertGRPCErrorToGitDomainError(err)
	}
	return &errorTranslatingExecClient{cc}, nil
}

type errorTranslatingExecClient struct {
	proto.GitserverService_ExecClient
}

func (r *errorTranslatingExecClient) Recv() (*proto.ExecResponse, error) {
	res, err := r.GitserverService_ExecClient.Recv()
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) BatchLog(ctx context.Context, in *proto.BatchLogRequest, opts ...grpc.CallOption) (*proto.BatchLogResponse, error) {
	res, err := r.base.BatchLog(ctx, in, opts...) //nolint:SA1019
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) DiskInfo(ctx context.Context, in *proto.DiskInfoRequest, opts ...grpc.CallOption) (*proto.DiskInfoResponse, error) {
	res, err := r.base.DiskInfo(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) GetObject(ctx context.Context, in *proto.GetObjectRequest, opts ...grpc.CallOption) (*proto.GetObjectResponse, error) {
	res, err := r.base.GetObject(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) IsRepoCloneable(ctx context.Context, in *proto.IsRepoCloneableRequest, opts ...grpc.CallOption) (*proto.IsRepoCloneableResponse, error) {
	res, err := r.base.IsRepoCloneable(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) ListGitolite(ctx context.Context, in *proto.ListGitoliteRequest, opts ...grpc.CallOption) (*proto.ListGitoliteResponse, error) {
	res, err := r.base.ListGitolite(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) Search(ctx context.Context, in *proto.SearchRequest, opts ...grpc.CallOption) (proto.GitserverService_SearchClient, error) {
	cc, err := r.base.Search(ctx, in, opts...)
	if err != nil {
		return nil, convertGRPCErrorToGitDomainError(err)
	}
	return &errorTranslatingSearchClient{cc}, nil
}

type errorTranslatingSearchClient struct {
	proto.GitserverService_SearchClient
}

func (r *errorTranslatingSearchClient) Recv() (*proto.SearchResponse, error) {
	res, err := r.GitserverService_SearchClient.Recv()
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) Archive(ctx context.Context, in *proto.ArchiveRequest, opts ...grpc.CallOption) (proto.GitserverService_ArchiveClient, error) {
	cc, err := r.base.Archive(ctx, in, opts...)
	if err != nil {
		return nil, convertGRPCErrorToGitDomainError(err)
	}
	return &errorTranslatingArchiveClient{cc}, nil
}

type errorTranslatingArchiveClient struct {
	proto.GitserverService_ArchiveClient
}

func (r *errorTranslatingArchiveClient) Recv() (*proto.ArchiveResponse, error) {
	res, err := r.GitserverService_ArchiveClient.Recv()
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) RepoClone(ctx context.Context, in *proto.RepoCloneRequest, opts ...grpc.CallOption) (*proto.RepoCloneResponse, error) {
	res, err := r.base.RepoClone(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) RepoCloneProgress(ctx context.Context, in *proto.RepoCloneProgressRequest, opts ...grpc.CallOption) (*proto.RepoCloneProgressResponse, error) {
	res, err := r.base.RepoCloneProgress(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) RepoUpdate(ctx context.Context, in *proto.RepoUpdateRequest, opts ...grpc.CallOption) (*proto.RepoUpdateResponse, error) {
	res, err := r.base.RepoUpdate(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) IsPerforcePathCloneable(ctx context.Context, in *proto.IsPerforcePathCloneableRequest, opts ...grpc.CallOption) (*proto.IsPerforcePathCloneableResponse, error) {
	res, err := r.base.IsPerforcePathCloneable(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) CheckPerforceCredentials(ctx context.Context, in *proto.CheckPerforceCredentialsRequest, opts ...grpc.CallOption) (*proto.CheckPerforceCredentialsResponse, error) {
	res, err := r.base.CheckPerforceCredentials(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) PerforceUsers(ctx context.Context, in *proto.PerforceUsersRequest, opts ...grpc.CallOption) (*proto.PerforceUsersResponse, error) {
	res, err := r.base.PerforceUsers(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) PerforceProtectsForUser(ctx context.Context, in *proto.PerforceProtectsForUserRequest, opts ...grpc.CallOption) (*proto.PerforceProtectsForUserResponse, error) {
	res, err := r.base.PerforceProtectsForUser(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) PerforceProtectsForDepot(ctx context.Context, in *proto.PerforceProtectsForDepotRequest, opts ...grpc.CallOption) (*proto.PerforceProtectsForDepotResponse, error) {
	res, err := r.base.PerforceProtectsForDepot(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) PerforceGroupMembers(ctx context.Context, in *proto.PerforceGroupMembersRequest, opts ...grpc.CallOption) (*proto.PerforceGroupMembersResponse, error) {
	res, err := r.base.PerforceGroupMembers(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) IsPerforceSuperUser(ctx context.Context, in *proto.IsPerforceSuperUserRequest, opts ...grpc.CallOption) (*proto.IsPerforceSuperUserResponse, error) {
	res, err := r.base.IsPerforceSuperUser(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) PerforceGetChangelist(ctx context.Context, in *proto.PerforceGetChangelistRequest, opts ...grpc.CallOption) (*proto.PerforceGetChangelistResponse, error) {
	res, err := r.base.PerforceGetChangelist(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) MergeBase(ctx context.Context, in *proto.MergeBaseRequest, opts ...grpc.CallOption) (*proto.MergeBaseResponse, error) {
	res, err := r.base.MergeBase(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) Blame(ctx context.Context, in *proto.BlameRequest, opts ...grpc.CallOption) (proto.GitserverService_BlameClient, error) {
	cc, err := r.base.Blame(ctx, in, opts...)
	if err != nil {
		return nil, convertGRPCErrorToGitDomainError(err)
	}
	return &errorTranslatingBlameClient{cc}, nil
}

type errorTranslatingBlameClient struct {
	proto.GitserverService_BlameClient
}

func (r *errorTranslatingBlameClient) Recv() (*proto.BlameResponse, error) {
	res, err := r.GitserverService_BlameClient.Recv()
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) DefaultBranch(ctx context.Context, in *proto.DefaultBranchRequest, opts ...grpc.CallOption) (*proto.DefaultBranchResponse, error) {
	res, err := r.base.DefaultBranch(ctx, in, opts...)
	return res, convertGRPCErrorToGitDomainError(err)
}

func (r *errorTranslatingClient) ReadFile(ctx context.Context, in *proto.ReadFileRequest, opts ...grpc.CallOption) (proto.GitserverService_ReadFileClient, error) {
	cc, err := r.base.ReadFile(ctx, in, opts...)
	if err != nil {
		return nil, convertGRPCErrorToGitDomainError(err)
	}
	return &errorTranslatingReadFileClient{cc}, nil
}

type errorTranslatingReadFileClient struct {
	proto.GitserverService_ReadFileClient
}

func (r *errorTranslatingReadFileClient) Recv() (*proto.ReadFileResponse, error) {
	res, err := r.GitserverService_ReadFileClient.Recv()
	return res, convertGRPCErrorToGitDomainError(err)
}

var _ proto.GitserverServiceClient = &errorTranslatingClient{}
