package resolvers

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"sort"
	"testing"
	"unicode/utf8"

	"github.com/sourcegraph/log/logtest"
	"github.com/sourcegraph/zoekt"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/pointer"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/codycontext"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	rtypes "github.com/sourcegraph/sourcegraph/internal/rbac/types"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/backend"
	"github.com/sourcegraph/sourcegraph/internal/search/client"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestCodyIgnore(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(t))
	repo1 := types.Repo{Name: "repo1"}
	repo2 := types.Repo{Name: "repo2"}
	truePtr := true
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			CodyEnabled: &truePtr,
			LicenseKey:  "asdf",
			ExperimentalFeatures: &schema.ExperimentalFeatures{
				CodyContextIgnore: pointers.Ptr(true),
			},
			CodyContextFilters: &schema.CodyContextFilters{
				Exclude: []*schema.CodyContextFilterItem{
					{RepoNamePattern: "^repo2$"},
				},
			},
		},
	})
	t.Cleanup(func() { conf.Mock(nil) })

	oldMock := licensing.MockCheckFeature
	defer func() {
		licensing.MockCheckFeature = oldMock
	}()

	licensing.MockCheckFeature = func(feature licensing.Feature) error {
		if feature == licensing.FeatureCody {
			return nil
		}
		return errors.New("error")
	}

	ctx = authenticateUser(t, db, ctx)

	// Create populates the IDs in the passed in types.Repo
	err := db.Repos().Create(ctx, &repo1, &repo2)
	require.NoError(t, err)

	files := map[api.RepoName]map[string][]byte{
		"repo1": {
			"testcode1.go":  []byte("testcode1"),
			"ignore_me1.go": []byte("secret"),
			".cody/ignore":  []byte("ignore_me1.go"),
		},
		"repo2": {
			"testcode2.go":  []byte("testcode2"),
			"ignore_me2.go": []byte("secret"),
			".cody/ignore":  []byte("ignore_me2.go"),
		},
	}

	mockGitserver := gitserver.NewMockClient()
	mockGitserver.GetDefaultBranchFunc.SetDefaultReturn("main", "abc123", nil)
	mockGitserver.StatFunc.SetDefaultHook(func(_ context.Context, repo api.RepoName, _ api.CommitID, fileName string) (fs.FileInfo, error) {
		return fakeFileInfo{path: fileName}, nil
	})
	mockGitserver.NewFileReaderFunc.SetDefaultHook(func(ctx context.Context, repo api.RepoName, ci api.CommitID, fileName string) (io.ReadCloser, error) {
		if content, ok := files[repo][fileName]; ok {
			return io.NopCloser(bytes.NewReader(content)), nil
		}
		return nil, os.ErrNotExist
	})

	lineRange := func(start, end uint32) []zoekt.ChunkMatch {
		return []zoekt.ChunkMatch{{
			Ranges: []zoekt.Range{{
				Start: zoekt.Location{LineNumber: start},
				End:   zoekt.Location{LineNumber: end},
			}},
		}}
	}
	mockZoektStreamer := &backend.FakeStreamer{
		Results: []*zoekt.SearchResult{{
			Files: []zoekt.FileMatch{{
				FileName:     "ignore_me1.go",
				RepositoryID: uint32(repo1.ID),
				ChunkMatches: lineRange(0, 4),
			}, {
				FileName:     "ignore_me2.go",
				RepositoryID: uint32(repo2.ID),
				ChunkMatches: lineRange(0, 4),
			}, {
				FileName:     "testcode1.go",
				RepositoryID: uint32(repo1.ID),
				ChunkMatches: lineRange(0, 4),
			}, {
				FileName:     "testcode2.go",
				RepositoryID: uint32(repo2.ID),
				ChunkMatches: lineRange(0, 4),
			}},
		}},
		Repos: []*zoekt.RepoListEntry{{
			Repository: zoekt.Repository{
				ID:       uint32(repo1.ID),
				Name:     string(repo1.Name),
				Branches: []zoekt.RepositoryBranch{{Name: "HEAD"}},
			}}, {
			Repository: zoekt.Repository{
				ID:       uint32(repo2.ID),
				Name:     string(repo2.Name),
				Branches: []zoekt.RepositoryBranch{{Name: "HEAD"}},
			}},
		},
	}

	searchClient := client.Mocked(job.RuntimeClients{
		Logger: logger,
		DB:     db,
		Zoekt:  mockZoektStreamer,
	})

	tests := []struct {
		name       string
		dotComMode bool
		want       []string
	}{
		{
			name:       "dotcom mode",
			dotComMode: true,
			// .cody/ignore files are respected in dotcom mode
			// Cody context filters in site config are not applied
			// Results are duplicated because the Cody search job calls the Zoekt streamer twice and appends the results
			want: []string{"testcode1.go", "testcode1.go", "testcode2.go", "testcode2.go"},
		},
		{
			name:       "enterprise mode",
			dotComMode: false,
			// "repo2" results are excluded according to the site config
			// .cody/ignore files don't have any effect for enterprise
			// Each result is duplicated because the Cody search job calls the Zoekt streamer twice and appends the results
			want: []string{"testcode1.go", "testcode1.go", "ignore_me1.go", "ignore_me1.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO (taras-yemets): remove feature flag mocking after `CodyContextFilters` support is added to the IDE clients (see: https://github.com/sourcegraph/sourcegraph/pull/62231)
			t.Cleanup(func() {
				db.FeatureFlags().DeleteFeatureFlag(ctx, "cody-context-filters-enabled")
			})
			if !tt.dotComMode {
				db.FeatureFlags().CreateBool(ctx, "cody-context-filters-enabled", true)
			}

			dotcom.MockSourcegraphDotComMode(t, tt.dotComMode)
			observationCtx := observation.TestContextTB(t)
			contextClient := codycontext.NewCodyContextClient(
				observationCtx,
				db,
				searchClient,
				mockGitserver,
			)

			resolver := NewResolver(db, mockGitserver, contextClient, nil)

			results, err := resolver.GetCodyContext(ctx, graphqlbackend.GetContextArgs{
				Repos:            graphqlbackend.MarshalRepositoryIDs([]api.RepoID{1, 2}),
				Query:            "my test query",
				TextResultsCount: 2,
				CodeResultsCount: 2,
			})
			require.NoError(t, err)

			paths := make([]string, len(results))
			for i, r := range results {
				paths[i] = r.(*graphqlbackend.FileChunkContextResolver).Blob().Path()
			}
			expected := tt.want
			sort.Strings(expected)
			sort.Strings(paths)
			require.Equal(t, expected, paths)
		})
	}
}

