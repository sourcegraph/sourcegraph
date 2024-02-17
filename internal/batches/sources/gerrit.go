package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	gerritbatches "github.com/sourcegraph/sourcegraph/internal/batches/sources/gerrit"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gerrit"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

type GerritSource struct {
	client gerrit.Client
}

func NewGerritSource(ctx context.Context, svc *types.ExternalService, cf *httpcli.Factory) (*GerritSource, error) {
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.GerritConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Wrapf(err, "external service id=%d", svc.ID)
	}

	if cf == nil {
		cf = httpcli.NewExternalClientFactory()
	}

	gerritURL, err := url.Parse(c.Url)
	if err != nil {
		return nil, errors.Wrap(err, "parsing Gerrit CodeHostURL")
	}

	client, err := gerrit.NewClient(svc.URN(), gerritURL, &gerrit.AccountCredentials{Username: c.Username, Password: c.Password}, cf)
	if err != nil {
		return nil, errors.Wrap(err, "creating Gerrit client")
	}

	return &GerritSource{client: client}, nil
}

// GitserverPushConfig returns an authenticated push config used for pushing
// commits to the code host.
func (s GerritSource) GitserverPushConfig(repo *types.Repo) (*protocol.PushConfig, error) {
	return GitserverPushConfig(repo, s.client.Authenticator())
}

// WithAuthenticator returns a copy of the original Source configured to use the
// given authenticator, provided that authenticator type is supported by the
// code host.
func (s GerritSource) WithAuthenticator(a auth.Authenticator) (ChangesetSource, error) {
	client, err := s.client.WithAuthenticator(a)
	if err != nil {
		return nil, err
	}

	return &GerritSource{client: client}, nil
}

// ValidateAuthenticator validates the currently set authenticator is usable.
// Returns an error, when validating the Authenticator yielded an error.
func (s GerritSource) ValidateAuthenticator(ctx context.Context) error {
	_, err := s.client.GetAuthenticatedUserAccount(ctx)
	return err
}

// LoadChangeset loads the given Changeset from the source and updates it. If
// the Changeset could not be found on the source, a ChangesetNotFoundError is
// returned.
func (s GerritSource) LoadChangeset(ctx context.Context, cs *Changeset) error {
	pr, err := s.client.GetChange(ctx, cs.ExternalID)
	if err != nil {
		if errcode.IsNotFound(err) {
			return ChangesetNotFoundError{Changeset: cs}
		}
		return errors.Wrap(err, "getting change")
	}
	return errors.Wrap(s.setChangesetMetadata(ctx, pr, cs), "setting Gerrit changeset metadata")
}

// CreateChangeset will create the Changeset on the source. If it already
// exists, *Changeset will be populated and the return value will be true.
func (s GerritSource) CreateChangeset(ctx context.Context, cs *Changeset) (bool, error) {
	changeID := GenerateGerritChangeID(*cs.Changeset)
	// For Gerrit, the Change is created at `git push` time, so we just load it here to verify it
	// was created successfully.
	pr, err := s.client.GetChange(ctx, changeID)
	if err != nil {
		if errcode.IsNotFound(err) {
			return false, ChangesetNotFoundError{Changeset: cs}
		}
		return false, errors.Wrap(err, "getting change")
	}

	// The Changeset technically "exists" at this point because it gets created at push time,
	// therefore exists would always return true. However, we send false here because otherwise we would always
	// enqueue a ChangesetUpdate webhook event instead of the regular publish event.
	return false, errors.Wrap(s.setChangesetMetadata(ctx, pr, cs), "setting Gerrit changeset metadata")
}

