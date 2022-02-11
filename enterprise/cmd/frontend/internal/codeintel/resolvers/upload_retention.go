package resolvers

import (
	"context"
	"sort"
	"time"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/policies"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/dbstore"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type RetentionPolicyMatchCandidate struct {
	dbstore.ConfigurationPolicy
	Matched           bool
	ProtectingCommits []string
}

func (r *resolver) RetentionPolicyOverview(ctx context.Context, upload dbstore.Upload, matchesOnly bool, first int, after int64, query string) ([]RetentionPolicyMatchCandidate, int, error) {
	policyMatcher := policies.NewMatcher(r.gitserverClient, policies.RetentionExtractor, false, false)

	policies, _, err := r.GetConfigurationPolicies(ctx, dbstore.GetConfigurationPoliciesOptions{
		RepositoryID:     upload.RepositoryID,
		Term:             query,
		ForDataRetention: true,
		Limit:            first,
		Offset:           int(after),
	})
	if err != nil {
		return nil, 0, err
	}

	visibileCommits, err := r.commitsVisibleToUpload(ctx, upload)
	if err != nil {
		return nil, 0, err
	}

	matchingPolicies, err := policyMatcher.CommitsDescribedByPolicy(ctx, upload.RepositoryID, policies, time.Now(), visibileCommits...)
	if err != nil {
		return nil, 0, err
	}

	var (
		now                    = time.Now()
		potentialMatchIndexSet map[int]int // map of polciy ID to array index
		potentialMatches       []RetentionPolicyMatchCandidate
	)

	if matchesOnly {
		potentialMatches, _ = r.populateMatchingCommits(visibileCommits, upload, matchingPolicies, policies, now)
	} else {
		potentialMatches, potentialMatchIndexSet = r.populateMatchingCommits(visibileCommits, upload, matchingPolicies, policies, now)

		// populate with remaining unmatched policies
		for _, policy := range policies {
			if _, ok := potentialMatchIndexSet[policy.ID]; !ok {
				potentialMatches = append(potentialMatches, RetentionPolicyMatchCandidate{
					ConfigurationPolicy: policy,
					Matched:             false,
				})
			}
		}
	}

	sort.Slice(potentialMatches, func(i, j int) bool {
		return potentialMatches[i].ID < potentialMatches[j].ID
	})

	return potentialMatches, len(potentialMatches), nil
}

func (r *resolver) commitsVisibleToUpload(ctx context.Context, upload dbstore.Upload) (commits []string, err error) {
	var token *string
	for first := true; first || token != nil; first = false {
		cs, nextToken, err := r.dbStore.CommitsVisibleToUpload(ctx, upload.ID, 50, token)
		if err != nil {
			return nil, errors.Wrap(err, "dbstore.CommitsVisibleToUpload")
		}
		token = nextToken

		commits = append(commits, cs...)
	}

	return
}

// populateMatchingCommits builds a slice of all retention policies that, either directly or via
// a visibile upload, apply to the upload. It returns the slice of policies and the set of matching
// policy IDs mapped to their index in the slice.
func (r *resolver) populateMatchingCommits(
	visibileCommits []string,
	upload dbstore.Upload,
	matchingPolicies map[string][]policies.PolicyMatch,
	policies []dbstore.ConfigurationPolicy,
	now time.Time,
) ([]RetentionPolicyMatchCandidate, map[int]int) {
	var (
		potentialMatches       = make([]RetentionPolicyMatchCandidate, 0, len(policies))
		potentialMatchIndexSet = make(map[int]int, len(policies))
	)

	// First add all matches for the commit of this upload. We do this to ensure that if a policy matches both the upload's commit
	// and a visible commit, we ensure an entry for that policy is only added for the upload's commit. This makes the logic in checking
	// the visible commits a bit simpler, as we don't have to check if policy X has already been added for a visible commit in the case
	// that the upload's commit is not first in the list.
	if policyMatches, ok := matchingPolicies[upload.Commit]; ok {
		for _, policyMatch := range policyMatches {
			potentialMatches = append(potentialMatches, RetentionPolicyMatchCandidate{
				ConfigurationPolicy: *policyByID(policies, *policyMatch.PolicyID),
				Matched:             true,
			})
			potentialMatchIndexSet[*policyMatch.PolicyID] = len(potentialMatches) - 1
		}
	}

	for _, commit := range visibileCommits {
		if commit == upload.Commit {
			continue
		}
		if policyMatches, ok := matchingPolicies[commit]; ok {
			for _, policyMatch := range policyMatches {
				if policyMatch.PolicyDuration == nil || now.Sub(upload.UploadedAt) < *policyMatch.PolicyDuration {
					if index, ok := potentialMatchIndexSet[*policyMatch.PolicyID]; ok && potentialMatches[index].ProtectingCommits != nil {
						//  If an entry for the policy already exists and it has > 1 "protecting commits", add this commit too.
						potentialMatches[index].ProtectingCommits = append(potentialMatches[index].ProtectingCommits, commit)
					} else if !ok {
						// Else if theres no entry for the policy, create an entry with this commit as the first "protecting commit".
						// This should never override an entry for a policy matched directly, see the first comment on how this is avoided.
						potentialMatches = append(potentialMatches, RetentionPolicyMatchCandidate{
							ConfigurationPolicy: *policyByID(policies, *policyMatch.PolicyID),
							Matched:             true,
							ProtectingCommits:   []string{commit},
						})
						potentialMatchIndexSet[*policyMatch.PolicyID] = len(potentialMatches) - 1
					}
				}
			}
		}
	}

	return potentialMatches, potentialMatchIndexSet
}

func policyByID(policies []dbstore.ConfigurationPolicy, id int) *dbstore.ConfigurationPolicy {
	for _, policy := range policies {
		if policy.ID == id {
			return &policy
		}
	}
	return nil
}
