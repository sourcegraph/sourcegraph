package proxy

import (
	"context"
	"errors"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/api"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
)

type Locator interface {
	// Locate returns the gitserver client and a modified repository for the given request.
	// The modified repository shall be passed on to the gitserver shard.
	Locate(ctx context.Context, repo *proto.GitserverRepository) (proto.GitserverServiceClient, *proto.GitserverRepository, error)
}

type ListRepo struct {
	UID  string
	Name api.RepoName
	// LastOptimizationAt records when the repository has been optimized last.
	LastOptimizationAt time.Time
	DeleteAfter        time.Time
}

type RepoLookupStore interface {
	// RepoByUID returns the repository name and path given an opaque UID string.
	// The implementation is accountable for using the same opaque ID generator
	// in the clients that call this method.
	//
	// Path must be an absolute path with no `..`. When used in a gitserver shard,
	// it is usually prepended by some data directory prefix, e.g. /data/repos.
	RepoByUID(context.Context, string) (name api.RepoName, path string, err error)
	ListRepos(ctx context.Context, pageCursor string) (repos []ListRepo, nextPage string, err error)
	SetLastOptimization(ctx context.Context, repoUID string, t time.Time) error

	// TODO: Should these exist?
	SetCloned(ctx context.Context, repoUID string) error
	SetDeleted(ctx context.Context, repoUID string) error
}

type cachedGitserverRepository struct {
	// An opaque identifier used by gitserver to uniquely identify the repository.
	Uid string `protobuf:"bytes,1,opt,name=uid,proto3" json:"uid,omitempty"`
	// The path on disk where the repository is stored.
	Path string `protobuf:"bytes,2,opt,name=path,proto3" json:"path,omitempty"`
	// name is optional and only used for logging purposes.
	Name string `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
}

type locator struct {
	cs    ClientSource
	cache map[string]cachedGitserverRepository
	store RepoLookupStore
}

func (l *locator) Locate(ctx context.Context, repo *proto.GitserverRepository) (proto.GitserverServiceClient, *proto.GitserverRepository, error) {
	if repo.GetUid() == "" {
		return nil, nil, errors.New("repo UID is empty")
	}

	if c, ok := l.cache[repo.GetUid()]; ok {
		cc, err := l.cs.ClientForRepo(ctx, api.RepoName(c.Name))
		if err != nil {
			return nil, nil, err
		}
		return cc, &proto.GitserverRepository{
			Uid: c.Uid, Path: c.Path, Name: c.Name,
		}, nil
	}

	name, path, err := l.store.RepoByUID(ctx, repo.GetUid())
	if err != nil {
		return nil, nil, err
	}

	cached := cachedGitserverRepository{
		Uid:  repo.GetUid(),
		Name: string(name),
		Path: path,
	}
	l.cache[repo.GetUid()] = cached

	cc, err := l.cs.ClientForRepo(ctx, api.RepoName(cached.Name))
	if err != nil {
		return nil, nil, err
	}
	return cc, &proto.GitserverRepository{
		Uid: cached.Uid, Path: cached.Path, Name: cached.Name,
	}, nil
}
