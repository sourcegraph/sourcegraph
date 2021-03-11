package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/testutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestProjectQueryToURL(t *testing.T) {
	tests := []struct {
		projectQuery string
		perPage      int
		expURL       string
		expErr       error
	}{{
		projectQuery: "?membership=true",
		perPage:      100,
		expURL:       "projects?membership=true&per_page=100",
	}, {
		projectQuery: "projects?membership=true",
		perPage:      100,
		expURL:       "projects?membership=true&per_page=100",
	}, {
		projectQuery: "groups/groupID/projects",
		perPage:      100,
		expURL:       "groups/groupID/projects?per_page=100",
	}, {
		projectQuery: "groups/groupID/projects?foo=bar",
		perPage:      100,
		expURL:       "groups/groupID/projects?foo=bar&per_page=100",
	}, {
		projectQuery: "",
		perPage:      100,
		expURL:       "projects?per_page=100",
	}, {
		projectQuery: "https://somethingelse.com/foo/bar",
		perPage:      100,
		expErr:       schemeOrHostNotEmptyErr,
	}}

	for _, test := range tests {
		t.Logf("Test case %+v", test)
		url, err := projectQueryToURL(test.projectQuery, test.perPage)
		if url != test.expURL {
			t.Errorf("expected %v, got %v", test.expURL, url)
		}
		if !reflect.DeepEqual(test.expErr, err) {
			t.Errorf("expected err %v, got %v", test.expErr, err)
		}
	}
}

func TestGitLabSource_GetRepo(t *testing.T) {
	testCases := []struct {
		name                 string
		projectWithNamespace string
		assert               func(*testing.T, *types.Repo)
		err                  string
	}{
		{
			name:                 "not found",
			projectWithNamespace: "foobarfoobarfoobar/please-let-this-not-exist",
			err:                  `unexpected response from GitLab API (https://gitlab.com/api/v4/projects/foobarfoobarfoobar%2Fplease-let-this-not-exist): HTTP error status 404`,
		},
		{
			name:                 "found",
			projectWithNamespace: "gitlab-org/gitaly",
			assert: func(t *testing.T, have *types.Repo) {
				t.Helper()

				want := &types.Repo{
					Name:        "gitlab.com/gitlab-org/gitaly",
					Description: "Gitaly is a Git RPC service for handling all the git calls made by GitLab",
					URI:         "gitlab.com/gitlab-org/gitaly",
					ExternalRepo: api.ExternalRepoSpec{
						ID:          "2009901",
						ServiceType: "gitlab",
						ServiceID:   "https://gitlab.com/",
					},
					Sources: map[string]*types.SourceInfo{
						"extsvc:gitlab:0": {
							ID:       "extsvc:gitlab:0",
							CloneURL: "https://gitlab.com/gitlab-org/gitaly.git",
						},
					},
					Metadata: &gitlab.Project{
						ProjectCommon: gitlab.ProjectCommon{
							ID:                2009901,
							PathWithNamespace: "gitlab-org/gitaly",
							Description:       "Gitaly is a Git RPC service for handling all the git calls made by GitLab",
							WebURL:            "https://gitlab.com/gitlab-org/gitaly",
							HTTPURLToRepo:     "https://gitlab.com/gitlab-org/gitaly.git",
							SSHURLToRepo:      "git@gitlab.com:gitlab-org/gitaly.git",
						},
						Visibility: "",
						Archived:   false,
					},
				}

				if !reflect.DeepEqual(have, want) {
					t.Errorf("response: %s", cmp.Diff(have, want))
				}
			},
			err: "<nil>",
		},
	}

	for _, tc := range testCases {
		tc := tc
		tc.name = "GITLAB-DOT-COM/" + tc.name

		t.Run(tc.name, func(t *testing.T) {
			// The GitLabSource uses the gitlab.Client under the hood, which
			// uses rcache, a caching layer that uses Redis.
			// We need to clear the cache before we run the tests
			rcache.SetupForTest(t)

			cf, save := newClientFactory(t, tc.name)
			defer save(t)

			lg := log15.New()
			lg.SetHandler(log15.DiscardHandler())

			svc := &types.ExternalService{
				Kind: extsvc.KindGitLab,
				Config: marshalJSON(t, &schema.GitLabConnection{
					Url: "https://gitlab.com",
				}),
			}

			gitlabSrc, err := NewGitLabSource(svc, cf)
			if err != nil {
				t.Fatal(err)
			}

			repo, err := gitlabSrc.GetRepo(context.Background(), tc.projectWithNamespace)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if tc.assert != nil {
				tc.assert(t, repo)
			}
		})
	}
}

