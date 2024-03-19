package codenav

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	sgtypes "github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

func TestDiagnostics(t *testing.T) {
	// Set up mocks
	mockRepoStore := defaultMockRepoStore()
	mockLsifStore := NewMockLsifStore()
	mockUploadSvc := NewMockUploadService()
	mockGitserverClient := gitserver.NewMockClient()
	hunkCache, _ := NewHunkCache(50)

	// Init service
	svc := newService(&observation.TestContext, mockRepoStore, mockLsifStore, mockUploadSvc, mockGitserverClient)

	// Set up request state
	mockRequestState := RequestState{}
	mockRequestState.SetLocalCommitCache(mockRepoStore, mockGitserverClient)
	mockRequestState.SetLocalGitTreeTranslator(mockGitserverClient, &sgtypes.Repo{}, mockCommit, mockPath, hunkCache)
	uploads := []uploadsshared.CompletedUpload{
		{ID: 50, Commit: "deadbeef", Root: "sub1/"},
		{ID: 51, Commit: "deadbeef", Root: "sub2/"},
		{ID: 52, Commit: "deadbeef", Root: "sub3/"},
		{ID: 53, Commit: "deadbeef", Root: "sub4/"},
	}
	mockRequestState.SetUploadsDataLoader(uploads)

	diagnostics := []shared.Diagnostic{
		{DiagnosticData: precise.DiagnosticData{Code: "c1"}},
		{DiagnosticData: precise.DiagnosticData{Code: "c2"}},
		{DiagnosticData: precise.DiagnosticData{Code: "c3"}},
		{DiagnosticData: precise.DiagnosticData{Code: "c4"}},
		{DiagnosticData: precise.DiagnosticData{Code: "c5"}},
	}
	mockLsifStore.GetDiagnosticsFunc.PushReturn(diagnostics[0:1], 1, nil)
	mockLsifStore.GetDiagnosticsFunc.PushReturn(diagnostics[1:4], 3, nil)
	mockLsifStore.GetDiagnosticsFunc.PushReturn(diagnostics[4:], 26, nil)

	mockRequest := PositionalRequestArgs{
		RequestArgs: RequestArgs{
			RepositoryID: 42,
			Commit:       mockCommit,
			Limit:        5,
		},
		Path:      mockPath,
		Line:      10,
		Character: 20,
	}
	adjustedDiagnostics, totalCount, err := svc.GetDiagnostics(context.Background(), mockRequest, mockRequestState)
	if err != nil {
		t.Fatalf("unexpected error querying diagnostics: %s", err)
	}

	if totalCount != 30 {
		t.Errorf("unexpected count. want=%d have=%d", 30, totalCount)
	}

	expectedDiagnostics := []DiagnosticAtUpload{
		{Upload: uploads[0], AdjustedCommit: "deadbeef", Diagnostic: shared.Diagnostic{Path: "sub1/", DiagnosticData: precise.DiagnosticData{Code: "c1"}}},
		{Upload: uploads[1], AdjustedCommit: "deadbeef", Diagnostic: shared.Diagnostic{Path: "sub2/", DiagnosticData: precise.DiagnosticData{Code: "c2"}}},
		{Upload: uploads[1], AdjustedCommit: "deadbeef", Diagnostic: shared.Diagnostic{Path: "sub2/", DiagnosticData: precise.DiagnosticData{Code: "c3"}}},
		{Upload: uploads[1], AdjustedCommit: "deadbeef", Diagnostic: shared.Diagnostic{Path: "sub2/", DiagnosticData: precise.DiagnosticData{Code: "c4"}}},
		{Upload: uploads[2], AdjustedCommit: "deadbeef", Diagnostic: shared.Diagnostic{Path: "sub3/", DiagnosticData: precise.DiagnosticData{Code: "c5"}}},
	}
	if diff := cmp.Diff(expectedDiagnostics, adjustedDiagnostics); diff != "" {
		t.Errorf("unexpected diagnostics (-want +got):\n%s", diff)
	}

	var limits []int
	for _, call := range mockLsifStore.GetDiagnosticsFunc.History() {
		limits = append(limits, call.Arg3)
	}
	if diff := cmp.Diff([]int{5, 4, 1, 0}, limits); diff != "" {
		t.Errorf("unexpected limits (-want +got):\n%s", diff)
	}
}

