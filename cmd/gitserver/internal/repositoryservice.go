package internal

import (
	"context"
	"sync"
	"time"

	"github.com/sourcegraph/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type GRPCRepositoryServiceConfig struct {
	ExhaustiveRequestLoggingEnabled bool
}

func NewRepositoryServiceServer(server *Server, config *GRPCRepositoryServiceConfig) proto.GitserverRepositoryServiceServer {
	var srv proto.GitserverRepositoryServiceServer = &repositoryServiceServer{
		logger:   server.logger,
		db:       server.db,
		hostname: server.hostname,
		svc:      server,
		fs:       server.fs,
		locker:   server.locker,
	}

	if config.ExhaustiveRequestLoggingEnabled {
		logger := server.logger.Scoped("gRPCRequestLogger")

		srv = &loggingRepositoryServiceServer{
			base:   srv,
			logger: logger,
		}
	}

	return srv
}

type repositoryServiceServer struct {
	logger   log.Logger
	db       database.DB
	hostname string
	fs       gitserverfs.FS
	svc      service
	locker   RepositoryLocker

	proto.UnimplementedGitserverRepositoryServiceServer
}

var _ proto.GitserverRepositoryServiceServer = &repositoryServiceServer{}

func (s *repositoryServiceServer) DeleteRepository(ctx context.Context, req *proto.DeleteRepositoryRequest) (*proto.DeleteRepositoryResponse, error) {
	if req.GetRepoName() == "" {
		return nil, status.New(codes.InvalidArgument, "repo_name must be specified").Err()
	}

	repoName := api.RepoName(req.GetRepoName())

	cloned, err := s.fs.RepoCloned(repoName)
	if err != nil {
		return nil, status.New(codes.Internal, "failed to determine clone status").Err()
	}

	if !cloned {
		return nil, status.New(codes.NotFound, "repository not found").Err()
	}

	if err := deleteRepo(ctx, s.db, s.hostname, s.fs, repoName); err != nil {
		s.logger.Error("failed to delete repository", log.String("repo", string(repoName)), log.Error(err))
		return &proto.DeleteRepositoryResponse{}, status.Errorf(codes.Internal, "failed to delete repository %s: %s", repoName, err)
	}

	s.logger.Info("repository deleted", log.String("repo", string(repoName)))

	return &proto.DeleteRepositoryResponse{}, nil
}

func (s *repositoryServiceServer) FetchRepository(req *proto.FetchRepositoryRequest, ss proto.GitserverRepositoryService_FetchRepositoryServer) error {
	if req.GetRepoName() == "" {
		return status.New(codes.InvalidArgument, "repo_name must be specified").Err()
	}

	repoName := api.RepoName(req.GetRepoName())

	var wg sync.WaitGroup
	wg.Add(1)
	done := make(chan struct{})

	go func() {
		defer wg.Done()

		for {
			select {
			case <-done:
				return
			case <-time.After(time.Second):
			}
			status, locked := s.locker.Status(repoName)
			if locked {
				err := ss.Send(&proto.FetchRepositoryResponse{
					Payload: &proto.FetchRepositoryResponse_Progress{
						Progress: &proto.FetchRepositoryResponse_FetchProgress{
							Output: []byte(status),
						},
					},
				})
				if err != nil {
					s.logger.Error("failed to send progress event", log.Error(err), log.String("repo", string(repoName)))
				}
			}
		}

	}()

	lastFetched, lastChanged, err := s.svc.FetchRepository(ss.Context(), repoName)
	if err != nil {
		return status.New(codes.Internal, errors.Wrap(err, "failed to fetch repository").Error()).Err()
	}

	close(done)
	wg.Wait()

	return ss.Send(&proto.FetchRepositoryResponse{
		Payload: &proto.FetchRepositoryResponse_Done{
			Done: &proto.FetchRepositoryResponse_FetchDone{
				LastFetched: timestamppb.New(lastFetched),
				LastChanged: timestamppb.New(lastChanged),
			},
		},
	})
}
