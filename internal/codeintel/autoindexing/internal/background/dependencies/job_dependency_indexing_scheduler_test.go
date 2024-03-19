package dependencies

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestDependencyIndexingSchedulerHandler(t *testing.T) {
	mockUploadsSvc := NewMockUploadService()
	mockRepoStore := NewMockReposStore()
	mockExtSvcStore := NewMockExternalServiceStore()
	mockRepoUpdater := NewMockRepoUpdaterClient()
	mockScanner := NewMockPackageReferenceScanner()
	mockWorkerStore := NewMockWorkerStore[dependencyIndexingJob]()

	mockRepoStore.ListMinimalReposFunc.PushReturn([]types.MinimalRepo{
		{
			ID:    0,
			Name:  "",
			Stars: 0,
		},
	}, nil)

	mockUploadsSvc.GetUploadByIDFunc.SetDefaultReturn(shared.Upload{ID: 42, RepositoryID: 50, Indexer: "lsif-go"}, true, nil)
	mockUploadsSvc.ReferencesForUploadFunc.SetDefaultReturn(mockScanner, nil)

	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/sample/text", Version: "v2.2.0"}}, true, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/sample/text", Version: "v3.2.0"}}, true, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/cheese/burger", Version: "v3.2.2"}}, true, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/cheese/burger", Version: "v2.2.1"}}, true, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/cheese/burger", Version: "v4.2.3"}}, true, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/sample/text", Version: "v1.2.0"}}, true, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/banana/world", Version: "v0.0.1"}}, true, nil)
	mockScanner.NextFunc.SetDefaultReturn(shared.PackageReference{}, false, nil)

	mockGitserverReposStore := NewMockGitserverRepoStore()
	mockGitserverReposStore.GetByNamesFunc.PushReturn(map[api.RepoName]*types.GitserverRepo{
		"github.com/sample/text": {
			CloneStatus: types.CloneStatusCloned,
		},
		"github.com/cheese/burger": {
			CloneStatus: types.CloneStatusCloned,
		},
		"github.com/banana/world": {
			CloneStatus: types.CloneStatusCloned,
		},
	}, nil)

	indexEnqueuer := NewMockIndexEnqueuer()

	dotcom.MockSourcegraphDotComMode(t, true)

	handler := &dependencyIndexingSchedulerHandler{
		uploadsSvc:         mockUploadsSvc,
		repoStore:          mockRepoStore,
		indexEnqueuer:      indexEnqueuer,
		extsvcStore:        mockExtSvcStore,
		workerStore:        mockWorkerStore,
		gitserverRepoStore: mockGitserverReposStore,
		repoUpdater:        mockRepoUpdater,
	}

	logger := logtest.Scoped(t)
	job := dependencyIndexingJob{
		UploadID:            42,
		ExternalServiceKind: "",
		ExternalServiceSync: time.Time{},
	}
	if err := handler.Handle(context.Background(), logger, job); err != nil {
		t.Fatalf("unexpected error performing update: %s", err)
	}

	if len(mockExtSvcStore.ListFunc.History()) != 0 {
		t.Errorf("unexpected number of calls to extsvcStore.List. want=%d have=%d", 0, len(mockExtSvcStore.ListFunc.History()))
	}

	if len(indexEnqueuer.QueueIndexesForPackageFunc.History()) != 7 {
		t.Errorf("unexpected number of calls to QueueIndexesForPackage. want=%d have=%d", 6, len(indexEnqueuer.QueueIndexesForPackageFunc.History()))
	} else {
		var packages []dependencies.MinimialVersionedPackageRepo
		for _, call := range indexEnqueuer.QueueIndexesForPackageFunc.History() {
			packages = append(packages, call.Arg1)
		}
		sort.Slice(packages, func(i, j int) bool {
			for _, pair := range [][2]string{
				{packages[i].Scheme, packages[j].Scheme},
				{string(packages[i].Name), string(packages[j].Name)},
				{packages[i].Version, packages[j].Version},
			} {
				if pair[0] < pair[1] {
					return true
				}
				if pair[1] < pair[0] {
					break
				}
			}

			return false
		})

		expectedPackages := []dependencies.MinimialVersionedPackageRepo{
			{Scheme: "gomod", Name: "https://github.com/banana/world", Version: "v0.0.1"},
			{Scheme: "gomod", Name: "https://github.com/cheese/burger", Version: "v2.2.1"},
			{Scheme: "gomod", Name: "https://github.com/cheese/burger", Version: "v3.2.2"},
			{Scheme: "gomod", Name: "https://github.com/cheese/burger", Version: "v4.2.3"},
			{Scheme: "gomod", Name: "https://github.com/sample/text", Version: "v1.2.0"},
			{Scheme: "gomod", Name: "https://github.com/sample/text", Version: "v2.2.0"},
			{Scheme: "gomod", Name: "https://github.com/sample/text", Version: "v3.2.0"},
		}
		if diff := cmp.Diff(expectedPackages, packages); diff != "" {
			t.Errorf("unexpected packages (-want +got):\n%s", diff)
		}
	}
}

