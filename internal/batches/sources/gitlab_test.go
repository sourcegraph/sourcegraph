package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/Masterminds/semver"
	"github.com/google/go-cmp/cmp"
	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/versions"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/testutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

var mockVersion = semver.MustParse("12.0.0-pre")
var mockVersion2 = semver.MustParse("14.10.0-pre")

func TestGitLabSource(t *testing.T) {
	t.Run("determineVersion", func(t *testing.T) {
		t.Run("external service is cached in redis", func(t *testing.T) {
			p := newGitLabChangesetSourceTestProvider(t)

			p.mockGetVersions(mockVersion.String(), p.source.client.Urn())
			v, err := p.source.determineVersion(p.ctx)

			assert.NoError(t, err)
			assert.True(t, mockVersion.Equal(v), fmt.Sprintf("expected: %s, got: %s", v.String(), mockVersion.String()))
			assert.False(t, p.isGetVersionCalled, "Client.GetVersion should not be called")
		})

		t.Run("external service (matching the key) does not exist in redis", func(t *testing.T) {
			p := newGitLabChangesetSourceTestProvider(t)

			p.mockGetVersions("", "random-urn-key-that-doesnt exist")
			p.mockGetVersion(mockVersion.String())
			v, err := p.source.determineVersion(p.ctx)

			assert.NoError(t, err)
			assert.True(t, mockVersion.Equal(v), fmt.Sprintf("expected: %s, got: %s", v.String(), mockVersion.String()))
			assert.True(t, p.isGetVersionCalled, "Client.GetVersion should be called")
		})
	})

	t.Run("CreateDraftChangeset", func(t *testing.T) {
		t.Run("GitLab version is greater than 14.0.0", func(t *testing.T) {
			p := newGitLabChangesetSourceTestProvider(t)

			p.mockGetVersions(mockVersion2.String(), p.source.client.Urn())
			p.mockCreateMergeRequest(gitlab.CreateMergeRequestOpts{
				SourceBranch: p.mr.SourceBranch,
				TargetBranch: p.mr.TargetBranch,
			}, p.mr, nil)
			p.mockGetMergeRequestNotes(p.mr.IID, nil, 20, nil)
			p.mockGetMergeRequestResourceStateEvents(p.mr.IID, nil, 20, nil)
			p.mockGetMergeRequestPipelines(p.mr.IID, nil, 20, nil)

			exists, err := p.source.CreateDraftChangeset(p.ctx, p.changeset)
			assert.NoError(t, err)
			assert.True(t, strings.HasPrefix(p.changeset.Title, "Draft:"))
			assert.False(t, exists)
		})

		t.Run("GitLab Version is less than 14.0.0", func(t *testing.T) {
			p := newGitLabChangesetSourceTestProvider(t)

			p.mockGetVersions(mockVersion.String(), p.source.client.Urn())
			p.mockCreateMergeRequest(gitlab.CreateMergeRequestOpts{
				SourceBranch: p.mr.SourceBranch,
				TargetBranch: p.mr.TargetBranch,
			}, p.mr, nil)
			p.mockGetMergeRequestNotes(p.mr.IID, nil, 20, nil)
			p.mockGetMergeRequestResourceStateEvents(p.mr.IID, nil, 20, nil)
			p.mockGetMergeRequestPipelines(p.mr.IID, nil, 20, nil)

			exists, err := p.source.CreateDraftChangeset(p.ctx, p.changeset)
			assert.NoError(t, err)
			assert.True(t, strings.HasPrefix(p.changeset.Title, "WIP:"))
			assert.False(t, exists)
		})
	})
}

