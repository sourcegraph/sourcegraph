package sources

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/Masterminds/semver"

	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/versions"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

type GitLabSource struct {
	client *gitlab.Client
	au     auth.Authenticator
}

var _ ChangesetSource = &GitLabSource{}
var _ DraftChangesetSource = &GitLabSource{}
var _ ForkableChangesetSource = &GitLabSource{}

// NewGitLabSource returns a new GitLabSource from the given external service.
func NewGitLabSource(ctx context.Context, svc *types.ExternalService, cf *httpcli.Factory) (*GitLabSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.GitLabConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	return newGitLabSource(svc.URN(), &c, cf)
}

func newGitLabSource(urn string, c *schema.GitLabConnection, cf *httpcli.Factory) (*GitLabSource, error) {
	baseURL, err := url.Parse(c.Url)
	if err != nil {
		return nil, err
	}
	baseURL = extsvc.NormalizeBaseURL(baseURL)

	if cf == nil {
		cf = httpcli.NewExternalClientFactory()
	}
	cf = cf.WithOpts(httpClientCertificateOptions(nil, c.Certificate)...)

	// Don't modify passed-in parameter.
	var authr auth.Authenticator
	if c.Token != "" {
		switch c.TokenType {
		case "oauth":
			authr = &auth.OAuthBearerToken{Token: c.Token}
		default:
			authr = &gitlab.SudoableToken{Token: c.Token}
		}
	}

	provider, err := gitlab.NewClientProvider(urn, baseURL, cf)
	if err != nil {
		return nil, err
	}

	return &GitLabSource{
		au:     authr,
		client: provider.GetAuthenticatorClient(authr),
	}, nil
}

func (s GitLabSource) GitserverPushConfig(repo *types.Repo) (*protocol.PushConfig, error) {
	return GitserverPushConfig(repo, s.au)
}

func (s GitLabSource) WithAuthenticator(a auth.Authenticator) (ChangesetSource, error) {
	switch a.(type) {
	case *auth.OAuthBearerToken,
		*auth.OAuthBearerTokenWithSSH:
		break

	default:
		return nil, newUnsupportedAuthenticatorError("GitLabSource", a)
	}

	sc := s
	sc.au = a
	sc.client = sc.client.WithAuthenticator(a)

	return &sc, nil
}

func (s GitLabSource) ValidateAuthenticator(ctx context.Context) error {
	return s.client.ValidateToken(ctx)
}

