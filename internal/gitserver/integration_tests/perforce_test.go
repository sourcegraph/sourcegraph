package inttests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/perforce"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestIntegration_PerforceSyncer(t *testing.T) {
	ctx := context.Background()

	// TODO: We should actually talk to a gitserver via gRPC here, to test
	// the full error handling over the wire.
	gs := gitserver.NewMockClient()
	gs.P4ExecFunc.SetDefaultHook(func(ctx context.Context, p4port, p4user, p4passwd string, args ...string) (io.ReadCloser, http.Header, error) {
		cmd := exec.CommandContext(ctx, "p4", args...)
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "P4PORT="+p4port, "P4USER="+p4user, "P4PASSWD="+p4passwd)
		out, err := cmd.CombinedOutput()
		return io.NopCloser(bytes.NewBuffer(out)), nil, err
	})

	homeDir := filepath.Join(t.TempDir(), "home")
	if err := os.Mkdir(homeDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	os.Setenv("HOME", homeDir)

	p4port, p4user, p4passwd, _ := setupPerforce(t)

	t.Run("Successfully discovers depots", func(t *testing.T) {
		perforceConfig, err := json.Marshal(schema.PerforceConnection{
			// TODO: Later add a test for sub-repo perms as well.
			Authorization: &schema.PerforceAuthorization{},
			Depots:        []string{"//test", "//alice", "//source", "//eng"},
			P4Port:        p4port,
			P4User:        p4user,
			P4Passwd:      p4passwd,
			// TODO: What does this do?
			P4Client: "",
			// Trusted fingerprint: fingerprint
		})
		require.NoError(t, err)

		src, err := repos.NewPerforceSourceWithGitserverClient(ctx, gs, &types.ExternalService{
			ID:     1,
			Kind:   "perforce",
			Config: types.NewUnencryptedSecret(string(perforceConfig)),
		})
		if err != nil {
			t.Fatal(err)
		}

		results := make(chan repos.SourceResult)
		go func() {
			src.ListRepos(ctx, results)
			close(results)
		}()
		repos := []*types.Repo{}
		for res := range results {
			if res.Err != nil {
				t.Fatal(res.Err)
			}
			repos = append(repos, res.Repo)
		}
		compareSourcedRepos(t, repos, []*types.Repo{
			{
				Name:        "test",
				URI:         "test",
				Description: "A great depot.",
				Private:     true,
				ExternalRepo: api.ExternalRepoSpec{
					ID:          "//test/",
					ServiceType: "perforce",
					ServiceID:   "ssl:127.0.0.1:1666",
				},
				Sources: map[string]*types.SourceInfo{
					"extsvc:perforce:1": {
						ID:       "extsvc:perforce:1",
						CloneURL: "perforce://ssl:127.0.0.1:1666//test/",
					},
				},
				CreatedAt: time.Now(),
				Metadata: &perforce.Depot{
					Depot:       "test",
					Date:        "2023/09/05 12:52:08",
					Description: "A great depot.\n",
					Map:         "test/...",
					Owner:       "admin",
					Type:        "local",
				},
			},
			{
				Name:        "alice",
				URI:         "alice",
				Description: "A great depot.",
				Private:     true,
				ExternalRepo: api.ExternalRepoSpec{
					ID:          "//alice/",
					ServiceType: "perforce",
					ServiceID:   "ssl:127.0.0.1:1666",
				},
				Sources: map[string]*types.SourceInfo{
					"extsvc:perforce:1": {
						ID:       "extsvc:perforce:1",
						CloneURL: "perforce://ssl:127.0.0.1:1666//alice/",
					},
				},
				Metadata: &perforce.Depot{
					Depot:       "alice",
					Date:        "2023/09/05 12:52:08",
					Description: "A great depot.\n",
					Map:         "alice/...",
					Owner:       "alice",
					Type:        "local",
				},
			},
			{
				Name:        "source",
				URI:         "source",
				Description: "A great depot.",
				Private:     true,
				ExternalRepo: api.ExternalRepoSpec{
					ID:          "//source/",
					ServiceType: "perforce",
					ServiceID:   "ssl:127.0.0.1:1666",
				},
				Sources: map[string]*types.SourceInfo{
					"extsvc:perforce:1": {
						ID:       "extsvc:perforce:1",
						CloneURL: "perforce://ssl:127.0.0.1:1666//source/",
					},
				},
				Metadata: &perforce.Depot{
					Depot:       "source",
					Date:        "2023/09/05 12:52:08",
					Description: "A great depot.\n",
					Map:         "source/...",
					Owner:       "admin",
					Type:        "local",
				},
			},
			{
				Name:        "eng",
				URI:         "eng",
				Description: "A great depot.",
				Private:     true,
				ExternalRepo: api.ExternalRepoSpec{
					ID:          "//eng/",
					ServiceType: "perforce",
					ServiceID:   "ssl:127.0.0.1:1666",
				},
				Sources: map[string]*types.SourceInfo{
					"extsvc:perforce:1": {
						ID:       "extsvc:perforce:1",
						CloneURL: "perforce://ssl:127.0.0.1:1666//eng/",
					},
				},
				Metadata: &perforce.Depot{
					Depot:       "eng",
					Date:        "2023/09/05 12:52:08",
					Description: "A great depot.\n",
					Map:         "eng/...",
					Owner:       "admin",
					Type:        "local",
				},
			},
		})
	})

	t.Run("Works with nested paths", func(t *testing.T) {
		perforceConfig, err := json.Marshal(schema.PerforceConnection{
			// TODO: Later add a test for sub-repo perms as well.
			Authorization: &schema.PerforceAuthorization{},
			Depots:        []string{"//eng/batches"},
			P4Port:        p4port,
			P4User:        p4user,
			P4Passwd:      p4passwd,
			// TODO: What does this do?
			P4Client: "",
			// Trusted fingerprint: fingerprint
		})
		require.NoError(t, err)

		src, err := repos.NewPerforceSourceWithGitserverClient(ctx, gs, &types.ExternalService{
			ID:     1,
			Kind:   "perforce",
			Config: types.NewUnencryptedSecret(string(perforceConfig)),
		})
		if err != nil {
			t.Fatal(err)
		}

		results := make(chan repos.SourceResult)
		go func() {
			src.ListRepos(ctx, results)
			close(results)
		}()
		repos := []*types.Repo{}
		for res := range results {
			if res.Err != nil {
				t.Fatal(res.Err)
			}
			repos = append(repos, res.Repo)
		}
		compareSourcedRepos(t, repos, []*types.Repo{
			{
				Name:        "eng/batches",
				URI:         "eng/batches",
				Description: "A great depot.",
				Private:     true,
				ExternalRepo: api.ExternalRepoSpec{
					ID:          "//eng/batches/",
					ServiceType: "perforce",
					ServiceID:   "ssl:127.0.0.1:1666",
				},
				Sources: map[string]*types.SourceInfo{
					"extsvc:perforce:1": {
						ID:       "extsvc:perforce:1",
						CloneURL: "perforce://ssl:127.0.0.1:1666//eng/batches/",
					},
				},
				Metadata: &perforce.Depot{
					Depot:       "eng",
					Date:        "2023/09/05 12:52:08",
					Description: "A great depot.\n",
					Map:         "eng/...",
					Owner:       "admin",
					Type:        "local",
				},
			},
		})
	})

	t.Run("Non-existant nested paths", func(t *testing.T) {
		perforceConfig, err := json.Marshal(schema.PerforceConnection{
			// TODO: Later add a test for sub-repo perms as well.
			Authorization: &schema.PerforceAuthorization{},
			Depots:        []string{"//eng/notathing"},
			P4Port:        p4port,
			P4User:        p4user,
			P4Passwd:      p4passwd,
			// TODO: What does this do?
			P4Client: "",
			// Trusted fingerprint: fingerprint
		})
		require.NoError(t, err)

		src, err := repos.NewPerforceSourceWithGitserverClient(ctx, gs, &types.ExternalService{
			ID:     1,
			Kind:   "perforce",
			Config: types.NewUnencryptedSecret(string(perforceConfig)),
		})
		if err != nil {
			t.Fatal(err)
		}

		results := make(chan repos.SourceResult)
		go func() {
			src.ListRepos(ctx, results)
			close(results)
		}()
		rs := []*types.Repo{}
		for res := range results {
			if res.Err != nil {
				// We expect a sync error for the unknown depot here, but it should
				// not be a fatal error, that would abort the sync entirely.
				if repos.IsFatalSyncError(res.Err) {
					t.Fatal(res.Err)
				}
				continue
			}
			rs = append(rs, res.Repo)
		}
		require.Equal(t, []*types.Repo{}, rs)
	})

	t.Run("Can handle inaccessible depot names", func(t *testing.T) {
		perforceConfig, err := json.Marshal(schema.PerforceConnection{
			// TODO: Later add a test for sub-repo perms as well.
			Authorization: &schema.PerforceAuthorization{},
			Depots:        []string{"//test", "//unknown"},
			P4Port:        p4port,
			P4User:        p4user,
			P4Passwd:      p4passwd,
			// TODO: What does this do?
			P4Client: "",
			// Trusted fingerprint: fingerprint
		})
		require.NoError(t, err)

		src, err := repos.NewPerforceSourceWithGitserverClient(ctx, gs, &types.ExternalService{
			ID:     1,
			Kind:   "perforce",
			Config: types.NewUnencryptedSecret(string(perforceConfig)),
		})
		if err != nil {
			t.Fatal(err)
		}

		results := make(chan repos.SourceResult)
		go func() {
			src.ListRepos(ctx, results)
			close(results)
		}()
		rs := []*types.Repo{}
		for res := range results {
			if res.Err != nil {
				// We expect a sync error for the unknown depot here, but it should
				// not be a fatal error, that would abort the sync entirely.
				if repos.IsFatalSyncError(res.Err) {
					t.Fatal(res.Err)
				}
				continue
			}
			rs = append(rs, res.Repo)
		}
		compareSourcedRepos(t, rs, []*types.Repo{
			{
				Name:        "test",
				URI:         "test",
				Description: "A great depot.",
				Private:     true,
				ExternalRepo: api.ExternalRepoSpec{
					ID:          "//test/",
					ServiceType: "perforce",
					ServiceID:   "ssl:127.0.0.1:1666",
				},
				Sources: map[string]*types.SourceInfo{
					"extsvc:perforce:1": {
						ID:       "extsvc:perforce:1",
						CloneURL: "perforce://ssl:127.0.0.1:1666//test/",
					},
				},
				CreatedAt: time.Now(),
				Metadata: &perforce.Depot{
					Depot:       "test",
					Date:        "2023/09/05 12:52:08",
					Description: "A great depot.\n",
					Map:         "test/...",
					Owner:       "admin",
					Type:        "local",
				},
			},
		})
	})

	t.Run("Skips and warns about non-local depots", func(t *testing.T) {
		// TODO: Implement me.
	})

	t.Run("List hard fails with invalid credentials", func(t *testing.T) {
		perforceConfig, err := json.Marshal(schema.PerforceConnection{
			// TODO: Later add a test for sub-repo perms as well.
			Authorization: &schema.PerforceAuthorization{},
			Depots:        []string{"//test"},
			P4Port:        p4port,
			P4User:        p4user,
			P4Passwd:      "verywrongsecret",
			// TODO: What does this do?
			P4Client: "",
			// Trusted fingerprint: fingerprint
		})
		require.NoError(t, err)

		src, err := repos.NewPerforceSourceWithGitserverClient(ctx, gs, &types.ExternalService{
			ID:     1,
			Kind:   "perforce",
			Config: types.NewUnencryptedSecret(string(perforceConfig)),
		})
		if err != nil {
			t.Fatal(err)
		}

		results := make(chan repos.SourceResult)
		go func() {
			src.ListRepos(ctx, results)
			close(results)
		}()
		for res := range results {
			if res.Err != nil {
				// We expect a fatal sync error to be returned.
				if !repos.IsFatalSyncError(res.Err) {
					t.Fatal("got a non fatal sync error", res.Err)
				}
			}
		}
	})

	t.Run("CheckConnection", func(t *testing.T) {
		t.Run("With valid credential", func(t *testing.T) {
			perforceConfig, err := json.Marshal(schema.PerforceConnection{
				// TODO: Later add a test for sub-repo perms as well.
				Authorization: &schema.PerforceAuthorization{},
				Depots:        []string{"//test", "//alice", "//source", "//eng"},
				P4Port:        p4port,
				P4User:        p4user,
				P4Passwd:      p4passwd,
				// TODO: What does this do?
				P4Client: "",
				// Trusted fingerprint: fingerprint
			})
			require.NoError(t, err)

			src, err := repos.NewPerforceSourceWithGitserverClient(ctx, gs, &types.ExternalService{
				ID:     1,
				Kind:   "perforce",
				Config: types.NewUnencryptedSecret(string(perforceConfig)),
			})
			if err != nil {
				t.Fatal(err)
			}

			// Ensure the connection check works.
			require.NoError(t, src.CheckConnection(ctx))
		})

		t.Run("With invalid credential", func(t *testing.T) {
			perforceConfig, err := json.Marshal(schema.PerforceConnection{
				// TODO: Later add a test for sub-repo perms as well.
				Authorization: &schema.PerforceAuthorization{},
				Depots:        []string{"//test", "//alice", "//source", "//eng"},
				P4Port:        p4port,
				P4User:        p4user,
				P4Passwd:      "verywrongpassword",
				// TODO: What does this do?
				P4Client: "",
				// Trusted fingerprint: fingerprint
			})
			require.NoError(t, err)

			src, err := repos.NewPerforceSourceWithGitserverClient(ctx, gs, &types.ExternalService{
				ID:     1,
				Kind:   "perforce",
				Config: types.NewUnencryptedSecret(string(perforceConfig)),
			})
			if err != nil {
				t.Fatal(err)
			}

			// Ensure the connection check fails.
			require.Error(t, src.CheckConnection(ctx))
		})

		t.Run("With invalid host", func(t *testing.T) {
			perforceConfig, err := json.Marshal(schema.PerforceConnection{
				// TODO: Later add a test for sub-repo perms as well.
				Authorization: &schema.PerforceAuthorization{},
				Depots:        []string{"//test", "//alice", "//source", "//eng"},
				P4Port:        "127.0.0.1:9111", // wrong port number!
				P4User:        p4user,
				P4Passwd:      p4passwd,
				// TODO: What does this do?
				P4Client: "",
				// Trusted fingerprint: fingerprint
			})
			require.NoError(t, err)

			src, err := repos.NewPerforceSourceWithGitserverClient(ctx, gs, &types.ExternalService{
				ID:     1,
				Kind:   "perforce",
				Config: types.NewUnencryptedSecret(string(perforceConfig)),
			})
			if err != nil {
				t.Fatal(err)
			}

			// Ensure the connection check fails.
			require.Error(t, src.CheckConnection(ctx))
		})
	})
}

