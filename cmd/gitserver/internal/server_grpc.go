package internal

import (
	"context"
	"io"
	"os"
	"strings"

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
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/internal/grpc/streamio"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

type service interface {
	CreateCommitFromPatch(ctx context.Context, req protocol.CreateCommitFromPatchRequest) (int, protocol.CreateCommitFromPatchResponse)
	LogIfCorrupt(context.Context, api.RepoName, error)
	Exec(ctx context.Context, req *protocol.ExecRequest, w io.Writer) (execStatus, error)
	MaybeStartClone(ctx context.Context, repo api.RepoName) (notFound *protocol.NotFoundPayload, cloned bool)
	IsRepoCloneable(ctx context.Context, repo api.RepoName) (protocol.IsRepoCloneableResponse, error)
	RepoUpdate(ctx context.Context, req *protocol.RepoUpdateRequest) protocol.RepoUpdateResponse
	CloneRepo(ctx context.Context, repo api.RepoName, opts CloneOptions) (cloneProgress string, err error)
	SearchWithObservability(ctx context.Context, tr trace.Trace, args *protocol.SearchRequest, onMatch func(*protocol.CommitMatch) error) (limitHit bool, err error)

	BatchGitLogInstrumentedHandler(ctx context.Context, req *proto.BatchLogRequest) (resp *proto.BatchLogResponse, err error)
	P4Exec(ctx context.Context, logger log.Logger, req *p4ExecRequest, w io.Writer) execStatus
}

func NewGRPCServer(server *Server) proto.GitserverServiceServer {
	return &grpcServer{
		logger:         server.Logger,
		reposDir:       server.ReposDir,
		db:             server.DB,
		hostname:       server.Hostname,
		subRepoChecker: authz.DefaultSubRepoPermsChecker,
		locker:         server.Locker,
		getBackendFunc: server.GetBackendFunc,
		svc:            server,
	}
}

type grpcServer struct {
	logger         log.Logger
	reposDir       string
	db             database.DB
	hostname       string
	subRepoChecker authz.SubRepoPermissionChecker
	locker         RepositoryLocker
	getBackendFunc Backender

	svc service

	proto.UnimplementedGitserverServiceServer
}

var _ proto.GitserverServiceServer = &grpcServer{}

func (gs *grpcServer) BatchLog(ctx context.Context, req *proto.BatchLogRequest) (*proto.BatchLogResponse, error) {
	// Validate request parameters
	if len(req.GetRepoCommits()) == 0 { //nolint:staticcheck
		return &proto.BatchLogResponse{}, nil
	}
	if !strings.HasPrefix(req.GetFormat(), "--format=") { //nolint:staticcheck
		return nil, status.Error(codes.InvalidArgument, "format parameter expected to be of the form `--format=<git log format>`")
	}

	// Handle unexpected error conditions
	resp, err := gs.svc.BatchGitLogInstrumentedHandler(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (gs *grpcServer) CreateCommitFromPatchBinary(s proto.GitserverService_CreateCommitFromPatchBinaryServer) error {
	var (
		metadata *proto.CreateCommitFromPatchBinaryRequest_Metadata
		patch    []byte
	)
	receivedMetadata := false

	for {
		msg, err := s.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		switch msg.Payload.(type) {
		case *proto.CreateCommitFromPatchBinaryRequest_Metadata_:
			if receivedMetadata {
				return status.Errorf(codes.InvalidArgument, "received metadata more than once")
			}
			metadata = msg.GetMetadata()
			receivedMetadata = true

		case *proto.CreateCommitFromPatchBinaryRequest_Patch_:
			m := msg.GetPatch()
			patch = append(patch, m.GetData()...)

		case nil:
			continue

		default:
			return status.Errorf(codes.InvalidArgument, "got malformed message %T", msg.Payload)
		}
	}

	var r protocol.CreateCommitFromPatchRequest
	r.FromProto(metadata, patch)
	_, resp := gs.svc.CreateCommitFromPatch(s.Context(), r)
	res, err := resp.ToProto()
	if err != nil {
		return err.ToStatus().Err()
	}

	return s.SendAndClose(res)
}

func (gs *grpcServer) DiskInfo(_ context.Context, _ *proto.DiskInfoRequest) (*proto.DiskInfoResponse, error) {
	return getDiskInfo(gs.reposDir)
}

func (gs *grpcServer) Exec(req *proto.ExecRequest, ss proto.GitserverService_ExecServer) error {
	internalReq := protocol.ExecRequest{
		Repo:      api.RepoName(req.GetRepo()),
		Args:      byteSlicesToStrings(req.GetArgs()),
		NoTimeout: req.GetNoTimeout(),

		// 🚨Warning🚨: There is no guarantee that EnsureRevision is a valid utf-8 string
		EnsureRevision: string(req.GetEnsureRevision()),
	}

	w := streamio.NewWriter(func(p []byte) error {
		return ss.Send(&proto.ExecResponse{
			Data: p,
		})
	})

	// Log which actor is accessing the repo.
	args := byteSlicesToStrings(req.GetArgs())
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}

	accesslog.Record(ss.Context(), req.GetRepo(),
		log.String("cmd", cmd),
		log.Strings("args", args),
	)

	return gs.doExec(ss.Context(), &internalReq, w)
}

func (gs *grpcServer) Archive(req *proto.ArchiveRequest, ss proto.GitserverService_ArchiveServer) error {
	// Log which which actor is accessing the repo.
	accesslog.Record(ss.Context(), req.GetRepo(),
		log.String("treeish", req.GetTreeish()),
		log.String("format", req.GetFormat()),
		log.Strings("path", req.GetPathspecs()),
	)

	if err := git.CheckSpecArgSafety(req.GetTreeish()); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	if req.GetRepo() == "" || req.GetFormat() == "" {
		return status.Error(codes.InvalidArgument, "empty repo or format")
	}

	execReq := &protocol.ExecRequest{
		Repo: api.RepoName(req.GetRepo()),
		Args: []string{
			"archive",
			"--worktree-attributes",
			"--format=" + req.GetFormat(),
		},
	}

	if req.GetFormat() == string(gitserver.ArchiveFormatZip) {
		execReq.Args = append(execReq.Args, "-0")
	}

	execReq.Args = append(execReq.Args, req.GetTreeish(), "--")
	execReq.Args = append(execReq.Args, req.GetPathspecs()...)

	// Archives can be huge, to make up a little of the overhead on transmit,
	// we use a chunk size of 2MB. This effectively reduces the number of messages
	// sent by a factor of 16 vs the default for larger archives.
	w := streamio.NewWriterWithChunkSize(2*1024*1024, func(p []byte) error {
		return ss.Send(&proto.ArchiveResponse{
			Data: p,
		})
	})

	// This is a long time, but this never blocks a user request for this
	// long. Even repos that are not that large can take a long time, for
	// example a search over all repos in an organization may have several
	// large repos. All of those repos will be competing for IO => we need
	// a larger timeout.
	ctx, cancel := context.WithTimeout(ss.Context(), conf.GitLongCommandTimeout())
	defer cancel()

	return gs.doExec(ctx, execReq, w)
}

// doExec executes the given git command and streams the output to the given writer.
//
// Note: This function wraps the underlying exec implementation and returns grpc specific error handling.
func (gs *grpcServer) doExec(ctx context.Context, req *protocol.ExecRequest, w io.Writer) error {
	execStatus, err := gs.svc.Exec(ctx, req, w)
	if err != nil {
		if v := (&NotFoundError{}); errors.As(err, &v) {
			s, err := status.New(codes.NotFound, "repo not found").WithDetails(&proto.RepoNotFoundPayload{
				Repo:            string(req.Repo),
				CloneInProgress: v.Payload.CloneInProgress,
				CloneProgress:   v.Payload.CloneProgress,
			})
			if err != nil {
				gs.logger.Error("failed to marshal status", log.Error(err))
				return err
			}
			return s.Err()
		} else if errors.Is(err, gitcli.ErrBadGitCommand) {
			return status.New(codes.InvalidArgument, "invalid command").Err()
		} else if ctxErr := ctx.Err(); ctxErr != nil {
			return status.FromContextError(ctxErr).Err()
		}

		return err
	}

	if execStatus.ExitStatus != 0 || execStatus.Err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return status.FromContextError(ctxErr).Err()
		}

		gRPCStatus := codes.Unknown
		if execStatus.Err != nil && strings.Contains(execStatus.Err.Error(), "signal: killed") {
			gRPCStatus = codes.Aborted
		}

		var errString string
		if execStatus.Err != nil {
			errString = execStatus.Err.Error()
		}
		s, err := status.New(gRPCStatus, errString).WithDetails(&proto.ExecStatusPayload{
			StatusCode: int32(execStatus.ExitStatus),
			Stderr:     execStatus.Stderr,
		})
		if err != nil {
			gs.logger.Error("failed to marshal status", log.Error(err))
			return err
		}
		return s.Err()
	}

	return nil

}

