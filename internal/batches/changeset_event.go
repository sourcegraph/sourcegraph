package batches

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	gitlabwebhooks "github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab/webhooks"
)

type changesetEventUpdateMismatchError struct {
	field    string
	original interface{}
	revised  interface{}
}

func (e *changesetEventUpdateMismatchError) Error() string {
	return fmt.Sprintf("%s '%v' on the revised changeset event does not match %s '%v' on the original changeset event", e.field, e.revised, e.field, e.original)
}

// ChangesetEventKind defines the kind of a ChangesetEvent. This type is unexported
// so that users of ChangesetEvent can't instantiate it with a Kind being an arbitrary
// string.
type ChangesetEventKind string

// Valid ChangesetEvent kinds
const (
	ChangesetEventKindGitHubAssigned             ChangesetEventKind = "github:assigned"
	ChangesetEventKindGitHubClosed               ChangesetEventKind = "github:closed"
	ChangesetEventKindGitHubCommented            ChangesetEventKind = "github:commented"
	ChangesetEventKindGitHubRenamedTitle         ChangesetEventKind = "github:renamed"
	ChangesetEventKindGitHubMerged               ChangesetEventKind = "github:merged"
	ChangesetEventKindGitHubReviewed             ChangesetEventKind = "github:reviewed"
	ChangesetEventKindGitHubReopened             ChangesetEventKind = "github:reopened"
	ChangesetEventKindGitHubReviewDismissed      ChangesetEventKind = "github:review_dismissed"
	ChangesetEventKindGitHubReviewRequestRemoved ChangesetEventKind = "github:review_request_removed"
	ChangesetEventKindGitHubReviewRequested      ChangesetEventKind = "github:review_requested"
	ChangesetEventKindGitHubReviewCommented      ChangesetEventKind = "github:review_commented"
	ChangesetEventKindGitHubReadyForReview       ChangesetEventKind = "github:ready_for_review"
	ChangesetEventKindGitHubConvertToDraft       ChangesetEventKind = "github:convert_to_draft"
	ChangesetEventKindGitHubUnassigned           ChangesetEventKind = "github:unassigned"
	ChangesetEventKindGitHubCommit               ChangesetEventKind = "github:commit"
	ChangesetEventKindGitHubLabeled              ChangesetEventKind = "github:labeled"
	ChangesetEventKindGitHubUnlabeled            ChangesetEventKind = "github:unlabeled"
	ChangesetEventKindCommitStatus               ChangesetEventKind = "github:commit_status"
	ChangesetEventKindCheckSuite                 ChangesetEventKind = "github:check_suite"
	ChangesetEventKindCheckRun                   ChangesetEventKind = "github:check_run"

	ChangesetEventKindBitbucketServerApproved     ChangesetEventKind = "bitbucketserver:approved"
	ChangesetEventKindBitbucketServerUnapproved   ChangesetEventKind = "bitbucketserver:unapproved"
	ChangesetEventKindBitbucketServerDeclined     ChangesetEventKind = "bitbucketserver:declined"
	ChangesetEventKindBitbucketServerReviewed     ChangesetEventKind = "bitbucketserver:reviewed"
	ChangesetEventKindBitbucketServerOpened       ChangesetEventKind = "bitbucketserver:opened"
	ChangesetEventKindBitbucketServerReopened     ChangesetEventKind = "bitbucketserver:reopened"
	ChangesetEventKindBitbucketServerRescoped     ChangesetEventKind = "bitbucketserver:rescoped"
	ChangesetEventKindBitbucketServerUpdated      ChangesetEventKind = "bitbucketserver:updated"
	ChangesetEventKindBitbucketServerCommented    ChangesetEventKind = "bitbucketserver:commented"
	ChangesetEventKindBitbucketServerMerged       ChangesetEventKind = "bitbucketserver:merged"
	ChangesetEventKindBitbucketServerCommitStatus ChangesetEventKind = "bitbucketserver:commit_status"

	// BitbucketServer calls this an Unapprove event but we've called it Dismissed to more
	// clearly convey that it only occurs when a request for changes has been dismissed.
	ChangesetEventKindBitbucketServerDismissed ChangesetEventKind = "bitbucketserver:participant_status:unapproved"

	ChangesetEventKindGitLabApproved             ChangesetEventKind = "gitlab:approved"
	ChangesetEventKindGitLabClosed               ChangesetEventKind = "gitlab:closed"
	ChangesetEventKindGitLabMerged               ChangesetEventKind = "gitlab:merged"
	ChangesetEventKindGitLabPipeline             ChangesetEventKind = "gitlab:pipeline"
	ChangesetEventKindGitLabReopened             ChangesetEventKind = "gitlab:reopened"
	ChangesetEventKindGitLabUnapproved           ChangesetEventKind = "gitlab:unapproved"
	ChangesetEventKindGitLabMarkWorkInProgress   ChangesetEventKind = "gitlab:mark_wip"
	ChangesetEventKindGitLabUnmarkWorkInProgress ChangesetEventKind = "gitlab:unmark_wip"
)

