package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/reconciler"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

func TestServiceApplyBatchChange(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	db := dbtesting.GetDB(t)

	admin := ct.CreateTestUser(t, db, true)
	adminCtx := actor.WithActor(context.Background(), actor.FromUser(admin.ID))

	user := ct.CreateTestUser(t, db, false)
	userCtx := actor.WithActor(context.Background(), actor.FromUser(user.ID))

	repos, _ := ct.CreateTestRepos(t, ctx, db, 4)

	now := timeutil.Now()
	clock := func() time.Time { return now }
	store := store.NewWithClock(db, clock)
	svc := New(store)

	t.Run("BatchSpec without changesetSpecs", func(t *testing.T) {
		t.Run("new batch change", func(t *testing.T) {
			ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
			batchSpec := ct.CreateBatchSpec(t, ctx, store, "batchchange1", admin.ID)
			batchChange, err := svc.ApplyBatchChange(adminCtx, ApplyBatchChangeOpts{
				BatchSpecRandID: batchSpec.RandID,
			})
			if err != nil {
				t.Fatal(err)
			}

			if batchChange.ID == 0 {
				t.Fatalf("batch change ID is 0")
			}

			want := &batches.BatchChange{
				Name:             batchSpec.Spec.Name,
				Description:      batchSpec.Spec.Description,
				InitialApplierID: admin.ID,
				LastApplierID:    admin.ID,
				LastAppliedAt:    now,
				NamespaceUserID:  batchSpec.NamespaceUserID,
				BatchSpecID:      batchSpec.ID,

				// Ignore these fields
				ID:        batchChange.ID,
				UpdatedAt: batchChange.UpdatedAt,
				CreatedAt: batchChange.CreatedAt,
			}

			if diff := cmp.Diff(want, batchChange); diff != "" {
				t.Fatalf("wrong spec fields (-want +got):\n%s", diff)
			}
		})

		t.Run("existing batch change", func(t *testing.T) {
			ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
			batchSpec := ct.CreateBatchSpec(t, ctx, store, "batchchange2", admin.ID)
			batchChange := ct.CreateBatchChange(t, ctx, store, "batchchange2", admin.ID, batchSpec.ID)

			t.Run("apply same BatchSpec", func(t *testing.T) {
				batchChange2, err := svc.ApplyBatchChange(adminCtx, ApplyBatchChangeOpts{
					BatchSpecRandID: batchSpec.RandID,
				})
				if err != nil {
					t.Fatal(err)
				}

				if have, want := batchChange2.ID, batchChange.ID; have != want {
					t.Fatalf("batch change ID is wrong. want=%d, have=%d", want, have)
				}
			})

			t.Run("apply same BatchSpec with FailIfExists", func(t *testing.T) {
				_, err := svc.ApplyBatchChange(ctx, ApplyBatchChangeOpts{
					BatchSpecRandID:         batchSpec.RandID,
					FailIfBatchChangeExists: true,
				})
				if err != ErrMatchingBatchChangeExists {
					t.Fatalf("unexpected error. want=%s, got=%s", ErrMatchingBatchChangeExists, err)
				}
			})

			t.Run("apply batch spec with same name", func(t *testing.T) {
				batchSpec2 := ct.CreateBatchSpec(t, ctx, store, "batchchange2", admin.ID)
				batchChange2, err := svc.ApplyBatchChange(adminCtx, ApplyBatchChangeOpts{
					BatchSpecRandID: batchSpec2.RandID,
				})
				if err != nil {
					t.Fatal(err)
				}

				if have, want := batchChange2.ID, batchChange.ID; have != want {
					t.Fatalf("batch change ID is wrong. want=%d, have=%d", want, have)
				}
			})

			t.Run("apply batch spec with same name but different current user", func(t *testing.T) {
				batchSpec := ct.CreateBatchSpec(t, ctx, store, "created-by-user", user.ID)
				batchChange := ct.CreateBatchChange(t, ctx, store, "created-by-user", user.ID, batchSpec.ID)

				if have, want := batchChange.InitialApplierID, user.ID; have != want {
					t.Fatalf("batch change InitialApplierID is wrong. want=%d, have=%d", want, have)
				}

				if have, want := batchChange.LastApplierID, user.ID; have != want {
					t.Fatalf("batch change LastApplierID is wrong. want=%d, have=%d", want, have)
				}

				batchSpec2 := ct.CreateBatchSpec(t, ctx, store, "created-by-user", user.ID)
				batchChange2, err := svc.ApplyBatchChange(adminCtx, ApplyBatchChangeOpts{
					BatchSpecRandID: batchSpec2.RandID,
				})
				if err != nil {
					t.Fatal(err)
				}

				if have, want := batchChange2.ID, batchChange.ID; have != want {
					t.Fatalf("batch change ID is wrong. want=%d, have=%d", want, have)
				}

				if have, want := batchChange2.InitialApplierID, batchChange.InitialApplierID; have != want {
					t.Fatalf("batch change InitialApplierID is wrong. want=%d, have=%d", want, have)
				}

				if have, want := batchChange2.LastApplierID, admin.ID; have != want {
					t.Fatalf("batch change LastApplierID is wrong. want=%d, have=%d", want, have)
				}
			})

			t.Run("apply batch spec with same name but different namespace", func(t *testing.T) {
				user2 := ct.CreateTestUser(t, db, false)
				batchSpec2 := ct.CreateBatchSpec(t, ctx, store, "batchchange2", user2.ID)

				batchChange2, err := svc.ApplyBatchChange(adminCtx, ApplyBatchChangeOpts{
					BatchSpecRandID: batchSpec2.RandID,
				})
				if err != nil {
					t.Fatal(err)
				}

				if batchChange2.ID == 0 {
					t.Fatalf("batchChange2 ID is 0")
				}

				if batchChange2.ID == batchChange.ID {
					t.Fatalf("batch change IDs are the same, but want different")
				}
			})

			t.Run("batch spec with same name and same ensureBatchChangeID", func(t *testing.T) {
				batchSpec2 := ct.CreateBatchSpec(t, ctx, store, "batchchange2", admin.ID)

				batchChange2, err := svc.ApplyBatchChange(adminCtx, ApplyBatchChangeOpts{
					BatchSpecRandID:     batchSpec2.RandID,
					EnsureBatchChangeID: batchChange.ID,
				})
				if err != nil {
					t.Fatal(err)
				}
				if have, want := batchChange2.ID, batchChange.ID; have != want {
					t.Fatalf("batch change has wrong ID. want=%d, have=%d", want, have)
				}
			})

			t.Run("batch spec with same name but different ensureBatchChangeID", func(t *testing.T) {
				batchSpec2 := ct.CreateBatchSpec(t, ctx, store, "batchchange2", admin.ID)

				_, err := svc.ApplyBatchChange(adminCtx, ApplyBatchChangeOpts{
					BatchSpecRandID:     batchSpec2.RandID,
					EnsureBatchChangeID: batchChange.ID + 999,
				})
				if err != ErrEnsureBatchChangeFailed {
					t.Fatalf("wrong error: %s", err)
				}
			})
		})
	})

	// These tests focus on changesetSpecs and wiring them up with changesets.
	// The applying/re-applying of a batchSpec to an existing batch change is
	// covered in the tests above.
	t.Run("batchSpec with changesetSpecs", func(t *testing.T) {
		t.Run("new batch change", func(t *testing.T) {
			ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
			batchSpec := ct.CreateBatchSpec(t, ctx, store, "batchchange3", admin.ID)

			spec1 := ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:       admin.ID,
				Repo:       repos[0].ID,
				BatchSpec:  batchSpec.ID,
				ExternalID: "1234",
			})

			spec2 := ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:      admin.ID,
				Repo:      repos[1].ID,
				BatchSpec: batchSpec.ID,
				HeadRef:   "refs/heads/my-branch",
			})

			batchChange, cs := applyAndListChangesets(adminCtx, t, svc, batchSpec.RandID, 2)

			if have, want := batchChange.Name, "batchchange3"; have != want {
				t.Fatalf("wrong batch change name. want=%s, have=%s", want, have)
			}

			c1 := cs.Find(batches.WithExternalID(spec1.Spec.ExternalID))
			ct.AssertChangeset(t, c1, ct.ChangesetAssertions{
				Repo:             spec1.RepoID,
				ExternalID:       "1234",
				ReconcilerState:  batches.ReconcilerStateQueued,
				PublicationState: batches.ChangesetPublicationStateUnpublished,
				AttachedTo:       []int64{batchChange.ID},
			})

			c2 := cs.Find(batches.WithCurrentSpecID(spec2.ID))
			ct.AssertChangeset(t, c2, ct.ChangesetAssertions{
				Repo:               spec2.RepoID,
				CurrentSpec:        spec2.ID,
				OwnedByBatchChange: batchChange.ID,
				ReconcilerState:    batches.ReconcilerStateQueued,
				PublicationState:   batches.ChangesetPublicationStateUnpublished,
				DiffStat:           ct.TestChangsetSpecDiffStat,
				AttachedTo:         []int64{batchChange.ID},
			})
		})

		t.Run("batch change with changesets", func(t *testing.T) {
			ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
			// First we create a batchSpec and apply it, so that we have
			// changesets and changesetSpecs in the database, wired up
			// correctly.
			batchSpec1 := ct.CreateBatchSpec(t, ctx, store, "batchchange4", admin.ID)

			ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:       admin.ID,
				Repo:       repos[0].ID,
				BatchSpec:  batchSpec1.ID,
				ExternalID: "1234",
			})

			ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:       admin.ID,
				Repo:       repos[0].ID,
				BatchSpec:  batchSpec1.ID,
				ExternalID: "5678",
			})

			oldSpec3 := ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:      admin.ID,
				Repo:      repos[1].ID,
				BatchSpec: batchSpec1.ID,
				HeadRef:   "refs/heads/repo-1-branch-1",
			})

			oldSpec4 := ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:      admin.ID,
				Repo:      repos[2].ID,
				BatchSpec: batchSpec1.ID,
				HeadRef:   "refs/heads/repo-2-branch-1",
			})

			// Apply and expect 4 changesets
			oldBatchChange, oldChangesets := applyAndListChangesets(adminCtx, t, svc, batchSpec1.RandID, 4)

			// Now we create another batch spec with the same batch change name
			// and namespace.
			batchSpec2 := ct.CreateBatchSpec(t, ctx, store, "batchchange4", admin.ID)

			// Same
			spec1 := ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:       admin.ID,
				Repo:       repos[0].ID,
				BatchSpec:  batchSpec2.ID,
				ExternalID: "1234",
			})

			// DIFFERENT: Track #9999 in repo[0]
			spec2 := ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:       admin.ID,
				Repo:       repos[0].ID,
				BatchSpec:  batchSpec2.ID,
				ExternalID: "5678",
			})

			// Same
			spec3 := ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:      admin.ID,
				Repo:      repos[1].ID,
				BatchSpec: batchSpec2.ID,
				HeadRef:   "refs/heads/repo-1-branch-1",
			})

			// DIFFERENT: branch changed in repo[2]
			spec4 := ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:      admin.ID,
				Repo:      repos[2].ID,
				BatchSpec: batchSpec2.ID,
				HeadRef:   "refs/heads/repo-2-branch-2",
			})

			// NEW: repo[3]
			spec5 := ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:      admin.ID,
				Repo:      repos[3].ID,
				BatchSpec: batchSpec2.ID,
				HeadRef:   "refs/heads/repo-3-branch-1",
			})

			// Before we apply the new batch spec, we make the changeset we
			// expect to be closed to look "published", otherwise it won't be
			// closed.
			wantClosed := oldChangesets.Find(batches.WithCurrentSpecID(oldSpec4.ID))
			ct.SetChangesetPublished(t, ctx, store, wantClosed, "98765", oldSpec4.Spec.HeadRef)

			changeset3 := oldChangesets.Find(batches.WithCurrentSpecID(oldSpec3.ID))
			ct.SetChangesetPublished(t, ctx, store, changeset3, "12345", oldSpec3.Spec.HeadRef)

			// Apply and expect 6 changesets
			batchChange, cs := applyAndListChangesets(adminCtx, t, svc, batchSpec2.RandID, 6)

			if oldBatchChange.ID != batchChange.ID {
				t.Fatal("expected to update batch change, but got a new one")
			}

			// This changeset we want marked as "to be closed"
			ct.ReloadAndAssertChangeset(t, ctx, store, wantClosed, ct.ChangesetAssertions{
				Repo:         repos[2].ID,
				CurrentSpec:  oldSpec4.ID,
				PreviousSpec: oldSpec4.ID,
				ExternalID:   wantClosed.ExternalID,
				// It's still open, just _marked as to be closed_.
				ExternalState:      batches.ChangesetExternalStateOpen,
				ExternalBranch:     wantClosed.ExternalBranch,
				OwnedByBatchChange: batchChange.ID,
				ReconcilerState:    batches.ReconcilerStateQueued,
				PublicationState:   batches.ChangesetPublicationStatePublished,
				DiffStat:           ct.TestChangsetSpecDiffStat,
				DetachFrom:         []int64{batchChange.ID},
				Closing:            true,
			})

			c1 := cs.Find(batches.WithExternalID(spec1.Spec.ExternalID))
			ct.AssertChangeset(t, c1, ct.ChangesetAssertions{
				Repo:             repos[0].ID,
				CurrentSpec:      0,
				PreviousSpec:     0,
				ExternalID:       "1234",
				ReconcilerState:  batches.ReconcilerStateQueued,
				PublicationState: batches.ChangesetPublicationStateUnpublished,
				AttachedTo:       []int64{batchChange.ID},
			})

			c2 := cs.Find(batches.WithExternalID(spec2.Spec.ExternalID))
			ct.AssertChangeset(t, c2, ct.ChangesetAssertions{
				Repo:             repos[0].ID,
				CurrentSpec:      0,
				PreviousSpec:     0,
				ExternalID:       "5678",
				ReconcilerState:  batches.ReconcilerStateQueued,
				PublicationState: batches.ChangesetPublicationStateUnpublished,
				AttachedTo:       []int64{batchChange.ID},
			})

			c3 := cs.Find(batches.WithCurrentSpecID(spec3.ID))
			ct.AssertChangeset(t, c3, ct.ChangesetAssertions{
				Repo:           repos[1].ID,
				CurrentSpec:    spec3.ID,
				ExternalID:     changeset3.ExternalID,
				ExternalBranch: changeset3.ExternalBranch,
				ExternalState:  batches.ChangesetExternalStateOpen,
				// Has a previous spec, because it succeeded publishing.
				PreviousSpec:       oldSpec3.ID,
				OwnedByBatchChange: batchChange.ID,
				ReconcilerState:    batches.ReconcilerStateQueued,
				PublicationState:   batches.ChangesetPublicationStatePublished,
				DiffStat:           ct.TestChangsetSpecDiffStat,
				AttachedTo:         []int64{batchChange.ID},
			})

			c4 := cs.Find(batches.WithCurrentSpecID(spec4.ID))
			ct.AssertChangeset(t, c4, ct.ChangesetAssertions{
				Repo:               repos[2].ID,
				CurrentSpec:        spec4.ID,
				OwnedByBatchChange: batchChange.ID,
				ReconcilerState:    batches.ReconcilerStateQueued,
				PublicationState:   batches.ChangesetPublicationStateUnpublished,
				DiffStat:           ct.TestChangsetSpecDiffStat,
				AttachedTo:         []int64{batchChange.ID},
			})

			c5 := cs.Find(batches.WithCurrentSpecID(spec5.ID))
			ct.AssertChangeset(t, c5, ct.ChangesetAssertions{
				Repo:               repos[3].ID,
				CurrentSpec:        spec5.ID,
				OwnedByBatchChange: batchChange.ID,
				ReconcilerState:    batches.ReconcilerStateQueued,
				PublicationState:   batches.ChangesetPublicationStateUnpublished,
				DiffStat:           ct.TestChangsetSpecDiffStat,
				AttachedTo:         []int64{batchChange.ID},
			})
		})

		t.Run("batch change tracking changesets owned by another batch change", func(t *testing.T) {
			ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
			batchSpec1 := ct.CreateBatchSpec(t, ctx, store, "owner-batch-change", admin.ID)

			oldSpec1 := ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:      admin.ID,
				Repo:      repos[0].ID,
				BatchSpec: batchSpec1.ID,
				HeadRef:   "refs/heads/repo-0-branch-0",
			})

			ownerBatchChange, ownerChangesets := applyAndListChangesets(adminCtx, t, svc, batchSpec1.RandID, 1)

			// Now we update the changeset so it looks like it's been published
			// on the code host.
			c := ownerChangesets[0]
			ct.SetChangesetPublished(t, ctx, store, c, "88888", "refs/heads/repo-0-branch-0")

			// This other batch change tracks the changeset created by the first one
			batchSpec2 := ct.CreateBatchSpec(t, ctx, store, "tracking-batch-change", admin.ID)
			ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:       admin.ID,
				Repo:       c.RepoID,
				BatchSpec:  batchSpec2.ID,
				ExternalID: c.ExternalID,
			})

			trackingBatchChange, trackedChangesets := applyAndListChangesets(adminCtx, t, svc, batchSpec2.RandID, 1)
			// This should still point to the owner batch change
			c2 := trackedChangesets[0]
			trackedChangesetAssertions := ct.ChangesetAssertions{
				Repo:               c.RepoID,
				CurrentSpec:        oldSpec1.ID,
				OwnedByBatchChange: ownerBatchChange.ID,
				ExternalBranch:     c.ExternalBranch,
				ExternalID:         c.ExternalID,
				ExternalState:      batches.ChangesetExternalStateOpen,
				ReconcilerState:    batches.ReconcilerStateCompleted,
				PublicationState:   batches.ChangesetPublicationStatePublished,
				DiffStat:           ct.TestChangsetSpecDiffStat,
				AttachedTo:         []int64{ownerBatchChange.ID, trackingBatchChange.ID},
			}
			ct.AssertChangeset(t, c2, trackedChangesetAssertions)

			// Now try to apply a new spec that wants to modify the formerly tracked changeset.
			batchSpec3 := ct.CreateBatchSpec(t, ctx, store, "tracking-batch-change", admin.ID)

			spec3 := ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:      admin.ID,
				Repo:      repos[0].ID,
				BatchSpec: batchSpec3.ID,
				HeadRef:   "refs/heads/repo-0-branch-0",
			})
			// Apply again. This should have flagged the association as detach
			// and it should not be closed, since the batch change is not the
			// owner.
			trackingBatchChange, cs := applyAndListChangesets(adminCtx, t, svc, batchSpec3.RandID, 2)

			trackedChangesetAssertions.Closing = false
			trackedChangesetAssertions.ReconcilerState = batches.ReconcilerStateQueued
			trackedChangesetAssertions.DetachFrom = []int64{trackingBatchChange.ID}
			trackedChangesetAssertions.AttachedTo = []int64{ownerBatchChange.ID}
			ct.ReloadAndAssertChangeset(t, ctx, store, c2, trackedChangesetAssertions)

			// But we do want to have a new changeset record that is going to create a new changeset on the code host.
			ct.ReloadAndAssertChangeset(t, ctx, store, cs[1], ct.ChangesetAssertions{
				Repo:               spec3.RepoID,
				CurrentSpec:        spec3.ID,
				OwnedByBatchChange: trackingBatchChange.ID,
				ReconcilerState:    batches.ReconcilerStateQueued,
				PublicationState:   batches.ChangesetPublicationStateUnpublished,
				DiffStat:           ct.TestChangsetSpecDiffStat,
				AttachedTo:         []int64{trackingBatchChange.ID},
			})
		})

		t.Run("batch change with changeset that is unpublished", func(t *testing.T) {
			ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
			batchSpec1 := ct.CreateBatchSpec(t, ctx, store, "unpublished-changesets", admin.ID)

			ct.CreateChangesetSpec(t, ctx, store, ct.TestSpecOpts{
				User:      admin.ID,
				Repo:      repos[3].ID,
				BatchSpec: batchSpec1.ID,
				HeadRef:   "refs/heads/never-published",
			})

			// We apply the spec and expect 1 changeset
			applyAndListChangesets(adminCtx, t, svc, batchSpec1.RandID, 1)

			// But the changeset was not published yet.
			// And now we apply a new spec without any changesets.
			batchSpec2 := ct.CreateBatchSpec(t, ctx, store, "unpublished-changesets", admin.ID)

			// That should close no changesets, but set the unpublished changesets to be detached when
			// the reconciler picks them up.
			applyAndListChangesets(adminCtx, t, svc, batchSpec2.RandID, 1)
		})

		t.Run("batch change with changeset that wasn't processed before reapply", func(t *testing.T) {
			ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
			batchSpec1 := ct.CreateBatchSpec(t, ctx, store, "queued-changesets", admin.ID)

			specOpts := ct.TestSpecOpts{
				User:      admin.ID,
				Repo:      repos[3].ID,
				BatchSpec: batchSpec1.ID,
				Title:     "Spec1",
				HeadRef:   "refs/heads/queued",
				Published: true,
			}
			spec1 := ct.CreateChangesetSpec(t, ctx, store, specOpts)

			// We apply the spec and expect 1 changeset
			batchChange, changesets := applyAndListChangesets(adminCtx, t, svc, batchSpec1.RandID, 1)

			// And publish it.
			ct.SetChangesetPublished(t, ctx, store, changesets[0], "123-queued", "refs/heads/queued")

			ct.ReloadAndAssertChangeset(t, ctx, store, changesets[0], ct.ChangesetAssertions{
				ReconcilerState:    batches.ReconcilerStateCompleted,
				PublicationState:   batches.ChangesetPublicationStatePublished,
				ExternalBranch:     "refs/heads/queued",
				ExternalID:         "123-queued",
				ExternalState:      batches.ChangesetExternalStateOpen,
				Repo:               repos[3].ID,
				CurrentSpec:        spec1.ID,
				OwnedByBatchChange: batchChange.ID,
				DiffStat:           ct.TestChangsetSpecDiffStat,
				AttachedTo:         []int64{batchChange.ID},
			})

			// Apply again so that an update to the changeset is pending.
			batchSpec2 := ct.CreateBatchSpec(t, ctx, store, "queued-changesets", admin.ID)

			specOpts.BatchSpec = batchSpec2.ID
			specOpts.Title = "Spec2"
			spec2 := ct.CreateChangesetSpec(t, ctx, store, specOpts)

			// That should still want to publish the changeset
			_, changesets = applyAndListChangesets(adminCtx, t, svc, batchSpec2.RandID, 1)

			ct.ReloadAndAssertChangeset(t, ctx, store, changesets[0], ct.ChangesetAssertions{
				ReconcilerState:  batches.ReconcilerStateQueued,
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalBranch:   "refs/heads/queued",
				ExternalID:       "123-queued",
				ExternalState:    batches.ChangesetExternalStateOpen,
				Repo:             repos[3].ID,
				CurrentSpec:      spec2.ID,
				// Track the previous spec.
				PreviousSpec:       spec1.ID,
				OwnedByBatchChange: batchChange.ID,
				DiffStat:           ct.TestChangsetSpecDiffStat,
				AttachedTo:         []int64{batchChange.ID},
			})

			// Make sure the reconciler wants to update this changeset.
			plan, err := reconciler.DeterminePlan(
				// changesets[0].PreviousSpecID
				spec1,
				// changesets[0].CurrentSpecID
				spec2,
				changesets[0],
			)
			if err != nil {
				t.Fatal(err)
			}
			if !plan.Ops.Equal(reconciler.Operations{batches.ReconcilerOperationUpdate}) {
				t.Fatalf("Got invalid reconciler operations: %q", plan.Ops.String())
			}

			// And now we apply a new spec before the reconciler could process the changeset.
			batchSpec3 := ct.CreateBatchSpec(t, ctx, store, "queued-changesets", admin.ID)

			// No change this time, just reapplying.
			specOpts.BatchSpec = batchSpec3.ID
			spec3 := ct.CreateChangesetSpec(t, ctx, store, specOpts)

			_, changesets = applyAndListChangesets(adminCtx, t, svc, batchSpec3.RandID, 1)

			ct.ReloadAndAssertChangeset(t, ctx, store, changesets[0], ct.ChangesetAssertions{
				ReconcilerState:  batches.ReconcilerStateQueued,
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalBranch:   "refs/heads/queued",
				ExternalID:       "123-queued",
				ExternalState:    batches.ChangesetExternalStateOpen,
				Repo:             repos[3].ID,
				CurrentSpec:      spec3.ID,
				// Still be pointing at the first spec, since the second was never applied.
				PreviousSpec:       spec1.ID,
				OwnedByBatchChange: batchChange.ID,
				DiffStat:           ct.TestChangsetSpecDiffStat,
				AttachedTo:         []int64{batchChange.ID},
			})

			// Make sure the reconciler would still update this changeset.
			plan, err = reconciler.DeterminePlan(
				// changesets[0].PreviousSpecID
				spec1,
				// changesets[0].CurrentSpecID
				spec3,
				changesets[0],
			)
			if err != nil {
				t.Fatal(err)
			}
			if !plan.Ops.Equal(reconciler.Operations{batches.ReconcilerOperationUpdate}) {
				t.Fatalf("Got invalid reconciler operations: %q", plan.Ops.String())
			}

			// Now test that it still updates when this update failed.
			ct.SetChangesetFailed(t, ctx, store, changesets[0])

			batchSpec4 := ct.CreateBatchSpec(t, ctx, store, "queued-changesets", admin.ID)

			// No change this time, just reapplying.
			specOpts.BatchSpec = batchSpec4.ID
			spec4 := ct.CreateChangesetSpec(t, ctx, store, specOpts)

			_, changesets = applyAndListChangesets(adminCtx, t, svc, batchSpec4.RandID, 1)

			ct.ReloadAndAssertChangeset(t, ctx, store, changesets[0], ct.ChangesetAssertions{
				ReconcilerState:  batches.ReconcilerStateQueued,
				PublicationState: batches.ChangesetPublicationStatePublished,
				ExternalBranch:   "refs/heads/queued",
				ExternalID:       "123-queued",
				ExternalState:    batches.ChangesetExternalStateOpen,
				Repo:             repos[3].ID,
				CurrentSpec:      spec4.ID,
				// Still be pointing at the first spec, since the second and third were never applied.
				PreviousSpec:       spec1.ID,
				OwnedByBatchChange: batchChange.ID,
				DiffStat:           ct.TestChangsetSpecDiffStat,
				AttachedTo:         []int64{batchChange.ID},
			})

			// Make sure the reconciler would still update this changeset.
			plan, err = reconciler.DeterminePlan(
				// changesets[0].PreviousSpecID
				spec1,
				// changesets[0].CurrentSpecID
				spec4,
				changesets[0],
			)
			if err != nil {
				t.Fatal(err)
			}
			if !plan.Ops.Equal(reconciler.Operations{batches.ReconcilerOperationUpdate}) {
				t.Fatalf("Got invalid reconciler operations: %q", plan.Ops.String())
			}
		})

		t.Run("missing repository permissions", func(t *testing.T) {
			ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
			ct.MockRepoPermissions(t, db, user.ID, repos[0].ID, repos[2].ID, repos[3].ID)

			// NOTE: We cannot use a context that has authz bypassed.
			batchSpec := ct.CreateBatchSpec(t, userCtx, store, "missing-permissions", user.ID)

			ct.CreateChangesetSpec(t, userCtx, store, ct.TestSpecOpts{
				User:       user.ID,
				Repo:       repos[0].ID,
				BatchSpec:  batchSpec.ID,
				ExternalID: "1234",
			})

			ct.CreateChangesetSpec(t, userCtx, store, ct.TestSpecOpts{
				User:      user.ID,
				Repo:      repos[1].ID, // Not authorized to access this repository
				BatchSpec: batchSpec.ID,
				HeadRef:   "refs/heads/my-branch",
			})

			_, err := svc.ApplyBatchChange(userCtx, ApplyBatchChangeOpts{
				BatchSpecRandID: batchSpec.RandID,
			})
			if err == nil {
				t.Fatal("expected error, but got none")
			}
			notFoundErr, ok := err.(*database.RepoNotFoundErr)
			if !ok {
				t.Fatalf("expected RepoNotFoundErr but got: %s", err)
			}
			if notFoundErr.ID != repos[1].ID {
				t.Fatalf("wrong repository ID in RepoNotFoundErr: %d", notFoundErr.ID)
			}
		})

		t.Run("batch change with errored changeset", func(t *testing.T) {
			ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
			batchSpec1 := ct.CreateBatchSpec(t, ctx, store, "errored-changeset-batch-change", admin.ID)

			spec1Opts := ct.TestSpecOpts{
				User:       admin.ID,
				Repo:       repos[0].ID,
				BatchSpec:  batchSpec1.ID,
				ExternalID: "1234",
				Published:  true,
			}
			ct.CreateChangesetSpec(t, ctx, store, spec1Opts)

			spec2Opts := ct.TestSpecOpts{
				User:      admin.ID,
				Repo:      repos[1].ID,
				BatchSpec: batchSpec1.ID,
				HeadRef:   "refs/heads/repo-1-branch-1",
				Published: true,
			}
			ct.CreateChangesetSpec(t, ctx, store, spec2Opts)

			_, oldChangesets := applyAndListChangesets(adminCtx, t, svc, batchSpec1.RandID, 2)

			// Set the changesets to look like they failed in the reconciler
			for _, c := range oldChangesets {
				ct.SetChangesetFailed(t, ctx, store, c)
			}

			// Now we create another batch spec with the same batch change name
			// and namespace.
			batchSpec2 := ct.CreateBatchSpec(t, ctx, store, "errored-changeset-batch-change", admin.ID)
			spec1Opts.BatchSpec = batchSpec2.ID
			newSpec1 := ct.CreateChangesetSpec(t, ctx, store, spec1Opts)
			spec2Opts.BatchSpec = batchSpec2.ID
			newSpec2 := ct.CreateChangesetSpec(t, ctx, store, spec2Opts)

			batchChange, cs := applyAndListChangesets(adminCtx, t, svc, batchSpec2.RandID, 2)

			c1 := cs.Find(batches.WithExternalID(newSpec1.Spec.ExternalID))
			ct.ReloadAndAssertChangeset(t, ctx, store, c1, ct.ChangesetAssertions{
				Repo:             spec1Opts.Repo,
				ExternalID:       "1234",
				PublicationState: batches.ChangesetPublicationStateUnpublished,
				AttachedTo:       []int64{batchChange.ID},

				ReconcilerState: batches.ReconcilerStateQueued,
				FailureMessage:  nil,
				NumFailures:     0,
			})

			c2 := cs.Find(batches.WithCurrentSpecID(newSpec2.ID))
			ct.AssertChangeset(t, c2, ct.ChangesetAssertions{
				Repo:        newSpec2.RepoID,
				CurrentSpec: newSpec2.ID,
				// An errored changeset doesn't get the specs rotated, to prevent https://github.com/sourcegraph/sourcegraph/issues/16041.
				PreviousSpec:       0,
				OwnedByBatchChange: batchChange.ID,
				PublicationState:   batches.ChangesetPublicationStateUnpublished,
				DiffStat:           ct.TestChangsetSpecDiffStat,
				AttachedTo:         []int64{batchChange.ID},

				ReconcilerState: batches.ReconcilerStateQueued,
				FailureMessage:  nil,
				NumFailures:     0,
			})

			// Make sure the reconciler would still publish this changeset.
			plan, err := reconciler.DeterminePlan(
				// c2.previousSpec is 0
				nil,
				// c2.currentSpec is newSpec2
				newSpec2,
				c2,
			)
			if err != nil {
				t.Fatal(err)
			}
			if !plan.Ops.Equal(reconciler.Operations{batches.ReconcilerOperationPush, batches.ReconcilerOperationPublish}) {
				t.Fatalf("Got invalid reconciler operations: %q", plan.Ops.String())
			}
		})

		t.Run("closed and detached changeset not re-enqueued for close", func(t *testing.T) {
			ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
			batchSpec1 := ct.CreateBatchSpec(t, ctx, store, "detached-closed-changeset", admin.ID)

			specOpts := ct.TestSpecOpts{
				User:      admin.ID,
				Repo:      repos[0].ID,
				BatchSpec: batchSpec1.ID,
				HeadRef:   "refs/heads/detached-closed",
			}
			spec1 := ct.CreateChangesetSpec(t, ctx, store, specOpts)

			// STEP 1: We apply the spec and expect 1 changeset.
			batchChange, changesets := applyAndListChangesets(adminCtx, t, svc, batchSpec1.RandID, 1)

			// Now we update the changeset so it looks like it's been published
			// on the code host.
			c := changesets[0]
			ct.SetChangesetPublished(t, ctx, store, c, "995544", specOpts.HeadRef)

			assertions := ct.ChangesetAssertions{
				Repo:               c.RepoID,
				CurrentSpec:        spec1.ID,
				ExternalID:         c.ExternalID,
				ExternalBranch:     c.ExternalBranch,
				ExternalState:      batches.ChangesetExternalStateOpen,
				OwnedByBatchChange: batchChange.ID,
				ReconcilerState:    batches.ReconcilerStateCompleted,
				PublicationState:   batches.ChangesetPublicationStatePublished,
				DiffStat:           ct.TestChangsetSpecDiffStat,
				AttachedTo:         []int64{batchChange.ID},
			}
			c = ct.ReloadAndAssertChangeset(t, ctx, store, c, assertions)

			// STEP 2: Now we apply a new spec without any changesets, but expect the changeset-to-be-detached to
			// be left in the batch change (the reconciler would detach it, if the executor picked up the changeset).
			batchSpec2 := ct.CreateBatchSpec(t, ctx, store, "detached-closed-changeset", admin.ID)
			applyAndListChangesets(adminCtx, t, svc, batchSpec2.RandID, 1)

			// Our previously published changeset should be marked as "to be closed"
			assertions.Closing = true
			assertions.ReconcilerState = batches.ReconcilerStateQueued
			// And the previous spec is recorded, because the previous run finished with reconcilerState completed.
			assertions.PreviousSpec = spec1.ID
			assertions.DetachFrom = []int64{batchChange.ID}
			assertions.AttachedTo = []int64{}
			c = ct.ReloadAndAssertChangeset(t, ctx, store, c, assertions)

			// Now we update the changeset to make it look closed.
			ct.SetChangesetClosed(t, ctx, store, c)
			assertions.Closing = false
			assertions.DetachFrom = []int64{}
			assertions.ReconcilerState = batches.ReconcilerStateCompleted
			assertions.ExternalState = batches.ChangesetExternalStateClosed
			c = ct.ReloadAndAssertChangeset(t, ctx, store, c, assertions)

			// STEP 3: We apply a new batch spec and expect that the detached changeset record is not re-enqueued.
			batchSpec3 := ct.CreateBatchSpec(t, ctx, store, "detached-closed-changeset", admin.ID)

			applyAndListChangesets(adminCtx, t, svc, batchSpec3.RandID, 0)

			// Assert that the changeset record is still completed and closed.
			ct.ReloadAndAssertChangeset(t, ctx, store, c, assertions)
		})

		t.Run("batch change with changeset that is detached and reattached", func(t *testing.T) {
			t.Run("changeset has been closed before re-attaching", func(t *testing.T) {
				ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
				batchSpec1 := ct.CreateBatchSpec(t, ctx, store, "detach-reattach-changeset", admin.ID)

				specOpts := ct.TestSpecOpts{
					User:      admin.ID,
					Repo:      repos[0].ID,
					BatchSpec: batchSpec1.ID,
					HeadRef:   "refs/heads/detached-reattached",
				}
				spec1 := ct.CreateChangesetSpec(t, ctx, store, specOpts)

				// STEP 1: We apply the spec and expect 1 changeset.
				batchChange, changesets := applyAndListChangesets(adminCtx, t, svc, batchSpec1.RandID, 1)

				// Now we update the changeset so it looks like it's been published
				// on the code host.
				c := changesets[0]
				ct.SetChangesetPublished(t, ctx, store, c, "995533", specOpts.HeadRef)

				assertions := ct.ChangesetAssertions{
					Repo:               c.RepoID,
					CurrentSpec:        spec1.ID,
					ExternalID:         c.ExternalID,
					ExternalBranch:     c.ExternalBranch,
					ExternalState:      batches.ChangesetExternalStateOpen,
					OwnedByBatchChange: batchChange.ID,
					ReconcilerState:    batches.ReconcilerStateCompleted,
					PublicationState:   batches.ChangesetPublicationStatePublished,
					DiffStat:           ct.TestChangsetSpecDiffStat,
					AttachedTo:         []int64{batchChange.ID},
				}
				ct.ReloadAndAssertChangeset(t, ctx, store, c, assertions)

				// STEP 2: Now we apply a new spec without any changesets.
				batchSpec2 := ct.CreateBatchSpec(t, ctx, store, "detach-reattach-changeset", admin.ID)
				applyAndListChangesets(adminCtx, t, svc, batchSpec2.RandID, 1)

				// Our previously published changeset should be marked as "to be closed"
				assertions.Closing = true
				assertions.DetachFrom = []int64{batchChange.ID}
				assertions.AttachedTo = []int64{}
				assertions.ReconcilerState = batches.ReconcilerStateQueued
				// And the previous spec is recorded.
				assertions.PreviousSpec = spec1.ID
				c = ct.ReloadAndAssertChangeset(t, ctx, store, c, assertions)

				// Now we update the changeset to make it look closed.
				ct.SetChangesetClosed(t, ctx, store, c)
				assertions.Closing = false
				assertions.DetachFrom = []int64{}
				assertions.ReconcilerState = batches.ReconcilerStateCompleted
				assertions.ExternalState = batches.ChangesetExternalStateClosed
				ct.ReloadAndAssertChangeset(t, ctx, store, c, assertions)

				// STEP 3: We apply a new batch spec with a changeset spec that
				// matches the old changeset and expect _the same changeset_ to be
				// re-attached.
				batchSpec3 := ct.CreateBatchSpec(t, ctx, store, "detach-reattach-changeset", admin.ID)

				specOpts.BatchSpec = batchSpec3.ID
				spec2 := ct.CreateChangesetSpec(t, ctx, store, specOpts)

				_, changesets = applyAndListChangesets(adminCtx, t, svc, batchSpec3.RandID, 1)

				attachedChangeset := changesets[0]
				if have, want := attachedChangeset.ID, c.ID; have != want {
					t.Fatalf("attached changeset has wrong ID. want=%d, have=%d", want, have)
				}

				// Assert that the changeset has been updated to point to the new spec
				assertions.CurrentSpec = spec2.ID
				// Assert that the previous spec is still spec 1
				assertions.PreviousSpec = spec1.ID
				assertions.ReconcilerState = batches.ReconcilerStateQueued
				assertions.AttachedTo = []int64{batchChange.ID}
				ct.AssertChangeset(t, attachedChangeset, assertions)
			})

			t.Run("changeset has failed closing before re-attaching", func(t *testing.T) {
				ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
				batchSpec1 := ct.CreateBatchSpec(t, ctx, store, "detach-reattach-failed-changeset", admin.ID)

				specOpts := ct.TestSpecOpts{
					User:      admin.ID,
					Repo:      repos[0].ID,
					BatchSpec: batchSpec1.ID,
					HeadRef:   "refs/heads/detached-reattach-failed",
				}
				spec1 := ct.CreateChangesetSpec(t, ctx, store, specOpts)

				// STEP 1: We apply the spec and expect 1 changeset.
				batchChange, changesets := applyAndListChangesets(adminCtx, t, svc, batchSpec1.RandID, 1)

				// Now we update the changeset so it looks like it's been published
				// on the code host.
				c := changesets[0]
				ct.SetChangesetPublished(t, ctx, store, c, "80022", specOpts.HeadRef)

				assertions := ct.ChangesetAssertions{
					Repo:               c.RepoID,
					CurrentSpec:        spec1.ID,
					ExternalID:         c.ExternalID,
					ExternalBranch:     c.ExternalBranch,
					ExternalState:      batches.ChangesetExternalStateOpen,
					OwnedByBatchChange: batchChange.ID,
					ReconcilerState:    batches.ReconcilerStateCompleted,
					PublicationState:   batches.ChangesetPublicationStatePublished,
					DiffStat:           ct.TestChangsetSpecDiffStat,
					AttachedTo:         []int64{batchChange.ID},
				}
				ct.ReloadAndAssertChangeset(t, ctx, store, c, assertions)

				// STEP 2: Now we apply a new spec without any changesets.
				batchSpec2 := ct.CreateBatchSpec(t, ctx, store, "detach-reattach-failed-changeset", admin.ID)
				applyAndListChangesets(adminCtx, t, svc, batchSpec2.RandID, 1)

				// Our previously published changeset should be marked as "to be closed"
				assertions.Closing = true
				assertions.DetachFrom = []int64{batchChange.ID}
				assertions.AttachedTo = []int64{}
				assertions.ReconcilerState = batches.ReconcilerStateQueued
				// And the previous spec is recorded.
				assertions.PreviousSpec = spec1.ID
				c = ct.ReloadAndAssertChangeset(t, ctx, store, c, assertions)

				if len(c.BatchChanges) != 1 {
					t.Fatal("Expected changeset to be still attached to batch change, but wasn't")
				}

				// Now we update the changeset to simulate that closing failed.
				ct.SetChangesetFailed(t, ctx, store, c)
				assertions.Closing = true
				assertions.ReconcilerState = batches.ReconcilerStateFailed
				assertions.ExternalState = batches.ChangesetExternalStateOpen

				// Side-effects of ct.setChangesetFailed.
				assertions.FailureMessage = c.FailureMessage
				assertions.NumFailures = c.NumFailures
				ct.ReloadAndAssertChangeset(t, ctx, store, c, assertions)

				// STEP 3: We apply a new batch spec with a changeset spec that
				// matches the old changeset and expect _the same changeset_ to be
				// re-attached.
				batchSpec3 := ct.CreateBatchSpec(t, ctx, store, "detach-reattach-failed-changeset", admin.ID)

				specOpts.BatchSpec = batchSpec3.ID
				spec2 := ct.CreateChangesetSpec(t, ctx, store, specOpts)

				_, changesets = applyAndListChangesets(adminCtx, t, svc, batchSpec3.RandID, 1)

				attachedChangeset := changesets[0]
				if have, want := attachedChangeset.ID, c.ID; have != want {
					t.Fatalf("attached changeset has wrong ID. want=%d, have=%d", want, have)
				}

				// Assert that the changeset has been updated to point to the new spec
				assertions.CurrentSpec = spec2.ID
				// Assert that the previous spec is still spec 1
				assertions.PreviousSpec = spec1.ID
				assertions.ReconcilerState = batches.ReconcilerStateQueued
				assertions.FailureMessage = nil
				assertions.NumFailures = 0
				assertions.DetachFrom = []int64{}
				assertions.AttachedTo = []int64{batchChange.ID}
				ct.AssertChangeset(t, attachedChangeset, assertions)
			})

			t.Run("changeset has not been closed before re-attaching", func(t *testing.T) {
				ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
				// The difference to the previous test: we DON'T update the
				// changeset to make it look closed. We want to make sure that
				// we also pick up enqueued-to-be-closed changesets.

				batchSpec1 := ct.CreateBatchSpec(t, ctx, store, "detach-reattach-changeset-2", admin.ID)

				specOpts := ct.TestSpecOpts{
					User:      admin.ID,
					Repo:      repos[0].ID,
					BatchSpec: batchSpec1.ID,
					HeadRef:   "refs/heads/detached-reattached-2",
				}
				spec1 := ct.CreateChangesetSpec(t, ctx, store, specOpts)

				// STEP 1: We apply the spec and expect 1 changeset.
				batchChange, changesets := applyAndListChangesets(adminCtx, t, svc, batchSpec1.RandID, 1)

				c := changesets[0]
				ct.SetChangesetPublished(t, ctx, store, c, "449955", specOpts.HeadRef)

				assertions := ct.ChangesetAssertions{
					Repo:               c.RepoID,
					CurrentSpec:        spec1.ID,
					ExternalID:         c.ExternalID,
					ExternalBranch:     c.ExternalBranch,
					ExternalState:      batches.ChangesetExternalStateOpen,
					OwnedByBatchChange: batchChange.ID,
					ReconcilerState:    batches.ReconcilerStateCompleted,
					PublicationState:   batches.ChangesetPublicationStatePublished,
					DiffStat:           ct.TestChangsetSpecDiffStat,
					AttachedTo:         []int64{batchChange.ID},
				}
				ct.ReloadAndAssertChangeset(t, ctx, store, c, assertions)

				// STEP 2: Now we apply a new spec without any changesets.
				batchChange2 := ct.CreateBatchSpec(t, ctx, store, "detach-reattach-changeset-2", admin.ID)
				applyAndListChangesets(adminCtx, t, svc, batchChange2.RandID, 1)

				// Our previously published changeset should be marked as "to be closed"
				assertions.Closing = true
				assertions.DetachFrom = []int64{batchChange.ID}
				assertions.AttachedTo = []int64{}
				assertions.ReconcilerState = batches.ReconcilerStateQueued
				// And the previous spec is recorded.
				assertions.PreviousSpec = spec1.ID
				ct.ReloadAndAssertChangeset(t, ctx, store, c, assertions)

				// STEP 3: We apply a new batch spec with a changeset spec that
				// matches the old changeset and expect _the same changeset_ to be
				// re-attached.
				batchSpec3 := ct.CreateBatchSpec(t, ctx, store, "detach-reattach-changeset-2", admin.ID)

				specOpts.BatchSpec = batchSpec3.ID
				spec2 := ct.CreateChangesetSpec(t, ctx, store, specOpts)

				_, changesets = applyAndListChangesets(adminCtx, t, svc, batchSpec3.RandID, 1)

				attachedChangeset := changesets[0]
				if have, want := attachedChangeset.ID, c.ID; have != want {
					t.Fatalf("attached changeset has wrong ID. want=%d, have=%d", want, have)
				}

				// Assert that the changeset has been updated to point to the new spec
				assertions.CurrentSpec = spec2.ID
				// Assert that the previous spec is still spec 1
				assertions.PreviousSpec = spec1.ID
				assertions.ReconcilerState = batches.ReconcilerStateQueued
				assertions.DetachFrom = []int64{}
				assertions.AttachedTo = []int64{batchChange.ID}
				ct.AssertChangeset(t, attachedChangeset, assertions)
			})
		})
	})

	t.Run("applying to closed batch change", func(t *testing.T) {
		ct.TruncateTables(t, db, "changeset_events", "changesets", "batch_changes", "batch_specs", "changeset_specs")
		batchSpec := ct.CreateBatchSpec(t, ctx, store, "closed-batch-change", admin.ID)
		batchChange := ct.CreateBatchChange(t, ctx, store, "closed-batch-change", admin.ID, batchSpec.ID)

		batchChange.ClosedAt = time.Now()
		if err := store.UpdateBatchChange(ctx, batchChange); err != nil {
			t.Fatalf("failed to update batch change: %s", err)
		}

		_, err := svc.ApplyBatchChange(adminCtx, ApplyBatchChangeOpts{
			BatchSpecRandID: batchSpec.RandID,
		})
		if err != ErrApplyClosedBatchChange {
			t.Fatalf("ApplyBatchChange returned unexpected error: %s", err)
		}
	})
}

func applyAndListChangesets(ctx context.Context, t *testing.T, svc *Service, batchSpecRandID string, wantChangesets int) (*batches.BatchChange, batches.Changesets) {
	t.Helper()

	batchChange, err := svc.ApplyBatchChange(ctx, ApplyBatchChangeOpts{
		BatchSpecRandID: batchSpecRandID,
	})
	if err != nil {
		t.Fatalf("failed to apply batch change: %s", err)
	}

	if batchChange.ID == 0 {
		t.Fatalf("batch change ID is zero")
	}

	changesets, _, err := svc.store.ListChangesets(ctx, store.ListChangesetsOpts{BatchChangeID: batchChange.ID})
	if err != nil {
		t.Fatal(err)
	}

	if have, want := len(changesets), wantChangesets; have != want {
		t.Fatalf("wrong number of changesets. want=%d, have=%d", want, have)
	}

	return batchChange, changesets
}
