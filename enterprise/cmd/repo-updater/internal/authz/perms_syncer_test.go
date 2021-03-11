package authz

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	edb "github.com/sourcegraph/sourcegraph/enterprise/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestPermsSyncer_ScheduleUsers(t *testing.T) {
	authz.SetProviders(true, []authz.Provider{&mockProvider{}})
	defer authz.SetProviders(true, nil)

	s := NewPermsSyncer(nil, nil, nil, nil)
	s.ScheduleUsers(context.Background(), 1)

	expHeap := []*syncRequest{
		{requestMeta: &requestMeta{
			Priority: priorityHigh,
			Type:     requestTypeUser,
			ID:       1,
		}, acquired: false, index: 0},
	}
	if diff := cmp.Diff(expHeap, s.queue.heap, cmpOpts); diff != "" {
		t.Fatalf("heap: %v", diff)
	}
}

func TestPermsSyncer_ScheduleRepos(t *testing.T) {
	authz.SetProviders(true, []authz.Provider{&mockProvider{}})
	defer authz.SetProviders(true, nil)

	s := NewPermsSyncer(nil, nil, nil, nil)
	s.ScheduleRepos(context.Background(), 1)

	expHeap := []*syncRequest{
		{requestMeta: &requestMeta{
			Priority: priorityHigh,
			Type:     requestTypeRepo,
			ID:       1,
		}, acquired: false, index: 0},
	}
	if diff := cmp.Diff(expHeap, s.queue.heap, cmpOpts); diff != "" {
		t.Fatalf("heap: %v", diff)
	}
}

type mockProvider struct {
	id          int64
	serviceType string
	serviceID   string

	fetchUserPerms func(context.Context, *extsvc.Account) (*authz.ExternalUserPermissions, error)
	fetchRepoPerms func(ctx context.Context, repo *extsvc.Repository) ([]extsvc.AccountID, error)
}

func (*mockProvider) FetchAccount(context.Context, *types.User, []*extsvc.Account) (*extsvc.Account, error) {
	return nil, nil
}

func (p *mockProvider) ServiceType() string { return p.serviceType }
func (p *mockProvider) ServiceID() string   { return p.serviceID }
func (p *mockProvider) URN() string         { return extsvc.URN(p.serviceType, p.id) }
func (*mockProvider) Validate() []string    { return nil }

func (p *mockProvider) FetchUserPerms(ctx context.Context, acct *extsvc.Account) (*authz.ExternalUserPermissions, error) {
	return p.fetchUserPerms(ctx, acct)
}

func (p *mockProvider) FetchRepoPerms(ctx context.Context, repo *extsvc.Repository) ([]extsvc.AccountID, error) {
	return p.fetchRepoPerms(ctx, repo)
}

