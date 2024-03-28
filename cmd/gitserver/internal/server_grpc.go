package internal

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/accesslog"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git/gitcli"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/perforce"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/internal/grpc/streamio"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

type service interface {
	CreateCommitFromPatch(ctx context.Context, req protocol.CreateCommitFromPatchRequest, patchReader io.Reader) protocol.CreateCommitFromPatchResponse
	LogIfCorrupt(context.Context, api.RepoName, error)
	MaybeStartClone(ctx context.Context, repo api.RepoName) (cloned bool, status CloneStatus, _ error)
	IsRepoCloneable(ctx context.Context, repo api.RepoName) (protocol.IsRepoCloneableResponse, error)
	RepoUpdate(ctx context.Context, req *protocol.RepoUpdateRequest) protocol.RepoUpdateResponse
	CloneRepo(ctx context.Context, repo api.RepoName, opts CloneOptions) (cloneProgress string, err error)
	SearchWithObservability(ctx context.Context, tr trace.Trace, args *protocol.SearchRequest, onMatch func(*protocol.CommitMatch) error) (limitHit bool, err error)
	EnsureRevision(ctx context.Context, repo api.RepoName, rev string) (didUpdate bool)
}

func NewGRPCServer(server *Server) proto.GitserverServiceServer {
	return &grpcServer{
		logger:         server.logger,
		db:             server.db,
		hostname:       server.hostname,
		subRepoChecker: authz.DefaultSubRepoPermsChecker,
		locker:         server.locker,
		getBackendFunc: server.getBackendFunc,
		svc:            server,
		fs:             server.fs,
	}
}

type grpcServer struct {
	logger         log.Logger
	db             database.DB
	hostname       string
	subRepoChecker authz.SubRepoPermissionChecker
	locker         RepositoryLocker
	getBackendFunc Backender
	fs             gitserverfs.FS

	svc service

	proto.UnimplementedGitserverServiceServer
}

var _ proto.GitserverServiceServer = &grpcServer{}

func (gs *grpcServer) CreateCommitFromPatchBinary(s proto.GitserverService_CreateCommitFromPatchBinaryServer) error {
	var metadata *proto.CreateCommitFromPatchBinaryRequest_Metadata

	firstMsg, err := s.Recv()
	if err != nil {
		return err
	}

	switch firstMsg.Payload.(type) {
	case *proto.CreateCommitFromPatchBinaryRequest_Metadata_:
		metadata = firstMsg.GetMetadata()
	default:
		return status.New(codes.InvalidArgument, "must send metadata event first").Err()
	}

	patchReader := streamio.NewReader(func() ([]byte, error) {
		msg, err := s.Recv()
		if err != nil {
			return nil, err
		}

		switch msg.Payload.(type) {
		case *proto.CreateCommitFromPatchBinaryRequest_Patch_:
			return msg.GetPatch().GetData(), nil
		default:
			return nil, status.New(codes.InvalidArgument, "must only send patch events after metadata").Err()
		}
	})

	var r protocol.CreateCommitFromPatchRequest
	r.FromProto(metadata)
	resp := gs.svc.CreateCommitFromPatch(s.Context(), r, patchReader)
	res, patchErr := resp.ToProto()
	if patchErr != nil {
		return patchErr.ToStatus().Err()
	}

	return s.SendAndClose(res)
}

func (gs *grpcServer) DiskInfo(_ context.Context, _ *proto.DiskInfoRequest) (*proto.DiskInfoResponse, error) {
	usage, err := gs.fs.DiskUsage()
	if err != nil {
		return nil, err
	}

	return &proto.DiskInfoResponse{
		TotalSpace:  usage.Size(),
		FreeSpace:   usage.Free(),
		PercentUsed: usage.PercentUsed(),
	}, nil
}

