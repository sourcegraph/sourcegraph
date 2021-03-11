package testing

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// FakeChangesetSource is a fake implementation of the repos.ChangesetSource
// interface to be used in tests.
type FakeChangesetSource struct {
	Svc *types.ExternalService

	authenticator auth.Authenticator

	CreateDraftChangesetCalled  bool
	UndraftedChangesetsCalled   bool
	CreateChangesetCalled       bool
	UpdateChangesetCalled       bool
	ListReposCalled             bool
	ExternalServicesCalled      bool
	LoadChangesetCalled         bool
	CloseChangesetCalled        bool
	ReopenChangesetCalled       bool
	AuthenticatedUsernameCalled bool

	// The Changeset.HeadRef to be expected in CreateChangeset/UpdateChangeset calls.
	WantHeadRef string
	// The Changeset.BaseRef to be expected in CreateChangeset/UpdateChangeset calls.
	WantBaseRef string

	// The metadata the FakeChangesetSource should set on the created/updated
	// Changeset with changeset.SetMetadata.
	FakeMetadata interface{}

	// Whether or not the changeset already ChangesetExists on the code host at the time
	// when CreateChangeset is called.
	ChangesetExists bool

	// error to be returned from every method
	Err error

	// ClosedChangesets contains the changesets that were passed to CloseChangeset
	ClosedChangesets []*repos.Changeset

	// CreatedChangesets contains the changesets that were passed to
	// CreateChangeset
	CreatedChangesets []*repos.Changeset

	// LoadedChangesets contains the changesets that were passed to LoadChangeset
	LoadedChangesets []*repos.Changeset

	// UpdateChangesets contains the changesets that were passed to
	// UpdateChangeset
	UpdatedChangesets []*repos.Changeset

	// ReopenedChangesets contains the changesets that were passed to ReopenedChangeset
	ReopenedChangesets []*repos.Changeset

	// UndraftedChangesets contains the changesets that were passed to UndraftChangeset
	UndraftedChangesets []*repos.Changeset

	// Username is the username returned by AuthenticatedUsername
	Username string
}

var _ repos.ChangesetSource = &FakeChangesetSource{}
var _ repos.DraftChangesetSource = &FakeChangesetSource{}
var _ repos.UserSource = &FakeChangesetSource{}

func (s *FakeChangesetSource) CreateDraftChangeset(ctx context.Context, c *repos.Changeset) (bool, error) {
	s.CreateDraftChangesetCalled = true

	if s.Err != nil {
		return s.ChangesetExists, s.Err
	}

	if c.Repo == nil {
		return false, NoReposErr
	}

	if c.HeadRef != s.WantHeadRef {
		return s.ChangesetExists, fmt.Errorf("wrong HeadRef. want=%s, have=%s", s.WantHeadRef, c.HeadRef)
	}

	if c.BaseRef != s.WantBaseRef {
		return s.ChangesetExists, fmt.Errorf("wrong BaseRef. want=%s, have=%s", s.WantBaseRef, c.BaseRef)
	}

	if err := c.SetMetadata(s.FakeMetadata); err != nil {
		return s.ChangesetExists, err
	}

	s.CreatedChangesets = append(s.CreatedChangesets, c)
	return s.ChangesetExists, s.Err
}

func (s *FakeChangesetSource) UndraftChangeset(ctx context.Context, c *repos.Changeset) error {
	s.UndraftedChangesetsCalled = true

	if s.Err != nil {
		return s.Err
	}

	if c.Repo == nil {
		return NoReposErr
	}

	s.UndraftedChangesets = append(s.UndraftedChangesets, c)

	return c.SetMetadata(s.FakeMetadata)
}

