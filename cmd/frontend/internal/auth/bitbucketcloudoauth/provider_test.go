package bitbucketcloudoauth

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/schema"
)

func TestRequestedScopes(t *testing.T) {
	tests := []struct {
		schema    *schema.BitbucketCloudAuthProvider
		expScopes []string
	}{
		{
			schema: &schema.BitbucketCloudAuthProvider{
				ApiScope: "account,email,repository",
			},
			expScopes: []string{"account", "email", "repository"},
		},
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			scopes := requestedScopes(test.schema.ApiScope)
			sort.Strings(scopes)
			if diff := cmp.Diff(test.expScopes, scopes); diff != "" {
				t.Fatalf("scopes: %s", diff)
			}
		})
	}
}
