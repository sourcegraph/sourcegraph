package reconciler

import (
	"testing"

	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
)

func TestDetermineReconcilerPlan(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		name           string
		previousSpec   ct.TestSpecOpts
		currentSpec    ct.TestSpecOpts
		changeset      ct.TestChangesetOpts
		wantOperations Operations
	}{
		{
			name:        "publish true",
			currentSpec: ct.TestSpecOpts{Published: true},
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStateUnpublished,
			},
			wantOperations: Operations{
				batches.ReconcilerOperationPush,
				batches.ReconcilerOperationPublish,
			},
		},
		{
			name:        "publish as draft",
			currentSpec: ct.TestSpecOpts{Published: "draft"},
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStateUnpublished,
			},
			wantOperations: Operations{batches.ReconcilerOperationPush, batches.ReconcilerOperationPublishDraft},
		},
		{
			name:        "publish false",
			currentSpec: ct.TestSpecOpts{Published: false},
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStateUnpublished,
			},
			wantOperations: Operations{},
		},
		{
			name:        "draft but unsupported",
			currentSpec: ct.TestSpecOpts{Published: "draft"},
			changeset: ct.TestChangesetOpts{
				ExternalServiceType: extsvc.TypeBitbucketServer,
				PublicationState:    batches.ChangesetPublicationStateUnpublished,
			},
			// should be a noop
			wantOperations: Operations{},
		},
		{
			name:         "draft to publish true",
			previousSpec: ct.TestSpecOpts{Published: "draft"},
			currentSpec:  ct.TestSpecOpts{Published: true},
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStatePublished,
			},
			wantOperations: Operations{batches.ReconcilerOperationUndraft},
		},
		{
			name:         "draft to publish true on unpublished changeset",
			previousSpec: ct.TestSpecOpts{Published: "draft"},
			currentSpec:  ct.TestSpecOpts{Published: true},
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStateUnpublished,
			},
			wantOperations: Operations{batches.ReconcilerOperationPush, batches.ReconcilerOperationPublish},
		},
		{
			name:         "title changed on published changeset",
			previousSpec: ct.TestSpecOpts{Published: true, Title: "Before"},
			currentSpec:  ct.TestSpecOpts{Published: true, Title: "After"},
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStatePublished,
			},
			wantOperations: Operations{batches.ReconcilerOperationUpdate},
		},
		{
			name:         "commit diff changed on published changeset",
			previousSpec: ct.TestSpecOpts{Published: true, CommitDiff: "testDiff"},
			currentSpec:  ct.TestSpecOpts{Published: true, CommitDiff: "newTestDiff"},
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStatePublished,
			},
			wantOperations: Operations{
				batches.ReconcilerOperationPush,
				batches.ReconcilerOperationSleep,
				batches.ReconcilerOperationSync,
			},
		},
		{
			name:         "commit message changed on published changeset",
			previousSpec: ct.TestSpecOpts{Published: true, CommitMessage: "old message"},
			currentSpec:  ct.TestSpecOpts{Published: true, CommitMessage: "new message"},
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStatePublished,
			},
			wantOperations: Operations{
				batches.ReconcilerOperationPush,
				batches.ReconcilerOperationSleep,
				batches.ReconcilerOperationSync,
			},
		},
		{
			name:         "commit diff changed on merge changeset",
			previousSpec: ct.TestSpecOpts{Published: true, CommitDiff: "testDiff"},
			currentSpec:  ct.TestSpecOpts{Published: true, CommitDiff: "newTestDiff"},
			changeset: ct.TestChangesetOpts{
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalState:    batches.ChangesetExternalStateMerged,
			},
			// should be a noop
			wantOperations: Operations{},
		},
		{
			name:         "changeset closed-and-detached will reopen",
			previousSpec: ct.TestSpecOpts{Published: true},
			currentSpec:  ct.TestSpecOpts{Published: true},
			changeset: ct.TestChangesetOpts{
				PublicationState:   batches.ChangesetPublicationStatePublished,
				ExternalState:      batches.ChangesetExternalStateClosed,
				OwnedByBatchChange: 1234,
				BatchChanges:       []batches.BatchChangeAssoc{{BatchChangeID: 1234}},
			},
			wantOperations: Operations{
				batches.ReconcilerOperationReopen,
			},
		},
		{
			name:         "closing",
			previousSpec: ct.TestSpecOpts{Published: true},
			currentSpec:  ct.TestSpecOpts{Published: true},
			changeset: ct.TestChangesetOpts{
				PublicationState:   batches.ChangesetPublicationStatePublished,
				ExternalState:      batches.ChangesetExternalStateOpen,
				OwnedByBatchChange: 1234,
				BatchChanges:       []batches.BatchChangeAssoc{{BatchChangeID: 1234}},
				// Important bit:
				Closing: true,
			},
			wantOperations: Operations{
				batches.ReconcilerOperationClose,
			},
		},
		{
			name:         "closing already-closed changeset",
			previousSpec: ct.TestSpecOpts{Published: true},
			currentSpec:  ct.TestSpecOpts{Published: true},
			changeset: ct.TestChangesetOpts{
				PublicationState:   batches.ChangesetPublicationStatePublished,
				ExternalState:      batches.ChangesetExternalStateClosed,
				OwnedByBatchChange: 1234,
				BatchChanges:       []batches.BatchChangeAssoc{{BatchChangeID: 1234}},
				// Important bit:
				Closing: true,
			},
			wantOperations: Operations{
				// TODO: This should probably be a noop in the future
				batches.ReconcilerOperationClose,
			},
		},
		{
			name:         "detaching",
			previousSpec: ct.TestSpecOpts{Published: true},
			currentSpec:  ct.TestSpecOpts{Published: true},
			changeset: ct.TestChangesetOpts{
				PublicationState:   batches.ChangesetPublicationStatePublished,
				ExternalState:      batches.ChangesetExternalStateOpen,
				OwnedByBatchChange: 1234,
				BatchChanges:       []batches.BatchChangeAssoc{{BatchChangeID: 1234, Detach: true}},
			},
			wantOperations: Operations{
				batches.ReconcilerOperationDetach,
			},
		},
		{
			name:         "detaching already-detached changeset",
			previousSpec: ct.TestSpecOpts{Published: true},
			currentSpec:  ct.TestSpecOpts{Published: true},
			changeset: ct.TestChangesetOpts{
				PublicationState:   batches.ChangesetPublicationStatePublished,
				ExternalState:      batches.ChangesetExternalStateClosed,
				OwnedByBatchChange: 1234,
				BatchChanges:       []batches.BatchChangeAssoc{},
			},
			wantOperations: Operations{
				// Expect no operations.
			},
		},
		{
			name:        "detaching a failed publish changeset",
			currentSpec: ct.TestSpecOpts{Published: true},
			changeset: ct.TestChangesetOpts{
				PublicationState:   batches.ChangesetPublicationStateUnpublished,
				ReconcilerState:    batches.ReconcilerStateFailed,
				OwnedByBatchChange: 1234,
				BatchChanges:       []batches.BatchChangeAssoc{{BatchChangeID: 1234, Detach: true}},
			},
			wantOperations: Operations{
				batches.ReconcilerOperationDetach,
			},
		},
		{
			name: "detaching a failed importing changeset",
			changeset: ct.TestChangesetOpts{
				ExternalID:       "123",
				PublicationState: batches.ChangesetPublicationStateUnpublished,
				ReconcilerState:  batches.ReconcilerStateFailed,
				BatchChanges:     []batches.BatchChangeAssoc{{BatchChangeID: 1234, Detach: true}},
			},
			wantOperations: Operations{
				batches.ReconcilerOperationDetach,
			},
		},
		{
			name: "import changeset",
			changeset: ct.TestChangesetOpts{
				ExternalID:       "123",
				PublicationState: batches.ChangesetPublicationStateUnpublished,
				ReconcilerState:  batches.ReconcilerStateQueued,
				BatchChanges:     []batches.BatchChangeAssoc{{BatchChangeID: 1234}},
			},
			wantOperations: Operations{
				batches.ReconcilerOperationImport,
			},
		},
		{
			name: "detaching an importing changeset but remains imported by another",
			changeset: ct.TestChangesetOpts{
				ExternalID:       "123",
				PublicationState: batches.ChangesetPublicationStateUnpublished,
				ReconcilerState:  batches.ReconcilerStateQueued,
				BatchChanges:     []batches.BatchChangeAssoc{{BatchChangeID: 1234, Detach: true}, {BatchChangeID: 2345}},
			},
			wantOperations: Operations{
				batches.ReconcilerOperationDetach,
				batches.ReconcilerOperationImport,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var previousSpec, currentSpec *batches.ChangesetSpec
			if tc.previousSpec != (ct.TestSpecOpts{}) {
				previousSpec = ct.BuildChangesetSpec(t, tc.previousSpec)
			}

			if tc.currentSpec != (ct.TestSpecOpts{}) {
				currentSpec = ct.BuildChangesetSpec(t, tc.currentSpec)
			}

			cs := ct.BuildChangeset(tc.changeset)

			plan, err := DeterminePlan(previousSpec, currentSpec, cs)
			if err != nil {
				t.Fatal(err)
			}
			if have, want := plan.Ops, tc.wantOperations; !have.Equal(want) {
				t.Fatalf("incorrect plan determined, want=%v have=%v", want, have)
			}
		})
	}
}
