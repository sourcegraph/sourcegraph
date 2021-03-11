package bitbucketserver

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

var update = flag.Bool("update", false, "update testdata")

func TestProvider_Validate(t *testing.T) {
	instanceURL := os.Getenv("BITBUCKET_SERVER_URL")
	if instanceURL == "" {
		instanceURL = "https://bitbucket.sgdev.org"
	}

	for _, tc := range []struct {
		name     string
		client   func(*bitbucketserver.Client)
		problems []string
	}{
		{
			name: "no-problems-when-authenticated-as-admin",
		},
		{
			name:   "problems-when-authenticated-as-non-admin",
			client: func(c *bitbucketserver.Client) { c.Auth = &auth.BasicAuth{} },
			problems: []string{
				`Bitbucket API HTTP error: code=401 url="${INSTANCEURL}/rest/api/1.0/admin/permissions/users?filter=" body="{\"errors\":[{\"context\":null,\"message\":\"You are not permitted to access this resource\",\"exceptionName\":\"com.atlassian.bitbucket.AuthorisationException\"}]}"`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cli, save := newClient(t, "Validate/"+tc.name)
			defer save()

			p := newProvider(cli)

			if tc.client != nil {
				tc.client(p.client)
			}

			for i := range tc.problems {
				tc.problems[i] = strings.ReplaceAll(tc.problems[i], "${INSTANCEURL}", instanceURL)
			}

			problems := p.Validate()
			if have, want := problems, tc.problems; !reflect.DeepEqual(have, want) {
				t.Error(cmp.Diff(have, want))
			}
		})
	}
}

func testProviderFetchAccount(f *fixtures, cli *bitbucketserver.Client) func(*testing.T) {
	return func(t *testing.T) {
		p := newProvider(cli)

		h := codeHost{CodeHost: p.codeHost}

		for _, tc := range []struct {
			name string
			ctx  context.Context
			user *types.User
			acct *extsvc.Account
			err  string
		}{
			{
				name: "no user given",
				user: nil,
				acct: nil,
			},
			{
				name: "user not found",
				user: &types.User{Username: "john"},
				acct: nil,
				err:  `no results returned by the Bitbucket Server API`,
			},
			{
				name: "user found by exact username match",
				user: &types.User{ID: 42, Username: "ceo"},
				acct: h.externalAccount(42, f.users["ceo"]),
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if tc.ctx == nil {
					tc.ctx = context.Background()
				}

				if tc.err == "" {
					tc.err = "<nil>"
				}

				acct, err := p.FetchAccount(tc.ctx, tc.user, nil)

				if have, want := fmt.Sprint(err), tc.err; have != want {
					t.Errorf("error:\nhave: %q\nwant: %q", have, want)
				}

				if have, want := acct, tc.acct; !reflect.DeepEqual(have, want) {
					t.Error(cmp.Diff(have, want))
				}
			})
		}
	}
}