// A ChangesetEvent is an event that happened in the lifetime
// and context of a Changeset.
type ChangesetEvent struct {
	ID          int64
	ChangesetID int64
	Kind        ChangesetEventKind
	Key         string // Deduplication key
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Metadata    interface{}
}

// Clone returns a clone of a ChangesetEvent.
func (e *ChangesetEvent) Clone() *ChangesetEvent {
	ee := *e
	return &ee
}

// ReviewAuthor returns the author of the review if the ChangesetEvent is related to a review.
// Returns an empty string if not a review event or the author has been deleted.
func (e *ChangesetEvent) ReviewAuthor() string {
	switch meta := e.Metadata.(type) {
	case *github.PullRequestReview:
		return meta.Author.Login

	case *github.ReviewDismissedEvent:
		return meta.Review.Author.Login

	case *bitbucketserver.Activity:
		return meta.User.Name

	case *bitbucketserver.ParticipantStatusEvent:
		return meta.User.Name

	case *gitlab.ReviewApprovedEvent:
		return meta.Author.Username

	case *gitlab.ReviewUnapprovedEvent:
		return meta.Author.Username

	default:
		return ""
	}
}

// ReviewState returns the review state of the ChangesetEvent if it is a review event.
func (e *ChangesetEvent) ReviewState() (ChangesetReviewState, error) {
	switch e.Kind {
	case ChangesetEventKindBitbucketServerApproved,
		ChangesetEventKindGitLabApproved:
		return ChangesetReviewStateApproved, nil

	// BitbucketServer's "REVIEWED" activity is created when someone clicks
	// the "Needs work" button in the UI, which is why we map it to "Changes Requested"
	case ChangesetEventKindBitbucketServerReviewed:
		return ChangesetReviewStateChangesRequested, nil

	case ChangesetEventKindGitHubReviewed:
		review, ok := e.Metadata.(*github.PullRequestReview)
		if !ok {
			return "", errors.New("ChangesetEvent metadata event not PullRequestReview")
		}

		s := ChangesetReviewState(strings.ToUpper(review.State))
		if !s.Valid() {
			// Ignore invalid states
			log15.Warn("invalid review state", "state", review.State)
			return ChangesetReviewStatePending, nil
		}
		return s, nil

	case ChangesetEventKindGitHubReviewDismissed,
		ChangesetEventKindBitbucketServerUnapproved,
		ChangesetEventKindBitbucketServerDismissed,
		ChangesetEventKindGitLabUnapproved:
		return ChangesetReviewStateDismissed, nil

	default:
		return ChangesetReviewStatePending, nil
	}
}

// Type returns the ChangesetEventKind of the ChangesetEvent.
func (e *ChangesetEvent) Type() ChangesetEventKind {
	return e.Kind
}

// Changeset returns the changeset ID of the ChangesetEvent.
func (e *ChangesetEvent) Changeset() int64 {
	return e.ChangesetID
}

