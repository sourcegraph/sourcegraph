package internal

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/sourcegraph/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/urlredactor"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/internal/grpc/grpcutil"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
)

// loggingGRPCServer is a wrapper around the provided GitserverServiceServer
// that logs requests, durations, and status codes.
type loggingGRPCServer struct {
	base   proto.GitserverServiceServer
	logger log.Logger

	proto.UnsafeGitserverServiceServer // Consciously opt out of forward compatibility checks to ensure that the go-compiler will catch any breaking changes.
}

func doLog(logger log.Logger, fullMethod string, statusCode codes.Code, traceID string, duration time.Duration, requestFields ...log.Field) {
	server, method := grpcutil.SplitMethodName(fullMethod)

	fields := []log.Field{
		log.String("server", server),
		log.String("method", method),
		log.String("status", statusCode.String()),
		log.String("traceID", traceID),
		log.Duration("duration", duration),
	}

	if len(requestFields) > 0 {
		fields = append(fields, log.Object("request", requestFields...))
	} else {
		fields = append(fields, log.String("request", "<empty>"))
	}

	logger.Debug(fmt.Sprintf("Handled %s RPC", method), fields...)
}

func (l *loggingGRPCServer) CreateCommitFromPatchBinary(server proto.GitserverService_CreateCommitFromPatchBinaryServer) (err error) {
	start := time.Now()
	var atomicMetadata atomic.Pointer[proto.CreateCommitFromPatchBinaryRequest_Metadata]

	defer func() {
		elapsed := time.Since(start)

		var fields []log.Field
		meta := atomicMetadata.Load()
		if meta != nil {
			fields = createCommitFromPatchBinaryRequestMetadataToLogFields(meta)
		}

		doLog(
			l.logger,
			proto.GitserverService_CreateCommitFromPatchBinary_FullMethodName,
			status.Code(err),
			trace.Context(server.Context()).TraceID,
			elapsed,

			log.Object("metadata", fields...),
		)

	}()

	recvCallback := func(req *proto.CreateCommitFromPatchBinaryRequest, err error) {
		if err != nil {
			return
		}

		switch req.GetPayload().(type) {
		case *proto.CreateCommitFromPatchBinaryRequest_Metadata_:
			// Save the first metadata message that we receive for later logging
			meta := req.GetMetadata()
			atomicMetadata.CompareAndSwap(nil, meta)
		default:
			return
		}
	}

	s := newCreateCommitFromPatchBinaryCallbackServer(server, recvCallback)
	return l.base.CreateCommitFromPatchBinary(s)
}

func createCommitFromPatchBinaryRequestMetadataToLogFields(req *proto.CreateCommitFromPatchBinaryRequest_Metadata) []log.Field {

	return []log.Field{
		log.String("repo", req.GetRepo()),
		log.String("baseCommit", req.GetBaseCommit()),
		log.String("targetRef", req.GetTargetRef()),
		log.Bool("uniqueRef", req.GetUniqueRef()),
		log.Object("commitInfo", patchCommitInfoToLogFields(req.GetCommitInfo())...),
		log.Object("push", pushConfigToLogFields(req.GetPush())...),
		log.Strings("gitApplyArgs", req.GetGitApplyArgs()),
		log.String("pushRef", req.GetPushRef()),
	}
}

func patchCommitInfoToLogFields(req *proto.PatchCommitInfo) []log.Field {
	return []log.Field{
		log.Strings("messages", req.GetMessages()),
		log.String("authorName", req.GetAuthorName()),
		log.String("authorEmail", req.GetAuthorEmail()),
		log.String("committerName", req.GetCommitterName()),
		log.String("committerEmail", req.GetCommitterEmail()),
		log.Time("date", req.GetDate().AsTime()),
	}
}