func testProviderFetchUserPerms(f *fixtures, cli *bitbucketserver.Client) func(*testing.T) {
	return func(t *testing.T) {
		p := newProvider(cli)

		h := codeHost{CodeHost: p.codeHost}

		repoIDs := func(names ...string) (ids []extsvc.RepoID) {
			for _, name := range names {
				if r, ok := f.repos[name]; ok {
					ids = append(ids, extsvc.RepoID(strconv.FormatInt(int64(r.ID), 10)))
				}
			}
			return ids
		}

		for _, tc := range []struct {
			name string
			ctx  context.Context
			acct *extsvc.Account
			ids  []extsvc.RepoID
			err  string
		}{
			{
				name: "no account provided",
				acct: nil,
				err:  "no account provided",
			},
			{
				name: "no account data provided",
				acct: &extsvc.Account{},
				err:  "no account data provided",
			},
			{
				name: "not a code host of the account",
				acct: &extsvc.Account{
					AccountSpec: extsvc.AccountSpec{
						ServiceType: extsvc.TypeGitHub,
						ServiceID:   "https://github.com",
						AccountID:   "john",
					},
					AccountData: extsvc.AccountData{
						Data: new(json.RawMessage),
					},
				},
				err: `not a code host of the account: want "${INSTANCEURL}" but have "https://github.com"`,
			},
			{
				name: "bad account data",
				acct: &extsvc.Account{
					AccountSpec: extsvc.AccountSpec{
						ServiceType: h.ServiceType,
						ServiceID:   h.ServiceID,
						AccountID:   "john",
					},
					AccountData: extsvc.AccountData{
						Data: new(json.RawMessage),
					},
				},
				err: "unmarshaling account data: unexpected end of JSON input",
			},
			{
				name: "private repo ids are retrieved",
				acct: h.externalAccount(0, f.users["ceo"]),
				ids:  repoIDs("private-repo", "secret-repo", "super-secret-repo"),
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if tc.ctx == nil {
					tc.ctx = context.Background()
				}

				if tc.err == "" {
					tc.err = "<nil>"
				}

				tc.err = strings.ReplaceAll(tc.err, "${INSTANCEURL}", cli.URL.String())

				got, err := p.FetchUserPerms(tc.ctx, tc.acct)
				if have, want := fmt.Sprint(err), tc.err; have != want {
					t.Errorf("error:\nhave: %q\nwant: %q", have, want)
				}
				if got != nil {
					sort.Slice(got.Exacts, func(i, j int) bool { return got.Exacts[i] < got.Exacts[j] })
				}

				var want *authz.ExternalUserPermissions
				if len(tc.ids) > 0 {
					sort.Slice(tc.ids, func(i, j int) bool { return tc.ids[i] < tc.ids[j] })
					want = &authz.ExternalUserPermissions{
						Exacts: tc.ids,
					}
				}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Fatalf("Mismatch (-want +got):\n%s", diff)
				}
			})
		}
	}
}

func testProviderFetchRepoPerms(f *fixtures, cli *bitbucketserver.Client) func(*testing.T) {
	return func(t *testing.T) {
		p := newProvider(cli)

		h := codeHost{CodeHost: p.codeHost}

		userIDs := func(names ...string) (ids []extsvc.AccountID) {
			for _, name := range names {
				if r, ok := f.users[name]; ok {
					ids = append(ids, extsvc.AccountID(strconv.FormatInt(int64(r.ID), 10)))
				}
			}
			return ids
		}

		for _, tc := range []struct {
			name string
			ctx  context.Context
			repo *extsvc.Repository
			ids  []extsvc.AccountID
			err  string
		}{
			{
				name: "no repo provided",
				repo: nil,
				err:  "no repo provided",
			},
			{
				name: "not a code host of the repo",
				repo: &extsvc.Repository{
					URI: "github.com/user/repo",
					ExternalRepoSpec: api.ExternalRepoSpec{
						ServiceType: extsvc.TypeGitHub,
						ServiceID:   "https://github.com",
					},
				},
				err: `not a code host of the repo: want "${INSTANCEURL}" but have "https://github.com"`,
			},
			{
				name: "private user ids are retrieved",
				repo: &extsvc.Repository{
					URI: "${INSTANCEURL}/user/repo",
					ExternalRepoSpec: api.ExternalRepoSpec{
						ID:          strconv.Itoa(f.repos["super-secret-repo"].ID),
						ServiceType: h.ServiceType,
						ServiceID:   h.ServiceID,
					},
				},
				ids: append(userIDs("ceo"), "1"), // admin user
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if tc.ctx == nil {
					tc.ctx = context.Background()
				}

				if tc.err == "" {
					tc.err = "<nil>"
				}

				tc.err = strings.ReplaceAll(tc.err, "${INSTANCEURL}", cli.URL.String())

				ids, err := p.FetchRepoPerms(tc.ctx, tc.repo)

				if have, want := fmt.Sprint(err), tc.err; have != want {
					t.Errorf("error:\nhave: %q\nwant: %q", have, want)
				}

				sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
				sort.Slice(tc.ids, func(i, j int) bool { return tc.ids[i] < tc.ids[j] })

				if have, want := ids, tc.ids; !reflect.DeepEqual(have, want) {
					t.Error(cmp.Diff(have, want))
				}
			})
		}
	}
}

func marshalJSON(v interface{}) []byte {
	bs, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return bs
}