// Timestamp returns the time when the ChangesetEvent happened (or was updated)
// on the codehost, not when it was created in Sourcegraph's database.
func (e *ChangesetEvent) Timestamp() time.Time {
	var t time.Time

	switch ev := e.Metadata.(type) {
	case *github.AssignedEvent:
		t = ev.CreatedAt
	case *github.ClosedEvent:
		t = ev.CreatedAt
	case *github.IssueComment:
		t = ev.UpdatedAt
	case *github.RenamedTitleEvent:
		t = ev.CreatedAt
	case *github.MergedEvent:
		t = ev.CreatedAt
	case *github.PullRequestReview:
		t = ev.UpdatedAt
	case *github.PullRequestReviewComment:
		t = ev.UpdatedAt
	case *github.ReopenedEvent:
		t = ev.CreatedAt
	case *github.ReviewDismissedEvent:
		t = ev.CreatedAt
	case *github.ReviewRequestRemovedEvent:
		t = ev.CreatedAt
	case *github.ReviewRequestedEvent:
		t = ev.CreatedAt
	case *github.ReadyForReviewEvent:
		t = ev.CreatedAt
	case *github.ConvertToDraftEvent:
		t = ev.CreatedAt
	case *github.UnassignedEvent:
		t = ev.CreatedAt
	case *github.LabelEvent:
		t = ev.CreatedAt
	case *github.CommitStatus:
		t = ev.ReceivedAt
	case *github.CheckSuite:
		t = ev.ReceivedAt
	case *github.CheckRun:
		t = ev.ReceivedAt
	case *bitbucketserver.Activity:
		t = unixMilliToTime(int64(ev.CreatedDate))
	case *bitbucketserver.ParticipantStatusEvent:
		t = unixMilliToTime(int64(ev.CreatedDate))
	case *bitbucketserver.CommitStatus:
		t = unixMilliToTime(ev.Status.DateAdded)
	case *gitlab.ReviewApprovedEvent:
		t = ev.CreatedAt.Time
	case *gitlab.ReviewUnapprovedEvent:
		t = ev.CreatedAt.Time
	case *gitlab.MarkWorkInProgressEvent:
		t = ev.CreatedAt.Time
	case *gitlab.UnmarkWorkInProgressEvent:
		t = ev.CreatedAt.Time
	case *gitlab.MergeRequestClosedEvent:
		t = ev.CreatedAt.Time
	case *gitlab.MergeRequestReopenedEvent:
		t = ev.CreatedAt.Time
	case *gitlab.MergeRequestMergedEvent:
		t = ev.CreatedAt.Time
	case *gitlabwebhooks.PipelineEvent:
		// These events do not inherently have timestamps from GitLab, so we
		// fall back to the event record we created when we received the
		// webhook.
		t = e.CreatedAt
	}

	return t
}

