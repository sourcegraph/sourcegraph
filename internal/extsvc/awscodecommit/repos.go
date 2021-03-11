package awscodecommit

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/codecommit"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Repository is an AWS CodeCommit repository.
type Repository struct {
	ARN          string     // the ARN (Amazon Resource Name) of the repository
	AccountID    string     // the ID of the AWS account associated with the repository
	ID           string     // the ID of the repository
	Name         string     // the name of the repository
	Description  string     // the description of the repository
	HTTPCloneURL string     // the HTTP(S) clone URL of the repository
	LastModified *time.Time // the last modified date of the repository
}

func (c *Client) repositoryCacheKey(ctx context.Context, arn string) (string, error) {
	key, err := c.cacheKeyPrefix(ctx)
	if err != nil {
		return "", err
	}
	return key + ":" + arn, nil
}

// GetRepositoryMock is set by tests to mock (*Client).GetRepository.
var GetRepositoryMock func(ctx context.Context, arn string) (*Repository, error)

// MockGetRepository_Return is called by tests to mock (*Client).GetRepository.
func MockGetRepository_Return(returns *Repository) {
	GetRepositoryMock = func(context.Context, string) (*Repository, error) {
		return returns, nil
	}
}

// GetRepository gets a repository from AWS CodeCommit by ARN (Amazon Resource Name).
func (c *Client) GetRepository(ctx context.Context, arn string) (*Repository, error) {
	if GetRepositoryMock != nil {
		return GetRepositoryMock(ctx, arn)
	}
	r, err := c.cachedGetRepository(ctx, arn)
	if err != nil {
		return r, &wrappedError{err: err}
	}
	return r, nil
}

// cachedGetRepository caches the getRepositoryFromAPI call.
func (c *Client) cachedGetRepository(ctx context.Context, arn string) (*Repository, error) {
	key, err := c.repositoryCacheKey(ctx, arn)
	if err != nil {
		return nil, err
	}

	if cached := c.getRepositoryFromCache(ctx, key); cached != nil {
		reposCacheCounter.WithLabelValues("hit").Inc()
		if cached.NotFound {
			return nil, ErrNotFound
		}
		return &cached.Repository, nil
	}

	repo, err := c.getRepositoryFromAPI(ctx, arn)
	if IsNotFound(err) {
		// Before we do anything, ensure we cache NotFound responses.
		c.addRepositoryToCache(key, &cachedRepo{NotFound: true})
		reposCacheCounter.WithLabelValues("notfound").Inc()
	}
	if err != nil {
		reposCacheCounter.WithLabelValues("error").Inc()
		return nil, err
	}

	c.addRepositoryToCache(key, &cachedRepo{Repository: *repo})
	reposCacheCounter.WithLabelValues("miss").Inc()

	return repo, nil
}

var reposCacheCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "src_repos_awscodecommit_cache_hit",
	Help: "Counts cache hits and misses for AWS CodeCommit repo metadata.",
}, []string{"type"})

type cachedRepo struct {
	Repository

	// NotFound indicates that the AWS CodeCommit API reported that the repository was not
	// found.
	NotFound bool
}

// getRepositoryFromCache attempts to get a response from the redis cache.
// It returns nil error for cache-hit condition and non-nil error for cache-miss.
func (c *Client) getRepositoryFromCache(ctx context.Context, key string) *cachedRepo {
	b, ok := c.repoCache.Get(key)
	if !ok {
		return nil
	}

	var cached cachedRepo
	if err := json.Unmarshal(b, &cached); err != nil {
		return nil
	}

	return &cached
}

// addRepositoryToCache will cache the value for repo. The caller can provide multiple cache key
// for the multiple ways that this repository can be retrieved (e.g., both "owner/name" and the
// GraphQL node ID).
func (c *Client) addRepositoryToCache(key string, repo *cachedRepo) {
	b, err := json.Marshal(repo)
	if err != nil {
		return
	}
	c.repoCache.Set(strings.ToLower(key), b)
}