func compareSourcedRepos(t *testing.T, got, want []*types.Repo) {
	t.Helper()
	if diff := cmp.Diff(got, want, cmpopts.IgnoreFields(types.Repo{}, "CreatedAt"), cmpopts.IgnoreFields(perforce.Depot{}, "Date")); diff != "" {
		t.Fatal(diff)
	}
}

func setupPerforce(t *testing.T) (p4port, p4user, p4passwd, fingerprint string) {
	var p4trustPath string
	p4port, p4user, p4passwd, p4trustPath, fingerprint = spawnHelixServer(t)
	r := p4Runner{t: t, p4port: p4port, p4user: p4user, p4passwd: p4passwd, p4trustPath: p4trustPath}

	// Make sure only the initial admin is considered a superuser. This should
	// be what every perforce installation has configured at the bare minimum.
	r.EnsureOnlyOneSuperuser(p4user)
	// Setup some basic users and group structures:
	// eng (group)
	// └─ source (group)
	// 	├─ alice (user)
	// 	└─ bob (user)
	r.AddUser("alice", "alice@sourcegraph.com", "Alice")
	r.AddUser("bob", "bob@sourcegraph.com", "Bob")
	r.AddGroup("source", []string{"admin"}, []string{"alice", "bob"}, []string{})
	r.AddGroup("eng", []string{"admin"}, []string{}, []string{"source"})

	// Write a simple test depot.
	// This depot should be open to read/write from all - aka a public depot.
	{
		t.Log("Creating test depot")
		testDepotDir := filepath.Join(t.TempDir(), "test_depot")
		if err := os.Mkdir(testDepotDir, os.ModePerm); err != nil {
			t.Fatal(err)
		}
		writeFS(t, testDepotDir, map[string][]byte{
			"README.md": []byte("# Hi world\n"),
		})
		var err error
		testDepotDir, err = filepath.EvalSymlinks(testDepotDir)
		if err != nil {
			t.Fatal(err)
		}
		r.CreateDepotFromDir(perforce.Local, "test", testDepotDir, p4user)
	}

	// Create a depot that is only writable by alice.
	{
		t.Log("Creating alice depot")
		aliceDepotDir := filepath.Join(t.TempDir(), "alice_depot")
		if err := os.Mkdir(aliceDepotDir, os.ModePerm); err != nil {
			t.Fatal(err)
		}
		writeFS(t, aliceDepotDir, map[string][]byte{
			"README.md": []byte("# Hi alice\n"),
		})
		r.CreateDepotFromDir(perforce.Local, "alice", aliceDepotDir, "alice")
		// Allow alice to write to the //alice/... depot, disallow everyone else.
		r.CreateProtectPolicy("=write user * * //alice/...", "write user alice * //alice/...")
	}

	// Create another depot that is only writable by members of source.
	{
		t.Log("Creating source depot")
		sourceDepotDir := filepath.Join(t.TempDir(), "source_depot")
		if err := os.Mkdir(sourceDepotDir, os.ModePerm); err != nil {
			t.Fatal(err)
		}
		writeFS(t, sourceDepotDir, map[string][]byte{
			"README.md": []byte("# Hi source\n"),
		})
		r.CreateDepotFromDir(perforce.Local, "source", sourceDepotDir, p4user)
		// Allow source to write to the //source/... depot, disallow everyone else.
		r.CreateProtectPolicy("=write user * * //source/...", "write group source * //source/...")
	}

	// Create another depot that is only writable by members of eng.
	{
		t.Log("Creating eng depot")
		engDepotDir := filepath.Join(t.TempDir(), "eng_depot")
		if err := os.Mkdir(engDepotDir, os.ModePerm); err != nil {
			t.Fatal(err)
		}
		writeFS(t, engDepotDir, map[string][]byte{
			"batches/README.md": []byte("# Hi batchers\n"),
			"README.md":         []byte("# Hi eng\n"),
		})
		r.CreateDepotFromDir(perforce.Local, "eng", engDepotDir, p4user)
		// Allow eng to write to the //eng/... depot, disallow everyone else.
		r.CreateProtectPolicy("=write user * * //eng/...", "write group eng * //eng/...")
	}

	return p4port, p4user, p4passwd, fingerprint
}