func TestGitLabSource_makeRepo(t *testing.T) {
	b, err := ioutil.ReadFile(filepath.Join("testdata", "gitlab-repos.json"))
	if err != nil {
		t.Fatal(err)
	}
	var repos []*gitlab.Project
	if err := json.Unmarshal(b, &repos); err != nil {
		t.Fatal(err)
	}

	svc := types.ExternalService{ID: 1, Kind: extsvc.KindGitLab}

	tests := []struct {
		name   string
		schmea *schema.GitLabConnection
	}{
		{
			name: "simple",
			schmea: &schema.GitLabConnection{
				Url: "https://gitlab.com",
			},
		}, {
			name: "ssh",
			schmea: &schema.GitLabConnection{
				Url:        "https://gitlab.com",
				GitURLType: "ssh",
			},
		}, {
			name: "path-pattern",
			schmea: &schema.GitLabConnection{
				Url:                   "https://gitlab.com",
				RepositoryPathPattern: "gl/{pathWithNamespace}",
			},
		},
	}
	for _, test := range tests {
		test.name = "GitLabSource_makeRepo_" + test.name
		t.Run(test.name, func(t *testing.T) {
			lg := log15.New()
			lg.SetHandler(log15.DiscardHandler())

			s, err := newGitLabSource(&svc, test.schmea, nil)
			if err != nil {
				t.Fatal(err)
			}

			var got []*types.Repo
			for _, r := range repos {
				got = append(got, s.makeRepo(r))
			}

			testutil.AssertGolden(t, "testdata/golden/"+test.name, update(test.name), got)
		})
	}
}

