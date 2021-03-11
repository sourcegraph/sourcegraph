package repos

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestStatusMessages(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	db := dbtest.NewDB(t, "")
	store := NewStore(db, sql.TxOptions{})

	admin, err := database.Users(db).Create(ctx, database.NewUser{
		Email:                 "a1@example.com",
		Username:              "a1",
		Password:              "p",
		EmailVerificationCode: "c",
	})
	if err != nil {
		t.Fatal(err)
	}
	nonAdmin, err := database.Users(db).Create(ctx, database.NewUser{
		Email:                 "u1@example.com",
		Username:              "u1",
		Password:              "p",
		EmailVerificationCode: "c",
	})
	if err != nil {
		t.Fatal(err)
	}

	siteLevelService := &types.ExternalService{
		ID:          1,
		Config:      `{}`,
		Kind:        extsvc.KindGitHub,
		DisplayName: "github.com - site",
	}
	err = database.ExternalServices(db).Upsert(ctx, siteLevelService)
	if err != nil {
		t.Fatal(err)
	}
	userService := &types.ExternalService{
		ID:              2,
		Config:          `{}`,
		Kind:            extsvc.KindGitHub,
		DisplayName:     "github.com - user",
		NamespaceUserID: nonAdmin.ID,
	}
	err = database.ExternalServices(db).Upsert(ctx, userService)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name            string
		stored          types.Repos
		gitserverCloned []string
		sourcerErr      error
		listRepoErr     error
		res             []StatusMessage
		user            *types.User
		// maps repoName to external service id
		repoOwner map[api.RepoName]int64
		cloud     bool
		err       string
	}{
		{
			name:            "all cloned",
			gitserverCloned: []string{"foobar"},
			stored:          []*types.Repo{{Name: "foobar", Cloned: true}},
			user:            admin,
			res:             nil,
		},
		{
			name:            "nothing cloned",
			stored:          []*types.Repo{{Name: "foobar"}},
			user:            admin,
			gitserverCloned: []string{},
			res: []StatusMessage{
				{
					Cloning: &CloningProgress{
						Message: "1 repositories enqueued for cloning...",
					},
				},
			},
		},
		{
			// We don't show uncloned count in Cloud as it is misleading
			name:            "nothing cloned cloud",
			stored:          []*types.Repo{{Name: "foobar"}},
			user:            admin,
			gitserverCloned: []string{},
			res:             nil,
			cloud:           true,
		},
		{
			name:            "subset cloned",
			stored:          []*types.Repo{{Name: "foobar", Cloned: true}, {Name: "barfoo"}},
			user:            admin,
			gitserverCloned: []string{"foobar"},
			res: []StatusMessage{
				{
					Cloning: &CloningProgress{
						Message: "1 repositories enqueued for cloning...",
					},
				},
			},
		},
		{
			name:   "non admin users should only count their own non cloned repos",
			stored: []*types.Repo{{Name: "foobar"}, {Name: "barfoo"}},
			repoOwner: map[api.RepoName]int64{
				"foobar": userService.ID,
			},
			user: nonAdmin,
			res: []StatusMessage{
				{
					Cloning: &CloningProgress{
						Message: "1 repositories enqueued for cloning...",
					},
				},
			},
		},
		{
			name:            "more cloned than stored",
			stored:          []*types.Repo{{Name: "foobar", Cloned: true}},
			user:            admin,
			gitserverCloned: []string{"foobar", "barfoo"},
			res:             nil,
		},
		{
			name:            "cloned different than stored",
			stored:          []*types.Repo{{Name: "foobar"}, {Name: "barfoo"}},
			user:            admin,
			gitserverCloned: []string{"one", "two", "three"},
			res: []StatusMessage{
				{
					Cloning: &CloningProgress{
						Message: "2 repositories enqueued for cloning...",
					},
				},
			},
		},
		{
			name:            "case insensitivity",
			gitserverCloned: []string{"foobar"},
			stored:          []*types.Repo{{Name: "FOOBar", Cloned: true}},
			user:            admin,
			res:             nil,
		},
		{
			name:            "case insensitivity to gitserver names",
			gitserverCloned: []string{"FOOBar"},
			stored:          []*types.Repo{{Name: "FOOBar", Cloned: true}},
			user:            admin,
			res:             nil,
		},
		{
			name:       "one external service syncer err",
			user:       admin,
			sourcerErr: errors.New("github is down"),
			res: []StatusMessage{
				{
					ExternalServiceSyncError: &ExternalServiceSyncError{
						Message:           "fetching from code host github.com - site: 1 error occurred:\n\t* github is down\n\n",
						ExternalServiceId: siteLevelService.ID,
					},
				},
			},
		},
		{
			name:        "one syncer err",
			user:        admin,
			listRepoErr: errors.New("could not connect to database"),
			res: []StatusMessage{
				{
					ExternalServiceSyncError: &ExternalServiceSyncError{
						Message:           "syncer.sync.store.list-repos: could not connect to database",
						ExternalServiceId: siteLevelService.ID,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		ctx := context.Background()

		t.Run(tc.name, func(t *testing.T) {
			stored := tc.stored.Clone()
			var cloned []string
			for _, r := range stored {
				r.ExternalRepo = api.ExternalRepoSpec{
					ID:          uuid.New().String(),
					ServiceType: extsvc.TypeGitHub,
					ServiceID:   "https://github.com/",
				}
				if r.Cloned {
					cloned = append(cloned, string(r.Name))
				}
			}

			err := database.Repos(db).Create(ctx, stored...)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				ids := make([]api.RepoID, 0, len(stored))
				for _, r := range stored {
					ids = append(ids, r.ID)
				}
				err := database.Repos(db).Delete(ctx, ids...)
				if err != nil {
					t.Fatal(err)
				}
			})

			// Set up ownership of repos
			if tc.repoOwner != nil {
				for _, repo := range stored {
					svcID, ok := tc.repoOwner[repo.Name]
					if !ok {
						continue
					}
					_, err = db.ExecContext(ctx, `INSERT INTO external_service_repos VALUES ($1, $2, 'example.com')`, svcID, repo.ID)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						_, err = db.ExecContext(ctx, `DELETE FROM external_service_repos WHERE external_service_id = $1`, svcID)
						if err != nil {
							t.Fatal(err)
						}
					})
				}
			}

			err = store.SetClonedRepos(ctx, cloned...)
			if err != nil {
				t.Fatal(err)
			}

			clock := timeutil.NewFakeClock(time.Now(), 0)
			syncer := &Syncer{
				Store: store,
				Now:   clock.Now,
			}

			if tc.sourcerErr != nil || tc.listRepoErr != nil {
				database.Mocks.Repos.List = func(v0 context.Context, v1 database.ReposListOptions) ([]*types.Repo, error) {
					return nil, tc.listRepoErr
				}
				defer func() {
					database.Mocks.Repos.List = nil
				}()
				sourcer := NewFakeSourcer(tc.sourcerErr, NewFakeSource(siteLevelService, nil))
				syncer.Sourcer = sourcer

				err = syncer.SyncExternalService(ctx, store, siteLevelService.ID, time.Millisecond)
				// In prod, SyncExternalService is kicked off by a worker queue. Any error
				// returned will be stored in the external_service_sync_jobs table so we fake
				// that here.
				if err != nil {
					defer func() { database.Mocks.ExternalServices = database.MockExternalServices{} }()
					database.Mocks.ExternalServices.ListSyncErrors = func(ctx context.Context) (map[int64]string, error) {
						return map[int64]string{
							siteLevelService.ID: err.Error(),
						}, nil
					}
				}
			}

			if tc.err == "" {
				tc.err = "<nil>"
			}

			res, err := FetchStatusMessages(ctx, db, tc.user, tc.cloud)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("have err: %q, want: %q", have, want)
			}

			if diff := cmp.Diff(tc.res, res); diff != "" {
				t.Error(diff)
			}
		})
	}
}
