package sources

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"

	adobatches "github.com/sourcegraph/sourcegraph/internal/batches/sources/azuredevops"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/azuredevops"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

type AzureDevOpsSource struct {
	client azuredevops.Client
}

var _ ForkableChangesetSource = AzureDevOpsSource{}

func NewAzureDevOpsSource(ctx context.Context, svc *types.ExternalService, cf *httpcli.Factory) (*AzureDevOpsSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.AzureDevOpsConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Wrapf(err, "external service id=%d", svc.ID)
	}

	if cf == nil {
		cf = httpcli.NewExternalClientFactory()
	}

	client, err := azuredevops.NewClient(svc.URN(), c.Url, &auth.BasicAuth{Username: c.Username, Password: c.Token}, cf)
	if err != nil {
		return nil, errors.Wrap(err, "creating Azure DevOps client")
	}

	return &AzureDevOpsSource{client: client}, nil
}

// GitserverPushConfig returns an authenticated push config used for pushing
// commits to the code host.
func (s AzureDevOpsSource) GitserverPushConfig(repo *types.Repo) (*protocol.PushConfig, error) {
	return GitserverPushConfig(repo, s.client.Authenticator())
}

// WithAuthenticator returns a copy of the original Source configured to use the
// given authenticator, provided that authenticator type is supported by the
// code host.
func (s AzureDevOpsSource) WithAuthenticator(a auth.Authenticator) (ChangesetSource, error) {
	client, err := s.client.WithAuthenticator(a)
	if err != nil {
		return nil, err
	}

	return &AzureDevOpsSource{client: client}, nil
}

// ValidateAuthenticator validates the currently set authenticator is usable.
// Returns an error, when validating the Authenticator yielded an error.
func (s AzureDevOpsSource) ValidateAuthenticator(ctx context.Context) error {
	_, err := s.client.GetAuthorizedProfile(ctx)
	return err
}

// LoadChangeset loads the given Changeset from the source and updates it. If
// the Changeset could not be found on the source, a ChangesetNotFoundError is
// returned.
func (s AzureDevOpsSource) LoadChangeset(ctx context.Context, cs *Changeset) error {
	repo := cs.TargetRepo.Metadata.(*azuredevops.Repository)
	args, err := s.createCommonPullRequestArgs(*repo, *cs)
	if err != nil {
		return err
	}

	pr, err := s.client.GetPullRequest(ctx, args)
	if err != nil {
		if errcode.IsNotFound(err) {
			return ChangesetNotFoundError{Changeset: cs}
		}
		return errors.Wrap(err, "getting pull request")
	}

	return errors.Wrap(s.setChangesetMetadata(ctx, repo, &pr, cs), "setting Azure DevOps changeset metadata")
}

// CreateChangeset will create the Changeset on the source. If it already
// exists, *Changeset will be populated and the return value will be true.
func (s AzureDevOpsSource) CreateChangeset(ctx context.Context, cs *Changeset) (bool, error) {
	input := s.changesetToPullRequestInput(cs)
	return s.createChangeset(ctx, cs, input)
}

// CreateDraftChangeset creates the given changeset on the code host in draft mode.
func (s AzureDevOpsSource) CreateDraftChangeset(ctx context.Context, cs *Changeset) (bool, error) {
	input := s.changesetToPullRequestInput(cs)
	input.IsDraft = true
	return s.createChangeset(ctx, cs, input)
}

func (s AzureDevOpsSource) createChangeset(ctx context.Context, cs *Changeset, input azuredevops.CreatePullRequestInput) (bool, error) {
	repo := cs.TargetRepo.Metadata.(*azuredevops.Repository)
	org, err := repo.GetOrganization()
	if err != nil {
		return false, errors.Wrap(err, "getting Azure DevOps organization from project")
	}
	args := azuredevops.OrgProjectRepoArgs{
		Org:          org,
		Project:      repo.Project.Name,
		RepoNameOrID: repo.Name,
	}

	pr, err := s.client.CreatePullRequest(ctx, args, input)
	if err != nil {
		return false, errors.Wrap(err, "creating pull request")
	}

	if err := s.setChangesetMetadata(ctx, repo, &pr, cs); err != nil {
		return false, errors.Wrap(err, "setting Azure DevOps changeset metadata")
	}

	return true, nil
}

// UndraftChangeset will update the Changeset on the source to be not in draft mode anymore.
func (s AzureDevOpsSource) UndraftChangeset(ctx context.Context, cs *Changeset) error {
	input := s.changesetToUpdatePullRequestInput(cs, false)
	isDraft := false
	input.IsDraft = &isDraft
	repo := cs.TargetRepo.Metadata.(*azuredevops.Repository)
	args, err := s.createCommonPullRequestArgs(*repo, *cs)
	if err != nil {
		return err
	}

	updated, err := s.client.UpdatePullRequest(ctx, args, input)
	if err != nil {
		return errors.Wrap(err, "updating pull request")
	}

	return errors.Wrap(s.setChangesetMetadata(ctx, repo, &updated, cs), "setting Azure DevOps changeset metadata")
}