type p4Runner struct {
	t                                     *testing.T
	p4port, p4user, p4passwd, p4trustPath string
}

func (r p4Runner) Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Env = []string{
		fmt.Sprintf("P4PORT=%s", r.p4port),
		fmt.Sprintf("P4USER=%s", r.p4user),
		fmt.Sprintf("P4PASSWD=%s", r.p4passwd),
		fmt.Sprintf("P4TRUST=%s", r.p4trustPath),
	}
	return cmd
}

const userManifestFmtstr = `# A Perforce User Specification.
#
#  User:        The user's user name.
#  Type:        Either 'service', 'operator', or 'standard'.
#               Default: 'standard'. Read only.
#  Email:       The user's email address; for email review.
#  Update:      The date this specification was last modified.
#  Access:      The date this user was last active.  Read only.
#  FullName:    The user's real name.
#  JobView:     Selects jobs for inclusion during changelist creation.
#  Password:    If set, user must have matching $P4PASSWD on client.
#  AuthMethod:  'perforce' if using standard authentication or 'ldap' if
#               this user should use native LDAP authentication.  The '+2fa'
#               modifier can be added to the AuthMethod, requiring the user to
#               perform multi factor authentication in addition to password
#               authentication. For example: 'perforce+2fa'.
#  Reviews:     Listing of depot files to be reviewed by user.

User:	%s

Type: 	standard

Email:	%s

FullName:	%s
`