// Update updates the metadata of e with new metadata in o.
func (e *ChangesetEvent) Update(o *ChangesetEvent) error {
	if e.ChangesetID != o.ChangesetID {
		return &changesetEventUpdateMismatchError{
			field:    "ChangesetID",
			original: e.ChangesetID,
			revised:  o.ChangesetID,
		}
	}
	if e.Kind != o.Kind {
		return &changesetEventUpdateMismatchError{
			field:    "Kind",
			original: e.Kind,
			revised:  o.Kind,
		}
	}
	if e.Key != o.Key {
		return &changesetEventUpdateMismatchError{
			field:    "Key",
			original: e.Key,
			revised:  o.Key,
		}
	}

	switch e := e.Metadata.(type) {
	case *github.LabelEvent:
		o := o.Metadata.(*github.LabelEvent)

		if e.Actor == (github.Actor{}) {
			e.Actor = o.Actor
		}

		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}

		if e.Label == (github.Label{}) {
			e.Label = o.Label
		}

	case *github.AssignedEvent:
		o := o.Metadata.(*github.AssignedEvent)

		if e.Actor == (github.Actor{}) {
			e.Actor = o.Actor
		}

		if e.Assignee == (github.Actor{}) {
			e.Assignee = o.Assignee
		}

		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}

	case *github.ClosedEvent:
		o := o.Metadata.(*github.ClosedEvent)

		if e.Actor == (github.Actor{}) {
			e.Actor = o.Actor
		}

		if o.URL != "" && e.URL != o.URL {
			e.URL = o.URL
		}

		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}

	case *github.IssueComment:
		o := o.Metadata.(*github.IssueComment)

		if e.DatabaseID == 0 {
			e.DatabaseID = o.DatabaseID
		}

		if e.Author == (github.Actor{}) {
			e.Author = o.Author
		}

		if e.Editor == nil {
			e.Editor = o.Editor
		}

		if o.AuthorAssociation != "" && e.AuthorAssociation != o.AuthorAssociation {
			e.AuthorAssociation = o.AuthorAssociation
		}

		if o.Body != "" && e.Body != o.Body {
			e.Body = o.Body
		}

		if o.URL != "" && e.URL != o.URL {
			e.URL = o.URL
		}

		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}

		if e.UpdatedAt.Before(o.UpdatedAt) {
			e.UpdatedAt = o.UpdatedAt
		}

		if o.IncludesCreatedEdit {
			e.IncludesCreatedEdit = true
		}

	case *github.RenamedTitleEvent:
		o := o.Metadata.(*github.RenamedTitleEvent)

		if e.Actor == (github.Actor{}) {
			e.Actor = o.Actor
		}

		if o.PreviousTitle != "" && e.PreviousTitle != o.PreviousTitle {
			e.PreviousTitle = o.PreviousTitle
		}

		if o.CurrentTitle != "" && e.CurrentTitle != o.CurrentTitle {
			e.CurrentTitle = o.CurrentTitle
		}

		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}

	case *github.MergedEvent:
		o := o.Metadata.(*github.MergedEvent)

		if e.Actor == (github.Actor{}) {
			e.Actor = o.Actor
		}

		if o.MergeRefName != "" && e.MergeRefName != o.MergeRefName {
			e.MergeRefName = o.MergeRefName
		}

		if o.URL != "" && e.URL != o.URL {
			e.URL = o.URL
		}

		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}

		updateGitHubCommit(&e.Commit, &o.Commit)

	case *github.PullRequestReview:
		o := o.Metadata.(*github.PullRequestReview)

		updateGitHubPullRequestReview(e, o)

	case *github.PullRequestReviewComment:
		o := o.Metadata.(*github.PullRequestReviewComment)

		if e.DatabaseID == 0 {
			e.DatabaseID = o.DatabaseID
		}

		if e.Author == (github.Actor{}) {
			e.Author = o.Author
		}

		if o.AuthorAssociation != "" && e.AuthorAssociation != o.AuthorAssociation {
			e.AuthorAssociation = o.AuthorAssociation
		}

		if e.Editor == (github.Actor{}) {
			e.Editor = o.Editor
		}

		if o.Body != "" && e.Body != o.Body {
			e.Body = o.Body
		}

		if o.State != "" && e.State != o.State {
			e.State = o.State
		}

		if o.URL != "" && e.URL != o.URL {
			e.URL = o.URL
		}

		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}

		if e.UpdatedAt.Before(o.UpdatedAt) {
			e.UpdatedAt = o.UpdatedAt
		}

		if e, o := e.Commit, o.Commit; !reflect.DeepEqual(e, o) {
			updateGitHubCommit(&e, &o)
		}

		if o.IncludesCreatedEdit {
			e.IncludesCreatedEdit = true
		}

	case *github.ReopenedEvent:
		o := o.Metadata.(*github.ReopenedEvent)

		if e.Actor == (github.Actor{}) {
			e.Actor = o.Actor
		}

		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}
	case *github.ReviewDismissedEvent:
		o := o.Metadata.(*github.ReviewDismissedEvent)

		if e.Actor == (github.Actor{}) {
			e.Actor = o.Actor
		}

		if o.DismissalMessage != "" && e.DismissalMessage != o.DismissalMessage {
			e.DismissalMessage = o.DismissalMessage
		}

		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}

		updateGitHubPullRequestReview(&e.Review, &o.Review)

	case *github.ReviewRequestRemovedEvent:
		o := o.Metadata.(*github.ReviewRequestRemovedEvent)

		if e.Actor == (github.Actor{}) {
			e.Actor = o.Actor
		}

		if e.RequestedReviewer == (github.Actor{}) {
			e.RequestedReviewer = o.RequestedReviewer
		}

		if e.RequestedTeam == (github.Team{}) {
			e.RequestedTeam = o.RequestedTeam
		}

		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}

	case *github.ReviewRequestedEvent:
		o := o.Metadata.(*github.ReviewRequestedEvent)

		if e.Actor == (github.Actor{}) {
			e.Actor = o.Actor
		}

		if e.RequestedReviewer == (github.Actor{}) {
			e.RequestedReviewer = o.RequestedReviewer
		}

		if e.RequestedTeam == (github.Team{}) {
			e.RequestedTeam = o.RequestedTeam
		}

		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}

	case *github.ReadyForReviewEvent:
		o := o.Metadata.(*github.ReadyForReviewEvent)

		if e.Actor == (github.Actor{}) {
			e.Actor = o.Actor
		}

		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}

	case *github.ConvertToDraftEvent:
		o := o.Metadata.(*github.ConvertToDraftEvent)

		if e.Actor == (github.Actor{}) {
			e.Actor = o.Actor
		}

		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}

	case *github.UnassignedEvent:
		o := o.Metadata.(*github.UnassignedEvent)

		if e.Actor == (github.Actor{}) {
			e.Actor = o.Actor
		}

		if e.Assignee == (github.Actor{}) {
			e.Assignee = o.Assignee
		}

		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}
	case *bitbucketserver.Activity:
		o := o.Metadata.(*bitbucketserver.Activity)

		if e.CreatedDate == 0 {
			e.CreatedDate = o.CreatedDate
		}

		if e.User == (bitbucketserver.User{}) {
			e.User = o.User
		}

		if e.Action == "" {
			e.Action = o.Action
		}

		if e.CommentAction == "" {
			e.CommentAction = o.CommentAction
		}

		if e.Comment == nil && o.Comment != nil {
			e.Comment = o.Comment
		}

		if len(e.AddedReviewers) == 0 {
			e.AddedReviewers = o.AddedReviewers
		}

		if len(e.RemovedReviewers) == 0 {
			e.RemovedReviewers = o.RemovedReviewers
		}

		if e.Commit == nil && o.Commit != nil {
			e.Commit = o.Commit
		}

	case *bitbucketserver.ParticipantStatusEvent:
		o := o.Metadata.(*bitbucketserver.ParticipantStatusEvent)

		if e.CreatedDate == 0 {
			e.CreatedDate = o.CreatedDate
		}

		if e.Action == "" {
			e.Action = o.Action
		}

		if e.User == (bitbucketserver.User{}) {
			e.User = o.User
		}

	case *bitbucketserver.CommitStatus:
		o := o.Metadata.(*bitbucketserver.CommitStatus)
		// We always get the full event, so safe to replace it
		*e = *o

	case *github.CheckRun:
		o := o.Metadata.(*github.CheckRun)
		if e.Status == "" {
			e.Status = o.Status
		}
		if e.Conclusion == "" {
			e.Conclusion = o.Conclusion
		}

	case *github.CheckSuite:
		o := o.Metadata.(*github.CheckSuite)
		if e.Status == "" {
			e.Status = o.Status
		}
		if e.Conclusion == "" {
			e.Conclusion = o.Conclusion
		}
		e.CheckRuns = o.CheckRuns

	case *gitlab.ReviewApprovedEvent:
		o := o.Metadata.(*gitlab.ReviewApprovedEvent)
		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}
		if !e.System {
			e.System = o.System
		}
		if e.Body == "" {
			e.Body = o.Body
		}
		if e.Author.ID == 0 {
			e.Author = o.Author
		}

	case *gitlab.ReviewUnapprovedEvent:
		o := o.Metadata.(*gitlab.ReviewUnapprovedEvent)
		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}
		if !e.System {
			e.System = o.System
		}
		if e.Body == "" {
			e.Body = o.Body
		}
		if e.Author.ID == 0 {
			e.Author = o.Author
		}

	case *gitlab.MarkWorkInProgressEvent:
		o := o.Metadata.(*gitlab.MarkWorkInProgressEvent)
		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}
		if !e.System {
			e.System = o.System
		}
		if e.Body == "" {
			e.Body = o.Body
		}
		if e.Author.ID == 0 {
			e.Author = o.Author
		}

	case *gitlab.UnmarkWorkInProgressEvent:
		o := o.Metadata.(*gitlab.UnmarkWorkInProgressEvent)
		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}
		if !e.System {
			e.System = o.System
		}
		if e.Body == "" {
			e.Body = o.Body
		}
		if e.Author.ID == 0 {
			e.Author = o.Author
		}

	case *gitlab.MergeRequestClosedEvent:
		o := o.Metadata.(*gitlab.MergeRequestClosedEvent)
		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}
		if e.ResourceID == 0 {
			e.ResourceID = o.ResourceID
		}
		if e.User.ID == 0 {
			e.User = o.User
		}

	case *gitlab.MergeRequestReopenedEvent:
		o := o.Metadata.(*gitlab.MergeRequestReopenedEvent)
		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}
		if e.ResourceID == 0 {
			e.ResourceID = o.ResourceID
		}
		if e.User.ID == 0 {
			e.User = o.User
		}

	case *gitlab.MergeRequestMergedEvent:
		o := o.Metadata.(*gitlab.MergeRequestMergedEvent)
		if e.CreatedAt.IsZero() {
			e.CreatedAt = o.CreatedAt
		}
		if e.ResourceID == 0 {
			e.ResourceID = o.ResourceID
		}
		if e.User.ID == 0 {
			e.User = o.User
		}

	case *gitlabwebhooks.PipelineEvent:
		o := o.Metadata.(*gitlabwebhooks.PipelineEvent)
		// We always get the full event, so safe to replace it
		*e = *o

	default:
		return errors.Errorf("unknown changeset event metadata %T", e)
	}

	return nil
}