func (gs *grpcServer) Exec(req *proto.ExecRequest, ss proto.GitserverService_ExecServer) error {
	ctx := ss.Context()

	// Log which actor is accessing the repo.
	args := byteSlicesToStrings(req.GetArgs())
	logAttrs := []log.Field{}
	if len(args) > 0 {
		logAttrs = append(logAttrs,
			log.String("cmd", args[0]),
			log.Strings("args", args[1:]),
		)
	}

	accesslog.Record(ctx, req.GetRepo(), logAttrs...)

	if req.GetRepo() == "" {
		return status.New(codes.InvalidArgument, "repo must be specified").Err()
	}

	repoName := api.RepoName(req.GetRepo())
	repoDir := gs.fs.RepoDir(repoName)
	backend := gs.getBackendFunc(repoDir, repoName)

	if err := gs.maybeStartClone(ctx, repoName); err != nil {
		return err
	}

	if req.GetNoTimeout() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 24*time.Hour)
		defer cancel()

	}

	w := streamio.NewWriter(func(p []byte) error {
		return ss.Send(&proto.ExecResponse{
			Data: p,
		})
	})

	// Special-case `git rev-parse HEAD` requests. These are invoked by search queries for every repo in scope.
	// For searches over large repo sets (> 1k), this leads to too many child process execs, which can lead
	// to a persistent failure mode where every exec takes > 10s, which is disastrous for gitserver performance.
	if len(args) == 2 && args[0] == "rev-parse" && args[1] == "HEAD" {
		if resolved, err := gitcli.QuickRevParseHead(repoDir); err == nil && gitdomain.IsAbsoluteRevision(resolved) {
			_, _ = w.Write([]byte(resolved))
			return nil
		}
	}

	// Special-case `git symbolic-ref HEAD` requests. These are invoked by resolvers determining the default branch of a repo.
	// For searches over large repo sets (> 1k), this leads to too many child process execs, which can lead
	// to a persistent failure mode where every exec takes > 10s, which is disastrous for gitserver performance.
	if len(args) == 2 && args[0] == "symbolic-ref" && args[1] == "HEAD" {
		if resolved, err := gitcli.QuickSymbolicRefHead(repoDir); err == nil {
			_, _ = w.Write([]byte(resolved))
			return nil
		}
	}

	stdout, err := backend.Exec(ctx, args...)
	if err != nil {
		if errors.Is(err, gitcli.ErrBadGitCommand) {
			return status.New(codes.InvalidArgument, "invalid command").Err()
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return status.FromContextError(ctxErr).Err()
		}

		return err
	}
	defer stdout.Close()

	_, err = io.Copy(w, stdout)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return status.FromContextError(ctxErr).Err()
		}

		commandFailedErr := &gitcli.CommandFailedError{}
		if errors.As(err, &commandFailedErr) {
			gRPCStatus := codes.Unknown
			if strings.Contains(commandFailedErr.Error(), "signal: killed") {
				gRPCStatus = codes.Aborted
			}

			var errString string
			if commandFailedErr.Unwrap() != nil {
				errString = commandFailedErr.Unwrap().Error()
			}
			s, err := status.New(gRPCStatus, errString).WithDetails(&proto.ExecStatusPayload{
				StatusCode: int32(commandFailedErr.ExitStatus),
				Stderr:     string(commandFailedErr.Stderr),
			})
			if err != nil {
				gs.logger.Error("failed to marshal status", log.Error(err))
				return err
			}
			return s.Err()
		}
		gs.svc.LogIfCorrupt(ctx, repoName, err)
		return err
	}

	return nil
}

func (gs *grpcServer) Archive(req *proto.ArchiveRequest, ss proto.GitserverService_ArchiveServer) error {
	ctx := ss.Context()

	if req.GetRepo() == "" {
		return status.New(codes.InvalidArgument, "repo must be specified").Err()
	}

	if req.GetTreeish() == "" {
		return status.New(codes.InvalidArgument, "treeish must be specified").Err()
	}

	var format git.ArchiveFormat
	switch req.GetFormat() {
	case proto.ArchiveFormat_ARCHIVE_FORMAT_ZIP:
		format = git.ArchiveFormatZip
	case proto.ArchiveFormat_ARCHIVE_FORMAT_TAR:
		format = git.ArchiveFormatTar
	default:
		return status.Error(codes.InvalidArgument, fmt.Sprintf("unknown archive format %q", req.GetFormat()))
	}

	if err := git.CheckSpecArgSafety(req.GetTreeish()); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	accesslog.Record(ctx, req.GetRepo(),
		log.String("treeish", req.GetTreeish()),
		log.String("format", string(format)),
		log.Strings("path", req.GetPaths()),
	)

	repoName := api.RepoName(req.GetRepo())
	repoDir := gs.fs.RepoDir(repoName)

	if err := gs.maybeStartClone(ss.Context(), repoName); err != nil {
		return err
	}

	if !actor.FromContext(ctx).IsInternal() {
		if enabled, err := gs.subRepoChecker.EnabledForRepo(ctx, repoName); err != nil {
			return errors.Wrap(err, "sub-repo permissions check")
		} else if enabled {
			s := status.New(codes.Unimplemented, "archiveReader invoked for a repo with sub-repo permissions")
			return s.Err()
		}
	}

	// This is a long time, but this never blocks a user request for this
	// long. Even repos that are not that large can take a long time, for
	// example a search over all repos in an organization may have several
	// large repos. All of those repos will be competing for IO => we need
	// a larger timeout.
	ctx, cancel := context.WithTimeout(ctx, conf.GitLongCommandTimeout())
	defer cancel()

	backend := gs.getBackendFunc(repoDir, repoName)

	r, err := backend.ArchiveReader(ctx, format, req.GetTreeish(), req.GetPaths())
	if err != nil {
		if os.IsNotExist(err) {
			var path string
			var pathError *os.PathError
			if errors.As(err, &pathError) {
				path = pathError.Path
			}
			s, err := status.New(codes.NotFound, "file not found").WithDetails(&proto.FileNotFoundPayload{
				Repo: string(repoName),
				// TODO: I'm not sure this should be allowed, a treeish is not necessarily
				// a commit.
				Commit: string(req.GetTreeish()),
				Path:   path,
			})
			if err != nil {
				return err
			}
			return s.Err()
		}

		var e *gitdomain.RevisionNotFoundError
		if errors.As(err, &e) {
			s, err := status.New(codes.NotFound, "revision not found").WithDetails(&proto.RevisionNotFoundPayload{
				Repo: req.GetRepo(),
				Spec: e.Spec,
			})
			if err != nil {
				return err
			}
			return s.Err()
		}

		gs.svc.LogIfCorrupt(ctx, repoName, err)
		// TODO: Better error checking.
		return err
	}
	defer r.Close()

	w := streamio.NewWriter(func(p []byte) error {
		return ss.Send(&proto.ArchiveResponse{
			Data: p,
		})
	})

	_, err = io.Copy(w, r)
	return err
}

