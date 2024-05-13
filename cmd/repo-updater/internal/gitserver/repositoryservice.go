package gitserver

import (
	"context"
	"fmt"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/connection"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
)

type RepositoryServiceClient interface {
	DeleteRepository(context.Context, api.RepoName) error
	FetchRepository(context.Context, api.RepoName) (lastFetched, lastChanged time.Time, err error)
}

func NewRepositoryServiceClient() RepositoryServiceClient {
	return &repositoryServiceClient{}
}

type repositoryServiceClient struct{}

func (c *repositoryServiceClient) DeleteRepository(ctx context.Context, repo api.RepoName) error {
	cc, err := c.clientForRepo(ctx, repo)
	if err != nil {
		return err
	}
	_, err = cc.DeleteRepository(ctx, &proto.DeleteRepositoryRequest{
		RepoName: string(repo),
	})
	return err
}

func (c *repositoryServiceClient) FetchRepository(ctx context.Context, repo api.RepoName) (lastFetched, lastChanged time.Time, err error) {
	cc, err := c.clientForRepo(ctx, repo)
	if err != nil {
		return lastFetched, lastChanged, err
	}
	req, err := cc.FetchRepository(ctx, &proto.FetchRepositoryRequest{
		RepoName: string(repo),
	})
	if err != nil {
		return lastFetched, lastChanged, err
	}

	lastProgress := ""

	for {
		resp, err := req.Recv()
		if err != nil {
			return lastFetched, lastChanged, err
		}

		if done := resp.GetDone(); done != nil {
			return done.GetLastFetched().AsTime(), done.GetLastChanged().AsTime(), nil
		}

		if pr := resp.GetProgress(); pr != nil {
			progress := string(pr.GetOutput())
			if progress != "" && progress != lastProgress {
				fmt.Printf("progress fetching repo %s: %s\n", repo, string(pr.GetOutput()))
			}
			lastProgress = progress
		}
	}
}

func (c *repositoryServiceClient) clientForRepo(ctx context.Context, repo api.RepoName) (proto.GitserverRepositoryServiceClient, error) {
	conn, err := connection.GlobalConns.ConnForRepo(ctx, repo)
	if err != nil {
		return nil, err
	}
	return proto.NewGitserverRepositoryServiceClient(conn), nil
}