// TestGitLabSource_ChangesetSource tests the various Changeset functions that
// implement the ChangesetSource interface.
func TestGitLabSource_ChangesetSource(t *testing.T) {
	t.Run("CreateChangeset", func(t *testing.T) {
		t.Run("invalid metadata", func(t *testing.T) {
			defer func() { _ = recover() }()

			p := newGitLabChangesetSourceTestProvider(t)
			_, _ = p.source.CreateChangeset(p.ctx, &Changeset{
				Repo: &types.Repo{
					Metadata: struct{}{},
				},
			})
			t.Error("invalid metadata did not panic")
		})

		t.Run("error from CreateMergeRequest", func(t *testing.T) {
			inner := errors.New("foo")

			p := newGitLabChangesetSourceTestProvider(t)
			p.mockCreateMergeRequest(gitlab.CreateMergeRequestOpts{
				SourceBranch: p.mr.SourceBranch,
				TargetBranch: p.mr.TargetBranch,
			}, nil, inner)

			exists, have := p.source.CreateChangeset(p.ctx, p.changeset)
			if exists {
				t.Errorf("unexpected exists value: %v", exists)
			}
			if !errors.Is(have, inner) {
				t.Errorf("error does not include inner error: have %+v; want %+v", have, inner)
			}
		})

		t.Run("error from GetOpenMergeRequestByRefs", func(t *testing.T) {
			inner := errors.New("foo")

			p := newGitLabChangesetSourceTestProvider(t)
			p.mockCreateMergeRequest(gitlab.CreateMergeRequestOpts{
				SourceBranch: p.mr.SourceBranch,
				TargetBranch: p.mr.TargetBranch,
			}, nil, gitlab.ErrMergeRequestAlreadyExists)
			p.mockGetOpenMergeRequestByRefs(nil, inner)

			exists, have := p.source.CreateChangeset(p.ctx, p.changeset)
			if !exists {
				t.Errorf("unexpected exists value: %v", exists)
			}
			if !errors.Is(have, inner) {
				t.Errorf("error does not include inner error: have %+v; want %+v", have, inner)
			}
		})

		t.Run("merge request already exists", func(t *testing.T) {
			p := newGitLabChangesetSourceTestProvider(t)
			p.mockCreateMergeRequest(gitlab.CreateMergeRequestOpts{
				SourceBranch: p.mr.SourceBranch,
				TargetBranch: p.mr.TargetBranch,
			}, nil, gitlab.ErrMergeRequestAlreadyExists)
			p.mockGetMergeRequestNotes(p.mr.IID, nil, 20, nil)
			p.mockGetMergeRequestResourceStateEvents(p.mr.IID, nil, 20, nil)
			p.mockGetMergeRequestPipelines(p.mr.IID, nil, 20, nil)
			p.mockGetOpenMergeRequestByRefs(p.mr, nil)

			exists, err := p.source.CreateChangeset(p.ctx, p.changeset)
			if !exists {
				t.Errorf("unexpected exists value: %v", exists)
			}
			if err != nil {
				t.Errorf("unexpected non-nil err: %+v", err)
			}

			if p.changeset.Changeset.Metadata != p.mr {
				t.Errorf("unexpected metadata: have %+v; want %+v", p.changeset.Changeset.Metadata, p.mr)
			}
		})

		t.Run("merge request is new", func(t *testing.T) {
			p := newGitLabChangesetSourceTestProvider(t)
			p.mockCreateMergeRequest(gitlab.CreateMergeRequestOpts{
				SourceBranch: p.mr.SourceBranch,
				TargetBranch: p.mr.TargetBranch,
			}, p.mr, nil)
			p.mockGetMergeRequestNotes(p.mr.IID, nil, 20, nil)
			p.mockGetMergeRequestResourceStateEvents(p.mr.IID, nil, 20, nil)
			p.mockGetMergeRequestPipelines(p.mr.IID, nil, 20, nil)

			exists, err := p.source.CreateChangeset(p.ctx, p.changeset)
			if exists {
				t.Errorf("unexpected exists value: %v", exists)
			}
			if err != nil {
				t.Errorf("unexpected non-nil err: %+v", err)
			}

			if p.changeset.Changeset.Metadata != p.mr {
				t.Errorf("unexpected metadata: have %+v; want %+v", p.changeset.Changeset.Metadata, p.mr)
			}
		})
	})

	t.Run("CloseChangeset", func(t *testing.T) {
		t.Run("invalid metadata", func(t *testing.T) {
			defer func() { _ = recover() }()

			p := newGitLabChangesetSourceTestProvider(t)
			_ = p.source.CloseChangeset(p.ctx, &Changeset{
				Repo: &types.Repo{
					Metadata: struct{}{},
				},
			})
			t.Error("invalid metadata did not panic")
		})

		t.Run("error from UpdateMergeRequest", func(t *testing.T) {
			inner := errors.New("foo")
			mr := &gitlab.MergeRequest{}

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.Metadata = mr
			p.mockUpdateMergeRequest(mr, nil, gitlab.UpdateMergeRequestStateEventClose, inner)

			have := p.source.CloseChangeset(p.ctx, p.changeset)
			if !errors.Is(have, inner) {
				t.Errorf("error does not include inner error: have %+v; want %+v", have, inner)
			}
		})

		t.Run("success", func(t *testing.T) {
			want := &gitlab.MergeRequest{}
			mr := &gitlab.MergeRequest{IID: 2}

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.Metadata = mr
			p.mockUpdateMergeRequest(mr, want, gitlab.UpdateMergeRequestStateEventClose, nil)
			p.mockGetMergeRequestNotes(mr.IID, nil, 20, nil)
			p.mockGetMergeRequestResourceStateEvents(mr.IID, nil, 20, nil)
			p.mockGetMergeRequestPipelines(mr.IID, nil, 20, nil)

			if err := p.source.CloseChangeset(p.ctx, p.changeset); err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
		})
	})

	t.Run("ReopenChangeset", func(t *testing.T) {
		t.Run("invalid metadata", func(t *testing.T) {
			defer func() { _ = recover() }()

			p := newGitLabChangesetSourceTestProvider(t)
			_ = p.source.ReopenChangeset(p.ctx, &Changeset{
				Repo: &types.Repo{
					Metadata: struct{}{},
				},
			})
			t.Error("invalid metadata did not panic")
		})

		t.Run("error from UpdateMergeRequest", func(t *testing.T) {
			inner := errors.New("foo")
			mr := &gitlab.MergeRequest{}

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.Metadata = mr
			p.mockUpdateMergeRequest(mr, nil, gitlab.UpdateMergeRequestStateEventReopen, inner)

			have := p.source.ReopenChangeset(p.ctx, p.changeset)
			if !errors.Is(have, inner) {
				t.Errorf("error does not include inner error: have %+v; want %+v", have, inner)
			}
		})

		t.Run("success", func(t *testing.T) {
			want := &gitlab.MergeRequest{}
			mr := &gitlab.MergeRequest{IID: 2}

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.Metadata = mr
			p.mockUpdateMergeRequest(mr, want, gitlab.UpdateMergeRequestStateEventReopen, nil)
			p.mockGetMergeRequestNotes(mr.IID, nil, 20, nil)
			// TODO: add event
			p.mockGetMergeRequestResourceStateEvents(mr.IID, nil, 20, nil)
			p.mockGetMergeRequestPipelines(mr.IID, nil, 20, nil)

			if err := p.source.ReopenChangeset(p.ctx, p.changeset); err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
		})
	})

	t.Run("LoadChangeset", func(t *testing.T) {
		t.Run("invalid metadata", func(t *testing.T) {
			defer func() { _ = recover() }()

			p := newGitLabChangesetSourceTestProvider(t)

			_ = p.source.LoadChangeset(p.ctx, &Changeset{
				Repo: &types.Repo{Metadata: struct{}{}},
			})
			t.Error("invalid metadata did not panic")
		})

		t.Run("error from ParseInt", func(t *testing.T) {
			p := newGitLabChangesetSourceTestProvider(t)
			if err := p.source.LoadChangeset(p.ctx, &Changeset{
				Changeset: &batches.Changeset{
					ExternalID: "foo",
					Metadata:   &gitlab.MergeRequest{},
				},
				Repo: &types.Repo{Metadata: &gitlab.Project{}},
			}); err == nil {
				t.Error("invalid ExternalID did not result in an error")
			}
		})

		t.Run("error from GetMergeRequest", func(t *testing.T) {
			inner := errors.New("foo")

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.ExternalID = "42"
			p.changeset.Changeset.Metadata = p.mr
			p.mockGetMergeRequest(42, nil, inner)
			p.mockGetMergeRequestNotes(42, nil, 20, nil)
			p.mockGetMergeRequestPipelines(42, nil, 20, nil)

			if have := p.source.LoadChangeset(p.ctx, p.changeset); !errors.Is(have, inner) {
				t.Errorf("error does not include inner error: have %+v; want %+v", have, inner)
			}
		})

		t.Run("error from GetMergeRequestNotes", func(t *testing.T) {
			// A new merge request with a new IID.
			mr := &gitlab.MergeRequest{IID: 43}
			inner := errors.New("foo")

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.ExternalID = "42"
			p.changeset.Changeset.Metadata = p.mr
			p.mockGetMergeRequest(42, mr, nil)
			p.mockGetMergeRequestNotes(43, nil, 20, inner)
			p.mockGetMergeRequestResourceStateEvents(43, nil, 20, nil)
			p.mockGetMergeRequestPipelines(43, nil, 20, nil)

			if err := p.source.LoadChangeset(p.ctx, p.changeset); !errors.Is(err, inner) {
				t.Errorf("unexpected error: %+v", err)
			}
			if p.changeset.Changeset.Metadata != p.mr {
				t.Errorf("metadata unexpectedly changed to %+v", p.changeset.Changeset.Metadata)
			}
		})

		t.Run("error from GetMergeRequestResourceStateEvents", func(t *testing.T) {
			// A new merge request with a new IID.
			mr := &gitlab.MergeRequest{IID: 43}
			inner := errors.New("foo")

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.ExternalID = "42"
			p.changeset.Changeset.Metadata = p.mr
			p.mockGetMergeRequest(42, mr, nil)
			p.mockGetMergeRequestNotes(43, nil, 20, nil)
			p.mockGetMergeRequestResourceStateEvents(43, nil, 20, inner)
			p.mockGetMergeRequestPipelines(43, nil, 20, nil)

			if err := p.source.LoadChangeset(p.ctx, p.changeset); !errors.Is(err, inner) {
				t.Errorf("unexpected error: %+v", err)
			}
			if p.changeset.Changeset.Metadata != p.mr {
				t.Errorf("metadata unexpectedly changed to %+v", p.changeset.Changeset.Metadata)
			}
		})

		t.Run("error from GetMergeRequestPipelines", func(t *testing.T) {
			// A new merge request with a new IID.
			mr := &gitlab.MergeRequest{IID: 43}
			inner := errors.New("foo")

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.ExternalID = "42"
			p.changeset.Changeset.Metadata = p.mr
			p.mockGetMergeRequest(42, mr, nil)
			p.mockGetMergeRequestNotes(43, nil, 20, nil)
			p.mockGetMergeRequestResourceStateEvents(43, nil, 20, nil)
			p.mockGetMergeRequestPipelines(43, nil, 20, inner)

			if err := p.source.LoadChangeset(p.ctx, p.changeset); !errors.Is(err, inner) {
				t.Errorf("unexpected error: %+v", err)
			}
			if p.changeset.Changeset.Metadata != p.mr {
				t.Errorf("metadata unexpectedly changed to %+v", p.changeset.Changeset.Metadata)
			}
		})

		t.Run("success", func(t *testing.T) {
			// A new merge request with a new IID.
			mr := &gitlab.MergeRequest{IID: 43}

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.ExternalID = "42"
			p.changeset.Changeset.Metadata = p.mr
			p.mockGetMergeRequest(42, mr, nil)
			p.mockGetMergeRequestNotes(43, nil, 20, nil)
			p.mockGetMergeRequestResourceStateEvents(43, nil, 20, nil)
			p.mockGetMergeRequestPipelines(43, nil, 20, nil)

			if err := p.source.LoadChangeset(p.ctx, p.changeset); err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if have := p.changeset.Changeset.Metadata.(*gitlab.MergeRequest); have != mr {
				t.Errorf("merge request metadata not updated: have %p; want %p", have, mr)
			}
		})

		t.Run("not found", func(t *testing.T) {
			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.ExternalID = "43"
			p.changeset.Changeset.Metadata = p.mr
			p.mockGetMergeRequest(43, nil, gitlab.ErrMergeRequestNotFound)

			if err := p.source.LoadChangeset(p.ctx, p.changeset); err == nil {
				t.Fatal("unexpectedly no error for not found changeset")
			} else if err.Error() != (ChangesetNotFoundError{Changeset: &Changeset{Changeset: &batches.Changeset{ExternalID: "43"}}}).Error() {
				t.Fatalf("unexpected error: %+v", err)
			}
		})

		t.Run("integration", func(t *testing.T) {
			testCases := []struct {
				name string
				cs   *Changeset
				err  string
			}{
				{
					name: "found",
					cs: &Changeset{
						Repo: &types.Repo{Metadata: &gitlab.Project{
							// sourcegraph/sourcegraph
							ProjectCommon: gitlab.ProjectCommon{ID: 16606088},
						}},
						Changeset: &batches.Changeset{ExternalID: "2"},
					},
				},
				{
					name: "not-found",
					cs: &Changeset{
						Repo: &types.Repo{Metadata: &gitlab.Project{
							// sourcegraph/sourcegraph
							ProjectCommon: gitlab.ProjectCommon{ID: 16606088},
						}},
						Changeset: &batches.Changeset{ExternalID: "100000"},
					},
					err: "Changeset with external ID 100000 not found",
				},
				{
					name: "project-not-found",
					cs: &Changeset{
						Repo: &types.Repo{Metadata: &gitlab.Project{
							ProjectCommon: gitlab.ProjectCommon{ID: 999999999999},
						}},
						Changeset: &batches.Changeset{ExternalID: "100000"},
					},
					// Not a changeset not found error. This is important so we don't set
					// a changeset as deleted, when the token scope cannot view the project
					// the MR lives in.
					err: "retrieving merge request 100000: sending request to get a merge request: GitLab project not found",
				},
			}

			for _, tc := range testCases {
				tc := tc
				tc.name = "GitlabSource_LoadChangeset_" + tc.name

				t.Run(tc.name, func(t *testing.T) {
					cf, save := newClientFactory(t, tc.name)
					defer save(t)

					lg := log15.New()
					lg.SetHandler(log15.DiscardHandler())

					svc := &types.ExternalService{
						Kind: extsvc.KindGitLab,
						Config: marshalJSON(t, &schema.GitLabConnection{
							Url:   "https://gitlab.com",
							Token: os.Getenv("GITLAB_TOKEN"),
						}),
					}

					gitlabSource, err := NewGitLabSource(svc, cf)
					if err != nil {
						t.Fatal(err)
					}

					ctx := context.Background()
					if tc.err == "" {
						tc.err = "<nil>"
					}

					err = gitlabSource.LoadChangeset(ctx, tc.cs)
					if have, want := fmt.Sprint(err), tc.err; have != want {
						t.Errorf("error:\nhave: %q\nwant: %q", have, want)
					}

					if err != nil {
						return
					}

					meta := tc.cs.Changeset.Metadata.(*gitlab.MergeRequest)
					testutil.AssertGolden(t, "testdata/golden/"+tc.name, update(tc.name), meta)
				})
			}
		})

		// The guts of the note and pipeline scenarios are tested in separate
		// unit tests below for read{Notes,Pipelines}UntilSeen, but we'll do a
		// couple of quick tests here just to ensure that
		// decorateMergeRequestData does the right thing.
		t.Run("notes", func(t *testing.T) {
			// A new merge request with a new IID.
			mr := &gitlab.MergeRequest{IID: 43}
			notes := []*gitlab.Note{
				{ID: 1, System: true},
				{ID: 2, System: true},
				{ID: 3, System: false},
			}

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.ExternalID = "42"
			p.changeset.Changeset.Metadata = p.mr
			p.mockGetMergeRequest(42, mr, nil)
			p.mockGetMergeRequestNotes(43, notes, 20, nil)
			p.mockGetMergeRequestResourceStateEvents(43, nil, 20, nil)
			p.mockGetMergeRequestPipelines(43, nil, 20, nil)

			if err := p.source.LoadChangeset(p.ctx, p.changeset); err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if diff := cmp.Diff(mr.Notes, notes[0:2]); diff != "" {
				t.Errorf("unexpected notes: %s", diff)
			}

			// A subsequent load should result in the same notes. Since we
			// changed the IID in the merge request, we do need to change the
			// getMergeRequest mock.
			p.mockGetMergeRequest(43, mr, nil)
			if err := p.source.LoadChangeset(p.ctx, p.changeset); err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if diff := cmp.Diff(mr.Notes, notes[0:2]); diff != "" {
				t.Errorf("unexpected notes: %s", diff)
			}
		})

		t.Run("resource state events", func(t *testing.T) {
			// A new merge request with a new IID.
			mr := &gitlab.MergeRequest{IID: 43}
			events := []*gitlab.ResourceStateEvent{
				{
					ID:    1,
					State: gitlab.ResourceStateEventStateClosed,
				},
				{
					ID:    2,
					State: gitlab.ResourceStateEventStateMerged,
				},
				{
					ID:    3,
					State: gitlab.ResourceStateEventStateReopened,
				},
			}

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.ExternalID = "42"
			p.changeset.Changeset.Metadata = p.mr
			p.mockGetMergeRequest(42, mr, nil)
			p.mockGetMergeRequestNotes(43, nil, 20, nil)
			p.mockGetMergeRequestResourceStateEvents(43, events, 20, nil)
			p.mockGetMergeRequestPipelines(43, nil, 20, nil)

			if err := p.source.LoadChangeset(p.ctx, p.changeset); err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if diff := cmp.Diff(mr.ResourceStateEvents, events); diff != "" {
				t.Errorf("unexpected events: %s", diff)
			}

			// A subsequent load should result in the same events. Since we
			// changed the IID in the merge request, we do need to change the
			// getMergeRequest mock.
			p.mockGetMergeRequest(43, mr, nil)
			if err := p.source.LoadChangeset(p.ctx, p.changeset); err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if diff := cmp.Diff(mr.ResourceStateEvents, events); diff != "" {
				t.Errorf("unexpected events: %s", diff)
			}
		})

		t.Run("pipelines", func(t *testing.T) {
			// A new merge request with a new IID.
			mr := &gitlab.MergeRequest{IID: 43}
			pipelines := []*gitlab.Pipeline{
				{ID: 1},
				{ID: 2},
				{ID: 3},
			}

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.ExternalID = "42"
			p.changeset.Changeset.Metadata = p.mr
			p.mockGetMergeRequest(42, mr, nil)
			p.mockGetMergeRequestNotes(43, nil, 20, nil)
			p.mockGetMergeRequestResourceStateEvents(43, nil, 20, nil)
			p.mockGetMergeRequestPipelines(43, pipelines, 20, nil)

			if err := p.source.LoadChangeset(p.ctx, p.changeset); err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if diff := cmp.Diff(mr.Pipelines, pipelines); diff != "" {
				t.Errorf("unexpected pipelines: %s", diff)
			}

			// A subsequent load should result in the same pipelines. Since we
			// changed the IID in the merge request, we do need to change the
			// getMergeRequest mock.
			p.mockGetMergeRequest(43, mr, nil)
			if err := p.source.LoadChangeset(p.ctx, p.changeset); err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if diff := cmp.Diff(mr.Pipelines, pipelines); diff != "" {
				t.Errorf("unexpected pipelines: %s", diff)
			}
		})
	})

	t.Run("UpdateChangeset", func(t *testing.T) {
		t.Run("invalid metadata", func(t *testing.T) {
			p := newGitLabChangesetSourceTestProvider(t)

			err := p.source.UpdateChangeset(p.ctx, &Changeset{
				Changeset: &batches.Changeset{Metadata: struct{}{}},
			})
			if err == nil {
				t.Error("unexpected nil error")
			}
		})

		t.Run("error from UpdateMergeRequest", func(t *testing.T) {
			inner := errors.New("foo")
			mr := &gitlab.MergeRequest{}

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.Metadata = mr
			p.mockUpdateMergeRequest(mr, nil, "", inner)

			have := p.source.UpdateChangeset(p.ctx, p.changeset)
			if !errors.Is(have, inner) {
				t.Errorf("error does not include inner error: have %+v; want %+v", have, inner)
			}
			if p.changeset.Changeset.Metadata != mr {
				t.Errorf("metadata unexpectedly updated: from %+v; to %+v", mr, p.changeset.Changeset.Metadata)
			}
		})

		t.Run("success", func(t *testing.T) {
			in := &gitlab.MergeRequest{IID: 2}
			out := &gitlab.MergeRequest{}

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.Metadata = in
			p.mockUpdateMergeRequest(in, out, "", nil)
			p.mockGetMergeRequestNotes(in.IID, nil, 20, nil)
			p.mockGetMergeRequestResourceStateEvents(in.IID, nil, 20, nil)
			p.mockGetMergeRequestPipelines(in.IID, nil, 20, nil)

			if err := p.source.UpdateChangeset(p.ctx, p.changeset); err != nil {
				t.Errorf("unexpected non-nil error: %+v", err)
			}
			if p.changeset.Changeset.Metadata != out {
				t.Errorf("metadata not correctly updated: have %+v; want %+v", p.changeset.Changeset.Metadata, out)
			}
		})
	})
}