// CreateChangeset creates a GitLab merge request. If it already exists,
// *Changeset will be populated and the return value will be true.
func (s *GitLabSource) CreateChangeset(ctx context.Context, c *Changeset) (bool, error) {
	remoteProject := c.RemoteRepo.Metadata.(*gitlab.Project)
	targetProject := c.TargetRepo.Metadata.(*gitlab.Project)
	exists := false
	source := gitdomain.AbbreviateRef(c.HeadRef)
	target := gitdomain.AbbreviateRef(c.BaseRef)
	targetProjectID := 0
	if c.RemoteRepo != c.TargetRepo {
		targetProjectID = c.TargetRepo.Metadata.(*gitlab.Project).ID
	}
	removeSource := conf.Get().BatchChangesAutoDeleteBranch

	// We have to create the merge request against the remote project, not the
	// target project, because that's how GitLab's API works: you provide the
	// target project ID as one of the parameters. Yes, this is weird.
	//
	// Of course, we then have to use the targetProject for everything else,
	// because that's what the merge request actually belongs to.
	mr, err := s.client.CreateMergeRequest(ctx, remoteProject, gitlab.CreateMergeRequestOpts{
		SourceBranch:       source,
		TargetBranch:       target,
		TargetProjectID:    targetProjectID,
		Title:              c.Title,
		Description:        c.Body,
		RemoveSourceBranch: removeSource,
	})
	if err != nil {
		if err == gitlab.ErrMergeRequestAlreadyExists {
			exists = true

			mr, err = s.client.GetOpenMergeRequestByRefs(ctx, targetProject, source, target)
			if err != nil {
				return exists, errors.Wrap(err, "retrieving an extant merge request")
			}
		} else {
			return exists, errors.Wrap(err, "creating the merge request")
		}
	}

	// These additional API calls can go away once we can use the GraphQL API.
	if err := s.decorateMergeRequestData(ctx, targetProject, mr); err != nil {
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
	v, err := s.determineVersion(ctx)
	if err != nil {
		return false, err
	}

	c.Title = gitlab.SetWIPOrDraft(c.Title, v)

	exists, err := s.CreateChangeset(ctx, c)
	if err != nil {
		return exists, err
	}

	mr, ok := c.Changeset.Metadata.(*gitlab.MergeRequest)
	if !ok {
		return false, errors.New("Changeset is not a GitLab merge request")
	}

	isDraftOrWIP := mr.WorkInProgress || mr.Draft

	// If it already exists, but is not a WIP, we need to update the title.
	if exists && !isDraftOrWIP {
		if err := s.UpdateChangeset(ctx, c); err != nil {
			return exists, err
		}
	}
	return exists, nil
}

// CloseChangeset closes the merge request on GitLab, leaving it unlocked.
func (s *GitLabSource) CloseChangeset(ctx context.Context, c *Changeset) error {
	project := c.TargetRepo.Metadata.(*gitlab.Project)
	mr, ok := c.Changeset.Metadata.(*gitlab.MergeRequest)
	if !ok {
		return errors.New("Changeset is not a GitLab merge request")
	}

	removeSource := conf.Get().BatchChangesAutoDeleteBranch

	// Title and TargetBranch are required, even though we're not actually
	// changing them.
	updated, err := s.client.UpdateMergeRequest(ctx, project, mr, gitlab.UpdateMergeRequestOpts{
		Title:              mr.Title,
		TargetBranch:       mr.TargetBranch,
		StateEvent:         gitlab.UpdateMergeRequestStateEventClose,
		RemoveSourceBranch: removeSource,
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
	project := cs.TargetRepo.Metadata.(*gitlab.Project)

	iid, err := strconv.ParseInt(cs.ExternalID, 10, 64)
	if err != nil {
		return errors.Wrapf(err, "parsing changeset external ID %s", cs.ExternalID)
	}

	mr, err := s.client.GetMergeRequest(ctx, project, gitlab.ID(iid))
	if err != nil {
		if errors.Is(err, gitlab.ErrMergeRequestNotFound) {
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
	project := c.TargetRepo.Metadata.(*gitlab.Project)
	mr, ok := c.Changeset.Metadata.(*gitlab.MergeRequest)
	if !ok {
		return errors.New("Changeset is not a GitLab merge request")
	}

	removeSource := conf.Get().BatchChangesAutoDeleteBranch

	// Title and TargetBranch are required, even though we're not actually
	// changing them.
	updated, err := s.client.UpdateMergeRequest(ctx, project, mr, gitlab.UpdateMergeRequestOpts{
		Title:              mr.Title,
		TargetBranch:       mr.TargetBranch,
		StateEvent:         gitlab.UpdateMergeRequestStateEventReopen,
		RemoveSourceBranch: removeSource,
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

	if mr.SourceProjectID != mr.ProjectID {
		project, err := s.client.GetProject(ctx, gitlab.GetProjectOp{
			ID: int(mr.SourceProjectID),
		})
		if err != nil {
			return errors.Wrap(err, "getting source project")
		}

		name, err := project.Name()
		if err != nil {
			return errors.Wrap(err, "parsing project name")
		}
		ns, err := project.Namespace()
		if err != nil {
			return errors.Wrap(err, "parsing project namespace")
		}

		mr.SourceProjectName = name
		mr.SourceProjectNamespace = ns
	} else {
		mr.SourceProjectName = ""
		mr.SourceProjectNamespace = ""
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

func (s *GitLabSource) determineVersion(ctx context.Context) (*semver.Version, error) {
	var v string
	chvs, err := versions.GetVersions()
	if err != nil {
		return nil, err
	}

	for _, chv := range chvs {
		if chv.ExternalServiceKind == extsvc.KindGitLab && chv.Key == s.client.Urn() {
			v = chv.Version
			break
		}
	}

	// if we are unable to get the version from Redis, we default to making a request
	// to the codehost to get the version.
	if v == "" {
		v, err = s.client.GetVersion(ctx)
		if err != nil {
			return nil, err
		}
	}

	version, err := semver.NewVersion(v)
	return version, err
}

// UpdateChangeset updates the merge request on GitLab to reflect the local
// state of the Changeset.
func (s *GitLabSource) UpdateChangeset(ctx context.Context, c *Changeset) error {
	mr, ok := c.Changeset.Metadata.(*gitlab.MergeRequest)
	if !ok {
		return errors.New("Changeset is not a GitLab merge request")
	}
	project := c.TargetRepo.Metadata.(*gitlab.Project)

	// Avoid accidentally undrafting the changeset by checking its current
	// status.
	title := c.Title
	if mr.WorkInProgress || mr.Draft {
		v, err := s.determineVersion(ctx)
		if err != nil {
			return err
		}

		title = gitlab.SetWIPOrDraft(c.Title, v)
	}

	removeSource := conf.Get().BatchChangesAutoDeleteBranch

	updated, err := s.client.UpdateMergeRequest(ctx, project, mr, gitlab.UpdateMergeRequestOpts{
		Title:              title,
		Description:        c.Body,
		TargetBranch:       gitdomain.AbbreviateRef(c.BaseRef),
		RemoveSourceBranch: removeSource,
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
	mr, ok := c.Changeset.Metadata.(*gitlab.MergeRequest)
	if !ok {
		return errors.New("Changeset is not a GitLab merge request")
	}

	// Remove WIP prefix from title.
	c.Title = gitlab.UnsetWIPOrDraft(c.Title)
	// And mark the mr as not WorkInProgress / Draft anymore, otherwise UpdateChangeset
	// will prepend the WIP: prefix again.

	// We have to set both Draft and WorkInProgress or else the changeset will retain it's
	// draft status. Both fields mirror each other, so if either is true then Gitlab assumes
	// the changeset is still a draft.
	mr.Draft = false
	mr.WorkInProgress = false

	return s.UpdateChangeset(ctx, c)
}

// CreateComment posts a comment on the Changeset.
func (s *GitLabSource) CreateComment(ctx context.Context, c *Changeset, text string) error {
	project := c.TargetRepo.Metadata.(*gitlab.Project)
	mr, ok := c.Changeset.Metadata.(*gitlab.MergeRequest)
	if !ok {
		return errors.New("Changeset is not a GitLab merge request")
	}

	return s.client.CreateMergeRequestNote(ctx, project, mr, text)
}

// MergeChangeset merges a Changeset on the code host, if in a mergeable state.
// If squash is true, a squash-then-merge merge will be performed.
func (s *GitLabSource) MergeChangeset(ctx context.Context, c *Changeset, squash bool) error {
	mr, ok := c.Changeset.Metadata.(*gitlab.MergeRequest)
	if !ok {
		return errors.New("Changeset is not a GitLab merge request")
	}
	project := c.TargetRepo.Metadata.(*gitlab.Project)

	updated, err := s.client.MergeMergeRequest(ctx, project, mr, squash)
	if err != nil {
		if errors.Is(err, gitlab.ErrNotMergeable) {
			return ChangesetNotMergeableError{ErrorMsg: err.Error()}
		}
		return errors.Wrap(err, "merging GitLab merge request")
	}

	// These additional API calls can go away once we can use the GraphQL API.
	if err := s.decorateMergeRequestData(ctx, project, mr); err != nil {
		return errors.Wrapf(err, "retrieving additional data for merge request %d", mr.IID)
	}

	return c.Changeset.SetMetadata(updated)
}

func (*GitLabSource) IsPushResponseArchived(s string) bool {
	return strings.Contains(s, "ERROR: You are not allowed to push code to this project")
}

func (s GitLabSource) GetFork(ctx context.Context, targetRepo *types.Repo, namespace, n *string) (*types.Repo, error) {
	return getGitLabForkInternal(ctx, targetRepo, s.client, namespace, n)
}

func (s GitLabSource) BuildCommitOpts(repo *types.Repo, _ *btypes.Changeset, spec *btypes.ChangesetSpec, pushOpts *protocol.PushConfig) protocol.CreateCommitFromPatchRequest {
	return BuildCommitOptsCommon(repo, spec, pushOpts)
}

type gitlabClientFork interface {
	ForkProject(ctx context.Context, project *gitlab.Project, namespace *string, name string) (*gitlab.Project, error)
}

func getGitLabForkInternal(ctx context.Context, targetRepo *types.Repo, client gitlabClientFork, namespace, n *string) (*types.Repo, error) {
	tr := targetRepo.Metadata.(*gitlab.Project)

	targetNamespace, err := tr.Namespace()
	if err != nil {
		return nil, errors.Wrap(err, "getting target project namespace")
	}

	// It's possible to nest namespaces on GitLab, so we need to remove any internal "/"s
	// to make the namespace repo-name-friendly when we use it to form the fork repo name.
	targetNamespace = strings.ReplaceAll(targetNamespace, "/", "-")

	var name string
	if n != nil {
		name = *n
	} else {
		targetName, err := tr.Name()
		if err != nil {
			return nil, errors.Wrap(err, "getting target project name")
		}
		name = DefaultForkName(targetNamespace, targetName)
	}

	// `client.ForkProject` returns an existing fork if it has already been created. It also automatically uses the currently authenticated user's namespace if none is provided.
	fork, err := client.ForkProject(ctx, tr, namespace, name)
	if err != nil {
		return nil, errors.Wrap(err, "fetching fork or forking project")
	}

	if fork.ForkedFromProject == nil {
		return nil, errors.New("project is not a fork")
	} else if fork.ForkedFromProject.ID != tr.ID {
		return nil, errors.New("project was not forked from the target project")
	}

	// Now we make a copy of targetRepo, but with its sources and metadata updated to
	// point to the fork
	forkRepo, err := CopyRepoAsFork(targetRepo, fork, tr.PathWithNamespace, fork.PathWithNamespace)
	if err != nil {
		return nil, errors.Wrap(err, "updating target repo sources and metadata")
	}

	return forkRepo, nil
}
