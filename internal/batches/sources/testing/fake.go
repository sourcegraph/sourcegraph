package testing

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/batches/sources"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// NewFakeSourcer returns a new faked Sourcer to be used for testing Batch Changes.
func NewFakeSourcer(err error, source sources.ChangesetSource) sources.Sourcer {
	return &fakeSourcer{
		err,
		source,
	}
}

type fakeSourcer struct {
	err    error
	source sources.ChangesetSource
}

func (s *fakeSourcer) ForChangeset(ctx context.Context, tx sources.SourcerStore, ch *btypes.Changeset, as sources.AuthenticationStrategy, repo *types.Repo) (sources.ChangesetSource, error) {
	return s.source, s.err
}

func (s *fakeSourcer) ForUser(ctx context.Context, tx sources.SourcerStore, uid int32, repo *types.Repo) (sources.ChangesetSource, error) {
	return s.source, s.err
}

func (s *fakeSourcer) ForExternalService(ctx context.Context, tx sources.SourcerStore, au auth.Authenticator, opts sources.ForExternalServiceOpts, as sources.AuthenticationStrategy) (sources.ChangesetSource, error) {
	return s.source, s.err
}

// FakeChangesetSource is a fake implementation of the ChangesetSource
// interface to be used in tests.
type FakeChangesetSource struct {
	Svc *types.ExternalService

	CurrentAuthenticator auth.Authenticator

	CreateDraftChangesetCalled  bool
	UndraftedChangesetsCalled   bool
	CreateChangesetCalled       bool
	UpdateChangesetCalled       bool
	ListReposCalled             bool
	ExternalServicesCalled      bool
	LoadChangesetCalled         bool
	CloseChangesetCalled        bool
	ReopenChangesetCalled       bool
	CreateCommentCalled         bool
	AuthenticatedUsernameCalled bool
	ValidateAuthenticatorCalled bool
	MergeChangesetCalled        bool
	IsArchivedPushErrorCalled   bool
	BuildCommitOptsCalled       bool

	// The Changeset.HeadRef to be expected in CreateChangeset/UpdateChangeset calls.
	WantHeadRef string
	// The Changeset.BaseRef to be expected in CreateChangeset/UpdateChangeset calls.
	WantBaseRef string

	// The metadata the FakeChangesetSource should set on the created/updated
	// Changeset with changeset.SetMetadata.
	FakeMetadata any

	// Whether or not the changeset already ChangesetExists on the code host at the time
	// when CreateChangeset is called.
	ChangesetExists bool

	// When true, ValidateAuthenticator will return no error.
	AuthenticatorIsValid bool

	// error to be returned from every method
	Err error

	// ClosedChangesets contains the changesets that were passed to CloseChangeset
	ClosedChangesets []*sources.Changeset

	// CreatedChangesets contains the changesets that were passed to
	// CreateChangeset
	CreatedChangesets []*sources.Changeset

	// LoadedChangesets contains the changesets that were passed to LoadChangeset
	LoadedChangesets []*sources.Changeset

	// UpdateChangesets contains the changesets that were passed to
	// UpdateChangeset
	UpdatedChangesets []*sources.Changeset

	// ReopenedChangesets contains the changesets that were passed to ReopenedChangeset
	ReopenedChangesets []*sources.Changeset

	// UndraftedChangesets contains the changesets that were passed to UndraftChangeset
	UndraftedChangesets []*sources.Changeset

	// Username is the username returned by AuthenticatedUsername
	Username string

	// IsArchivedPushErrorTrue is returned when IsArchivedPushError is invoked.
	IsArchivedPushErrorTrue bool
}

var (
	_ sources.ChangesetSource           = &FakeChangesetSource{}
	_ sources.ArchivableChangesetSource = &FakeChangesetSource{}
	_ sources.DraftChangesetSource      = &FakeChangesetSource{}
)

func (s *FakeChangesetSource) CreateDraftChangeset(ctx context.Context, c *sources.Changeset) (bool, error) {
	s.CreateDraftChangesetCalled = true

	if s.Err != nil {
		return s.ChangesetExists, s.Err
	}

	if c.TargetRepo == nil {
		return false, noReposErr{name: "target"}
	}
	if c.RemoteRepo == nil {
		return false, noReposErr{name: "remote"}
	}

	if c.HeadRef != s.WantHeadRef {
		return s.ChangesetExists, errors.Errorf("wrong HeadRef. want=%s, have=%s", s.WantHeadRef, c.HeadRef)
	}

	if c.BaseRef != s.WantBaseRef {
		return s.ChangesetExists, errors.Errorf("wrong BaseRef. want=%s, have=%s", s.WantBaseRef, c.BaseRef)
	}

	if err := c.SetMetadata(s.FakeMetadata); err != nil {
		return s.ChangesetExists, err
	}

	s.CreatedChangesets = append(s.CreatedChangesets, c)
	return s.ChangesetExists, s.Err
}

func (s *FakeChangesetSource) UndraftChangeset(ctx context.Context, c *sources.Changeset) error {
	s.UndraftedChangesetsCalled = true

	if s.Err != nil {
		return s.Err
	}

	if c.TargetRepo == nil {
		return noReposErr{name: "target"}
	}
	if c.RemoteRepo == nil {
		return noReposErr{name: "remote"}
	}

	s.UndraftedChangesets = append(s.UndraftedChangesets, c)

	return c.SetMetadata(s.FakeMetadata)
}

