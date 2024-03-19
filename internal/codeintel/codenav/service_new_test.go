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

func TestGetDefinitions(t *testing.T) {
	t.Run("local", func(t *testing.T) {
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
			{ID: 50, Commit: mockCommit, Root: "sub1/"},
			{ID: 51, Commit: mockCommit, Root: "sub2/"},
			{ID: 52, Commit: mockCommit, Root: "sub3/"},
			{ID: 53, Commit: mockCommit, Root: "sub4/"},
		}
		mockRequestState.SetUploadsDataLoader(uploads)

		locations := []shared.Location{
			{UploadID: 51, Path: "a.go", Range: testRange1},
			{UploadID: 51, Path: "b.go", Range: testRange2},
			{UploadID: 51, Path: "a.go", Range: testRange3},
			{UploadID: 51, Path: "b.go", Range: testRange4},
			{UploadID: 51, Path: "c.go", Range: testRange5},
		}
		mockLsifStore.ExtractDefinitionLocationsFromPositionFunc.PushReturn(locations, nil, nil)

		mockRequest := PositionalRequestArgs{
			RequestArgs: RequestArgs{
				RepositoryID: 51,
				Commit:       mockCommit,
				Limit:        50,
			},
			Path:      mockPath,
			Line:      10,
			Character: 20,
		}
		adjustedLocations, err := svc.GetDefinitions(context.Background(), mockRequest, mockRequestState)
		if err != nil {
			t.Fatalf("unexpected error querying definitions: %s", err)
		}
		expectedLocations := []shared.UploadLocation{
			{Upload: uploads[1], Path: "sub2/a.go", TargetCommit: mockCommit, TargetRange: testRange1},
			{Upload: uploads[1], Path: "sub2/b.go", TargetCommit: mockCommit, TargetRange: testRange2},
			{Upload: uploads[1], Path: "sub2/a.go", TargetCommit: mockCommit, TargetRange: testRange3},
			{Upload: uploads[1], Path: "sub2/b.go", TargetCommit: mockCommit, TargetRange: testRange4},
			{Upload: uploads[1], Path: "sub2/c.go", TargetCommit: mockCommit, TargetRange: testRange5},
		}

		if diff := cmp.Diff(expectedLocations, adjustedLocations); diff != "" {
			t.Errorf("unexpected locations (-want +got):\n%s", diff)
		}
	})

	t.Run("remote", func(t *testing.T) {
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
		mockRequestState.GitTreeTranslator = mockedGitTreeTranslator()
		uploads1 := []uploadsshared.CompletedUpload{
			{ID: 50, Commit: "deadbeef", Root: "sub1/"},
			{ID: 51, Commit: "deadbeef", Root: "sub2/"},
			{ID: 52, Commit: "deadbeef", Root: "sub3/"},
			{ID: 53, Commit: "deadbeef", Root: "sub4/"},
		}
		mockRequestState.SetUploadsDataLoader(uploads1)

		uploads2 := []uploadsshared.CompletedUpload{
			{ID: 150, Commit: "deadbeef1", Root: "sub1/"},
			{ID: 151, Commit: "deadbeef2", Root: "sub2/"},
			{ID: 152, Commit: "deadbeef3", Root: "sub3/"},
			{ID: 153, Commit: "deadbeef4", Root: "sub4/"},
		}
		mockUploadSvc.GetCompletedUploadsWithDefinitionsForMonikersFunc.PushReturn(uploads2, nil)

		// upload #150's commit no longer exists; all others do
		mockGitserverClient.GetCommitFunc.SetDefaultHook(func(ctx context.Context, rn api.RepoName, ci api.CommitID) (*gitdomain.Commit, error) {
			if ci == "deadbeef1" {
				return nil, &gitdomain.RevisionNotFoundError{Repo: rn, Spec: string(ci)}
			}
			return &gitdomain.Commit{ID: ci}, nil
		})

		symbolNames := []string{
			"tsc npm leftpad 0.1.0 padLeft.",
			"local pad_left",
			"tsc npm leftpad 0.2.0 pad-left.",
			"local left_pad",
		}
		mockLsifStore.ExtractDefinitionLocationsFromPositionFunc.PushReturn(nil, symbolNames, nil)

		locations := []shared.Location{
			{UploadID: 151, Path: "a.go", Range: testRange1},
			{UploadID: 151, Path: "b.go", Range: testRange2},
			{UploadID: 151, Path: "a.go", Range: testRange3},
			{UploadID: 151, Path: "b.go", Range: testRange4},
			{UploadID: 151, Path: "c.go", Range: testRange5},
		}
		mockLsifStore.GetMinimalBulkMonikerLocationsFunc.PushReturn(locations, len(locations), nil)

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
		remoteUploads := uploads2
		adjustedLocations, err := svc.GetDefinitions(context.Background(), mockRequest, mockRequestState)
		if err != nil {
			t.Fatalf("unexpected error querying definitions: %s", err)
		}

		xLocations := []shared.UploadLocation{
			{Upload: remoteUploads[1], Path: "sub2/a.go", TargetCommit: "deadbeef2", TargetRange: testRange1},
			{Upload: remoteUploads[1], Path: "sub2/b.go", TargetCommit: "deadbeef2", TargetRange: testRange2},
			{Upload: remoteUploads[1], Path: "sub2/a.go", TargetCommit: "deadbeef2", TargetRange: testRange3},
			{Upload: remoteUploads[1], Path: "sub2/b.go", TargetCommit: "deadbeef2", TargetRange: testRange4},
			{Upload: remoteUploads[1], Path: "sub2/c.go", TargetCommit: "deadbeef2", TargetRange: testRange5},
		}

		if diff := cmp.Diff(xLocations, adjustedLocations); diff != "" {
			t.Errorf("unexpected locations (-want +got):\n%s", diff)
		}

		if history := mockUploadSvc.GetCompletedUploadsWithDefinitionsForMonikersFunc.History(); len(history) != 1 {
			t.Fatalf("unexpected call count for dbstore.DefinitionDump. want=%d have=%d", 1, len(history))
		} else {
			expectedMonikers := []precise.QualifiedMonikerData{
				{
					MonikerData:            precise.MonikerData{Kind: "", Scheme: "tsc", Identifier: "tsc npm leftpad 0.1.0 padLeft."},
					PackageInformationData: precise.PackageInformationData{Manager: "npm", Name: "leftpad", Version: "0.1.0"},
				},
				{
					MonikerData:            precise.MonikerData{Kind: "", Scheme: "tsc", Identifier: "tsc npm leftpad 0.2.0 pad-left."},
					PackageInformationData: precise.PackageInformationData{Manager: "npm", Name: "leftpad", Version: "0.2.0"},
				},
			}
			if diff := cmp.Diff(expectedMonikers, history[0].Arg1); diff != "" {
				t.Errorf("unexpected monikers (-want +got):\n%s", diff)
			}
		}

		if history := mockLsifStore.GetMinimalBulkMonikerLocationsFunc.History(); len(history) != 1 {
			t.Fatalf("unexpected call count for lsifstore.BulkMonikerResults. want=%d have=%d", 1, len(history))
		} else {
			if diff := cmp.Diff([]int{50, 51, 52, 53, 151, 152, 153}, history[0].Arg2); diff != "" {
				t.Errorf("unexpected ids (-want +got):\n%s", diff)
			}

			expectedMonikers := []precise.MonikerData{
				{Kind: "", Scheme: "tsc", Identifier: "tsc npm leftpad 0.1.0 padLeft."},
				{Kind: "", Scheme: "tsc", Identifier: "tsc npm leftpad 0.2.0 pad-left."},
			}
			if diff := cmp.Diff(expectedMonikers, history[0].Arg4); diff != "" {
				t.Errorf("unexpected ids (-want +got):\n%s", diff)
			}
		}
	})
}