func TestReadNotesUntilSeen(t *testing.T) {
	commonNotes := []*gitlab.Note{
		{ID: 1, System: true},
		{ID: 2, System: true},
		{ID: 3, System: true},
		{ID: 4, System: true},
	}

	t.Run("reads all notes", func(t *testing.T) {
		notes, err := readSystemNotes(paginatedNoteIterator(commonNotes, 2))
		if err != nil {
			t.Errorf("unexpected non-nil error: %+v", err)
		}
		if diff := cmp.Diff(notes, commonNotes); diff != "" {
			t.Errorf("unexpected notes: %s", diff)
		}
	})

	t.Run("error from iterator", func(t *testing.T) {
		want := errors.New("foo")
		notes, err := readSystemNotes(func() ([]*gitlab.Note, error) {
			return nil, want
		})
		if notes != nil {
			t.Errorf("unexpected non-nil notes: %+v", notes)
		}
		if !errors.Is(err, want) {
			t.Errorf("expected error not found in chain: have %+v; want %+v", err, want)
		}
	})

	t.Run("no system notes", func(t *testing.T) {
		notes, err := readSystemNotes(paginatedNoteIterator([]*gitlab.Note{
			{ID: 1, System: false},
			{ID: 2, System: false},
			{ID: 3, System: false},
			{ID: 4, System: false},
		}, 2))
		if err != nil {
			t.Errorf("unexpected non-nil error: %+v", err)
		}
		if len(notes) > 0 {
			t.Errorf("unexpected notes: %+v", notes)
		}
	})

	t.Run("no pages", func(t *testing.T) {
		notes, err := readSystemNotes(paginatedNoteIterator([]*gitlab.Note{}, 2))
		if err != nil {
			t.Errorf("unexpected non-nil error: %+v", err)
		}
		if len(notes) > 0 {
			t.Errorf("unexpected notes: %+v", notes)
		}
	})
}

