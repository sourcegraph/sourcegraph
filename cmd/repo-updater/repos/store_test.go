package repos

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/kylelemons/godebug/pretty"
	"github.com/pkg/errors"
	"github.com/sourcegraph/sourcegraph/pkg/api"
	"github.com/sourcegraph/sourcegraph/pkg/extsvc/github"
)

// This error is passed to txstore.Done in order to always
// roll-back the transaction a test case executes in.
// This is meant to ensure each test case has a clean slate.
var errRollback = errors.New("tx: rollback")

func TestIntegration_DBStore(t *testing.T) {
	t.Parallel()

	db, cleanup := testDatabase(t)
	defer cleanup()

	for _, tc := range []struct {
		name string
		test func(*testing.T)
	}{
		{"GetRepoByName", testDBStoreGetRepoByName(db)},
		{"UpsertRepos", testDBStoreUpsertRepos(db)},
		{"ListRepos", testDBStoreListRepos(db)},
	} {
		t.Run(tc.name, tc.test)
	}
}

func testDBStoreUpsertRepos(db *sql.DB) func(*testing.T) {
	return func(t *testing.T) {
		kinds := []string{
			"GITHUB",
		}

		ctx := context.Background()
		store := NewDBStore(ctx, db, kinds, sql.TxOptions{Isolation: sql.LevelSerializable})

		t.Run("no repos", func(t *testing.T) {
			if err := store.UpsertRepos(ctx); err != nil {
				t.Fatalf("UpsertRepos error: %s", err)
			}
		})

		t.Run("many repos", transact(ctx, store, func(t testing.TB, tx TxStore) {
			want := make([]*Repo, 0, 512) // Test more than one page load
			for i := 0; i < cap(want); i++ {
				id := strconv.Itoa(i)
				want = append(want, &Repo{
					Name:        "github.com/foo/bar" + id,
					Description: "The description",
					Language:    "barlang",
					Enabled:     true,
					Archived:    false,
					Fork:        false,
					CreatedAt:   time.Now().UTC(),
					ExternalRepo: api.ExternalRepoSpec{
						ID:          id,
						ServiceType: "github",
						ServiceID:   "http://github.com",
					},
					Sources: map[string]*SourceInfo{
						"extsvc:123": {
							ID:       "extsvc:123",
							CloneURL: "git@github.com:foo/bar.git",
						},
					},
					Metadata: []byte("{}"),
				})
			}

			if err := tx.UpsertRepos(ctx, want...); err != nil {
				t.Errorf("UpsertRepos error: %s", err)
				return
			}

			sort.Slice(want, func(i, j int) bool {
				return want[i].ID < want[j].ID
			})

			have, err := tx.ListRepos(ctx)
			if err != nil {
				t.Errorf("ListRepos error: %s", err)
				return
			}

			if diff := pretty.Compare(have, want); diff != "" {
				t.Errorf("ListRepos:\n%s", diff)
				return
			}

			suffix := "-updated"
			now := time.Now()
			for _, r := range want {
				r.Name += suffix
				r.Description += suffix
				r.Language += suffix
				r.UpdatedAt = now
				r.Archived = !r.Archived
				r.Fork = !r.Fork
			}

			if err = tx.UpsertRepos(ctx, want...); err != nil {
				t.Errorf("UpsertRepos error: %s", err)
			} else if have, err = tx.ListRepos(ctx); err != nil {
				t.Errorf("ListRepos error: %s", err)
			} else if diff := pretty.Compare(have, want); diff != "" {
				t.Errorf("ListRepos:\n%s", diff)
			}

			for _, repo := range want {
				repo.DeletedAt = time.Now().UTC()
			}

			if err = tx.UpsertRepos(ctx, want...); err != nil {
				t.Errorf("UpsertRepos error: %s", err)
			} else if have, err = tx.ListRepos(ctx); err != nil {
				t.Errorf("ListRepos error: %s", err)
			} else if diff := pretty.Compare(have, []*Repo{}); diff != "" {
				t.Errorf("ListRepos:\n%s", diff)
			}

		}))
	}
}