// CloseChangeset will close the Changeset on the source, where "close"
// means the appropriate final state on the codehost (e.g. "abandoned" on
// AzureDevOps).
func (s AzureDevOpsSource) CloseChangeset(ctx context.Context, cs *Changeset) error {
	repo := cs.TargetRepo.Metadata.(*azuredevops.Repository)
	args, err := s.createCommonPullRequestArgs(*repo, *cs)
	if err != nil {
		return err
	}

	updated, err := s.client.AbandonPullRequest(ctx, args)
	if err != nil {
		return errors.Wrap(err, "abandoning pull request")
	}

	// TODO: We ought to check the AutoDeleteBranch setting here and delete the branch if
	// it's set, but we don't have all the necessary details of the head ref here in order
	// to perform that update, so currently we only honor the setting on "completion" aka
	// merge. In order to accomplish this, we would need to issue a POST request to update
	// the ref and supply its name and old Object ID (which we don't have) and then
	// "0000000000000000000000000000000000000000" as the new Object ID. See
	// https://learn.microsoft.com/en-us/rest/api/azure/devops/git/refs/update-refs?view=azure-devops-rest-7.0&tabs=HTTP#gitrefupdate

	return errors.Wrap(s.setChangesetMetadata(ctx, repo, &updated, cs), "setting Azure DevOps changeset metadata")
}

// UpdateChangeset can update Changesets.
func (s AzureDevOpsSource) UpdateChangeset(ctx context.Context, cs *Changeset) error {
	repo := cs.TargetRepo.Metadata.(*azuredevops.Repository)
	args, err := s.createCommonPullRequestArgs(*repo, *cs)
	if err != nil {
		return err
	}

	// ADO does not support updating the target branch alongside other fields, so we have
	// to check it separately, and make 2 calls if there is a change.
	pr, err := s.client.GetPullRequest(ctx, args)
	if err != nil {
		if errcode.IsNotFound(err) {
			return ChangesetNotFoundError{Changeset: cs}
		}
		return errors.Wrap(err, "getting pull request")
	}
	if pr.TargetRefName != cs.BaseRef {
		input := s.changesetToUpdatePullRequestInput(cs, true)
		_, err := s.client.UpdatePullRequest(ctx, args, input)
		if err != nil {
			return errors.Wrap(err, "updating pull request")
		}
	}

	input := s.changesetToUpdatePullRequestInput(cs, false)
	updated, err := s.client.UpdatePullRequest(ctx, args, input)
	if err != nil {
		return errors.Wrap(err, "updating pull request")
	}

	return errors.Wrap(s.setChangesetMetadata(ctx, repo, &updated, cs), "setting Azure DevOps changeset metadata")
}

// ReopenChangeset will reopen the Changeset on the source, if it's closed.
// If not, it's a noop.
func (s AzureDevOpsSource) ReopenChangeset(ctx context.Context, cs *Changeset) error {
	deleteSourceBranch := conf.Get().BatchChangesAutoDeleteBranch
	input := azuredevops.PullRequestUpdateInput{
		Status: &azuredevops.PullRequestStatusActive,
		CompletionOptions: &azuredevops.PullRequestCompletionOptions{
			DeleteSourceBranch: deleteSourceBranch,
		},
	}
	repo := cs.TargetRepo.Metadata.(*azuredevops.Repository)
	args, err := s.createCommonPullRequestArgs(*repo, *cs)
	if err != nil {
		return err
	}

	updated, err := s.client.UpdatePullRequest(ctx, args, input)
	if err != nil {
		return errors.Wrap(err, "updating pull request")
	}

	return errors.Wrap(s.setChangesetMetadata(ctx, repo, &updated, cs), "setting Azure DevOps changeset metadata")
}

// CreateComment posts a comment on the Changeset.
func (s AzureDevOpsSource) CreateComment(ctx context.Context, cs *Changeset, comment string) error {
	repo := cs.TargetRepo.Metadata.(*azuredevops.Repository)
	args, err := s.createCommonPullRequestArgs(*repo, *cs)
	if err != nil {
		return err
	}

	_, err = s.client.CreatePullRequestCommentThread(ctx, args, azuredevops.PullRequestCommentInput{
		Comments: []azuredevops.PullRequestCommentForInput{
			{
				ParentCommentID: 0,
				Content:         comment,
				CommentType:     1,
			},
		},
	})
	return err
}