// getRepositoryFromAPI attempts to fetch a repository from the GitHub API without use of the redis cache.
func (c *Client) getRepositoryFromAPI(ctx context.Context, arn string) (*Repository, error) {
	// The repository name always comes after the last ":" in the ARN.
	var repoName string
	if i := strings.LastIndex(arn, ":"); i >= 0 {
		repoName = arn[i+1:]
	}

	svc := codecommit.New(c.aws)
	req := svc.GetRepositoryRequest(&codecommit.GetRepositoryInput{RepositoryName: &repoName})
	req.SetContext(ctx)
	result, err := req.Send(ctx)
	if err != nil {
		return nil, err
	}
	return fromRepoMetadata(result.RepositoryMetadata), nil
}

// We can only fetch the metadata in batches of 25 as documented here:
// https://docs.aws.amazon.com/AWSJavaSDK/latest/javadoc/com/amazonaws/services/codecommit/model/MaximumRepositoryNamesExceededException.html
const MaxMetadataBatch = 25

// ListRepositories calls the ListRepositories API method of AWS CodeCommit.
func (c *Client) ListRepositories(ctx context.Context, nextToken string) (repos []*Repository, nextNextToken string, err error) {
	defer func() {
		if err != nil {
			err = &wrappedError{err}
		}
	}()

	svc := codecommit.New(c.aws)

	// List repositories.
	listInput := codecommit.ListRepositoriesInput{
		Order:  codecommit.OrderEnumDescending,
		SortBy: codecommit.SortByEnumLastModifiedDate,
	}
	if nextToken != "" {
		listInput.NextToken = &nextToken
	}
	listReq := svc.ListRepositoriesRequest(&listInput)
	listReq.SetContext(ctx)
	listResult, err := listReq.Send(ctx)
	if err != nil {
		return nil, "", err
	}
	if listResult.NextToken != nil {
		nextNextToken = *listResult.NextToken
	}

	// Batch get the repositories to get the metadata we need (the list result doesn't
	// contain all the necessary repository metadata).
	total := len(listResult.Repositories)
	repos = make([]*Repository, 0, total)
	for i := 0; i < total; i += MaxMetadataBatch {
		j := i + MaxMetadataBatch
		if j > total {
			j = total
		}

		repositoryNames := make([]string, 0, MaxMetadataBatch)
		for _, repo := range listResult.Repositories[i:j] {
			repositoryNames = append(repositoryNames, *repo.RepositoryName)
		}

		rs, err := c.getRepositories(ctx, svc, repositoryNames)
		if err != nil {
			return nil, "", err
		}
		repos = append(repos, rs...)
	}

	return repos, nextNextToken, nil
}

func (c *Client) getRepositories(ctx context.Context, svc *codecommit.Client, repositoryNames []string) ([]*Repository, error) {
	getInput := codecommit.BatchGetRepositoriesInput{RepositoryNames: repositoryNames}
	getReq := svc.BatchGetRepositoriesRequest(&getInput)
	getReq.SetContext(ctx)
	getResult, err := getReq.Send(ctx)
	if err != nil {
		return nil, err
	}

	// Ignore getResult.RepositoriesNotFound because it would only occur in the rare case
	// of a repository being deleted right after our ListRepositories request, and in that
	// case we wouldn't want to return an error.
	repos := make([]*Repository, len(getResult.Repositories))
	for i, repo := range getResult.Repositories {
		repos[i] = fromRepoMetadata(&repo)

		key, err := c.repositoryCacheKey(ctx, *repo.Arn)
		if err != nil {
			return nil, err
		}
		c.addRepositoryToCache(key, &cachedRepo{Repository: *repos[i]})
	}
	return repos, nil
}

type wrappedError struct {
	err error
}

func (w *wrappedError) Error() string {
	if w.err != nil {
		return w.err.Error()
	}
	return ""
}

func (w *wrappedError) NotFound() bool {
	return IsNotFound(w.err)
}

func (w *wrappedError) Unauthorized() bool {
	return IsUnauthorized(w.err)
}

func fromRepoMetadata(m *codecommit.RepositoryMetadata) *Repository {
	repo := Repository{
		ARN:          *m.Arn,
		AccountID:    *m.AccountId,
		ID:           *m.RepositoryId,
		Name:         *m.RepositoryName,
		HTTPCloneURL: *m.CloneUrlHttp,
		LastModified: m.LastModifiedDate,
	}
	if m.RepositoryDescription != nil {
		repo.Description = *m.RepositoryDescription
	}
	return &repo
}