func testDBStoreListRepos(db *sql.DB) func(*testing.T) {
	foo := Repo{
		Name: "foo",
		Sources: map[string]*SourceInfo{
			"extsvc:123": {
				ID:       "extsvc:123",
				CloneURL: "git@github.com:bar/foo.git",
			},
		},
		Metadata: new(github.Repository),
		ExternalRepo: api.ExternalRepoSpec{
			ServiceType: "github",
			ServiceID:   "https://github.com/",
			ID:          "foo",
		},
	}

	return func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			kinds  []string
			ctx    context.Context
			names  []string
			stored []*Repo
			repos  []*Repo
			err    error
		}{
			{
				name:  "case-insensitive kinds",
				kinds: []string{"GiThUb"},
				stored: Repos{foo.With(func(r *Repo) {
					r.ExternalRepo.ServiceType = "gItHuB"
				})},
				repos: Repos{foo.With(func(r *Repo) {
					r.ExternalRepo.ServiceType = "gItHuB"
				})},
			},
		} {
			tc := tc
			ctx := context.Background()
			store := NewDBStore(ctx, db, tc.kinds, sql.TxOptions{Isolation: sql.LevelDefault})

			t.Run(tc.name, transact(ctx, store, func(t testing.TB, tx TxStore) {
				if err := tx.UpsertRepos(ctx, tc.stored...); err != nil {
					t.Errorf("failed to setup store: %v", err)
					return
				}

				repos, err := tx.ListRepos(ctx, tc.names...)
				if have, want := fmt.Sprint(err), fmt.Sprint(tc.err); have != want {
					t.Errorf("error:\nhave: %v\nwant: %v", have, want)
				}

				for _, r := range repos {
					r.ID = 0 // Exclude auto-generated IDs from equality tests
				}

				if have, want := repos, tc.repos; !reflect.DeepEqual(have, want) {
					t.Errorf("repos: %s", cmp.Diff(have, want))
				}
			}))
		}
	}
}

func testDBStoreGetRepoByName(db *sql.DB) func(*testing.T) {
	foo := Repo{
		Name: "github.com/foo/bar",
		Sources: map[string]*SourceInfo{
			"extsvc:123": {
				ID:       "extsvc:123",
				CloneURL: "git@github.com:foo/bar.git",
			},
		},
		Metadata: new(github.Repository),
		ExternalRepo: api.ExternalRepoSpec{
			ServiceType: "github",
			ServiceID:   "https://github.com/",
			ID:          "bar",
		},
	}

	return func(t *testing.T) {
		for _, tc := range []struct {
			test   string
			name   string
			stored []*Repo
			repo   *Repo
			err    error
		}{
			{
				test: "no results error",
				name: "intergalatical repo lost in spaaaaaace",
				err:  ErrNoResults,
			},
			{
				test:   "success",
				stored: Repos{foo.Clone()},
				name:   foo.Name,
				repo:   foo.Clone(),
			},
		} {
			// NOTE: We use t.Errorf instead of t.Fatalf in order to run defers.

			tc := tc
			ctx := context.Background()
			store := NewDBStore(ctx, db, []string{"GITHUB"}, sql.TxOptions{Isolation: sql.LevelDefault})

			t.Run(tc.test, transact(ctx, store, func(t testing.TB, tx TxStore) {
				if err := tx.UpsertRepos(ctx, tc.stored...); err != nil {
					t.Errorf("failed to setup store: %v", err)
					return
				}

				repo, err := tx.GetRepoByName(ctx, tc.name)
				if have, want := fmt.Sprint(err), fmt.Sprint(tc.err); have != want {
					t.Errorf("error:\nhave: %v\nwant: %v", have, want)
				}

				if repo != nil {
					repo.ID = 0 // Exclude auto-generated IDs from equality tests
				}

				if have, want := repo, tc.repo; !reflect.DeepEqual(have, want) {
					t.Errorf("repos: %s", cmp.Diff(have, want))
				}
			}))
		}
	}
}

func transact(ctx context.Context, store *DBStore, test func(testing.TB, TxStore)) func(*testing.T) {
	// NOTE: We use t.Errorf instead of t.Fatalf in order to run defers.
	return func(t *testing.T) {
		txstore, err := store.Transact(ctx)
		if err != nil {
			t.Errorf("failed to start transaction: %v", err)
			return
		}
		defer txstore.Done(&errRollback)
		test(t, txstore)
	}
}