func (s *FakeChangesetSource) CreateChangeset(ctx context.Context, c *sources.Changeset) (bool, error) {
	s.CreateChangesetCalled = true

	if s.Err != nil {
		return s.ChangesetExists, s.Err
	}

	if c.TargetRepo == nil {
		return false, noReposErr{name: "target"}
	}
	if c.RemoteRepo == nil {
		return false, noReposErr{name: "remote"}
	}

	if c.HeadRef != s.WantHeadRef {
		return s.ChangesetExists, errors.Errorf("wrong HeadRef. want=%s, have=%s", s.WantHeadRef, c.HeadRef)
	}

	if c.BaseRef != s.WantBaseRef {
		return s.ChangesetExists, errors.Errorf("wrong BaseRef. want=%s, have=%s", s.WantBaseRef, c.BaseRef)
	}

	if err := c.SetMetadata(s.FakeMetadata); err != nil {
		return s.ChangesetExists, err
	}

	s.CreatedChangesets = append(s.CreatedChangesets, c)
	return s.ChangesetExists, s.Err
}

func (s *FakeChangesetSource) UpdateChangeset(ctx context.Context, c *sources.Changeset) error {
	s.UpdateChangesetCalled = true

	if s.Err != nil {
		return s.Err
	}
	if c.TargetRepo == nil {
		return noReposErr{name: "target"}
	}
	if c.RemoteRepo == nil {
		return noReposErr{name: "remote"}
	}

	if c.BaseRef != s.WantBaseRef {
		return errors.Errorf("wrong BaseRef. want=%s, have=%s", s.WantBaseRef, c.BaseRef)
	}

	s.UpdatedChangesets = append(s.UpdatedChangesets, c)
	return c.SetMetadata(s.FakeMetadata)
}

func (s *FakeChangesetSource) ExternalServices() types.ExternalServices {
	s.ExternalServicesCalled = true

	return types.ExternalServices{s.Svc}
}

func (s *FakeChangesetSource) LoadChangeset(ctx context.Context, c *sources.Changeset) error {
	s.LoadChangesetCalled = true

	if s.Err != nil {
		return s.Err
	}

	if c.TargetRepo == nil {
		return noReposErr{name: "target"}
	}
	if c.RemoteRepo == nil {
		return noReposErr{name: "remote"}
	}

	if err := c.SetMetadata(s.FakeMetadata); err != nil {
		return err
	}

	s.LoadedChangesets = append(s.LoadedChangesets, c)
	return nil
}

func (s *FakeChangesetSource) CloseChangeset(ctx context.Context, c *sources.Changeset) error {
	s.CloseChangesetCalled = true

	if s.Err != nil {
		return s.Err
	}

	if c.TargetRepo == nil {
		return noReposErr{name: "target"}
	}
	if c.RemoteRepo == nil {
		return noReposErr{name: "remote"}
	}

	s.ClosedChangesets = append(s.ClosedChangesets, c)

	return c.SetMetadata(s.FakeMetadata)
}

func (s *FakeChangesetSource) ReopenChangeset(ctx context.Context, c *sources.Changeset) error {
	s.ReopenChangesetCalled = true

	if s.Err != nil {
		return s.Err
	}

	if c.TargetRepo == nil {
		return noReposErr{name: "target"}
	}
	if c.RemoteRepo == nil {
		return noReposErr{name: "remote"}
	}

	s.ReopenedChangesets = append(s.ReopenedChangesets, c)

	return c.SetMetadata(s.FakeMetadata)
}

func (s *FakeChangesetSource) CreateComment(ctx context.Context, c *sources.Changeset, body string) error {
	s.CreateCommentCalled = true
	return s.Err
}

func (s *FakeChangesetSource) GitserverPushConfig(repo *types.Repo) (*protocol.PushConfig, error) {
	return sources.GitserverPushConfig(repo, s.CurrentAuthenticator)
}

func (s *FakeChangesetSource) WithAuthenticator(a auth.Authenticator) (sources.ChangesetSource, error) {
	s.CurrentAuthenticator = a
	return s, nil
}

func (s *FakeChangesetSource) ValidateAuthenticator(context.Context) error {
	s.ValidateAuthenticatorCalled = true
	if s.AuthenticatorIsValid {
		return nil
	}
	return errors.New("invalid authenticator in fake source")
}

func (s *FakeChangesetSource) AuthenticatedUsername(ctx context.Context) (string, error) {
	s.AuthenticatedUsernameCalled = true
	return s.Username, nil
}

func (s *FakeChangesetSource) MergeChangeset(ctx context.Context, c *sources.Changeset, squash bool) error {
	s.MergeChangesetCalled = true
	return s.Err
}

func (s *FakeChangesetSource) IsArchivedPushError(output string) bool {
	s.IsArchivedPushErrorCalled = true
	return s.IsArchivedPushErrorTrue
}

func (s *FakeChangesetSource) BuildCommitOpts(repo *types.Repo, _ *btypes.Changeset, spec *btypes.ChangesetSpec, cfg *protocol.PushConfig) protocol.CreateCommitFromPatchRequest {
	s.BuildCommitOptsCalled = true
	return sources.BuildCommitOptsCommon(repo, spec, cfg)
}

type noReposErr struct{ name string }

func (e noReposErr) Error() string {
	return "no " + e.name + " repository set on Changeset"
}
