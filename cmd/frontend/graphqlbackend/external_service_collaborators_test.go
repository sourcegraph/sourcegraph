package graphqlbackend

import (
	"github.com/hexops/autogold"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"

	"context"
	"sort"
	"testing"
)

func TestExternalServiceCollaborators_parallelRecentCommitters(t *testing.T) {
	ctx := context.Background()

	calls := []*github.RecentCommittersParams{}
	recentCommittersFunc := func(ctx context.Context, params *github.RecentCommittersParams) (*github.RecentCommittersResults, error) {
		calls = append(calls, params)

		var results github.RecentCommittersResults
		results.Nodes = append(results.Nodes, struct {
			Authors struct {
				Nodes []struct {
					Date      string
					Email     string
					Name      string
					AvatarURL string
				}
			}
		}{
			Authors: struct {
				Nodes []struct {
					Date      string
					Email     string
					Name      string
					AvatarURL string
				}
			}{
				Nodes: []struct {
					Date      string
					Email     string
					Name      string
					AvatarURL string
				}{
					{Name: params.Name + "-joe"},
					{Name: params.Name + "-jane"},
					{Name: params.Name + "-janet"},
				},
			},
		})

		return &results, nil
	}

	repos := []string{"gorilla/mux", "golang/go", "sourcegraph/sourcegraph"}
	recentCommitters, err := parallelRecentCommitters(ctx, repos, recentCommittersFunc)
	if err != nil {
		t.Fatal(err)
	}

	sort.Slice(calls, func(i, j int) bool {
		return calls[i].Name < calls[j].Name
	})
	sort.Slice(recentCommitters, func(i, j int) bool {
		return recentCommitters[i].name < recentCommitters[j].name
	})

	autogold.Want("calls", []*github.RecentCommittersParams{
		{
			Name:  "go",
			Owner: "golang",
			First: 100,
		},
		{
			Name:  "mux",
			Owner: "gorilla",
			First: 100,
		},
		{
			Name:  "sourcegraph",
			Owner: "sourcegraph",
			First: 100,
		},
	}).Equal(t, calls)

	autogold.Want("recentCommitters", []*invitableCollaboratorResolver{
		{
			name: "go-jane",
		},
		{name: "go-janet"},
		{name: "go-joe"},
		{name: "mux-jane"},
		{name: "mux-janet"},
		{name: "mux-joe"},
		{name: "sourcegraph-jane"},
		{name: "sourcegraph-janet"},
		{name: "sourcegraph-joe"},
	}).Equal(t, recentCommitters)
}

func TestExternalServiceCollaborators_filterInvitableCollaborators(t *testing.T) {
	tests := []struct {
		want             autogold.Value
		recentCommitters []*invitableCollaboratorResolver
		authUserEmails   []*database.UserEmail
	}{
		{
			recentCommitters: []*invitableCollaboratorResolver{},
			authUserEmails:   []*database.UserEmail{},
			want:             autogold.Want("empty", []*invitableCollaboratorResolver{}),
		},
	}
	for _, tst := range tests {
		t.Run(tst.want.Name(), func(t *testing.T) {
			got := filterInvitableCollaborators(tst.recentCommitters, tst.authUserEmails)
			tst.want.Equal(t, got)
		})
	}
}