func (gs *grpcServer) GetObject(ctx context.Context, req *proto.GetObjectRequest) (*proto.GetObjectResponse, error) {
	repoName := api.RepoName(req.GetRepo())
	repoDir := gitserverfs.RepoDirFromName(gs.reposDir, repoName)

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

	// Ensure that the repo is cloned and if not start a background clone, then
	// return a well-known NotFound payload error.
	if notFoundPayload, cloned := gs.svc.MaybeStartClone(ss.Context(), repoName); !cloned {
		s, err := status.New(codes.NotFound, "repo not cloned").WithDetails(&proto.RepoNotFoundPayload{
			CloneInProgress: notFoundPayload.CloneInProgress,
			CloneProgress:   notFoundPayload.CloneProgress,
			Repo:            req.GetRepo(),
		})
		if err != nil {
			return err
		}
		return s.Err()
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

func (gs *grpcServer) RepoClone(ctx context.Context, in *proto.RepoCloneRequest) (*proto.RepoCloneResponse, error) {
	repo := protocol.NormalizeRepo(api.RepoName(in.GetRepo()))

	if _, err := gs.svc.CloneRepo(ctx, repo, CloneOptions{Block: false}); err != nil {

		return &proto.RepoCloneResponse{Error: err.Error()}, nil
	}

	return &proto.RepoCloneResponse{Error: ""}, nil
}

func (gs *grpcServer) RepoCloneProgress(_ context.Context, req *proto.RepoCloneProgressRequest) (*proto.RepoCloneProgressResponse, error) {
	repositories := req.GetRepos()

	resp := protocol.RepoCloneProgressResponse{
		Results: make(map[api.RepoName]*protocol.RepoCloneProgress, len(repositories)),
	}
	for _, repo := range repositories {
		repoName := api.RepoName(repo)
		result := repoCloneProgress(gs.reposDir, gs.locker, repoName)
		resp.Results[repoName] = result
	}
	return resp.ToProto(), nil
}

func (gs *grpcServer) RepoDelete(ctx context.Context, req *proto.RepoDeleteRequest) (*proto.RepoDeleteResponse, error) {
	repo := req.GetRepo()

	if err := deleteRepo(ctx, gs.logger, gs.db, gs.hostname, gs.reposDir, api.RepoName(repo)); err != nil {
		gs.logger.Error("failed to delete repository", log.String("repo", repo), log.Error(err))
		return &proto.RepoDeleteResponse{}, status.Errorf(codes.Internal, "failed to delete repository %s: %s", repo, err)
	}
	gs.logger.Info("deleted repository", log.String("repo", repo))
	return &proto.RepoDeleteResponse{}, nil
}

func (gs *grpcServer) RepoUpdate(ctx context.Context, req *proto.RepoUpdateRequest) (*proto.RepoUpdateResponse, error) {
	var in protocol.RepoUpdateRequest
	in.FromProto(req)
	grpcResp := gs.svc.RepoUpdate(ctx, &in)

	return grpcResp.ToProto(), nil
}

func (gs *grpcServer) IsRepoCloneable(ctx context.Context, req *proto.IsRepoCloneableRequest) (*proto.IsRepoCloneableResponse, error) {
	repo := api.RepoName(req.GetRepo())

	if req.Repo == "" {
		return nil, status.Error(codes.InvalidArgument, "no Repo given")
	}

	resp, err := gs.svc.IsRepoCloneable(ctx, repo)
	if err != nil {
		return nil, err
	}

	return resp.ToProto(), nil
}

func (gs *grpcServer) IsPerforcePathCloneable(ctx context.Context, req *proto.IsPerforcePathCloneableRequest) (*proto.IsPerforcePathCloneableResponse, error) {
	if req.DepotPath == "" {
		return nil, status.Error(codes.InvalidArgument, "no DepotPath given")
	}

	p4home, err := gitserverfs.MakeP4HomeDir(gs.reposDir)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	conn := req.GetConnectionDetails()
	err = perforce.IsDepotPathCloneable(ctx, perforce.IsDepotPathCloneableArguments{
		ReposDir: gs.reposDir,
		P4Home:   p4home,

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
	p4home, err := gitserverfs.MakeP4HomeDir(gs.reposDir)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	conn := req.GetConnectionDetails()
	err = perforce.P4TestWithTrust(ctx, perforce.P4TestWithTrustArguments{
		ReposDir: gs.reposDir,
		P4Home:   p4home,

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
	p4home, err := gitserverfs.MakeP4HomeDir(gs.reposDir)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	conn := req.GetConnectionDetails()
	err = perforce.P4TestWithTrust(ctx, perforce.P4TestWithTrustArguments{
		ReposDir: gs.reposDir,
		P4Home:   p4home,

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

	users, err := perforce.P4Users(ctx, perforce.P4UsersArguments{
		ReposDir: gs.reposDir,
		P4Home:   p4home,

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
	p4home, err := gitserverfs.MakeP4HomeDir(gs.reposDir)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	conn := req.GetConnectionDetails()
	err = perforce.P4TestWithTrust(ctx, perforce.P4TestWithTrustArguments{
		ReposDir: gs.reposDir,
		P4Home:   p4home,

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
		ReposDir: gs.reposDir,
		P4Home:   p4home,

		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),

		Username: req.GetUsername(),
	}
	protects, err := perforce.P4ProtectsForUser(ctx, args)
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
	p4home, err := gitserverfs.MakeP4HomeDir(gs.reposDir)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	conn := req.GetConnectionDetails()
	err = perforce.P4TestWithTrust(ctx, perforce.P4TestWithTrustArguments{
		ReposDir: gs.reposDir,
		P4Home:   p4home,

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

	protects, err := perforce.P4ProtectsForDepot(ctx, perforce.P4ProtectsForDepotArguments{
		ReposDir: gs.reposDir,
		P4Home:   p4home,
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
	p4home, err := gitserverfs.MakeP4HomeDir(gs.reposDir)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	conn := req.GetConnectionDetails()
	err = perforce.P4TestWithTrust(ctx, perforce.P4TestWithTrustArguments{
		ReposDir: gs.reposDir,
		P4Home:   p4home,

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
		ReposDir: gs.reposDir,
		P4Home:   p4home,

		P4Port:   conn.GetP4Port(),
		P4User:   conn.GetP4User(),
		P4Passwd: conn.GetP4Passwd(),

		Group: req.GetGroup(),
	}

	members, err := perforce.P4GroupMembers(ctx, args)
	if err != nil {
		return nil, err
	}

	return &proto.PerforceGroupMembersResponse{
		Usernames: members,
	}, nil
}

func (gs *grpcServer) IsPerforceSuperUser(ctx context.Context, req *proto.IsPerforceSuperUserRequest) (*proto.IsPerforceSuperUserResponse, error) {
	p4home, err := gitserverfs.MakeP4HomeDir(gs.reposDir)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	conn := req.GetConnectionDetails()
	err = perforce.P4TestWithTrust(ctx, perforce.P4TestWithTrustArguments{
		ReposDir: gs.reposDir,
		P4Home:   p4home,

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

	err = perforce.P4UserIsSuperUser(ctx, perforce.P4UserIsSuperUserArguments{
		ReposDir: gs.reposDir,
		P4Home:   p4home,
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
	p4home, err := gitserverfs.MakeP4HomeDir(gs.reposDir)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	conn := req.GetConnectionDetails()
	err = perforce.P4TestWithTrust(ctx, perforce.P4TestWithTrustArguments{
		ReposDir: gs.reposDir,
		P4Home:   p4home,

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

	changelist, err := perforce.GetChangelistByID(ctx, perforce.GetChangeListByIDArguments{
		ReposDir: gs.reposDir,

		P4Home: p4home,

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
	repoDir := gitserverfs.RepoDirFromName(gs.reposDir, repoName)

	// Ensure that the repo is cloned and if not start a background clone, then
	// return a well-known NotFound payload error.
	if notFoundPayload, cloned := gs.svc.MaybeStartClone(ctx, repoName); !cloned {
		s, err := status.New(codes.NotFound, "repo not cloned").WithDetails(&proto.RepoNotFoundPayload{
			CloneInProgress: notFoundPayload.CloneInProgress,
			CloneProgress:   notFoundPayload.CloneProgress,
			Repo:            req.GetRepoName(),
		})
		if err != nil {
			return nil, err
		}
		return nil, s.Err()
	}

	// TODO: This should be included in requests where we do ensure the revision exists.
	// gs.server.ensureRevision(ctx, repoName, "THE REVISION", repoDir)

	backend := gs.getBackendFunc(repoDir, repoName)

	sha, err := backend.MergeBase(ctx, string(req.GetBase()), string(req.GetHead()))
	if err != nil {
		gs.svc.LogIfCorrupt(ctx, repoName, err)
		// TODO: Better error checking.
		return nil, err
	}

	return &proto.MergeBaseResponse{
		MergeBaseCommitSha: string(sha),
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
	repoDir := gitserverfs.RepoDirFromName(gs.reposDir, repoName)

	// Ensure that the repo is cloned and if not start a background clone, then
	// return a well-known NotFound payload error.
	if notFoundPayload, cloned := gs.svc.MaybeStartClone(ctx, repoName); !cloned {
		s, err := status.New(codes.NotFound, "repo not cloned").WithDetails(&proto.RepoNotFoundPayload{
			CloneInProgress: notFoundPayload.CloneInProgress,
			CloneProgress:   notFoundPayload.CloneProgress,
			Repo:            req.GetRepoName(),
		})
		if err != nil {
			return err
		}
		return s.Err()
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
	repoDir := gitserverfs.RepoDirFromName(gs.reposDir, repoName)

	// Ensure that the repo is cloned and if not start a background clone, then
	// return a well-known NotFound payload error.
	if notFoundPayload, cloned := gs.svc.MaybeStartClone(ctx, repoName); !cloned {
		s, err := status.New(codes.NotFound, "repo not cloned").WithDetails(&proto.RepoNotFoundPayload{
			CloneInProgress: notFoundPayload.CloneInProgress,
			CloneProgress:   notFoundPayload.CloneProgress,
			Repo:            req.GetRepoName(),
		})
		if err != nil {
			return nil, err
		}
		return nil, s.Err()
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
	repoDir := gitserverfs.RepoDirFromName(gs.reposDir, repoName)

	// Ensure that the repo is cloned and if not start a background clone, then
	// return a well-known NotFound payload error.
	if notFoundPayload, cloned := gs.svc.MaybeStartClone(ctx, repoName); !cloned {
		s, err := status.New(codes.NotFound, "repo not cloned").WithDetails(&proto.RepoNotFoundPayload{
			CloneInProgress: notFoundPayload.CloneInProgress,
			CloneProgress:   notFoundPayload.CloneProgress,
			Repo:            req.GetRepoName(),
		})
		if err != nil {
			return err
		}
		return s.Err()
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

func (gs *grpcServer) P4Exec(_ *proto.P4ExecRequest, _ proto.GitserverService_P4ExecServer) error {
	return status.Error(codes.Unimplemented, "P4Exec has been deprecated and removed")
}

// p4ExecRequest is a request to execute a p4 command with given arguments.
//
// Note that this request is deserialized by both gitserver and the frontend's
// internal proxy route and any major change to this structure will need to be
// reconciled in both places.
type p4ExecRequest struct {
	P4Port   string   `json:"p4port"`
	P4User   string   `json:"p4user"`
	P4Passwd string   `json:"p4passwd"`
	Args     []string `json:"args"`
}