func pushConfigToLogFields(req *proto.PushConfig) []log.Field {
	u, err := vcs.ParseURL(req.GetRemoteUrl())
	if err != nil {
		return []log.Field{
			log.String("remoteURL", "<unable-to-parse-and-redact>"),
		}
	}

	redactor := urlredactor.New(u)
	return []log.Field{
		log.String("remoteURL", redactor.Redact(req.GetRemoteUrl())),
	}

	// 🚨SECURITY: We don't log the privateKey field because it contains sensitive data.
	// 🚨SECURITY: We don't log the passphrase field because it contains sensitive data.
}

func (l *loggingGRPCServer) DiskInfo(ctx context.Context, request *proto.DiskInfoRequest) (response *proto.DiskInfoResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_DiskInfo_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,
		)
	}()

	return l.base.DiskInfo(ctx, request)
}

func (l *loggingGRPCServer) Exec(request *proto.ExecRequest, server proto.GitserverService_ExecServer) (err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_Exec_FullMethodName,
			status.Code(err),
			trace.Context(server.Context()).TraceID,
			elapsed,

			execRequestToLogFields(request)...)
	}()

	//lint:ignore SA1019 existing usage of deprecated functionality. We are just logging an existing field.
	return l.base.Exec(request, server)
}

func execRequestToLogFields(req *proto.ExecRequest) []log.Field {
	return []log.Field{
		//lint:ignore SA1019 existing usage of deprecated functionality. We are just logging an existing field.
		log.String("repo", req.GetRepo()),
		//lint:ignore SA1019 existing usage of deprecated functionality. We are just logging an existing field.
		log.String("ensureRevision", string(req.GetEnsureRevision())),
		//lint:ignore SA1019 existing usage of deprecated functionality. We are just logging an existing field.
		log.Strings("args", byteSlicesToStrings(req.GetArgs())),
		// 🚨SECURITY: We don't log the stdin field because it could 1) contain sensitive data 2) be very large.
	}
}

func (l *loggingGRPCServer) GetObject(ctx context.Context, request *proto.GetObjectRequest) (response *proto.GetObjectResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_GetObject_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			getObjectRequestToLogFields(request)...,
		)
	}()

	return l.base.GetObject(ctx, request)
}

func getObjectRequestToLogFields(req *proto.GetObjectRequest) []log.Field {
	return []log.Field{
		log.String("repo", req.GetRepo()),
		log.String("objectName", req.GetObjectName()),
	}
}

func (l *loggingGRPCServer) IsRepoCloneable(ctx context.Context, request *proto.IsRepoCloneableRequest) (response *proto.IsRepoCloneableResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_IsRepoCloneable_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			isRepoCloneableRequestToLogFields(request)...,
		)

	}()

	return l.base.IsRepoCloneable(ctx, request)
}

func isRepoCloneableRequestToLogFields(req *proto.IsRepoCloneableRequest) []log.Field {
	return []log.Field{
		log.String("repo", req.GetRepo()),
	}
}
func (l *loggingGRPCServer) ListGitolite(ctx context.Context, request *proto.ListGitoliteRequest) (response *proto.ListGitoliteResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_ListGitolite_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			listGitoliteRequestToLogFields(request)...,
		)
	}()

	return l.base.ListGitolite(ctx, request)
}

func listGitoliteRequestToLogFields(req *proto.ListGitoliteRequest) []log.Field {
	u, err := vcs.ParseURL(req.GetGitoliteHost())
	if err != nil {
		return []log.Field{
			log.String("gitoliteHost", "<unable-to-parse-and-redact>"),
		}
	}

	redactor := urlredactor.New(u)
	return []log.Field{
		log.String("gitoliteHost", redactor.Redact(req.GetGitoliteHost())),
	}
}

func (l *loggingGRPCServer) Search(request *proto.SearchRequest, server proto.GitserverService_SearchServer) (err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_Search_FullMethodName,
			status.Code(err),
			trace.Context(server.Context()).TraceID,
			elapsed,

			searchRequestToLogFields(request)...,
		)

	}()

	return l.base.Search(request, server)
}

