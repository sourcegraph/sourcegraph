package policies

import (
	"context"
	"sort"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/policies/internal/store"
	policiesshared "github.com/sourcegraph/sourcegraph/internal/codeintel/policies/shared"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Service struct {
	store      store.Store
	repoStore  database.RepoStore
	uploadSvc  UploadService
	gitserver  gitserver.Client
	operations *operations
}

func newService(
	observationCtx *observation.Context,
	policiesStore store.Store,
	repoStore database.RepoStore,
	uploadSvc UploadService,
	gitserver gitserver.Client,
) *Service {
	return &Service{
		store:      policiesStore,
		repoStore:  repoStore,
		uploadSvc:  uploadSvc,
		gitserver:  gitserver,
		operations: newOperations(observationCtx),
	}
}

func (s *Service) getPolicyMatcherFromFactory(extractor Extractor, includeTipOfDefaultBranch bool, filterByCreatedDate bool) *Matcher {
	return NewMatcher(s.gitserver, extractor, includeTipOfDefaultBranch, filterByCreatedDate)
}

func (s *Service) GetConfigurationPolicies(ctx context.Context, opts policiesshared.GetConfigurationPoliciesOptions) ([]policiesshared.ConfigurationPolicy, int, error) {
	return s.store.GetConfigurationPolicies(ctx, opts)
}

func (s *Service) GetConfigurationPolicyByID(ctx context.Context, id int) (policiesshared.ConfigurationPolicy, bool, error) {
	return s.store.GetConfigurationPolicyByID(ctx, id)
}

func (s *Service) CreateConfigurationPolicy(ctx context.Context, configurationPolicy policiesshared.ConfigurationPolicy) (policiesshared.ConfigurationPolicy, error) {
	policy, err := s.store.CreateConfigurationPolicy(ctx, configurationPolicy)
	if err != nil {
		return policy, err
	}

	if err := s.updateReposMatchingPolicyPatterns(ctx, policy); err != nil {
		return policy, err
	}

	return policy, nil
}

func (s *Service) updateReposMatchingPolicyPatterns(ctx context.Context, policy policiesshared.ConfigurationPolicy) error {
	var patterns []string
	if policy.RepositoryPatterns != nil {
		patterns = *policy.RepositoryPatterns
	}

	if len(patterns) == 0 {
		return nil
	}

	var repositoryMatchLimit *int
	if val := conf.CodeIntelAutoIndexingPolicyRepositoryMatchLimit(); val != -1 {
		repositoryMatchLimit = &val
	}

	if err := s.store.UpdateReposMatchingPatterns(ctx, patterns, policy.ID, repositoryMatchLimit); err != nil {
		return err
	}

	return nil
}

func (s *Service) UpdateConfigurationPolicy(ctx context.Context, policy policiesshared.ConfigurationPolicy) (err error) {
	ctx, _, endObservation := s.operations.updateConfigurationPolicy.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	if err := s.store.UpdateConfigurationPolicy(ctx, policy); err != nil {
		return err
	}

	return s.updateReposMatchingPolicyPatterns(ctx, policy)
}

func (s *Service) DeleteConfigurationPolicyByID(ctx context.Context, id int) error {
	return s.store.DeleteConfigurationPolicyByID(ctx, id)
}

func (s *Service) GetRetentionPolicyOverview(ctx context.Context, upload shared.Upload, matchesOnly bool, first int, after int64, query string, now time.Time) (matches []policiesshared.RetentionPolicyMatchCandidate, totalCount int, err error) {
	ctx, _, endObservation := s.operations.getRetentionPolicyOverview.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	var (
		t             = true
		policyMatcher = s.getPolicyMatcherFromFactory(RetentionExtractor, true, false)
	)

	configPolicies, _, err := s.GetConfigurationPolicies(ctx, policiesshared.GetConfigurationPoliciesOptions{
		RepositoryID:     upload.RepositoryID,
		Term:             query,
		ForDataRetention: &t,
		Limit:            first,
		Offset:           int(after),
	})
	if err != nil {
		return nil, 0, err
	}

	visibleCommits, err := s.getCommitsVisibleToUpload(ctx, upload)
	if err != nil {
		return nil, 0, err
	}

	repo, err := s.repoStore.Get(ctx, api.RepoID(upload.RepositoryID))
	if err != nil {
		return nil, 0, err
	}

	matchingPolicies, err := policyMatcher.CommitsDescribedByPolicy(ctx, upload.RepositoryID, repo.Name, configPolicies, time.Now(), visibleCommits...)
	if err != nil {
		return nil, 0, err
	}

	var (
		potentialMatchIndexSet map[int]int // map of policy ID to array index
		potentialMatches       []policiesshared.RetentionPolicyMatchCandidate
	)

	potentialMatches, potentialMatchIndexSet = s.populateMatchingCommits(visibleCommits, upload, matchingPolicies, configPolicies, now)

	if !matchesOnly {
		// populate with remaining unmatched policies
		for _, policy := range configPolicies {
			policy := policy
			if _, ok := potentialMatchIndexSet[policy.ID]; !ok {
				potentialMatches = append(potentialMatches, policiesshared.RetentionPolicyMatchCandidate{
					ConfigurationPolicy: &policy,
					Matched:             false,
				})
			}
		}
	}

	sort.Slice(potentialMatches, func(i, j int) bool {
		// Sort implicit policy at the top
		if potentialMatches[i].ConfigurationPolicy == nil {
			return true
		} else if potentialMatches[j].ConfigurationPolicy == nil {
			return false
		}

		// Then sort matches first
		if potentialMatches[i].Matched {
			return !potentialMatches[j].Matched
		}
		if potentialMatches[j].Matched {
			return false
		}

		// Then sort by ids
		return potentialMatches[i].ID < potentialMatches[j].ID
	})

	return potentialMatches, len(potentialMatches), nil
}

func (s *Service) GetPreviewRepositoryFilter(ctx context.Context, patterns []string, limit int) (_ []int, totalCount int, matchesAll bool, repositoryMatchLimit *int, err error) {
	ctx, _, endObservation := s.operations.getPreviewRepositoryFilter.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	if val := conf.CodeIntelAutoIndexingPolicyRepositoryMatchLimit(); val != -1 {
		repositoryMatchLimit = &val

		if limit > *repositoryMatchLimit {
			limit = *repositoryMatchLimit
		}
	}

	ids, totalCount, err := s.store.GetRepoIDsByGlobPatterns(ctx, patterns, limit, 0)
	if err != nil {
		return nil, 0, false, nil, err
	}
	totalRepoCount, err := s.store.RepoCount(ctx)
	if err != nil {
		return nil, 0, false, nil, err
	}

	return ids, totalCount, totalCount == totalRepoCount, repositoryMatchLimit, nil
}

type GitObject struct {
	Name        string
	Rev         string
	CommittedAt time.Time
}

func (s *Service) GetPreviewGitObjectFilter(
	ctx context.Context,
	repositoryID int,
	gitObjectType policiesshared.GitObjectType,
	pattern string,
	limit int,
	countObjectsYoungerThanHours *int32,
) (_ []GitObject, totalCount int, totalCountYoungerThanThreshold *int, err error) {
	ctx, _, endObservation := s.operations.getPreviewGitObjectFilter.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	repo, err := s.repoStore.Get(ctx, api.RepoID(repositoryID))
	if err != nil {
		return nil, 0, nil, err
	}

	policyMatcher := s.getPolicyMatcherFromFactory(NoopExtractor, false, false)
	policyMatches, err := policyMatcher.CommitsDescribedByPolicy(
		ctx,
		repositoryID,
		repo.Name,
		[]policiesshared.ConfigurationPolicy{{Type: gitObjectType, Pattern: pattern}},
		timeutil.Now(),
	)
	if err != nil {
		return nil, 0, nil, err
	}

	gitObjects := make([]GitObject, 0, len(policyMatches))
	for commit, policyMatches := range policyMatches {
		for _, policyMatch := range policyMatches {
			gitObjects = append(gitObjects, GitObject{
				Name:        policyMatch.Name,
				Rev:         commit,
				CommittedAt: policyMatch.CommittedAt,
			})
		}
	}
	sort.Slice(gitObjects, func(i, j int) bool {
		if countObjectsYoungerThanHours != nil && gitObjects[i].CommittedAt != gitObjects[j].CommittedAt {
			return !gitObjects[i].CommittedAt.Before(gitObjects[j].CommittedAt)
		}

		if gitObjects[i].Name == gitObjects[j].Name {
			return gitObjects[i].Rev < gitObjects[j].Rev
		}

		return gitObjects[i].Name < gitObjects[j].Name
	})

	if countObjectsYoungerThanHours != nil {
		count := 0
		for _, gitObject := range gitObjects {
			if time.Since(gitObject.CommittedAt) <= time.Duration(*countObjectsYoungerThanHours)*time.Hour {
				count++
			}
		}

		totalCountYoungerThanThreshold = &count
	}

	totalCount = len(gitObjects)
	if limit < totalCount {
		gitObjects = gitObjects[:limit]
	}

	return gitObjects, totalCount, totalCountYoungerThanThreshold, nil
}

func (s *Service) getCommitsVisibleToUpload(ctx context.Context, upload shared.Upload) (commits []string, err error) {
	var token *string
	for first := true; first || token != nil; first = false {
		cs, nextToken, err := s.uploadSvc.GetCommitsVisibleToUpload(ctx, upload.ID, 50, token)
		if err != nil {
			return nil, errors.Wrap(err, "uploadSvc.GetCommitsVisibleToUpload")
		}
		token = nextToken

		commits = append(commits, cs...)
	}

	return
}

// populateMatchingCommits builds a slice of all retention policies that, either directly or via
// a visible upload, apply to the upload. It returns the slice of policies and the set of matching
// policy IDs mapped to their index in the slice.
func (s *Service) populateMatchingCommits(
	visibleCommits []string,
	upload shared.Upload,
	matchingPolicies map[string][]PolicyMatch,
	policies []policiesshared.ConfigurationPolicy,
	now time.Time,
) ([]policiesshared.RetentionPolicyMatchCandidate, map[int]int) {
	var (
		potentialMatches       = make([]policiesshared.RetentionPolicyMatchCandidate, 0, len(policies))
		potentialMatchIndexSet = make(map[int]int, len(policies))
	)

	// First add all matches for the commit of this upload. We do this to ensure that if a policy matches both the upload's commit
	// and a visible commit, we ensure an entry for that policy is only added for the upload's commit. This makes the logic in checking
	// the visible commits a bit simpler, as we don't have to check if policy X has already been added for a visible commit in the case
	// that the upload's commit is not first in the list.
	if policyMatches, ok := matchingPolicies[upload.Commit]; ok {
		for _, policyMatch := range policyMatches {
			if policyMatch.PolicyDuration == nil || now.Sub(upload.UploadedAt) < *policyMatch.PolicyDuration {
				policyID := -1
				if policyMatch.PolicyID != nil {
					policyID = *policyMatch.PolicyID
				}
				potentialMatches = append(potentialMatches, policiesshared.RetentionPolicyMatchCandidate{
					ConfigurationPolicy: policyByID(policies, policyID),
					Matched:             true,
				})
				potentialMatchIndexSet[policyID] = len(potentialMatches) - 1
			}
		}
	}

	for _, commit := range visibleCommits {
		if commit == upload.Commit {
			continue
		}
		if policyMatches, ok := matchingPolicies[commit]; ok {
			for _, policyMatch := range policyMatches {
				if policyMatch.PolicyDuration == nil || now.Sub(upload.UploadedAt) < *policyMatch.PolicyDuration {
					policyID := -1
					if policyMatch.PolicyID != nil {
						policyID = *policyMatch.PolicyID
					}
					if index, ok := potentialMatchIndexSet[policyID]; ok && potentialMatches[index].ProtectingCommits != nil {
						//  If an entry for the policy already exists and it has > 1 "protecting commits", add this commit too.
						potentialMatches[index].ProtectingCommits = append(potentialMatches[index].ProtectingCommits, commit)
					} else if !ok {
						// Else if there's no entry for the policy, create an entry with this commit as the first "protecting commit".
						// This should never override an entry for a policy matched directly, see the first comment on how this is avoided.
						potentialMatches = append(potentialMatches, policiesshared.RetentionPolicyMatchCandidate{
							ConfigurationPolicy: policyByID(policies, policyID),
							Matched:             true,
							ProtectingCommits:   []string{commit},
						})
						potentialMatchIndexSet[policyID] = len(potentialMatches) - 1
					}
				}
			}
		}
	}

	return potentialMatches, potentialMatchIndexSet
}

func policyByID(policies []policiesshared.ConfigurationPolicy, id int) *policiesshared.ConfigurationPolicy {
	if id == -1 {
		return nil
	}

	for _, policy := range policies {
		if policy.ID == id {
			return &policy
		}
	}

	return nil
}