func (r p4Runner) AddUser(name, email, fullName string) {
	cmd := r.Command("p4", "user", "-f", "-i")
	cmd.Stdin = strings.NewReader(fmt.Sprintf(userManifestFmtstr, name, email, fullName))
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatal(errors.Wrap(err, string(out)))
	}
	// We also make sure users have a password to prevent impersonation.
	// password := "Secret$1" + name
	// cmd = r.Command("p4", "passwd", "-P", name)
	// cmd.Stdin = strings.NewReader(password)
	// if out, err := cmd.CombinedOutput(); err != nil {
	// 	r.t.Fatal(errors.Wrap(err, string(out)))
	// }
}

const groupManifestFmtstr = `# A Perforce Group Specification.
#
#  Group:       The name of the group.
#  Description: A description for the group (optional).
#  MaxResults:  Limits the rows (unless 'unlimited' or 'unset') any one
#               operation can return to the client.
#               See 'p4 help maxresults'.
#  MaxScanRows: Limits the rows (unless 'unlimited' or 'unset') any one
#               operation can scan from any one database table.
#               See 'p4 help maxresults'.
#  MaxLockTime: Limits the time (in milliseconds, unless 'unlimited' or
#               'unset') any one operation can lock any database table when
#               scanning data. See 'p4 help maxresults'.
#  MaxOpenFiles:
#               Limits files (unless 'unlimited' or 'unset') any one
#               operation can open. See 'p4 help maxresults'.
#  MaxMemory:
#               Limits the amount of memory a command may consume.
#               Unit is megabytes.  See 'p4 help maxresults'.
#  Timeout:     A time (in seconds, unless 'unlimited' or 'unset')
#               which determines how long a 'p4 login'
#               session ticket remains valid (default is 12 hours).
#  PasswordTimeout:
#               A time (in seconds, unless 'unlimited' or 'unset')
#               which determines how long a 'p4 password'
#               password remains valid (default is unset).
#  LdapConfig:  The LDAP configuration to use when populating the group's
#               user list from an LDAP query. See 'p4 help ldap'.
#  LdapSearchQuery:
#               The LDAP query used to identify the members of the group.
#  LdapUserAttribute:
#               The LDAP attribute that represents the user's username.
#  LdapUserDNAttribute:
#               The LDAP attribute in the group object that contains the
#               DN of the user object.
#  Subgroups:   Other groups automatically included in this group.
#  Owners:      Users allowed to change this group without requiring super
#               access permission.
#  Users:       The users in the group.  One per line.

Group:	%s

Description:

MaxResults:	unset

MaxScanRows:	unset

MaxLockTime:	unset

MaxOpenFiles:	unset

MaxMemory:	unset

Timeout:	43200

PasswordTimeout:	unset

Subgroups:
	%s

Owners:
	%s

Users:
	%s
`