// CreateDraftChangeset creates the given changeset on the code host in draft mode.
func (s GerritSource) CreateDraftChangeset(ctx context.Context, cs *Changeset) (bool, error) {
	changeID := GenerateGerritChangeID(*cs.Changeset)

	// For Gerrit, the Change is created at `git push` time, so we just call the API to mark it as WIP.
	if err := s.client.SetWIP(ctx, changeID); err != nil {
		if errcode.IsNotFound(err) {
			return false, ChangesetNotFoundError{Changeset: cs}
		}
		return false, errors.Wrap(err, "making change WIP")
	}

	pr, err := s.client.GetChange(ctx, changeID)
	if err != nil {
		if errcode.IsNotFound(err) {
			return false, ChangesetNotFoundError{Changeset: cs}
		}
		return false, errors.Wrap(err, "getting change")
	}
	// The Changeset technically "exists" at this point because it gets created at push time,
	// therefore exists would always return true. However, we send false here because otherwise we would always
	// enqueue a ChangesetUpdate webhook event instead of the regular publish event.
	return false, errors.Wrap(s.setChangesetMetadata(ctx, pr, cs), "setting Gerrit changeset metadata")
}

// UndraftChangeset will update the Changeset on the source to be not in draft mode anymore.
func (s GerritSource) UndraftChangeset(ctx context.Context, cs *Changeset) error {
	if err := s.client.SetReadyForReview(ctx, cs.ExternalID); err != nil {
		if errcode.IsNotFound(err) {
			return ChangesetNotFoundError{Changeset: cs}
		}
		return errors.Wrap(err, "setting change as ready")
	}

	if err := s.LoadChangeset(ctx, cs); err != nil {
		return errors.Wrap(err, "getting change")
	}
	return nil
}

// CloseChangeset will close the Changeset on the source, where "close"
// means the appropriate final state on the codehost (e.g. "abandoned" on
// Gerrit).
func (s GerritSource) CloseChangeset(ctx context.Context, cs *Changeset) error {
	updated, err := s.client.AbandonChange(ctx, cs.ExternalID)
	if err != nil {
		return errors.Wrap(err, "abandoning change")
	}

	if conf.Get().BatchChangesAutoDeleteBranch {
		if err := s.client.DeleteChange(ctx, cs.ExternalID); err != nil {
			return errors.Wrap(err, "deleting change")
		}
	}

	return errors.Wrap(s.setChangesetMetadata(ctx, updated, cs), "setting Gerrit changeset metadata")
}

// UpdateChangeset can update Changesets.
func (s GerritSource) UpdateChangeset(ctx context.Context, cs *Changeset) error {
	pr, err := s.client.GetChange(ctx, cs.ExternalID)
	if err != nil {
		// Route 1
		// The most recent push has created two Gerrit changes with the same Change ID.
		// This happens when the target branch is changed at the same time that the diffs are changed,
		// it is a bit of a fringe scenario, but it causes us to have 2 changes with the same Change ID,
		// but different ID. What we do here, is delete the change that existed before our most
		// recent push, and then load the new change now that it doesn't have a conflict.
		if errors.As(err, &gerrit.MultipleChangesError{}) {
			originalPR := cs.Metadata.(*gerritbatches.AnnotatedChange)
			err = s.client.DeleteChange(ctx, originalPR.Change.ID)
			if err != nil {
				return errors.Wrap(err, "deleting change")
			}
			// If the original PR was a WIP, the new one needs to be as well.
			if originalPR.Change.WorkInProgress {
				err = s.client.SetWIP(ctx, cs.ExternalID)
				if err != nil {
					return errors.Wrap(err, "setting updated change as WIP")
				}
			}
			return s.LoadChangeset(ctx, cs)
		} else {
			if errcode.IsNotFound(err) {
				return ChangesetNotFoundError{Changeset: cs}
			}
			return errors.Wrap(err, "getting newer change")
		}
	}
	// Route 2
	// We did not push before this, therefore this update, is only through API
	if pr.Branch != cs.BaseRef {
		_, err = s.client.MoveChange(ctx, cs.ExternalID, gerrit.MoveChangePayload{
			DestinationBranch: cs.BaseRef,
		})
		if err != nil {
			return errors.Wrap(err, "moving change")
		}
	}
	if pr.Subject != cs.Title {
		err = s.client.SetCommitMessage(ctx, cs.ExternalID, gerrit.SetCommitMessagePayload{
			Message: fmt.Sprintf("%s\n\nChange-Id: %s\n", cs.Title, cs.ExternalID),
		})
		if err != nil {
			return errors.Wrap(err, "setting change commit message")
		}
	}
	return s.LoadChangeset(ctx, cs)
}