func searchRequestToLogFields(req *proto.SearchRequest) []log.Field {
	revisionLogFields := make([]log.Field, 0, len(req.GetRevisions()))
	for i, rev := range req.GetRevisions() {
		revisionLogFields = append(revisionLogFields, log.Object(fmt.Sprintf("%d", i), revisionSpecifierToLogFields(rev)...))
	}

	return []log.Field{
		log.String("repo", req.GetRepo()),
		log.Object("revisions", revisionLogFields...),
		log.Int64("limit", req.GetLimit()),
		log.Bool("includeDiff", req.GetIncludeDiff()),
		log.Bool("includeModifiedFiles", req.GetIncludeModifiedFiles()),
		log.Object("query", queryNodeToLogFields(req.GetQuery())...),
	}
}

func revisionSpecifierToLogFields(r *proto.RevisionSpecifier) []log.Field {
	return []log.Field{
		log.String("revSpec", r.GetRevSpec()),
	}
}

func queryNodeToLogFields(p *proto.QueryNode) []log.Field {
	switch v := p.GetValue().(type) {
	case *proto.QueryNode_AuthorMatches:
		return []log.Field{
			log.Object("AuthorMatches",
				log.String("Expr", v.AuthorMatches.GetExpr()),
				log.Bool("IgnoreCase", v.AuthorMatches.GetIgnoreCase()),
			),
		}
	case *proto.QueryNode_CommitterMatches:
		return []log.Field{
			log.Object("CommitterMatches",
				log.String("Expr", v.CommitterMatches.GetExpr()),
				log.Bool("IgnoreCase", v.CommitterMatches.GetIgnoreCase()),
			),
		}
	case *proto.QueryNode_CommitBefore:
		return []log.Field{
			log.Object("CommitBefore",
				log.Time("Time", v.CommitBefore.GetTimestamp().AsTime()),
			),
		}
	case *proto.QueryNode_CommitAfter:
		return []log.Field{
			log.Object("CommitAfter",
				log.Time("Time", v.CommitAfter.GetTimestamp().AsTime()),
			),
		}
	case *proto.QueryNode_MessageMatches:
		return []log.Field{
			log.Object("MessageMatches",
				log.String("Expr", v.MessageMatches.GetExpr()),
				log.Bool("IgnoreCase", v.MessageMatches.GetIgnoreCase()),
			),
		}
	case *proto.QueryNode_DiffMatches:
		return []log.Field{
			log.Object("DiffMatches",
				log.String("Expr", v.DiffMatches.GetExpr()),
				log.Bool("IgnoreCase", v.DiffMatches.GetIgnoreCase()),
			),
		}
	case *proto.QueryNode_DiffModifiesFile:
		return []log.Field{
			log.Object("DiffModifiesFile",
				log.String("Expr", v.DiffModifiesFile.GetExpr()),
				log.Bool("IgnoreCase", v.DiffModifiesFile.GetIgnoreCase()),
			),
		}
	case *proto.QueryNode_Boolean:
		return []log.Field{
			log.Object("Boolean",
				log.Bool("Value", v.Boolean.GetValue()),
			),
		}
	case *proto.QueryNode_Operator:
		operands := make([]log.Field, 0, len(v.Operator.GetOperands()))
		for _, operand := range v.Operator.GetOperands() {
			operands = append(operands, log.Object("Operand", queryNodeToLogFields(operand)...))
		}
		return []log.Field{
			log.Object("Operator",
				log.Object("Kind", log.String("Kind", v.Operator.GetKind().String())),
				log.Object("Operands", operands...),
			),
		}
	default:
		return []log.Field{
			log.String("unknownQueryNodeType", fmt.Sprintf("%T", p.GetValue())),
		}
	}
}