func (gs *grpcServer) GetObject(ctx context.Context, req *proto.GetObjectRequest) (*proto.GetObjectResponse, error) {
	repoName := api.RepoName(req.GetRepo())
	repoDir := gs.fs.RepoDir(repoName)

	// Log which actor is accessing the repo.
	accesslog.Record(ctx, string(repoName), log.String("objectname", req.GetObjectName()))

	backend := gs.getBackendFunc(repoDir, repoName)

	obj, err := backend.GetObject(ctx, req.GetObjectName())
	if err != nil {
		gs.svc.LogIfCorrupt(ctx, repoName, err)
		gs.logger.Error("getting object", log.Error(err))
		return nil, err
	}

	resp := protocol.GetObjectResponse{
		Object: *obj,
	}

	return resp.ToProto(), nil
}

func (gs *grpcServer) ListGitolite(ctx context.Context, req *proto.ListGitoliteRequest) (*proto.ListGitoliteResponse, error) {
	host := req.GetGitoliteHost()
	repos, err := defaultGitolite.listRepos(ctx, host)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoRepos := make([]*proto.GitoliteRepo, 0, len(repos))

	for _, repo := range repos {
		protoRepos = append(protoRepos, repo.ToProto())
	}

	return &proto.ListGitoliteResponse{
		Repos: protoRepos,
	}, nil
}

func (gs *grpcServer) Search(req *proto.SearchRequest, ss proto.GitserverService_SearchServer) error {
	args, err := protocol.SearchRequestFromProto(req)
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	if req.GetRepo() == "" {
		return status.New(codes.InvalidArgument, "repo must be specified").Err()
	}

	repoName := api.RepoName(req.GetRepo())

	if err := gs.maybeStartClone(ss.Context(), repoName); err != nil {
		return err
	}

	onMatch := func(match *protocol.CommitMatch) error {
		return ss.Send(&proto.SearchResponse{
			Message: &proto.SearchResponse_Match{Match: match.ToProto()},
		})
	}

	tr, ctx := trace.New(ss.Context(), "search")
	defer tr.End()

	limitHit, err := gs.svc.SearchWithObservability(ctx, tr, args, onMatch)
	if err != nil {
		return err
	}

	return ss.Send(&proto.SearchResponse{
		Message: &proto.SearchResponse_LimitHit{
			LimitHit: limitHit,
		},
	})
}

func (gs *grpcServer) RepoClone(ctx context.Context, req *proto.RepoCloneRequest) (*proto.RepoCloneResponse, error) {
	if req.GetRepo() == "" {
		return nil, status.New(codes.InvalidArgument, "repo must be specified").Err()
	}

	repoName := api.RepoName(req.GetRepo())

	if _, err := gs.svc.CloneRepo(ctx, repoName, CloneOptions{Block: false}); err != nil {
		return &proto.RepoCloneResponse{Error: err.Error()}, nil
	}

	return &proto.RepoCloneResponse{Error: ""}, nil
}

func (gs *grpcServer) RepoCloneProgress(_ context.Context, req *proto.RepoCloneProgressRequest) (*proto.RepoCloneProgressResponse, error) {
	if req.GetRepoName() == "" {
		return nil, status.New(codes.InvalidArgument, "repo must be specified").Err()
	}

	repoName := api.RepoName(req.GetRepoName())

	progress, err := repoCloneProgress(gs.fs, gs.locker, repoName)
	if err != nil {
		return nil, err
	}

	return progress.ToProto(), nil
}