func (s *FakeChangesetSource) CreateChangeset(ctx context.Context, c *repos.Changeset) (bool, error) {
	s.CreateChangesetCalled = true

	if s.Err != nil {
		return s.ChangesetExists, s.Err
	}

	if c.Repo == nil {
		return false, NoReposErr
	}

	if c.HeadRef != s.WantHeadRef {
		return s.ChangesetExists, fmt.Errorf("wrong HeadRef. want=%s, have=%s", s.WantHeadRef, c.HeadRef)
	}

	if c.BaseRef != s.WantBaseRef {
		return s.ChangesetExists, fmt.Errorf("wrong BaseRef. want=%s, have=%s", s.WantBaseRef, c.BaseRef)
	}

	if err := c.SetMetadata(s.FakeMetadata); err != nil {
		return s.ChangesetExists, err
	}

	s.CreatedChangesets = append(s.CreatedChangesets, c)
	return s.ChangesetExists, s.Err
}

func (s *FakeChangesetSource) UpdateChangeset(ctx context.Context, c *repos.Changeset) error {
	s.UpdateChangesetCalled = true

	if s.Err != nil {
		return s.Err
	}
	if c.Repo == nil {
		return NoReposErr
	}

	if c.BaseRef != s.WantBaseRef {
		return fmt.Errorf("wrong BaseRef. want=%s, have=%s", s.WantBaseRef, c.BaseRef)
	}

	s.UpdatedChangesets = append(s.UpdatedChangesets, c)
	return c.SetMetadata(s.FakeMetadata)
}

var fakeNotImplemented = errors.New("not implemented in FakeChangesetSource")

func (s *FakeChangesetSource) ListRepos(ctx context.Context, results chan repos.SourceResult) {
	s.ListReposCalled = true

	results <- repos.SourceResult{Source: s, Err: fakeNotImplemented}
}

func (s *FakeChangesetSource) ExternalServices() types.ExternalServices {
	s.ExternalServicesCalled = true

	return types.ExternalServices{s.Svc}
}
func (s *FakeChangesetSource) LoadChangeset(ctx context.Context, c *repos.Changeset) error {
	s.LoadChangesetCalled = true

	if s.Err != nil {
		return s.Err
	}

	if c.Repo == nil {
		return NoReposErr
	}

	if err := c.SetMetadata(s.FakeMetadata); err != nil {
		return err
	}

	s.LoadedChangesets = append(s.LoadedChangesets, c)
	return nil
}

var NoReposErr = errors.New("no repository set on repos.Changeset")

func (s *FakeChangesetSource) CloseChangeset(ctx context.Context, c *repos.Changeset) error {
	s.CloseChangesetCalled = true

	if s.Err != nil {
		return s.Err
	}

	if c.Repo == nil {
		return NoReposErr
	}

	s.ClosedChangesets = append(s.ClosedChangesets, c)

	return c.SetMetadata(s.FakeMetadata)
}

func (s *FakeChangesetSource) ReopenChangeset(ctx context.Context, c *repos.Changeset) error {
	s.ReopenChangesetCalled = true

	if s.Err != nil {
		return s.Err
	}

	if c.Repo == nil {
		return NoReposErr
	}

	s.ReopenedChangesets = append(s.ReopenedChangesets, c)

	return c.SetMetadata(s.FakeMetadata)
}

func (s *FakeChangesetSource) WithAuthenticator(a auth.Authenticator) (repos.Source, error) {
	s.authenticator = a
	return s, nil
}

func (s *FakeChangesetSource) AuthenticatedUsername(ctx context.Context) (string, error) {
	s.AuthenticatedUsernameCalled = true
	return s.Username, nil
}

// FakeGitserverClient is a test implementation of the GitserverClient
// interface required by ExecChangesetJob.
type FakeGitserverClient struct {
	Response    string
	ResponseErr error

	CreateCommitFromPatchCalled bool
	CreateCommitFromPatchReq    *protocol.CreateCommitFromPatchRequest
}

func (f *FakeGitserverClient) CreateCommitFromPatch(ctx context.Context, req protocol.CreateCommitFromPatchRequest) (string, error) {
	f.CreateCommitFromPatchCalled = true
	f.CreateCommitFromPatchReq = &req
	return f.Response, f.ResponseErr
}