func (l *loggingGRPCServer) Archive(request *proto.ArchiveRequest, server proto.GitserverService_ArchiveServer) (err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_Archive_FullMethodName,
			status.Code(err),
			trace.Context(server.Context()).TraceID,
			elapsed,

			archiveRequestToLogFields(request)...,
		)
	}()

	return l.base.Archive(request, server)
}

func archiveRequestToLogFields(req *proto.ArchiveRequest) []log.Field {
	return []log.Field{
		log.String("repo", req.GetRepo()),
		log.String("treeish", req.GetTreeish()),
		log.String("format", req.GetFormat().String()),
		log.Strings("paths", byteSlicesToStrings(req.GetPaths())),
	}
}

func (l *loggingGRPCServer) RepoCloneProgress(ctx context.Context, request *proto.RepoCloneProgressRequest) (response *proto.RepoCloneProgressResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_RepoCloneProgress_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			repoCloneProgressRequest(request)...,
		)

	}()

	return l.base.RepoCloneProgress(ctx, request)
}

func repoCloneProgressRequest(req *proto.RepoCloneProgressRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
	}
}

func (l *loggingGRPCServer) IsPerforcePathCloneable(ctx context.Context, request *proto.IsPerforcePathCloneableRequest) (response *proto.IsPerforcePathCloneableResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		doLog(
			l.logger,
			proto.GitserverService_IsPerforcePathCloneable_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			isPerforcePathCloneableRequestToLogFields(request)...,
		)
	}()

	return l.base.IsPerforcePathCloneable(ctx, request)
}

func isPerforcePathCloneableRequestToLogFields(req *proto.IsPerforcePathCloneableRequest) []log.Field {
	return []log.Field{
		log.String("depotPath", req.GetDepotPath()),
		log.Object("connectionDetails", perforceConnectionDetailsToLogFields(req.GetConnectionDetails())...),
	}
}

func perforceConnectionDetailsToLogFields(req *proto.PerforceConnectionDetails) []log.Field {
	return []log.Field{
		log.String("p4Port", req.GetP4Port()),
		log.String("p4User", req.GetP4User()),
		// 🚨SECURITY: We don't log the p4Password field because it could contain sensitive data.
	}
}

func (l *loggingGRPCServer) CheckPerforceCredentials(ctx context.Context, request *proto.CheckPerforceCredentialsRequest) (response *proto.CheckPerforceCredentialsResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_CheckPerforceCredentials_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			checkPerforceCredentialsRequestToLogFields(request)...,
		)

	}()

	return l.base.CheckPerforceCredentials(ctx, request)
}

func checkPerforceCredentialsRequestToLogFields(req *proto.CheckPerforceCredentialsRequest) []log.Field {
	return []log.Field{
		log.Object("connectionDetails", perforceConnectionDetailsToLogFields(req.GetConnectionDetails())...),
	}
}

func (l *loggingGRPCServer) PerforceUsers(ctx context.Context, request *proto.PerforceUsersRequest) (response *proto.PerforceUsersResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_PerforceUsers_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			perforceUsersRequestToLogFields(request)...,
		)
	}()

	return l.base.PerforceUsers(ctx, request)
}

func perforceUsersRequestToLogFields(req *proto.PerforceUsersRequest) []log.Field {
	return []log.Field{
		log.Object("connectionDetails", perforceConnectionDetailsToLogFields(req.GetConnectionDetails())...),
	}
}

func (l *loggingGRPCServer) PerforceProtectsForUser(ctx context.Context, request *proto.PerforceProtectsForUserRequest) (response *proto.PerforceProtectsForUserResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_PerforceProtectsForUser_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			perforceProtectsForUserRequestToLogFields(request)...,
		)
	}()

	return l.base.PerforceProtectsForUser(ctx, request)
}

func perforceProtectsForUserRequestToLogFields(req *proto.PerforceProtectsForUserRequest) []log.Field {
	return []log.Field{
		log.String("username", req.GetUsername()),
		log.Object("connectionDetails", perforceConnectionDetailsToLogFields(req.GetConnectionDetails())...),
	}
}

