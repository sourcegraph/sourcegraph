package repos

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	awscredentials "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/codecommit"
	"golang.org/x/net/http2"

	"github.com/sourcegraph/sourcegraph/lib/errors"

	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/awscodecommit"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

// An AWSCodeCommitSource yields repositories from a single AWS Code Commit
// connection configured in Sourcegraph via the external services
// configuration.
type AWSCodeCommitSource struct {
	svc    *types.ExternalService
	config *schema.AWSCodeCommitConnection

	awsPartition string // "aws", "aws-cn", "aws-us-gov"
	awsRegion    string
	client       *awscodecommit.Client

	excluder repoExcluder
}

// NewAWSCodeCommitSource returns a new AWSCodeCommitSource from the given external service.
func NewAWSCodeCommitSource(ctx context.Context, svc *types.ExternalService, cf *httpcli.Factory) (*AWSCodeCommitSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.AWSCodeCommitConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	return newAWSCodeCommitSource(svc, &c, cf)
}

func newAWSCodeCommitSource(svc *types.ExternalService, c *schema.AWSCodeCommitConnection, cf *httpcli.Factory) (*AWSCodeCommitSource, error) {
	if cf == nil {
		cf = httpcli.ExternalClientFactory
	}

	cli, err := cf.Doer(func(c *http.Client) error {
		tr := awshttp.NewBuildableClient().GetTransport()
		if err := http2.ConfigureTransport(tr); err != nil {
			return err
		}
		c.Transport = tr
		wrapWithoutRedirect(c)

		return nil
	})
	if err != nil {
		return nil, err
	}

	awsConfig, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(c.Region),
		config.WithCredentialsProvider(
			awscredentials.StaticCredentialsProvider{
				Value: aws.Credentials{
					AccessKeyID:     c.AccessKeyID,
					SecretAccessKey: c.SecretAccessKey,
					Source:          "sourcegraph-site-configuration",
				},
			},
		),
		config.WithHTTPClient(cli),
	)
	if err != nil {
		return nil, err
	}

	var ex repoExcluder
	for _, r := range c.Exclude {
		// Either Name OR ID must match.
		ex.AddRule(NewRule().
			Exact(r.Name).
			Exact(r.Id))
	}
	if err := ex.RuleErrors(); err != nil {
		return nil, err
	}

	s := &AWSCodeCommitSource{
		svc:      svc,
		config:   c,
		excluder: ex,
		client:   awscodecommit.NewClient(awsConfig),
	}

	endpoint, err := codecommit.NewDefaultEndpointResolver().ResolveEndpoint(c.Region, codecommit.EndpointResolverOptions{})
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to resolve AWS region %q", c.Region))
	}
	s.awsPartition = endpoint.PartitionID
	s.awsRegion = endpoint.SigningRegion

	return s, nil
}

// CheckConnection at this point assumes availability and relies on errors returned
// from the subsequent calls. This is going to be expanded as part of issue #44683
// to actually only return true if the source can serve requests.
func (s *AWSCodeCommitSource) CheckConnection(ctx context.Context) error {
	return nil
}

// ListRepos returns all AWS Code Commit repositories accessible to all
// connections configured in Sourcegraph via the external services
// configuration.
func (s *AWSCodeCommitSource) ListRepos(ctx context.Context, results chan SourceResult) {
	s.listAllRepositories(ctx, results)
}

// ExternalServices returns a singleton slice containing the external service.
func (s *AWSCodeCommitSource) ExternalServices() types.ExternalServices {
	return types.ExternalServices{s.svc}
}

func (s *AWSCodeCommitSource) makeRepo(r *awscodecommit.Repository) *types.Repo {
	urn := s.svc.URN()
	serviceID := awscodecommit.ServiceID(s.awsPartition, s.awsRegion, r.AccountID)

	return &types.Repo{
		Name:         reposource.AWSRepoName(s.config.RepositoryPathPattern, r.Name),
		URI:          string(reposource.AWSRepoName("", r.Name)),
		ExternalRepo: awscodecommit.ExternalRepoSpec(r, serviceID),
		Description:  r.Description,
		Sources: map[string]*types.SourceInfo{
			urn: {
				ID:       urn,
				CloneURL: r.HTTPCloneURL,
			},
		},
		Metadata: r,
		Private:  !s.svc.Unrestricted,
	}
}

func (s *AWSCodeCommitSource) listAllRepositories(ctx context.Context, results chan SourceResult) {
	var nextToken string
	for {
		batch, token, err := s.client.ListRepositories(ctx, nextToken)
		if err != nil {
			results <- SourceResult{Source: s, Err: err}
			return
		}

		for _, r := range batch {
			if !s.excludes(r) {
				repo := s.makeRepo(r)
				results <- SourceResult{Source: s, Repo: repo}
			}
		}

		if len(batch) == 0 || token == "" {
			break // last page
		}

		nextToken = token
	}
}

func (s *AWSCodeCommitSource) excludes(r *awscodecommit.Repository) bool {
	return s.excluder.ShouldExclude(r.Name) || s.excluder.ShouldExclude(r.ID)
}

// The code below is copied from
// github.com/aws/aws-sdk-go-v2@v0.11.0/aws/client.go so we use the same HTTP
// client that AWS wants to use, but fits into our HTTP factory
// pattern. Additionally we change wrapWithoutRedirect to mutate c instead of
// returning a copy.
func wrapWithoutRedirect(c *http.Client) {
	tr := c.Transport
	if tr == nil {
		tr = http.DefaultTransport
	}

	c.CheckRedirect = limitedRedirect
	c.Transport = stubBadHTTPRedirectTransport{
		tr: tr,
	}
}

func limitedRedirect(r *http.Request, _ []*http.Request) error {
	// Request.Response, in CheckRedirect is the response that is triggering
	// the redirect.
	resp := r.Response
	if r.URL.String() == stubBadHTTPRedirectLocation {
		resp.Header.Del(stubBadHTTPRedirectLocation)
		return http.ErrUseLastResponse
	}

	switch resp.StatusCode {
	case 307, 308:
		// Only allow 307 and 308 redirects as they preserve the method.
		return nil
	}

	return http.ErrUseLastResponse
}

type stubBadHTTPRedirectTransport struct {
	tr http.RoundTripper
}

var _ httpcli.WrappedTransport = &stubBadHTTPRedirectTransport{}

const stubBadHTTPRedirectLocation = `https://amazonaws.com/badhttpredirectlocation`

func (t stubBadHTTPRedirectTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.tr.RoundTrip(r)
	if err != nil {
		return resp, err
	}

	// TODO S3 is the only known service to return 301 without location header.
	// consider moving this to a S3 customization.
	switch resp.StatusCode {
	case 301, 302:
		if v := resp.Header.Get("Location"); len(v) == 0 {
			resp.Header.Set("Location", stubBadHTTPRedirectLocation)
		}
	}

	return resp, err
}

func (t stubBadHTTPRedirectTransport) Unwrap() *http.RoundTripper { return &t.tr }
