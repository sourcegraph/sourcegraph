package store

import (
	"context"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/sentinel/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

var badConfig = shared.AffectedPackage{
	Language:          "go",
	PackageName:       "go-nacelle/config",
	VersionConstraint: []string{"<= v1.2.5"},
}

var testVulnerabilities = []shared.Vulnerability{
	// IDs assumed by insertion order
	{ID: 1, SourceID: "CVE-ABC", AffectedPackages: []shared.AffectedPackage{badConfig}},
	{ID: 2, SourceID: "CVE-DEF"},
	{ID: 3, SourceID: "CVE-GHI"},
	{ID: 4, SourceID: "CVE-JKL"},
	{ID: 5, SourceID: "CVE-MNO"},
	{ID: 6, SourceID: "CVE-PQR"},
	{ID: 7, SourceID: "CVE-STU"},
	{ID: 8, SourceID: "CVE-VWX"},
	{ID: 9, SourceID: "CVE-Y&Z"},
}

func TestVulnerabilityByID(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	if _, err := store.InsertVulnerabilities(ctx, testVulnerabilities); err != nil {
		t.Fatalf("unexpected error inserting vulnerabilities: %s", err)
	}

	vulnerability, ok, err := store.VulnerabilityByID(ctx, 2)
	if err != nil {
		t.Fatalf("failed to get vulnerability by id: %s", err)
	}
	if !ok {
		t.Fatalf("unexpected vulnerability to exist")
	}
	if diff := cmp.Diff(canonicalizeVulnerability(testVulnerabilities[1]), vulnerability); diff != "" {
		t.Errorf("unexpected vulnerability (-want +got):\n%s", diff)
	}
}

func TestGetVulnerabilitiesByIDs(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	if _, err := store.InsertVulnerabilities(ctx, testVulnerabilities); err != nil {
		t.Fatalf("unexpected error inserting vulnerabilities: %s", err)
	}

	vulnerabilities, err := store.GetVulnerabilitiesByIDs(ctx, 2, 3, 4)
	if err != nil {
		t.Fatalf("failed to get vulnerability by id: %s", err)
	}
	if diff := cmp.Diff(canonicalizeVulnerabilities(testVulnerabilities[1:4]), vulnerabilities); diff != "" {
		t.Errorf("unexpected vulnerabilities (-want +got):\n%s", diff)
	}
}

func TestGetVulnerabilities(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	if _, err := store.InsertVulnerabilities(ctx, testVulnerabilities); err != nil {
		t.Fatalf("unexpected error inserting vulnerabilities: %s", err)
	}

	type testCase struct {
		name              string
		expectedSourceIDs []string
	}
	testCases := []testCase{
		{
			name:              "all",
			expectedSourceIDs: []string{"CVE-ABC", "CVE-DEF", "CVE-GHI", "CVE-JKL", "CVE-MNO", "CVE-PQR", "CVE-STU", "CVE-VWX", "CVE-Y&Z"},
		},
	}

	runTest := func(testCase testCase, lo, hi int) (errors int) {
		t.Run(testCase.name, func(t *testing.T) {
			vulnerabilities, totalCount, err := store.GetVulnerabilities(ctx, shared.GetVulnerabilitiesArgs{
				Limit:  3,
				Offset: lo,
			})
			if err != nil {
				t.Fatalf("unexpected error getting vulnerabilities: %s", err)
			}
			if totalCount != len(testCase.expectedSourceIDs) {
				t.Errorf("unexpected total count. want=%d have=%d", len(testCase.expectedSourceIDs), totalCount)
			}

			if totalCount != 0 {
				var ids []string
				for _, vulnerability := range vulnerabilities {
					ids = append(ids, vulnerability.SourceID)
				}
				if diff := cmp.Diff(testCase.expectedSourceIDs[lo:hi], ids); diff != "" {
					t.Errorf("unexpected vulnerability ids at offset %d-%d (-want +got):\n%s", lo, hi, diff)
					errors++
				}
			}
		})

		return
	}

	for _, testCase := range testCases {
		if n := len(testCase.expectedSourceIDs); n == 0 {
			runTest(testCase, 0, 0)
		} else {
			for lo := range n {
				if numErrors := runTest(testCase, lo, int(math.Min(float64(lo)+3, float64(n)))); numErrors > 0 {
					break
				}
			}
		}
	}
}
