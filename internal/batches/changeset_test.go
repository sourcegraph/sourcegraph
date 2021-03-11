package batches

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/go-diff/diff"

	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

func TestChangeset_DiffStat(t *testing.T) {
	var (
		added   int32 = 77
		changed int32 = 88
		deleted int32 = 99
	)

	for name, tc := range map[string]struct {
		c    Changeset
		want *diff.Stat
	}{
		"added missing": {
			c: Changeset{
				DiffStatAdded:   nil,
				DiffStatChanged: &changed,
				DiffStatDeleted: &deleted,
			},
			want: nil,
		},
		"changed missing": {
			c: Changeset{
				DiffStatAdded:   &added,
				DiffStatChanged: nil,
				DiffStatDeleted: &deleted,
			},
			want: nil,
		},
		"deleted missing": {
			c: Changeset{
				DiffStatAdded:   &added,
				DiffStatChanged: &changed,
				DiffStatDeleted: nil,
			},
			want: nil,
		},
		"all present": {
			c: Changeset{
				DiffStatAdded:   &added,
				DiffStatChanged: &changed,
				DiffStatDeleted: &deleted,
			},
			want: &diff.Stat{
				Added:   added,
				Changed: changed,
				Deleted: deleted,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			have := tc.c.DiffStat()
			if (tc.want == nil && have != nil) || (tc.want != nil && have == nil) {
				t.Errorf("mismatched nils in diff stats: have %+v; want %+v", have, tc.want)
			} else if tc.want != nil && have != nil {
				if d := cmp.Diff(*have, *tc.want); d != "" {
					t.Errorf("incorrect diff stat: %s", d)
				}
			}
		})
	}
}

func TestChangeset_SetMetadata(t *testing.T) {
	for name, tc := range map[string]struct {
		meta interface{}
		want *Changeset
	}{
		"bitbucketserver": {
			meta: &bitbucketserver.PullRequest{
				ID:          12345,
				FromRef:     bitbucketserver.Ref{ID: "refs/heads/branch"},
				UpdatedDate: 10 * 1000,
			},
			want: &Changeset{
				ExternalID:          "12345",
				ExternalServiceType: extsvc.TypeBitbucketServer,
				ExternalBranch:      "refs/heads/branch",
				ExternalUpdatedAt:   time.Unix(10, 0),
			},
		},
		"GitHub": {
			meta: &github.PullRequest{
				Number:      12345,
				HeadRefName: "branch",
				UpdatedAt:   time.Unix(10, 0),
			},
			want: &Changeset{
				ExternalID:          "12345",
				ExternalServiceType: extsvc.TypeGitHub,
				ExternalBranch:      "refs/heads/branch",
				ExternalUpdatedAt:   time.Unix(10, 0),
			},
		},
		"GitLab": {
			meta: &gitlab.MergeRequest{
				IID:          12345,
				SourceBranch: "branch",
				UpdatedAt:    gitlab.Time{Time: time.Unix(10, 0)},
			},
			want: &Changeset{
				ExternalID:          "12345",
				ExternalServiceType: extsvc.TypeGitLab,
				ExternalBranch:      "refs/heads/branch",
				ExternalUpdatedAt:   time.Unix(10, 0),
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			have := &Changeset{}
			want := tc.want
			want.Metadata = tc.meta

			if err := have.SetMetadata(tc.meta); err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if d := cmp.Diff(have, want); d != "" {
				t.Errorf("metadata not updated as expected: %s", d)
			}
		})
	}
}

func TestChangeset_Title(t *testing.T) {
	want := "foo"
	for name, meta := range map[string]interface{}{
		"bitbucketserver": &bitbucketserver.PullRequest{
			Title: want,
		},
		"GitHub": &github.PullRequest{
			Title: want,
		},
		"GitLab": &gitlab.MergeRequest{
			Title: want,
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := &Changeset{Metadata: meta}
			have, err := c.Title()
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if have != want {
				t.Errorf("unexpected title: have %s; want %s", have, want)
			}
		})
	}

	t.Run("unknown changeset type", func(t *testing.T) {
		c := &Changeset{}
		if _, err := c.Title(); err == nil {
			t.Error("unexpected nil error")
		}
	})
}