func authenticateUser(t *testing.T, db database.DB, ctx context.Context) context.Context {
	// Create a normal user role with Cody access permission
	normalUserRole, err := db.Roles().Create(ctx, "normal user role", false)
	require.NoError(t, err)
	codyAccessPermission, err := db.Permissions().Create(ctx, database.CreatePermissionOpts{
		Namespace: rtypes.CodyNamespace,
		Action:    rtypes.CodyAccessAction,
	})
	require.NoError(t, err)
	err = db.RolePermissions().Assign(ctx, database.AssignRolePermissionOpts{
		PermissionID: codyAccessPermission.ID,
		RoleID:       normalUserRole.ID,
	})
	require.NoError(t, err)

	// Create an admin user, give them the normal user role, and authenticate our actor.
	newAdminUser, err := db.Users().Create(ctx, database.NewUser{
		Email:                 "test@example.com",
		Username:              "test",
		DisplayName:           "Test User",
		Password:              "hunter123",
		EmailIsVerified:       true,
		FailIfNotInitialUser:  true, // initial site admin account
		EnforcePasswordLength: false,
		TosAccepted:           true,
	})
	require.NoError(t, err)
	db.UserRoles().SetRolesForUser(ctx, database.SetRolesForUserOpts{
		UserID: newAdminUser.ID,
		Roles:  []int32{normalUserRole.ID},
	})
	require.NoError(t, err)
	ctx = actor.WithActor(ctx, actor.FromMockUser(newAdminUser.ID))
	return ctx
}

type fakeFileInfo struct {
	path string
	fs.FileInfo
}

func (f fakeFileInfo) Name() string {
	return f.path
}