////////////////////////////////////////////////////
// Helpers for updating changesets from metadata. //
////////////////////////////////////////////////////

func updateGitHubPullRequestReview(e, o *github.PullRequestReview) {
	if e.DatabaseID == 0 {
		e.DatabaseID = o.DatabaseID
	}

	if e.Author == (github.Actor{}) {
		e.Author = o.Author
	}

	if o.AuthorAssociation != "" && e.AuthorAssociation != o.AuthorAssociation {
		e.AuthorAssociation = o.AuthorAssociation
	}

	if o.Body != "" && e.Body != o.Body {
		e.Body = o.Body
	}

	if o.State != "" && e.State != o.State {
		e.State = o.State
	}

	if o.URL != "" && e.URL != o.URL {
		e.URL = o.URL
	}

	if e.CreatedAt.IsZero() {
		e.CreatedAt = o.CreatedAt
	}

	if e.UpdatedAt.Before(o.UpdatedAt) {
		e.UpdatedAt = o.UpdatedAt
	}

	if e, o := e.Commit, o.Commit; !reflect.DeepEqual(e, o) {
		updateGitHubCommit(&e, &o)
	}

	if o.IncludesCreatedEdit {
		e.IncludesCreatedEdit = true
	}
}

func updateGitHubCommit(e, o *github.Commit) {
	if o.OID != "" && e.OID != o.OID {
		e.OID = o.OID
	}

	if o.Message != "" && e.Message != o.Message {
		e.Message = o.Message
	}

	if o.MessageHeadline != "" && e.MessageHeadline != o.MessageHeadline {
		e.MessageHeadline = o.MessageHeadline
	}

	if o.URL != "" && e.URL != o.URL {
		e.URL = o.URL
	}

	if e.Committer != (github.GitActor{}) && e.Committer != o.Committer {
		e.Committer = o.Committer
	}

	if e.CommittedDate.IsZero() {
		e.CommittedDate = o.CommittedDate
	}

	if e.PushedDate.IsZero() {
		e.PushedDate = o.PushedDate
	}
}
