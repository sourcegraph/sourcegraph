package repos

import (
	"context"
	"fmt"

	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
	"github.com/sourcegraph/sourcegraph/schema"
)

// A GitLabSource yields repositories from a single GitLab connection configured
// in Sourcegraph via the external services configuration.
type GitLabSource struct {
	svc                 *types.ExternalService
	config              *schema.GitLabConnection
	exclude             excludeFunc
	baseURL             *url.URL // URL with path /api/v4 (no trailing slash)
	nameTransformations reposource.NameTransformations
	provider            *gitlab.ClientProvider
	client              *gitlab.Client
}

var _ Source = &GitLabSource{}
var _ UserSource = &GitLabSource{}
var _ DraftChangesetSource = &GitLabSource{}
var _ ChangesetSource = &GitLabSource{}
var _ AffiliatedRepositorySource = &GitLabSource{}

// NewGitLabSource returns a new GitLabSource from the given external service.
func NewGitLabSource(svc *types.ExternalService, cf *httpcli.Factory) (*GitLabSource, error) {
	var c schema.GitLabConnection
	if err := jsonc.Unmarshal(svc.Config, &c); err != nil {
		return nil, fmt.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	return newGitLabSource(svc, &c, cf)
}

var gitlabRemainingGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "src_gitlab_rate_limit_remaining",
	Help: "Number of calls to GitLab's API remaining before hitting the rate limit.",
}, []string{"resource", "name"})

var gitlabRatelimitWaitCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "src_gitlab_rate_limit_wait_duration_seconds",
	Help: "The amount of time spent waiting on the rate limit",
}, []string{"resource", "name"})

func newGitLabSource(svc *types.ExternalService, c *schema.GitLabConnection, cf *httpcli.Factory) (*GitLabSource, error) {
	baseURL, err := url.Parse(c.Url)
	if err != nil {
		return nil, err
	}
	baseURL = extsvc.NormalizeBaseURL(baseURL)

	if cf == nil {
		cf = httpcli.NewExternalHTTPClientFactory()
	}

	var opts []httpcli.Opt
	if c.Certificate != "" {
		opts = append(opts, httpcli.NewCertPoolOpt(c.Certificate))
	}

	cli, err := cf.Doer(opts...)
	if err != nil {
		return nil, err
	}

	var eb excludeBuilder
	for _, r := range c.Exclude {
		eb.Exact(r.Name)
		eb.Exact(strconv.Itoa(r.Id))
	}
	exclude, err := eb.Build()
	if err != nil {
		return nil, err
	}

	// Validate and cache user-defined name transformations.
	nts, err := reposource.CompileGitLabNameTransformations(c.NameTransformations)
	if err != nil {
		return nil, err
	}

	provider := gitlab.NewClientProvider(baseURL, cli)

	client := provider.GetPATClient(c.Token, "")

	if !envvar.SourcegraphDotComMode() || svc.CloudDefault {
		client.RateLimitMonitor().SetCollector(&ratelimit.MetricsCollector{
			Remaining: func(n float64) {
				gitlabRemainingGauge.WithLabelValues("rest", svc.DisplayName).Set(n)
			},
			WaitDuration: func(n time.Duration) {
				gitlabRatelimitWaitCounter.WithLabelValues("rest", svc.DisplayName).Add(n.Seconds())
			},
		})
	}

	return &GitLabSource{
		svc:                 svc,
		config:              c,
		exclude:             exclude,
		baseURL:             baseURL,
		nameTransformations: nts,
		provider:            provider,
		client:              client,
	}, nil
}

func (s GitLabSource) WithAuthenticator(a auth.Authenticator) (Source, error) {
	switch a.(type) {
	case *auth.OAuthBearerToken,
		*auth.OAuthBearerTokenWithSSH:
		break

	default:
		return nil, newUnsupportedAuthenticatorError("GitLabSource", a)
	}

	sc := s
	sc.client = sc.client.WithAuthenticator(a)

	return &sc, nil
}

// ListRepos returns all GitLab repositories accessible to all connections configured
// in Sourcegraph via the external services configuration.
func (s GitLabSource) ListRepos(ctx context.Context, results chan SourceResult) {
	s.listAllProjects(ctx, results)
}

// GetRepo returns the GitLab repository with the given pathWithNamespace.
func (s GitLabSource) GetRepo(ctx context.Context, pathWithNamespace string) (*types.Repo, error) {
	proj, err := s.client.GetProject(ctx, gitlab.GetProjectOp{
		PathWithNamespace: pathWithNamespace,
		CommonOp:          gitlab.CommonOp{NoCache: true},
	})

	if err != nil {
		return nil, err
	}

	return s.makeRepo(proj), nil
}