func TestChunkSize(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(t))
	repo := types.Repo{Name: "repo"}

	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			CodyEnabled: pointer.Bool(true),
			LicenseKey:  "asdf",
		},
	})
	t.Cleanup(func() { conf.Mock(nil) })

	oldMock := licensing.MockCheckFeature
	defer func() {
		licensing.MockCheckFeature = oldMock
	}()

	licensing.MockCheckFeature = func(feature licensing.Feature) error {
		if feature == licensing.FeatureCody {
			return nil
		}
		return errors.New("error")
	}

	ctx = authenticateUser(t, db, ctx)

	// Create populates the IDs in the passed in types.Repo
	err := db.Repos().Create(ctx, &repo)
	require.NoError(t, err)

	var content string
	for i := 0; i < chunkSizeRunes; i++ {
		if i != 0 && i%10 == 0 {
			content += "\n"
		} else {
			content += "a"
		}
	}

	wantLines := bytes.Count([]byte(content), []byte("\n"))

	files := map[api.RepoName]map[string][]byte{
		"repo": {
			"testcode1.go": []byte(content),
			"testcode2.go": []byte(content + "\n"),
			"testcode3.go": []byte(content + "extra info"),
		},
	}

	mockGitserver := gitserver.NewMockClient()
	mockGitserver.GetDefaultBranchFunc.SetDefaultReturn("main", "abc123", nil)
	mockGitserver.StatFunc.SetDefaultHook(func(_ context.Context, repo api.RepoName, _ api.CommitID, fileName string) (fs.FileInfo, error) {
		return fakeFileInfo{path: fileName}, nil
	})
	mockGitserver.NewFileReaderFunc.SetDefaultHook(func(ctx context.Context, repo api.RepoName, ci api.CommitID, fileName string) (io.ReadCloser, error) {
		if content, ok := files[repo][fileName]; ok {
			return io.NopCloser(bytes.NewReader(content)), nil
		}
		return nil, os.ErrNotExist
	})

	lineRange := result.ChunkMatches{{
		Ranges: result.Ranges{{
			Start: result.Location{Line: 2},
			End:   result.Location{Line: 6},
		}},
	}}

	mockSearchClient := client.NewMockSearchClient()
	mockSearchClient.PlanFunc.SetDefaultHook(func(_ context.Context, _ string, _ *string, query string, _ search.Mode, _ search.Protocol, _ *int32) (*search.Inputs, error) {
		return &search.Inputs{OriginalQuery: query}, nil
	})
	mockSearchClient.ExecuteFunc.SetDefaultHook(func(_ context.Context, stream streaming.Sender, inputs *search.Inputs) (*search.Alert, error) {
		stream.Send(streaming.SearchEvent{
			Results: result.Matches{&result.FileMatch{
				File: result.File{
					Path: "testcode1.go",
					Repo: types.MinimalRepo{ID: repo.ID, Name: repo.Name},
				},
				ChunkMatches: lineRange,
			}, &result.FileMatch{
				File: result.File{
					Path: "testcode2.go",
					Repo: types.MinimalRepo{ID: repo.ID, Name: repo.Name},
				},
				ChunkMatches: lineRange,
			}, &result.FileMatch{
				File: result.File{
					Path: "testcode3.go",
					Repo: types.MinimalRepo{ID: repo.ID, Name: repo.Name},
				},
				ChunkMatches: lineRange,
			}}})
		return nil, nil
	})

	observationCtx := observation.TestContextTB(t)
	contextClient := codycontext.NewCodyContextClient(
		observationCtx,
		db,
		mockSearchClient,
		mockGitserver,
	)

	resolver := NewResolver(db, mockGitserver, contextClient, nil)

	results, err := resolver.GetCodyContext(ctx, graphqlbackend.GetContextArgs{
		Repos:            graphqlbackend.MarshalRepositoryIDs([]api.RepoID{1}),
		Query:            "my test query",
		TextResultsCount: 0,
		CodeResultsCount: 5,
	})
	require.NoError(t, err)
	require.Equal(t, 3, len(results), "expected 3 results but got %d", len(results))

	for _, r := range results {
		f := r.(*graphqlbackend.FileChunkContextResolver)
		gotLines := int(f.EndLine() - f.StartLine() + 1)
		require.Equal(t, wantLines, gotLines, "expected %d lines but got %d", wantLines, gotLines)

		c, err := f.ChunkContent(ctx)
		require.NoError(t, err)
		require.LessOrEqual(t, utf8.RuneCountInString(c), chunkSizeRunes)
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		numRunes int
		want     int
	}{
		{
			name:     "empty string",
			content:  "",
			numRunes: 1024,
			want:     0,
		}, {
			name:     "single line",
			content:  "Hello world\n",
			numRunes: 1024,
			want:     1,
		}, {}, {
			name:     "single line, no trailing newline",
			content:  "Hello world",
			numRunes: 1024,
			want:     1,
		}, {
			name:     "should truncate",
			content:  "Hello\nWorld\nGo is awesome\n",
			numRunes: 24,
			want:     2,
		}, {
			name:     "should truncate, no trailing newline",
			content:  "Hello\nWorld\nGo is awesome",
			numRunes: 24,
			want:     2,
		}, {
			name:     "all lines",
			content:  "Hello\nWorld\nGo is awesome\n",
			numRunes: 1024,
			want:     3,
		}, {
			name:     "all lines, no trailing newline",
			content:  "Hello\nWorld\nGo is awesome",
			numRunes: 1024,
			want:     3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countLines(tt.content, tt.numRunes); got != tt.want {
				t.Errorf("countLines() = %v, want %v", got, tt.want)
			}
		})
	}
}