func (gs *grpcServer) RepoDelete(ctx context.Context, req *proto.RepoDeleteRequest) (*proto.RepoDeleteResponse, error) {
	if req.GetRepo() == "" {
		return nil, status.New(codes.InvalidArgument, "repo must be specified").Err()
	}

	repoName := api.RepoName(req.GetRepo())

	if err := deleteRepo(ctx, gs.db, gs.hostname, gs.fs, repoName); err != nil {
		gs.logger.Error("failed to delete repository", log.String("repo", string(repoName)), log.Error(err))
		return &proto.RepoDeleteResponse{}, status.Errorf(codes.Internal, "failed to delete repository %s: %s", repoName, err)
	}
	gs.logger.Info("deleted repository", log.String("repo", string(repoName)))
	return &proto.RepoDeleteResponse{}, nil
}

func (gs *grpcServer) RepoUpdate(ctx context.Context, req *proto.RepoUpdateRequest) (*proto.RepoUpdateResponse, error) {
	var in protocol.RepoUpdateRequest
	in.FromProto(req)

	resp := gs.svc.RepoUpdate(ctx, &in)

	return resp.ToProto(), nil
}

func (gs *grpcServer) IsRepoCloneable(ctx context.Context, req *proto.IsRepoCloneableRequest) (*proto.IsRepoCloneableResponse, error) {
	repo := api.RepoName(req.GetRepo())

	if repo == "" {
		return nil, status.Error(codes.InvalidArgument, "no Repo given")
	}

	resp, err := gs.svc.IsRepoCloneable(ctx, repo)
	if err != nil {
		return nil, err
	}

	return resp.ToProto(), nil
}

func (gs *grpcServer) IsPerforcePathCloneable(ctx context.Context, req *proto.IsPerforcePathCloneableRequest) (*proto.IsPerforcePathCloneableResponse, error) {
	if req.GetDepotPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "no DepotPath given")
	}

	conn := req.GetConnectionDetails()
	err := perforce.IsDepotPathCloneable(ctx, gs.fs, perforce.IsDepotPathCloneableArguments{
		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),

		DepotPath: req.GetDepotPath(),
	})
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &proto.IsPerforcePathCloneableResponse{}, nil
}

