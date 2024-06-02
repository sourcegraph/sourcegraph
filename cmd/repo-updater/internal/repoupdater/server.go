package repoupdater

import (
	"context"
	"fmt"
	"time"

	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sourcegraph/sourcegraph/cmd/repo-updater/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches/syncer"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	proto "github.com/sourcegraph/sourcegraph/internal/repoupdater/v1"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Scheduler interface {
	UpdateOnce(id api.RepoID, name api.RepoName)
	ScheduleInfo(id api.RepoID) *protocol.RepoUpdateSchedulerInfoResult
}

// Server is a repoupdater server.
type Server struct {
	proto.UnimplementedRepoUpdaterServiceServer

	Store                 repos.Store
	Syncer                *repos.Syncer
	Logger                log.Logger
	Scheduler             Scheduler
	ChangesetSyncRegistry syncer.ChangesetSyncRegistry
}

func (s *Server) RepoUpdateSchedulerInfo(_ context.Context, req *proto.RepoUpdateSchedulerInfoRequest) (*proto.RepoUpdateSchedulerInfoResponse, error) {
	res := s.Scheduler.ScheduleInfo(api.RepoID(req.GetId()))
	return res.ToProto(), nil
}

func (s *Server) RepoLookup(ctx context.Context, req *proto.RepoLookupRequest) (result *proto.RepoLookupResponse, err error) {
	// NOTE: Internal actor is required to have full visibility of the repo table
	// 	(i.e. bypass repository authorization).
	ctx = actor.WithInternalActor(ctx)

	repoName := api.RepoName(req.GetRepo())

	// Sourcegraph.com: this is on the user path, do not block forever if codehost is
	// being bad. Ideally block before cloudflare 504s the request (1min). Other: we
	// only speak to our database, so response should be in a few ms.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tr, ctx := trace.New(ctx, "repoLookup", attribute.String("repo", string(repoName)))
	defer func() {
		s.Logger.Debug("repoLookup", log.String("result", fmt.Sprint(result)), log.Error(err))
		tr.SetError(err)
		tr.End()
	}()

	if repoName == "" {
		return nil, errors.New("Repo must be set (is blank)")
	}

	repo, err := s.Syncer.SyncRepo(ctx, repoName, true)
	if err != nil {
		if errcode.IsNotFound(err) {
			return (&protocol.RepoLookupResult{ErrorNotFound: true}).ToProto(), nil
		}
		if errcode.IsUnauthorized(err) || errcode.IsForbidden(err) {
			return (&protocol.RepoLookupResult{ErrorUnauthorized: true}).ToProto(), nil
		}
		if errcode.IsTemporary(err) {
			return (&protocol.RepoLookupResult{ErrorTemporarilyUnavailable: true}).ToProto(), nil
		}
		if errcode.IsRepoDenied(err) {
			return (&protocol.RepoLookupResult{ErrorRepoDenied: err.Error()}).ToProto(), nil
		}
		return nil, err
	}

	return (&protocol.RepoLookupResult{Repo: protocol.NewRepoInfo(repo)}).ToProto(), nil
}

func (s *Server) EnqueueRepoUpdate(ctx context.Context, req *proto.EnqueueRepoUpdateRequest) (resp *proto.EnqueueRepoUpdateResponse, err error) {
	// NOTE: Internal actor is required to have full visibility of the repo table
	// 	(i.e. bypass repository authorization).
	ctx = actor.WithInternalActor(ctx)

	tr, ctx := trace.New(ctx, "enqueueRepoUpdate", attribute.Stringer("req", req))
	defer func() {
		s.Logger.Debug("enqueueRepoUpdate", log.Object("http", log.String("resp", fmt.Sprint(resp)), log.Error(err)))
		if resp != nil {
			tr.SetAttributes(
				attribute.Int("resp.id", int(resp.Id)),
				attribute.String("resp.name", resp.Name),
			)
		}
		tr.SetError(err)
		tr.End()
	}()

	repo, err := s.Store.RepoStore().GetByName(ctx, api.RepoName(req.GetRepo()))
	if err != nil {
		return nil, errors.Wrap(err, "store.list-repos")
	}

	s.Scheduler.UpdateOnce(repo.ID, repo.Name)

	return &proto.EnqueueRepoUpdateResponse{
		Id:   int32(repo.ID),
		Name: string(repo.Name),
	}, nil
}

func (s *Server) RecloneRepository(ctx context.Context, req *proto.RecloneRepositoryRequest) (*proto.RecloneRepositoryResponse, error) {
	// NOTE: Internal actor is required to have full visibility of the repo table
	// 	(i.e. bypass repository authorization).
	ctx = actor.WithInternalActor(ctx)

	repoName := api.RepoName(req.GetRepoName())
	if repoName == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_name must be specified")
	}

	rs, err := s.Store.RepoStore().List(ctx, database.ReposListOptions{Names: []string{string(repoName)}})
	if err != nil {
		return nil, errors.Wrap(err, "store.list-repos")
	}

	if len(rs) != 1 {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("repo %q not found in store", repoName))
	}

	repo := rs[0]

	svc := gitserver.NewRepositoryServiceClient()

	if err := svc.DeleteRepository(ctx, repoName); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to delete repository %q: %s", repoName, err))
	}

	// Enqueue a reclone through scheduler.
	s.Scheduler.UpdateOnce(repo.ID, repo.Name)

	return &proto.RecloneRepositoryResponse{}, nil
}

func (s *Server) EnqueueChangesetSync(ctx context.Context, req *proto.EnqueueChangesetSyncRequest) (*proto.EnqueueChangesetSyncResponse, error) {
	// NOTE: Internal actor is required to have full visibility of the repo table
	// 	(i.e. bypass repository authorization).
	ctx = actor.WithInternalActor(ctx)

	if s.ChangesetSyncRegistry == nil {
		s.Logger.Warn("ChangesetSyncer is nil")
		return nil, status.Error(codes.Internal, "changeset syncer is not configured")
	}

	ids := req.GetIds()

	if len(ids) == 0 {
		return nil, nil
	}

	return &proto.EnqueueChangesetSyncResponse{}, s.ChangesetSyncRegistry.EnqueueChangesetSyncs(ctx, ids)
}