func TestChangeset_ExternalCreatedAt(t *testing.T) {
	want := time.Unix(10, 0)
	for name, meta := range map[string]interface{}{
		"bitbucketserver": &bitbucketserver.PullRequest{
			CreatedDate: 10 * 1000,
		},
		"GitHub": &github.PullRequest{
			CreatedAt: want,
		},
		"GitLab": &gitlab.MergeRequest{
			CreatedAt: gitlab.Time{Time: want},
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := &Changeset{Metadata: meta}
			if have := c.ExternalCreatedAt(); have != want {
				t.Errorf("unexpected external creation date: have %+v; want %+v", have, want)
			}
		})
	}

	t.Run("unknown changeset type", func(t *testing.T) {
		c := &Changeset{}
		want := time.Time{}
		if have := c.ExternalCreatedAt(); have != want {
			t.Errorf("unexpected external creation date: have %+v; want %+v", have, want)
		}
	})
}

func TestChangeset_Body(t *testing.T) {
	want := "foo"
	for name, meta := range map[string]interface{}{
		"bitbucketserver": &bitbucketserver.PullRequest{
			Description: want,
		},
		"GitHub": &github.PullRequest{
			Body: want,
		},
		"GitLab": &gitlab.MergeRequest{
			Description: want,
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := &Changeset{Metadata: meta}
			have, err := c.Body()
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if have != want {
				t.Errorf("unexpected body: have %s; want %s", have, want)
			}
		})
	}

	t.Run("unknown changeset type", func(t *testing.T) {
		c := &Changeset{}
		if _, err := c.Body(); err == nil {
			t.Error("unexpected nil error")
		}
	})
}

func TestChangeset_URL(t *testing.T) {
	want := "foo"
	for name, meta := range map[string]interface{}{
		"bitbucketserver": &bitbucketserver.PullRequest{
			Links: struct {
				Self []struct {
					Href string `json:"href"`
				} `json:"self"`
			}{
				Self: []struct {
					Href string `json:"href"`
				}{{Href: want}},
			},
		},
		"GitHub": &github.PullRequest{
			URL: want,
		},
		"GitLab": &gitlab.MergeRequest{
			WebURL: want,
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := &Changeset{Metadata: meta}
			have, err := c.URL()
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if have != want {
				t.Errorf("unexpected URL: have %s; want %s", have, want)
			}
		})
	}

	t.Run("unknown changeset type", func(t *testing.T) {
		c := &Changeset{}
		if _, err := c.URL(); err == nil {
			t.Error("unexpected nil error")
		}
	})
}

func TestChangeset_HeadRefOid(t *testing.T) {
	for name, tc := range map[string]struct {
		meta interface{}
		want string
	}{
		"bitbucketserver": {
			meta: &bitbucketserver.PullRequest{},
			want: "",
		},
		"GitHub": {
			meta: &github.PullRequest{HeadRefOid: "foo"},
			want: "foo",
		},
		"GitLab": {
			meta: &gitlab.MergeRequest{
				DiffRefs: gitlab.DiffRefs{HeadSHA: "foo"},
			},
			want: "foo",
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := &Changeset{Metadata: tc.meta}
			have, err := c.HeadRefOid()
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if have != tc.want {
				t.Errorf("unexpected head ref OID: have %s; want %s", have, tc.want)
			}
		})
	}

	t.Run("unknown changeset type", func(t *testing.T) {
		c := &Changeset{}
		if _, err := c.HeadRefOid(); err == nil {
			t.Error("unexpected nil error")
		}
	})
}

func TestChangeset_HeadRef(t *testing.T) {
	for name, tc := range map[string]struct {
		meta interface{}
		want string
	}{
		"bitbucketserver": {
			meta: &bitbucketserver.PullRequest{
				FromRef: bitbucketserver.Ref{ID: "foo"},
			},
			want: "foo",
		},
		"GitHub": {
			meta: &github.PullRequest{HeadRefName: "foo"},
			want: "refs/heads/foo",
		},
		"GitLab": {
			meta: &gitlab.MergeRequest{
				SourceBranch: "foo",
			},
			want: "refs/heads/foo",
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := &Changeset{Metadata: tc.meta}
			have, err := c.HeadRef()
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if have != tc.want {
				t.Errorf("unexpected head ref: have %s; want %s", have, tc.want)
			}
		})
	}

	t.Run("unknown changeset type", func(t *testing.T) {
		c := &Changeset{}
		if _, err := c.HeadRef(); err == nil {
			t.Error("unexpected nil error")
		}
	})
}