// ExternalServices returns a singleton slice containing the external service.
func (s GitLabSource) ExternalServices() types.ExternalServices {
	return types.ExternalServices{s.svc}
}

func (s GitLabSource) makeRepo(proj *gitlab.Project) *types.Repo {
	urn := s.svc.URN()
	return &types.Repo{
		Name: reposource.GitLabRepoName(
			s.config.RepositoryPathPattern,
			s.baseURL.Hostname(),
			proj.PathWithNamespace,
			s.nameTransformations,
		),
		URI: string(reposource.GitLabRepoName(
			"",
			s.baseURL.Hostname(),
			proj.PathWithNamespace,
			s.nameTransformations,
		)),
		ExternalRepo: gitlab.ExternalRepoSpec(proj, *s.baseURL),
		Description:  proj.Description,
		Fork:         proj.ForkedFromProject != nil,
		Archived:     proj.Archived,
		Private:      proj.Visibility == "private",
		Sources: map[string]*types.SourceInfo{
			urn: {
				ID:       urn,
				CloneURL: s.authenticatedRemoteURL(proj),
			},
		},
		Metadata: proj,
	}
}

// authenticatedRemoteURL returns the GitLab projects's Git remote URL with the
// configured GitLab personal access token inserted in the URL userinfo.
func (s *GitLabSource) authenticatedRemoteURL(proj *gitlab.Project) string {
	if s.config.GitURLType == "ssh" {
		return proj.SSHURLToRepo // SSH authentication must be provided out-of-band
	}
	if s.config.Token == "" {
		return proj.HTTPURLToRepo
	}
	u, err := url.Parse(proj.HTTPURLToRepo)
	if err != nil {
		log15.Warn("Error adding authentication to GitLab repository Git remote URL.", "url", proj.HTTPURLToRepo, "error", err)
		return proj.HTTPURLToRepo
	}
	// Any username works; "git" is not special.
	u.User = url.UserPassword("git", s.config.Token)
	return u.String()
}

func (s *GitLabSource) excludes(p *gitlab.Project) bool {
	return s.exclude(p.PathWithNamespace) || s.exclude(strconv.Itoa(p.ID))
}

