package resolvers

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/dbstore"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/lsifstore"
	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func TestDiagnostics(t *testing.T) {
	mockDBStore := NewMockDBStore()
	mockLSIFStore := NewMockLSIFStore()
	mockGitserverClient := NewMockGitserverClient()
	mockPositionAdjuster := noopPositionAdjuster()

	diagnostics := []lsifstore.Diagnostic{
		{DiagnosticData: semantic.DiagnosticData{Code: "c1"}},
		{DiagnosticData: semantic.DiagnosticData{Code: "c2"}},
		{DiagnosticData: semantic.DiagnosticData{Code: "c3"}},
		{DiagnosticData: semantic.DiagnosticData{Code: "c4"}},
		{DiagnosticData: semantic.DiagnosticData{Code: "c5"}},
	}

	mockLSIFStore.DiagnosticsFunc.PushReturn(diagnostics[0:1], 1, nil)
	mockLSIFStore.DiagnosticsFunc.PushReturn(diagnostics[1:4], 3, nil)
	mockLSIFStore.DiagnosticsFunc.PushReturn(diagnostics[4:], 26, nil)

	uploads := []dbstore.Dump{
		{ID: 50, Commit: "deadbeef", Root: "sub1/"},
		{ID: 51, Commit: "deadbeef", Root: "sub2/"},
		{ID: 52, Commit: "deadbeef", Root: "sub3/"},
		{ID: 53, Commit: "deadbeef", Root: "sub4/"},
	}
	resolver := newQueryResolver(
		mockDBStore,
		mockLSIFStore,
		newCachedCommitChecker(mockGitserverClient),
		mockPositionAdjuster,
		42,
		"deadbeef",
		"s1/main.go",
		uploads,
		newOperations(&observation.TestContext),
	)
	adjustedDiagnostics, totalCount, err := resolver.Diagnostics(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error querying diagnostics: %s", err)
	}

	if totalCount != 30 {
		t.Errorf("unexpected count. want=%d have=%d", 30, totalCount)
	}

	expectedDiagnostics := []AdjustedDiagnostic{
		{Dump: uploads[0], AdjustedCommit: "deadbeef", Diagnostic: lsifstore.Diagnostic{Path: "sub1/", DiagnosticData: semantic.DiagnosticData{Code: "c1"}}},
		{Dump: uploads[1], AdjustedCommit: "deadbeef", Diagnostic: lsifstore.Diagnostic{Path: "sub2/", DiagnosticData: semantic.DiagnosticData{Code: "c2"}}},
		{Dump: uploads[1], AdjustedCommit: "deadbeef", Diagnostic: lsifstore.Diagnostic{Path: "sub2/", DiagnosticData: semantic.DiagnosticData{Code: "c3"}}},
		{Dump: uploads[1], AdjustedCommit: "deadbeef", Diagnostic: lsifstore.Diagnostic{Path: "sub2/", DiagnosticData: semantic.DiagnosticData{Code: "c4"}}},
		{Dump: uploads[2], AdjustedCommit: "deadbeef", Diagnostic: lsifstore.Diagnostic{Path: "sub3/", DiagnosticData: semantic.DiagnosticData{Code: "c5"}}},
	}
	if diff := cmp.Diff(expectedDiagnostics, adjustedDiagnostics); diff != "" {
		t.Errorf("unexpected diagnostics (-want +got):\n%s", diff)
	}

	var limits []int
	for _, call := range mockLSIFStore.DiagnosticsFunc.History() {
		limits = append(limits, call.Arg3)
	}
	if diff := cmp.Diff([]int{5, 4, 1, 0}, limits); diff != "" {
		t.Errorf("unexpected limits (-want +got):\n%s", diff)
	}
}