func TestDependencyIndexingSchedulerHandlerCustomer(t *testing.T) {
	mockUploadsSvc := NewMockUploadService()
	mockRepoStore := NewMockReposStore()
	mockExtSvcStore := NewMockExternalServiceStore()
	mockRepoUpdater := NewMockRepoUpdaterClient()
	mockScanner := NewMockPackageReferenceScanner()
	mockWorkerStore := NewMockWorkerStore[dependencyIndexingJob]()
	mockUploadsSvc.GetUploadByIDFunc.SetDefaultReturn(shared.Upload{ID: 42, RepositoryID: 50, Indexer: "lsif-go"}, true, nil)
	mockUploadsSvc.ReferencesForUploadFunc.SetDefaultReturn(mockScanner, nil)

	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/sample/text", Version: "v2.2.0"}}, true, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/sample/text", Version: "v3.2.0"}}, true, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/cheese/burger", Version: "v3.2.2"}}, true, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/cheese/burger", Version: "v2.2.1"}}, true, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/cheese/burger", Version: "v4.2.3"}}, true, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/sample/text", Version: "v1.2.0"}}, true, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/banana/world", Version: "v1.2.0"}}, true, nil)
	mockScanner.NextFunc.SetDefaultReturn(shared.PackageReference{}, false, nil)

	// simulate github.com/banana/world not being known to the instance
	mockRepoStore.ListMinimalReposFunc.PushReturn([]types.MinimalRepo{
		{Name: "github.com/cheese/burger"}, {Name: "github.com/sample/text"},
	}, nil)

	mockGitserverReposStore := NewMockGitserverRepoStore()
	mockGitserverReposStore.GetByNamesFunc.PushReturn(map[api.RepoName]*types.GitserverRepo{
		"github.com/sample/text": {
			CloneStatus: types.CloneStatusCloned,
		},
		"github.com/cheese/burger": {
			CloneStatus: types.CloneStatusCloned,
		},
	}, nil)

	indexEnqueuer := NewMockIndexEnqueuer()

	dotcom.MockSourcegraphDotComMode(t, false)

	handler := &dependencyIndexingSchedulerHandler{
		uploadsSvc:         mockUploadsSvc,
		repoStore:          mockRepoStore,
		indexEnqueuer:      indexEnqueuer,
		extsvcStore:        mockExtSvcStore,
		workerStore:        mockWorkerStore,
		gitserverRepoStore: mockGitserverReposStore,
		repoUpdater:        mockRepoUpdater,
	}

	logger := logtest.Scoped(t)
	job := dependencyIndexingJob{
		UploadID:            42,
		ExternalServiceKind: "",
		ExternalServiceSync: time.Time{},
	}
	if err := handler.Handle(context.Background(), logger, job); err != nil {
		t.Fatalf("unexpected error performing update: %s", err)
	}

	if len(mockRepoUpdater.RepoLookupFunc.History()) != 0 {
		t.Errorf("unexpected number of calls to repoUpdater.RepoLookup. want=%d have=%d", 0, len(mockRepoUpdater.RepoLookupFunc.History()))
	}

	if len(mockExtSvcStore.ListFunc.History()) != 0 {
		t.Errorf("unexpected number of calls to extsvcStore.List. want=%d have=%d", 0, len(mockExtSvcStore.ListFunc.History()))
	}

	if len(indexEnqueuer.QueueIndexesForPackageFunc.History()) != 6 {
		t.Errorf("unexpected number of calls to QueueIndexesForPackage. want=%d have=%d", 6, len(indexEnqueuer.QueueIndexesForPackageFunc.History()))
	} else {
		var packages []dependencies.MinimialVersionedPackageRepo
		for _, call := range indexEnqueuer.QueueIndexesForPackageFunc.History() {
			packages = append(packages, call.Arg1)
		}
		sort.Slice(packages, func(i, j int) bool {
			for _, pair := range [][2]string{
				{packages[i].Scheme, packages[j].Scheme},
				{string(packages[i].Name), string(packages[j].Name)},
				{packages[i].Version, packages[j].Version},
			} {
				if pair[0] < pair[1] {
					return true
				}
				if pair[1] < pair[0] {
					break
				}
			}

			return false
		})

		expectedPackages := []dependencies.MinimialVersionedPackageRepo{
			{Scheme: "gomod", Name: "https://github.com/cheese/burger", Version: "v2.2.1"},
			{Scheme: "gomod", Name: "https://github.com/cheese/burger", Version: "v3.2.2"},
			{Scheme: "gomod", Name: "https://github.com/cheese/burger", Version: "v4.2.3"},
			{Scheme: "gomod", Name: "https://github.com/sample/text", Version: "v1.2.0"},
			{Scheme: "gomod", Name: "https://github.com/sample/text", Version: "v2.2.0"},
			{Scheme: "gomod", Name: "https://github.com/sample/text", Version: "v3.2.0"},
		}
		if diff := cmp.Diff(expectedPackages, packages); diff != "" {
			t.Errorf("unexpected packages (-want +got):\n%s", diff)
		}
	}
}