func TestGetReferences(t *testing.T) {
	t.Run("local", func(t *testing.T) {
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

		// Empty result set (prevents nil pointer as scanner is always non-nil)
		mockUploadSvc.GetUploadIDsWithReferencesFunc.PushReturn([]int{}, 0, 0, nil)

		locations := []shared.Location{
			{UploadID: 51, Path: "a.go", Range: testRange1},
			{UploadID: 51, Path: "b.go", Range: testRange2},
			{UploadID: 51, Path: "a.go", Range: testRange3},
			{UploadID: 51, Path: "b.go", Range: testRange4},
			{UploadID: 51, Path: "c.go", Range: testRange5},
		}
		mockLsifStore.ExtractReferenceLocationsFromPositionFunc.PushReturn(locations[:1], nil, nil)
		mockLsifStore.ExtractReferenceLocationsFromPositionFunc.PushReturn(locations[1:4], nil, nil)
		mockLsifStore.ExtractReferenceLocationsFromPositionFunc.PushReturn(locations[4:], nil, nil)

		mockCursor := Cursor{}
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
		adjustedLocations, _, err := svc.GetReferences(context.Background(), mockRequest, mockRequestState, mockCursor)
		if err != nil {
			t.Fatalf("unexpected error querying references: %s", err)
		}

		expectedLocations := []shared.UploadLocation{
			{Upload: uploads[1], Path: "sub2/a.go", TargetCommit: "deadbeef", TargetRange: testRange1},
			{Upload: uploads[1], Path: "sub2/b.go", TargetCommit: "deadbeef", TargetRange: testRange2},
			{Upload: uploads[1], Path: "sub2/a.go", TargetCommit: "deadbeef", TargetRange: testRange3},
			{Upload: uploads[1], Path: "sub2/b.go", TargetCommit: "deadbeef", TargetRange: testRange4},
			{Upload: uploads[1], Path: "sub2/c.go", TargetCommit: "deadbeef", TargetRange: testRange5},
		}
		if diff := cmp.Diff(expectedLocations, adjustedLocations); diff != "" {
			t.Errorf("unexpected locations (-want +got):\n%s", diff)
		}
	})

	t.Run("remote", func(t *testing.T) {
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

		definitionUploads := []uploadsshared.CompletedUpload{
			{ID: 150, Commit: "deadbeef1", Root: "sub1/"},
			{ID: 151, Commit: "deadbeef2", Root: "sub2/"},
			{ID: 152, Commit: "deadbeef3", Root: "sub3/"},
			{ID: 153, Commit: "deadbeef4", Root: "sub4/"},
		}
		mockUploadSvc.GetCompletedUploadsWithDefinitionsForMonikersFunc.PushReturn(definitionUploads, nil)

		referenceUploads := []uploadsshared.CompletedUpload{
			{ID: 250, Commit: "deadbeef1", Root: "sub1/"},
			{ID: 251, Commit: "deadbeef2", Root: "sub2/"},
			{ID: 252, Commit: "deadbeef3", Root: "sub3/"},
			{ID: 253, Commit: "deadbeef4", Root: "sub4/"},
		}
		mockUploadSvc.GetCompletedUploadsByIDsFunc.PushReturn(nil, nil) // empty
		mockUploadSvc.GetCompletedUploadsByIDsFunc.PushReturn(referenceUploads[:2], nil)
		mockUploadSvc.GetCompletedUploadsByIDsFunc.PushReturn(referenceUploads[2:], nil)

		mockUploadSvc.GetUploadIDsWithReferencesFunc.PushReturn([]int{250, 251}, 0, 4, nil)
		mockUploadSvc.GetUploadIDsWithReferencesFunc.PushReturn([]int{252, 253}, 0, 2, nil)

		// upload #150/#250's commits no longer exists; all others do
		mockGitserverClient.GetCommitFunc.SetDefaultHook(func(ctx context.Context, rn api.RepoName, ci api.CommitID) (*gitdomain.Commit, error) {
			if ci == "deadbeef1" {
				return nil, &gitdomain.RevisionNotFoundError{Repo: rn, Spec: string(ci)}
			}
			return &gitdomain.Commit{ID: ci}, nil
		})

		monikers := []precise.MonikerData{
			{Scheme: "tsc", Identifier: "tsc npm leftpad 0.1.0 padLeft."},
			{Scheme: "tsc", Identifier: "tsc npm leftpad 0.2.0 pad_left."},
			{Scheme: "tsc", Identifier: "tsc npm leftpad 0.3.0 pad-left."},
		}
		// mockLsifStore.GetMonikersByPositionFunc.PushReturn([][]precise.MonikerData{{monikers[0]}}, nil)
		// mockLsifStore.GetMonikersByPositionFunc.PushReturn([][]precise.MonikerData{{monikers[1]}}, nil)
		// mockLsifStore.GetMonikersByPositionFunc.PushReturn([][]precise.MonikerData{{monikers[2]}}, nil)
		// mockLsifStore.GetMonikersByPositionFunc.PushReturn([][]precise.MonikerData{{monikers[3]}}, nil)

		packageInformation1 := precise.PackageInformationData{Manager: "npm", Name: "leftpad", Version: "0.1.0"}
		packageInformation2 := precise.PackageInformationData{Manager: "npm", Name: "leftpad", Version: "0.2.0"}
		packageInformation3 := precise.PackageInformationData{Manager: "npm", Name: "leftpad", Version: "0.3.0"}
		// mockLsifStore.GetPackageInformationFunc.PushReturn(packageInformation1, true, nil)
		// mockLsifStore.GetPackageInformationFunc.PushReturn(packageInformation2, true, nil)
		// mockLsifStore.GetPackageInformationFunc.PushReturn(packageInformation3, true, nil)

		locations := []shared.Location{
			{UploadID: 51, Path: "a.go", Range: testRange1},
			{UploadID: 51, Path: "b.go", Range: testRange2},
			{UploadID: 51, Path: "a.go", Range: testRange3},
			{UploadID: 51, Path: "b.go", Range: testRange4},
			{UploadID: 51, Path: "c.go", Range: testRange5},
		}
		symbolNames := []string{
			"tsc npm leftpad 0.1.0 padLeft.",
			"tsc npm leftpad 0.2.0 pad_left.",
			"tsc npm leftpad 0.3.0 pad-left.",
		}
		mockLsifStore.ExtractReferenceLocationsFromPositionFunc.PushReturn(locations, symbolNames, nil)

		monikerLocations := []shared.Location{
			{UploadID: 53, Path: "a.go", Range: testRange1},
			{UploadID: 53, Path: "b.go", Range: testRange2},
			{UploadID: 53, Path: "a.go", Range: testRange3},
			{UploadID: 53, Path: "b.go", Range: testRange4},
			{UploadID: 53, Path: "c.go", Range: testRange5},
		}
		mockLsifStore.GetMinimalBulkMonikerLocationsFunc.PushReturn(monikerLocations[0:1], 1, nil) // defs
		mockLsifStore.GetMinimalBulkMonikerLocationsFunc.PushReturn(monikerLocations[1:2], 1, nil) // refs batch 1
		mockLsifStore.GetMinimalBulkMonikerLocationsFunc.PushReturn(monikerLocations[2:], 3, nil)  // refs batch 2

		// uploads := []dbstore.CompletedUpload{
		// 	{ID: 50, Commit: "deadbeef", Root: "sub1/"},
		// 	{ID: 51, Commit: "deadbeef", Root: "sub2/"},
		// 	{ID: 52, Commit: "deadbeef", Root: "sub3/"},
		// 	{ID: 53, Commit: "deadbeef", Root: "sub4/"},
		// }
		// resolver.SetUploadsDataLoader(uploads)

		mockCursor := Cursor{}
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
		adjustedLocations, _, err := svc.GetReferences(context.Background(), mockRequest, mockRequestState, mockCursor)
		if err != nil {
			t.Fatalf("unexpected error querying references: %s", err)
		}

		expectedLocations := []shared.UploadLocation{
			{Upload: uploads[1], Path: "sub2/a.go", TargetCommit: "deadbeef", TargetRange: testRange1},
			{Upload: uploads[1], Path: "sub2/b.go", TargetCommit: "deadbeef", TargetRange: testRange2},
			{Upload: uploads[1], Path: "sub2/a.go", TargetCommit: "deadbeef", TargetRange: testRange3},
			{Upload: uploads[1], Path: "sub2/b.go", TargetCommit: "deadbeef", TargetRange: testRange4},
			{Upload: uploads[1], Path: "sub2/c.go", TargetCommit: "deadbeef", TargetRange: testRange5},
			{Upload: uploads[3], Path: "sub4/a.go", TargetCommit: "deadbeef", TargetRange: testRange1},
			{Upload: uploads[3], Path: "sub4/b.go", TargetCommit: "deadbeef", TargetRange: testRange2},
			{Upload: uploads[3], Path: "sub4/a.go", TargetCommit: "deadbeef", TargetRange: testRange3},
			{Upload: uploads[3], Path: "sub4/b.go", TargetCommit: "deadbeef", TargetRange: testRange4},
			{Upload: uploads[3], Path: "sub4/c.go", TargetCommit: "deadbeef", TargetRange: testRange5},
		}
		if diff := cmp.Diff(expectedLocations, adjustedLocations); diff != "" {
			t.Errorf("unexpected locations (-want +got):\n%s", diff)
		}

		if history := mockUploadSvc.GetCompletedUploadsWithDefinitionsForMonikersFunc.History(); len(history) != 1 {
			t.Fatalf("unexpected call count for dbstore.DefinitionDump. want=%d have=%d", 1, len(history))
		} else {
			expectedMonikers := []precise.QualifiedMonikerData{
				{MonikerData: monikers[0], PackageInformationData: packageInformation1},
				{MonikerData: monikers[1], PackageInformationData: packageInformation2},
				{MonikerData: monikers[2], PackageInformationData: packageInformation3},
			}
			if diff := cmp.Diff(expectedMonikers, history[0].Arg1); diff != "" {
				t.Errorf("unexpected monikers (-want +got):\n%s", diff)
			}
		}

		if history := mockLsifStore.GetMinimalBulkMonikerLocationsFunc.History(); len(history) != 3 {
			t.Fatalf("unexpected call count for lsifstore.BulkMonikerResults. want=%d have=%d", 3, len(history))
		} else {
			if diff := cmp.Diff([]int{50, 51, 52, 53, 151, 152, 153}, history[0].Arg2); diff != "" {
				t.Errorf("unexpected ids (-want +got):\n%s", diff)
			}

			expectedMonikers := []precise.MonikerData{
				monikers[0],
				monikers[1],
				monikers[2],
			}
			if diff := cmp.Diff(expectedMonikers, history[0].Arg4); diff != "" {
				t.Errorf("unexpected monikers (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff([]int{250, 251}, history[1].Arg2); diff != "" {
				t.Errorf("unexpected ids (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(expectedMonikers, history[1].Arg4); diff != "" {
				t.Errorf("unexpected monikers (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff([]int{252, 253}, history[2].Arg2); diff != "" {
				t.Errorf("unexpected ids (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(expectedMonikers, history[2].Arg4); diff != "" {
				t.Errorf("unexpected monikers (-want +got):\n%s", diff)
			}
		}
	})
}

func TestGetImplementations(t *testing.T) {
	t.Run("local", func(t *testing.T) {
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

		// Empty result set (prevents nil pointer as scanner is always non-nil)
		mockUploadSvc.GetUploadIDsWithReferencesFunc.PushReturn([]int{}, 0, 0, nil)

		locations := []shared.Location{
			{UploadID: 51, Path: "a.go", Range: testRange1},
			{UploadID: 51, Path: "b.go", Range: testRange2},
			{UploadID: 51, Path: "a.go", Range: testRange3},
			{UploadID: 51, Path: "b.go", Range: testRange4},
			{UploadID: 51, Path: "c.go", Range: testRange5},
		}
		mockLsifStore.ExtractImplementationLocationsFromPositionFunc.PushReturn(locations, nil, nil)

		uploads := []uploadsshared.CompletedUpload{
			{ID: 50, Commit: "deadbeef", Root: "sub1/"},
			{ID: 51, Commit: "deadbeef", Root: "sub2/"},
			{ID: 52, Commit: "deadbeef", Root: "sub3/"},
			{ID: 53, Commit: "deadbeef", Root: "sub4/"},
		}
		mockRequestState.SetUploadsDataLoader(uploads)
		mockCursor := Cursor{}
		mockRequest := PositionalRequestArgs{
			RequestArgs: RequestArgs{
				RepositoryID: 51,
				Commit:       "deadbeef",
				Limit:        50,
			},
			Path:      "s1/main.go",
			Line:      10,
			Character: 20,
		}
		adjustedLocations, _, err := svc.GetImplementations(context.Background(), mockRequest, mockRequestState, mockCursor)
		if err != nil {
			t.Fatalf("unexpected error querying implementations: %s", err)
		}

		expectedLocations := []shared.UploadLocation{
			{Upload: uploads[1], Path: "sub2/a.go", TargetCommit: "deadbeef", TargetRange: testRange1},
			{Upload: uploads[1], Path: "sub2/b.go", TargetCommit: "deadbeef", TargetRange: testRange2},
			{Upload: uploads[1], Path: "sub2/a.go", TargetCommit: "deadbeef", TargetRange: testRange3},
			{Upload: uploads[1], Path: "sub2/b.go", TargetCommit: "deadbeef", TargetRange: testRange4},
			{Upload: uploads[1], Path: "sub2/c.go", TargetCommit: "deadbeef", TargetRange: testRange5},
		}
		if diff := cmp.Diff(expectedLocations, adjustedLocations); diff != "" {
			t.Errorf("unexpected locations (-want +got):\n%s", diff)
		}
	})
}