func (r p4Runner) AddGroup(name string, owners, users, subgroups []string) {
	cmd := r.Command("p4", "group", "-i")
	cmd.Stdin = strings.NewReader(fmt.Sprintf(groupManifestFmtstr, name, strings.Join(subgroups, "\n	"), strings.Join(owners, "\n	"), strings.Join(users, "\n	")))
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatal(errors.Wrap(err, string(out)))
	}
}

const initialProtectManifestFmtstr = `Protections:
	write user * * //...
	list user * * -//spec/...
	super user %s * //...
`

func (r p4Runner) EnsureOnlyOneSuperuser(admin string) {
	cmd := r.Command("p4", "protect", "-i")
	cmd.Stdin = strings.NewReader(fmt.Sprintf(initialProtectManifestFmtstr, admin))
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatal(errors.Wrap(err, string(out)))
	}
}

func (r p4Runner) CreateProtectPolicy(protects ...string) {
	currentPolicy, err := r.Command("p4", "protect", "-o").Output()
	if err != nil {
		r.t.Fatal(err)
	}
	newPolicy := string(currentPolicy) + "	" + strings.Join(protects, "\n	")
	cmd := r.Command("p4", "protect", "-i")
	cmd.Stdin = strings.NewReader(newPolicy)
	if err := cmd.Run(); err != nil {
		r.t.Fatal(err)
	}
}