func TestReadPipelinesUntilSeen(t *testing.T) {
	commonPipelines := []*gitlab.Pipeline{
		{ID: 1},
		{ID: 2},
		{ID: 3},
		{ID: 4},
	}

	t.Run("reads all pipelines", func(t *testing.T) {
		notes, err := readPipelines(paginatedPipelineIterator(commonPipelines, 2))
		if err != nil {
			t.Errorf("unexpected non-nil error: %+v", err)
		}
		if diff := cmp.Diff(notes, commonPipelines); diff != "" {
			t.Errorf("unexpected notes: %s", diff)
		}
	})

	t.Run("error from iterator", func(t *testing.T) {
		want := errors.New("foo")
		pipelines, err := readPipelines(func() ([]*gitlab.Pipeline, error) {
			return nil, want
		})
		if pipelines != nil {
			t.Errorf("unexpected non-nil pipelines: %+v", pipelines)
		}
		if !errors.Is(err, want) {
			t.Errorf("expected error not found in chain: have %+v; want %+v", err, want)
		}
	})

	t.Run("no pages", func(t *testing.T) {
		pipelines, err := readPipelines(paginatedPipelineIterator([]*gitlab.Pipeline{}, 2))
		if err != nil {
			t.Errorf("unexpected non-nil error: %+v", err)
		}
		if len(pipelines) > 0 {
			t.Errorf("unexpected pipelines: %+v", pipelines)
		}
	})
}