// fixtures contains the data we need loaded in Bitbucket Server API
// to run the Provider tests. Because we use VCR recordings, we don't
// need a Bitbucket Server API up and running to run those tests. But if
// you want to work on these tests / code, you need to start a new instance
// of Bitbucket Server with docker, create an Application Link as per
// https://docs.sourcegraph.com/admin/external_service/bitbucket_server, and
// then run the tests with -update=true.
type fixtures struct {
	users             map[string]*bitbucketserver.User
	groups            map[string]*bitbucketserver.Group
	projects          map[string]*bitbucketserver.Project
	repos             map[string]*bitbucketserver.Repo
	groupProjectPerms []*bitbucketserver.GroupProjectPermission
	userRepoPerms     []*bitbucketserver.UserRepoPermission
}

func (f fixtures) load(t *testing.T, cli *bitbucketserver.Client) {
	ctx := context.Background()

	for _, u := range f.users {
		u.Password = "password"
		u.Slug = u.Name

		if err := cli.LoadUser(ctx, u); err != nil {
			t.Log(err)
			if err := cli.CreateUser(ctx, u); err != nil {
				t.Error(err)
			}
		}
	}

	for _, g := range f.groups {
		if err := cli.LoadGroup(ctx, g); err != nil {
			t.Log(err)

			if err := cli.CreateGroup(ctx, g); err != nil {
				t.Error(err)
			}

			if err := cli.CreateGroupMembership(ctx, g); err != nil {
				t.Error(err)
			}
		}
	}

	for _, p := range f.projects {
		if err := cli.LoadProject(ctx, p); err != nil {
			t.Log(err)
			if err := cli.CreateProject(ctx, p); err != nil {
				t.Error(err)
			}
		}
	}

	for _, r := range f.repos {
		repo, err := cli.Repo(ctx, r.Project.Key, r.Slug)
		if err != nil {
			t.Log(err)
			if err := cli.CreateRepo(ctx, r); err != nil {
				t.Error(err)
			}
		} else {
			*r = *repo
		}
	}

	for _, p := range f.groupProjectPerms {
		if err := cli.CreateGroupProjectPermission(ctx, p); err != nil {
			t.Error(err)
		}
	}

	for _, p := range f.userRepoPerms {
		if err := cli.CreateUserRepoPermission(ctx, p); err != nil {
			t.Error(err)
		}
	}
}

func newFixtures() *fixtures {
	users := map[string]*bitbucketserver.User{
		"engineer1": {Name: "engineer1", DisplayName: "Mr. Engineer 1", EmailAddress: "engineer1@mycorp.com"},
		"engineer2": {Name: "engineer2", DisplayName: "Mr. Engineer 2", EmailAddress: "engineer2@mycorp.com"},
		"scientist": {Name: "scientist", DisplayName: "Ms. Scientist", EmailAddress: "scientist@mycorp.com"},
		"ceo":       {Name: "ceo", DisplayName: "Mrs. CEO", EmailAddress: "ceo@mycorp.com"},
	}

	groups := map[string]*bitbucketserver.Group{
		"engineers":      {Name: "engineers", Users: []string{"engineer1", "engineer2"}},
		"scientists":     {Name: "scientists", Users: []string{"scientist"}},
		"secret-project": {Name: "secret-project", Users: []string{"engineer1", "scientist"}},
		"management":     {Name: "management", Users: []string{"ceo"}},
	}

	projects := map[string]*bitbucketserver.Project{
		"SECRET":      {Name: "Secret", Key: "SECRET", Public: false},
		"SUPERSECRET": {Name: "Super Secret", Key: "SUPERSECRET", Public: false},
		"PRIVATE":     {Name: "Private", Key: "PRIVATE", Public: false},
		"PUBLIC":      {Name: "Public", Key: "PUBLIC", Public: true},
	}

	repos := map[string]*bitbucketserver.Repo{
		"secret-repo":       {Slug: "secret-repo", Name: "secret-repo", Project: projects["SECRET"]},
		"super-secret-repo": {Slug: "super-secret-repo", Name: "super-secret-repo", Project: projects["SUPERSECRET"]},
		"public-repo":       {Slug: "public-repo", Name: "public-repo", Project: projects["PUBLIC"], Public: true},
		"private-repo":      {Slug: "private-repo", Name: "private-repo", Project: projects["PRIVATE"]},
	}

	groupProjectPerms := []*bitbucketserver.GroupProjectPermission{
		{
			Group:   groups["engineers"],
			Perm:    bitbucketserver.PermProjectWrite,
			Project: projects["PRIVATE"],
		},
		{
			Group:   groups["engineers"],
			Perm:    bitbucketserver.PermProjectWrite,
			Project: projects["PUBLIC"],
		},
		{
			Group:   groups["scientists"],
			Perm:    bitbucketserver.PermProjectRead,
			Project: projects["PRIVATE"],
		},
		{
			Group:   groups["secret-project"],
			Perm:    bitbucketserver.PermProjectWrite,
			Project: projects["SECRET"],
		},
		{
			Group:   groups["management"],
			Perm:    bitbucketserver.PermProjectRead,
			Project: projects["SECRET"],
		},
		{
			Group:   groups["management"],
			Perm:    bitbucketserver.PermProjectRead,
			Project: projects["PRIVATE"],
		},
	}

	userRepoPerms := []*bitbucketserver.UserRepoPermission{
		{
			User: users["ceo"],
			Perm: bitbucketserver.PermRepoWrite,
			Repo: repos["super-secret-repo"],
		},
	}

	return &fixtures{
		users:             users,
		groups:            groups,
		projects:          projects,
		repos:             repos,
		groupProjectPerms: groupProjectPerms,
		userRepoPerms:     userRepoPerms,
	}
}