func (s *GitLabSource) listAllProjects(ctx context.Context, results chan SourceResult) {
	type batch struct {
		projs []*gitlab.Project
		err   error
	}

	ch := make(chan batch)

	var wg sync.WaitGroup

	projch := make(chan *schema.GitLabProject)
	for i := 0; i < 5; i++ { // 5 concurrent requests
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range projch {
				proj, err := s.client.GetProject(ctx, gitlab.GetProjectOp{
					ID:                p.Id,
					PathWithNamespace: p.Name,
					CommonOp:          gitlab.CommonOp{NoCache: true},
				})

				if err != nil {
					// TODO(tsenart): When implementing dry-run, reconsider alternatives to return
					// 404 errors on external service config validation.
					if gitlab.IsNotFound(err) {
						log15.Warn("skipping missing gitlab.projects entry:", "name", p.Name, "id", p.Id, "err", err)
						continue
					}
					ch <- batch{err: errors.Wrapf(err, "gitlab.projects: id: %d, name: %q", p.Id, p.Name)}
				} else {
					ch <- batch{projs: []*gitlab.Project{proj}}
				}

				time.Sleep(s.client.RateLimitMonitor().RecommendedWaitForBackgroundOp(1))
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(projch)
		// Admins normally add to end of lists, so end of list most likely has
		// new repos => stream them first.
		for i := len(s.config.Projects) - 1; i >= 0; i-- {
			select {
			case projch <- s.config.Projects[i]:
			case <-ctx.Done():
				return
			}
		}
	}()

	for _, projectQuery := range s.config.ProjectQuery {
		if projectQuery == "none" {
			continue
		}

		const perPage = 100
		wg.Add(1)
		go func(projectQuery string) {
			defer wg.Done()

			url, err := projectQueryToURL(projectQuery, perPage) // first page URL
			if err != nil {
				ch <- batch{err: errors.Wrapf(err, "invalid GitLab projectQuery=%q", projectQuery)}
				return
			}

			for {
				if err := ctx.Err(); err != nil {
					ch <- batch{err: err}
					return
				}
				projects, nextPageURL, err := s.client.ListProjects(ctx, url)
				if err != nil {
					ch <- batch{err: errors.Wrapf(err, "error listing GitLab projects: url=%q", url)}
					return
				}
				ch <- batch{projs: projects}
				if nextPageURL == nil {
					return
				}
				url = *nextPageURL

				// 0-duration sleep unless nearing rate limit exhaustion
				time.Sleep(s.client.RateLimitMonitor().RecommendedWaitForBackgroundOp(1))
			}
		}(projectQuery)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	seen := make(map[int]bool)
	for b := range ch {
		if b.err != nil {
			results <- SourceResult{Source: s, Err: b.err}
			continue
		}

		for _, proj := range b.projs {
			if !seen[proj.ID] && !s.excludes(proj) {
				results <- SourceResult{Source: s, Repo: s.makeRepo(proj)}
				seen[proj.ID] = true
			}
		}
	}
}

var schemeOrHostNotEmptyErr = errors.New("scheme and host should be empty")

func projectQueryToURL(projectQuery string, perPage int) (string, error) {
	// If all we have is the URL query, prepend "projects"
	if strings.HasPrefix(projectQuery, "?") {
		projectQuery = "projects" + projectQuery
	} else if projectQuery == "" {
		projectQuery = "projects"
	}

	u, err := url.Parse(projectQuery)
	if err != nil {
		return "", err
	}
	if u.Scheme != "" || u.Host != "" {
		return "", schemeOrHostNotEmptyErr
	}
	q := u.Query()
	q.Set("per_page", strconv.Itoa(perPage))
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// CreateChangeset creates a GitLab merge request. If it already exists,
// *Changeset will be populated and the return value will be true.
func (s *GitLabSource) CreateChangeset(ctx context.Context, c *Changeset) (bool, error) {
	project := c.Repo.Metadata.(*gitlab.Project)
	exists := false
	source := git.AbbreviateRef(c.HeadRef)
	target := git.AbbreviateRef(c.BaseRef)

	mr, err := s.client.CreateMergeRequest(ctx, project, gitlab.CreateMergeRequestOpts{
		SourceBranch: source,
		TargetBranch: target,
		Title:        c.Title,
		Description:  c.Body,
	})
	if err != nil {
		if err == gitlab.ErrMergeRequestAlreadyExists {
			exists = true

			mr, err = s.client.GetOpenMergeRequestByRefs(ctx, project, source, target)
			if err != nil {
				return exists, errors.Wrap(err, "retrieving an extant merge request")
			}
		} else {
			return exists, errors.Wrap(err, "creating the merge request")
		}
	}

	// These additional API calls can go away once we can use the GraphQL API.
	if err := s.decorateMergeRequestData(ctx, project, mr); err != nil {
		return exists, errors.Wrapf(err, "retrieving additional data for merge request %d", mr.IID)
	}

	if err := c.SetMetadata(mr); err != nil {
		return exists, errors.Wrap(err, "setting changeset metadata")
	}
	return exists, nil
}

// CreateDraftChangeset creates a GitLab merge request. If it already exists,
// *Changeset will be populated and the return value will be true.
func (s *GitLabSource) CreateDraftChangeset(ctx context.Context, c *Changeset) (bool, error) {
	c.Title = gitlab.SetWIP(c.Title)

	exists, err := s.CreateChangeset(ctx, c)
	if err != nil {
		return exists, err
	}

	mr, ok := c.Changeset.Metadata.(*gitlab.MergeRequest)
	if !ok {
		return false, errors.New("Changeset is not a GitLab merge request")
	}

	// If it already exists, but is not a WIP, we need to update the title.
	if exists && !mr.WorkInProgress {
		if err := s.UpdateChangeset(ctx, c); err != nil {
			return exists, err
		}
	}
	return exists, nil
}

// CloseChangeset closes the merge request on GitLab, leaving it unlocked.
func (s *GitLabSource) CloseChangeset(ctx context.Context, c *Changeset) error {
	project := c.Repo.Metadata.(*gitlab.Project)
	mr, ok := c.Changeset.Metadata.(*gitlab.MergeRequest)
	if !ok {
		return errors.New("Changeset is not a GitLab merge request")
	}

	// Title and TargetBranch are required, even though we're not actually
	// changing them.
	updated, err := s.client.UpdateMergeRequest(ctx, project, mr, gitlab.UpdateMergeRequestOpts{
		Title:        mr.Title,
		TargetBranch: mr.TargetBranch,
		StateEvent:   gitlab.UpdateMergeRequestStateEventClose,
	})
	if err != nil {
		return errors.Wrap(err, "updating GitLab merge request")
	}

	// These additional API calls can go away once we can use the GraphQL API.
	if err := s.decorateMergeRequestData(ctx, project, mr); err != nil {
		return errors.Wrapf(err, "retrieving additional data for merge request %d", mr.IID)
	}

	if err := c.SetMetadata(updated); err != nil {
		return errors.Wrap(err, "setting changeset metadata")
	}
	return nil
}

// LoadChangeset loads the given merge request from GitLab and updates it.
func (s *GitLabSource) LoadChangeset(ctx context.Context, cs *Changeset) error {
	project := cs.Repo.Metadata.(*gitlab.Project)

	iid, err := strconv.ParseInt(cs.ExternalID, 10, 64)
	if err != nil {
		return errors.Wrapf(err, "parsing changeset external ID %s", cs.ExternalID)
	}

	mr, err := s.client.GetMergeRequest(ctx, project, gitlab.ID(iid))
	if err != nil {
		if errors.Cause(err) == gitlab.ErrMergeRequestNotFound {
			return ChangesetNotFoundError{Changeset: cs}
		}
		return errors.Wrapf(err, "retrieving merge request %d", iid)
	}

	// These additional API calls can go away once we can use the GraphQL API.
	if err := s.decorateMergeRequestData(ctx, project, mr); err != nil {
		return errors.Wrapf(err, "retrieving additional data for merge request %d", iid)
	}

	if err := cs.SetMetadata(mr); err != nil {
		return errors.Wrapf(err, "setting changeset metadata for merge request %d", iid)
	}

	return nil
}

// ReopenChangeset closes the merge request on GitLab, leaving it unlocked.
func (s *GitLabSource) ReopenChangeset(ctx context.Context, c *Changeset) error {
	project := c.Repo.Metadata.(*gitlab.Project)
	mr, ok := c.Changeset.Metadata.(*gitlab.MergeRequest)
	if !ok {
		return errors.New("Changeset is not a GitLab merge request")
	}

	// Title and TargetBranch are required, even though we're not actually
	// changing them.
	updated, err := s.client.UpdateMergeRequest(ctx, project, mr, gitlab.UpdateMergeRequestOpts{
		Title:        mr.Title,
		TargetBranch: mr.TargetBranch,
		StateEvent:   gitlab.UpdateMergeRequestStateEventReopen,
	})
	if err != nil {
		return errors.Wrap(err, "reopening GitLab merge request")
	}

	// These additional API calls can go away once we can use the GraphQL API.
	if err := s.decorateMergeRequestData(ctx, project, mr); err != nil {
		return errors.Wrapf(err, "retrieving additional data for merge request %d", mr.IID)
	}

	if err := c.SetMetadata(updated); err != nil {
		return errors.Wrap(err, "setting changeset metadata")
	}
	return nil
}

func (s *GitLabSource) decorateMergeRequestData(ctx context.Context, project *gitlab.Project, mr *gitlab.MergeRequest) error {
	notes, err := s.getMergeRequestNotes(ctx, project, mr)
	if err != nil {
		return errors.Wrap(err, "retrieving notes")
	}

	events, err := s.getMergeRequestResourceStateEvents(ctx, project, mr)
	if err != nil {
		return errors.Wrap(err, "retrieving resource state events")
	}

	pipelines, err := s.getMergeRequestPipelines(ctx, project, mr)
	if err != nil {
		return errors.Wrap(err, "retrieving pipelines")
	}

	mr.Notes = notes
	mr.Pipelines = pipelines
	mr.ResourceStateEvents = events
	return nil
}

// getMergeRequestNotes retrieves the notes attached to a merge request in
// descending time order.
func (s *GitLabSource) getMergeRequestNotes(ctx context.Context, project *gitlab.Project, mr *gitlab.MergeRequest) ([]*gitlab.Note, error) {
	// Get the forward iterator that gives us a note page at a time.
	it := s.client.GetMergeRequestNotes(ctx, project, mr.IID)

	// Now we can iterate over the pages of notes and fill in the slice to be
	// returned.
	notes, err := readSystemNotes(it)
	if err != nil {
		return nil, errors.Wrap(err, "reading note pages")
	}

	return notes, nil
}

func readSystemNotes(it func() ([]*gitlab.Note, error)) ([]*gitlab.Note, error) {
	var notes []*gitlab.Note

	for {
		page, err := it()
		if err != nil {
			return nil, errors.Wrap(err, "retrieving note page")
		}
		if len(page) == 0 {
			// The terminal condition for the iterator is returning an empty
			// slice with no error, so we can stop iterating here.
			return notes, nil
		}

		for _, note := range page {
			// We're only interested in system notes for batch changes, since they
			// include the review state changes we need; let's not even bother
			// storing the non-system ones.
			if note.System {
				notes = append(notes, note)
			}
		}
	}
}

// getMergeRequestResourceStateEvents retrieves the events attached to a merge request in
// descending time order.
func (s *GitLabSource) getMergeRequestResourceStateEvents(ctx context.Context, project *gitlab.Project, mr *gitlab.MergeRequest) ([]*gitlab.ResourceStateEvent, error) {
	// Get the forward iterator that gives us a note page at a time.
	it := s.client.GetMergeRequestResourceStateEvents(ctx, project, mr.IID)

	// Now we can iterate over the pages of notes and fill in the slice to be
	// returned.
	events, err := readMergeRequestResourceStateEvents(it)
	if err != nil {
		return nil, errors.Wrap(err, "reading resource state events pages")
	}

	return events, nil
}

func readMergeRequestResourceStateEvents(it func() ([]*gitlab.ResourceStateEvent, error)) ([]*gitlab.ResourceStateEvent, error) {
	var events []*gitlab.ResourceStateEvent

	for {
		page, err := it()
		if err != nil {
			return nil, errors.Wrap(err, "retrieving resource state events page")
		}
		if len(page) == 0 {
			// The terminal condition for the iterator is returning an empty
			// slice with no error, so we can stop iterating here.
			return events, nil
		}

		events = append(events, page...)
	}
}

// getMergeRequestPipelines retrieves the pipelines attached to a merge request
// in descending time order.
func (s *GitLabSource) getMergeRequestPipelines(ctx context.Context, project *gitlab.Project, mr *gitlab.MergeRequest) ([]*gitlab.Pipeline, error) {
	// Get the forward iterator that gives us a pipeline page at a time.
	it := s.client.GetMergeRequestPipelines(ctx, project, mr.IID)

	// Now we can iterate over the pages of pipelines and fill in the slice to
	// be returned.
	pipelines, err := readPipelines(it)
	if err != nil {
		return nil, errors.Wrap(err, "reading pipeline pages")
	}
	return pipelines, nil
}

func readPipelines(it func() ([]*gitlab.Pipeline, error)) ([]*gitlab.Pipeline, error) {
	var pipelines []*gitlab.Pipeline

	for {
		page, err := it()
		if err != nil {
			return nil, errors.Wrap(err, "retrieving pipeline page")
		}
		if len(page) == 0 {
			// The terminal condition for the iterator is returning an empty
			// slice with no error, so we can stop iterating here.
			return pipelines, nil
		}

		pipelines = append(pipelines, page...)
	}
}

// UpdateChangeset updates the merge request on GitLab to reflect the local
// state of the Changeset.
func (s *GitLabSource) UpdateChangeset(ctx context.Context, c *Changeset) error {
	mr, ok := c.Changeset.Metadata.(*gitlab.MergeRequest)
	if !ok {
		return errors.New("Changeset is not a GitLab merge request")
	}
	project := c.Repo.Metadata.(*gitlab.Project)

	updated, err := s.client.UpdateMergeRequest(ctx, project, mr, gitlab.UpdateMergeRequestOpts{
		Title:        c.Title,
		Description:  c.Body,
		TargetBranch: git.AbbreviateRef(c.BaseRef),
	})
	if err != nil {
		return errors.Wrap(err, "updating GitLab merge request")
	}

	// These additional API calls can go away once we can use the GraphQL API.
	if err := s.decorateMergeRequestData(ctx, project, mr); err != nil {
		return errors.Wrapf(err, "retrieving additional data for merge request %d", mr.IID)
	}

	return c.Changeset.SetMetadata(updated)
}

// UndraftChangeset marks the changeset as *not* work in progress anymore.
func (s *GitLabSource) UndraftChangeset(ctx context.Context, c *Changeset) error {
	c.Title = gitlab.UnsetWIP(c.Title)
	return s.UpdateChangeset(ctx, c)
}

func (s *GitLabSource) AffiliatedRepositories(ctx context.Context) ([]types.CodeHostRepository, error) {
	queryURL, err := projectQueryToURL("projects?membership=true&archived=no", 40) // first page URL
	if err != nil {
		return nil, err
	}
	var (
		projects    []*gitlab.Project
		nextPageURL = &queryURL
	)

	out := []types.CodeHostRepository{}
	for nextPageURL != nil {
		projects, nextPageURL, err = s.client.ListProjects(ctx, *nextPageURL)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			out = append(out, types.CodeHostRepository{
				Name:       p.PathWithNamespace,
				Private:    p.Visibility == "private",
				CodeHostID: s.svc.ID,
			})
		}
	}
	return out, nil
}