// MergeChangeset merges a Changeset on the code host, if in a mergeable state.
// If squash is true, and the code host supports squash merges, the source
// must attempt a squash merge. Otherwise, it is expected to perform a regular
// merge. If the changeset cannot be merged, because it is in an unmergeable
// state, ChangesetNotMergeableError must be returned.
func (s AzureDevOpsSource) MergeChangeset(ctx context.Context, cs *Changeset, squash bool) error {
	repo := cs.TargetRepo.Metadata.(*azuredevops.Repository)
	args, err := s.createCommonPullRequestArgs(*repo, *cs)
	if err != nil {
		return err
	}

	var mergeStrategy *azuredevops.PullRequestMergeStrategy
	if squash {
		ms := azuredevops.PullRequestMergeStrategySquash
		mergeStrategy = &ms
	}

	deleteSourceBranch := conf.Get().BatchChangesAutoDeleteBranch
	updated, err := s.client.CompletePullRequest(ctx, args, azuredevops.PullRequestCompleteInput{
		CommitID:           cs.SyncState.HeadRefOid,
		MergeStrategy:      mergeStrategy,
		DeleteSourceBranch: deleteSourceBranch,
	})
	if err != nil {
		if errcode.IsNotFound(err) {
			return errors.Wrap(err, "merging pull request")
		}
		return ChangesetNotMergeableError{ErrorMsg: err.Error()}
	}

	return errors.Wrap(s.setChangesetMetadata(ctx, repo, &updated, cs), "setting Azure DevOps changeset metadata")
}

