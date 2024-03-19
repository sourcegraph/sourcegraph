package store

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"

	policiesshared "github.com/sourcegraph/sourcegraph/internal/codeintel/policies/shared"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestRepoIDsByGlobPatterns(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertRepo(t, db, 50, "Darth Vader", true)
	insertRepo(t, db, 51, "Darth Venamis", true)
	insertRepo(t, db, 52, "Darth Maul", true)
	insertRepo(t, db, 53, "Anakin Skywalker", true)
	insertRepo(t, db, 54, "Luke Skywalker", true)
	insertRepo(t, db, 55, "7th Sky Corps", true)

	testCases := []struct {
		patterns              []string
		expectedRepositoryIDs []int
	}{
		{patterns: []string{""}, expectedRepositoryIDs: nil},                                             // No patterns
		{patterns: []string{"*"}, expectedRepositoryIDs: []int{50, 51, 52, 53, 54, 55}},                  // Wildcard
		{patterns: []string{"Darth*"}, expectedRepositoryIDs: []int{50, 51, 52}},                         // Prefix
		{patterns: []string{"Darth V*"}, expectedRepositoryIDs: []int{50, 51}},                           // Prefix
		{patterns: []string{"* Skywalker"}, expectedRepositoryIDs: []int{53, 54}},                        // Suffix
		{patterns: []string{"*er"}, expectedRepositoryIDs: []int{50, 53, 54}},                            // Suffix
		{patterns: []string{"*Sky*"}, expectedRepositoryIDs: []int{53, 54, 55}},                          // Infix
		{patterns: []string{"Darth *", "* Skywalker"}, expectedRepositoryIDs: []int{50, 51, 52, 53, 54}}, // Multiple patterns
		{patterns: []string{"Rey Skywalker"}, expectedRepositoryIDs: nil},                                // No match, never happened
	}

	for _, testCase := range testCases {
		for lo := range len(testCase.expectedRepositoryIDs) {
			hi := lo + 3
			if hi > len(testCase.expectedRepositoryIDs) {
				hi = len(testCase.expectedRepositoryIDs)
			}

			name := fmt.Sprintf(
				"patterns=%v offset=%d",
				testCase.patterns,
				lo,
			)

			t.Run(name, func(t *testing.T) {
				repositoryIDs, _, err := store.GetRepoIDsByGlobPatterns(ctx, testCase.patterns, 3, lo)
				if err != nil {
					t.Fatalf("unexpected error fetching repository ids by glob pattern: %s", err)
				}

				if diff := cmp.Diff(testCase.expectedRepositoryIDs[lo:hi], repositoryIDs); diff != "" {
					t.Errorf("unexpected repository ids (-want +got):\n%s", diff)
				}
			})
		}
	}

	t.Run("enforce repository permissions", func(t *testing.T) {
		// Turning on explicit permissions forces checking repository permissions
		// against permissions tables in the database, which should effectively block
		// all access because permissions tables are empty and repos are private.
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				PermissionsUserMapping: &schema.PermissionsUserMapping{
					Enabled: true,
					BindID:  "email",
				},
			},
		})
		t.Cleanup(func() { conf.Mock(nil) })

		repoIDs, _, err := store.GetRepoIDsByGlobPatterns(ctx, []string{"*"}, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(repoIDs) > 0 {
			t.Fatalf("Want no repositories but got %d repositories", len(repoIDs))
		}
	})
}

func TestUpdateReposMatchingPatterns(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertRepo(t, db, 50, "r1", false)
	insertRepo(t, db, 51, "r2", false)
	insertRepo(t, db, 52, "r3", false)
	insertRepo(t, db, 53, "r4", false)
	insertRepo(t, db, 54, "r5", false)

	updates := []struct {
		policyID int
		pattern  []string
	}{
		// multiple matches
		{100, []string{"r*"}},

		// exact identifiers
		{101, []string{"r1"}},

		// multiple exact identifiers
		{102, []string{"r2", "r3"}},

		// updated patterns (disjoint)
		{103, []string{"r4"}},
		{103, []string{"r5"}},

		// updated patterns (intersecting)
		{104, []string{"r1", "r2", "r3"}},
		{104, []string{"r2", "r3", "r4"}},

		// deleted matches
		{105, []string{"r5"}},
		{105, []string{}},
	}
	for _, update := range updates {
		if err := store.UpdateReposMatchingPatterns(ctx, update.pattern, update.policyID, nil); err != nil {
			t.Fatalf("unexpected error updating repositories matching patterns: %s", err)
		}
	}

	policies, err := scanPolicyRepositories(db.QueryContext(context.Background(), `
		SELECT policy_id, repo_id
		FROM lsif_configuration_policies_repository_pattern_lookup
	`))
	if err != nil {
		t.Fatalf("unexpected error while scanning policies: %s", err)
	}

	for _, repositoryIDs := range policies {
		sort.Ints(repositoryIDs)
	}

	expectedPolicies := map[int][]int{
		100: {50, 51, 52, 53, 54}, // multiple matches
		101: {50},                 // exact identifiers
		102: {51, 52},             // multiple exact identifiers
		103: {54},                 // updated patterns (disjoint)
		104: {51, 52, 53},         // updated patterns (intersecting)
	}
	if diff := cmp.Diff(expectedPolicies, policies); diff != "" {
		t.Errorf("unexpected repository identifiers for policies (-want +got):\n%s", diff)
	}
}