func TestGitLabSource_WithAuthenticator(t *testing.T) {
	p := newGitLabChangesetSourceTestProvider(t)

	t.Run("supported", func(t *testing.T) {
		src, err := p.source.WithAuthenticator(&auth.OAuthBearerToken{})
		if err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}

		if gs, ok := src.(*GitLabSource); !ok {
			t.Error("cannot coerce Source into GitLabSource")
		} else if gs == nil {
			t.Error("unexpected nil Source")
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		for name, tc := range map[string]auth.Authenticator{
			"nil":         nil,
			"BasicAuth":   &auth.BasicAuth{},
			"OAuthClient": &auth.OAuthClient{},
		} {
			t.Run(name, func(t *testing.T) {
				src, err := p.source.WithAuthenticator(tc)
				if err == nil {
					t.Error("unexpected nil error")
				} else if _, ok := err.(UnsupportedAuthenticatorError); !ok {
					t.Errorf("unexpected error of type %T: %v", err, err)
				}
				if src != nil {
					t.Errorf("expected non-nil Source: %v", src)
				}
			})
		}
	})
}

// panicDoer provides a httpcli.Doer implementation that panics if any attempt
// is made to issue a HTTP request; thereby ensuring that our unit tests don't
// actually try to talk to GitLab.
type panicDoer struct{}