func TestDependencyIndexingSchedulerHandlerRequeueNotCloned(t *testing.T) {
	mockUploadsSvc := NewMockUploadService()
	mockRepoStore := NewMockReposStore()
	mockExtSvcStore := NewMockExternalServiceStore()
	mockRepoUpdater := NewMockRepoUpdaterClient()
	mockScanner := NewMockPackageReferenceScanner()
	mockWorkerStore := NewMockWorkerStore[dependencyIndexingJob]()
	mockUploadsSvc.GetUploadByIDFunc.SetDefaultReturn(shared.Upload{ID: 42, RepositoryID: 50, Indexer: "lsif-go"}, true, nil)
	mockUploadsSvc.ReferencesForUploadFunc.SetDefaultReturn(mockScanner, nil)

	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/sample/text", Version: "v3.2.0"}}, true, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/cheese/burger", Version: "v4.2.3"}}, true, nil)
	mockScanner.NextFunc.SetDefaultReturn(shared.PackageReference{}, false, nil)

	mockGitserverReposStore := NewMockGitserverRepoStore()
	mockGitserverReposStore.GetByNamesFunc.PushReturn(map[api.RepoName]*types.GitserverRepo{
		"github.com/sample/text": {
			CloneStatus: types.CloneStatusCloned,
		},
		"github.com/cheese/burger": {
			CloneStatus: types.CloneStatusCloning,
		},
	}, nil)

	indexEnqueuer := NewMockIndexEnqueuer()

	dotcom.MockSourcegraphDotComMode(t, true)

	handler := &dependencyIndexingSchedulerHandler{
		uploadsSvc:         mockUploadsSvc,
		repoStore:          mockRepoStore,
		indexEnqueuer:      indexEnqueuer,
		extsvcStore:        mockExtSvcStore,
		gitserverRepoStore: mockGitserverReposStore,
		workerStore:        mockWorkerStore,
		repoUpdater:        mockRepoUpdater,
	}

	job := dependencyIndexingJob{
		UploadID:            42,
		ExternalServiceKind: "",
		ExternalServiceSync: time.Time{},
	}
	logger := logtest.Scoped(t)
	if err := handler.Handle(context.Background(), logger, job); err != nil {
		t.Fatalf("unexpected error performing update: %s", err)
	}

	if len(mockWorkerStore.RequeueFunc.History()) != 1 {
		t.Errorf("unexpected number of calls to Requeue. want=%d have=%d", 1, len(mockWorkerStore.RequeueFunc.History()))
	}

	if len(mockExtSvcStore.ListFunc.History()) != 0 {
		t.Errorf("unexpected number of calls to extsvcStore.List. want=%d have=%d", 0, len(mockExtSvcStore.ListFunc.History()))
	}

	if len(indexEnqueuer.QueueIndexesForPackageFunc.History()) != 0 {
		t.Errorf("unexpected number of calls to QueueIndexesForPackage. want=%d have=%d", 0, len(indexEnqueuer.QueueIndexesForPackageFunc.History()))
	}
}