func (l *loggingGRPCServer) PerforceProtectsForDepot(ctx context.Context, request *proto.PerforceProtectsForDepotRequest) (response *proto.PerforceProtectsForDepotResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_PerforceProtectsForDepot_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			perforceProtectsForDepotRequestToLogFields(request)...,
		)
	}()

	return l.base.PerforceProtectsForDepot(ctx, request)
}

func perforceProtectsForDepotRequestToLogFields(req *proto.PerforceProtectsForDepotRequest) []log.Field {
	return []log.Field{
		log.String("depot", req.GetDepot()),
		log.Object("connectionDetails", perforceConnectionDetailsToLogFields(req.GetConnectionDetails())...),
	}
}

func (l *loggingGRPCServer) PerforceGroupMembers(ctx context.Context, request *proto.PerforceGroupMembersRequest) (response *proto.PerforceGroupMembersResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_PerforceGroupMembers_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			perforceGroupMembersRequestToLogFields(request)...,
		)
	}()

	return l.base.PerforceGroupMembers(ctx, request)
}

func perforceGroupMembersRequestToLogFields(req *proto.PerforceGroupMembersRequest) []log.Field {
	return []log.Field{
		log.String("group", req.GetGroup()),
		log.Object("connectionDetails", perforceConnectionDetailsToLogFields(req.GetConnectionDetails())...),
	}
}

func (l *loggingGRPCServer) IsPerforceSuperUser(ctx context.Context, request *proto.IsPerforceSuperUserRequest) (response *proto.IsPerforceSuperUserResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_IsPerforceSuperUser_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			isPerforceSuperUserRequestToLogFields(request)...,
		)
	}()

	return l.base.IsPerforceSuperUser(ctx, request)
}

func isPerforceSuperUserRequestToLogFields(req *proto.IsPerforceSuperUserRequest) []log.Field {
	return []log.Field{
		log.Object("connectionDetails", perforceConnectionDetailsToLogFields(req.GetConnectionDetails())...),
	}
}

func (l *loggingGRPCServer) PerforceGetChangelist(ctx context.Context, request *proto.PerforceGetChangelistRequest) (response *proto.PerforceGetChangelistResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_PerforceGetChangelist_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			perforceGetChangelistRequestToLogFields(request)...,
		)
	}()

	return l.base.PerforceGetChangelist(ctx, request)
}

func perforceGetChangelistRequestToLogFields(req *proto.PerforceGetChangelistRequest) []log.Field {
	return []log.Field{
		log.String("changelistId", req.GetChangelistId()),
		log.Object("connectionDetails", perforceConnectionDetailsToLogFields(req.GetConnectionDetails())...),
	}
}

func (l *loggingGRPCServer) MergeBase(ctx context.Context, request *proto.MergeBaseRequest) (response *proto.MergeBaseResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_MergeBase_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			mergeBaseRequestToLogFields(request)...,
		)
	}()

	return l.base.MergeBase(ctx, request)
}

func mergeBaseRequestToLogFields(req *proto.MergeBaseRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.String("base", string(req.GetBase())),
		log.String("head", string(req.GetHead())),
	}
}

func (l *loggingGRPCServer) Blame(request *proto.BlameRequest, server proto.GitserverService_BlameServer) (err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_Blame_FullMethodName,
			status.Code(err),
			trace.Context(server.Context()).TraceID,
			elapsed,

			blameRequestToLogFields(request)...,
		)
	}()

	return l.base.Blame(request, server)
}

func blameRequestToLogFields(req *proto.BlameRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.String("commit", req.GetCommit()),
		log.String("path", req.GetPath()),
		log.Bool("ignoreWhitespace", req.GetIgnoreWhitespace()),
		log.Object("range", blameRangeToLogFields(req.GetRange())...),
	}
}

