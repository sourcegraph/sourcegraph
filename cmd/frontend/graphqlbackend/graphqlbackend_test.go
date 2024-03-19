package graphqlbackend

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log" //nolint:logging // TODO move all logging to sourcegraph/log
	"os"
	"reflect"
	"testing"

	mockassert "github.com/derision-test/go-mockgen/v2/testutil/assert"
	"github.com/grafana/regexp"
	"github.com/graph-gophers/graphql-go"
	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	sglog "github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Verbose() {
		log15.Root().SetHandler(log15.DiscardHandler())
		log.SetOutput(io.Discard)
		logtest.InitWithLevel(m, sglog.LevelNone)
	} else {
		logtest.Init(m)
	}
	os.Exit(m.Run())
}

func BenchmarkPrometheusFieldName(b *testing.B) {
	tests := [][3]string{
		{"Query", "settingsSubject", "settingsSubject"},
		{"SearchResultMatch", "highlights", "highlights"},
		{"TreeEntry", "isSingleChild", "isSingleChild"},
		{"NoMatch", "NotMatch", "other"},
	}
	for i, t := range tests {
		typeName, fieldName, want := t[0], t[1], t[2]
		b.Run(fmt.Sprintf("test-%v", i), func(b *testing.B) {
			for range b.N {
				got := prometheusFieldName(typeName, fieldName)
				if got != want {
					b.Fatalf("got %q want %q", got, want)
				}
			}
		})
	}
}

func TestRepository(t *testing.T) {
	db := dbmocks.NewMockDB()
	repos := dbmocks.NewMockRepoStore()
	repos.GetByNameFunc.SetDefaultReturn(&types.Repo{ID: 2, Name: "github.com/gorilla/mux"}, nil)
	db.ReposFunc.SetDefaultReturn(repos)
	RunTests(t, []*Test{
		{
			Schema: mustParseGraphQLSchema(t, db),
			Query: `
				{
					repository(name: "github.com/gorilla/mux") {
						name
					}
				}
			`,
			ExpectedResult: `
				{
					"repository": {
						"name": "github.com/gorilla/mux"
					}
				}
			`,
		},
	})
}

func TestRecloneRepository(t *testing.T) {
	resetMocks()

	repos := dbmocks.NewMockRepoStore()
	repos.GetFunc.SetDefaultReturn(&types.Repo{ID: 1, Name: "github.com/gorilla/mux"}, nil)

	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)

	gitserverRepos := dbmocks.NewMockGitserverRepoStore()
	gitserverRepos.GetByIDFunc.SetDefaultReturn(&types.GitserverRepo{RepoID: 1, CloneStatus: "cloned"}, nil)

	db := dbmocks.NewMockDB()
	db.ReposFunc.SetDefaultReturn(repos)
	db.UsersFunc.SetDefaultReturn(users)
	db.GitserverReposFunc.SetDefaultReturn(gitserverRepos)

	repoID := MarshalRepositoryID(1)

	gc := gitserver.NewMockClient()
	gc.RequestRepoCloneFunc.SetDefaultReturn(&protocol.RepoCloneResponse{}, nil)
	r := newSchemaResolver(db, gc)

	_, err := r.RecloneRepository(context.Background(), &struct{ Repo graphql.ID }{Repo: repoID})
	require.NoError(t, err)

	// To reclone, we first make a request to delete the repository, followed by a request
	// to clone the repository again.
	mockassert.CalledN(t, gc.RemoveFunc, 1)
	mockassert.CalledN(t, gc.RequestRepoCloneFunc, 1)
}

func TestDeleteRepositoryFromDisk(t *testing.T) {
	resetMocks()

	repos := dbmocks.NewMockRepoStore()

	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)
	called := backend.Mocks.Repos.MockDeleteRepositoryFromDisk(t, 1)

	gitserverRepos := dbmocks.NewMockGitserverRepoStore()
	gitserverRepos.GetByIDFunc.SetDefaultReturn(&types.GitserverRepo{RepoID: 1, CloneStatus: "cloned"}, nil)

	db := dbmocks.NewMockDB()
	db.ReposFunc.SetDefaultReturn(repos)
	db.UsersFunc.SetDefaultReturn(users)
	db.GitserverReposFunc.SetDefaultReturn(gitserverRepos)
	repoID := base64.StdEncoding.EncodeToString([]byte("Repository:1"))

	RunTests(t, []*Test{
		{
			Schema: mustParseGraphQLSchema(t, db),
			Query: fmt.Sprintf(`
                mutation {
                    deleteRepositoryFromDisk(repo: "%s") {
                        alwaysNil
                    }
                }
            `, repoID),
			ExpectedResult: `
                {
                    "deleteRepositoryFromDisk": {
                        "alwaysNil": null
                    }
                }
            `,
		},
	})

	assert.True(t, *called)
}

func TestResolverTo(t *testing.T) {
	db := dbmocks.NewMockDB()
	// This test exists purely to remove some non determinism in our tests
	// run. The To* resolvers are stored in a map in our graphql
	// implementation => the order we call them is non deterministic =>
	// code coverage reports are noisy.
	resolvers := []any{
		&FileMatchResolver{db: db},
		&NamespaceResolver{},
		&NodeResolver{},
		&RepositoryResolver{db: db, logger: logtest.Scoped(t)},
		&CommitSearchResultResolver{},
		&gitRevSpec{},
		&settingsSubjectResolver{},
		&statusMessageResolver{db: db},
	}

	re := regexp.MustCompile("To[A-Z]")

	for _, r := range resolvers {
		typ := reflect.TypeOf(r)
		t.Run(typ.Name(), func(t *testing.T) {
			for i := range typ.NumMethod() {
				if name := typ.Method(i).Name; re.MatchString(name) {
					reflect.ValueOf(r).MethodByName(name).Call(nil)
				}
			}
		})
	}

	t.Run("GitTreeEntryResolver", func(t *testing.T) {
		blobStat, err := os.Stat("graphqlbackend_test.go")
		if err != nil {
			t.Fatalf("unexpected error opening file: %s", err)
		}
		blobEntry := &GitTreeEntryResolver{db: db, stat: blobStat}
		if _, isBlob := blobEntry.ToGitBlob(); !isBlob {
			t.Errorf("expected blobEntry to be blob")
		}
		if _, isTree := blobEntry.ToGitTree(); isTree {
			t.Errorf("expected blobEntry to be blob, but is tree")
		}

		treeStat, err := os.Stat(".")
		if err != nil {
			t.Fatalf("unexpected error opening directory: %s", err)
		}
		treeEntry := &GitTreeEntryResolver{db: db, stat: treeStat}
		if _, isBlob := treeEntry.ToGitBlob(); isBlob {
			t.Errorf("expected treeEntry to be tree, but is blob")
		}
		if _, isTree := treeEntry.ToGitTree(); !isTree {
			t.Errorf("expected treeEntry to be tree")
		}
	})
}

func boolPointer(b bool) *bool {
	return &b
}