// TestGitLabSource_ChangesetSource tests the various Changeset functions that
// implement the ChangesetSource interface.
func TestGitLabSource_ChangesetSource(t *testing.T) {
	t.Run("CreateChangeset", func(t *testing.T) {
		t.Run("invalid metadata", func(t *testing.T) {
			defer func() { _ = recover() }()

			p := newGitLabChangesetSourceTestProvider(t)
			repo := &types.Repo{Metadata: struct{}{}}
			_, _ = p.source.CreateChangeset(p.ctx, &Changeset{
				RemoteRepo: repo,
				TargetRepo: repo,
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

		t.Run("integration", func(t *testing.T) {
			// Repository used: https://gitlab.com/batch-changes-testing/batch-changes-test-repo
			// This repository does not have any project setting to delete source branches
			// automatically on PR merge.
			//
			// The requests here cannot be easily rerun with `-update` since you can only open a
			// pull request once. To update, push a new branch with at least one commit to
			// batch-changes-test-repo, then update the branch names in the test cases below.
			//
			// You can update just this test with `-update GitLabSource_CreateChangeset`.
			repo := &types.Repo{
				Metadata: &gitlab.Project{
					// https://gitlab.com/batch-changes-testing/batch-changes-test-repo
					ProjectCommon: gitlab.ProjectCommon{ID: 40370047},
				},
			}

			testCases := []struct {
				name               string
				cs                 *Changeset
				removeSourceBranch bool
			}{
				{
					name: "no-remove-source-branch",
					cs: &Changeset{
						Title:      "This is a test PR",
						Body:       "This is the description of the test PR",
						HeadRef:    "refs/heads/test-pr-3",
						BaseRef:    "refs/heads/main",
						RemoteRepo: repo,
						TargetRepo: repo,
						Changeset:  &btypes.Changeset{},
					},
					removeSourceBranch: false,
				},
				{
					name: "yes-remove-source-branch",
					cs: &Changeset{
						Title:      "This is a test PR",
						Body:       "This is the description of the test PR",
						HeadRef:    "refs/heads/test-pr-4",
						BaseRef:    "refs/heads/main",
						RemoteRepo: repo,
						TargetRepo: repo,
						Changeset:  &btypes.Changeset{},
					},
					removeSourceBranch: true,
				},
			}

			for _, tc := range testCases {
				tc := tc
				tc.name = "GitLabSource_CreateChangeset_" + tc.name

				t.Run(tc.name, func(t *testing.T) {
					cf, save := newClientFactory(t, tc.name)
					defer save(t)

					if tc.removeSourceBranch {
						conf.Mock(&conf.Unified{
							SiteConfiguration: schema.SiteConfiguration{
								BatchChangesAutoDeleteBranch: true,
							},
						})
						defer conf.Mock(nil)
					}

					lg := log15.New()
					lg.SetHandler(log15.DiscardHandler())

					svc := &types.ExternalService{
						Kind: extsvc.KindGitLab,
						Config: extsvc.NewUnencryptedConfig(marshalJSON(t, &schema.GitLabConnection{
							Url:   "https://gitlab.com",
							Token: os.Getenv("GITLAB_TOKEN"),
						})),
					}

					ctx := context.Background()
					gitlabSource, err := NewGitLabSource(ctx, svc, cf)
					if err != nil {
						t.Fatal(err)
					}

					_, err = gitlabSource.CreateChangeset(ctx, tc.cs)
					if err != nil {
						t.Fatal(err)
					}

					meta := tc.cs.Changeset.Metadata.(*gitlab.MergeRequest)
					testutil.AssertGolden(t, "testdata/golden/"+tc.name, update(tc.name), meta)
					if meta.ForceRemoveSourceBranch != tc.removeSourceBranch {
						t.Fatalf("unexpected ForceRemoveSourceBranch value: have %v, want %v", meta.ForceRemoveSourceBranch, tc.removeSourceBranch)
					}
				})
			}
		})
	})

	t.Run("CloseChangeset", func(t *testing.T) {
		t.Run("invalid metadata", func(t *testing.T) {
			defer func() { _ = recover() }()

			p := newGitLabChangesetSourceTestProvider(t)
			repo := &types.Repo{Metadata: struct{}{}}
			_ = p.source.CloseChangeset(p.ctx, &Changeset{
				RemoteRepo: repo,
				TargetRepo: repo,
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
			repo := &types.Repo{Metadata: struct{}{}}
			_ = p.source.ReopenChangeset(p.ctx, &Changeset{
				RemoteRepo: repo,
				TargetRepo: repo,
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

			repo := &types.Repo{Metadata: struct{}{}}
			_ = p.source.LoadChangeset(p.ctx, &Changeset{
				RemoteRepo: repo,
				TargetRepo: repo,
			})
			t.Error("invalid metadata did not panic")
		})

		t.Run("error from ParseInt", func(t *testing.T) {
			p := newGitLabChangesetSourceTestProvider(t)
			repo := &types.Repo{Metadata: &gitlab.Project{}}
			if err := p.source.LoadChangeset(p.ctx, &Changeset{
				Changeset: &btypes.Changeset{
					ExternalID: "foo",
					Metadata:   &gitlab.MergeRequest{},
				},
				RemoteRepo: repo,
				TargetRepo: repo,
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

			expected := ChangesetNotFoundError{
				Changeset: &Changeset{
					Changeset: &btypes.Changeset{ExternalID: "43"},
				},
			}

			if err := p.source.LoadChangeset(p.ctx, p.changeset); err == nil {
				t.Fatal("unexpectedly no error for not found changeset")
			} else if !errors.Is(err, expected) {
				t.Fatalf("unexpected error: %+v", err)
			}
		})

		t.Run("integration", func(t *testing.T) {
			repo := &types.Repo{
				Metadata: &gitlab.Project{
					// sourcegraph/sourcegraph
					ProjectCommon: gitlab.ProjectCommon{ID: 16606088},
				},
			}
			testCases := []struct {
				name string
				cs   *Changeset
				err  string
			}{
				{
					name: "found",
					cs: &Changeset{
						RemoteRepo: repo,
						TargetRepo: repo,
						Changeset:  &btypes.Changeset{ExternalID: "2"},
					},
				},
				{
					name: "not-found",
					cs: &Changeset{
						RemoteRepo: repo,
						TargetRepo: repo,
						Changeset:  &btypes.Changeset{ExternalID: "100000"},
					},
					err: "Changeset with external ID 100000 not found",
				},
				{
					name: "project-not-found",
					cs: &Changeset{
						RemoteRepo: repo,
						TargetRepo: &types.Repo{Metadata: &gitlab.Project{
							ProjectCommon: gitlab.ProjectCommon{ID: 999999999999},
						}},
						Changeset: &btypes.Changeset{ExternalID: "100000"},
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
						Config: extsvc.NewUnencryptedConfig(marshalJSON(t, &schema.GitLabConnection{
							Url:   "https://gitlab.com",
							Token: os.Getenv("GITLAB_TOKEN"),
						})),
					}

					ctx := context.Background()
					gitlabSource, err := NewGitLabSource(ctx, svc, cf)
					if err != nil {
						t.Fatal(err)
					}

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
				Changeset: &btypes.Changeset{Metadata: struct{}{}},
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

	t.Run("UpdateChangeset draft", func(t *testing.T) {
		t.Run("GitLab version is greater than 14.0.0", func(t *testing.T) {
			// We won't test the full set of UpdateChangeset scenarios; instead
			// we'll just make sure the title is appropriately munged.
			in := &gitlab.MergeRequest{IID: 2, WorkInProgress: true}
			out := &gitlab.MergeRequest{}

			p := newGitLabChangesetSourceTestProvider(t)
			p.mockGetVersions(mockVersion2.String(), p.source.client.Urn())
			p.changeset.Changeset.Metadata = in

			oldMock := gitlab.MockUpdateMergeRequest
			t.Cleanup(func() { gitlab.MockUpdateMergeRequest = oldMock })
			gitlab.MockUpdateMergeRequest = func(c *gitlab.Client, ctx context.Context, project *gitlab.Project, mr *gitlab.MergeRequest, opts gitlab.UpdateMergeRequestOpts) (*gitlab.MergeRequest, error) {
				if have, want := opts.Title, "Draft: title"; have != want {
					t.Errorf("unexpected title: have=%q want=%q", have, want)
				}
				return out, nil
			}

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

		t.Run("GitLab version is less than 14.0.0", func(t *testing.T) {
			// We won't test the full set of UpdateChangeset scenarios; instead
			// we'll just make sure the title is appropriately munged.
			in := &gitlab.MergeRequest{IID: 2, WorkInProgress: true}
			out := &gitlab.MergeRequest{}

			p := newGitLabChangesetSourceTestProvider(t)
			p.mockGetVersions(mockVersion.String(), p.source.client.Urn())
			p.changeset.Changeset.Metadata = in

			oldMock := gitlab.MockUpdateMergeRequest
			t.Cleanup(func() { gitlab.MockUpdateMergeRequest = oldMock })
			gitlab.MockUpdateMergeRequest = func(c *gitlab.Client, ctx context.Context, project *gitlab.Project, mr *gitlab.MergeRequest, opts gitlab.UpdateMergeRequestOpts) (*gitlab.MergeRequest, error) {
				if have, want := opts.Title, "WIP: title"; have != want {
					t.Errorf("unexpected title: have=%q want=%q", have, want)
				}
				return out, nil
			}

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

	t.Run("UndraftChangeset", func(t *testing.T) {
		in := &gitlab.MergeRequest{IID: 2, WorkInProgress: true}
		out := &gitlab.MergeRequest{}

		p := newGitLabChangesetSourceTestProvider(t)
		p.changeset.Changeset.Metadata = in

		oldMock := gitlab.MockUpdateMergeRequest
		t.Cleanup(func() { gitlab.MockUpdateMergeRequest = oldMock })
		gitlab.MockUpdateMergeRequest = func(c *gitlab.Client, ctx context.Context, project *gitlab.Project, mr *gitlab.MergeRequest, opts gitlab.UpdateMergeRequestOpts) (*gitlab.MergeRequest, error) {
			if have, want := opts.Title, "title"; have != want {
				t.Errorf("unexpected title: have=%q want=%q", have, want)
			}
			return out, nil
		}

		p.mockGetVersions(mockVersion.String(), p.source.client.Urn())
		p.mockGetMergeRequestNotes(in.IID, nil, 20, nil)
		p.mockGetMergeRequestResourceStateEvents(in.IID, nil, 20, nil)
		p.mockGetMergeRequestPipelines(in.IID, nil, 20, nil)

		if err := p.source.UndraftChangeset(p.ctx, p.changeset); err != nil {
			t.Errorf("unexpected non-nil error: %+v", err)
		}
		if p.changeset.Changeset.Metadata != out {
			t.Errorf("metadata not correctly updated: have %+v; want %+v", p.changeset.Changeset.Metadata, out)
		}
	})

	t.Run("CreateComment", func(t *testing.T) {
		commentBody := "test-comment"
		t.Run("invalid metadata", func(t *testing.T) {
			defer func() { _ = recover() }()

			p := newGitLabChangesetSourceTestProvider(t)
			repo := &types.Repo{Metadata: struct{}{}}
			_ = p.source.CreateComment(p.ctx, &Changeset{
				RemoteRepo: repo,
				TargetRepo: repo,
			}, commentBody)
			t.Error("invalid metadata did not panic")
		})

		t.Run("error from CreateComment", func(t *testing.T) {
			inner := errors.New("foo")
			mr := &gitlab.MergeRequest{}

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.Metadata = mr
			p.mockCreateComment(commentBody, inner)

			have := p.source.CreateComment(p.ctx, p.changeset, commentBody)
			if !errors.Is(have, inner) {
				t.Errorf("error does not include inner error: have %+v; want %+v", have, inner)
			}
		})

		t.Run("success", func(t *testing.T) {
			mr := &gitlab.MergeRequest{IID: 2}

			p := newGitLabChangesetSourceTestProvider(t)
			p.changeset.Changeset.Metadata = mr
			p.mockCreateComment(commentBody, nil)

			if err := p.source.CreateComment(p.ctx, p.changeset, commentBody); err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
		})

		t.Run("integration", func(t *testing.T) {
			name := "GitlabSource_CreateComment_success"

			t.Run(name, func(t *testing.T) {
				cf, save := newClientFactory(t, name)
				defer save(t)

				lg := log15.New()
				lg.SetHandler(log15.DiscardHandler())

				svc := &types.ExternalService{
					Kind: extsvc.KindGitLab,
					Config: extsvc.NewUnencryptedConfig(marshalJSON(t, &schema.GitLabConnection{
						Url:   "https://gitlab.com",
						Token: os.Getenv("GITLAB_TOKEN"),
					})),
				}

				ctx := context.Background()
				gitlabSource, err := NewGitLabSource(ctx, svc, cf)
				if err != nil {
					t.Fatal(err)
				}

				repo := &types.Repo{Metadata: newGitLabProject(16606088)}
				cs := &Changeset{
					RemoteRepo: repo,
					TargetRepo: repo,
					Changeset: &btypes.Changeset{Metadata: &gitlab.MergeRequest{
						IID: gitlab.ID(2),
					}},
				}

				if err := gitlabSource.CreateComment(ctx, cs, "test-comment"); err != nil {
					t.Fatal(err)
				}
			})
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

type gitLabChangesetSourceTestProvider struct {
	changeset *Changeset
	ctx       context.Context
	mr        *gitlab.MergeRequest
	source    *GitLabSource
	t         *testing.T

	isGetVersionCalled bool
}

// newGitLabChangesetSourceTestProvider provides a set of useful pre-canned
// objects, along with a handful of methods to mock underlying
// internal/extsvc/gitlab functions.
func newGitLabChangesetSourceTestProvider(t *testing.T) *gitLabChangesetSourceTestProvider {
	prov, err := gitlab.NewClientProvider("Test", &url.URL{}, httpcli.NewFactory(nil, func(c *http.Client) error {
		c.Transport = &panicTransport{}
		return nil
	}))
	require.NoError(t, err)
	repo := &types.Repo{Metadata: &gitlab.Project{}}
	p := &gitLabChangesetSourceTestProvider{
		changeset: &Changeset{
			Changeset:  &btypes.Changeset{},
			RemoteRepo: repo,
			TargetRepo: repo,
			HeadRef:    "refs/heads/head",
			BaseRef:    "refs/heads/base",
			Title:      "title",
			Body:       "description",
		},
		ctx: context.Background(),
		mr: &gitlab.MergeRequest{
			ID:              1,
			IID:             2,
			ProjectID:       3,
			SourceProjectID: 3,
			Title:           "title",
			Description:     "description",
			SourceBranch:    "head",
			TargetBranch:    "base",
		},
		source: &GitLabSource{
			client: prov.GetClient(),
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
	if project != p.changeset.TargetRepo.Metadata.(*gitlab.Project) {
		p.t.Errorf("unexpected Project: have %+v; want %+v", project, p.changeset.TargetRepo.Metadata)
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

func (p *gitLabChangesetSourceTestProvider) mockCreateComment(expected string, err error) {
	gitlab.MockCreateMergeRequestNote = func(client *gitlab.Client, ctx context.Context, project *gitlab.Project, mr *gitlab.MergeRequest, body string) error {
		p.testCommonParams(ctx, client, project)
		if expected != body {
			p.t.Errorf("invalid body passed: have %q; want %q", body, expected)
		}
		return err
	}
}

func (p *gitLabChangesetSourceTestProvider) mockGetVersions(expected, key string) {
	versions.MockGetVersions = func() ([]*versions.Version, error) {
		return []*versions.Version{
			{
				ExternalServiceKind: extsvc.KindGitLab,
				Version:             expected,
				Key:                 key,
			},
			{
				ExternalServiceKind: extsvc.KindGitHub,
				Version:             "2.38.0",
				Key:                 "random-key-<1>",
			},
			{
				ExternalServiceKind: extsvc.KindGitLab,
				Version:             "1.3.5",
				Key:                 "random-key-<2>",
			},
			{
				ExternalServiceKind: extsvc.KindBitbucketCloud,
				Version:             "1.2.5",
				Key:                 "random-key-<3>",
			},
		}, nil
	}
}

func (p *gitLabChangesetSourceTestProvider) mockGetVersion(expected string) {
	gitlab.MockGetVersion = func(ctx context.Context) (string, error) {
		p.isGetVersionCalled = true
		return expected, nil
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
	gitlab.MockCreateMergeRequestNote = nil

	versions.MockGetVersions = nil
}

// panicTransport provides a httpcli.Doer implementation that panics if any attempt
// is made to issue a HTTP request; thereby ensuring that our unit tests don't
// actually try to talk to GitLab.
type panicTransport struct{}

func (d *panicTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	panic("this function should not be called; a mock must be missing")
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

func TestGitLabSource_WithAuthenticator(t *testing.T) {
	t.Run("supported", func(t *testing.T) {
		var src ChangesetSource
		src, err := newGitLabSource("Test", &schema.GitLabConnection{}, nil)
		if err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}
		src, err = src.WithAuthenticator(&auth.OAuthBearerToken{})
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
				var src ChangesetSource
				src, err := newGitLabSource("Test", &schema.GitLabConnection{}, nil)
				if err != nil {
					t.Errorf("unexpected non-nil error: %v", err)
				}
				src, err = src.WithAuthenticator(tc)
				if err == nil {
					t.Error("unexpected nil error")
				} else if !errors.HasType(err, UnsupportedAuthenticatorError{}) {
					t.Errorf("unexpected error of type %T: %v", err, err)
				}
				if src != nil {
					t.Errorf("expected non-nil Source: %v", src)
				}
			})
		}
	})
}

func TestGitlabSource_GetFork(t *testing.T) {
	ctx := context.Background()

	t.Run("failures", func(t *testing.T) {
		for name, tc := range map[string]struct {
			targetRepo *types.Repo
			client     gitlabClientFork
		}{
			"invalid PathWithNamespace": {
				targetRepo: &types.Repo{
					Metadata: &gitlab.Project{
						ProjectCommon: gitlab.ProjectCommon{
							PathWithNamespace: "foo",
						},
					},
				},
				client: nil,
			},
			"client error": {
				targetRepo: &types.Repo{
					Metadata: &gitlab.Project{
						ProjectCommon: gitlab.ProjectCommon{
							PathWithNamespace: "foo/bar",
						},
					},
				},
				client: &mockGitlabClientFork{err: errors.New("hello!")},
			},
		} {
			t.Run(name, func(t *testing.T) {
				fork, err := getGitLabForkInternal(ctx, tc.targetRepo, tc.client, nil, nil)
				assert.Nil(t, fork)
				assert.NotNil(t, err)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		org := "org"
		user := "user"
		urn := extsvc.URN(extsvc.KindGitLab, 1)

		for name, tc := range map[string]struct {
			targetRepo    *types.Repo
			forkRepo      *gitlab.Project
			namespace     *string
			wantNamespace string
			name          *string
			wantName      string
			client        gitlabClientFork
		}{
			"no namespace": {
				targetRepo: &types.Repo{
					Metadata: &gitlab.Project{
						ProjectCommon: gitlab.ProjectCommon{
							ID:                1,
							PathWithNamespace: "foo/bar",
						},
					},
					Sources: map[string]*types.SourceInfo{
						urn: {
							ID:       urn,
							CloneURL: "https://gitlab.com/foo/bar",
						},
					},
				},
				forkRepo: &gitlab.Project{
					ForkedFromProject: &gitlab.ProjectCommon{ID: 1},
					ProjectCommon:     gitlab.ProjectCommon{ID: 2, PathWithNamespace: user + "/user-bar"}},
				namespace:     nil,
				wantNamespace: user,
				wantName:      user + "-bar",
				client: &mockGitlabClientFork{fork: &gitlab.Project{ForkedFromProject: &gitlab.ProjectCommon{ID: 1},
					ProjectCommon: gitlab.ProjectCommon{ID: 2, PathWithNamespace: user + "/user-bar"}}},
			},
			"with namespace": {
				targetRepo: &types.Repo{
					Metadata: &gitlab.Project{
						ProjectCommon: gitlab.ProjectCommon{
							ID:                1,
							PathWithNamespace: "foo/bar",
						},
					},
					Sources: map[string]*types.SourceInfo{
						urn: {
							ID:       urn,
							CloneURL: "https://gitlab.com/foo/bar",
						},
					},
				},
				forkRepo: &gitlab.Project{
					ForkedFromProject: &gitlab.ProjectCommon{ID: 1},
					ProjectCommon:     gitlab.ProjectCommon{ID: 2, PathWithNamespace: org + "/" + org + "-bar"}},
				namespace:     &org,
				wantNamespace: org,
				wantName:      org + "-bar",
				client: &mockGitlabClientFork{
					fork: &gitlab.Project{
						ForkedFromProject: &gitlab.ProjectCommon{ID: 1},
						ProjectCommon:     gitlab.ProjectCommon{ID: 2, PathWithNamespace: org + "/" + org + "-bar"}},
					wantOrg: &org,
				},
			},
			"with namespace and name": {
				targetRepo: &types.Repo{
					Metadata: &gitlab.Project{
						ProjectCommon: gitlab.ProjectCommon{
							ID:                1,
							PathWithNamespace: "foo/bar",
						},
					},
					Sources: map[string]*types.SourceInfo{
						urn: {
							ID:       urn,
							CloneURL: "https://gitlab.com/foo/bar",
						},
					},
				},
				forkRepo: &gitlab.Project{
					ForkedFromProject: &gitlab.ProjectCommon{ID: 1},
					ProjectCommon:     gitlab.ProjectCommon{ID: 2, PathWithNamespace: org + "/custom-bar"}},
				namespace:     &org,
				wantNamespace: org,
				name:          pointers.Ptr("custom-bar"),
				wantName:      "custom-bar",
				client: &mockGitlabClientFork{
					fork: &gitlab.Project{
						ForkedFromProject: &gitlab.ProjectCommon{ID: 1},
						ProjectCommon:     gitlab.ProjectCommon{ID: 2, PathWithNamespace: org + "/custom-bar"}},
					wantOrg: &org,
				},
			},
		} {
			t.Run(name, func(t *testing.T) {
				fork, err := getGitLabForkInternal(ctx, tc.targetRepo, tc.client, tc.namespace, tc.name)
				assert.Nil(t, err)
				assert.NotNil(t, fork)
				assert.NotEqual(t, fork, tc.targetRepo)
				assert.Equal(t, tc.forkRepo, fork.Metadata)
				assert.Equal(t, fork.Sources[urn].CloneURL, "https://gitlab.com/"+tc.wantNamespace+"/"+tc.wantName)
			})
		}
	})
}

type mockGitlabClientFork struct {
	wantOrg *string
	fork    *gitlab.Project
	err     error
}

var _ gitlabClientFork = &mockGitlabClientFork{}

func (mock *mockGitlabClientFork) ForkProject(ctx context.Context, project *gitlab.Project, namespace *string, name string) (*gitlab.Project, error) {
	if (mock.wantOrg == nil && namespace != nil) || (mock.wantOrg != nil && namespace == nil) || (mock.wantOrg != nil && namespace != nil && *mock.wantOrg != *namespace) {
		return nil, errors.Newf("unexpected organisation: have=%v want=%v", namespace, mock.wantOrg)
	}

	return mock.fork, mock.err
}

func TestDecorateMergeRequestData(t *testing.T) {
	ctx := context.Background()

	// The test fixtures use publicly available merge requests, and should be
	// able to be updated at any time without any action required.
	createSource := func(t *testing.T) *GitLabSource {
		cf, save := newClientFactory(t, t.Name())
		t.Cleanup(func() { save(t) })

		src, err := newGitLabSource(
			"Test",
			&schema.GitLabConnection{
				Url:   "https://gitlab.com",
				Token: os.Getenv("GITLAB_TOKEN"),
			},
			cf,
		)

		assert.Nil(t, err)
		return src
	}

	src := createSource(t)

	// https://gitlab.com/sourcegraph/src-cli/-/merge_requests/6
	forked, err := src.client.GetMergeRequest(ctx, newGitLabProject(16606399), 6)
	assert.Nil(t, err)

	// https://gitlab.com/sourcegraph/sourcegraph/-/merge_requests/1
	unforked, err := src.client.GetMergeRequest(ctx, newGitLabProject(16606088), 1)
	assert.Nil(t, err)

	t.Run("fork", func(t *testing.T) {
		err := createSource(t).decorateMergeRequestData(ctx, newGitLabProject(int(forked.ProjectID)), forked)
		assert.Nil(t, err)
		assert.Equal(t, "courier-new", forked.SourceProjectNamespace)
		assert.Equal(t, "src-cli-forked", forked.SourceProjectName)
	})

	t.Run("not a fork", func(t *testing.T) {
		err := createSource(t).decorateMergeRequestData(ctx, newGitLabProject(int(unforked.ProjectID)), unforked)
		assert.Nil(t, err)
		assert.Equal(t, "", unforked.SourceProjectNamespace)
		assert.Equal(t, "", unforked.SourceProjectName)
	})
}

func newGitLabProject(id int) *gitlab.Project {
	return &gitlab.Project{
		ProjectCommon: gitlab.ProjectCommon{ID: id},
	}
}
