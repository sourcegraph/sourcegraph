package graphqlbackend

import (
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/graph-gophers/graphql-go/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/fileutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestGitCommitResolver(t *testing.T) {
	ctx := context.Background()
	db := dbmocks.NewMockDB()

	client := gitserver.NewMockClient()

	commit := &gitdomain.Commit{
		ID:      "c1",
		Message: "subject: Changes things\nBody of changes",
		Parents: []api.CommitID{"p1", "p2"},
		Author: gitdomain.Signature{
			Name:  "Bob",
			Email: "bob@alice.com",
			Date:  time.Now(),
		},
		Committer: &gitdomain.Signature{
			Name:  "Alice",
			Email: "alice@bob.com",
			Date:  time.Now(),
		},
	}

	t.Run("URL Escaping", func(t *testing.T) {
		repo := NewRepositoryResolver(db, client, &types.Repo{Name: "xyz"})
		commitResolver := NewGitCommitResolver(db, client, repo, "c1", commit)
		{
			inputRev := "master^1"
			commitResolver.inputRev = &inputRev
			require.Equal(t, "/xyz/-/commit/master%5E1", commitResolver.URL())

			opts := GitTreeEntryResolverOpts{
				Commit: commitResolver,
				Stat:   CreateFileInfo("a/b", false),
			}
			treeResolver := NewGitTreeEntryResolver(db, client, opts)
			url, err := treeResolver.URL(ctx)
			require.Nil(t, err)
			require.Equal(t, "/xyz@master%5E1/-/blob/a/b", url)
		}
		{
			inputRev := "refs/heads/main"
			commitResolver.inputRev = &inputRev
			require.Equal(t, "/xyz/-/commit/refs/heads/main", commitResolver.URL())
		}
	})

	t.Run("Lazy loading", func(t *testing.T) {
		repo := &types.Repo{
			ID:           1,
			Name:         "bob-repo",
			ExternalRepo: api.ExternalRepoSpec{ServiceType: extsvc.TypeGitHub},
		}

		repos := dbmocks.NewMockRepoStore()
		repos.GetFunc.SetDefaultReturn(repo, nil)
		db.ReposFunc.SetDefaultReturn(repos)

		client := gitserver.NewMockClient()
		client.GetCommitFunc.SetDefaultHook(func(context.Context, api.RepoName, api.CommitID) (*gitdomain.Commit, error) {
			return commit, nil
		})

		for _, tc := range []struct {
			name string
			want any
			have func(*GitCommitResolver) (any, error)
		}{{
			name: "author",
			want: toSignatureResolver(db, &commit.Author, true),
			have: func(r *GitCommitResolver) (any, error) {
				return r.Author(ctx)
			},
		}, {
			name: "committer",
			want: toSignatureResolver(db, commit.Committer, true),
			have: func(r *GitCommitResolver) (any, error) {
				return r.Committer(ctx)
			},
		}, {
			name: "message",
			want: string(commit.Message),
			have: func(r *GitCommitResolver) (any, error) {
				return r.Message(ctx)
			},
		}, {
			name: "subject",
			want: "subject: Changes things",
			have: func(r *GitCommitResolver) (any, error) {
				return r.Subject(ctx)
			},
		}, {
			name: "body",
			want: "Body of changes",
			have: func(r *GitCommitResolver) (any, error) {
				s, err := r.Body(ctx)
				return *s, err
			},
		}, {
			name: "url",
			want: "/bob-repo/-/commit/c1",
			have: func(r *GitCommitResolver) (any, error) {
				return r.URL(), nil
			},
		}, {
			name: "canonical-url",
			want: "/bob-repo/-/commit/c1",
			have: func(r *GitCommitResolver) (any, error) {
				return r.CanonicalURL(), nil
			},
		}} {
			t.Run(tc.name, func(t *testing.T) {
				repo := NewRepositoryResolver(db, client, repo)
				// We pass no commit here to test that it gets lazy loaded via
				// the git.GetCommit mock above.
				r := NewGitCommitResolver(db, client, repo, "c1", nil)

				have, err := tc.have(r)
				if err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(have, tc.want) {
					t.Errorf("\nhave: %s\nwant: %s", spew.Sprint(have), spew.Sprint(tc.want))
				}

				source, err := r.repoResolver.SourceType(ctx)
				require.NoError(t, err)
				require.Equal(t, GitRepositorySourceType, *source)

				pf, err := r.PerforceChangelist(ctx)
				require.NoError(t, err)
				require.Nil(t, pf)

				f, err := ioutil.TempFile("/tmp", "foo")
				require.NoError(t, err)

				fs, err := f.Stat()
				require.NoError(t, err)
				client.StatFunc.SetDefaultReturn(fs, nil)

				path, err := filepath.Abs(filepath.Dir(f.Name()))
				require.NoError(t, err)

				gitTree, err := r.Blob(ctx, &struct{ Path string }{Path: path})
				require.NoError(t, err)
				require.NotNil(t, gitTree)

				cl, err := gitTree.ChangelistURL(ctx)
				require.NoError(t, err)
				require.Nil(t, cl)
			})
		}
	})

	runPerforceTests := func(t *testing.T, commit *gitdomain.Commit) {
		repo := &types.Repo{
			ID:           1,
			Name:         "perforce/test-depot",
			ExternalRepo: api.ExternalRepoSpec{ServiceType: extsvc.TypePerforce},
		}

		repoResolver := NewRepositoryResolver(db, client, repo)

		repos := dbmocks.NewMockRepoStore()
		repos.GetFunc.SetDefaultReturn(repo, nil)
		db.ReposFunc.SetDefaultReturn(repos)

		commitResolver := NewGitCommitResolver(db, client, repoResolver, "8aa15f6a85c07a882821053f361b538f404f238e", commit)

		ctx := actor.WithInternalActor(context.Background())

		source, err := commitResolver.repoResolver.SourceType(ctx)
		require.NoError(t, err)

		require.Equal(t, PerforceDepotSourceType, *source)

		pf, err := commitResolver.PerforceChangelist(ctx)
		require.NoError(t, err)
		require.NotNil(t, pf)

		require.Equal(t, "123", pf.cid)
		subject, err := commitResolver.Subject(ctx)
		require.NoError(t, err)
		require.Equal(t, "subject: Changes things", subject)

		f, err := ioutil.TempFile("/tmp", "foo")
		require.NoError(t, err)

		fs, err := f.Stat()
		require.NoError(t, err)
		client.StatFunc.SetDefaultReturn(fs, nil)

		path, err := filepath.Abs(filepath.Dir(f.Name()))
		require.NoError(t, err)

		gitTree, err := commitResolver.Blob(ctx, &struct{ Path string }{Path: path})
		require.NoError(t, err)

		gotURL, err := gitTree.ChangelistURL(ctx)
		require.NoError(t, err)

		_, fileName := filepath.Split(f.Name())
		require.Equal(
			t,
			filepath.Join("/perforce/test-depot@123/-/blob", fileName),
			*gotURL,
		)
	}

	t.Run("perforce depot, git-p4 commit", func(t *testing.T) {
		commit := &gitdomain.Commit{
			ID: "c1",
			Message: `subject: Changes things
[git-p4: depot-paths = "//test-depot/": change = 123]"`,
			Parents: []api.CommitID{"p1", "p2"},
			Author: gitdomain.Signature{
				Name:  "Bob",
				Email: "bob@alice.com",
			},
			Committer: &gitdomain.Signature{
				Name:  "Alice",
				Email: "alice@bob.com",
			},
		}

		runPerforceTests(t, commit)
	})

	t.Run("perforce depot, p4-fusion commit", func(t *testing.T) {
		commit := &gitdomain.Commit{
			ID: "c1",
			Message: `123 - subject: Changes things
[p4-fusion: depot-paths = "//test-perms/": change = 123]"`,
			Parents: []api.CommitID{"p1", "p2"},
			Author: gitdomain.Signature{
				Name:  "Bob",
				Email: "bob@alice.com",
			},
			Committer: &gitdomain.Signature{
				Name:  "Alice",
				Email: "alice@bob.com",
			},
		}

		runPerforceTests(t, commit)
	})
}

func TestGitCommitFileNames(t *testing.T) {
	externalServices := dbmocks.NewMockExternalServiceStore()
	externalServices.ListFunc.SetDefaultReturn(nil, nil)

	repos := dbmocks.NewMockRepoStore()
	repos.GetByNameFunc.SetDefaultReturn(&types.Repo{ID: 2, Name: "github.com/gorilla/mux"}, nil)

	db := dbmocks.NewMockDB()
	db.ExternalServicesFunc.SetDefaultReturn(externalServices)
	db.ReposFunc.SetDefaultReturn(repos)

	backend.Mocks.Repos.ResolveRev = func(ctx context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		assert.Equal(t, exampleCommitSHA1, rev)
		return exampleCommitSHA1, nil
	}
	backend.Mocks.Repos.MockGetCommit_Return_NoCheck(t, &gitdomain.Commit{ID: exampleCommitSHA1})
	gitserverClient := gitserver.NewMockClient()
	gitserverClient.ReadDirFunc.SetDefaultReturn([]fs.FileInfo{
		&fileutil.FileInfo{Name_: "a"},
		&fileutil.FileInfo{Name_: "b"},
		// We also return a dir to check that it's skipped in the output.
		&fileutil.FileInfo{Name_: "dir", Mode_: os.ModeDir},
	}, nil)
	defer func() {
		backend.Mocks = backend.MockServices{}
	}()

	RunTests(t, []*Test{
		{
			Schema: mustParseGraphQLSchemaWithClient(t, db, gitserverClient),
			Query: `
				{
					repository(name: "github.com/gorilla/mux") {
						commit(rev: "` + exampleCommitSHA1 + `") {
							fileNames
						}
					}
				}
			`,
			ExpectedResult: `
{
  "repository": {
    "commit": {
		"fileNames": ["a", "b"]
    }
  }
}
			`,
		},
	})
}

func TestGitCommitAncestors(t *testing.T) {
	repos := dbmocks.NewMockRepoStore()
	repos.GetFunc.SetDefaultReturn(&types.Repo{ID: 2, Name: "github.com/gorilla/mux"}, nil)

	db := dbmocks.NewMockDB()
	db.ReposFunc.SetDefaultReturn(repos)

	backend.Mocks.Repos.ResolveRev = func(ctx context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		return api.CommitID(rev), nil
	}

	backend.Mocks.Repos.MockGetCommit_Return_NoCheck(t, &gitdomain.Commit{ID: exampleCommitSHA1})

	client := gitserver.NewMockClient()
	client.ReadDirFunc.SetDefaultReturn([]fs.FileInfo{
		&fileutil.FileInfo{Name_: "a"},
		&fileutil.FileInfo{Name_: "b"},
		// We also return a dir to check that it's skipped in the output.
		&fileutil.FileInfo{Name_: "dir", Mode_: os.ModeDir},
	}, nil)

	// A linear commit tree:
	// * -> c1 -> c2 -> c3 -> c4 -> c5 (HEAD)
	c1 := gitdomain.Commit{
		ID: api.CommitID("aabbc12345"),
	}
	c2 := gitdomain.Commit{
		ID:      api.CommitID("ccdde12345"),
		Parents: []api.CommitID{c1.ID},
	}
	c3 := gitdomain.Commit{
		ID:      api.CommitID("eeffg12345"),
		Parents: []api.CommitID{c2.ID},
	}
	c4 := gitdomain.Commit{
		ID:      api.CommitID("gghhi12345"),
		Parents: []api.CommitID{c3.ID},
	}
	c5 := gitdomain.Commit{
		ID:      api.CommitID("ijklm12345"),
		Parents: []api.CommitID{c4.ID},
	}

	commits := []*gitdomain.Commit{
		&c1, &c2, &c3, &c4, &c5,
	}

	client.CommitsFunc.SetDefaultHook(func(
		ctx context.Context,
		repo api.RepoName,
		opt gitserver.CommitsOptions) ([]*gitdomain.Commit, error) {

		// Offset the returned list of commits based on the value of the Skip option.
		return commits[opt.Skip:], nil
	})

	defer func() {
		backend.Mocks = backend.MockServices{}
	}()

	RunTests(t, []*Test{
		// Invalid value for afterCursor.
		// Expect errors and no result.
		{
			Schema: mustParseGraphQLSchemaWithClient(t, db, client),
			Query: `
				{
				  repository(name: "github.com/gorilla/mux") {
					commit(rev: "aabbc12345") {
					  ancestors(first: 2, path: "bill-of-materials.json", afterCursor: "n") {
						nodes {
						  id
						  oid
						  abbreviatedOID
						}
						pageInfo {
						  endCursor
						  hasNextPage
						}
					  }
					}
				  }
				}`,
			ExpectedErrors: []*errors.QueryError{
				{
					Message: "failed to parse afterCursor: strconv.Atoi: parsing \"n\": invalid syntax",
					Path:    []any{"repository", "commit", "ancestors", "nodes"},
				},
				{
					Message: "failed to parse afterCursor: strconv.Atoi: parsing \"n\": invalid syntax",
					Path:    []any{"repository", "commit", "ancestors", "pageInfo"},
				},
			},
			ExpectedResult: `
				{
				  "repository": {
					"commit": null
				  }
				}`,
		},

		// When first:0 and commits exist.
		// Expect no nodes, but hasNextPage: true.
		{
			Schema: mustParseGraphQLSchemaWithClient(t, db, client),
			Query: `
				{
				  repository(name: "github.com/gorilla/mux") {
					commit(rev: "aabbc12345") {
					  ancestors(first: 0, path: "bill-of-materials.json") {
						nodes {
						  id
						  oid
						  abbreviatedOID
						  perforceChangelist {
							cid
                            canonicalURL
						  }
						}
						pageInfo {
						  endCursor
						  hasNextPage
						}
					  }
					}
				  }
				}`,
			ExpectedResult: `
				{
				  "repository": {
					"commit": {
					  "ancestors": {
						"nodes": [],
						"pageInfo": {
						  "endCursor": "0",
						  "hasNextPage": true
						}
					  }
					}
				  }
				}`,
		},

		// When first:0 and afterCursor: 5, no commits exist.
		// Expect no nodes, but hasNextPage: false.
		{
			Schema: mustParseGraphQLSchemaWithClient(t, db, client),
			Query: `
				{
				  repository(name: "github.com/gorilla/mux") {
					commit(rev: "aabbc12345") {
					  ancestors(first: 0, path: "bill-of-materials.json", afterCursor: "5") {
						nodes {
						  id
						  oid
						  abbreviatedOID
						}
						pageInfo {
						  endCursor
						  hasNextPage
						}
					  }
					}
				  }
				}`,
			ExpectedResult: `
				{
				  "repository": {
					"commit": {
					  "ancestors": {
						"nodes": [],
						"pageInfo": {
                          "endCursor": null,
						  "hasNextPage": false
						}
					  }
					}
				  }
				}`,
		},

		// Start at commit c1.
		// Expect c1 and c2 in the nodes. 2 in the endCursor.
		{
			Schema: mustParseGraphQLSchemaWithClient(t, db, client),
			Query: `
				{
				  repository(name: "github.com/gorilla/mux") {
					commit(rev: "aabbc12345") {
					  ancestors(first: 2, path: "bill-of-materials.json") {
						nodes {
						  id
						  oid
						  abbreviatedOID
						}
						pageInfo {
						  endCursor
						  hasNextPage
						}
					  }
					}
				  }
				}`,
			ExpectedResult: `
				{
				  "repository": {
					"commit": {
					  "ancestors": {
						"nodes": [
						  {
							"id": "R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3ciLCJjIjoiYWFiYmMxMjM0NSJ9",
							"oid": "aabbc12345",
							"abbreviatedOID": "aabbc12"
						  },
						  {
							"id": "R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3ciLCJjIjoiY2NkZGUxMjM0NSJ9",
							"oid": "ccdde12345",

							"abbreviatedOID": "ccdde12"
						  }
						],
						"pageInfo": {
						  "endCursor": "2",
						  "hasNextPage": true
						}
					  }
					}
				  }
				}`,
		},

		// Start at commit c1 with afterCursor:1.
		// Expect c2 and c3 in the nodes. 3 in the endCursor.
		{
			Schema: mustParseGraphQLSchemaWithClient(t, db, client),
			Query: `
				{
				  repository(name: "github.com/gorilla/mux") {
					commit(rev: "aabbc12345") {
					  ancestors(first: 2, path: "bill-of-materials.json", afterCursor: "1") {
						nodes {
						  id
						  oid
						  abbreviatedOID
						}
						pageInfo {
						  endCursor
						  hasNextPage
						}
					  }
					}
				  }
				}`,
			ExpectedResult: `
				{
				  "repository": {
					"commit": {
					  "ancestors": {
						"nodes": [
						  {
							"id": "R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3ciLCJjIjoiY2NkZGUxMjM0NSJ9",
							"oid": "ccdde12345",

							"abbreviatedOID": "ccdde12"
						  },
						  {
							"id": "R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3ciLCJjIjoiZWVmZmcxMjM0NSJ9",
							"oid": "eeffg12345",
							"abbreviatedOID": "eeffg12"
						  }
						],
						"pageInfo": {
						  "endCursor": "3",
						  "hasNextPage": true
						}
					  }
					}
				  }
				}`,
		},

		// Start at commit c1 with afterCursor:2
		// Expect c3, c4, c5 in the nodes. No endCursor because there will be no new commits.
		{
			Schema: mustParseGraphQLSchemaWithClient(t, db, client),
			Query: `
				{
				  repository(name: "github.com/gorilla/mux") {
					commit(rev: "aabbc12345") {
					  ancestors(first: 3, path: "bill-of-materials.json", afterCursor: "2") {
						nodes {
						  id
						  oid
						  abbreviatedOID
						}
						pageInfo {
						  endCursor
						  hasNextPage
						}
					  }
					}
				  }
				}`,
			ExpectedResult: `
				{
				  "repository": {
					"commit": {
					  "ancestors": {
						"nodes": [
						  {
							"id": "R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3ciLCJjIjoiZWVmZmcxMjM0NSJ9",
							"oid": "eeffg12345",
							"abbreviatedOID": "eeffg12"
						  },
						  {
							"id": "R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3ciLCJjIjoiZ2doaGkxMjM0NSJ9",
							"oid": "gghhi12345",
							"abbreviatedOID": "gghhi12"
						  },
						  {
							"id": "R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3ciLCJjIjoiaWprbG0xMjM0NSJ9",
							"oid": "ijklm12345",
							"abbreviatedOID": "ijklm12"
						  }
						],
						"pageInfo": {
						  "endCursor": null,
						  "hasNextPage": false
						}
					  }
					}
				  }
				}`,
		},
	})
}

func TestGitCommitPerforceChangelist(t *testing.T) {
	repos := dbmocks.NewMockRepoStore()

	db := dbmocks.NewMockDB()
	db.ReposFunc.SetDefaultReturn(repos)

	backend.Mocks.Repos.ResolveRev = func(ctx context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		return api.CommitID(rev), nil
	}

	backend.Mocks.Repos.MockGetCommit_Return_NoCheck(t, &gitdomain.Commit{ID: exampleCommitSHA1})

	client := gitserver.NewMockClient()

	t.Run("git repo", func(t *testing.T) {
		repos.GetFunc.SetDefaultReturn(
			&types.Repo{
				ID:   2,
				Name: "github.com/gorilla/mux",
				ExternalRepo: api.ExternalRepoSpec{
					ServiceType: extsvc.TypeGitHub,
				},
			},
			nil,
		)

		c1 := gitdomain.Commit{
			ID:      api.CommitID("aabbc12345"),
			Message: gitdomain.Message(`adding sourcegraph repos`),
		}

		client.CommitsFunc.SetDefaultReturn([]*gitdomain.Commit{&c1}, nil)

		RunTest(t, &Test{
			Schema: mustParseGraphQLSchemaWithClient(t, db, client),
			Query: `
				{
				  repository(name: "github.com/gorilla/mux") {
						commit(rev: "aabbc12345") {
							ancestors(first: 10) {
								nodes {
									id
									oid
									perforceChangelist {
										cid
                                        canonicalURL
									}
								}
							}
						}
				  }
				}`,
			ExpectedResult: `
				{
				  "repository": {
						"commit": {
							"ancestors": {
								"nodes": [
									{
										"id": "R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3ciLCJjIjoiYWFiYmMxMjM0NSJ9",
										"oid": "aabbc12345",
										"perforceChangelist": null
									}
								]
							}
						}
				  }
				}`,
		})
	})

	t.Run("perforce depot", func(t *testing.T) {
		repo := &types.Repo{
			ID:           2,
			Name:         "github.com/gorilla/mux",
			ExternalRepo: api.ExternalRepoSpec{ServiceType: extsvc.TypePerforce},
		}

		repos.GetFunc.SetDefaultReturn(repo, nil)
		repos.GetByNameFunc.SetDefaultReturn(repo, nil)

		// git-p4 commit.
		c1 := gitdomain.Commit{
			ID: api.CommitID("aabbc12345"),
			Message: gitdomain.Message(`87654 - adding sourcegraph repos
[git-p4: depot-paths = "//test-perms/": change = 87654]`),
		}

		// p4-fusion commit.
		c2 := gitdomain.Commit{
			ID: api.CommitID("ccdde12345"),
			Message: gitdomain.Message(`87655 - testing sourcegraph repos
[p4-fusion: depot-paths = "//test-perms/": change = 87655]`),
		}

		client.CommitsFunc.SetDefaultReturn([]*gitdomain.Commit{&c1, &c2}, nil)

		RunTest(t, &Test{
			Schema: mustParseGraphQLSchemaWithClient(t, db, client),
			Query: `
				{
				  repository(name: "github.com/gorilla/mux") {
						commit(rev: "aabbc12345") {
							ancestors(first: 10) {
								nodes {
									id
									oid
									perforceChangelist {
										cid
                                        canonicalURL
									}
								}
							}
						}
				  }
				}`,
			ExpectedResult: `
				{
				  "repository": {
						"commit": {
							"ancestors": {
								"nodes": [
									{
										"id": "R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3kiLCJjIjoiYWFiYmMxMjM0NSJ9",
										"oid": "aabbc12345",
										"perforceChangelist": {
											"cid": "87654",
											"canonicalURL": "/github.com/gorilla/mux/-/changelist/87654"
										}
									},
									{
										"id": "R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3kiLCJjIjoiY2NkZGUxMjM0NSJ9",
										"oid": "ccdde12345",
										"perforceChangelist": {
											"cid": "87655",
											"canonicalURL": "/github.com/gorilla/mux/-/changelist/87655"
										}
									}
								]
							}
						}
				  }
				}`,
		})
	})
}
