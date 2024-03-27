package internal

import (
	"context"
	"time"

	"github.com/sourcegraph/log"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

type loggingGRPCServer struct {
	proto.GitserverServiceServer
	logger log.Logger
}

func (l *loggingGRPCServer) doLog(message string, fields ...log.Field) {
	l.logger.Info(message, fields...)
}

func (l *loggingGRPCServer) CreateCommitFromPatchBinary(server proto.GitserverService_CreateCommitFromPatchBinaryServer) error {
	return l.GitserverServiceServer.CreateCommitFromPatchBinary(server)
}

func (l *loggingGRPCServer) DiskInfo(ctx context.Context, request *proto.DiskInfoRequest) (response *proto.DiskInfoResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "DiskInfo"),
			log.String("request", "<empty>"),
			log.String("status", status.Code(err).String()),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received DiskInfo request", fields...)
	}()

	return l.GitserverServiceServer.DiskInfo(ctx, request)
}

func (l *loggingGRPCServer) Exec(request *proto.ExecRequest, server proto.GitserverService_ExecServer) (err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(server.Context()).TraceID),
			log.String("method", "Exec"),
			log.String("repo", request.GetRepo()),
			log.Strings("args", byteSlicesToStrings(request.GetArgs())),
			log.Bool("noTimeout", request.GetNoTimeout()),
			log.String("status", status.Code(err).String()),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received Exec request", fields...)
	}()

	return l.GitserverServiceServer.Exec(request, server)
}

func (l *loggingGRPCServer) GetObject(ctx context.Context, request *proto.GetObjectRequest) (response *proto.GetObjectResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "GetObject"),
			log.String("status", status.Code(err).String()),
			log.String("request", protojson.Format(request)),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received GetObject request", fields...)
	}()

	return l.GitserverServiceServer.GetObject(ctx, request)
}

func (l *loggingGRPCServer) IsRepoCloneable(ctx context.Context, request *proto.IsRepoCloneableRequest) (response *proto.IsRepoCloneableResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "IsRepoCloneable"),
			log.String("status", status.Code(err).String()),
			log.String("request", protojson.Format(request)),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received IsRepoCloneable request", fields...)
	}()

	return l.GitserverServiceServer.IsRepoCloneable(ctx, request)
}

func (l *loggingGRPCServer) ListGitolite(ctx context.Context, request *proto.ListGitoliteRequest) (response *proto.ListGitoliteResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "ListGitolite"),
			log.String("status", status.Code(err).String()),
			log.String("request", protojson.Format(request)),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received ListGitolite request", fields...)
	}()
	return l.GitserverServiceServer.ListGitolite(ctx, request)
}

func (l *loggingGRPCServer) Search(request *proto.SearchRequest, server proto.GitserverService_SearchServer) (err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(server.Context()).TraceID),
			log.String("method", "Search"),
			log.String("status", status.Code(err).String()),
			log.String("request", protojson.Format(request)),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received Search request", fields...)
	}()

	return l.GitserverServiceServer.Search(request, server)
}

func (l *loggingGRPCServer) Archive(request *proto.ArchiveRequest, server proto.GitserverService_ArchiveServer) (err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(server.Context()).TraceID),
			log.String("method", "Archive"),
			log.String("status", status.Code(err).String()),
			log.String("request", protojson.Format(request)),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received Archive request", fields...)
	}()

	return l.GitserverServiceServer.Archive(request, server)
}

func (l *loggingGRPCServer) RepoClone(ctx context.Context, request *proto.RepoCloneRequest) (response *proto.RepoCloneResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "RepoClone"),
			log.String("status", status.Code(err).String()),
			log.String("request", protojson.Format(request)),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received RepoClone request", fields...)
	}()

	return l.GitserverServiceServer.RepoClone(ctx, request)
}

func (l *loggingGRPCServer) RepoCloneProgress(ctx context.Context, request *proto.RepoCloneProgressRequest) (response *proto.RepoCloneProgressResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "RepoCloneProgress"),
			log.String("status", status.Code(err).String()),
			log.String("request", protojson.Format(request)),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received RepoCloneProgress request", fields...)
	}()

	return l.GitserverServiceServer.RepoCloneProgress(ctx, request)
}

func (l *loggingGRPCServer) RepoDelete(ctx context.Context, request *proto.RepoDeleteRequest) (response *proto.RepoDeleteResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "RepoDelete"),
			log.String("status", status.Code(err).String()),
			log.String("request", protojson.Format(request)),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received RepoDelete request", fields...)
	}()

	return l.GitserverServiceServer.RepoDelete(ctx, request)
}

func (l *loggingGRPCServer) RepoUpdate(ctx context.Context, request *proto.RepoUpdateRequest) (response *proto.RepoUpdateResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "RepoUpdate"),
			log.String("status", status.Code(err).String()),
			log.String("request", protojson.Format(request)),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received RepoUpdate request", fields...)
	}()

	return l.GitserverServiceServer.RepoUpdate(ctx, request)
}

func (l *loggingGRPCServer) IsPerforcePathCloneable(ctx context.Context, request *proto.IsPerforcePathCloneableRequest) (response *proto.IsPerforcePathCloneableResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "IsPerforcePathCloneable"),
			log.String("status", status.Code(err).String()),
			log.String("depotPath", request.GetDepotPath()),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received IsPerforcePathCloneable request", fields...)
	}()

	return l.GitserverServiceServer.IsPerforcePathCloneable(ctx, request)
}

