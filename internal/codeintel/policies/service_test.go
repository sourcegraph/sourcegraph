package policies

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/derision-test/glock"
	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/api"
	policiesshared "github.com/sourcegraph/sourcegraph/internal/codeintel/policies/shared"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	internaltypes "github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestGetRetentionPolicyOverview(t *testing.T) {
	mockStore := NewMockStore()
	mockRepoStore := defaultMockRepoStore()
	mockUploadSvc := NewMockUploadService()
	mockGitserverClient := gitserver.NewMockClient()

	svc := newService(&observation.TestContext, mockStore, mockRepoStore, mockUploadSvc, mockGitserverClient)

	mockClock := glock.NewMockClock()

	cases := []struct {
		name            string
		expectedMatches int
		upload          shared.Upload
		mockPolicies    []policiesshared.RetentionPolicyMatchCandidate
		refs            []gitdomain.Ref
	}{
		{
			name:            "basic single upload match",
			expectedMatches: 1,
			upload: shared.Upload{
				Commit:     "deadbeef0",
				UploadedAt: mockClock.Now().Add(-time.Hour * 23),
			},
			mockPolicies: []policiesshared.RetentionPolicyMatchCandidate{
				{
					ConfigurationPolicy: &policiesshared.ConfigurationPolicy{
						RetentionDuration:         pointers.Ptr(time.Hour * 24),
						RetainIntermediateCommits: false,
						Type:                      policiesshared.GitObjectTypeTag,
						Pattern:                   "*",
					},
					Matched: true,
				},
			},
			refs: []gitdomain.Ref{
				{
					Name:     "v4.2.0",
					Type:     gitdomain.RefTypeTag,
					IsHead:   false,
					CommitID: "deadbeef0",
				},
			},
		},
		{
			name:            "matching but expired",
			expectedMatches: 0,
			upload: shared.Upload{
				Commit:     "deadbeef0",
				UploadedAt: mockClock.Now().Add(-time.Hour * 25),
			},
			mockPolicies: []policiesshared.RetentionPolicyMatchCandidate{
				{
					ConfigurationPolicy: &policiesshared.ConfigurationPolicy{
						RetentionDuration:         pointers.Ptr(time.Hour * 24),
						RetainIntermediateCommits: false,
						Type:                      policiesshared.GitObjectTypeTag,
						Pattern:                   "*",
					},
					Matched: false,
				},
			},
			refs: []gitdomain.Ref{
				{
					Name:     "v4.2.0",
					Type:     gitdomain.RefTypeTag,
					IsHead:   false,
					CommitID: "deadbeef0",
				},
			},
		},
		{
			name:            "tip of default branch match",
			expectedMatches: 1,
			upload: shared.Upload{
				Commit:     "deadbeef0",
				UploadedAt: mockClock.Now().Add(-time.Hour * 25),
			},
			mockPolicies: []policiesshared.RetentionPolicyMatchCandidate{
				{
					ConfigurationPolicy: nil,
					Matched:             true,
				},
			},
			refs: []gitdomain.Ref{
				{
					Name:     "main",
					Type:     gitdomain.RefTypeBranch,
					IsHead:   true,
					CommitID: "deadbeef0",
				},
			},
		},
		{
			name:            "direct match (1 of 2 policies)",
			expectedMatches: 1,
			upload: shared.Upload{
				Commit:     "deadbeef0",
				UploadedAt: mockClock.Now().Add(-time.Minute),
			},
			mockPolicies: []policiesshared.RetentionPolicyMatchCandidate{
				{
					ConfigurationPolicy: &policiesshared.ConfigurationPolicy{
						RetentionDuration:         pointers.Ptr(time.Hour * 24),
						RetainIntermediateCommits: false,
						Type:                      policiesshared.GitObjectTypeTag,
						Pattern:                   "*",
					},
					Matched: true,
				},
				{
					ConfigurationPolicy: &policiesshared.ConfigurationPolicy{
						RetentionDuration:         pointers.Ptr(time.Hour * 24),
						RetainIntermediateCommits: false,
						Type:                      policiesshared.GitObjectTypeTree,
						Pattern:                   "*",
					},
					Matched: false,
				},
			},
			refs: []gitdomain.Ref{
				{
					Name:     "v4.2.0",
					Type:     gitdomain.RefTypeTag,
					IsHead:   false,
					CommitID: "deadbeef0",
				},
			},
		},
		{
			name:            "direct match (ignore visible)",
			expectedMatches: 1,
			upload: shared.Upload{
				Commit:     "deadbeef1",
				UploadedAt: mockClock.Now().Add(-time.Minute),
			},
			mockPolicies: []policiesshared.RetentionPolicyMatchCandidate{
				{
					ConfigurationPolicy: &policiesshared.ConfigurationPolicy{
						RetentionDuration:         pointers.Ptr(time.Hour * 24),
						RetainIntermediateCommits: false,
						Type:                      policiesshared.GitObjectTypeTag,
						Pattern:                   "*",
					},
					Matched: true,
				},
			},
			refs: []gitdomain.Ref{
				{
					Name:     "v4.2.0",
					Type:     gitdomain.RefTypeTag,
					IsHead:   false,
					CommitID: "deadbeef1",
				},
				{
					Name:     "v4.1.9",
					Type:     gitdomain.RefTypeTag,
					IsHead:   false,
					CommitID: "deadbeef0",
				},
			},
		},
	}

	for _, c := range cases {
		t.Run("PolicyOverview "+c.name, func(t *testing.T) {
			expectedPolicyCandidates, mockedStorePolicies := mockConfigurationPolicies(c.mockPolicies)
			mockStore.GetConfigurationPoliciesFunc.PushReturn(mockedStorePolicies, len(mockedStorePolicies), nil)

			mockGitserverClient.ListRefsFunc.PushReturn(c.refs, nil)

			matches, _, err := svc.GetRetentionPolicyOverview(context.Background(), c.upload, false, 10, 0, "", mockClock.Now())
			if err != nil {
				t.Fatalf("unexpected error resolving retention policy overview: %v", err)
			}

			var matchCount int
			for _, match := range matches {
				if match.Matched {
					matchCount++
				}
			}

			if matchCount != c.expectedMatches {
				t.Errorf("unexpected number of matched policies: want=%d have=%d", c.expectedMatches, matchCount)
			}

			if diff := cmp.Diff(expectedPolicyCandidates, matches); diff != "" {
				t.Errorf("unexpected retention policy matches (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRetentionPolicyOverview_ByVisibility(t *testing.T) {
	mockStore := NewMockStore()
	mockRepoStore := defaultMockRepoStore()
	mockUploadSvc := NewMockUploadService()
	mockGitserverClient := gitserver.NewMockClient()

	svc := newService(&observation.TestContext, mockStore, mockRepoStore, mockUploadSvc, mockGitserverClient)

	mockClock := glock.NewMockClock()

	// deadbeef2 ----\
	// deadbeef0 ---- deadbeef1
	// T0------------------->T1

	cases := []struct {
		name            string
		upload          shared.Upload
		mockPolicies    []policiesshared.RetentionPolicyMatchCandidate
		visibleCommits  []string
		refs            []gitdomain.Ref
		expectedMatches int
	}{
		{
			name:            "basic single visibility",
			expectedMatches: 1,
			upload: shared.Upload{
				Commit:     "deadbeef0",
				UploadedAt: mockClock.Now().Add(-time.Minute * 24),
			},
			visibleCommits: []string{"deadbeef1"},
			mockPolicies: []policiesshared.RetentionPolicyMatchCandidate{
				{
					ConfigurationPolicy: &policiesshared.ConfigurationPolicy{
						RetentionDuration:         pointers.Ptr(time.Hour * 24),
						RetainIntermediateCommits: false,
						Type:                      policiesshared.GitObjectTypeTag,
						Pattern:                   "*",
					},
					ProtectingCommits: []string{"deadbeef1"},
					Matched:           true,
				},
			},
			refs: []gitdomain.Ref{
				{
					Name:     "v4.2.0",
					Type:     gitdomain.RefTypeTag,
					IsHead:   false,
					CommitID: "deadbeef1",
				},
			},
		},
		{
			name:            "visibile to tip of default branch",
			expectedMatches: 1,
			visibleCommits:  []string{"deadbeef0", "deadbeef1"},
			upload: shared.Upload{
				Commit:     "deadbeef0",
				UploadedAt: mockClock.Now().Add(-time.Hour * 24),
			},
			mockPolicies: []policiesshared.RetentionPolicyMatchCandidate{
				{
					ConfigurationPolicy: nil,
					ProtectingCommits:   []string{"deadbeef1"},
					Matched:             true,
				},
			},
			refs: []gitdomain.Ref{
				{
					Name:     "main",
					Type:     gitdomain.RefTypeBranch,
					IsHead:   true,
					CommitID: "deadbeef1",
				},
			},
		},
	}

	for _, c := range cases {
		t.Run("ByVisibility "+c.name, func(t *testing.T) {
			expectedPolicyCandidates, mockedStorePolicies := mockConfigurationPolicies(c.mockPolicies)
			mockStore.GetConfigurationPoliciesFunc.PushReturn(mockedStorePolicies, len(mockedStorePolicies), nil)
			mockUploadSvc.GetCommitsVisibleToUploadFunc.PushReturn(c.visibleCommits, nil, nil)

			mockGitserverClient.ListRefsFunc.PushReturn(c.refs, nil)

			matches, _, err := svc.GetRetentionPolicyOverview(context.Background(), c.upload, false, 10, 0, "", mockClock.Now())
			if err != nil {
				t.Fatalf("unexpected error resolving retention policy overview: %v", err)
			}

			var matchCount int
			for _, match := range matches {
				if match.Matched {
					matchCount++
				}
			}

			if matchCount != c.expectedMatches {
				t.Errorf("unexpected number of matched policies: want=%d have=%d", c.expectedMatches, matchCount)
			}

			if diff := cmp.Diff(expectedPolicyCandidates, matches); diff != "" {
				t.Errorf("unexpected retention policy matches (-want +got):\n%s", diff)
			}
		})
	}
}

func mockConfigurationPolicies(policies []policiesshared.RetentionPolicyMatchCandidate) (mockedCandidates []policiesshared.RetentionPolicyMatchCandidate, mockedPolicies []policiesshared.ConfigurationPolicy) {
	for i, policy := range policies {
		if policy.ConfigurationPolicy != nil {
			policy.ID = i + 1
			mockedPolicies = append(mockedPolicies, *policy.ConfigurationPolicy)
		}
		policies[i] = policy
		mockedCandidates = append(mockedCandidates, policy)
	}
	return
}

func defaultMockRepoStore() *dbmocks.MockRepoStore {
	repoStore := dbmocks.NewMockRepoStore()
	repoStore.GetFunc.SetDefaultHook(func(ctx context.Context, id api.RepoID) (*internaltypes.Repo, error) {
		return &internaltypes.Repo{
			ID:   id,
			Name: api.RepoName(fmt.Sprintf("r%d", id)),
		}, nil
	})
	return repoStore
}