func TestChangeset_BaseRefOid(t *testing.T) {
	for name, tc := range map[string]struct {
		meta interface{}
		want string
	}{
		"bitbucketserver": {
			meta: &bitbucketserver.PullRequest{},
			want: "",
		},
		"GitHub": {
			meta: &github.PullRequest{BaseRefOid: "foo"},
			want: "foo",
		},
		"GitLab": {
			meta: &gitlab.MergeRequest{
				DiffRefs: gitlab.DiffRefs{BaseSHA: "foo"},
			},
			want: "foo",
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := &Changeset{Metadata: tc.meta}
			have, err := c.BaseRefOid()
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if have != tc.want {
				t.Errorf("unexpected base ref OID: have %s; want %s", have, tc.want)
			}
		})
	}

	t.Run("unknown changeset type", func(t *testing.T) {
		c := &Changeset{}
		if _, err := c.BaseRefOid(); err == nil {
			t.Error("unexpected nil error")
		}
	})
}

func TestChangeset_BaseRef(t *testing.T) {
	for name, tc := range map[string]struct {
		meta interface{}
		want string
	}{
		"bitbucketserver": {
			meta: &bitbucketserver.PullRequest{
				ToRef: bitbucketserver.Ref{ID: "foo"},
			},
			want: "foo",
		},
		"GitHub": {
			meta: &github.PullRequest{BaseRefName: "foo"},
			want: "refs/heads/foo",
		},
		"GitLab": {
			meta: &gitlab.MergeRequest{
				TargetBranch: "foo",
			},
			want: "refs/heads/foo",
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := &Changeset{Metadata: tc.meta}
			have, err := c.BaseRef()
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if have != tc.want {
				t.Errorf("unexpected base ref: have %s; want %s", have, tc.want)
			}
		})
	}

	t.Run("unknown changeset type", func(t *testing.T) {
		c := &Changeset{}
		if _, err := c.BaseRef(); err == nil {
			t.Error("unexpected nil error")
		}
	})
}

func TestChangeset_Labels(t *testing.T) {
	for name, tc := range map[string]struct {
		meta interface{}
		want []ChangesetLabel
	}{
		"bitbucketserver": {
			meta: &bitbucketserver.PullRequest{},
			want: []ChangesetLabel{},
		},
		"GitHub": {
			meta: &github.PullRequest{
				Labels: struct{ Nodes []github.Label }{
					Nodes: []github.Label{
						{
							Name:        "red door",
							Color:       "black",
							Description: "paint it black",
						},
						{
							Name:        "grün",
							Color:       "green",
							Description: "groan",
						},
					},
				},
			},
			want: []ChangesetLabel{
				{
					Name:        "red door",
					Color:       "black",
					Description: "paint it black",
				},
				{
					Name:        "grün",
					Color:       "green",
					Description: "groan",
				},
			},
		},
		"GitLab": {
			meta: &gitlab.MergeRequest{
				Labels: []string{"black", "green"},
			},
			want: []ChangesetLabel{
				{Name: "black", Color: "000000"},
				{Name: "green", Color: "000000"},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := &Changeset{Metadata: tc.meta}
			if d := cmp.Diff(c.Labels(), tc.want); d != "" {
				t.Errorf("unexpected labels: %s", d)
			}
		})
	}
}

func TestChangesetMetadata(t *testing.T) {
	now := timeutil.Now()

	githubActor := github.Actor{
		AvatarURL: "https://avatars2.githubusercontent.com/u/1185253",
		Login:     "mrnugget",
		URL:       "https://github.com/mrnugget",
	}

	githubPR := &github.PullRequest{
		ID:           "FOOBARID",
		Title:        "Fix a bunch of bugs",
		Body:         "This fixes a bunch of bugs",
		URL:          "https://github.com/sourcegraph/sourcegraph/pull/12345",
		Number:       12345,
		State:        "MERGED",
		Author:       githubActor,
		Participants: []github.Actor{githubActor},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	changeset := &Changeset{
		RepoID:              42,
		CreatedAt:           now,
		UpdatedAt:           now,
		Metadata:            githubPR,
		BatchChanges:        []BatchChangeAssoc{},
		ExternalID:          "12345",
		ExternalServiceType: extsvc.TypeGitHub,
	}

	title, err := changeset.Title()
	if err != nil {
		t.Fatal(err)
	}

	if want, have := githubPR.Title, title; want != have {
		t.Errorf("changeset title wrong. want=%q, have=%q", want, have)
	}

	body, err := changeset.Body()
	if err != nil {
		t.Fatal(err)
	}

	if want, have := githubPR.Body, body; want != have {
		t.Errorf("changeset body wrong. want=%q, have=%q", want, have)
	}

	url, err := changeset.URL()
	if err != nil {
		t.Fatal(err)
	}

	if want, have := githubPR.URL, url; want != have {
		t.Errorf("changeset url wrong. want=%q, have=%q", want, have)
	}
}
