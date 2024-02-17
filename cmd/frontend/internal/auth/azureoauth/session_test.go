package azureoauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/internal/extsvc/azuredevops"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
)

func Test_verifyAllowOrgs(t *testing.T) {
	rcache.SetupForTest(t)
	ratelimit.SetupForTest(t)

	profile := azuredevops.Profile{
		ID:          "1",
		DisplayName: "test-user",
		PublicAlias: "public-alias-123",
	}

	mockServerInvokedCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mockServerInvokedCount += 1
		if strings.HasPrefix(r.URL.Path, "/_apis/accounts") {
			memberID := r.URL.Query().Get("memberId")
			if memberID != profile.PublicAlias {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(fmt.Sprintf("incorrect public alias used in API call: %q", memberID)))
				return
			}

			response := azuredevops.ListAuthorizedUserOrgsResponse{
				Count: 2,
				Value: []azuredevops.Org{
					{
						ID:   "1",
						Name: "foo",
					},
					{
						ID:   "2",
						Name: "bar",
					},
				},
			}

			if err := json.NewEncoder(w).Encode(response); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
			}

			return
		}

		w.WriteHeader(http.StatusBadRequest)
	}))
	azuredevops.MockVisualStudioAppURL = mockServer.URL

	testCases := []struct {
		name                           string
		allowOrgs                      map[string]struct{}
		expectedAllow                  bool
		expectedMockServerInvokedCount int
	}{
		{
			name:                           "empty allowOrgs",
			allowOrgs:                      map[string]struct{}{},
			expectedAllow:                  true,
			expectedMockServerInvokedCount: 0,
		},
		{
			name: "user is not part of org",
			allowOrgs: map[string]struct{}{
				"this-org-does-not-exist": {},
			},
			expectedAllow:                  false,
			expectedMockServerInvokedCount: 1,
		},
		{
			name: "user is part of org",
			allowOrgs: map[string]struct{}{
				"foo": {},
			},
			expectedAllow:                  true,
			expectedMockServerInvokedCount: 1,
		},
	}

	for _, tc := range testCases {

		t.Run(tc.name, func(t *testing.T) {
			mockServerInvokedCount = 0
			s := &sessionIssuerHelper{allowOrgs: tc.allowOrgs}

			ctx := context.Background()
			allow, err := s.verifyAllowOrgs(ctx, &profile, &oauth2.Token{AccessToken: "foo"}, httpcli.NewFactory(nil))
			require.NoError(t, err, "unexpected error")
			if allow != tc.expectedAllow {
				t.Fatalf("expected allow to be %v, but got %v", tc.expectedAllow, allow)
			}

			if mockServerInvokedCount != tc.expectedMockServerInvokedCount {
				t.Fatalf("expected mockServer to receive %d requests, but it received %d", tc.expectedMockServerInvokedCount, mockServerInvokedCount)
			}
		})
	}
}
