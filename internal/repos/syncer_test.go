package repos_test

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/keegancsmith/sqlf"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/awscodecommit"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketcloud"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitolite"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/types/typestest"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestSyncerSync(t *testing.T) {
	t.Parallel()
	store := getTestRepoStore(t)

	servicesPerKind := createExternalServices(t, store)

	githubService := servicesPerKind[extsvc.KindGitHub]

	githubRepo := (&types.Repo{
		Name:     "github.com/org/foo",
		Metadata: &github.Repository{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "foo-external-12345",
			ServiceID:   "https://github.com/",
			ServiceType: extsvc.TypeGitHub,
		},
	}).With(
		typestest.Opt.RepoSources(githubService.URN()),
	)

	gitlabService := servicesPerKind[extsvc.KindGitLab]

	gitlabRepo := (&types.Repo{
		Name:     "gitlab.com/org/foo",
		Metadata: &gitlab.Project{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "12345",
			ServiceID:   "https://gitlab.com/",
			ServiceType: extsvc.TypeGitLab,
		},
	}).With(
		typestest.Opt.RepoSources(gitlabService.URN()),
	)

	bitbucketServerService := servicesPerKind[extsvc.KindBitbucketServer]

	bitbucketServerRepo := (&types.Repo{
		Name:     "bitbucketserver.mycorp.com/org/foo",
		Metadata: &bitbucketserver.Repo{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "23456",
			ServiceID:   "https://bitbucketserver.mycorp.com/",
			ServiceType: "bitbucketServer",
		},
	}).With(
		typestest.Opt.RepoSources(bitbucketServerService.URN()),
	)

	awsCodeCommitService := servicesPerKind[extsvc.KindAWSCodeCommit]

	awsCodeCommitRepo := (&types.Repo{
		Name:     "git-codecommit.us-west-1.amazonaws.com/stripe-go",
		Metadata: &awscodecommit.Repository{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "f001337a-3450-46fd-b7d2-650c0EXAMPLE",
			ServiceID:   "arn:aws:codecommit:us-west-1:999999999999:",
			ServiceType: extsvc.TypeAWSCodeCommit,
		},
	}).With(
		typestest.Opt.RepoSources(awsCodeCommitService.URN()),
	)

	otherService := servicesPerKind[extsvc.KindOther]

	otherRepo := (&types.Repo{
		Name: "git-host.com/org/foo",
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "git-host.com/org/foo",
			ServiceID:   "https://git-host.com/",
			ServiceType: extsvc.TypeOther,
		},
		Metadata: &extsvc.OtherRepoMetadata{},
	}).With(
		typestest.Opt.RepoSources(otherService.URN()),
	)

	gitoliteService := servicesPerKind[extsvc.KindGitolite]

	gitoliteRepo := (&types.Repo{
		Name:     "gitolite.mycorp.com/foo",
		Metadata: &gitolite.Repo{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "foo",
			ServiceID:   "git@gitolite.mycorp.com",
			ServiceType: extsvc.TypeGitolite,
		},
	}).With(
		typestest.Opt.RepoSources(gitoliteService.URN()),
	)

	bitbucketCloudService := servicesPerKind[extsvc.KindBitbucketCloud]

	bitbucketCloudRepo := (&types.Repo{
		Name:     "bitbucket.org/team/foo",
		Metadata: &bitbucketcloud.Repo{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "{e164a64c-bd73-4a40-b447-d71b43f328a8}",
			ServiceID:   "https://bitbucket.org/",
			ServiceType: extsvc.TypeBitbucketCloud,
		},
	}).With(
		typestest.Opt.RepoSources(bitbucketCloudService.URN()),
	)

	clock := timeutil.NewFakeClock(time.Now(), 0)

	svcdup := types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github2 - Test",
		Config:      extsvc.NewUnencryptedConfig(basicGitHubConfig),
		CreatedAt:   clock.Now(),
		UpdatedAt:   clock.Now(),
	}

	q := sqlf.Sprintf(`INSERT INTO users (id, username) VALUES (1, 'u')`)
	_, err := store.Handle().ExecContext(context.Background(), q.Query(sqlf.PostgresBindVar), q.Args()...)
	if err != nil {
		t.Fatal(err)
	}

	// create a few external services
	if err := store.ExternalServiceStore().Upsert(context.Background(), &svcdup); err != nil {
		t.Fatalf("failed to insert external services: %v", err)
	}

	type testCase struct {
		name    string
		sourcer repos.Sourcer
		store   repos.Store
		stored  types.Repos
		svcs    []*types.ExternalService
		ctx     context.Context
		now     func() time.Time
		diff    types.RepoSyncDiff
		err     string
	}

	var testCases []testCase
	for _, tc := range []struct {
		repo *types.Repo
		svc  *types.ExternalService
	}{
		{repo: githubRepo, svc: githubService},
		{repo: gitlabRepo, svc: gitlabService},
		{repo: bitbucketServerRepo, svc: bitbucketServerService},
		{repo: awsCodeCommitRepo, svc: awsCodeCommitService},
		{repo: otherRepo, svc: otherService},
		{repo: gitoliteRepo, svc: gitoliteService},
		{repo: bitbucketCloudRepo, svc: bitbucketCloudService},
	} {
		testCases = append(testCases,
			testCase{
				name: string(tc.repo.Name) + "/new repo",
				sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(tc.svc.Clone(), nil, tc.repo.Clone()),
				),
				store:  store,
				stored: types.Repos{},
				now:    clock.Now,
				diff: types.RepoSyncDiff{Added: types.Repos{tc.repo.With(
					typestest.Opt.RepoCreatedAt(clock.Time(1)),
					typestest.Opt.RepoSources(tc.svc.Clone().URN()),
				)}},
				svcs: []*types.ExternalService{tc.svc},
				err:  "<nil>",
			},
		)

		var diff types.RepoSyncDiff
		diff.Unmodified = append(diff.Unmodified, tc.repo.With(
			typestest.Opt.RepoSources(tc.svc.URN()),
		))

		testCases = append(testCases,
			testCase{
				// If the source is unauthorized we should treat this as if zero repos were
				// returned as it indicates that the source no longer has access to its repos
				name: string(tc.repo.Name) + "/unauthorized",
				sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(tc.svc.Clone(), &repos.ErrUnauthorized{}),
				),
				store: store,
				stored: types.Repos{tc.repo.With(
					typestest.Opt.RepoSources(tc.svc.URN()),
				)},
				now:  clock.Now,
				diff: diff,
				svcs: []*types.ExternalService{tc.svc},
				err:  "bad credentials",
			},
			testCase{
				// If the source is unauthorized with a warning rather than an error,
				// the sync will continue. If the warning error is unauthorized, the
				// corresponding repos will be deleted as it's seen as permissions changes.
				name: string(tc.repo.Name) + "/unauthorized-with-warning",
				sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(tc.svc.Clone(), errors.NewWarningError(&repos.ErrUnauthorized{})),
				),
				store: store,
				stored: types.Repos{tc.repo.With(
					typestest.Opt.RepoSources(tc.svc.URN()),
				)},
				now: clock.Now,
				diff: types.RepoSyncDiff{
					Deleted: types.Repos{
						tc.repo.With(func(r *types.Repo) {
							r.Sources = map[string]*types.SourceInfo{}
							r.DeletedAt = clock.Time(0)
							r.UpdatedAt = clock.Time(0)
						}),
					},
				},
				svcs: []*types.ExternalService{tc.svc},
				err:  "bad credentials",
			},
			testCase{
				// If the source is forbidden we should treat this as if zero repos were returned
				// as it indicates that the source no longer has access to its repos
				name: string(tc.repo.Name) + "/forbidden",
				sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(tc.svc.Clone(), &repos.ErrForbidden{}),
				),
				store: store,
				stored: types.Repos{tc.repo.With(
					typestest.Opt.RepoSources(tc.svc.URN()),
				)},
				now:  clock.Now,
				diff: diff,
				svcs: []*types.ExternalService{tc.svc},
				err:  "forbidden",
			},
			testCase{
				// If the source is forbidden with a warning rather than an error,
				// the sync will continue. If the warning error is forbidden, the
				// corresponding repos will be deleted as it's seen as permissions changes.
				name: string(tc.repo.Name) + "/forbidden-with-warning",
				sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(tc.svc.Clone(), errors.NewWarningError(&repos.ErrForbidden{})),
				),
				store: store,
				stored: types.Repos{tc.repo.With(
					typestest.Opt.RepoSources(tc.svc.URN()),
				)},
				now: clock.Now,
				diff: types.RepoSyncDiff{
					Deleted: types.Repos{
						tc.repo.With(func(r *types.Repo) {
							r.Sources = map[string]*types.SourceInfo{}
							r.DeletedAt = clock.Time(0)
							r.UpdatedAt = clock.Time(0)
						}),
					},
				},
				svcs: []*types.ExternalService{tc.svc},
				err:  "forbidden",
			},
			testCase{
				// If the source account has been suspended we should treat this as if zero repos were returned as it indicates
				// that the source no longer has access to its repos
				name: string(tc.repo.Name) + "/accountsuspended",
				sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(tc.svc.Clone(), &repos.ErrAccountSuspended{}),
				),
				store: store,
				stored: types.Repos{tc.repo.With(
					typestest.Opt.RepoSources(tc.svc.URN()),
				)},
				now:  clock.Now,
				diff: diff,
				svcs: []*types.ExternalService{tc.svc},
				err:  "account suspended",
			},
			testCase{
				// If the source is account suspended with a warning rather than an error,
				// the sync will terminate. This is the only warning error that the sync will abort
				name: string(tc.repo.Name) + "/accountsuspended-with-warning",
				sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(tc.svc.Clone(), errors.NewWarningError(&repos.ErrAccountSuspended{})),
				),
				store: store,
				stored: types.Repos{tc.repo.With(
					typestest.Opt.RepoSources(tc.svc.URN()),
				)},
				now:  clock.Now,
				diff: diff,
				svcs: []*types.ExternalService{tc.svc},
				err:  "account suspended",
			},
			testCase{
				// Test that spurious errors don't cause deletions.
				name: string(tc.repo.Name) + "/spurious-error",
				sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(tc.svc.Clone(), io.EOF),
				),
				store: store,
				stored: types.Repos{tc.repo.With(
					typestest.Opt.RepoSources(tc.svc.URN()),
				)},
				now: clock.Now,
				diff: types.RepoSyncDiff{Unmodified: types.Repos{tc.repo.With(
					typestest.Opt.RepoSources(tc.svc.URN()),
				)}},
				svcs: []*types.ExternalService{tc.svc},
				err:  io.EOF.Error(),
			},
			testCase{
				// If the source is a spurious error with a warning rather than an error,
				// the sync will continue. However, no repos will be deleted.
				name: string(tc.repo.Name) + "/spurious-error-with-warning",
				sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(tc.svc.Clone(), errors.NewWarningError(io.EOF)),
				),
				store: store,
				stored: types.Repos{tc.repo.With(
					typestest.Opt.RepoSources(tc.svc.URN()),
				)},
				now: clock.Now,
				diff: types.RepoSyncDiff{Unmodified: types.Repos{tc.repo.With(
					typestest.Opt.RepoSources(tc.svc.URN()),
				)}},
				svcs: []*types.ExternalService{tc.svc},
				err:  io.EOF.Error(),
			},
			testCase{
				// It's expected that there could be multiple stored sources but only one will ever be returned
				// by the code host as it can't know about others.
				name: string(tc.repo.Name) + "/source already stored",
				sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(tc.svc.Clone(), nil, tc.repo.Clone()),
				),
				store: store,
				stored: types.Repos{tc.repo.With(
					typestest.Opt.RepoSources(tc.svc.URN(), svcdup.URN()),
				)},
				now: clock.Now,
				diff: types.RepoSyncDiff{Unmodified: types.Repos{tc.repo.With(
					typestest.Opt.RepoSources(tc.svc.URN(), svcdup.URN()),
				)}},
				svcs: []*types.ExternalService{tc.svc},
				err:  "<nil>",
			},
			testCase{
				name: string(tc.repo.Name) + "/deleted ALL repo sources",
				sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(tc.svc.Clone(), nil),
				),
				store: store,
				stored: types.Repos{tc.repo.With(
					typestest.Opt.RepoSources(tc.svc.URN(), svcdup.URN()),
				)},
				now: clock.Now,
				diff: types.RepoSyncDiff{Deleted: types.Repos{tc.repo.With(
					typestest.Opt.RepoDeletedAt(clock.Time(1)),
				)}},
				svcs: []*types.ExternalService{tc.svc, &svcdup},
				err:  "<nil>",
			},
			testCase{
				name:    string(tc.repo.Name) + "/renamed repo is detected via external_id",
				sourcer: repos.NewFakeSourcer(nil, repos.NewFakeSource(tc.svc.Clone(), nil, tc.repo.Clone())),
				store:   store,
				stored: types.Repos{tc.repo.With(func(r *types.Repo) {
					r.Name = "old-name"
				})},
				now: clock.Now,
				diff: types.RepoSyncDiff{
					Modified: types.ReposModified{
						{
							Repo:     tc.repo.With(typestest.Opt.RepoModifiedAt(clock.Time(1))),
							Modified: types.RepoModifiedName,
						},
					},
				},
				svcs: []*types.ExternalService{tc.svc},
				err:  "<nil>",
			},
			testCase{
				name: string(tc.repo.Name) + "/repo got renamed to another repo that gets deleted",
				sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(tc.svc.Clone(), nil,
						tc.repo.With(func(r *types.Repo) { r.ExternalRepo.ID = "another-id" }),
					),
				),
				store: store,
				stored: types.Repos{
					tc.repo.Clone(),
					tc.repo.With(func(r *types.Repo) {
						r.Name = "another-repo"
						r.ExternalRepo.ID = "another-id"
					}),
				},
				now: clock.Now,
				diff: types.RepoSyncDiff{
					Deleted: types.Repos{
						tc.repo.With(func(r *types.Repo) {
							r.Sources = map[string]*types.SourceInfo{}
							r.DeletedAt = clock.Time(0)
							r.UpdatedAt = clock.Time(0)
						}),
					},
					Modified: types.ReposModified{
						{
							Repo: tc.repo.With(
								typestest.Opt.RepoModifiedAt(clock.Time(1)),
								func(r *types.Repo) { r.ExternalRepo.ID = "another-id" },
							),
							Modified: types.RepoModifiedExternalRepo,
						},
					},
				},
				svcs: []*types.ExternalService{tc.svc},
				err:  "<nil>",
			},
			testCase{
				name: string(tc.repo.Name) + "/repo inserted with same name as another repo that gets deleted",
				sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(tc.svc.Clone(), nil,
						tc.repo,
					),
				),
				store: store,
				stored: types.Repos{
					tc.repo.With(typestest.Opt.RepoExternalID("another-id")),
				},
				now: clock.Now,
				diff: types.RepoSyncDiff{
					Added: types.Repos{
						tc.repo.With(
							typestest.Opt.RepoCreatedAt(clock.Time(1)),
							typestest.Opt.RepoModifiedAt(clock.Time(1)),
						),
					},
					Deleted: types.Repos{
						tc.repo.With(func(r *types.Repo) {
							r.ExternalRepo.ID = "another-id"
							r.Sources = map[string]*types.SourceInfo{}
							r.DeletedAt = clock.Time(0)
							r.UpdatedAt = clock.Time(0)
						}),
					},
				},
				svcs: []*types.ExternalService{tc.svc},
				err:  "<nil>",
			},
			testCase{
				name: string(tc.repo.Name) + "/repo inserted with same name as repo without id",
				sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(tc.svc.Clone(), nil,
						tc.repo,
					),
				),
				store: store,
				stored: types.Repos{
					tc.repo.With(typestest.Opt.RepoName("old-name")),  // same external id as sourced
					tc.repo.With(typestest.Opt.RepoExternalID("bar")), // same name as sourced
				}.With(typestest.Opt.RepoCreatedAt(clock.Time(1))),
				now: clock.Now,
				diff: types.RepoSyncDiff{
					Modified: types.ReposModified{
						{
							Repo: tc.repo.With(
								typestest.Opt.RepoCreatedAt(clock.Time(1)),
								typestest.Opt.RepoModifiedAt(clock.Time(1)),
							),
							Modified: types.RepoModifiedName | types.RepoModifiedExternalRepo,
						},
					},
					Deleted: types.Repos{
						tc.repo.With(func(r *types.Repo) {
							r.ExternalRepo.ID = ""
							r.Sources = map[string]*types.SourceInfo{}
							r.DeletedAt = clock.Time(0)
							r.UpdatedAt = clock.Time(0)
							r.CreatedAt = clock.Time(0)
						}),
					},
				},
				svcs: []*types.ExternalService{tc.svc},
				err:  "<nil>",
			},
			testCase{
				name:    string(tc.repo.Name) + "/renamed repo which was deleted is detected and added",
				sourcer: repos.NewFakeSourcer(nil, repos.NewFakeSource(tc.svc.Clone(), nil, tc.repo.Clone())),
				store:   store,
				stored: types.Repos{tc.repo.With(func(r *types.Repo) {
					r.Sources = map[string]*types.SourceInfo{}
					r.Name = "old-name"
					r.DeletedAt = clock.Time(0)
				})},
				now: clock.Now,
				diff: types.RepoSyncDiff{Added: types.Repos{
					tc.repo.With(
						typestest.Opt.RepoCreatedAt(clock.Time(1))),
				}},
				svcs: []*types.ExternalService{tc.svc},
				err:  "<nil>",
			},
			testCase{
				name: string(tc.repo.Name) + "/repos have their names swapped",
				sourcer: repos.NewFakeSourcer(nil, repos.NewFakeSource(tc.svc.Clone(), nil,
					tc.repo.With(func(r *types.Repo) {
						r.Name = "foo"
						r.ExternalRepo.ID = "1"
					}),
					tc.repo.With(func(r *types.Repo) {
						r.Name = "bar"
						r.ExternalRepo.ID = "2"
					}),
				)),
				now:   clock.Now,
				store: store,
				stored: types.Repos{
					tc.repo.With(func(r *types.Repo) {
						r.Name = "bar"
						r.ExternalRepo.ID = "1"
					}),
					tc.repo.With(func(r *types.Repo) {
						r.Name = "foo"
						r.ExternalRepo.ID = "2"
					}),
				},
				diff: types.RepoSyncDiff{
					Modified: types.ReposModified{
						{
							Repo: tc.repo.With(func(r *types.Repo) {
								r.Name = "foo"
								r.ExternalRepo.ID = "1"
								r.UpdatedAt = clock.Time(0)
							}),
						},
						{
							Repo: tc.repo.With(func(r *types.Repo) {
								r.Name = "bar"
								r.ExternalRepo.ID = "2"
								r.UpdatedAt = clock.Time(0)
							}),
						},
					},
				},
				svcs: []*types.ExternalService{tc.svc},
				err:  "<nil>",
			},
			testCase{
				name: string(tc.repo.Name) + "/case insensitive name",
				sourcer: repos.NewFakeSourcer(nil, repos.NewFakeSource(tc.svc.Clone(), nil,
					tc.repo.Clone(),
					tc.repo.With(typestest.Opt.RepoName(api.RepoName(strings.ToUpper(string(tc.repo.Name))))),
				)),
				store: store,
				stored: types.Repos{
					tc.repo.With(typestest.Opt.RepoName(api.RepoName(strings.ToUpper(string(tc.repo.Name))))),
				},
				now: clock.Now,
				diff: types.RepoSyncDiff{
					Modified: types.ReposModified{
						{Repo: tc.repo.With(typestest.Opt.RepoModifiedAt(clock.Time(0)))},
					},
				},
				svcs: []*types.ExternalService{tc.svc},
				err:  "<nil>",
			},
		)
	}

	for _, tc := range testCases {
		if tc.name == "" {
			t.Error("Test case name is blank")
			continue
		}

		tc := tc
		ctx := context.Background()

		t.Run(tc.name, transact(ctx, tc.store, func(t testing.TB, st repos.Store) {
			defer func() {
				if err := recover(); err != nil {
					t.Fatalf("%q panicked: %v", tc.name, err)
				}
			}()
			if st == nil {
				t.Fatal("nil store")
			}
			now := tc.now
			if now == nil {
				clock := timeutil.NewFakeClock(time.Now(), time.Second)
				now = clock.Now
			}

			ctx := tc.ctx
			if ctx == nil {
				ctx = context.Background()
			}

			if len(tc.stored) > 0 {
				cloned := tc.stored.Clone()
				if err := st.RepoStore().Create(ctx, cloned...); err != nil {
					t.Fatalf("failed to prepare store: %v", err)
				}
			}

			syncer := &repos.Syncer{
				ObsvCtx: observation.TestContextTB(t),
				Sourcer: tc.sourcer,
				Store:   st,
				Now:     now,
			}

			for _, svc := range tc.svcs {
				before, err := st.ExternalServiceStore().GetByID(ctx, svc.ID)
				if err != nil {
					t.Fatal(err)
				}

				err = syncer.SyncExternalService(ctx, svc.ID, time.Millisecond, noopProgressRecorder)
				if have, want := fmt.Sprint(err), tc.err; !strings.Contains(have, want) {
					t.Errorf("error %q doesn't contain %q", have, want)
				}

				after, err := st.ExternalServiceStore().GetByID(ctx, svc.ID)
				if err != nil {
					t.Fatal(err)
				}

				// last_synced should always be updated
				if before.LastSyncAt == after.LastSyncAt {
					t.Log(before.LastSyncAt, after.LastSyncAt)
					t.Errorf("Service %q last_synced was not updated", svc.DisplayName)
				}
			}

			var want, have types.Repos
			want.Concat(tc.diff.Added, tc.diff.Modified.Repos(), tc.diff.Unmodified)
			have, _ = st.RepoStore().List(ctx, database.ReposListOptions{})

			want = want.With(typestest.Opt.RepoID(0))
			have = have.With(typestest.Opt.RepoID(0))
			sort.Sort(want)
			sort.Sort(have)

			typestest.AssertReposEqual(want...)(t, have)
		}))
	}
}

