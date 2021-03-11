package githuboauth

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestRequestedScopes(t *testing.T) {
	defer envvar.MockSourcegraphDotComMode(false)

	tests := []struct {
		dotComMode bool
		schema     *schema.GitHubAuthProvider
		expScopes  []string
	}{
		{
			dotComMode: false,
			schema: &schema.GitHubAuthProvider{
				AllowOrgs: nil,
			},
			expScopes: []string{"repo", "user:email"},
		},
		{
			dotComMode: false,
			schema: &schema.GitHubAuthProvider{
				AllowOrgs: []string{"myorg"},
			},
			expScopes: []string{"read:org", "repo", "user:email"},
		},

		{
			dotComMode: true,
			schema: &schema.GitHubAuthProvider{
				AllowOrgs: nil,
			},
			expScopes: []string{"user:email"},
		},
		{
			dotComMode: true,
			schema: &schema.GitHubAuthProvider{
				AllowOrgs: []string{"myorg"},
			},
			expScopes: []string{"read:org", "user:email"},
		},
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			envvar.MockSourcegraphDotComMode(test.dotComMode)
			scopes := requestedScopes(test.schema)
			sort.Strings(scopes)
			if diff := cmp.Diff(test.expScopes, scopes); diff != "" {
				t.Fatalf("scopes: %s", diff)
			}
		})
	}
}
