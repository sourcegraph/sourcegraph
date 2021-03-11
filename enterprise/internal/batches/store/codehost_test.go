package store

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func testStoreCodeHost(t *testing.T, ctx context.Context, s *Store, clock ct.Clock) {
	rs := database.ReposWith(s.Store)
	es := database.ExternalServicesWith(s.Store)

	repo := ct.TestRepo(t, es, extsvc.KindGitHub)
	otherRepo := ct.TestRepo(t, es, extsvc.KindGitHub)

	gh, _ := ct.CreateGitHubSSHTestRepos(t, ctx, s.DB(), 1)
	bbs, _ := ct.CreateBbsSSHTestRepos(t, ctx, s.DB(), 1)
	sshRepos := []*types.Repo{gh[0], bbs[0]}

	gitlabRepo := ct.TestRepo(t, es, extsvc.KindGitLab)
	bitbucketRepo := ct.TestRepo(t, es, extsvc.KindBitbucketServer)
	awsRepo := ct.TestRepo(t, es, extsvc.KindAWSCodeCommit)

	if err := rs.Create(ctx, repo, otherRepo, gitlabRepo, bitbucketRepo, awsRepo); err != nil {
		t.Fatal(err)
	}
	deletedRepo := otherRepo.With(types.Opt.RepoDeletedAt(clock.Now()))
	if err := rs.Delete(ctx, deletedRepo.ID); err != nil {
		t.Fatal(err)
	}

	t.Run("ListCodeHosts", func(t *testing.T) {
		t.Run("List all", func(t *testing.T) {
			have, err := s.ListCodeHosts(ctx, ListCodeHostsOpts{})
			if err != nil {
				t.Fatal(err)
			}
			want := []*batches.CodeHost{
				{
					ExternalServiceType: extsvc.TypeBitbucketServer,
					ExternalServiceID:   "https://bitbucketserver.com/",
					RequiresSSH:         true,
				},
				{
					ExternalServiceType: extsvc.TypeGitHub,
					ExternalServiceID:   "https://github.com/",
					RequiresSSH:         true,
				},
				{
					ExternalServiceType: extsvc.TypeGitLab,
					ExternalServiceID:   "https://gitlab.com/",
				},
			}
			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatalf("Invalid code hosts returned. %s", diff)
			}
		})
		t.Run("By RepoIDs", func(t *testing.T) {
			have, err := s.ListCodeHosts(ctx, ListCodeHostsOpts{RepoIDs: []api.RepoID{repo.ID}})
			if err != nil {
				t.Fatal(err)
			}
			want := []*batches.CodeHost{
				{
					ExternalServiceType: extsvc.TypeGitHub,
					ExternalServiceID:   "https://github.com/",
				},
			}
			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatalf("Invalid code hosts returned. %s", diff)
			}

		})
	})

	t.Run("GetExternalServiceID", func(t *testing.T) {
		for _, repo := range []*types.Repo{repo, otherRepo, gitlabRepo, bitbucketRepo, sshRepos[0], sshRepos[1]} {
			id, err := s.GetExternalServiceID(ctx, GetExternalServiceIDOpts{
				ExternalServiceType: repo.ExternalRepo.ServiceType,
				ExternalServiceID:   repo.ExternalRepo.ServiceID,
			})
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			// We fetch the ExternalService and make sure that Type and URL match
			extSvc, err := es.GetByID(ctx, id)
			if err != nil {
				if errcode.IsNotFound(err) {
					t.Fatalf("external service %d not found", id)
				}

				t.Fatalf("unexpected error: %s", err)
			}
			if have, want := extSvc.Kind, extsvc.TypeToKind(repo.ExternalRepo.ServiceType); have != want {
				t.Fatalf("wrong external service kind. want=%q, have=%q", want, have)
			}
		}
	})
}
