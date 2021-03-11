package testing

import (
	"context"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/go-diff/diff"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

type TestChangesetOpts struct {
	Repo         api.RepoID
	BatchChange  int64
	CurrentSpec  int64
	PreviousSpec int64
	BatchChanges []batches.BatchChangeAssoc

	ExternalServiceType string
	ExternalID          string
	ExternalBranch      string
	ExternalState       batches.ChangesetExternalState
	ExternalReviewState batches.ChangesetReviewState
	ExternalCheckState  batches.ChangesetCheckState

	DiffStatAdded   int32
	DiffStatChanged int32
	DiffStatDeleted int32

	PublicationState batches.ChangesetPublicationState

	ReconcilerState batches.ReconcilerState
	FailureMessage  string
	NumFailures     int64

	OwnedByBatchChange int64

	Closing bool

	Metadata interface{}
}

type CreateChangeseter interface {
	CreateChangeset(ctx context.Context, changeset *batches.Changeset) error
}

func CreateChangeset(
	t *testing.T,
	ctx context.Context,
	store CreateChangeseter,
	opts TestChangesetOpts,
) *batches.Changeset {
	t.Helper()

	changeset := BuildChangeset(opts)

	if err := store.CreateChangeset(ctx, changeset); err != nil {
		t.Fatalf("creating changeset failed: %s", err)
	}

	return changeset
}

func BuildChangeset(opts TestChangesetOpts) *batches.Changeset {
	if opts.ExternalServiceType == "" {
		opts.ExternalServiceType = extsvc.TypeGitHub
	}

	changeset := &batches.Changeset{
		RepoID:         opts.Repo,
		CurrentSpecID:  opts.CurrentSpec,
		PreviousSpecID: opts.PreviousSpec,
		BatchChanges:   opts.BatchChanges,

		ExternalServiceType: opts.ExternalServiceType,
		ExternalID:          opts.ExternalID,
		ExternalState:       opts.ExternalState,
		ExternalReviewState: opts.ExternalReviewState,
		ExternalCheckState:  opts.ExternalCheckState,

		PublicationState: opts.PublicationState,

		OwnedByBatchChangeID: opts.OwnedByBatchChange,

		Closing: opts.Closing,

		ReconcilerState: opts.ReconcilerState,
		NumFailures:     opts.NumFailures,

		Metadata: opts.Metadata,
	}

	if opts.ExternalBranch != "" {
		changeset.ExternalBranch = git.EnsureRefPrefix(opts.ExternalBranch)
	}

	if opts.FailureMessage != "" {
		changeset.FailureMessage = &opts.FailureMessage
	}

	if opts.BatchChange != 0 {
		changeset.BatchChanges = []batches.BatchChangeAssoc{{BatchChangeID: opts.BatchChange}}
	}

	if opts.DiffStatAdded > 0 || opts.DiffStatChanged > 0 || opts.DiffStatDeleted > 0 {
		changeset.DiffStatAdded = &opts.DiffStatAdded
		changeset.DiffStatChanged = &opts.DiffStatChanged
		changeset.DiffStatDeleted = &opts.DiffStatDeleted
	}

	return changeset
}

type ChangesetAssertions struct {
	Repo               api.RepoID
	CurrentSpec        int64
	PreviousSpec       int64
	OwnedByBatchChange int64
	ReconcilerState    batches.ReconcilerState
	PublicationState   batches.ChangesetPublicationState
	ExternalState      batches.ChangesetExternalState
	ExternalID         string
	ExternalBranch     string
	DiffStat           *diff.Stat
	Closing            bool

	Title string
	Body  string

	FailureMessage   *string
	SyncErrorMessage *string
	NumFailures      int64
	NumResets        int64

	AttachedTo []int64
	DetachFrom []int64
}

func AssertChangeset(t *testing.T, c *batches.Changeset, a ChangesetAssertions) {
	t.Helper()

	if c == nil {
		t.Fatalf("changeset is nil")
	}

	if have, want := c.RepoID, a.Repo; have != want {
		t.Fatalf("changeset RepoID wrong. want=%d, have=%d", want, have)
	}

	if have, want := c.CurrentSpecID, a.CurrentSpec; have != want {
		t.Fatalf("changeset CurrentSpecID wrong. want=%d, have=%d", want, have)
	}

	if have, want := c.PreviousSpecID, a.PreviousSpec; have != want {
		t.Fatalf("changeset PreviousSpecID wrong. want=%d, have=%d", want, have)
	}

	if have, want := c.OwnedByBatchChangeID, a.OwnedByBatchChange; have != want {
		t.Fatalf("changeset OwnedByBatchChangeID wrong. want=%d, have=%d", want, have)
	}

	if have, want := c.ReconcilerState, a.ReconcilerState; have != want {
		t.Fatalf("changeset ReconcilerState wrong. want=%s, have=%s", want, have)
	}

	if have, want := c.PublicationState, a.PublicationState; have != want {
		t.Fatalf("changeset PublicationState wrong. want=%s, have=%s", want, have)
	}

	if have, want := c.ExternalState, a.ExternalState; have != want {
		t.Fatalf("changeset ExternalState wrong. want=%s, have=%s", want, have)
	}

	if have, want := c.ExternalID, a.ExternalID; have != want {
		t.Fatalf("changeset ExternalID wrong. want=%s, have=%s", want, have)
	}

	if have, want := c.ExternalBranch, a.ExternalBranch; have != want {
		t.Fatalf("changeset ExternalBranch wrong. want=%s, have=%s", want, have)
	}

	if want, have := a.FailureMessage, c.FailureMessage; want == nil && have != nil {
		t.Fatalf("expected no failure message, but have=%q", *have)
	}

	if diff := cmp.Diff(a.DiffStat, c.DiffStat()); diff != "" {
		t.Fatalf("changeset DiffStat wrong. (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(a.Closing, c.Closing); diff != "" {
		t.Fatalf("changeset Closing wrong. (-want +got):\n%s", diff)
	}

	toDetach := []int64{}
	for _, assoc := range c.BatchChanges {
		if assoc.Detach {
			toDetach = append(toDetach, assoc.BatchChangeID)
		}
	}
	if a.DetachFrom == nil {
		a.DetachFrom = []int64{}
	}
	sort.Slice(toDetach, func(i, j int) bool { return toDetach[i] < toDetach[j] })
	sort.Slice(a.DetachFrom, func(i, j int) bool { return a.DetachFrom[i] < a.DetachFrom[j] })
	if diff := cmp.Diff(a.DetachFrom, toDetach); diff != "" {
		t.Fatalf("changeset DetachFrom wrong. (-want +got):\n%s", diff)
	}

	attachedTo := []int64{}
	for _, assoc := range c.BatchChanges {
		if !assoc.Detach {
			attachedTo = append(attachedTo, assoc.BatchChangeID)
		}
	}
	if a.AttachedTo == nil {
		a.AttachedTo = []int64{}
	}
	sort.Slice(attachedTo, func(i, j int) bool { return attachedTo[i] < attachedTo[j] })
	sort.Slice(a.AttachedTo, func(i, j int) bool { return a.AttachedTo[i] < a.AttachedTo[j] })
	if diff := cmp.Diff(a.AttachedTo, attachedTo); diff != "" {
		t.Fatalf("changeset AttachedTo wrong. (-want +got):\n%s", diff)
	}

	if want := c.FailureMessage; want != nil {
		if c.FailureMessage == nil {
			t.Fatalf("expected failure message %q but have none", *want)
		}
		if want, have := *a.FailureMessage, *c.FailureMessage; have != want {
			t.Fatalf("wrong failure message. want=%q, have=%q", want, have)
		}
	}

	if want := c.SyncErrorMessage; want != nil {
		if c.SyncErrorMessage == nil {
			t.Fatalf("expected sync error message %q but have none", *want)
		}
		if want, have := *a.SyncErrorMessage, *c.SyncErrorMessage; have != want {
			t.Fatalf("wrong sync error message. want=%q, have=%q", want, have)
		}
	}

	if have, want := c.NumFailures, a.NumFailures; have != want {
		t.Fatalf("changeset NumFailures wrong. want=%d, have=%d", want, have)
	}

	if have, want := c.NumResets, a.NumResets; have != want {
		t.Fatalf("changeset NumResets wrong. want=%d, have=%d", want, have)
	}

	if have, want := c.ExternalBranch, a.ExternalBranch; have != want {
		t.Fatalf("changeset ExternalBranch wrong. want=%s, have=%s", want, have)
	}

	if want := a.Title; want != "" {
		have, err := c.Title()
		if err != nil {
			t.Fatalf("changeset.Title failed: %s", err)
		}

		if have != want {
			t.Fatalf("changeset Title wrong. want=%s, have=%s", want, have)
		}
	}

	if want := a.Body; want != "" {
		have, err := c.Body()
		if err != nil {
			t.Fatalf("changeset.Body failed: %s", err)
		}

		if have != want {
			t.Fatalf("changeset Body wrong. want=%s, have=%s", want, have)
		}
	}
}

type GetChangesetByIDer interface {
	GetChangesetByID(ctx context.Context, id int64) (*batches.Changeset, error)
}

func ReloadAndAssertChangeset(t *testing.T, ctx context.Context, s GetChangesetByIDer, c *batches.Changeset, a ChangesetAssertions) (reloaded *batches.Changeset) {
	t.Helper()

	reloaded, err := s.GetChangesetByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("reloading changeset %d failed: %s", c.ID, err)
	}

	AssertChangeset(t, reloaded, a)

	return reloaded
}

type UpdateChangeseter interface {
	UpdateChangeset(ctx context.Context, changeset *batches.Changeset) error
}

func SetChangesetPublished(t *testing.T, ctx context.Context, s UpdateChangeseter, c *batches.Changeset, externalID, externalBranch string) {
	t.Helper()

	c.ExternalBranch = externalBranch
	c.ExternalID = externalID
	c.PublicationState = batches.ChangesetPublicationStatePublished
	c.ReconcilerState = batches.ReconcilerStateCompleted
	c.ExternalState = batches.ChangesetExternalStateOpen

	if err := s.UpdateChangeset(ctx, c); err != nil {
		t.Fatalf("failed to update changeset: %s", err)
	}
}

var FailedChangesetFailureMessage = "Failed test"

func SetChangesetFailed(t *testing.T, ctx context.Context, s UpdateChangeseter, c *batches.Changeset) {
	t.Helper()

	c.ReconcilerState = batches.ReconcilerStateFailed
	c.FailureMessage = &FailedChangesetFailureMessage
	c.NumFailures = 5

	if err := s.UpdateChangeset(ctx, c); err != nil {
		t.Fatalf("failed to update changeset: %s", err)
	}
}

func SetChangesetClosed(t *testing.T, ctx context.Context, s UpdateChangeseter, c *batches.Changeset) {
	t.Helper()

	c.PublicationState = batches.ChangesetPublicationStatePublished
	c.ReconcilerState = batches.ReconcilerStateCompleted
	c.Closing = false
	c.ExternalState = batches.ChangesetExternalStateClosed

	assocs := make([]batches.BatchChangeAssoc, 0)
	for _, assoc := range c.BatchChanges {
		if !assoc.Detach {
			assocs = append(assocs, assoc)
		}
	}

	c.BatchChanges = assocs

	if err := s.UpdateChangeset(ctx, c); err != nil {
		t.Fatalf("failed to update changeset: %s", err)
	}
}