func (l *loggingGRPCServer) CheckPerforceCredentials(ctx context.Context, request *proto.CheckPerforceCredentialsRequest) (response *proto.CheckPerforceCredentialsResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "CheckPerforceCredentials"),
			log.String("status", status.Code(err).String()),
			log.String("request", "<empty"),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received CheckPerforceCredentials request", fields...)
	}()

	return l.GitserverServiceServer.CheckPerforceCredentials(ctx, request)
}

func (l *loggingGRPCServer) PerforceUsers(ctx context.Context, request *proto.PerforceUsersRequest) (response *proto.PerforceUsersResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "PerforceUsers"),
			log.String("status", status.Code(err).String()),
			log.String("request", "<empty>"),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received PerforceUsers request", fields...)
	}()

	return l.GitserverServiceServer.PerforceUsers(ctx, request)
}

func (l *loggingGRPCServer) PerforceProtectsForUser(ctx context.Context, request *proto.PerforceProtectsForUserRequest) (response *proto.PerforceProtectsForUserResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "PerforceProtectsForUser"),
			log.String("status", status.Code(err).String()),
			log.String("username", request.GetUsername()),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received PerforceProtectsForUser request", fields...)
	}()

	return l.GitserverServiceServer.PerforceProtectsForUser(ctx, request)
}

func (l *loggingGRPCServer) PerforceProtectsForDepot(ctx context.Context, request *proto.PerforceProtectsForDepotRequest) (response *proto.PerforceProtectsForDepotResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "PerforceProtectsForDepot"),
			log.String("status", status.Code(err).String()),
			log.String("depot", request.GetDepot()),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received PerforceProtectsForDepot request", fields...)
	}()

	return l.GitserverServiceServer.PerforceProtectsForDepot(ctx, request)
}

func (l *loggingGRPCServer) PerforceGroupMembers(ctx context.Context, request *proto.PerforceGroupMembersRequest) (response *proto.PerforceGroupMembersResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "PerforceGroupMembers"),
			log.String("status", status.Code(err).String()),
			log.String("group", request.GetGroup()),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received PerforceGroupMembers request", fields...)
	}()

	return l.GitserverServiceServer.PerforceGroupMembers(ctx, request)
}

func (l *loggingGRPCServer) IsPerforceSuperUser(ctx context.Context, request *proto.IsPerforceSuperUserRequest) (response *proto.IsPerforceSuperUserResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "IsPerforceSuperUser"),
			log.String("status", status.Code(err).String()),
			log.String("request", "<empty>"),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received IsPerforceSuperUser request", fields...)
	}()

	return l.GitserverServiceServer.IsPerforceSuperUser(ctx, request)
}

func (l *loggingGRPCServer) PerforceGetChangelist(ctx context.Context, request *proto.PerforceGetChangelistRequest) (response *proto.PerforceGetChangelistResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "PerforceGetChangelist"),
			log.String("status", status.Code(err).String()),
			log.String("changelistId", request.GetChangelistId()),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received PerforceGetChangelist request", fields...)
	}()

	return l.GitserverServiceServer.PerforceGetChangelist(ctx, request)
}

func (l *loggingGRPCServer) MergeBase(ctx context.Context, request *proto.MergeBaseRequest) (response *proto.MergeBaseResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "MergeBase"),
			log.String("status", status.Code(err).String()),
			log.String("request", protojson.Format(request)),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received MergeBase request", fields...)
	}()

	return l.GitserverServiceServer.MergeBase(ctx, request)
}

func (l *loggingGRPCServer) Blame(request *proto.BlameRequest, server proto.GitserverService_BlameServer) (err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(server.Context()).TraceID),
			log.String("method", "Blame"),
			log.String("status", status.Code(err).String()),
			log.String("request", protojson.Format(request)),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received Blame request", fields...)
	}()

	return l.GitserverServiceServer.Blame(request, server)
}

func (l *loggingGRPCServer) DefaultBranch(ctx context.Context, request *proto.DefaultBranchRequest) (response *proto.DefaultBranchResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "DefaultBranch"),
			log.String("status", status.Code(err).String()),
			log.String("request", protojson.Format(request)),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received DefaultBranch request", fields...)
	}()

	return l.GitserverServiceServer.DefaultBranch(ctx, request)
}

func (l *loggingGRPCServer) ReadFile(request *proto.ReadFileRequest, server proto.GitserverService_ReadFileServer) (err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(server.Context()).TraceID),
			log.String("method", "ReadFile"),
			log.String("status", status.Code(err).String()),
			log.String("request", protojson.Format(request)),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received ReadFile request", fields...)
	}()

	return l.GitserverServiceServer.ReadFile(request, server)
}

func (l *loggingGRPCServer) GetCommit(ctx context.Context, request *proto.GetCommitRequest) (response *proto.GetCommitResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "GetCommit"),
			log.String("status", status.Code(err).String()),
			log.String("request", protojson.Format(request)),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received GetCommit request", fields...)
	}()

	return l.GitserverServiceServer.GetCommit(ctx, request)
}

func (l *loggingGRPCServer) ResolveRevision(ctx context.Context, request *proto.ResolveRevisionRequest) (response *proto.ResolveRevisionResponse, err error) {
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		fields := []log.Field{
			log.String("traceID", trace.Context(ctx).TraceID),
			log.String("method", "ResolveRevision"),
			log.String("status", status.Code(err).String()),
			log.String("repoName", request.GetRepoName()),
			log.String("revSpec", string(request.GetRevSpec())),
			log.Duration("duration", elapsed),
		}

		l.doLog("Received ResolveRevision request", fields...)
	}()

	return l.GitserverServiceServer.ResolveRevision(ctx, request)
}

func (l *loggingGRPCServer) mustEmbedUnimplementedGitserverServiceServer() {
}

var _ proto.GitserverServiceServer = &loggingGRPCServer{}