func TestDependencyIndexingSchedulerHandlerSkipNonExistant(t *testing.T) {
	mockUploadsSvc := NewMockUploadService()
	mockExtSvcStore := NewMockExternalServiceStore()
	mockRepoUpdater := NewMockRepoUpdaterClient()
	mockScanner := NewMockPackageReferenceScanner()
	mockWorkerStore := NewMockWorkerStore[dependencyIndexingJob]()
	mockRepoStore := NewMockReposStore()

	mockUploadsSvc.GetUploadByIDFunc.SetDefaultReturn(shared.Upload{ID: 42, RepositoryID: 50, Indexer: "lsif-go"}, true, nil)
	mockUploadsSvc.ReferencesForUploadFunc.SetDefaultReturn(mockScanner, nil)

	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/sample/text", Version: "v3.2.0"}}, true, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{Package: shared.Package{UploadID: 42, Scheme: "gomod", Name: "https://github.com/cheese/burger", Version: "v4.2.3"}}, true, nil)
	mockScanner.NextFunc.SetDefaultReturn(shared.PackageReference{}, false, nil)

	mockGitserverReposStore := NewMockGitserverRepoStore()
	mockGitserverReposStore.GetByNamesFunc.PushReturn(map[api.RepoName]*types.GitserverRepo{
		"github.com/sample/text": {
			CloneStatus: types.CloneStatusCloned,
		},
		"github.com/cheese/burger": {
			CloneStatus: types.CloneStatusNotCloned,
		},
	}, nil)

	indexEnqueuer := NewMockIndexEnqueuer()

	dotcom.MockSourcegraphDotComMode(t, true)

	handler := &dependencyIndexingSchedulerHandler{
		uploadsSvc:         mockUploadsSvc,
		indexEnqueuer:      indexEnqueuer,
		extsvcStore:        mockExtSvcStore,
		gitserverRepoStore: mockGitserverReposStore,
		workerStore:        mockWorkerStore,
		repoUpdater:        mockRepoUpdater,
		repoStore:          mockRepoStore,
	}

	job := dependencyIndexingJob{
		UploadID:            42,
		ExternalServiceKind: "",
		ExternalServiceSync: time.Time{},
	}
	logger := logtest.Scoped(t)
	if err := handler.Handle(context.Background(), logger, job); err != nil {
		t.Fatalf("unexpected error performing update: %s", err)
	}

	if len(mockWorkerStore.RequeueFunc.History()) != 0 {
		t.Errorf("unexpected number of calls to Requeue. want=%d have=%d", 0, len(mockWorkerStore.RequeueFunc.History()))
	}

	if len(mockExtSvcStore.ListFunc.History()) != 0 {
		t.Errorf("unexpected number of calls to extsvcStore.List. want=%d have=%d", 0, len(mockExtSvcStore.ListFunc.History()))
	}

	if len(indexEnqueuer.QueueIndexesForPackageFunc.History()) != 1 {
		t.Errorf("unexpected number of calls to QueueIndexesForPackage. want=%d have=%d", 1, len(indexEnqueuer.QueueIndexesForPackageFunc.History()))
	}
}