func (d *panicDoer) Do(r *http.Request) (*http.Response, error) {
	panic("this function should not be called; a mock must be missing")
}

type gitLabChangesetSourceTestProvider struct {
	changeset *Changeset
	ctx       context.Context
	mr        *gitlab.MergeRequest
	source    *GitLabSource
	t         *testing.T
}

// newGitLabChangesetSourceTestProvider provides a set of useful pre-canned
// objects, along with a handful of methods to mock underlying
// internal/extsvc/gitlab functions.
func newGitLabChangesetSourceTestProvider(t *testing.T) *gitLabChangesetSourceTestProvider {
	prov := gitlab.NewClientProvider(&url.URL{}, &panicDoer{})
	p := &gitLabChangesetSourceTestProvider{
		changeset: &Changeset{
			Changeset: &batches.Changeset{},
			Repo:      &types.Repo{Metadata: &gitlab.Project{}},
			HeadRef:   "refs/heads/head",
			BaseRef:   "refs/heads/base",
			Title:     "title",
			Body:      "description",
		},
		ctx: context.Background(),
		mr: &gitlab.MergeRequest{
			ID:           1,
			IID:          2,
			ProjectID:    3,
			Title:        "title",
			Description:  "description",
			SourceBranch: "head",
			TargetBranch: "base",
		},
		source: &GitLabSource{
			client:   prov.GetClient(),
			provider: prov,
		},
		t: t,
	}

	// Rather than requiring the caller to defer a call to unmock, we can do it
	// here and be sure we'll have it done when the test is complete.
	t.Cleanup(func() { p.unmock() })

	return p
}

func (p *gitLabChangesetSourceTestProvider) testCommonParams(ctx context.Context, client *gitlab.Client, project *gitlab.Project) {
	if client != p.source.client {
		p.t.Errorf("unexpected GitLabSource client: have %+v; want %+v", client, p.source.client)
	}
	if ctx != p.ctx {
		p.t.Errorf("unexpected context: have %+v; want %+v", ctx, p.ctx)
	}
	if project != p.changeset.Repo.Metadata.(*gitlab.Project) {
		p.t.Errorf("unexpected Project: have %+v; want %+v", project, p.changeset.Repo.Metadata)
	}
}

// mockCreateMergeRequest mocks a gitlab.CreateMergeRequest call. Note that only
// the SourceBranch and TargetBranch fields of the expected options are checked.
func (p *gitLabChangesetSourceTestProvider) mockCreateMergeRequest(expected gitlab.CreateMergeRequestOpts, mr *gitlab.MergeRequest, err error) {
	gitlab.MockCreateMergeRequest = func(client *gitlab.Client, ctx context.Context, project *gitlab.Project, opts gitlab.CreateMergeRequestOpts) (*gitlab.MergeRequest, error) {
		p.testCommonParams(ctx, client, project)

		if want := expected.SourceBranch; opts.SourceBranch != want {
			p.t.Errorf("unexpected SourceBranch: have %s; want %s", opts.SourceBranch, want)
		}
		if want := expected.TargetBranch; opts.TargetBranch != want {
			p.t.Errorf("unexpected TargetBranch: have %s; want %s", opts.TargetBranch, want)
		}

		return mr, err
	}
}

func (p *gitLabChangesetSourceTestProvider) mockGetMergeRequest(expected gitlab.ID, mr *gitlab.MergeRequest, err error) {
	gitlab.MockGetMergeRequest = func(client *gitlab.Client, ctx context.Context, project *gitlab.Project, iid gitlab.ID) (*gitlab.MergeRequest, error) {
		p.testCommonParams(ctx, client, project)
		if expected != iid {
			p.t.Errorf("invalid IID: have %d; want %d", iid, expected)
		}
		return mr, err
	}
}

func (p *gitLabChangesetSourceTestProvider) mockGetMergeRequestNotes(expectedIID gitlab.ID, notes []*gitlab.Note, pageSize int, err error) {
	gitlab.MockGetMergeRequestNotes = func(client *gitlab.Client, ctx context.Context, project *gitlab.Project, iid gitlab.ID) func() ([]*gitlab.Note, error) {
		p.testCommonParams(ctx, client, project)
		if expectedIID != iid {
			p.t.Errorf("unexpected IID: have %d; want %d", iid, expectedIID)
		}

		if err != nil {
			return func() ([]*gitlab.Note, error) { return nil, err }
		}
		return paginatedNoteIterator(notes, pageSize)
	}
}

