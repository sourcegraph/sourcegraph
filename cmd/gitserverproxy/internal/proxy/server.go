package proxy

import (
	"context"

	"github.com/sourcegraph/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
)

var conns = &atomicGitServerConns{}

func NewGRPCServer(logger log.Logger, db database.DB, store RepoLookupStore) proto.GitserverServiceServer {
	return &grpcServer{
		logger:  logger,
		db:      db,
		locator: &locator{cs: conns, cache: make(map[string]cachedGitserverRepository), store: store},
	}
}

type grpcServer struct {
	logger  log.Logger
	db      database.DB
	locator Locator

	proto.UnimplementedGitserverServiceServer
}

var _ proto.GitserverServiceServer = &grpcServer{}

func (gs *grpcServer) GetCommit(ctx context.Context, req *proto.GetCommitRequest) (*proto.GetCommitResponse, error) {
	cc, repo, err := gs.locator.Locate(ctx, req.GetRepo())
	if err != nil {
		return nil, err
	}
	req.Repo = repo
	resp, err := cc.GetCommit(ctx, req)
	// Check if the shard reported that the repo is not cloned, in that case
	// we might schedule a clone request for later.
	if err != nil {
		gs.MaybeStartClone(ctx, err)
	}
	return resp, err
}

func (gs *grpcServer) CreateCommitFromPatchBinary(ss proto.GitserverService_CreateCommitFromPatchBinaryServer) error {
	firstMsg, err := ss.Recv()
	if err != nil {
		return err
	}

	m := firstMsg.GetMetadata()
	if m == nil {
		return status.New(codes.InvalidArgument, "must send metadata event first").Err()
	}

	cc, repo, err := gs.locator.Locate(ss.Context(), m.GetRepo())
	if err != nil {
		return err
	}
	m.Repo = repo
	firstMsg.Payload = m

	cli, err := cc.CreateCommitFromPatchBinary(ss.Context())
	if err != nil {
		return err
	}
	err = cli.Send(firstMsg)
	if err != nil {
		return err
	}

	errChan := make(chan error)
	go func() {
		for {
			in, err := ss.Recv()
			if err != nil {
				errChan <- err
				return
			}
			if err := cli.Send(in); err != nil {
				errChan <- err
				return
			}
		}
	}()

	go func() {
		for {
			out, err := cli.Recv()
			if err != nil {
				errChan <- err
				return
			}
			if err := ss.Send(out); err != nil {
				errChan <- err
				return
			}
		}
	}()

	return <-errChan
}

// MaybeStartClone checks if a given repository is cloned on disk. If not, it starts
// cloning the repository in the background and returns a CloneStatus.
// Note: If disableAutoGitUpdates is set in the site config, no operation is taken and
// a NotFound error is returned.
func (s *grpcServer) MaybeStartClone(ctx context.Context, err error) (cloned bool, status CloneStatus, _ error) {
	s, ok := status.FromError(err)

	if conf.Get().DisableAutoGitUpdates {
		s.logger.Debug("not cloning on demand as DisableAutoGitUpdates is set")
		return false, CloneStatus{}, nil
	}

	cloneProgress, err := s.CloneRepo(ctx, repo, CloneOptions{})
	if err != nil {
		s.logger.Warn("error starting repo clone", log.String("repo", string(repo)), log.Error(err))
		return false, CloneStatus{}, nil
	}

	return false, CloneStatus{
		CloneInProgress: true,
		CloneProgress:   cloneProgress,
	}, nil
}
