package codenav

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	sgtypes "github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

func TestHover(t *testing.T) {
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
	mockRequestState.SetLocalGitTreeTranslator(mockGitserverClient, &sgtypes.Repo{ID: 42}, mockCommit, mockPath, hunkCache)
	uploads := []uploadsshared.CompletedUpload{
		{ID: 50, Commit: "deadbeef", Root: "sub1/"},
		{ID: 51, Commit: "deadbeef", Root: "sub2/"},
		{ID: 52, Commit: "deadbeef", Root: "sub3/"},
		{ID: 53, Commit: "deadbeef", Root: "sub4/"},
	}
	mockRequestState.SetUploadsDataLoader(uploads)

	expectedRange := shared.Range{
		Start: shared.Position{Line: 10, Character: 10},
		End:   shared.Position{Line: 15, Character: 25},
	}
	mockLsifStore.GetHoverFunc.PushReturn("", shared.Range{}, false, nil)
	mockLsifStore.GetHoverFunc.PushReturn("doctext", expectedRange, true, nil)

	mockRequest := PositionalRequestArgs{
		RequestArgs: RequestArgs{
			RepositoryID: 42,
			Commit:       mockCommit,
			Limit:        50,
		},
		Path:      mockPath,
		Line:      10,
		Character: 20,
	}
	text, rn, exists, err := svc.GetHover(context.Background(), mockRequest, mockRequestState)
	if err != nil {
		t.Fatalf("unexpected error querying hover: %s", err)
	}
	if !exists {
		t.Fatalf("expected hover to exist")
	}

	if text != "doctext" {
		t.Errorf("unexpected text. want=%q have=%q", "doctext", text)
	}
	if diff := cmp.Diff(expectedRange, rn); diff != "" {
		t.Errorf("unexpected range (-want +got):\n%s", diff)
	}
}

func TestHoverRemote(t *testing.T) {
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
	mockRequestState.SetLocalGitTreeTranslator(mockGitserverClient, &sgtypes.Repo{ID: 42}, mockCommit, mockPath, hunkCache)
	uploads := []uploadsshared.CompletedUpload{
		{ID: 50, Commit: "deadbeef"},
	}
	mockRequestState.SetUploadsDataLoader(uploads)

	expectedRange := shared.Range{
		Start: shared.Position{Line: 10, Character: 10},
		End:   shared.Position{Line: 15, Character: 25},
	}
	mockLsifStore.GetHoverFunc.PushReturn("", expectedRange, true, nil)

	remoteRange := shared.Range{
		Start: shared.Position{Line: 30, Character: 30},
		End:   shared.Position{Line: 35, Character: 45},
	}
	mockLsifStore.GetHoverFunc.PushReturn("doctext", remoteRange, true, nil)

	uploadsWithDefinitions := []uploadsshared.CompletedUpload{
		{ID: 150, Commit: "deadbeef1", Root: "sub1/"},
		{ID: 151, Commit: "deadbeef2", Root: "sub2/"},
		{ID: 152, Commit: "deadbeef3", Root: "sub3/"},
		{ID: 153, Commit: "deadbeef4", Root: "sub4/"},
	}
	mockUploadSvc.GetCompletedUploadsWithDefinitionsForMonikersFunc.PushReturn(uploadsWithDefinitions, nil)

	monikers := []precise.MonikerData{
		{Kind: "import", Scheme: "tsc", Identifier: "padLeft", PackageInformationID: "51"},
		{Kind: "export", Scheme: "tsc", Identifier: "pad_left", PackageInformationID: "52"},
		{Kind: "import", Scheme: "tsc", Identifier: "pad-left", PackageInformationID: "53"},
		{Kind: "import", Scheme: "tsc", Identifier: "left_pad"},
	}
	mockLsifStore.GetMonikersByPositionFunc.PushReturn([][]precise.MonikerData{{monikers[0]}}, nil)
	mockLsifStore.GetMonikersByPositionFunc.PushReturn([][]precise.MonikerData{{monikers[1]}}, nil)
	mockLsifStore.GetMonikersByPositionFunc.PushReturn([][]precise.MonikerData{{monikers[2]}}, nil)
	mockLsifStore.GetMonikersByPositionFunc.PushReturn([][]precise.MonikerData{{monikers[3]}}, nil)

	packageInformation1 := precise.PackageInformationData{Name: "leftpad", Version: "0.1.0"}
	packageInformation2 := precise.PackageInformationData{Name: "leftpad", Version: "0.2.0"}
	mockLsifStore.GetPackageInformationFunc.PushReturn(packageInformation1, true, nil)
	mockLsifStore.GetPackageInformationFunc.PushReturn(packageInformation2, true, nil)

	locations := []shared.Location{
		{UploadID: 151, Path: "a.go", Range: testRange1},
		{UploadID: 151, Path: "b.go", Range: testRange2},
		{UploadID: 151, Path: "a.go", Range: testRange3},
		{UploadID: 151, Path: "b.go", Range: testRange4},
		{UploadID: 151, Path: "c.go", Range: testRange5},
	}
	mockLsifStore.GetBulkMonikerLocationsFunc.PushReturn(locations, 0, nil)
	mockLsifStore.GetBulkMonikerLocationsFunc.PushReturn(locations, len(locations), nil)

	mockGitserverClient.GetCommitFunc.SetDefaultHook(func(ctx context.Context, rn api.RepoName, ci api.CommitID) (*gitdomain.Commit, error) {
		return &gitdomain.Commit{ID: "sha"}, nil
	})

	mockRequest := PositionalRequestArgs{
		RequestArgs: RequestArgs{
			RepositoryID: 42,
			Commit:       mockCommit,
			Limit:        50,
		},
		Path:      mockPath,
		Line:      10,
		Character: 20,
	}
	text, rn, exists, err := svc.GetHover(context.Background(), mockRequest, mockRequestState)
	if err != nil {
		t.Fatalf("unexpected error querying hover: %s", err)
	}
	if !exists {
		t.Fatalf("expected hover to exist")
	}

	if text != "doctext" {
		t.Errorf("unexpected text. want=%q have=%q", "doctext", text)
	}
	if diff := cmp.Diff(expectedRange, rn); diff != "" {
		t.Errorf("unexpected range (-want +got):\n%s", diff)
	}
}