type codeHost struct {
	*extsvc.CodeHost
}

func (h codeHost) externalAccount(userID int32, u *bitbucketserver.User) *extsvc.Account {
	bs := marshalJSON(u)
	return &extsvc.Account{
		UserID: userID,
		AccountSpec: extsvc.AccountSpec{
			ServiceType: h.ServiceType,
			ServiceID:   h.ServiceID,
			AccountID:   strconv.Itoa(u.ID),
		},
		AccountData: extsvc.AccountData{
			Data: (*json.RawMessage)(&bs),
		},
	}
}

func newClient(t *testing.T, name string) (*bitbucketserver.Client, func()) {
	cli, save := bitbucketserver.NewTestClient(t, name, *update)

	signingKey := os.Getenv("BITBUCKET_SERVER_SIGNING_KEY")
	if signingKey == "" {
		// Bogus key
		signingKey = `LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlCUEFJQkFBSkJBUEpIaWprdG1UMUlLYUd0YTVFZXAzQVo5Q2VPZUw4alBESUZUN3dRZ0tabXQzRUZxRGhCCk93bitRVUhKdUs5Zm92UkROSmVWTDJvWTVCT0l6NHJ3L0cwQ0F3RUFBUUpCQU1BK0o5Mks0d2NQVllsbWMrM28KcHU5NmlKTkNwMmp5Nm5hK1pEQlQzK0VvSUo1VFJGdnN3R2kvTHUzZThYUWwxTDNTM21ub0xPSlZNcTF0bUxOMgpIY0VDSVFEK3daeS83RlYxUEFtdmlXeWlYVklETzJnNWJOaUJlbmdKQ3hFa3Nia1VtUUloQVBOMlZaczN6UFFwCk1EVG9vTlJXcnl0RW1URERkamdiOFpzTldYL1JPRGIxQWlCZWNKblNVQ05TQllLMXJ5VTFmNURTbitoQU9ZaDkKWDFBMlVnTDE3bWhsS1FJaEFPK2JMNmRDWktpTGZORWxmVnRkTUtxQnFjNlBIK01heFU2VzlkVlFvR1dkQWlFQQptdGZ5cE9zYTFiS2hFTDg0blovaXZFYkJyaVJHalAya3lERHYzUlg0V0JrPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=`
	}

	consumerKey := os.Getenv("BITBUCKET_SERVER_CONSUMER_KEY")
	if consumerKey == "" {
		consumerKey = "sourcegraph"
	}

	if err := cli.SetOAuth(consumerKey, signingKey); err != nil {
		t.Fatal(err)
	}

	return cli, save
}

func newProvider(cli *bitbucketserver.Client) *Provider {
	p := NewProvider(cli, "", false)
	p.pageSize = 1 // Exercise pagination
	return p
}