func blameRangeToLogFields(req *proto.BlameRange) []log.Field {
	return []log.Field{
		log.Uint32("startLine", req.GetStartLine()),
		log.Uint32("endLine", req.GetEndLine()),
	}
}

func (l *loggingGRPCServer) DefaultBranch(ctx context.Context, request *proto.DefaultBranchRequest) (response *proto.DefaultBranchResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_DefaultBranch_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			defaultBranchRequestToLogFields(request)...,
		)
	}()

	return l.base.DefaultBranch(ctx, request)
}

func defaultBranchRequestToLogFields(req *proto.DefaultBranchRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.Bool("shortRef", req.GetShortRef()),
	}
}

func (l *loggingGRPCServer) ReadFile(request *proto.ReadFileRequest, server proto.GitserverService_ReadFileServer) (err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_ReadFile_FullMethodName,
			status.Code(err),
			trace.Context(server.Context()).TraceID,
			elapsed,

			readFileRequestToLogFields(request)...,
		)

	}()

	return l.base.ReadFile(request, server)
}

func readFileRequestToLogFields(req *proto.ReadFileRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.String("commit", req.GetCommit()),
		log.String("path", req.GetPath()),
	}
}

func (l *loggingGRPCServer) GetCommit(ctx context.Context, request *proto.GetCommitRequest) (response *proto.GetCommitResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		doLog(
			l.logger,
			proto.GitserverService_GetCommit_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			getCommitRequestToLogFields(request)...,
		)
	}()

	return l.base.GetCommit(ctx, request)
}

func getCommitRequestToLogFields(req *proto.GetCommitRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.String("commit", req.GetCommit()),
	}
}

func (l *loggingGRPCServer) ResolveRevision(ctx context.Context, request *proto.ResolveRevisionRequest) (response *proto.ResolveRevisionResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_ResolveRevision_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			resolveRevisionRequestToLogFields(request)...,
		)

	}()

	return l.base.ResolveRevision(ctx, request)
}

func resolveRevisionRequestToLogFields(req *proto.ResolveRevisionRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.String("revSpec", string(req.GetRevSpec())),
		log.Bool("ensureRevision", req.GetEnsureRevision()),
	}
}

func (l *loggingGRPCServer) ListRefs(request *proto.ListRefsRequest, server proto.GitserverService_ListRefsServer) (err error) {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_ListRefs_FullMethodName,
			status.Code(err),
			trace.Context(server.Context()).TraceID,
			elapsed,

			listRefsRequestToLogFields(request)...,
		)
	}()

	return l.base.ListRefs(request, server)
}

func listRefsRequestToLogFields(req *proto.ListRefsRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.Bool("headsOnly", req.GetHeadsOnly()),
		log.Bool("tagsOnly", req.GetTagsOnly()),
		log.Strings("pointsAtCommit", req.GetPointsAtCommit()),
		log.String("containsSha", req.GetContainsSha()),
	}
}

func (l *loggingGRPCServer) RevAtTime(ctx context.Context, request *proto.RevAtTimeRequest) (resp *proto.RevAtTimeResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_RevAtTime_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			revAtTimeRequestToLogFields(request)...,
		)
	}()

	return l.base.RevAtTime(ctx, request)
}

func revAtTimeRequestToLogFields(req *proto.RevAtTimeRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.String("revSpec", string(req.GetRevSpec())),
		log.Time("time", req.GetTime().AsTime()),
	}
}

func (l *loggingGRPCServer) RawDiff(request *proto.RawDiffRequest, server proto.GitserverService_RawDiffServer) error {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_RawDiff_FullMethodName,
			status.Code(server.Context().Err()),
			trace.Context(server.Context()).TraceID,
			elapsed,

			rawDiffRequestToLogFields(request)...,
		)
	}()

	return l.base.RawDiff(request, server)
}

