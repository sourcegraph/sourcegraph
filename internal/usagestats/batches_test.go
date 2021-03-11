package usagestats

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestGetBatchChangesUsageStatistics(t *testing.T) {
	ctx := context.Background()
	db := dbtesting.GetDB(t)

	// Create stub repo.
	repoStore := database.Repos(db)
	esStore := database.ExternalServices(db)

	now := time.Now()
	svc := types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test",
		Config:      `{"url": "https://github.com"}`,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := esStore.Upsert(ctx, &svc); err != nil {
		t.Fatalf("failed to insert external services: %v", err)
	}
	repo := &types.Repo{
		Name: "test/repo",
		ExternalRepo: api.ExternalRepoSpec{
			ID:          fmt.Sprintf("external-id-%d", svc.ID),
			ServiceType: extsvc.TypeGitHub,
			ServiceID:   "https://github.com/",
		},
		Sources: map[string]*types.SourceInfo{
			svc.URN(): {
				ID:       svc.URN(),
				CloneURL: "https://secrettoken@test/repo",
			},
		},
	}
	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	// Create a user.
	user, err := database.Users(db).Create(ctx, database.NewUser{Username: "test"})
	if err != nil {
		t.Fatal(err)
	}

	// Create batch specs 1, 2.
	_, err = db.Exec(`
		INSERT INTO batch_specs
			(id, rand_id, raw_spec, namespace_user_id)
		VALUES
			(1, '123', '{}', $1),
			(2, '456', '{}', $1)
	`, user.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Create event logs
	_, err = db.Exec(`
		INSERT INTO event_logs
			(id, name, argument, url, user_id, anonymous_user_id, source, version, timestamp)
		VALUES
			(1, 'CampaignSpecCreated', '{"changeset_specs_count": 3}', '', 23, '', 'backend', 'version', now()),
			(2, 'CampaignSpecCreated', '{"changeset_specs_count": 1}', '', 23, '', 'backend', 'version', now()),
			(3, 'CampaignSpecCreated', '{}', '', 23, '', 'backend', 'version', now()),
			(4, 'ViewCampaignApplyPage', '{}', 'https://sourcegraph.test:3443/users/mrnugget/batch-changes/apply/RANDID', 23, '5d302f47-9e91-4b3d-9e96-469b5601a765', 'WEB', 'version', now()),
			(5, 'ViewCampaignDetailsPageAfterCreate', '{}', 'https://sourcegraph.test:3443/users/mrnugget/batch-changes/gitignore-files', 23, '5d302f47-9e91-4b3d-9e96-469b5601a765', 'WEB', 'version', now()),
			(6, 'ViewCampaignDetailsPageAfterUpdate', '{}', 'https://sourcegraph.test:3443/users/mrnugget/batch-changes/gitignore-files', 23, '5d302f47-9e91-4b3d-9e96-469b5601a765', 'WEB', 'version', now())
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Create batch changes 1, 2.
	_, err = db.Exec(`
		INSERT INTO batch_changes
			(id, name, batch_spec_id, last_applied_at, namespace_user_id, closed_at)
		VALUES
			(1, 'test', 1, NOW(), $1, NULL),
			(2, 'test-2', 2, NOW(), $1, NOW())
	`, user.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Create 6 changesets.
	// 2 tracked: one OPEN, one MERGED.
	// 4 created by a batch change: 2 open (one with diffstat, one without), 2 merged (one with diffstat, one without)
	// missing diffstat shouldn't happen anymore (due to migration), but it's still a nullable field.
	_, err = db.Exec(`
		INSERT INTO changesets
			(id, repo_id, external_service_type, owned_by_batch_change_id, external_state, publication_state, diff_stat_added, diff_stat_changed, diff_stat_deleted)
		VALUES
		    -- tracked
			(1, $1, 'github', NULL, 'OPEN',   'PUBLISHED', 9, 7, 5),
			(2, $1, 'github', NULL, 'MERGED', 'PUBLISHED', 7, 9, 5),
			-- created by batch change
			(4, $1, 'github', 1, 'OPEN',   'PUBLISHED', 5, 7, 9),
			(5, $1, 'github', 1, 'OPEN',   'PUBLISHED', NULL, NULL, NULL),
			(6, $1, 'github', 2, 'MERGED', 'PUBLISHED', 9, 7, 5),
			(7, $1, 'github', 2, 'MERGED', 'PUBLISHED', NULL, NULL, NULL)
	`, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	have, err := GetBatchChangesUsageStatistics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := &types.CampaignsUsageStatistics{
		ViewCampaignApplyPageCount:               1,
		ViewCampaignDetailsPageAfterCreateCount:  1,
		ViewCampaignDetailsPageAfterUpdateCount:  1,
		CampaignsCount:                           2,
		CampaignsClosedCount:                     1,
		ActionChangesetsCount:                    4,
		ActionChangesetsDiffStatAddedSum:         14,
		ActionChangesetsDiffStatChangedSum:       14,
		ActionChangesetsDiffStatDeletedSum:       14,
		ActionChangesetsMergedCount:              2,
		ActionChangesetsMergedDiffStatAddedSum:   9,
		ActionChangesetsMergedDiffStatChangedSum: 7,
		ActionChangesetsMergedDiffStatDeletedSum: 5,
		ManualChangesetsCount:                    2,
		ManualChangesetsMergedCount:              1,
		CampaignSpecsCreatedCount:                3,
		ChangesetSpecsCreatedCount:               4,
	}
	if diff := cmp.Diff(want, have); diff != "" {
		t.Fatal(diff)
	}
}