// ReopenChangeset will reopen the Changeset on the source, if it's closed.
// If not, it's a noop.
func (s GerritSource) ReopenChangeset(ctx context.Context, cs *Changeset) error {
	updated, err := s.client.RestoreChange(ctx, cs.ExternalID)
	if err != nil {
		return errors.Wrap(err, "restoring change")
	}

	return errors.Wrap(s.setChangesetMetadata(ctx, updated, cs), "setting Gerrit changeset metadata")
}

// CreateComment posts a comment on the Changeset.
func (s GerritSource) CreateComment(ctx context.Context, cs *Changeset, comment string) error {
	return s.client.WriteReviewComment(ctx, cs.ExternalID, gerrit.ChangeReviewComment{
		Message: comment,
	})
}

// MergeChangeset merges a Changeset on the code host, if in a mergeable state.
// If squash is true, and the code host supports squash merges, the source
// must attempt a squash merge. Otherwise, it is expected to perform a regular
// merge. If the changeset cannot be merged, because it is in an unmergeable
// state, ChangesetNotMergeableError must be returned.
// Gerrit changes are always single commit, so squash does not matter.
func (s GerritSource) MergeChangeset(ctx context.Context, cs *Changeset, _ bool) error {
	updated, err := s.client.SubmitChange(ctx, cs.ExternalID)
	if err != nil {
		if errcode.IsNotFound(err) {
			return errors.Wrap(err, "submitting change")
		}
		return ChangesetNotMergeableError{ErrorMsg: err.Error()}
	}
	return errors.Wrap(s.setChangesetMetadata(ctx, updated, cs), "setting Gerrit changeset metadata")
}

func (s GerritSource) BuildCommitOpts(repo *types.Repo, changeset *btypes.Changeset, spec *btypes.ChangesetSpec, pushOpts *protocol.PushConfig) protocol.CreateCommitFromPatchRequest {
	opts := BuildCommitOptsCommon(repo, spec, pushOpts)
	pushRef := strings.Replace(gitdomain.EnsureRefPrefix(spec.BaseRef), "refs/heads", "refs/for", 1) //Magical Gerrit ref for pushing changes.
	opts.PushRef = &pushRef
	changeID := changeset.ExternalID
	if changeID == "" {
		changeID = GenerateGerritChangeID(*changeset)
	}
	// We append the "title" as the first line of the commit message because Gerrit doesn't have a concept of title.
	opts.CommitInfo.Messages = append([]string{spec.Title}, opts.CommitInfo.Messages...)
	// We attach the Change ID to the bottom of the commit message because this is how Gerrit creates it's Changes.
	opts.CommitInfo.Messages = append(opts.CommitInfo.Messages, "Change-Id: "+changeID)
	return opts
}

func (s GerritSource) setChangesetMetadata(ctx context.Context, change *gerrit.Change, cs *Changeset) error {
	apr, err := s.annotateChange(ctx, change)
	if err != nil {
		return errors.Wrap(err, "annotating Change")
	}
	if err = cs.SetMetadata(apr); err != nil {
		return errors.Wrap(err, "setting changeset metadata")
	}
	return nil
}

func (s GerritSource) annotateChange(ctx context.Context, change *gerrit.Change) (*gerritbatches.AnnotatedChange, error) {
	reviewers, err := s.client.GetChangeReviews(ctx, change.ChangeID)
	if err != nil {
		return nil, err
	}
	return &gerritbatches.AnnotatedChange{
		Change:      change,
		Reviewers:   *reviewers,
		CodeHostURL: *s.client.GetURL(),
	}, nil
}

// GenerateGerritChangeID deterministically generates a Gerrit Change ID from a Changeset object.
// We do this because Gerrit Change IDs are required at commit time, and deterministically generating
// the Change IDs allows us to locate and track a Change once it's created.
func GenerateGerritChangeID(cs btypes.Changeset) string {
	jsonData, err := json.Marshal(cs)
	if err != nil {
		panic(err)
	}

	hash := sha256.Sum256(jsonData)
	hexString := hex.EncodeToString(hash[:])
	changeID := hexString[:40]

	return "I" + strings.ToLower(changeID)
}