const depotManifestFmtstr = `# A Perforce Depot Specification.
#
#  Depot:       The name of the depot.
#  Owner:       The user who created this depot.
#  Date:        The date this specification was last modified.
#  Description: A short description of the depot (optional).
#  Type:        Whether the depot is 'local', 'remote',
#               'stream', 'spec', 'archive', 'tangent',
#               'unload', 'extension' or 'graph'.
#               Default is 'local'.
#  Address:     Connection address (remote depots only).
#  Suffix:      Suffix for all saved specs (spec depot only).
#  StreamDepth: Depth for streams in this depot (stream depots only).
#  Map:         Path translation information (must have ... in it).
#  SpecMap:     For spec depot, which specs should be recorded (optional).
#
# Use 'p4 help depot' to see more about depot forms.

Depot:	%s

Owner:	%s

Description:
	A great depot.

Type:	%s

Address:	local

Suffix:	.p4s

StreamDepth:	//%s/1

Map:	%s/...
`

const clientManifestFmtstr = `# A Perforce Client Specification.
#
#  Client:      The client name.
#  Update:      The date this specification was last modified.
#  Access:      The date this client was last used in any way.
#  Owner:       The Perforce user name of the user who owns the client
#               workspace. The default is the user who created the
#               client workspace.
#  Host:        If set, restricts access to the named host.
#  Description: A short description of the client (optional).
#  Root:        The base directory of the client workspace.
#  AltRoots:    Up to two alternate client workspace roots.
#  Options:     Client options:
#                      [no]allwrite [no]clobber [no]compress
#                      [un]locked [no]modtime [no]rmdir [no]altsync
#  SubmitOptions:
#                      submitunchanged/submitunchanged+reopen
#                      revertunchanged/revertunchanged+reopen
#                      leaveunchanged/leaveunchanged+reopen
#  LineEnd:     Text file line endings on client: local/unix/mac/win/share.
#  Type:        Type of client: writeable/readonly/graph/partitioned.
#  ServerID:    If set, restricts access to the named server.
#  View:        Lines to map depot files into the client workspace.
#  ChangeView:  Lines to restrict depot files to specific changelists.
#  Stream:      The stream to which this client's view will be dedicated.
#               (Files in stream paths can be submitted only by dedicated
#               stream clients.) When this optional field is set, the
#               View field will be automatically replaced by a stream
#               view as the client spec is saved.
#  StreamAtChange:  A changelist number that sets a back-in-time view of a
#                   stream ( Stream field is required ).
#                   Changes cannot be submitted when this field is set.
#
# Use 'p4 help client' to see more about client views and options.

Client:	%s

Owner:	%s

Description:
	Created by admin.

Root:	%s

Options:	noallwrite noclobber nocompress unlocked nomodtime normdir noaltsync

SubmitOptions:	submitunchanged

LineEnd:	local

View:
	//%s/... //%s/...
`