func (p *gitLabChangesetSourceTestProvider) mockGetMergeRequestResourceStateEvents(expectedIID gitlab.ID, events []*gitlab.ResourceStateEvent, pageSize int, err error) {
	gitlab.MockGetMergeRequestResourceStateEvents = func(client *gitlab.Client, ctx context.Context, project *gitlab.Project, iid gitlab.ID) func() ([]*gitlab.ResourceStateEvent, error) {
		p.testCommonParams(ctx, client, project)
		if expectedIID != iid {
			p.t.Errorf("unexpected IID: have %d; want %d", iid, expectedIID)
		}

		if err != nil {
			return func() ([]*gitlab.ResourceStateEvent, error) { return nil, err }
		}
		return paginatedResourceStateEventIterator(events, pageSize)
	}
}

func (p *gitLabChangesetSourceTestProvider) mockGetMergeRequestPipelines(expectedIID gitlab.ID, pipelines []*gitlab.Pipeline, pageSize int, err error) {
	gitlab.MockGetMergeRequestPipelines = func(client *gitlab.Client, ctx context.Context, project *gitlab.Project, iid gitlab.ID) func() ([]*gitlab.Pipeline, error) {
		p.testCommonParams(ctx, client, project)
		if expectedIID != iid {
			p.t.Errorf("unexpected IID: have %d; want %d", iid, expectedIID)
		}

		if err != nil {
			return func() ([]*gitlab.Pipeline, error) { return nil, err }
		}
		return paginatedPipelineIterator(pipelines, pageSize)
	}
}

func (p *gitLabChangesetSourceTestProvider) mockGetOpenMergeRequestByRefs(mr *gitlab.MergeRequest, err error) {
	gitlab.MockGetOpenMergeRequestByRefs = func(client *gitlab.Client, ctx context.Context, project *gitlab.Project, source, target string) (*gitlab.MergeRequest, error) {
		p.testCommonParams(ctx, client, project)
		return mr, err
	}
}

func (p *gitLabChangesetSourceTestProvider) mockUpdateMergeRequest(expectedMR, updated *gitlab.MergeRequest, expectedStateEvent gitlab.UpdateMergeRequestStateEvent, err error) {
	gitlab.MockUpdateMergeRequest = func(client *gitlab.Client, ctx context.Context, project *gitlab.Project, mrIn *gitlab.MergeRequest, opts gitlab.UpdateMergeRequestOpts) (*gitlab.MergeRequest, error) {
		p.testCommonParams(ctx, client, project)
		if expectedMR != mrIn {
			p.t.Errorf("unexpected MergeRequest: have %+v; want %+v", mrIn, expectedMR)
		}
		if len(expectedStateEvent) != 0 && opts.StateEvent != expectedStateEvent {
			p.t.Errorf("unexpected StateEvent: have %+v; want %+v", opts.StateEvent, expectedStateEvent)
		}

		return updated, err
	}
}

func (p *gitLabChangesetSourceTestProvider) unmock() {
	gitlab.MockCreateMergeRequest = nil
	gitlab.MockGetMergeRequest = nil
	gitlab.MockGetMergeRequestNotes = nil
	gitlab.MockGetMergeRequestResourceStateEvents = nil
	gitlab.MockGetMergeRequestPipelines = nil
	gitlab.MockGetOpenMergeRequestByRefs = nil
	gitlab.MockUpdateMergeRequest = nil
}

// paginatedNoteIterator essentially fakes the pagination behaviour implemented
// by gitlab.GetMergeRequestNotes with a canned notes list.
func paginatedNoteIterator(notes []*gitlab.Note, pageSize int) func() ([]*gitlab.Note, error) {
	page := 0

	return func() ([]*gitlab.Note, error) {
		low := pageSize * page
		high := pageSize * (page + 1)
		page++

		if low >= len(notes) {
			return []*gitlab.Note{}, nil
		}
		if high > len(notes) {
			return notes[low:], nil
		}
		return notes[low:high], nil
	}
}

// paginatedResourceStateEventIterator essentially fakes the pagination behaviour implemented
// by gitlab.GetMergeRequestResourceStateEvents with a canned resource state events list.
func paginatedResourceStateEventIterator(events []*gitlab.ResourceStateEvent, pageSize int) func() ([]*gitlab.ResourceStateEvent, error) {
	page := 0

	return func() ([]*gitlab.ResourceStateEvent, error) {
		low := pageSize * page
		high := pageSize * (page + 1)
		page++

		if low >= len(events) {
			return []*gitlab.ResourceStateEvent{}, nil
		}
		if high > len(events) {
			return events[low:], nil
		}
		return events[low:high], nil
	}
}

// paginatedPipelineIterator essentially fakes the pagination behaviour
// implemented by gitlab.GetMergeRequestPipelines with a canned pipelines list.
func paginatedPipelineIterator(pipelines []*gitlab.Pipeline, pageSize int) func() ([]*gitlab.Pipeline, error) {
	page := 0

	return func() ([]*gitlab.Pipeline, error) {
		low := pageSize * page
		high := pageSize * (page + 1)
		page++

		if low >= len(pipelines) {
			return []*gitlab.Pipeline{}, nil
		}
		if high > len(pipelines) {
			return pipelines[low:], nil
		}
		return pipelines[low:high], nil
	}
}
