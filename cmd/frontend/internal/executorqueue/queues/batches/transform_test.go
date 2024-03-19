package batches

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	mockassert "github.com/derision-test/go-mockgen/v2/testutil/assert"
	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/api"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	apiclient "github.com/sourcegraph/sourcegraph/internal/executor/types"
	"github.com/sourcegraph/sourcegraph/internal/types"
	batcheslib "github.com/sourcegraph/sourcegraph/lib/batches"
	"github.com/sourcegraph/sourcegraph/lib/batches/execution"
	"github.com/sourcegraph/sourcegraph/lib/batches/template"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestTransformRecord(t *testing.T) {
	db := dbmocks.NewMockDB()
	repos := dbmocks.NewMockRepoStore()
	repos.GetFunc.SetDefaultHook(func(ctx context.Context, id api.RepoID) (*types.Repo, error) {
		return &types.Repo{ID: id, Name: "github.com/sourcegraph/sourcegraph"}, nil
	})
	db.ReposFunc.SetDefaultReturn(repos)

	conf.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{ExternalURL: "https://test.io"}})
	t.Cleanup(func() {
		conf.Mock(nil)
	})

	secs := dbmocks.NewMockExecutorSecretStore()
	secs.ListFunc.SetDefaultHook(func(ctx context.Context, ess database.ExecutorSecretScope, eslo database.ExecutorSecretsListOpts) ([]*database.ExecutorSecret, int, error) {
		if len(eslo.Keys) == 1 && eslo.Keys[0] == "DOCKER_AUTH_CONFIG" {
			return nil, 0, nil
		}
		return []*database.ExecutorSecret{
			database.NewMockExecutorSecret(&database.ExecutorSecret{
				Key:       "FOO",
				Scope:     database.ExecutorSecretScopeBatches,
				CreatorID: 1,
			}, "bar"),
		}, 0, nil
	})
	db.ExecutorSecretsFunc.SetDefaultReturn(secs)

	sal := dbmocks.NewMockExecutorSecretAccessLogStore()
	db.ExecutorSecretAccessLogsFunc.SetDefaultReturn(sal)

	spec := batcheslib.BatchSpec{}
	err := yaml.Unmarshal([]byte(`
steps:
  - run: echo lol >> readme.md
    container: alpine:3
    env:
      - FOO
  - run: echo more lol >> readme.md
    container: alpine:3
`), &spec)
	if err != nil {
		t.Fatal(err)
	}

	batchSpec := &btypes.BatchSpec{
		RandID:          "abc",
		UserID:          123,
		NamespaceUserID: 123,
		RawSpec:         "horse",
		Spec:            &spec,
	}

	workspace := &btypes.BatchSpecWorkspace{
		BatchSpecID:        batchSpec.ID,
		ChangesetSpecIDs:   []int64{},
		RepoID:             5678,
		Branch:             "refs/heads/base-branch",
		Commit:             "d34db33f",
		Path:               "a/b/c",
		FileMatches:        []string{"a/b/c/foobar.go"},
		OnlyFetchWorkspace: true,
		StepCacheResults: map[int]btypes.StepCacheResult{
			1: {
				Key: "testcachekey",
				Value: &execution.AfterStepResult{
					Diff: []byte("123"),
				},
			},
		},
	}

	workspaceExecutionJob := &btypes.BatchSpecWorkspaceExecutionJob{
		ID:                   42,
		BatchSpecWorkspaceID: workspace.ID,
		UserID:               123,
	}

	store := NewMockBatchesStore()
	store.GetBatchSpecFunc.SetDefaultReturn(batchSpec, nil)
	store.GetBatchSpecWorkspaceFunc.SetDefaultReturn(workspace, nil)
	store.DatabaseDBFunc.SetDefaultReturn(db)

	wantInput := func(cachedStepResultFound bool, cachedStepResult execution.AfterStepResult) batcheslib.WorkspacesExecutionInput {
		return batcheslib.WorkspacesExecutionInput{
			BatchChangeAttributes: template.BatchChangeAttributes{
				Name:        batchSpec.Spec.Name,
				Description: batchSpec.Spec.Description,
			},
			Repository: batcheslib.WorkspaceRepo{
				ID:   string(graphqlbackend.MarshalRepositoryID(workspace.RepoID)),
				Name: "github.com/sourcegraph/sourcegraph",
			},
			Branch: batcheslib.WorkspaceBranch{
				Name:   workspace.Branch,
				Target: batcheslib.Commit{OID: workspace.Commit},
			},
			Path:                  workspace.Path,
			OnlyFetchWorkspace:    workspace.OnlyFetchWorkspace,
			Steps:                 batchSpec.Spec.Steps,
			SearchResultPaths:     workspace.FileMatches,
			CachedStepResultFound: cachedStepResultFound,
			CachedStepResult:      cachedStepResult,
			SkippedSteps:          make(map[int]struct{}),
		}
	}

	t.Run("with cache entry", func(t *testing.T) {
		job, err := transformRecord(context.Background(), logtest.Scoped(t), store, workspaceExecutionJob, "0.0.0-dev")
		if err != nil {
			t.Fatalf("unexpected error transforming record: %s", err)
		}

		marshaledInput, err := json.Marshal(wantInput(true, execution.AfterStepResult{Diff: []byte("123")}))
		if err != nil {
			t.Fatal(err)
		}

		expected := apiclient.Job{
			ID:                  int(workspaceExecutionJob.ID),
			RepositoryName:      "github.com/sourcegraph/sourcegraph",
			RepositoryDirectory: "repository",
			Commit:              workspace.Commit,
			ShallowClone:        true,
			SparseCheckout:      []string{"a/b/c/*"},
			VirtualMachineFiles: map[string]apiclient.VirtualMachineFile{
				"input.json": {Content: marshaledInput},
			},
			CliSteps: []apiclient.CliStep{
				{
					Key: "batch-exec",
					Commands: []string{
						"batch",
						"exec",
						"-f",
						"input.json",
						"-repo",
						"repository",
						"-tmp",
						".src-tmp",
					},
					Dir: ".",
					Env: []string{
						"FOO=bar",
					},
				},
			},
			RedactedValues: map[string]string{
				"bar": "${{ secrets.FOO }}",
			},
		}
		if diff := cmp.Diff(expected, job); diff != "" {
			t.Errorf("unexpected job (-want +got):\n%s", diff)
		}

		mockassert.CalledN(t, secs.ListFunc, 2)
		mockassert.CalledOnce(t, sal.CreateFunc)
	})

	t.Run("with cache disabled", func(t *testing.T) {
		// Copy.
		workspace := *workspace
		workspace.CachedResultFound = false
		workspace.StepCacheResults = map[int]btypes.StepCacheResult{}
		workspace.ChangesetSpecIDs = []int64{}
		store.GetBatchSpecWorkspaceFunc.PushReturn(&workspace, nil)

		job, err := transformRecord(context.Background(), logtest.Scoped(t), store, workspaceExecutionJob, "0.0.0-dev")
		if err != nil {
			t.Fatalf("unexpected error transforming record: %s", err)
		}

		marshaledInput, err := json.Marshal(wantInput(false, execution.AfterStepResult{}))
		if err != nil {
			t.Fatal(err)
		}

		expected := apiclient.Job{
			ID:                  int(workspaceExecutionJob.ID),
			RepositoryName:      "github.com/sourcegraph/sourcegraph",
			RepositoryDirectory: "repository",
			Commit:              workspace.Commit,
			ShallowClone:        true,
			SparseCheckout:      []string{"a/b/c/*"},
			VirtualMachineFiles: map[string]apiclient.VirtualMachineFile{
				"input.json": {Content: marshaledInput},
			},
			CliSteps: []apiclient.CliStep{
				{
					Key: "batch-exec",
					Commands: []string{
						"batch",
						"exec",
						"-f",
						"input.json",
						"-repo",
						"repository",
						"-tmp",
						".src-tmp",
					},
					Dir: ".",
					Env: []string{
						"FOO=bar",
					},
				},
			},
			RedactedValues: map[string]string{
				"bar": "${{ secrets.FOO }}",
			},
		}
		if diff := cmp.Diff(expected, job); diff != "" {
			t.Errorf("unexpected job (-want +got):\n%s", diff)
		}

		mockassert.CalledN(t, secs.ListFunc, 4)
		mockassert.CalledN(t, sal.CreateFunc, 2)
	})

	t.Run("with docker auth config", func(t *testing.T) {
		// Copy.
		workspace := *workspace
		workspace.CachedResultFound = false
		workspace.StepCacheResults = map[int]btypes.StepCacheResult{}
		workspace.ChangesetSpecIDs = []int64{}
		store.GetBatchSpecWorkspaceFunc.PushReturn(&workspace, nil)

		secs.ListFunc.PushReturn(secs.List(context.Background(), database.ExecutorSecretScopeBatches, database.ExecutorSecretsListOpts{}))
		secs.ListFunc.PushReturn(
			[]*database.ExecutorSecret{
				database.NewMockExecutorSecret(&database.ExecutorSecret{
					Key:       "DOCKER_AUTH_CONFIG",
					Scope:     database.ExecutorSecretScopeBatches,
					CreatorID: 1,
				}, `{"auths": { "hub.docker.com": { "auth": "aHVudGVyOmh1bnRlcjI=" }}}`),
			},
			0,
			nil,
		)

		job, err := transformRecord(context.Background(), logtest.Scoped(t), store, workspaceExecutionJob, "0.0.0-dev")
		if err != nil {
			t.Fatalf("unexpected error transforming record: %s", err)
		}

		marshaledInput, err := json.Marshal(wantInput(false, execution.AfterStepResult{}))
		if err != nil {
			t.Fatal(err)
		}

		expected := apiclient.Job{
			ID:                  int(workspaceExecutionJob.ID),
			RepositoryName:      "github.com/sourcegraph/sourcegraph",
			RepositoryDirectory: "repository",
			Commit:              workspace.Commit,
			ShallowClone:        true,
			SparseCheckout:      []string{"a/b/c/*"},
			VirtualMachineFiles: map[string]apiclient.VirtualMachineFile{
				"input.json": {Content: marshaledInput},
			},
			CliSteps: []apiclient.CliStep{
				{
					Key: "batch-exec",
					Commands: []string{
						"batch",
						"exec",
						"-f",
						"input.json",
						"-repo",
						"repository",
						"-tmp",
						".src-tmp",
					},
					Dir: ".",
					Env: []string{
						"FOO=bar",
					},
				},
			},
			RedactedValues: map[string]string{
				"bar": "${{ secrets.FOO }}",
			},
			DockerAuthConfig: apiclient.DockerAuthConfig{
				Auths: apiclient.DockerAuthConfigAuths{
					"hub.docker.com": apiclient.DockerAuthConfigAuth{
						Auth: []byte("hunter:hunter2"),
					},
				},
			},
		}
		if diff := cmp.Diff(expected, job); diff != "" {
			t.Errorf("unexpected job (-want +got):\n%s", diff)
		}

		mockassert.CalledN(t, secs.ListFunc, 7)
		mockassert.CalledN(t, sal.CreateFunc, 4)
	})

	t.Run("workspace file", func(t *testing.T) {
		t.Cleanup(func() {
			store.ListBatchSpecWorkspaceFilesFunc.SetDefaultReturn(nil, 0, nil)
		})

		workspaceFileModifiedAt := time.Now()
		store.ListBatchSpecWorkspaceFilesFunc.SetDefaultReturn(
			[]*btypes.BatchSpecWorkspaceFile{
				{
					RandID:     "xyz",
					FileName:   "script.sh",
					Path:       "foo/bar",
					Size:       12,
					ModifiedAt: workspaceFileModifiedAt,
				},
			},
			0,
			nil,
		)

		job, err := transformRecord(context.Background(), logtest.Scoped(t), store, workspaceExecutionJob, "0.0.0-dev")
		if err != nil {
			t.Fatalf("unexpected error transforming record: %s", err)
		}

		marshaledInput, err := json.Marshal(wantInput(true, execution.AfterStepResult{Diff: []byte("123")}))
		if err != nil {
			t.Fatal(err)
		}

		expected := apiclient.Job{
			ID:                  int(workspaceExecutionJob.ID),
			RepositoryName:      "github.com/sourcegraph/sourcegraph",
			RepositoryDirectory: "repository",
			Commit:              workspace.Commit,
			ShallowClone:        true,
			SparseCheckout:      []string{"a/b/c/*"},
			VirtualMachineFiles: map[string]apiclient.VirtualMachineFile{
				"input.json":                        {Content: marshaledInput},
				"workspace-files/foo/bar/script.sh": {Bucket: "batch-changes", Key: "abc/xyz", ModifiedAt: workspaceFileModifiedAt},
			},
			CliSteps: []apiclient.CliStep{
				{
					Key: "batch-exec",
					Commands: []string{
						"batch",
						"exec",
						"-f",
						"input.json",
						"-repo",
						"repository",
						"-tmp",
						".src-tmp",
						"-workspaceFiles",
						"workspace-files",
					},
					Dir: ".",
					Env: []string{
						"FOO=bar",
					},
				},
			},
			RedactedValues: map[string]string{
				"bar": "${{ secrets.FOO }}",
			},
		}
		if diff := cmp.Diff(expected, job); diff != "" {
			t.Errorf("unexpected job (-want +got):\n%s", diff)
		}

		mockassert.CalledN(t, secs.ListFunc, 9)
		mockassert.CalledN(t, sal.CreateFunc, 5)
	})
}