func TestDependencyIndexingSchedulerHandlerShouldSkipRepository(t *testing.T) {
	mockUploadsSvc := NewMockUploadService()
	mockExtSvcStore := NewMockExternalServiceStore()
	mockGitserverReposStore := NewMockGitserverRepoStore()
	mockScanner := NewMockPackageReferenceScanner()
	mockRepoStore := NewMockReposStore()

	mockUploadsSvc.GetUploadByIDFunc.SetDefaultReturn(shared.Upload{ID: 42, RepositoryID: 51, Indexer: "scip-typescript"}, true, nil)
	mockUploadsSvc.ReferencesForUploadFunc.SetDefaultReturn(mockScanner, nil)

	indexEnqueuer := NewMockIndexEnqueuer()

	dotcom.MockSourcegraphDotComMode(t, true)

	handler := &dependencyIndexingSchedulerHandler{
		uploadsSvc:         mockUploadsSvc,
		indexEnqueuer:      indexEnqueuer,
		extsvcStore:        mockExtSvcStore,
		gitserverRepoStore: mockGitserverReposStore,
		repoStore:          mockRepoStore,
	}

	job := dependencyIndexingJob{
		ExternalServiceKind: "",
		ExternalServiceSync: time.Time{},
		UploadID:            42,
	}
	logger := logtest.Scoped(t)
	if err := handler.Handle(context.Background(), logger, job); err != nil {
		t.Fatalf("unexpected error performing update: %s", err)
	}

	if len(indexEnqueuer.QueueIndexesForPackageFunc.History()) != 0 {
		t.Errorf("unexpected number of calls to QueueIndexesForPackage. want=%d have=%d", 0, len(indexEnqueuer.QueueIndexesForPackageFunc.History()))
	}
}

func TestDependencyIndexingSchedulerHandlerNoExtsvc(t *testing.T) {
	mockUploadsSvc := NewMockUploadService()
	mockExtSvcStore := NewMockExternalServiceStore()
	mockGitserverReposStore := NewMockGitserverRepoStore()
	mockScanner := NewMockPackageReferenceScanner()
	mockRepoStore := NewMockReposStore()

	mockUploadsSvc.GetUploadByIDFunc.SetDefaultReturn(shared.Upload{ID: 42, RepositoryID: 51, Indexer: "scip-java"}, true, nil)
	mockUploadsSvc.ReferencesForUploadFunc.SetDefaultReturn(mockScanner, nil)
	mockScanner.NextFunc.PushReturn(shared.PackageReference{
		Package: shared.Package{
			UploadID: 42,
			Scheme:   dependencies.JVMPackagesScheme,
			Name:     "banana",
			Version:  "v1.2.3",
		},
	}, true, nil)
	mockGitserverReposStore.GetByNamesFunc.PushReturn(map[api.RepoName]*types.GitserverRepo{
		"banana": {CloneStatus: types.CloneStatusCloned},
	}, nil)

	indexEnqueuer := NewMockIndexEnqueuer()

	dotcom.MockSourcegraphDotComMode(t, true)

	handler := &dependencyIndexingSchedulerHandler{
		uploadsSvc:         mockUploadsSvc,
		indexEnqueuer:      indexEnqueuer,
		extsvcStore:        mockExtSvcStore,
		gitserverRepoStore: mockGitserverReposStore,
		repoStore:          mockRepoStore,
	}

	job := dependencyIndexingJob{
		ExternalServiceKind: extsvc.KindJVMPackages,
		ExternalServiceSync: time.Time{},
		UploadID:            42,
	}
	logger := logtest.Scoped(t)
	if err := handler.Handle(context.Background(), logger, job); err != nil {
		t.Fatalf("unexpected error performing update: %s", err)
	}

	if len(indexEnqueuer.QueueIndexesForPackageFunc.History()) != 0 {
		t.Errorf("unexpected number of calls to QueueIndexesForPackage. want=%d have=%d", 0, len(indexEnqueuer.QueueIndexesForPackageFunc.History()))
	}
}