func (r p4Runner) CreateDepotFromDir(typ perforce.PerforceDepotType, name, dir, owner string) {
	// On Mac, the t.TempDir() is in /var/, which is a symlink to /private/var/,
	// and perforce doesn't seem to resolve these to the same path, so it complains
	// about a view mismatch.
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		r.t.Fatal(err)
	}

	cmd := r.Command("p4", "depot", "-i")
	cmd.Stdin = strings.NewReader(fmt.Sprintf(depotManifestFmtstr, name, owner, typ, name, name))
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatal(errors.Wrap(err, "p4 depot -i: "+fmt.Sprintf(depotManifestFmtstr, name, owner, typ, name, name)+string(out)))
	} else {
		r.t.Log(string(out))
	}

	cUID, err := uuid.NewV1()
	if err != nil {
		r.t.Fatal(err)
	}
	client := cUID.String()

	// Create client.
	cmd = r.Command("p4", "client", "-i")
	cmd.Stdin = strings.NewReader(fmt.Sprintf(clientManifestFmtstr, client, owner, dir, name, client))
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatal(errors.Wrap(err, "p4 client -i"+fmt.Sprintf(clientManifestFmtstr, client, owner, dir, name, client)+string(out)))
	} else {
		r.t.Log(string(out))
	}

	// Reconcile files from dir.
	cmd = r.Command("p4", "-c", client, "reconcile")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatal(errors.Wrap(err, string(out)))
	} else {
		r.t.Log(string(out))
	}

	// Submit files.
	if out, err := r.Command("p4", "-c", client, "submit", "-d", "Add files").CombinedOutput(); err != nil {
		r.t.Fatal(errors.Wrap(err, string(out)))
	} else {
		r.t.Log(string(out))
	}

	// Delete client.
	if out, err := r.Command("p4", "client", "-d", client).CombinedOutput(); err != nil {
		r.t.Fatal(errors.Wrap(err, string(out)))
	} else {
		r.t.Log(string(out))
	}
}

func writeFS(t *testing.T, root string, fs map[string][]byte) {
	for name, content := range fs {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, os.ModePerm); err != nil {
			t.Fatal(err)
		}
	}
}