func TestPermsSyncer_syncUserPerms(t *testing.T) {
	p := &mockProvider{
		serviceType: extsvc.TypeGitLab,
		serviceID:   "https://gitlab.com/",
	}
	authz.SetProviders(false, []authz.Provider{p})
	defer authz.SetProviders(true, nil)

	extAccount := extsvc.Account{
		AccountSpec: extsvc.AccountSpec{
			ServiceType: p.ServiceType(),
			ServiceID:   p.ServiceID(),
		},
	}

	database.Mocks.Users.GetByID = func(ctx context.Context, id int32) (*types.User, error) {
		return &types.User{ID: id}, nil
	}
	database.Mocks.ExternalAccounts.TouchLastValid = func(ctx context.Context, id int32) error {
		return nil
	}
	edb.Mocks.Perms.ListExternalAccounts = func(context.Context, int32) ([]*extsvc.Account, error) {
		return []*extsvc.Account{&extAccount}, nil
	}
	edb.Mocks.Perms.SetUserPermissions = func(_ context.Context, p *authz.UserPermissions) error {
		if p.UserID != 1 {
			return fmt.Errorf("UserID: want 1 but got %d", p.UserID)
		}

		wantIDs := []uint32{1}
		if diff := cmp.Diff(wantIDs, p.IDs.ToArray()); diff != "" {
			return fmt.Errorf("IDs mismatch (-want +got):\n%s", diff)
		}
		return nil
	}
	database.Mocks.Repos.List = func(v0 context.Context, args database.ReposListOptions) ([]*types.Repo, error) {
		if !args.OnlyPrivate {
			return nil, errors.New("OnlyPrivate want true but got false")
		}
		return []*types.Repo{{ID: 1}}, nil
	}
	defer func() {
		database.Mocks.Users = database.MockUsers{}
		database.Mocks.ExternalAccounts = database.MockExternalAccounts{}
		edb.Mocks.Perms = edb.MockPerms{}
		database.Mocks.Repos = database.MockRepos{}
	}()

	permsStore := edb.Perms(nil, timeutil.Now)
	s := NewPermsSyncer(repos.NewStore(dbconn.Global, sql.TxOptions{}), permsStore, timeutil.Now, nil)

	tests := []struct {
		name     string
		noPerms  bool
		fetchErr error
	}{
		{
			name:     "sync for the first time and encounter an error",
			noPerms:  true,
			fetchErr: errors.New("random error"),
		},
		{
			name:    "sync for the second time and succeed",
			noPerms: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p.fetchUserPerms = func(context.Context, *extsvc.Account) (*authz.ExternalUserPermissions, error) {
				return &authz.ExternalUserPermissions{
					Exacts: []extsvc.RepoID{"1"},
				}, test.fetchErr
			}

			err := s.syncUserPerms(context.Background(), 1, test.noPerms)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPermsSyncer_syncUserPerms_tokenExpire(t *testing.T) {
	p := &mockProvider{
		serviceType: extsvc.TypeGitHub,
		serviceID:   "https://github.com/",
	}
	authz.SetProviders(false, []authz.Provider{p})
	defer authz.SetProviders(true, nil)

	extAccount := extsvc.Account{
		AccountSpec: extsvc.AccountSpec{
			ServiceType: p.ServiceType(),
			ServiceID:   p.ServiceID(),
		},
	}

	database.Mocks.Users.GetByID = func(ctx context.Context, id int32) (*types.User, error) {
		return &types.User{ID: id}, nil
	}
	edb.Mocks.Perms.ListExternalAccounts = func(context.Context, int32) ([]*extsvc.Account, error) {
		return []*extsvc.Account{&extAccount}, nil
	}
	edb.Mocks.Perms.SetUserPermissions = func(_ context.Context, p *authz.UserPermissions) error {
		return nil
	}
	database.Mocks.Repos.List = func(v0 context.Context, args database.ReposListOptions) ([]*types.Repo, error) {
		if !args.OnlyPrivate {
			return nil, errors.New("OnlyPrivate want true but got false")
		}
		return []*types.Repo{{ID: 1}}, nil
	}
	defer func() {
		database.Mocks.Users = database.MockUsers{}
		database.Mocks.ExternalAccounts = database.MockExternalAccounts{}
		edb.Mocks.Perms = edb.MockPerms{}
		database.Mocks.Repos = database.MockRepos{}
	}()

	permsStore := edb.Perms(nil, timeutil.Now)
	s := NewPermsSyncer(repos.NewStore(dbconn.Global, sql.TxOptions{}), permsStore, timeutil.Now, nil)

	t.Run("invalid token", func(t *testing.T) {
		calledTouchExpired := false
		database.Mocks.ExternalAccounts.TouchExpired = func(ctx context.Context, id int32) error {
			calledTouchExpired = true
			return nil
		}

		p.fetchUserPerms = func(ctx context.Context, account *extsvc.Account) (*authz.ExternalUserPermissions, error) {
			return nil, &github.APIError{Code: http.StatusUnauthorized}
		}

		err := s.syncUserPerms(context.Background(), 1, false)
		if err != nil {
			t.Fatal(err)
		}

		if !calledTouchExpired {
			t.Fatal("!calledTouchExpired")
		}
	})

	t.Run("account suspension", func(t *testing.T) {
		calledTouchExpired := false
		database.Mocks.ExternalAccounts.TouchExpired = func(ctx context.Context, id int32) error {
			calledTouchExpired = true
			return nil
		}

		p.fetchUserPerms = func(ctx context.Context, account *extsvc.Account) (*authz.ExternalUserPermissions, error) {
			return nil, &github.APIError{
				URL:     "https://api.github.com/user/repos",
				Code:    http.StatusForbidden,
				Message: "Sorry. Your account was suspended",
			}
		}

		err := s.syncUserPerms(context.Background(), 1, false)
		if err != nil {
			t.Fatal(err)
		}

		if !calledTouchExpired {
			t.Fatal("!calledTouchExpired")
		}
	})
}

func TestPermsSyncer_syncRepoPerms(t *testing.T) {
	newPermsSyncer := func(store *repos.Store) *PermsSyncer {
		return NewPermsSyncer(store, edb.Perms(nil, timeutil.Now), timeutil.Now, nil)
	}

	t.Run("TouchRepoPermissions is called when no authz provider", func(t *testing.T) {
		calledTouchRepoPermissions := false
		edb.Mocks.Perms.TouchRepoPermissions = func(ctx context.Context, repoID int32) error {
			calledTouchRepoPermissions = true
			return nil
		}
		database.Mocks.Repos.List = func(context.Context, database.ReposListOptions) ([]*types.Repo, error) {
			return []*types.Repo{
				{
					ID:      1,
					Private: true,
					ExternalRepo: api.ExternalRepoSpec{
						ServiceID: "https://gitlab.com/",
					},
					Sources: map[string]*types.SourceInfo{
						extsvc.URN(extsvc.TypeGitLab, 0): {},
					},
				},
			}, nil
		}
		defer func() {
			edb.Mocks.Perms = edb.MockPerms{}
			database.Mocks.Repos = database.MockRepos{}
		}()

		s := newPermsSyncer(repos.NewStore(dbconn.Global, sql.TxOptions{}))

		err := s.syncRepoPerms(context.Background(), 1, false)
		if err != nil {
			t.Fatal(err)
		}

		if !calledTouchRepoPermissions {
			t.Fatal("!calledTouchRepoPermissions")
		}
	})

	t.Run("identify authz provider by URN", func(t *testing.T) {
		// Even though both p1 and p2 are pointing to the same code host,
		// but p2 should not be used because it is not responsible for listing
		// test repository.
		p1 := &mockProvider{
			id:          1,
			serviceType: extsvc.TypeGitLab,
			serviceID:   "https://gitlab.com/",
			fetchRepoPerms: func(ctx context.Context, repo *extsvc.Repository) ([]extsvc.AccountID, error) {
				return []extsvc.AccountID{"user"}, nil
			},
		}
		p2 := &mockProvider{
			id:          2,
			serviceType: extsvc.TypeGitLab,
			serviceID:   "https://gitlab.com/",
			fetchRepoPerms: func(ctx context.Context, repo *extsvc.Repository) ([]extsvc.AccountID, error) {
				return nil, errors.New("not supposed to be called")
			},
		}
		authz.SetProviders(false, []authz.Provider{p1, p2})
		defer authz.SetProviders(true, nil)

		edb.Mocks.Perms.Transact = func(context.Context) (*edb.PermsStore, error) {
			return &edb.PermsStore{}, nil
		}
		edb.Mocks.Perms.GetUserIDsByExternalAccounts = func(context.Context, *extsvc.Accounts) (map[string]int32, error) {
			return map[string]int32{"user": 1}, nil
		}
		edb.Mocks.Perms.SetRepoPermissions = func(_ context.Context, p *authz.RepoPermissions) error {
			if p.RepoID != 1 {
				return fmt.Errorf("RepoID: want 1 but got %d", p.RepoID)
			}

			wantUserIDs := []uint32{1}
			if diff := cmp.Diff(wantUserIDs, p.UserIDs.ToArray()); diff != "" {
				return fmt.Errorf("UserIDs mismatch (-want +got):\n%s", diff)
			}
			return nil
		}
		edb.Mocks.Perms.SetRepoPendingPermissions = func(ctx context.Context, accounts *extsvc.Accounts, p *authz.RepoPermissions) error {
			return nil
		}
		database.Mocks.Repos.List = func(context.Context, database.ReposListOptions) ([]*types.Repo, error) {
			return []*types.Repo{
				{
					ID:      1,
					Private: true,
					ExternalRepo: api.ExternalRepoSpec{
						ServiceID: p1.ServiceID(),
					},
					Sources: map[string]*types.SourceInfo{
						p1.URN(): {},
					},
				},
			}, nil
		}
		defer func() {
			edb.Mocks.Perms = edb.MockPerms{}
			database.Mocks.Repos = database.MockRepos{}
		}()

		s := newPermsSyncer(repos.NewStore(dbconn.Global, sql.TxOptions{}))

		err := s.syncRepoPerms(context.Background(), 1, false)
		if err != nil {
			t.Fatal(err)
		}
	})

	p := &mockProvider{
		serviceType: extsvc.TypeGitLab,
		serviceID:   "https://gitlab.com/",
	}
	authz.SetProviders(false, []authz.Provider{p})
	defer authz.SetProviders(true, nil)

	edb.Mocks.Perms.Transact = func(context.Context) (*edb.PermsStore, error) {
		return &edb.PermsStore{}, nil
	}
	edb.Mocks.Perms.GetUserIDsByExternalAccounts = func(context.Context, *extsvc.Accounts) (map[string]int32, error) {
		return map[string]int32{"user": 1}, nil
	}
	edb.Mocks.Perms.SetRepoPermissions = func(_ context.Context, p *authz.RepoPermissions) error {
		if p.RepoID != 1 {
			return fmt.Errorf("RepoID: want 1 but got %d", p.RepoID)
		}

		wantUserIDs := []uint32{1}
		if diff := cmp.Diff(wantUserIDs, p.UserIDs.ToArray()); diff != "" {
			return fmt.Errorf("UserIDs mismatch (-want +got):\n%s", diff)
		}
		return nil
	}
	edb.Mocks.Perms.SetRepoPendingPermissions = func(_ context.Context, accounts *extsvc.Accounts, _ *authz.RepoPermissions) error {
		wantAccounts := &extsvc.Accounts{
			ServiceType: p.ServiceType(),
			ServiceID:   p.ServiceID(),
			AccountIDs:  []string{"pending_user"},
		}
		if diff := cmp.Diff(wantAccounts, accounts); diff != "" {
			return fmt.Errorf("accounts mismatch (-want +got):\n%s", diff)
		}
		return nil
	}
	database.Mocks.Repos.List = func(context.Context, database.ReposListOptions) ([]*types.Repo, error) {
		return []*types.Repo{
			{
				ID:      1,
				Private: true,
				ExternalRepo: api.ExternalRepoSpec{
					ServiceID: p.ServiceID(),
				},
				Sources: map[string]*types.SourceInfo{
					p.URN(): {},
				},
			},
		}, nil
	}
	defer func() {
		edb.Mocks.Perms = edb.MockPerms{}
		database.Mocks.Repos = database.MockRepos{}
	}()

	s := newPermsSyncer(repos.NewStore(dbconn.Global, sql.TxOptions{}))

	tests := []struct {
		name     string
		noPerms  bool
		fetchErr error
	}{
		{
			name:     "sync for the first time and encounter an error",
			noPerms:  true,
			fetchErr: errors.New("random error"),
		},
		{
			name:    "sync for the second time and succeed",
			noPerms: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p.fetchRepoPerms = func(context.Context, *extsvc.Repository) ([]extsvc.AccountID, error) {
				return []extsvc.AccountID{"user", "pending_user"}, test.fetchErr
			}

			err := s.syncRepoPerms(context.Background(), 1, test.noPerms)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPermsSyncer_waitForRateLimit(t *testing.T) {
	ctx := context.Background()
	t.Run("no rate limit registry", func(t *testing.T) {
		s := NewPermsSyncer(nil, nil, nil, nil)

		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		err := s.waitForRateLimit(ctx, "https://github.com/", 100000)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("enough quota available", func(t *testing.T) {
		rateLimiterRegistry := ratelimit.NewRegistry()
		s := NewPermsSyncer(nil, nil, nil, rateLimiterRegistry)

		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		err := s.waitForRateLimit(ctx, "https://github.com/", 1)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("not enough quota available", func(t *testing.T) {
		rateLimiterRegistry := ratelimit.NewRegistry()
		l := rateLimiterRegistry.Get("https://github.com/")
		l.SetLimit(1)
		s := NewPermsSyncer(nil, nil, nil, rateLimiterRegistry)

		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		err := s.waitForRateLimit(ctx, "https://github.com/", 10)
		if err == nil {
			t.Fatalf("err: want %v but got nil", context.Canceled)
		}
	})
}

func TestPermsSyncer_syncPerms(t *testing.T) {
	request := &syncRequest{
		requestMeta: &requestMeta{
			Type: 3,
			ID:   1,
		},
		acquired: true,
	}

	// Request should be removed from the queue even if error occurred.
	s := NewPermsSyncer(nil, nil, nil, nil)
	s.queue.Push(request)

	expErr := "unexpected request type: 3"
	err := s.syncPerms(context.Background(), request)
	if err == nil || err.Error() != expErr {
		t.Fatalf("err: want %q but got %v", expErr, err)
	}

	if s.queue.Len() != 0 {
		t.Fatalf("queue length: want 0 but got %d", s.queue.Len())
	}
}