func TestUpdateReposMatchingPatternsOverLimit(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	limit := 50
	ids := make([]int, 0, limit*3)
	for i := range cap(ids) {
		ids = append(ids, 50+i)
	}

	for _, id := range ids {
		insertRepo(t, db, id, fmt.Sprintf("r%03d", id), false)
	}

	if err := store.UpdateReposMatchingPatterns(ctx, []string{"r*"}, 100, &limit); err != nil {
		t.Fatalf("unexpected error updating repositories matching patterns: %s", err)
	}

	policies, err := scanPolicyRepositories(db.QueryContext(context.Background(), `
		SELECT policy_id, repo_id
		FROM lsif_configuration_policies_repository_pattern_lookup
	`))
	if err != nil {
		t.Fatalf("unexpected error while scanning policies: %s", err)
	}

	for _, repositoryIDs := range policies {
		sort.Ints(repositoryIDs)
	}

	expectedPolicies := map[int][]int{
		100: ids[:limit],
	}
	if diff := cmp.Diff(expectedPolicies, policies); diff != "" {
		t.Errorf("unexpected repository identifiers for policies (-want +got):\n%s", diff)
	}
}

func TestSelectPoliciesForRepositoryMembershipUpdate(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := testStoreWithoutConfigurationPolicies(t, db)
	ctx := context.Background()

	query := `
		INSERT INTO lsif_configuration_policies (
			id,
			repository_id,
			name,
			type,
			pattern,
			repository_patterns,
			retention_enabled,
			retention_duration_hours,
			retain_intermediate_commits,
			indexing_enabled,
			index_commit_max_age_hours,
			index_intermediate_commits
		) VALUES
			(101, NULL, 'policy 1', 'GIT_TREE', 'ab/', null, true,  1, true,  true,  1, true),
			(102, NULL, 'policy 2', 'GIT_TREE', 'cd/', null, false, 2, true,  true,  2, true),
			(103, NULL, 'policy 3', 'GIT_TREE', 'ef/', null, true,  3, false, false, 3, false),
			(104, NULL, 'policy 4', 'GIT_TREE', 'gh/', null, false, 4, false, false, 4, false)
	`
	if _, err := db.ExecContext(ctx, query); err != nil {
		t.Fatalf("unexpected error while inserting configuration policies: %s", err)
	}

	ids := func(policies []policiesshared.ConfigurationPolicy) (ids []int) {
		for _, policy := range policies {
			ids = append(ids, policy.ID)
		}

		sort.Slice(ids, func(i, j int) bool {
			return ids[i] < ids[j]
		})

		return ids
	}

	// Can return nulls
	if policies, err := store.SelectPoliciesForRepositoryMembershipUpdate(context.Background(), 2); err != nil {
		t.Fatalf("unexpected error fetching configuration policies for repository membership update: %s", err)
	} else if diff := cmp.Diff([]int{101, 102}, ids(policies)); diff != "" {
		t.Fatalf("unexpected configuration policy list (-want +got):\n%s", diff)
	}

	// Returns new batch
	if policies, err := store.SelectPoliciesForRepositoryMembershipUpdate(context.Background(), 2); err != nil {
		t.Fatalf("unexpected error fetching configuration policies for repository membership update: %s", err)
	} else if diff := cmp.Diff([]int{103, 104}, ids(policies)); diff != "" {
		t.Fatalf("unexpected configuration policy list (-want +got):\n%s", diff)
	}

	// Recycles policies by age
	if policies, err := store.SelectPoliciesForRepositoryMembershipUpdate(context.Background(), 3); err != nil {
		t.Fatalf("unexpected error fetching configuration policies for repository membership update: %s", err)
	} else if diff := cmp.Diff([]int{101, 102, 103}, ids(policies)); diff != "" {
		t.Fatalf("unexpected configuration policy list (-want +got):\n%s", diff)
	}

	// Recycles policies by age
	if policies, err := store.SelectPoliciesForRepositoryMembershipUpdate(context.Background(), 3); err != nil {
		t.Fatalf("unexpected error fetching configuration policies for repository membership update: %s", err)
	} else if diff := cmp.Diff([]int{101, 102, 104}, ids(policies)); diff != "" {
		t.Fatalf("unexpected configuration policy list (-want +got):\n%s", diff)
	}
}