func spawnHelixServer(t *testing.T) (p4port, adminUser, adminTicket, p4trustPath, fingerprint string) {
	// Setup dir to store SSL certificates for this server instance. We want to test
	// with SSL enabled, since this is the most common setup we expect at customers.
	sslDir := filepath.Join(t.TempDir(), "ssl")
	if err := os.Mkdir(sslDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// Generate the key and certificate.
	t.Log("creating SSL certificates")
	cmd := exec.Command("openssl", "genrsa", "-out", "privatekey.txt", "2048")
	cmd.Dir = sslDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("openssl", "req", "-new", "-key", "privatekey.txt", "-out", "certrequest.csr", "-subj", "/C=US/L=San Francisco/O=Sourcegraph-Test/CN=sourcegraph.com/emailAddress=integration-tests@sgdev.org")
	cmd.Dir = sslDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("openssl", "x509", "-req", "-days", "365", "-in", "certrequest.csr", "-signkey", "privatekey.txt", "-out", "certificate.txt")
	cmd.Dir = sslDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Grab a unique name for the container so we can reference it in docker stop
	// in t.Cleanup.
	containerName, err := uuid.NewV1()
	if err != nil {
		t.Fatal(err)
	}

	adminUser = "admin"
	adminPass := "Supersecretpassword$1"

	// Run the helix server in the background.
	t.Log("spawning helix server container")
	dataDir := filepath.Join(t.TempDir(), "p4")
	if err := os.Mkdir(dataDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(
		"docker",
		"run",
		// Forward 1666 to our host.
		"--publish", "1666:1666",
		// Set admin username.
		"--env", fmt.Sprintf("P4USER=%s", adminUser),
		// Set admin password.
		"--env", fmt.Sprintf("P4PASSWD=%s", adminPass),
		// Enable SSL.
		"--env", "P4PORT=ssl:1666",
		// Configure SSL certs to be read from /ssl.
		"--env", "P4SSLDIR=/ssl",
		// Mount our sslDir to the container.
		"--volume", fmt.Sprintf("%s:/opt/ssl", sslDir),
		// Mount the p4 data dir.
		// "--volume", fmt.Sprintf("%s:/p4", dataDir),
		// Make the container referencable by name.
		"--name", containerName.String(),
		// Run in the background,
		"--detach",
		"--entrypoint", "/bin/sh",
		"sourcegraph/helix-p4d:2023.1",
		// Fixup permissions of SSL certs first.
		"-c", `mkdir /ssl && cp -R /opt/ssl/* /ssl/ && chmod -R 600 /ssl && chmod 0500 /ssl && \
		init.sh && \
		/usr/bin/tail -F $P4ROOT/logs/log`,
	)
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(errors.Wrap(err, string(out)))
	}

	// Make sure we stop the container properly.
	t.Cleanup(func() {
		if t.Failed() {
			cmd := exec.Command("docker", "logs", containerName.String())
			cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Log("Helix container logs:" + string(out))
			} else {
				t.Log("failed to get container logs: " + err.Error())
			}
		}

		t.Log("stopping helix server container")
		cmd = exec.Command("docker", "stop", containerName.String(), "-s", "sigkill", "-t", "0")
		cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(errors.Wrap(err, string(out)))
		}
	})

	// Wait for perforce to be up.
	for {
		t.Log("Checking for perforce to come up")
		conn, err := net.DialTimeout("tcp", "127.0.0.1:1666", 2*time.Second)
		if err != nil {
			t.Log("Not up yet")
			time.Sleep(10 * time.Millisecond)
			continue
		}
		_ = conn.Close()
		break
	}

	// TODO: The above check isn't reliable enough, need to find a better way to
	// wait for perforce to be fully initialized.
	time.Sleep(30 * time.Second)

	// Collect the servers fingerprint.
	// TODO: This is quite nasty to fix FS permissions.
	// Set the P4PORT to the EXTERNAL value, using ssl:1666 here returns a DIFFERENT
	// value.
	cmd = exec.Command("docker", "exec", containerName.String(), "/bin/sh", "-c", "P4PORT=ssl:127.0.0.1:1666 p4d -Gf")
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(errors.Wrap(err, string(out)))
	}
	fingerprint = strings.TrimSpace(strings.TrimPrefix(string(out), "Fingerprint: "))

	p4port = "ssl:127.0.0.1:1666"

	// Establish trust:
	// TODO: Later we want to test here that without trust the connection fails.
	// Also, we can use this fingerprint to test the connection.trustedFingerprint
	// setting.
	t.Log("Establishing p4 trust for fingerprint " + fingerprint)
	p4trustPath = filepath.Join(t.TempDir(), ".p4home", ".p4trust")
	if err := os.Mkdir(filepath.Dir(p4trustPath), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	{
		// TODO: Didn't get this to work at the moment, somehow the generated fingerprint
		// always differs from what the server returns.
		// cmd = exec.Command("p4", "trust", "-i", fingerprint)
		cmd = exec.Command("p4", "trust", "-y", "-f")
		cmd.Dir = t.TempDir()
		cmd.Env = []string{
			fmt.Sprintf("P4PORT=%s", p4port),
			fmt.Sprintf("P4TRUST=%s", p4trustPath),
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(errors.Wrap(err, "failed to establish trust: "+string(out)))
		}
	}

	t.Log("Getting ticket for admin user")
	cmd = exec.Command("p4", "login", "-p", "-a")
	cmd.Dir = t.TempDir()
	cmd.Env = []string{
		fmt.Sprintf("P4PORT=%s", p4port),
		fmt.Sprintf("P4USER=%s", adminUser),
		fmt.Sprintf("P4TRUST=%s", p4trustPath),
	}
	cmd.Stdin = strings.NewReader(adminPass)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatal(errors.Wrap(err, string(out)))
	}
	adminTicket = strings.TrimSpace(strings.TrimPrefix(string(out), "Enter password:"))

	t.Log("p4port", p4port)
	t.Log("adminUser", adminUser)
	t.Log("adminTicket", adminTicket)
	t.Log("p4trustPath", p4trustPath)
	t.Log("fingerprint", fingerprint)

	return p4port, adminUser, adminTicket, p4trustPath, fingerprint
}