// GetFork returns a repo pointing to a fork of the target repo, ensuring that the fork
// exists and creating it if it doesn't. If namespace is not provided, the original namespace is used.
// If name is not provided, the fork will be named with the default Sourcegraph convention:
// "${original-namespace}-${original-name}"
func (s AzureDevOpsSource) GetFork(ctx context.Context, targetRepo *types.Repo, ns, n *string) (*types.Repo, error) {
	tr := targetRepo.Metadata.(*azuredevops.Repository)

	var namespace string
	if ns == nil {
		namespace = tr.Namespace()
	} else {
		namespace = *ns
	}

	targetNamespace := tr.Namespace()

	var name string
	if n != nil {
		name = *n
	} else {
		name = DefaultForkName(targetNamespace, tr.Name)
	}

	org, err := tr.GetOrganization()
	if err != nil {
		return nil, err
	}

	// Figure out if we already have a fork of the repo in the given namespace.
	fork, err := s.client.GetRepo(ctx, azuredevops.OrgProjectRepoArgs{
		Org:          org,
		Project:      namespace,
		RepoNameOrID: name,
	})

	// If we already have the forked repo, there is no need to create it, we can return early.
	if err == nil {
		return s.checkAndCopy(targetRepo, &fork)
	} else if !errcode.IsNotFound(err) {
		return nil, errors.Wrap(err, "checking for fork existence")
	}

	pFork := tr.Project

	// If the fork is in a different namespace(project), we need to get that so we can get the ID.
	if namespace != tr.Namespace() {
		pFork, err = s.client.GetProject(ctx, org, namespace)
		if err != nil {
			return nil, err
		}
	}

	fork, err = s.client.ForkRepository(ctx, org, azuredevops.ForkRepositoryInput{
		Name: name,
		Project: azuredevops.ForkRepositoryInputProject{
			ID: pFork.ID,
		},
		ParentRepository: azuredevops.ForkRepositoryInputParentRepository{
			ID: tr.ID,
			Project: azuredevops.ForkRepositoryInputProject{
				ID: tr.Project.ID,
			},
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "forking repository")
	}

	return s.checkAndCopy(targetRepo, &fork)
}

func (s AzureDevOpsSource) BuildCommitOpts(repo *types.Repo, _ *btypes.Changeset, spec *btypes.ChangesetSpec, pushOpts *protocol.PushConfig) protocol.CreateCommitFromPatchRequest {
	return BuildCommitOptsCommon(repo, spec, pushOpts)
}

// checkAndCopy creates a types.Repo representation of the forked repository useing the original repo (targetRepo).
func (s AzureDevOpsSource) checkAndCopy(targetRepo *types.Repo, fork *azuredevops.Repository) (*types.Repo, error) {
	if !fork.IsFork {
		return nil, errors.New("repo is not a fork")
	}

	// Now we make a copy of targetRepo, but with its sources and metadata updated to
	// point to the fork
	forkNamespace := fork.Namespace()
	forkRepo, err := copyAzureDevOpsRepoAsFork(targetRepo, fork, forkNamespace, fork.Name)
	if err != nil {
		return nil, errors.Wrap(err, "updating target repo sources")
	}

	return forkRepo, nil
}

func (s AzureDevOpsSource) annotatePullRequest(ctx context.Context, repo *azuredevops.Repository, pr *azuredevops.PullRequest) (*adobatches.AnnotatedPullRequest, error) {
	org, err := repo.GetOrganization()
	if err != nil {
		return nil, err
	}
	srs, err := s.client.GetPullRequestStatuses(ctx, azuredevops.PullRequestCommonArgs{
		PullRequestID: strconv.Itoa(pr.ID),
		Org:           org,
		Project:       repo.Project.Name,
		RepoNameOrID:  repo.Name,
	})
	if err != nil {
		return nil, errors.Wrap(err, "getting pull request statuses")
	}

	var statuses []*azuredevops.PullRequestBuildStatus
	for _, status := range srs {
		localStatus := status
		statuses = append(statuses, &localStatus)
	}

	return &adobatches.AnnotatedPullRequest{
		PullRequest: pr,
		Statuses:    statuses,
	}, nil
}

func (s AzureDevOpsSource) setChangesetMetadata(ctx context.Context, repo *azuredevops.Repository, pr *azuredevops.PullRequest, cs *Changeset) error {
	apr, err := s.annotatePullRequest(ctx, repo, pr)
	if err != nil {
		return errors.Wrap(err, "annotating pull request")
	}

	if err := cs.SetMetadata(apr); err != nil {
		return errors.Wrap(err, "setting changeset metadata")
	}

	return nil
}

func (s AzureDevOpsSource) changesetToPullRequestInput(cs *Changeset) azuredevops.CreatePullRequestInput {
	deleteSourceBranch := conf.Get().BatchChangesAutoDeleteBranch
	input := azuredevops.CreatePullRequestInput{
		Title:         cs.Title,
		Description:   cs.Body,
		SourceRefName: cs.HeadRef,
		TargetRefName: cs.BaseRef,
		CompletionOptions: &azuredevops.PullRequestCompletionOptions{
			DeleteSourceBranch: deleteSourceBranch,
		},
	}

	// If we're forking, then we need to set the source repository as well.
	if cs.RemoteRepo != cs.TargetRepo {
		input.ForkSource = &azuredevops.ForkRef{
			Repository: *cs.RemoteRepo.Metadata.(*azuredevops.Repository),
		}
	}

	return input
}

func (s AzureDevOpsSource) changesetToUpdatePullRequestInput(cs *Changeset, targetRefChanged bool) azuredevops.PullRequestUpdateInput {
	targetRef := gitdomain.EnsureRefPrefix(cs.BaseRef)
	if targetRefChanged {
		return azuredevops.PullRequestUpdateInput{
			TargetRefName: &targetRef,
		}
	}

	deleteSourceBranch := conf.Get().BatchChangesAutoDeleteBranch
	return azuredevops.PullRequestUpdateInput{
		Title:       &cs.Title,
		Description: &cs.Body,
		CompletionOptions: &azuredevops.PullRequestCompletionOptions{
			DeleteSourceBranch: deleteSourceBranch,
		},
	}
}

func (s AzureDevOpsSource) createCommonPullRequestArgs(repo azuredevops.Repository, cs Changeset) (azuredevops.PullRequestCommonArgs, error) {
	org, err := repo.GetOrganization()
	if err != nil {
		return azuredevops.PullRequestCommonArgs{}, errors.Wrap(err, "getting Azure DevOps organization from project")
	}
	return azuredevops.PullRequestCommonArgs{
		PullRequestID: cs.ExternalID,
		Org:           org,
		Project:       repo.Project.Name,
		RepoNameOrID:  repo.Name,
	}, nil
}

func copyAzureDevOpsRepoAsFork(repo *types.Repo, fork *azuredevops.Repository, forkNamespace, forkName string) (*types.Repo, error) {
	if repo.Sources == nil || len(repo.Sources) == 0 {
		return nil, errors.New("repo has no sources")
	}

	forkRepo := *repo
	forkSources := map[string]*types.SourceInfo{}

	for urn, src := range repo.Sources {
		if src == nil || src.CloneURL == "" {
			continue
		}
		forkURL, err := url.Parse(src.CloneURL)
		if err != nil {
			return nil, err
		}

		// Will look like: /org/project/_git/repo, project is our namespace.
		forkURLPathSplit := strings.SplitN(forkURL.Path, "/", 5)
		if len(forkURLPathSplit) < 5 {
			return nil, errors.Errorf("repo has malformed clone url: %s", src.CloneURL)
		}
		forkURLPathSplit[2] = forkNamespace
		forkURLPathSplit[4] = forkName

		forkPath := strings.Join(forkURLPathSplit, "/")
		forkURL.Path = forkPath

		forkSources[urn] = &types.SourceInfo{
			ID:       src.ID,
			CloneURL: forkURL.String(),
		}
	}

	forkRepo.Sources = forkSources
	forkRepo.Metadata = fork

	return &forkRepo, nil
}