func TestSyncRepo(t *testing.T) {
	t.Parallel()
	store := getTestRepoStore(t)

	servicesPerKind := createExternalServices(t, store, func(svc *types.ExternalService) { svc.CloudDefault = true })

	repo := &types.Repo{
		ID:          0, // explicitly make default value for sourced repo
		Name:        "github.com/foo/bar",
		Description: "The description",
		Archived:    false,
		Fork:        false,
		Stars:       100,
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==",
			ServiceType: extsvc.TypeGitHub,
			ServiceID:   "https://github.com/",
		},
		Sources: map[string]*types.SourceInfo{
			servicesPerKind[extsvc.KindGitHub].URN(): {
				ID:       servicesPerKind[extsvc.KindGitHub].URN(),
				CloneURL: "git@github.com:foo/bar.git",
			},
		},
		Metadata: &github.Repository{
			ID:             "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==",
			URL:            "github.com/foo/bar",
			DatabaseID:     1234,
			Description:    "The description",
			NameWithOwner:  "foo/bar",
			StargazerCount: 100,
		},
	}

	now := time.Now().UTC()
	oldRepo := repo.With(func(r *types.Repo) {
		r.UpdatedAt = now.Add(-time.Hour)
		r.CreatedAt = r.UpdatedAt.Add(-time.Hour)
		r.Stars = 0
	})

	testCases := []struct {
		name       string
		repo       api.RepoName
		background bool               // whether to run SyncRepo in the background
		before     types.Repos        // the repos to insert into the database before syncing
		sourced    *types.Repo        // the repo that is returned by the fake sourcer
		returned   *types.Repo        // the expected return value from SyncRepo (which changes meaning depending on background)
		after      types.Repos        // the expected database repos after syncing
		diff       types.RepoSyncDiff // the expected types.Diff sent by the syncer
	}{{
		name:       "insert",
		repo:       repo.Name,
		background: true,
		sourced:    repo.Clone(),
		returned:   repo,
		after:      types.Repos{repo},
		diff: types.RepoSyncDiff{
			Added: types.Repos{repo},
		},
	}, {
		name:       "update",
		repo:       repo.Name,
		background: true,
		before:     types.Repos{oldRepo},
		sourced:    repo.Clone(),
		returned:   oldRepo,
		after:      types.Repos{repo},
		diff: types.RepoSyncDiff{
			Modified: types.ReposModified{
				{Repo: repo, Modified: types.RepoModifiedStars},
			},
		},
	}, {
		name:       "blocking update",
		repo:       repo.Name,
		background: false,
		before:     types.Repos{oldRepo},
		sourced:    repo.Clone(),
		returned:   repo,
		after:      types.Repos{repo},
		diff: types.RepoSyncDiff{
			Modified: types.ReposModified{
				{Repo: repo, Modified: types.RepoModifiedStars},
			},
		},
	}, {
		name:       "update name",
		repo:       repo.Name,
		background: true,
		before:     types.Repos{repo.With(typestest.Opt.RepoName("old/name"))},
		sourced:    repo.Clone(),
		returned:   repo,
		after:      types.Repos{repo},
		diff: types.RepoSyncDiff{
			Modified: types.ReposModified{
				{Repo: repo, Modified: types.RepoModifiedName},
			},
		},
	}, {
		name:       "archived",
		repo:       repo.Name,
		background: true,
		before:     types.Repos{repo},
		sourced:    repo.With(typestest.Opt.RepoArchived(true)),
		returned:   repo,
		after:      types.Repos{repo.With(typestest.Opt.RepoArchived(true))},
		diff: types.RepoSyncDiff{
			Modified: types.ReposModified{
				{
					Repo:     repo.With(typestest.Opt.RepoArchived(true)),
					Modified: types.RepoModifiedArchived,
				},
			},
		},
	}, {
		name:       "unarchived",
		repo:       repo.Name,
		background: true,
		before:     types.Repos{repo.With(typestest.Opt.RepoArchived(true))},
		sourced:    repo.Clone(),
		returned:   repo.With(typestest.Opt.RepoArchived(true)),
		after:      types.Repos{repo},
		diff: types.RepoSyncDiff{
			Modified: types.ReposModified{
				{Repo: repo, Modified: types.RepoModifiedArchived},
			},
		},
	}, {
		name:       "delete conflicting name",
		repo:       repo.Name,
		background: true,
		before:     types.Repos{repo.With(typestest.Opt.RepoExternalID("old id"))},
		sourced:    repo.Clone(),
		returned:   repo.With(typestest.Opt.RepoExternalID("old id")),
		after:      types.Repos{repo},
		diff: types.RepoSyncDiff{
			Modified: types.ReposModified{
				{Repo: repo, Modified: types.RepoModifiedExternalRepo},
			},
		},
	}, {
		name:       "rename and delete conflicting name",
		repo:       repo.Name,
		background: true,
		before: types.Repos{
			repo.With(typestest.Opt.RepoExternalID("old id")),
			repo.With(typestest.Opt.RepoName("old name")),
		},
		sourced:  repo.Clone(),
		returned: repo.With(typestest.Opt.RepoExternalID("old id")),
		after:    types.Repos{repo},
		diff: types.RepoSyncDiff{
			Modified: types.ReposModified{
				{Repo: repo, Modified: types.RepoModifiedName},
			},
		},
	}}

	for _, tc := range testCases {
		tc := tc
		ctx := context.Background()

		t.Run(tc.name, func(t *testing.T) {
			q := sqlf.Sprintf("DELETE FROM repo")
			_, err := store.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
			if err != nil {
				t.Fatal(err)
			}

			if len(tc.before) > 0 {
				if err := store.RepoStore().Create(ctx, tc.before.Clone()...); err != nil {
					t.Fatalf("failed to prepare store: %v", err)
				}
			}

			syncer := &repos.Syncer{
				ObsvCtx: observation.TestContextTB(t),
				Now:     time.Now,
				Store:   store,
				Synced:  make(chan types.RepoSyncDiff, 1),
				Sourcer: repos.NewFakeSourcer(nil,
					repos.NewFakeSource(servicesPerKind[extsvc.KindGitHub], nil, tc.sourced),
				),
			}

			have, err := syncer.SyncRepo(ctx, tc.repo, tc.background)
			if err != nil {
				t.Fatal(err)
			}

			if have.ID == 0 {
				t.Errorf("expected returned synced repo to have an ID set")
			}

			opt := cmpopts.IgnoreFields(types.Repo{}, "ID", "CreatedAt", "UpdatedAt")
			if diff := cmp.Diff(have, tc.returned, opt); diff != "" {
				t.Errorf("returned mismatch: (-have, +want):\n%s", diff)
			}

			if diff := cmp.Diff(<-syncer.Synced, tc.diff, opt); diff != "" {
				t.Errorf("diff mismatch: (-have, +want):\n%s", diff)
			}

			after, err := store.RepoStore().List(ctx, database.ReposListOptions{})
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(types.Repos(after), tc.after, opt); diff != "" {
				t.Errorf("repos mismatch: (-have, +want):\n%s", diff)
			}
		})
	}
}

