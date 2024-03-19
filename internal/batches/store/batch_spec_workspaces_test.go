package store

import (
	"context"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/batches/search"
	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types/typestest"
	"github.com/sourcegraph/sourcegraph/lib/batches/execution"
)

func testStoreBatchSpecWorkspaces(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	logger := logtest.Scoped(t)
	repoStore := database.ReposWith(logger, s)

	user := bt.CreateTestUser(t, s.DatabaseDB(), false)
	repos, _ := bt.CreateTestRepos(t, ctx, s.DatabaseDB(), 4)
	deletedRepo := repos[3].With(typestest.Opt.RepoDeletedAt(clock.Now()))
	if err := repoStore.Delete(ctx, deletedRepo.ID); err != nil {
		t.Fatal(err)
	}
	// Allow all repos but repos[2]
	bt.MockRepoPermissions(t, s.DatabaseDB(), user.ID, repos[0].ID, repos[1].ID, repos[3].ID)

	workspaces := make([]*btypes.BatchSpecWorkspace, 0, 4)
	for i := range cap(workspaces) {
		job := &btypes.BatchSpecWorkspace{
			BatchSpecID:      int64(i + 567),
			ChangesetSpecIDs: []int64{int64(i + 456), int64(i + 678)},

			RepoID: repos[i].ID,
			Branch: "master",
			Commit: "d34db33f",
			Path:   "sub/dir/ectory",
			FileMatches: []string{
				"a.go",
				"a/b/horse.go",
				"a/b/c.go",
			},
			OnlyFetchWorkspace: true,
			Unsupported:        true,
			Ignored:            true,
			Skipped:            i == 1,
			CachedResultFound:  i == 1,
		}

		workspaces = append(workspaces, job)
	}

	t.Run("Create", func(t *testing.T) {
		for _, job := range workspaces {
			if err := s.CreateBatchSpecWorkspace(ctx, job); err != nil {
				t.Fatal(err)
			}

			have := job
			if have.ID == 0 {
				t.Fatal("ID should not be zero")
			}

			want := have
			want.CreatedAt = clock.Now()
			want.UpdatedAt = clock.Now()

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		}
	})

	if err := s.Exec(ctx, sqlf.Sprintf("INSERT INTO batch_spec_workspace_execution_jobs (batch_spec_workspace_id, user_id, state, cancel) VALUES (%s, %s, %s, %s)", workspaces[0].ID, user.ID, btypes.BatchSpecWorkspaceExecutionJobStateCompleted, true)); err != nil {
		t.Fatal(err)
	}

	t.Run("Get", func(t *testing.T) {
		t.Run("GetByID", func(t *testing.T) {
			for i, job := range workspaces {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					have, err := s.GetBatchSpecWorkspace(ctx, GetBatchSpecWorkspaceOpts{ID: job.ID})

					if job.RepoID == deletedRepo.ID {
						if err != ErrNoResults {
							t.Fatalf("wrong error: %s", err)
						}
						return
					}

					if err != nil {
						t.Fatal(err)
					}

					if diff := cmp.Diff(have, job); diff != "" {
						t.Fatal(diff)
					}
				})
			}
		})

		t.Run("NoResults", func(t *testing.T) {
			opts := GetBatchSpecWorkspaceOpts{ID: 0xdeadbeef}

			_, have := s.GetBatchSpecWorkspace(ctx, opts)
			want := ErrNoResults

			if have != want {
				t.Fatalf("have err %v, want %v", have, want)
			}
		})
	})

	t.Run("List", func(t *testing.T) {
		t.Run("All", func(t *testing.T) {
			have, _, err := s.ListBatchSpecWorkspaces(ctx, ListBatchSpecWorkspacesOpts{})
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(have, workspaces[:len(workspaces)-1]); diff != "" {
				t.Fatalf("invalid jobs returned: %s", diff)
			}
		})

		t.Run("ByBatchSpecID", func(t *testing.T) {
			for _, ws := range workspaces {
				have, _, err := s.ListBatchSpecWorkspaces(ctx, ListBatchSpecWorkspacesOpts{
					BatchSpecID: ws.BatchSpecID,
				})

				if err != nil {
					t.Fatal(err)
				}

				if ws.RepoID == deletedRepo.ID {
					if len(have) != 0 {
						t.Fatalf("expected zero results, but got: %d", len(have))
					}
					return
				}
				if len(have) != 1 {
					t.Fatalf("wrong number of results. have=%d", len(have))
				}

				if diff := cmp.Diff(have, []*btypes.BatchSpecWorkspace{ws}); diff != "" {
					t.Fatalf("invalid jobs returned: %s", diff)
				}
			}
		})

		t.Run("ByID", func(t *testing.T) {
			for _, ws := range workspaces {
				have, _, err := s.ListBatchSpecWorkspaces(ctx, ListBatchSpecWorkspacesOpts{
					IDs: []int64{ws.ID},
				})

				if err != nil {
					t.Fatal(err)
				}

				if ws.RepoID == deletedRepo.ID {
					if len(have) != 0 {
						t.Fatalf("expected zero results, but got: %d", len(have))
					}
					return
				}
				if len(have) != 1 {
					t.Fatalf("wrong number of results. have=%d", len(have))
				}

				if diff := cmp.Diff(have, []*btypes.BatchSpecWorkspace{ws}); diff != "" {
					t.Fatalf("invalid jobs returned: %s", diff)
				}
			}
		})

		t.Run("ByState", func(t *testing.T) {
			// Grab the completed one:
			have, _, err := s.ListBatchSpecWorkspaces(ctx, ListBatchSpecWorkspacesOpts{
				State: btypes.BatchSpecWorkspaceExecutionJobStateCompleted,
			})
			if err != nil {
				t.Fatal(err)
			}

			if len(have) != 1 {
				t.Fatalf("wrong number of results. have=%d", len(have))
			}

			if diff := cmp.Diff(have, workspaces[:1]); diff != "" {
				t.Fatalf("invalid jobs returned: %s", diff)
			}
		})

		t.Run("OnlyWithoutExecution", func(t *testing.T) {
			have, _, err := s.ListBatchSpecWorkspaces(ctx, ListBatchSpecWorkspacesOpts{
				OnlyWithoutExecutionAndNotCached: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			if len(have) != 1 {
				t.Fatalf("wrong number of results. have=%d", len(have))
			}

			if diff := cmp.Diff(have, workspaces[2:3]); diff != "" {
				t.Fatalf("invalid jobs returned: %s", diff)
			}
		})

		t.Run("OnlyCachedOrCompleted", func(t *testing.T) {
			have, _, err := s.ListBatchSpecWorkspaces(ctx, ListBatchSpecWorkspacesOpts{
				OnlyCachedOrCompleted: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			if len(have) != 2 {
				t.Fatalf("wrong number of results. have=%d", len(have))
			}

			if diff := cmp.Diff(have, workspaces[:2]); diff != "" {
				t.Fatalf("invalid jobs returned: %s", diff)
			}
		})

		t.Run("Cancel", func(t *testing.T) {
			tr := true
			have, _, err := s.ListBatchSpecWorkspaces(ctx, ListBatchSpecWorkspacesOpts{
				Cancel: &tr,
			})
			if err != nil {
				t.Fatal(err)
			}

			if len(have) != 1 {
				t.Fatalf("wrong number of results. have=%d", len(have))
			}

			if diff := cmp.Diff(have, workspaces[:1]); diff != "" {
				t.Fatalf("invalid jobs returned: %s", diff)
			}
		})

		t.Run("Skipped", func(t *testing.T) {
			tr := true
			have, _, err := s.ListBatchSpecWorkspaces(ctx, ListBatchSpecWorkspacesOpts{
				Skipped: &tr,
			})
			if err != nil {
				t.Fatal(err)
			}

			if len(have) != 1 {
				t.Fatalf("wrong number of results. have=%d", len(have))
			}

			if diff := cmp.Diff(have, workspaces[1:2]); diff != "" {
				t.Fatalf("invalid jobs returned: %s", diff)
			}
		})

		t.Run("TextSearch", func(t *testing.T) {
			for i, r := range repos[:3] {
				userCtx := actor.WithActor(ctx, actor.FromUser(user.ID))
				have, _, err := s.ListBatchSpecWorkspaces(userCtx, ListBatchSpecWorkspacesOpts{
					TextSearch: []search.TextSearchTerm{{Term: string(r.Name)}},
				})
				if err != nil {
					t.Fatal(err)
				}

				// Expect to return no results for repo[2], which the user cannot access.
				if i == 2 {
					if len(have) != 0 {
						t.Fatalf("wrong number of results. have=%d", len(have))
					}
					break

				} else if len(have) != 1 {
					t.Fatalf("wrong number of results. have=%d", len(have))
				}

				if diff := cmp.Diff(have, []*btypes.BatchSpecWorkspace{workspaces[i]}); diff != "" {
					t.Fatalf("invalid jobs returned: %s", diff)
				}
			}
		})
	})

	t.Run("Count", func(t *testing.T) {
		t.Run("All", func(t *testing.T) {
			have, err := s.CountBatchSpecWorkspaces(ctx, ListBatchSpecWorkspacesOpts{})
			if err != nil {
				t.Fatal(err)
			}
			if want := int64(3); have != want {
				t.Fatalf("invalid count returned: want=%d have=%d", want, have)
			}
		})

		t.Run("ByBatchSpecID", func(t *testing.T) {
			for _, ws := range workspaces {
				have, err := s.CountBatchSpecWorkspaces(ctx, ListBatchSpecWorkspacesOpts{
					BatchSpecID: ws.BatchSpecID,
				})

				if err != nil {
					t.Fatal(err)
				}

				if ws.RepoID == deletedRepo.ID {
					if have != 0 {
						t.Fatalf("expected zero results, but got: %d", have)
					}
					return
				}
				if have != 1 {
					t.Fatalf("wrong number of results. have=%d", have)
				}
			}
		})

		t.Run("ByID", func(t *testing.T) {
			for _, ws := range workspaces {
				have, err := s.CountBatchSpecWorkspaces(ctx, ListBatchSpecWorkspacesOpts{
					IDs: []int64{ws.ID},
				})

				if err != nil {
					t.Fatal(err)
				}

				if ws.RepoID == deletedRepo.ID {
					if have != 0 {
						t.Fatalf("expected zero results, but got: %d", have)
					}
					return
				}
				if have != 1 {
					t.Fatalf("wrong number of results. have=%d", have)
				}
			}
		})
	})

	t.Run("MarkSkippedBatchSpecWorkspaces", func(t *testing.T) {
		tests := []struct {
			batchSpec   *btypes.BatchSpec
			workspace   *btypes.BatchSpecWorkspace
			wantSkipped bool
		}{
			{
				batchSpec:   &btypes.BatchSpec{AllowIgnored: false, AllowUnsupported: false},
				workspace:   &btypes.BatchSpecWorkspace{Ignored: true},
				wantSkipped: true,
			},
			{
				batchSpec:   &btypes.BatchSpec{AllowIgnored: true, AllowUnsupported: false},
				workspace:   &btypes.BatchSpecWorkspace{Ignored: true},
				wantSkipped: false,
			},
			{
				batchSpec:   &btypes.BatchSpec{AllowIgnored: false, AllowUnsupported: false},
				workspace:   &btypes.BatchSpecWorkspace{Unsupported: true},
				wantSkipped: true,
			},
			{
				batchSpec:   &btypes.BatchSpec{AllowIgnored: false, AllowUnsupported: true},
				workspace:   &btypes.BatchSpecWorkspace{Unsupported: true},
				wantSkipped: false,
			},
			// TODO: Add test that workspace with no steps to be executed is skipped properly.
		}

		for _, tt := range tests {
			tt.batchSpec.NamespaceUserID = 1
			tt.batchSpec.UserID = 1
			err := s.CreateBatchSpec(ctx, tt.batchSpec)
			if err != nil {
				t.Fatal(err)
			}

			tt.workspace.BatchSpecID = tt.batchSpec.ID
			tt.workspace.RepoID = repos[0].ID
			tt.workspace.Branch = "master"
			tt.workspace.Commit = "d34db33f"
			tt.workspace.Path = "sub/dir/ectory"
			tt.workspace.FileMatches = []string{}

			if err := s.CreateBatchSpecWorkspace(ctx, tt.workspace); err != nil {
				t.Fatal(err)
			}

			if err := s.MarkSkippedBatchSpecWorkspaces(ctx, tt.batchSpec.ID); err != nil {
				t.Fatal(err)
			}

			reloaded, err := s.GetBatchSpecWorkspace(ctx, GetBatchSpecWorkspaceOpts{ID: tt.workspace.ID})
			if err != nil {
				t.Fatal(err)
			}

			if want, have := tt.wantSkipped, reloaded.Skipped; have != want {
				t.Fatalf("workspace.Skipped is wrong. want=%t, have=%t", want, have)
			}
		}
	})

	t.Run("ListRetryBatchSpecWorkspaces", func(t *testing.T) {
		successfulWorkspace := &btypes.BatchSpecWorkspace{
			BatchSpecID: 9999,
			RepoID:      repos[0].ID,
		}
		failedWorkspace := &btypes.BatchSpecWorkspace{
			BatchSpecID: 9999,
			RepoID:      repos[0].ID,
		}

		err := s.CreateBatchSpecWorkspace(ctx, successfulWorkspace)
		require.NoError(t, err)
		err = s.CreateBatchSpecWorkspace(ctx, failedWorkspace)
		require.NoError(t, err)

		err = s.Exec(ctx, sqlf.Sprintf("INSERT INTO batch_spec_workspace_execution_jobs (batch_spec_workspace_id, user_id, state, cancel) VALUES (%s, %s, %s, %s)", successfulWorkspace.ID, user.ID, btypes.BatchSpecWorkspaceExecutionJobStateCompleted, true))
		require.NoError(t, err)
		err = s.Exec(ctx, sqlf.Sprintf("INSERT INTO batch_spec_workspace_execution_jobs (batch_spec_workspace_id, user_id, state, cancel) VALUES (%s, %s, %s, %s)", failedWorkspace.ID, user.ID, btypes.BatchSpecResolutionJobStateFailed, true))
		require.NoError(t, err)

		t.Run("All", func(t *testing.T) {
			have, err := s.ListRetryBatchSpecWorkspaces(ctx, ListRetryBatchSpecWorkspacesOpts{
				BatchSpecID:      9999,
				IncludeCompleted: true,
			})
			require.NoError(t, err)
			assert.Len(t, have, 2)
		})

		t.Run("Uncompleted", func(t *testing.T) {
			have, err := s.ListRetryBatchSpecWorkspaces(ctx, ListRetryBatchSpecWorkspacesOpts{
				BatchSpecID: 9999,
			})
			require.NoError(t, err)
			assert.Len(t, have, 1)
		})
	})

	t.Run("DisableBatchSpecWorkspaceExecutionCache", func(t *testing.T) {
		cs := &btypes.ChangesetSpec{}
		require.NoError(t, s.CreateChangesetSpec(ctx, cs))

		bc := &btypes.BatchSpec{NoCache: true, NamespaceUserID: 1}
		require.NoError(t, s.CreateBatchSpec(ctx, bc))
		batchSpecID := bc.ID

		workspace := &btypes.BatchSpecWorkspace{
			BatchSpecID:       batchSpecID,
			RepoID:            repos[0].ID,
			CachedResultFound: true,
			StepCacheResults: map[int]btypes.StepCacheResult{
				1: {
					Key: "asdf",
					Value: &execution.AfterStepResult{
						StepIndex: 1,
					},
				},
			},
			ChangesetSpecIDs: []int64{cs.ID, 2, 3},
		}
		err := s.CreateBatchSpecWorkspace(ctx, workspace)
		require.NoError(t, err)

		require.NoError(t, s.DisableBatchSpecWorkspaceExecutionCache(ctx, batchSpecID))

		want := workspace
		want.ChangesetSpecIDs = []int64{}
		want.StepCacheResults = map[int]btypes.StepCacheResult{}
		want.CachedResultFound = false

		have, err := s.GetBatchSpecWorkspace(ctx, GetBatchSpecWorkspaceOpts{
			ID: workspace.ID,
		})
		require.NoError(t, err)

		if diff := cmp.Diff(want, have); diff != "" {
			t.Fatalf("invalid workspace state: %s", diff)
		}

		_, err = s.GetChangesetSpec(ctx, GetChangesetSpecOpts{
			ID: cs.ID,
		})
		require.Error(t, err, ErrNoResults)
	})
}