func rawDiffRequestToLogFields(req *proto.RawDiffRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.String("baseRevSpec", string(req.GetBaseRevSpec())),
		log.String("headRevSpec", string(req.GetHeadRevSpec())),
		log.String("comparisonType", req.GetComparisonType().String()),
		log.Strings("paths", byteSlicesToStrings(req.GetPaths())),
	}
}

func (l *loggingGRPCServer) ContributorCounts(ctx context.Context, request *proto.ContributorCountsRequest) (resp *proto.ContributorCountsResponse, err error) {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_ContributorCounts_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			contributorCountsToLogFields(request)...,
		)
	}()

	return l.base.ContributorCounts(ctx, request)
}

func contributorCountsToLogFields(req *proto.ContributorCountsRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.String("range", string(req.GetRange())),
		log.Time("after", req.GetAfter().AsTime()),
		log.String("path", string(req.GetPath())),
	}
}

func (l *loggingGRPCServer) FirstEverCommit(ctx context.Context, request *proto.FirstEverCommitRequest) (resp *proto.FirstEverCommitResponse, err error) {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_FirstEverCommit_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			firstEverCommitRequestToLogFields(request)...,
		)
	}()

	return l.base.FirstEverCommit(ctx, request)
}

func firstEverCommitRequestToLogFields(req *proto.FirstEverCommitRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
	}
}

func (l *loggingGRPCServer) BehindAhead(ctx context.Context, request *proto.BehindAheadRequest) (response *proto.BehindAheadResponse, err error) {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_BehindAhead_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			BehindAheadRequestToLogFields(request)...,
		)
	}()

	return l.base.BehindAhead(ctx, request)
}

func BehindAheadRequestToLogFields(req *proto.BehindAheadRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.String("left", string(req.GetLeft())),
		log.String("right", string(req.GetRight())),
	}
}

func (l *loggingGRPCServer) ChangedFiles(req *proto.ChangedFilesRequest, ss proto.GitserverService_ChangedFilesServer) (err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_ChangedFiles_FullMethodName,
			status.Code(err),
			trace.Context(ss.Context()).TraceID,
			elapsed,

			changedFilesRequestToLogFields(req)...,
		)

	}()

	return l.base.ChangedFiles(req, ss)
}

func changedFilesRequestToLogFields(req *proto.ChangedFilesRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.String("base", string(req.GetBase())),
		log.String("head", string(req.GetHead())),
	}
}

func (l *loggingGRPCServer) Stat(ctx context.Context, request *proto.StatRequest) (resp *proto.StatResponse, err error) {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_Stat_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			statRequestToLogFields(request)...,
		)
	}()

	return l.base.Stat(ctx, request)
}

func statRequestToLogFields(req *proto.StatRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.String("commit", string(req.GetCommitSha())),
		log.String("path", string(req.GetPath())),
	}
}

func (l *loggingGRPCServer) ReadDir(request *proto.ReadDirRequest, server proto.GitserverService_ReadDirServer) error {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_ReadDir_FullMethodName,
			status.Code(server.Context().Err()),
			trace.Context(server.Context()).TraceID,
			elapsed,

			readDirRequestToLogFields(request)...,
		)
	}()

	return l.base.ReadDir(request, server)
}

func readDirRequestToLogFields(req *proto.ReadDirRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.String("commit", string(req.GetCommitSha())),
		log.String("path", string(req.GetPath())),
		log.Bool("recursive", req.GetRecursive()),
	}
}

func (l *loggingGRPCServer) CommitLog(request *proto.CommitLogRequest, server proto.GitserverService_CommitLogServer) error {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,
			proto.GitserverService_CommitLog_FullMethodName,
			status.Code(server.Context().Err()),
			trace.Context(server.Context()).TraceID,
			elapsed,

			commitLogRequestToLogFields(request)...,
		)
	}()

	return l.base.CommitLog(request, server)
}