func TestSyncRun(t *testing.T) {
	t.Parallel()
	store := getTestRepoStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := &types.ExternalService{
		Config: extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "repositoryQuery": ["none"], "token": "abc"}`),
		Kind:   extsvc.KindGitHub,
	}

	if err := store.ExternalServiceStore().Upsert(ctx, svc); err != nil {
		t.Fatal(err)
	}

	mk := func(name string) *types.Repo {
		return &types.Repo{
			Name:     api.RepoName(name),
			Metadata: &github.Repository{},
			ExternalRepo: api.ExternalRepoSpec{
				ID:          name,
				ServiceID:   "https://github.com",
				ServiceType: svc.Kind,
			},
		}
	}

	// Our test will have 1 initial repo, and discover a new repo on sourcing.
	stored := types.Repos{mk("initial")}.With(typestest.Opt.RepoSources(svc.URN()))
	sourced := types.Repos{
		mk("initial").With(func(r *types.Repo) { r.Description = "updated" }),
		mk("new"),
	}

	fakeSource := repos.NewFakeSource(svc, nil, sourced...)

	// Lock our source here so that we block when trying to list repos, this allows
	// us to test lower down that we can't delete a syncing service.
	lockChan := fakeSource.InitLockChan()

	syncer := &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: repos.NewFakeSourcer(nil, fakeSource),
		Store:   store,
		Synced:  make(chan types.RepoSyncDiff),
		Now:     time.Now,
	}

	// Initial repos in store
	if err := store.RepoStore().Create(ctx, stored...); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		goroutine.MonitorBackgroundRoutines(ctx, syncer.Routines(ctx, store, repos.RunOptions{
			EnqueueInterval: func() time.Duration { return time.Second },
			IsDotCom:        false,
			MinSyncInterval: func() time.Duration { return 1 * time.Millisecond },
			DequeueInterval: 1 * time.Millisecond,
		})...)
		done <- struct{}{}
	}()

	// Ignore fields store adds
	ignore := cmpopts.IgnoreFields(types.Repo{}, "ID", "CreatedAt", "UpdatedAt", "Sources")

	// The first thing sent down Synced is the list of repos in store during
	// initialisation
	diff := <-syncer.Synced
	if d := cmp.Diff(types.RepoSyncDiff{Unmodified: stored}, diff, ignore); d != "" {
		t.Fatalf("Synced mismatch (-want +got):\n%s", d)
	}

	// Once we receive on lockChan we know our syncer is running
	<-lockChan

	// We can now send on lockChan again to unblock the sync job
	lockChan <- struct{}{}

	// Next up it should find the existing repo and send it down Synced
	diff = <-syncer.Synced
	if d := cmp.Diff(types.RepoSyncDiff{
		Modified: types.ReposModified{
			{Repo: sourced[0], Modified: types.RepoModifiedDescription},
		},
	}, diff, ignore); d != "" {
		t.Fatalf("Synced mismatch (-want +got):\n%s", d)
	}

	// Then the new repo.
	diff = <-syncer.Synced
	if d := cmp.Diff(types.RepoSyncDiff{Added: sourced[1:]}, diff, ignore); d != "" {
		t.Fatalf("Synced mismatch (-want +got):\n%s", d)
	}

	// Allow second round
	<-lockChan
	lockChan <- struct{}{}

	// We check synced again to test us going around the Run loop 2 times in
	// total.
	diff = <-syncer.Synced
	if d := cmp.Diff(types.RepoSyncDiff{Unmodified: sourced[:1]}, diff, ignore); d != "" {
		t.Fatalf("Synced mismatch (-want +got):\n%s", d)
	}

	diff = <-syncer.Synced
	if d := cmp.Diff(types.RepoSyncDiff{Unmodified: sourced[1:]}, diff, ignore); d != "" {
		t.Fatalf("Synced mismatch (-want +got):\n%s", d)
	}

	// Cancel context and the run loop should stop
	cancel()
	<-done
}

func TestSyncerMultipleServices(t *testing.T) {
	t.Parallel()
	store := getTestRepoStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	services := mkExternalServices(time.Now())

	githubService := services[0]
	gitlabService := services[1]
	bitbucketCloudService := services[3]

	services = types.ExternalServices{
		githubService,
		gitlabService,
		bitbucketCloudService,
	}

	// setup services
	if err := store.ExternalServiceStore().Upsert(ctx, services...); err != nil {
		t.Fatal(err)
	}

	githubRepo := (&types.Repo{
		Name:     "github.com/org/foo",
		Metadata: &github.Repository{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "foo-external-12345",
			ServiceID:   "https://github.com/",
			ServiceType: extsvc.TypeGitHub,
		},
	}).With(
		typestest.Opt.RepoSources(githubService.URN()),
	)

	gitlabRepo := (&types.Repo{
		Name:     "gitlab.com/org/foo",
		Metadata: &gitlab.Project{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "12345",
			ServiceID:   "https://gitlab.com/",
			ServiceType: extsvc.TypeGitLab,
		},
	}).With(
		typestest.Opt.RepoSources(gitlabService.URN()),
	)

	bitbucketCloudRepo := (&types.Repo{
		Name:     "bitbucket.org/team/foo",
		Metadata: &bitbucketcloud.Repo{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "{e164a64c-bd73-4a40-b447-d71b43f328a8}",
			ServiceID:   "https://bitbucket.org/",
			ServiceType: extsvc.TypeBitbucketCloud,
		},
	}).With(
		typestest.Opt.RepoSources(bitbucketCloudService.URN()),
	)

	removeSources := func(r *types.Repo) {
		r.Sources = nil
	}

	baseGithubRepos := mkRepos(10, githubRepo)
	githubSourced := baseGithubRepos.Clone().With(removeSources)
	baseGitlabRepos := mkRepos(10, gitlabRepo)
	gitlabSourced := baseGitlabRepos.Clone().With(removeSources)
	baseBitbucketCloudRepos := mkRepos(10, bitbucketCloudRepo)
	bitbucketCloudSourced := baseBitbucketCloudRepos.Clone().With(removeSources)

	sourcers := map[int64]repos.Source{
		githubService.ID:         repos.NewFakeSource(githubService, nil, githubSourced...),
		gitlabService.ID:         repos.NewFakeSource(gitlabService, nil, gitlabSourced...),
		bitbucketCloudService.ID: repos.NewFakeSource(bitbucketCloudService, nil, bitbucketCloudSourced...),
	}

	syncer := &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
			s, ok := sourcers[service.ID]
			if !ok {
				t.Fatalf("sourcer not found: %d", service.ID)
			}
			return s, nil
		},
		Store:  store,
		Synced: make(chan types.RepoSyncDiff),
		Now:    time.Now,
	}

	done := make(chan struct{})
	go func() {
		goroutine.MonitorBackgroundRoutines(ctx, syncer.Routines(ctx, store, repos.RunOptions{
			EnqueueInterval: func() time.Duration { return time.Second },
			IsDotCom:        false,
			MinSyncInterval: func() time.Duration { return 1 * time.Minute },
			DequeueInterval: 1 * time.Millisecond,
		})...)
		done <- struct{}{}
	}()

	// Ignore fields store adds
	ignore := cmpopts.IgnoreFields(types.Repo{}, "ID", "CreatedAt", "UpdatedAt", "Sources")

	// The first thing sent down Synced is an empty list of repos in store.
	diff := <-syncer.Synced
	if d := cmp.Diff(types.RepoSyncDiff{}, diff, ignore); d != "" {
		t.Fatalf("initial Synced mismatch (-want +got):\n%s", d)
	}

	// we poll, so lets set an aggressive deadline
	deadline := time.Now().Add(10 * time.Second)
	if tDeadline, ok := t.Deadline(); ok && tDeadline.Before(deadline) {
		// give time to report errors
		deadline = tDeadline.Add(-100 * time.Millisecond)
	}

	// it should add a job for all external services
	var jobCount int
	for time.Now().Before(deadline) {
		q := sqlf.Sprintf("SELECT COUNT(*) FROM external_service_sync_jobs")
		if err := store.Handle().QueryRowContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...).Scan(&jobCount); err != nil {
			t.Fatal(err)
		}
		if jobCount == len(services) {
			break
		}
		// We need to give the worker package time to create the jobs
		time.Sleep(10 * time.Millisecond)
	}
	if jobCount != len(services) {
		t.Fatalf("expected %d sync jobs, got %d", len(services), jobCount)
	}

	for range len(services) * 10 {
		diff := <-syncer.Synced

		if len(diff.Added) != 1 {
			t.Fatalf("Expected 1 Added repos. got %d", len(diff.Added))
		}
		if len(diff.Deleted) != 0 {
			t.Fatalf("Expected 0 Deleted repos. got %d", len(diff.Added))
		}
		if len(diff.Modified) != 0 {
			t.Fatalf("Expected 0 Modified repos. got %d", len(diff.Added))
		}
		if len(diff.Unmodified) != 0 {
			t.Fatalf("Expected 0 Unmodified repos. got %d", len(diff.Added))
		}
	}

	var jobsCompleted int
	for time.Now().Before(deadline) {
		q := sqlf.Sprintf("SELECT COUNT(*) FROM external_service_sync_jobs where state = 'completed'")
		if err := store.Handle().QueryRowContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...).Scan(&jobsCompleted); err != nil {
			t.Fatal(err)
		}
		if jobsCompleted == len(services) {
			break
		}
		// We need to give the worker package time to create the jobs
		time.Sleep(10 * time.Millisecond)
	}

	// Cancel context and the run loop should stop
	cancel()
	<-done
}

func TestOrphanedRepo(t *testing.T) {
	t.Parallel()
	store := getTestRepoStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now := time.Now()

	svc1 := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test1",
		Config:      extsvc.NewUnencryptedConfig(basicGitHubConfig),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	svc2 := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test2",
		Config:      extsvc.NewUnencryptedConfig(basicGitHubConfig),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// setup services
	if err := store.ExternalServiceStore().Upsert(ctx, svc1, svc2); err != nil {
		t.Fatal(err)
	}

	githubRepo := &types.Repo{
		Name:     "github.com/org/foo",
		Metadata: &github.Repository{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "foo-external-12345",
			ServiceID:   "https://github.com/",
			ServiceType: extsvc.TypeGitHub,
		},
	}

	// Add two services, both pointing at the same repo

	// Sync first service
	syncer := &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
			s := repos.NewFakeSource(svc1, nil, githubRepo)
			return s, nil
		},
		Store: store,
		Now:   time.Now,
	}
	if err := syncer.SyncExternalService(ctx, svc1.ID, 10*time.Second, noopProgressRecorder); err != nil {
		t.Fatal(err)
	}

	// Sync second service
	syncer.Sourcer = func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
		s := repos.NewFakeSource(svc2, nil, githubRepo)
		return s, nil
	}
	if err := syncer.SyncExternalService(ctx, svc2.ID, 10*time.Second, noopProgressRecorder); err != nil {
		t.Fatal(err)
	}

	// Confirm that there are two relationships
	assertSourceCount(ctx, t, store, 2)

	// We should have no deleted repos
	assertDeletedRepoCount(ctx, t, store, 0)

	// Remove the repo from one service and sync again
	syncer.Sourcer = func(ctx context.Context, services *types.ExternalService) (repos.Source, error) {
		s := repos.NewFakeSource(svc1, nil)
		return s, nil
	}
	if err := syncer.SyncExternalService(ctx, svc1.ID, 10*time.Second, noopProgressRecorder); err != nil {
		t.Fatal(err)
	}

	// Confirm that the repository hasn't been deleted
	rs, err := store.RepoStore().List(ctx, database.ReposListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 {
		t.Fatalf("Expected 1 repo, got %d", len(rs))
	}

	// Confirm that there is one relationship
	assertSourceCount(ctx, t, store, 1)

	// We should have no deleted repos
	assertDeletedRepoCount(ctx, t, store, 0)

	// Remove the repo from the second service and sync again
	syncer.Sourcer = func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
		s := repos.NewFakeSource(svc2, nil)
		return s, nil
	}
	if err := syncer.SyncExternalService(ctx, svc2.ID, 10*time.Second, noopProgressRecorder); err != nil {
		t.Fatal(err)
	}

	// Confirm that there no relationships
	assertSourceCount(ctx, t, store, 0)

	// We should have one deleted repo
	assertDeletedRepoCount(ctx, t, store, 1)
}

func TestCloudDefaultExternalServicesDontSync(t *testing.T) {
	t.Parallel()
	store := getTestRepoStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now := time.Now()

	svc1 := &types.ExternalService{
		Kind:         extsvc.KindGitHub,
		DisplayName:  "Github - Test1",
		Config:       extsvc.NewUnencryptedConfig(basicGitHubConfig),
		CloudDefault: true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// setup services
	if err := store.ExternalServiceStore().Upsert(ctx, svc1); err != nil {
		t.Fatal(err)
	}

	githubRepo := &types.Repo{
		Name:     "github.com/org/foo",
		Metadata: &github.Repository{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "foo-external-12345",
			ServiceID:   "https://github.com/",
			ServiceType: extsvc.TypeGitHub,
		},
	}

	syncer := &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
			s := repos.NewFakeSource(svc1, nil, githubRepo)
			return s, nil
		},
		Store: store,
		Now:   time.Now,
	}

	have := syncer.SyncExternalService(ctx, svc1.ID, 10*time.Second, noopProgressRecorder)
	want := repos.ErrCloudDefaultSync

	if !errors.Is(have, want) {
		t.Fatalf("have err: %v, want %v", have, want)
	}
}

func TestDotComPrivateReposDontSync(t *testing.T) {
	dotcom.MockSourcegraphDotComMode(t, true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := getTestRepoStore(t)

	now := time.Now()

	svc1 := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test1",
		Config:      extsvc.NewUnencryptedConfig(basicGitHubConfig),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// setup services
	if err := store.ExternalServiceStore().Upsert(ctx, svc1); err != nil {
		t.Fatal(err)
	}

	privateRepo := &types.Repo{
		Name:    "github.com/org/foo",
		Private: true,
	}

	syncer := &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
			s := repos.NewFakeSource(svc1, nil, privateRepo)
			return s, nil
		},
		Store: store,
		Now:   time.Now,
	}

	have := syncer.SyncExternalService(ctx, svc1.ID, 10*time.Second, noopProgressRecorder)
	errorMsg := fmt.Sprintf("%s is private, but dotcom does not support private repositories.", string(privateRepo.Name))

	require.EqualError(t, have, errorMsg)
}

var basicGitHubConfig = `{"url": "https://github.com", "token": "beef", "repos": ["owner/name"]}`

func TestConflictingSyncers(t *testing.T) {
	t.Parallel()
	store := getTestRepoStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now := time.Now()

	svc1 := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test1",
		Config:      extsvc.NewUnencryptedConfig(basicGitHubConfig),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	svc2 := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test2",
		Config:      extsvc.NewUnencryptedConfig(basicGitHubConfig),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// setup services
	if err := store.ExternalServiceStore().Upsert(ctx, svc1, svc2); err != nil {
		t.Fatal(err)
	}

	githubRepo := &types.Repo{
		Name:     "github.com/org/foo",
		Metadata: &github.Repository{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "foo-external-12345",
			ServiceID:   "https://github.com/",
			ServiceType: extsvc.TypeGitHub,
		},
	}

	// Add two services, both pointing at the same repo

	// Sync first service
	syncer := &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
			s := repos.NewFakeSource(svc1, nil, githubRepo)
			return s, nil
		},
		Store: store,
		Now:   time.Now,
	}
	if err := syncer.SyncExternalService(ctx, svc1.ID, 10*time.Second, noopProgressRecorder); err != nil {
		t.Fatal(err)
	}

	// Sync second service
	syncer = &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
			s := repos.NewFakeSource(svc2, nil, githubRepo)
			return s, nil
		},
		Store: store,
		Now:   time.Now,
	}
	if err := syncer.SyncExternalService(ctx, svc2.ID, 10*time.Second, noopProgressRecorder); err != nil {
		t.Fatal(err)
	}

	// Confirm that there are two relationships
	assertSourceCount(ctx, t, store, 2)

	fromDB, err := store.RepoStore().List(ctx, database.ReposListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(fromDB) != 1 {
		t.Fatalf("Expected 1 repo, got %d", len(fromDB))
	}
	beforeUpdate := fromDB[0]
	if beforeUpdate.Description != "" {
		t.Fatalf("Expected %q, got %q", "", beforeUpdate.Description)
	}

	// Create two transactions
	tx1, err := store.Transact(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tx2, err := store.Transact(ctx)
	if err != nil {
		t.Fatal(err)
	}

	newDescription := "This has changed"
	updatedRepo := githubRepo.With(func(r *types.Repo) {
		r.Description = newDescription
	})

	// Start syncing using tx1
	syncer = &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
			s := repos.NewFakeSource(svc1, nil, updatedRepo)
			return s, nil
		},
		Store: tx1,
		Now:   time.Now,
	}
	if err := syncer.SyncExternalService(ctx, svc1.ID, 10*time.Second, noopProgressRecorder); err != nil {
		t.Fatal(err)
	}

	syncer2 := &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
			s := repos.NewFakeSource(svc2, nil, githubRepo.With(func(r *types.Repo) {
				r.Description = newDescription
			}))
			return s, nil
		},
		Store:  tx2,
		Synced: make(chan types.RepoSyncDiff, 2),
		Now:    time.Now,
	}

	errChan := make(chan error)
	go func() {
		errChan <- syncer2.SyncExternalService(ctx, svc2.ID, 10*time.Second, noopProgressRecorder)
	}()

	tx1.Done(nil)

	if err = <-errChan; err != nil {
		t.Fatalf("syncer2 err: %v", err)
	}

	diff := <-syncer2.Synced
	if have, want := diff.Repos().Names(), []string{string(updatedRepo.Name)}; !cmp.Equal(want, have) {
		t.Fatalf("syncer2 Synced mismatch: (-want, +have): %s", cmp.Diff(want, have))
	}

	tx2.Done(nil)

	fromDB, err = store.RepoStore().List(ctx, database.ReposListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(fromDB) != 1 {
		t.Fatalf("Expected 1 repo, got %d", len(fromDB))
	}
	afterUpdate := fromDB[0]
	if afterUpdate.Description != newDescription {
		t.Fatalf("Expected %q, got %q", newDescription, afterUpdate.Description)
	}
}

// Test that sync repo does not clear out any other repo relationships
func TestSyncRepoMaintainsOtherSources(t *testing.T) {
	t.Parallel()
	store := getTestRepoStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now := time.Now()

	svc1 := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test1",
		Config:      extsvc.NewUnencryptedConfig(basicGitHubConfig),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	svc2 := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test2",
		Config:      extsvc.NewUnencryptedConfig(basicGitHubConfig),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// setup services
	if err := store.ExternalServiceStore().Upsert(ctx, svc1, svc2); err != nil {
		t.Fatal(err)
	}

	githubRepo := &types.Repo{
		Name:     "github.com/org/foo",
		Metadata: &github.Repository{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "foo-external-12345",
			ServiceID:   "https://github.com/",
			ServiceType: extsvc.TypeGitHub,
		},
	}

	// Add two services, both pointing at the same repo

	// Sync first service
	syncer := &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
			s := repos.NewFakeSource(svc1, nil, githubRepo)
			return s, nil
		},
		Store: store,
		Now:   time.Now,
	}
	if err := syncer.SyncExternalService(ctx, svc1.ID, 10*time.Second, noopProgressRecorder); err != nil {
		t.Fatal(err)
	}

	// Sync second service
	syncer = &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
			s := repos.NewFakeSource(svc2, nil, githubRepo)
			return s, nil
		},
		Store: store,
		Now:   time.Now,
	}
	if err := syncer.SyncExternalService(ctx, svc2.ID, 10*time.Second, noopProgressRecorder); err != nil {
		t.Fatal(err)
	}

	// Confirm that there are two relationships
	assertSourceCount(ctx, t, store, 2)

	// Run syncRepo with only one source
	urn := extsvc.URN(extsvc.KindGitHub, svc1.ID)
	githubRepo.Sources = map[string]*types.SourceInfo{
		urn: {
			ID:       urn,
			CloneURL: "cloneURL",
		},
	}
	_, err := syncer.SyncRepo(ctx, githubRepo.Name, true)
	if err != nil {
		t.Fatal(err)
	}

	// We should still have two sources
	assertSourceCount(ctx, t, store, 2)
}

func TestNameOnConflictOnRename(t *testing.T) {
	// Test the case where more than one external service returns the same name for different repos. The names
	// are the same, but the external id are different.
	t.Parallel()
	store := getTestRepoStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now := time.Now()

	svc1 := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test1",
		Config:      extsvc.NewUnencryptedConfig(basicGitHubConfig),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	svc2 := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test2",
		Config:      extsvc.NewUnencryptedConfig(basicGitHubConfig),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// setup services
	if err := store.ExternalServiceStore().Upsert(ctx, svc1, svc2); err != nil {
		t.Fatal(err)
	}

	githubRepo1 := &types.Repo{
		Name:     "github.com/org/foo",
		Metadata: &github.Repository{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "foo-external-foo",
			ServiceID:   "https://github.com/",
			ServiceType: extsvc.TypeGitHub,
		},
	}

	githubRepo2 := &types.Repo{
		Name:     "github.com/org/bar",
		Metadata: &github.Repository{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "foo-external-bar",
			ServiceID:   "https://github.com/",
			ServiceType: extsvc.TypeGitHub,
		},
	}

	// Add two services, one with each repo

	// Sync first service
	syncer := &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
			s := repos.NewFakeSource(svc1, nil, githubRepo1)
			return s, nil
		},
		Store: store,
		Now:   time.Now,
	}
	if err := syncer.SyncExternalService(ctx, svc1.ID, 10*time.Second, noopProgressRecorder); err != nil {
		t.Fatal(err)
	}

	// Sync second service
	syncer = &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(context.Context, *types.ExternalService) (repos.Source, error) {
			s := repos.NewFakeSource(svc2, nil, githubRepo2)
			return s, nil
		},
		Store: store,
		Now:   time.Now,
	}
	if err := syncer.SyncExternalService(ctx, svc2.ID, 10*time.Second, noopProgressRecorder); err != nil {
		t.Fatal(err)
	}

	// Rename repo1 with the same name as repo2
	renamedRepo1 := githubRepo1.With(func(r *types.Repo) {
		r.Name = githubRepo2.Name
	})

	// Sync first service
	syncer = &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(context.Context, *types.ExternalService) (repos.Source, error) {
			s := repos.NewFakeSource(svc1, nil, renamedRepo1)
			return s, nil
		},
		Store: store,
		Now:   time.Now,
	}
	if err := syncer.SyncExternalService(ctx, svc1.ID, 10*time.Second, noopProgressRecorder); err != nil {
		t.Fatal(err)
	}

	fromDB, err := store.RepoStore().List(ctx, database.ReposListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(fromDB) != 1 {
		t.Fatalf("Expected 1 repo, have %d", len(fromDB))
	}

	found := fromDB[0]
	// We expect repo2 to be synced since we always pick the just sourced repo as the winner, deleting the other.
	// If the existing conflicting repo still exists, it'll have a different name (because names are unique in
	// the code host), so it'll get re-created once we sync it later.
	expectedID := "foo-external-foo"

	if found.ExternalRepo.ID != expectedID {
		t.Fatalf("Want %q, got %q", expectedID, found.ExternalRepo.ID)
	}
}

func TestDeleteExternalService(t *testing.T) {
	t.Parallel()
	store := getTestRepoStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now := time.Now()

	svc1 := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test1",
		Config:      extsvc.NewUnencryptedConfig(basicGitHubConfig),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	svc2 := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test2",
		Config:      extsvc.NewUnencryptedConfig(basicGitHubConfig),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// setup services
	if err := store.ExternalServiceStore().Upsert(ctx, svc1, svc2); err != nil {
		t.Fatal(err)
	}

	githubRepo := &types.Repo{
		Name:     "github.com/org/foo",
		Metadata: &github.Repository{},
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "foo-external-12345",
			ServiceID:   "https://github.com/",
			ServiceType: extsvc.TypeGitHub,
		},
	}

	// Add two services, both pointing at the same repo

	// Sync first service
	syncer := &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
			s := repos.NewFakeSource(svc1, nil, githubRepo)
			return s, nil
		},
		Store: store,
		Now:   time.Now,
	}
	if err := syncer.SyncExternalService(ctx, svc1.ID, 10*time.Second, noopProgressRecorder); err != nil {
		t.Fatal(err)
	}

	// Sync second service
	syncer = &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Sourcer: func(ctx context.Context, service *types.ExternalService) (repos.Source, error) {
			s := repos.NewFakeSource(svc2, nil, githubRepo)
			return s, nil
		},
		Store: store,
		Now:   time.Now,
	}
	if err := syncer.SyncExternalService(ctx, svc2.ID, 10*time.Second, noopProgressRecorder); err != nil {
		t.Fatal(err)
	}

	// Delete the first service
	if err := store.ExternalServiceStore().Delete(ctx, svc1.ID); err != nil {
		t.Fatal(err)
	}

	// Confirm that there is one relationship
	assertSourceCount(ctx, t, store, 1)

	// We should have no deleted repos
	assertDeletedRepoCount(ctx, t, store, 0)

	// Delete the second service
	if err := store.ExternalServiceStore().Delete(ctx, svc2.ID); err != nil {
		t.Fatal(err)
	}

	// Confirm that there no relationships
	assertSourceCount(ctx, t, store, 0)

	// We should have one deleted repo
	assertDeletedRepoCount(ctx, t, store, 1)
}

func assertSourceCount(ctx context.Context, t *testing.T, store repos.Store, want int) {
	t.Helper()
	var rowCount int
	q := sqlf.Sprintf("SELECT COUNT(*) FROM external_service_repos")
	if err := store.Handle().QueryRowContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...).Scan(&rowCount); err != nil {
		t.Fatal(err)
	}
	if rowCount != want {
		t.Fatalf("Expected %d rows, got %d", want, rowCount)
	}
}

func assertDeletedRepoCount(ctx context.Context, t *testing.T, store repos.Store, want int) {
	t.Helper()
	var rowCount int
	q := sqlf.Sprintf("SELECT COUNT(*) FROM repo where deleted_at is not null")
	if err := store.Handle().QueryRowContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...).Scan(&rowCount); err != nil {
		t.Fatal(err)
	}
	if rowCount != want {
		t.Fatalf("Expected %d rows, got %d", want, rowCount)
	}
}

func TestSyncReposWithLastErrors(t *testing.T) {
	t.Parallel()
	store := getTestRepoStore(t)

	ctx := context.Background()
	testCases := []struct {
		label     string
		svcKind   string
		repoName  api.RepoName
		config    string
		extSvcErr error
		serviceID string
	}{
		{
			label:     "github test",
			svcKind:   extsvc.KindGitHub,
			repoName:  api.RepoName("github.com/foo/bar"),
			config:    `{"url": "https://github.com", "repositoryQuery": ["none"], "token": "abc"}`,
			extSvcErr: github.ErrRepoNotFound,
			serviceID: "https://github.com/",
		},
		{
			label:     "gitlab test",
			svcKind:   extsvc.KindGitLab,
			repoName:  api.RepoName("gitlab.com/foo/bar"),
			config:    `{"url": "https://gitlab.com", "projectQuery": ["none"], "token": "abc"}`,
			extSvcErr: gitlab.ProjectNotFoundError{Name: "/foo/bar"},
			serviceID: "https://gitlab.com/",
		},
	}

	for i, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			syncer, dbRepos := setupSyncErroredTest(ctx, store, t, tc.svcKind,
				tc.extSvcErr, tc.config, tc.serviceID, tc.repoName)
			if len(dbRepos) != 1 {
				t.Fatalf("should've inserted exactly 1 repo in the db for testing, got %d instead", len(dbRepos))
			}

			// Run the syncer, which should find the repo with non-empty last_error and delete it
			err := syncer.SyncReposWithLastErrors(ctx, ratelimit.NewInstrumentedLimiter("TestSyncRepos", rate.NewLimiter(200, 1)))
			if err != nil {
				t.Fatalf("unexpected error running SyncReposWithLastErrors: %s", err)
			}

			diff := <-syncer.Synced

			deleted := types.Repos{&types.Repo{ID: dbRepos[0].ID}}
			if d := cmp.Diff(types.RepoSyncDiff{Deleted: deleted}, diff); d != "" {
				t.Fatalf("Deleted mismatch (-want +got):\n%s", d)
			}

			// each iteration will result in one more deleted repo.
			assertDeletedRepoCount(ctx, t, store, i+1)
			// Try to fetch the repo to verify that it was deleted by the syncer
			myRepo, err := store.RepoStore().GetByName(ctx, tc.repoName)
			if err == nil {
				t.Fatalf("repo should've been deleted. expected a repo not found error")
			}
			if !errors.Is(err, &database.RepoNotFoundErr{Name: tc.repoName}) {
				t.Fatalf("expected a RepoNotFound error, got %s", err)
			}
			if myRepo != nil {
				t.Fatalf("repo should've been deleted: %v", myRepo)
			}
		})
	}
}

func TestSyncReposWithLastErrorsHitsRateLimiter(t *testing.T) {
	t.Parallel()
	store := getTestRepoStore(t)

	ctx := context.Background()
	repoNames := []api.RepoName{
		"github.com/asdf/jkl",
		"github.com/foo/bar",
	}
	syncer, _ := setupSyncErroredTest(ctx, store, t, extsvc.KindGitLab, github.ErrRepoNotFound, `{"url": "https://github.com", "projectQuery": ["none"], "token": "abc"}`, "https://gitlab.com/", repoNames...)

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	// Run the syncer, which should return an error due to hitting the rate limit
	err := syncer.SyncReposWithLastErrors(ctx, ratelimit.NewInstrumentedLimiter("TestSyncRepos", rate.NewLimiter(1, 1)))
	if err == nil {
		t.Fatal("SyncReposWithLastErrors should've returned an error due to hitting rate limit")
	}
	if !strings.Contains(err.Error(), "error waiting for rate limiter: rate: Wait(n=1) would exceed context deadline") {
		t.Fatalf("expected an error from rate limiting, got %s instead", err)
	}
}

func setupSyncErroredTest(ctx context.Context, s repos.Store, t *testing.T,
	serviceType string, externalSvcError error, config, serviceID string, repoNames ...api.RepoName,
) (*repos.Syncer, types.Repos) {
	t.Helper()
	now := time.Now()
	dbRepos := types.Repos{}
	service := types.ExternalService{
		Kind:         serviceType,
		DisplayName:  fmt.Sprintf("%s - Test", serviceType),
		Config:       extsvc.NewUnencryptedConfig(config),
		CreatedAt:    now,
		UpdatedAt:    now,
		CloudDefault: true,
	}

	// Create a new external service
	confGet := func() *conf.Unified {
		return &conf.Unified{}
	}

	err := s.ExternalServiceStore().Create(ctx, confGet, &service)
	if err != nil {
		t.Fatal(err)
	}

	for _, repoName := range repoNames {
		dbRepo := (&types.Repo{
			Name:        repoName,
			Description: "",
			ExternalRepo: api.ExternalRepoSpec{
				ID:          fmt.Sprintf("external-%s", repoName), // TODO: make this something else?
				ServiceID:   serviceID,
				ServiceType: serviceType,
			},
		}).With(typestest.Opt.RepoSources(service.URN()))
		// Insert the repo into our database
		if err := s.RepoStore().Create(ctx, dbRepo); err != nil {
			t.Fatal(err)
		}
		// Log a failure in gitserver_repos for this repo
		if err := s.GitserverReposStore().Update(ctx, &types.GitserverRepo{
			RepoID:      dbRepo.ID,
			ShardID:     "test",
			CloneStatus: types.CloneStatusCloned,
			LastError:   "error fetching repo: Not found",
		}); err != nil {
			t.Fatal(err)
		}
		// Validate that the repo exists and we can fetch it
		_, err := s.RepoStore().GetByName(ctx, dbRepo.Name)
		if err != nil {
			t.Fatal(err)
		}
		dbRepos = append(dbRepos, dbRepo)
	}

	syncer := &repos.Syncer{
		ObsvCtx: observation.TestContextTB(t),
		Now:     time.Now,
		Store:   s,
		Synced:  make(chan types.RepoSyncDiff, 1),
		Sourcer: repos.NewFakeSourcer(
			nil,
			repos.NewFakeSource(&service,
				externalSvcError,
				dbRepos...),
		),
	}
	return syncer, dbRepos
}

var noopProgressRecorder = func(ctx context.Context, progress repos.SyncProgress, final bool) error {
	return nil
}

func TestCreateRepoLicenseHook(t *testing.T) {
	ctx := context.Background()

	// Set up mock repo count
	mockRepoStore := dbmocks.NewMockRepoStore()
	mockStore := repos.NewMockStore()
	mockStore.RepoStoreFunc.SetDefaultReturn(mockRepoStore)

	tests := map[string]struct {
		maxPrivateRepos int
		unrestricted    bool
		numPrivateRepos int
		newRepo         *types.Repo
		wantErr         bool
	}{
		"private repo, unrestricted": {
			unrestricted:    true,
			numPrivateRepos: 99999999,
			newRepo:         &types.Repo{Private: true},
			wantErr:         false,
		},
		"private repo, max private repos reached": {
			maxPrivateRepos: 1,
			numPrivateRepos: 1,
			newRepo:         &types.Repo{Private: true},
			wantErr:         true,
		},
		"public repo, max private repos reached": {
			maxPrivateRepos: 1,
			numPrivateRepos: 1,
			newRepo:         &types.Repo{Private: false},
			wantErr:         false,
		},
		"private repo, max private repos not reached": {
			maxPrivateRepos: 2,
			numPrivateRepos: 1,
			newRepo:         &types.Repo{Private: true},
			wantErr:         false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mockRepoStore.CountFunc.SetDefaultReturn(test.numPrivateRepos, nil)

			defaultMock := licensing.MockCheckFeature
			licensing.MockCheckFeature = func(feature licensing.Feature) error {
				if prFeature, ok := feature.(*licensing.FeaturePrivateRepositories); ok {
					prFeature.MaxNumPrivateRepos = test.maxPrivateRepos
					prFeature.Unrestricted = test.unrestricted
				}

				return nil
			}
			defer func() {
				licensing.MockCheckFeature = defaultMock
			}()

			err := repos.CreateRepoLicenseHook(ctx, mockStore, test.newRepo)
			if gotErr := err != nil; gotErr != test.wantErr {
				t.Fatalf("got err: %t, want err: %t, err: %q", gotErr, test.wantErr, err)
			}
		})
	}
}

func TestUpdateRepoLicenseHook(t *testing.T) {
	ctx := context.Background()

	// Set up mock repo count
	mockRepoStore := dbmocks.NewMockRepoStore()
	mockStore := repos.NewMockStore()
	mockStore.RepoStoreFunc.SetDefaultReturn(mockRepoStore)

	tests := map[string]struct {
		maxPrivateRepos int
		unrestricted    bool
		numPrivateRepos int
		existingRepo    *types.Repo
		newRepo         *types.Repo
		wantErr         bool
	}{
		"from public to private, unrestricted": {
			unrestricted:    true,
			numPrivateRepos: 99999999,
			existingRepo:    &types.Repo{Private: false},
			newRepo:         &types.Repo{Private: true},
			wantErr:         false,
		},
		"from public to private, max private repos reached": {
			maxPrivateRepos: 1,
			numPrivateRepos: 1,
			existingRepo:    &types.Repo{Private: false},
			newRepo:         &types.Repo{Private: true},
			wantErr:         true,
		},
		"from private to private, max private repos reached": {
			maxPrivateRepos: 1,
			numPrivateRepos: 1,
			existingRepo:    &types.Repo{Private: true},
			newRepo:         &types.Repo{Private: true},
			wantErr:         false,
		},
		"from private to public, max private repos reached": {
			maxPrivateRepos: 1,
			numPrivateRepos: 1,
			existingRepo:    &types.Repo{Private: true},
			newRepo:         &types.Repo{Private: false},
			wantErr:         false,
		},
		"from private deleted to private not deleted, max private repos reached": {
			maxPrivateRepos: 1,
			numPrivateRepos: 1,
			existingRepo:    &types.Repo{Private: true, DeletedAt: time.Now()},
			newRepo:         &types.Repo{Private: true, DeletedAt: time.Time{}},
			wantErr:         true,
		},
		"from public to private, max private repos not reached": {
			maxPrivateRepos: 2,
			numPrivateRepos: 1,
			existingRepo:    &types.Repo{Private: false},
			newRepo:         &types.Repo{Private: true},
			wantErr:         false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mockRepoStore.CountFunc.SetDefaultReturn(test.numPrivateRepos, nil)

			defaultMock := licensing.MockCheckFeature
			licensing.MockCheckFeature = func(feature licensing.Feature) error {
				if prFeature, ok := feature.(*licensing.FeaturePrivateRepositories); ok {
					prFeature.MaxNumPrivateRepos = test.maxPrivateRepos
					prFeature.Unrestricted = test.unrestricted
				}

				return nil
			}
			defer func() {
				licensing.MockCheckFeature = defaultMock
			}()

			err := repos.UpdateRepoLicenseHook(ctx, mockStore, test.existingRepo, test.newRepo)
			if gotErr := err != nil; gotErr != test.wantErr {
				t.Fatalf("got err: %t, want err: %t, err: %q", gotErr, test.wantErr, err)
			}
		})
	}
}