func (gs *grpcServer) CheckPerforceCredentials(ctx context.Context, req *proto.CheckPerforceCredentialsRequest) (*proto.CheckPerforceCredentialsResponse, error) {
	conn := req.GetConnectionDetails()
	err := perforce.P4TestWithTrust(ctx, gs.fs, perforce.P4TestWithTrustArguments{
		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, status.FromContextError(ctxErr).Err()
		}

		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &proto.CheckPerforceCredentialsResponse{}, nil
}

func (gs *grpcServer) PerforceUsers(ctx context.Context, req *proto.PerforceUsersRequest) (*proto.PerforceUsersResponse, error) {
	conn := req.GetConnectionDetails()
	err := perforce.P4TestWithTrust(ctx, gs.fs, perforce.P4TestWithTrustArguments{
		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, status.FromContextError(ctxErr).Err()
		}

		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	accesslog.Record(
		ctx,
		"<no-repo>",
		log.String("p4user", conn.GetP4User()),
		log.String("p4port", conn.GetP4Port()),
	)

	users, err := perforce.P4Users(ctx, gs.fs, perforce.P4UsersArguments{
		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &proto.PerforceUsersResponse{
		Users: make([]*proto.PerforceUser, 0, len(users)),
	}

	for _, user := range users {
		resp.Users = append(resp.Users, user.ToProto())
	}

	return resp, nil
}

func (gs *grpcServer) PerforceProtectsForUser(ctx context.Context, req *proto.PerforceProtectsForUserRequest) (*proto.PerforceProtectsForUserResponse, error) {
	conn := req.GetConnectionDetails()
	err := perforce.P4TestWithTrust(ctx, gs.fs, perforce.P4TestWithTrustArguments{
		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, status.FromContextError(ctxErr).Err()
		}

		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	accesslog.Record(
		ctx,
		"<no-repo>",
		log.String("p4user", conn.GetP4User()),
		log.String("p4port", conn.GetP4Port()),
	)

	args := perforce.P4ProtectsForUserArguments{
		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),

		Username: req.GetUsername(),
	}
	protects, err := perforce.P4ProtectsForUser(ctx, gs.fs, args)
	if err != nil {
		return nil, err
	}

	protoProtects := make([]*proto.PerforceProtect, len(protects))
	for i, p := range protects {
		protoProtects[i] = p.ToProto()
	}

	return &proto.PerforceProtectsForUserResponse{
		Protects: protoProtects,
	}, nil
}

func (gs *grpcServer) PerforceProtectsForDepot(ctx context.Context, req *proto.PerforceProtectsForDepotRequest) (*proto.PerforceProtectsForDepotResponse, error) {
	conn := req.GetConnectionDetails()
	err := perforce.P4TestWithTrust(ctx, gs.fs, perforce.P4TestWithTrustArguments{
		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, status.FromContextError(ctxErr).Err()
		}

		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	accesslog.Record(
		ctx,
		"<no-repo>",
		log.String("p4user", conn.GetP4User()),
		log.String("p4port", conn.GetP4Port()),
	)

	protects, err := perforce.P4ProtectsForDepot(ctx, gs.fs, perforce.P4ProtectsForDepotArguments{
		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),
		Depot:    req.GetDepot(),
	})
	if err != nil {
		return nil, err
	}

	protoProtects := make([]*proto.PerforceProtect, len(protects))
	for i, p := range protects {
		protoProtects[i] = p.ToProto()
	}

	return &proto.PerforceProtectsForDepotResponse{
		Protects: protoProtects,
	}, nil
}

func (gs *grpcServer) PerforceGroupMembers(ctx context.Context, req *proto.PerforceGroupMembersRequest) (*proto.PerforceGroupMembersResponse, error) {
	conn := req.GetConnectionDetails()
	err := perforce.P4TestWithTrust(ctx, gs.fs, perforce.P4TestWithTrustArguments{
		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, status.FromContextError(ctxErr).Err()
		}

		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	accesslog.Record(
		ctx,
		"<no-repo>",
		log.String("p4user", conn.GetP4User()),
		log.String("p4port", conn.GetP4Port()),
	)

	args := perforce.P4GroupMembersArguments{
		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),

		Group: req.GetGroup(),
	}

	members, err := perforce.P4GroupMembers(ctx, gs.fs, args)
	if err != nil {
		return nil, err
	}

	return &proto.PerforceGroupMembersResponse{
		Usernames: members,
	}, nil
}

func (gs *grpcServer) IsPerforceSuperUser(ctx context.Context, req *proto.IsPerforceSuperUserRequest) (*proto.IsPerforceSuperUserResponse, error) {
	conn := req.GetConnectionDetails()
	err := perforce.P4TestWithTrust(ctx, gs.fs, perforce.P4TestWithTrustArguments{
		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, status.FromContextError(ctxErr).Err()
		}

		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	err = perforce.P4UserIsSuperUser(ctx, gs.fs, perforce.P4UserIsSuperUserArguments{
		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, status.FromContextError(ctxErr).Err()
		}

		if err == perforce.ErrIsNotSuperUser {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		return nil, status.Error(codes.Internal, err.Error())
	}

	return &proto.IsPerforceSuperUserResponse{}, nil
}

func (gs *grpcServer) PerforceGetChangelist(ctx context.Context, req *proto.PerforceGetChangelistRequest) (*proto.PerforceGetChangelistResponse, error) {
	conn := req.GetConnectionDetails()
	err := perforce.P4TestWithTrust(ctx, gs.fs, perforce.P4TestWithTrustArguments{
		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, status.FromContextError(ctxErr).Err()
		}

		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	accesslog.Record(
		ctx,
		"<no-repo>",
		log.String("p4user", conn.GetP4User()),
		log.String("p4port", conn.GetP4Port()),
	)

	changelist, err := perforce.GetChangelistByID(ctx, gs.fs, perforce.GetChangeListByIDArguments{
		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),

		ChangelistID: req.GetChangelistId(),
	})
	if err != nil {
		return nil, err
	}

	return &proto.PerforceGetChangelistResponse{
		Changelist: changelist.ToProto(),
	}, nil
}

func byteSlicesToStrings(in [][]byte) []string {
	res := make([]string, len(in))
	for i, b := range in {
		res[i] = string(b)
	}
	return res
}

func (gs *grpcServer) MergeBase(ctx context.Context, req *proto.MergeBaseRequest) (*proto.MergeBaseResponse, error) {
	accesslog.Record(
		ctx,
		req.GetRepoName(),
		log.String("base", string(req.GetBase())),
		log.String("head", string(req.GetHead())),
	)

	if req.GetRepoName() == "" {
		return nil, status.New(codes.InvalidArgument, "repo must be specified").Err()
	}

	if len(req.GetBase()) == 0 {
		return nil, status.New(codes.InvalidArgument, "base must be specified").Err()
	}

	if len(req.GetHead()) == 0 {
		return nil, status.New(codes.InvalidArgument, "head must be specified").Err()
	}

	repoName := api.RepoName(req.GetRepoName())
	repoDir := gs.fs.RepoDir(repoName)

	if err := gs.maybeStartClone(ctx, repoName); err != nil {
		return nil, err
	}

	backend := gs.getBackendFunc(repoDir, repoName)

	sha, err := backend.MergeBase(ctx, string(req.GetBase()), string(req.GetHead()))
	if err != nil {
		var e *gitdomain.RevisionNotFoundError
		if errors.As(err, &e) {
			s, err := status.New(codes.NotFound, "revision not found").WithDetails(&proto.RevisionNotFoundPayload{
				Repo: req.GetRepoName(),
				Spec: e.Spec,
			})
			if err != nil {
				return nil, err
			}
			return nil, s.Err()
		}

		gs.svc.LogIfCorrupt(ctx, repoName, err)
		// TODO: Better error checking.
		return nil, err
	}

	return &proto.MergeBaseResponse{
		MergeBaseCommitSha: string(sha),
	}, nil
}

func (gs *grpcServer) GetCommit(ctx context.Context, req *proto.GetCommitRequest) (*proto.GetCommitResponse, error) {
	accesslog.Record(
		ctx,
		req.GetRepoName(),
		log.String("commit", req.GetCommit()),
	)

	if req.GetRepoName() == "" {
		return nil, status.New(codes.InvalidArgument, "repo must be specified").Err()
	}

	if req.GetCommit() == "" {
		return nil, status.New(codes.InvalidArgument, "commit must be specified").Err()
	}

	repoName := api.RepoName(req.GetRepoName())
	repoDir := gs.fs.RepoDir(repoName)

	if err := gs.maybeStartClone(ctx, repoName); err != nil {
		return nil, err
	}

	subRepoPermsEnabled, err := authz.SubRepoEnabledForRepo(ctx, gs.subRepoChecker, repoName)
	if err != nil {
		return nil, err
	}

	backend := gs.getBackendFunc(repoDir, repoName)

	commit, err := backend.GetCommit(ctx, api.CommitID(req.GetCommit()), subRepoPermsEnabled)
	if err != nil {
		var e *gitdomain.RevisionNotFoundError
		if errors.As(err, &e) {
			s, err := status.New(codes.NotFound, "revision not found").WithDetails(&proto.RevisionNotFoundPayload{
				Repo: req.GetRepoName(),
				Spec: e.Spec,
			})
			if err != nil {
				return nil, err
			}
			return nil, s.Err()
		}
		gs.svc.LogIfCorrupt(ctx, repoName, err)
		// TODO: Better error checking.
		return nil, err
	}

	hasAccess, err := hasAccessToCommit(ctx, repoName, commit.ModifiedFiles, gs.subRepoChecker)
	if err != nil {
		return nil, err
	}

	if !hasAccess {
		s, err := status.New(codes.NotFound, "revision not found").WithDetails(&proto.RevisionNotFoundPayload{
			Repo: req.GetRepoName(),
			Spec: req.GetCommit(),
		})
		if err != nil {
			return nil, err
		}
		return nil, s.Err()
	}

	return &proto.GetCommitResponse{
		Commit: commit.ToProto(),
	}, nil
}

func (gs *grpcServer) Blame(req *proto.BlameRequest, ss proto.GitserverService_BlameServer) error {
	ctx := ss.Context()

	accesslog.Record(
		ctx,
		req.GetRepoName(),
		log.String("path", req.GetPath()),
		log.String("commit", req.GetCommit()),
	)

	if req.GetRepoName() == "" {
		return status.New(codes.InvalidArgument, "repo must be specified").Err()
	}

	if req.GetCommit() == "" {
		return status.New(codes.InvalidArgument, "commit must be specified").Err()
	}

	if len(req.GetPath()) == 0 {
		return status.New(codes.InvalidArgument, "path must be specified").Err()
	}

	repoName := api.RepoName(req.GetRepoName())
	repoDir := gs.fs.RepoDir(repoName)

	if err := gs.maybeStartClone(ctx, repoName); err != nil {
		return err
	}

	// First, verify that the actor has access to the given path.
	hasAccess, err := authz.FilterActorPath(ctx, gs.subRepoChecker, actor.FromContext(ctx), repoName, req.GetPath())
	if err != nil {
		return err
	}
	if !hasAccess {
		up := &proto.UnauthorizedPayload{
			RepoName: req.GetRepoName(),
			Commit:   pointers.Ptr(req.GetCommit()),
			Path:     pointers.Ptr(req.GetPath()),
		}

		s, marshalErr := status.New(codes.PermissionDenied, "no access to path").WithDetails(up)
		if marshalErr != nil {
			gs.logger.Error("failed to marshal error", log.Error(marshalErr))
			return err
		}
		return s.Err()
	}

	backend := gs.getBackendFunc(repoDir, repoName)

	opts := git.BlameOptions{
		IgnoreWhitespace: req.GetIgnoreWhitespace(),
	}

	if r := req.GetRange(); r != nil {
		opts.Range = &git.BlameRange{
			StartLine: int(r.GetStartLine()),
			EndLine:   int(r.GetEndLine()),
		}
	}

	r, err := backend.Blame(ctx, api.CommitID(req.GetCommit()), req.GetPath(), opts)
	if err != nil {
		if os.IsNotExist(err) {
			s, err := status.New(codes.NotFound, "file not found").WithDetails(&proto.FileNotFoundPayload{
				Repo:   req.GetRepoName(),
				Commit: req.GetCommit(),
				Path:   req.GetPath(),
			})
			if err != nil {
				return err
			}
			return s.Err()
		}
		var e *gitdomain.RevisionNotFoundError
		if errors.As(err, &e) {
			s, err := status.New(codes.NotFound, "revision not found").WithDetails(&proto.RevisionNotFoundPayload{
				Repo: req.GetRepoName(),
				Spec: e.Spec,
			})
			if err != nil {
				return err
			}
			return s.Err()
		}
		gs.svc.LogIfCorrupt(ctx, repoName, err)
		// TODO: Better error checking.
		return err
	}
	defer r.Close()

	for {
		h, err := r.Read()
		if err != nil {
			// Check if we're done yet.
			if err == io.EOF {
				return nil
			}
			gs.svc.LogIfCorrupt(ctx, repoName, err)
			return err
		}
		if err := ss.Send(&proto.BlameResponse{
			Hunk: h.ToProto(),
		}); err != nil {
			return err
		}
	}
}

func (gs *grpcServer) DefaultBranch(ctx context.Context, req *proto.DefaultBranchRequest) (*proto.DefaultBranchResponse, error) {
	accesslog.Record(
		ctx,
		req.GetRepoName(),
		log.Bool("short", req.GetShortRef()),
	)

	if req.GetRepoName() == "" {
		return nil, status.New(codes.InvalidArgument, "repo must be specified").Err()
	}

	repoName := api.RepoName(req.GetRepoName())
	repoDir := gs.fs.RepoDir(repoName)

	if err := gs.maybeStartClone(ctx, repoName); err != nil {
		return nil, err
	}

	backend := gs.getBackendFunc(repoDir, repoName)

	refName, err := backend.SymbolicRefHead(ctx, req.GetShortRef())
	if err != nil {
		gs.svc.LogIfCorrupt(ctx, repoName, err)
		// TODO: Better error checking.
		return nil, err
	}

	sha, err := backend.RevParseHead(ctx)
	if err != nil {
		var e *gitdomain.RevisionNotFoundError
		if errors.As(err, &e) {
			s, err := status.New(codes.NotFound, "revision not found").WithDetails(&proto.RevisionNotFoundPayload{
				Repo: req.GetRepoName(),
				Spec: e.Spec,
			})
			if err != nil {
				return nil, err
			}
			return nil, s.Err()
		}
		gs.svc.LogIfCorrupt(ctx, repoName, err)
		return nil, err
	}

	return &proto.DefaultBranchResponse{
		RefName: refName,
		Commit:  string(sha),
	}, nil
}

func (gs *grpcServer) ReadFile(req *proto.ReadFileRequest, ss proto.GitserverService_ReadFileServer) error {
	ctx := ss.Context()

	accesslog.Record(
		ctx,
		req.GetRepoName(),
		log.String("commit", req.GetCommit()),
		log.String("path", req.GetPath()),
	)

	if req.GetRepoName() == "" {
		return status.New(codes.InvalidArgument, "repo must be specified").Err()
	}

	if len(req.GetPath()) == 0 {
		return status.New(codes.InvalidArgument, "path must be specified").Err()
	}

	if len(req.GetCommit()) == 0 {
		return status.New(codes.InvalidArgument, "commit must be specified").Err()
	}

	repoName := api.RepoName(req.GetRepoName())
	repoDir := gs.fs.RepoDir(repoName)

	if err := gs.maybeStartClone(ctx, repoName); err != nil {
		return err
	}

	// First, verify that the actor has access to the given path.
	hasAccess, err := authz.FilterActorPath(ctx, gs.subRepoChecker, actor.FromContext(ctx), repoName, req.GetPath())
	if err != nil {
		return err
	}
	if !hasAccess {
		up := &proto.UnauthorizedPayload{
			RepoName: req.GetRepoName(),
			Path:     pointers.Ptr(req.GetPath()),
		}
		if c := req.GetCommit(); c != "" {
			up.Commit = &c
		}
		s, marshalErr := status.New(codes.PermissionDenied, "no access to path").WithDetails(up)
		if marshalErr != nil {
			gs.logger.Error("failed to marshal error", log.Error(marshalErr))
			return err
		}
		return s.Err()
	}

	backend := gs.getBackendFunc(repoDir, repoName)

	r, err := backend.ReadFile(ctx, api.CommitID(req.GetCommit()), req.GetPath())
	if err != nil {
		if os.IsNotExist(err) {
			s, err := status.New(codes.NotFound, "file not found").WithDetails(&proto.FileNotFoundPayload{
				Repo:   req.GetRepoName(),
				Commit: req.GetCommit(),
				Path:   req.GetPath(),
			})
			if err != nil {
				return err
			}
			return s.Err()
		}
		var e *gitdomain.RevisionNotFoundError
		if errors.As(err, &e) {
			s, err := status.New(codes.NotFound, "revision not found").WithDetails(&proto.RevisionNotFoundPayload{
				Repo: req.GetRepoName(),
				Spec: e.Spec,
			})
			if err != nil {
				return err
			}
			return s.Err()
		}
		gs.svc.LogIfCorrupt(ctx, repoName, err)
		// TODO: Better error checking.
		return err
	}
	defer r.Close()

	w := streamio.NewWriter(func(p []byte) error {
		return ss.Send(&proto.ReadFileResponse{Data: p})
	})

	_, err = io.Copy(w, r)
	return err
}

func (gs *grpcServer) ResolveRevision(ctx context.Context, req *proto.ResolveRevisionRequest) (*proto.ResolveRevisionResponse, error) {
	accesslog.Record(
		ctx,
		req.GetRepoName(),
		log.String("revspec", string(req.GetRevSpec())),
	)

	if req.GetRepoName() == "" {
		return nil, status.New(codes.InvalidArgument, "repo must be specified").Err()
	}

	repoName := api.RepoName(req.GetRepoName())
	repoDir := gs.fs.RepoDir(repoName)

	if err := gs.maybeStartClone(ctx, repoName); err != nil {
		return nil, err
	}

	revspec := string(req.GetRevSpec())

	backend := gs.getBackendFunc(repoDir, repoName)

	// First, try to resolve the revspec.
	sha, err := backend.ResolveRevision(ctx, revspec)
	if err != nil {
		// If that fails to resolve the revspec, try to ensure the revision exists,
		// if requested by the caller.
		if req.GetEnsureRevision() && errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
			// We ensured the revision exists, so try to resolve it again.
			if gs.svc.EnsureRevision(ctx, repoName, revspec) {
				sha, err = backend.ResolveRevision(ctx, revspec)
			}
		}
	}

	if err != nil {
		var e *gitdomain.RevisionNotFoundError
		if errors.As(err, &e) {
			s, err := status.New(codes.NotFound, "revision not found").WithDetails(&proto.RevisionNotFoundPayload{
				Repo: req.GetRepoName(),
				Spec: e.Spec,
			})
			if err != nil {
				return nil, err
			}
			return nil, s.Err()
		}

		gs.svc.LogIfCorrupt(ctx, repoName, err)
		// TODO: Better error checking.
		return nil, err
	}

	return &proto.ResolveRevisionResponse{
		CommitSha: string(sha),
	}, nil
}

func (gs *grpcServer) maybeStartClone(ctx context.Context, repo api.RepoName) error {
	cloned, state, err := gs.svc.MaybeStartClone(ctx, repo)
	if err != nil {
		return status.New(codes.Internal, "failed to check if repo is cloned").Err()
	}

	if cloned {
		return nil
	}

	return newRepoNotFoundError(repo, state.CloneInProgress, state.CloneProgress)
}

func hasAccessToCommit(ctx context.Context, repoName api.RepoName, files []string, checker authz.SubRepoPermissionChecker) (bool, error) {
	if len(files) == 0 {
		return true, nil // If commit has no files, assume user has access to view the commit.
	}

	if enabled, err := authz.SubRepoEnabledForRepo(ctx, checker, repoName); err != nil {
		return false, err
	} else if !enabled {
		return true, nil
	}

	a := actor.FromContext(ctx)
	for _, fileName := range files {
		if hasAccess, err := authz.FilterActorPath(ctx, checker, a, repoName, fileName); err != nil {
			return false, err
		} else if hasAccess {
			// if the user has access to one file modified in the commit, they have access to view the commit
			return true, nil
		}
	}
	return false, nil
}

func newRepoNotFoundError(repo api.RepoName, cloneInProgress bool, cloneProgress string) error {
	s, err := status.New(codes.NotFound, "repo not found").WithDetails(&proto.RepoNotFoundPayload{
		CloneInProgress: cloneInProgress,
		CloneProgress:   cloneProgress,
		Repo:            string(repo),
	})
	if err != nil {
		return err
	}
	return s.Err()
}