func commitLogRequestToLogFields(req *proto.CommitLogRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
		log.Strings("ranges", byteSlicesToStrings(req.GetRanges())),
		log.Bool("allRefs", req.GetAllRefs()),
		log.Time("after", req.GetAfter().AsTime()),
		log.Time("before", req.GetBefore().AsTime()),
		log.Int("maxCommits", int(req.GetMaxCommits())),
		log.Int("skip", int(req.GetSkip())),
		log.Bool("followOnlyFirstParent", req.GetFollowOnlyFirstParent()),
		log.Bool("includeModifiedFiles", req.GetIncludeModifiedFiles()),
		log.Int("order", int(req.GetOrder())),
		log.String("messageQuery", string(req.GetMessageQuery())),
		log.String("authorQuery", string(req.GetAuthorQuery())),
		log.Bool("followPathRenames", req.GetFollowPathRenames()),
		log.String("path", string(req.GetPath())),
	}
}

type loggingRepositoryServiceServer struct {
	base   proto.GitserverRepositoryServiceServer
	logger log.Logger

	proto.UnsafeGitserverRepositoryServiceServer
}

func (l *loggingRepositoryServiceServer) DeleteRepository(ctx context.Context, request *proto.DeleteRepositoryRequest) (resp *proto.DeleteRepositoryResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,

			proto.GitserverRepositoryService_DeleteRepository_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			deleteRepositoryRequestToLogFields(request)...,
		)

	}()
	return l.base.DeleteRepository(ctx, request)
}

func deleteRepositoryRequestToLogFields(req *proto.DeleteRepositoryRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
	}
}

func (l *loggingRepositoryServiceServer) FetchRepository(ctx context.Context, request *proto.FetchRepositoryRequest) (resp *proto.FetchRepositoryResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,

			proto.GitserverRepositoryService_FetchRepository_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			fetchRepositoryRequestToLogFields(request)...,
		)
	}()

	return l.base.FetchRepository(ctx, request)
}

func fetchRepositoryRequestToLogFields(req *proto.FetchRepositoryRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
	}
}

func (l *loggingRepositoryServiceServer) OptimizeRepository(ctx context.Context, request *proto.OptimizeRepositoryRequest) (resp *proto.OptimizeRepositoryResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,

			proto.GitserverRepositoryService_OptimizeRepository_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			optimizeRepositoryRequestToLogFields(request)...,
		)
	}()

	return l.base.OptimizeRepository(ctx, request)
}

func optimizeRepositoryRequestToLogFields(req *proto.OptimizeRepositoryRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
	}
}

func (l *loggingRepositoryServiceServer) RepositoryStatistics(ctx context.Context, request *proto.RepositoryStatisticsRequest) (resp *proto.RepositoryStatisticsResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,

			proto.GitserverRepositoryService_RepositoryStatistics_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			repositoryStatisticsRequestToLogFields(request)...,
		)
	}()

	return l.base.RepositoryStatistics(ctx, request)
}

func repositoryStatisticsRequestToLogFields(req *proto.RepositoryStatisticsRequest) []log.Field {
	return []log.Field{
		log.String("repoName", req.GetRepoName()),
	}
}

func (l *loggingRepositoryServiceServer) ListRepositories(ctx context.Context, request *proto.ListRepositoriesRequest) (resp *proto.ListRepositoriesResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)

		doLog(
			l.logger,

			proto.GitserverRepositoryService_ListRepositories_FullMethodName,
			status.Code(err),
			trace.Context(ctx).TraceID,
			elapsed,

			listRepositoriesRequestToLogFields(request)...,
		)
	}()

	return l.base.ListRepositories(ctx, request)
}

func listRepositoriesRequestToLogFields(req *proto.ListRepositoriesRequest) []log.Field {
	return []log.Field{
		log.Int("page_size", int(req.GetPageSize())),
	}
}

var (
	_ proto.GitserverServiceServer           = &loggingGRPCServer{}
	_ proto.GitserverRepositoryServiceServer = &loggingRepositoryServiceServer{}
)