func TestDiagnosticsWithSubRepoPermissions(t *testing.T) {
	// Set up mocks
	mockRepoStore := defaultMockRepoStore()
	mockLsifStore := NewMockLsifStore()
	mockUploadSvc := NewMockUploadService()
	mockGitserverClient := gitserver.NewMockClient()
	hunkCache, _ := NewHunkCache(50)

	// Init service
	svc := newService(&observation.TestContext, mockRepoStore, mockLsifStore, mockUploadSvc, mockGitserverClient)

	// Set up request state
	mockRequestState := RequestState{}
	mockRequestState.SetLocalCommitCache(mockRepoStore, mockGitserverClient)
	mockRequestState.SetLocalGitTreeTranslator(mockGitserverClient, &sgtypes.Repo{}, mockCommit, mockPath, hunkCache)
	uploads := []uploadsshared.CompletedUpload{
		{ID: 50, Commit: "deadbeef", Root: "sub1/"},
		{ID: 51, Commit: "deadbeef", Root: "sub2/"},
		{ID: 52, Commit: "deadbeef", Root: "sub3/"},
		{ID: 53, Commit: "deadbeef", Root: "sub4/"},
	}
	mockRequestState.SetUploadsDataLoader(uploads)

	// Applying sub-repo permissions
	checker := authz.NewMockSubRepoPermissionChecker()
	checker.EnabledFunc.SetDefaultHook(func() bool {
		return true
	})
	checker.PermissionsFunc.SetDefaultHook(func(ctx context.Context, i int32, content authz.RepoContent) (authz.Perms, error) {
		if content.Path == "sub2/" {
			return authz.Read, nil
		}
		return authz.None, nil
	})
	mockRequestState.SetAuthChecker(checker)

	diagnostics := []shared.Diagnostic{
		{DiagnosticData: precise.DiagnosticData{Code: "c1"}},
		{DiagnosticData: precise.DiagnosticData{Code: "c2"}},
		{DiagnosticData: precise.DiagnosticData{Code: "c3"}},
		{DiagnosticData: precise.DiagnosticData{Code: "c4"}},
		{DiagnosticData: precise.DiagnosticData{Code: "c5"}},
	}
	mockLsifStore.GetDiagnosticsFunc.PushReturn(diagnostics[0:1], 1, nil)
	mockLsifStore.GetDiagnosticsFunc.PushReturn(diagnostics[1:4], 3, nil)
	mockLsifStore.GetDiagnosticsFunc.PushReturn(diagnostics[4:], 26, nil)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
	mockRequest := PositionalRequestArgs{
		RequestArgs: RequestArgs{
			RepositoryID: 42,
			Commit:       mockCommit,
			Limit:        5,
		},
		Path:      mockPath,
		Line:      10,
		Character: 20,
	}
	adjustedDiagnostics, totalCount, err := svc.GetDiagnostics(ctx, mockRequest, mockRequestState)
	if err != nil {
		t.Fatalf("unexpected error querying diagnostics: %s", err)
	}

	if totalCount != 30 {
		t.Errorf("unexpected count. want=%d have=%d", 30, totalCount)
	}

	expectedDiagnostics := []DiagnosticAtUpload{
		{Upload: uploads[1], AdjustedCommit: "deadbeef", Diagnostic: shared.Diagnostic{Path: "sub2/", DiagnosticData: precise.DiagnosticData{Code: "c2"}}},
		{Upload: uploads[1], AdjustedCommit: "deadbeef", Diagnostic: shared.Diagnostic{Path: "sub2/", DiagnosticData: precise.DiagnosticData{Code: "c3"}}},
		{Upload: uploads[1], AdjustedCommit: "deadbeef", Diagnostic: shared.Diagnostic{Path: "sub2/", DiagnosticData: precise.DiagnosticData{Code: "c4"}}},
	}
	if diff := cmp.Diff(expectedDiagnostics, adjustedDiagnostics); diff != "" {
		t.Errorf("unexpected diagnostics (-want +got):\n%s", diff)
	}

	var limits []int
	for _, call := range mockLsifStore.GetDiagnosticsFunc.History() {
		limits = append(limits, call.Arg3)
	}
	if diff := cmp.Diff([]int{5, 5, 2, 2}, limits); diff != "" {
		t.Errorf("unexpected limits (-want +got):\n%s", diff)
	}
}
