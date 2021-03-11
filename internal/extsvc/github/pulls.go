package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"github.com/segmentio/fasthash/fnv1"
)

// PageInfo contains the paging information based on the Redux conventions.
type PageInfo struct {
	HasNextPage bool
	EndCursor   string
}

// An Actor represents an object which can take actions on GitHub. Typically a User or Bot.
type Actor struct {
	AvatarURL string
	Login     string
	URL       string
}

// A Team represents a team on Github.
type Team struct {
	Name string
	URL  string
}

// A GitActor represents an actor in a Git commit (ie. an author or committer).
type GitActor struct {
	AvatarURL string
	Email     string
	Name      string
	User      *Actor `json:"User,omitempty"`
}

// A Review of a PullRequest.
type Review struct {
	Body        string
	State       string
	URL         string
	Author      Actor
	Commit      Commit
	CreatedAt   time.Time
	SubmittedAt time.Time
}

// CheckSuite represents the status of a checksuite
type CheckSuite struct {
	ID string
	// One of COMPLETED, IN_PROGRESS, QUEUED, REQUESTED
	Status string
	// One of ACTION_REQUIRED, CANCELLED, FAILURE, NEUTRAL, SUCCESS, TIMED_OUT
	Conclusion string
	ReceivedAt time.Time
	// When the suite was received via a webhook
	CheckRuns struct{ Nodes []CheckRun }
}

func (c *CheckSuite) Key() string {
	key := fmt.Sprintf("%s:%s:%s:%d", c.ID, c.Status, c.Conclusion, c.ReceivedAt.UnixNano())
	return strconv.FormatUint(fnv1.HashString64(key), 16)
}

// CheckRun represents the status of a checkrun
type CheckRun struct {
	ID string
	// One of COMPLETED, IN_PROGRESS, QUEUED, REQUESTED
	Status string
	// One of ACTION_REQUIRED, CANCELLED, FAILURE, NEUTRAL, SUCCESS, TIMED_OUT
	Conclusion string
	// When the run was received via a webhook
	ReceivedAt time.Time
}

func (c *CheckRun) Key() string {
	key := fmt.Sprintf("%s:%s:%s:%d", c.ID, c.Status, c.Conclusion, c.ReceivedAt.UnixNano())
	return strconv.FormatUint(fnv1.HashString64(key), 16)
}

// A Commit in a Repository.
type Commit struct {
	OID             string
	Message         string
	MessageHeadline string
	URL             string
	Committer       GitActor
	CommittedDate   time.Time
	PushedDate      time.Time
}

// A Status represents a Commit status.
type Status struct {
	State    string
	Contexts []Context
}

// CommitStatus represents the state of a commit context received
// via the StatusEvent webhook
type CommitStatus struct {
	SHA        string
	Context    string
	State      string
	ReceivedAt time.Time
}

func (c *CommitStatus) Key() string {
	key := fmt.Sprintf("%s:%s:%s:%d", c.SHA, c.State, c.Context, c.ReceivedAt.UnixNano())
	return strconv.FormatInt(int64(fnv1.HashString64(key)), 16)
}

// Context represent the individual commit status context
type Context struct {
	ID          string
	Context     string
	Description string
	State       string
}

type Label struct {
	ID          string
	Color       string
	Description string
	Name        string
}

// PullRequest is a GitHub pull request.
type PullRequest struct {
	RepoWithOwner string `json:"-"`
	ID            string
	Title         string
	Body          string
	State         string
	URL           string
	HeadRefOid    string
	BaseRefOid    string
	HeadRefName   string
	BaseRefName   string
	Number        int64
	Author        Actor
	Participants  []Actor
	Labels        struct{ Nodes []Label }
	TimelineItems []TimelineItem
	Commits       struct{ Nodes []CommitWithChecks }
	IsDraft       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// AssignedEvent represents an 'assigned' event on a PullRequest.
type AssignedEvent struct {
	Actor     Actor
	Assignee  Actor
	CreatedAt time.Time
}

// Key is a unique key identifying this event in the context of its pull request.
func (e AssignedEvent) Key() string {
	return fmt.Sprintf("%s:%s:%d", e.Actor.Login, e.Assignee.Login, e.CreatedAt.UnixNano())
}

// ClosedEvent represents a 'closed' event on a PullRequest.
type ClosedEvent struct {
	Actor     Actor
	CreatedAt time.Time
	URL       string
}

// Key is a unique key identifying this event in the context of its pull request.
func (e ClosedEvent) Key() string {
	return fmt.Sprintf("%s:%d", e.Actor.Login, e.CreatedAt.UnixNano())
}

// IssueComment represents a comment on an PullRequest that isn't
// a commit or review comment.
type IssueComment struct {
	DatabaseID          int64
	Author              Actor
	Editor              *Actor
	AuthorAssociation   string
	Body                string
	URL                 string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	IncludesCreatedEdit bool
}

// Key is a unique key identifying this event in the context of its pull request.
func (e IssueComment) Key() string {
	return strconv.FormatInt(e.DatabaseID, 10)
}

// RenamedTitleEvent represents a 'renamed' event on a given pull request.
type RenamedTitleEvent struct {
	Actor         Actor
	PreviousTitle string
	CurrentTitle  string
	CreatedAt     time.Time
}

// Key is a unique key identifying this event in the context of its pull request.
func (e RenamedTitleEvent) Key() string {
	return fmt.Sprintf("%s:%d", e.Actor.Login, e.CreatedAt.UnixNano())
}

// MergedEvent represents a 'merged' event on a given pull request.
type MergedEvent struct {
	Actor        Actor
	MergeRefName string
	URL          string
	Commit       Commit
	CreatedAt    time.Time
}

// Key is a unique key identifying this event in the context of its pull request.
func (e MergedEvent) Key() string {
	return fmt.Sprintf("%s:%d", e.Actor.Login, e.CreatedAt.UnixNano())
}

// PullRequestReview represents a review on a given pull request.
type PullRequestReview struct {
	DatabaseID          int64
	Author              Actor
	AuthorAssociation   string
	Body                string
	State               string
	URL                 string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Commit              Commit
	IncludesCreatedEdit bool
}

// Key is a unique key identifying this event in the context of its pull request.
func (e PullRequestReview) Key() string {
	return strconv.FormatInt(e.DatabaseID, 10)
}

// PullRequestReviewThread represents a thread of review comments on a given pull request.
// Since webhooks only send pull request review comment payloads, we normalize
// each thread we receive via GraphQL, and don't store this event as the metadata
// of a ChangesetEvent, instead storing each contained comment as a separate ChangesetEvent.
// That's why this type doesn't have a Key method like the others.
type PullRequestReviewThread struct {
	Comments []*PullRequestReviewComment
}

type PullRequestCommit struct {
	Commit Commit
}

func (p PullRequestCommit) Key() string {
	return p.Commit.OID
}

// CommitWithChecks represents check/build status of a commit. When we load the PR
// from GitHub we fetch the most recent commit into this type to check build status.
type CommitWithChecks struct {
	Commit struct {
		OID           string
		CheckSuites   struct{ Nodes []CheckSuite }
		Status        Status
		CommittedDate time.Time
	}
}

// PullRequestReviewComment represents a review comment on a given pull request.
type PullRequestReviewComment struct {
	DatabaseID          int64
	Author              Actor
	AuthorAssociation   string
	Editor              Actor
	Commit              Commit
	Body                string
	State               string
	URL                 string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	IncludesCreatedEdit bool
}

// Key is a unique key identifying this event in the context of its pull request.
func (e PullRequestReviewComment) Key() string {
	return strconv.FormatInt(e.DatabaseID, 10)
}

// ReopenedEvent represents a 'reopened' event on a pull request.
type ReopenedEvent struct {
	Actor     Actor
	CreatedAt time.Time
}

// Key is a unique key identifying this event in the context of its pull request.
func (e ReopenedEvent) Key() string {
	return fmt.Sprintf("%s:%d", e.Actor.Login, e.CreatedAt.UnixNano())
}

// ReviewDismissedEvent represents a 'review_dismissed' event on a pull request.
type ReviewDismissedEvent struct {
	Actor            Actor
	Review           PullRequestReview
	DismissalMessage string
	CreatedAt        time.Time
}

// Key is a unique key identifying this event in the context of its pull request.
func (e ReviewDismissedEvent) Key() string {
	return fmt.Sprintf(
		"%s:%d:%d",
		e.Actor.Login,
		e.Review.DatabaseID,
		e.CreatedAt.UnixNano(),
	)
}

// ReviewRequestRemovedEvent represents a 'review_request_removed' event on a
// pull request.
type ReviewRequestRemovedEvent struct {
	Actor             Actor
	RequestedReviewer Actor
	RequestedTeam     Team
	CreatedAt         time.Time
}

// Key is a unique key identifying this event in the context of its pull request.
func (e ReviewRequestRemovedEvent) Key() string {
	requestedFrom := e.RequestedReviewer.Login
	if requestedFrom == "" {
		requestedFrom = e.RequestedTeam.Name
	}

	return fmt.Sprintf("%s:%s:%d", e.Actor.Login, requestedFrom, e.CreatedAt.UnixNano())
}

// ReviewRequestedRevent represents a 'review_requested' event on a
// pull request.
type ReviewRequestedEvent struct {
	Actor             Actor
	RequestedReviewer Actor
	RequestedTeam     Team
	CreatedAt         time.Time
}

// Key is a unique key identifying this event in the context of its pull request.
func (e ReviewRequestedEvent) Key() string {
	requestedFrom := e.RequestedReviewer.Login
	if requestedFrom == "" {
		requestedFrom = e.RequestedTeam.Name
	}

	return fmt.Sprintf("%s:%s:%d", e.Actor.Login, requestedFrom, e.CreatedAt.UnixNano())
}

// ReviewerDeleted returns true if both RequestedReviewer and RequestedTeam are
// blank, indicating that one or the other has been deleted.
// We use it to drop the event.
func (e ReviewRequestedEvent) ReviewerDeleted() bool {
	return e.RequestedReviewer.Login == "" && e.RequestedTeam.Name == ""
}

// ReadyForReviewEvent represents a 'ready_for_review' event on a
// pull request.
type ReadyForReviewEvent struct {
	Actor     Actor
	CreatedAt time.Time
}

// Key is a unique key identifying this event in the context of its pull request.
func (e ReadyForReviewEvent) Key() string {
	return fmt.Sprintf("%s:%d", e.Actor.Login, e.CreatedAt.UnixNano())
}

// ConvertToDraftEvent represents a 'convert_to_draft' event on a
// pull request.
type ConvertToDraftEvent struct {
	Actor     Actor
	CreatedAt time.Time
}

// Key is a unique key identifying this event in the context of its pull request.
func (e ConvertToDraftEvent) Key() string {
	return fmt.Sprintf("%s:%d", e.Actor.Login, e.CreatedAt.UnixNano())
}

// UnassignedEvent represents an 'unassigned' event on a pull request.
type UnassignedEvent struct {
	Actor     Actor
	Assignee  Actor
	CreatedAt time.Time
}

// Key is a unique key identifying this event in the context of its pull request.
func (e UnassignedEvent) Key() string {
	return fmt.Sprintf("%s:%s:%d", e.Actor.Login, e.Assignee.Login, e.CreatedAt.UnixNano())
}

// LabelEvent represents a label being added or removed from a pull request
type LabelEvent struct {
	Actor     Actor
	Label     Label
	CreatedAt time.Time
	// Will be true if we had an "unlabeled" event
	Removed bool
}

func (e LabelEvent) Key() string {
	action := "add"
	if e.Removed {
		action = "delete"
	}
	return fmt.Sprintf("%s:%s:%d", e.Label.ID, action, e.CreatedAt.UnixNano())
}

type TimelineItemConnection struct {
	PageInfo PageInfo
	Nodes    []TimelineItem
}

// TimelineItem is a union type of all supported pull request timeline items.
type TimelineItem struct {
	Type string
	Item interface{}
}

// UnmarshalJSON knows how to unmarshal a TimelineItem as produced
// by json.Marshal or as returned by the GitHub GraphQL API.
func (i *TimelineItem) UnmarshalJSON(data []byte) error {
	v := struct {
		Typename *string `json:"__typename"`
		Type     *string
		Item     json.RawMessage
	}{
		Typename: &i.Type,
		Type:     &i.Type,
	}

	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	switch i.Type {
	case "AssignedEvent":
		i.Item = new(AssignedEvent)
	case "ClosedEvent":
		i.Item = new(ClosedEvent)
	case "IssueComment":
		i.Item = new(IssueComment)
	case "RenamedTitleEvent":
		i.Item = new(RenamedTitleEvent)
	case "MergedEvent":
		i.Item = new(MergedEvent)
	case "PullRequestReview":
		i.Item = new(PullRequestReview)
	case "PullRequestReviewComment":
		i.Item = new(PullRequestReviewComment)
	case "PullRequestReviewThread":
		i.Item = new(PullRequestReviewThread)
	case "PullRequestCommit":
		i.Item = new(PullRequestCommit)
	case "ReopenedEvent":
		i.Item = new(ReopenedEvent)
	case "ReviewDismissedEvent":
		i.Item = new(ReviewDismissedEvent)
	case "ReviewRequestRemovedEvent":
		i.Item = new(ReviewRequestRemovedEvent)
	case "ReviewRequestedEvent":
		i.Item = new(ReviewRequestedEvent)
	case "ReadyForReviewEvent":
		i.Item = new(ReadyForReviewEvent)
	case "ConvertToDraftEvent":
		i.Item = new(ConvertToDraftEvent)
	case "UnassignedEvent":
		i.Item = new(UnassignedEvent)
	case "LabeledEvent":
		i.Item = new(LabelEvent)
	case "UnlabeledEvent":
		i.Item = &LabelEvent{Removed: true}
	default:
		return errors.Errorf("unknown timeline item type %q", i.Type)
	}

	if len(v.Item) > 0 {
		data = v.Item
	}

	return json.Unmarshal(data, i.Item)
}

type CreatePullRequestInput struct {
	// The Node ID of the repository.
	RepositoryID string `json:"repositoryId"`
	// The name of the branch you want your changes pulled into. This should be
	// an existing branch on the current repository.
	BaseRefName string `json:"baseRefName"`
	// The name of the branch where your changes are implemented.
	HeadRefName string `json:"headRefName"`
	// The title of the pull request.
	Title string `json:"title"`
	// The body of the pull request (optional).
	Body string `json:"body"`
	// When true the PR will be in draft mode initially.
	Draft bool `json:"draft"`
}

// CreatePullRequest creates a PullRequest on Github.
func (c *V4Client) CreatePullRequest(ctx context.Context, in *CreatePullRequestInput) (*PullRequest, error) {
	version := c.determineGitHubVersion(ctx)

	prFragment, err := pullRequestFragments(version)
	if err != nil {
		return nil, err
	}
	var q strings.Builder
	q.WriteString(prFragment)
	q.WriteString(`mutation	CreatePullRequest($input:CreatePullRequestInput!) {
  createPullRequest(input:$input) {
    pullRequest {
      ... pr
    }
  }
}`)

	var result struct {
		CreatePullRequest struct {
			PullRequest struct {
				PullRequest
				Participants  struct{ Nodes []Actor }
				TimelineItems TimelineItemConnection
			} `json:"pullRequest"`
		} `json:"createPullRequest"`
	}

	compatibleInput := map[string]interface{}{
		"repositoryId": in.RepositoryID,
		"baseRefName":  in.BaseRefName,
		"headRefName":  in.HeadRefName,
		"title":        in.Title,
		"body":         in.Body,
	}

	if ghe221PlusOrDotComSemver.Check(version) {
		compatibleInput["draft"] = in.Draft
	} else if in.Draft {
		return nil, errors.New("draft PRs not supported by this version of GitHub enterprise. GitHub Enterprise v3.21 is the first version to support draft PRs.\nPotential fix: set `published: true` in your batch spec.")
	}

	input := map[string]interface{}{"input": compatibleInput}
	err = c.requestGraphQL(ctx, q.String(), input, &result)
	if err != nil {
		if gqlErrs, ok := err.(graphqlErrors); ok && len(gqlErrs) == 1 {
			e := gqlErrs[0]
			if strings.Contains(e.Message, "A pull request already exists for") {
				return nil, ErrPullRequestAlreadyExists
			}
		}
		return nil, err
	}

	ti := result.CreatePullRequest.PullRequest.TimelineItems
	pr := &result.CreatePullRequest.PullRequest.PullRequest
	pr.TimelineItems = ti.Nodes
	pr.Participants = result.CreatePullRequest.PullRequest.Participants.Nodes

	items, err := c.loadRemainingTimelineItems(ctx, pr.ID, ti.PageInfo)
	if err != nil {
		return nil, err
	}
	pr.TimelineItems = append(pr.TimelineItems, items...)

	return pr, nil
}

type UpdatePullRequestInput struct {
	// The Node ID of the pull request.
	PullRequestID string `json:"pullRequestId"`
	// The name of the branch you want your changes pulled into. This should be
	// an existing branch on the current repository.
	BaseRefName string `json:"baseRefName"`
	// The title of the pull request.
	Title string `json:"title"`
	// The body of the pull request (optional).
	Body string `json:"body"`
}

// UpdatePullRequest creates a PullRequest on Github.
func (c *V4Client) UpdatePullRequest(ctx context.Context, in *UpdatePullRequestInput) (*PullRequest, error) {
	version := c.determineGitHubVersion(ctx)
	prFragment, err := pullRequestFragments(version)
	if err != nil {
		return nil, err
	}
	var q strings.Builder
	q.WriteString(prFragment)
	q.WriteString(`mutation	UpdatePullRequest($input:UpdatePullRequestInput!) {
  updatePullRequest(input:$input) {
    pullRequest {
      ... pr
    }
  }
}`)

	var result struct {
		UpdatePullRequest struct {
			PullRequest struct {
				PullRequest
				Participants  struct{ Nodes []Actor }
				TimelineItems TimelineItemConnection
			} `json:"pullRequest"`
		} `json:"updatePullRequest"`
	}

	input := map[string]interface{}{"input": in}
	err = c.requestGraphQL(ctx, q.String(), input, &result)
	if err != nil {
		if gqlErrs, ok := err.(graphqlErrors); ok && len(gqlErrs) == 1 {
			e := gqlErrs[0]
			if strings.Contains(e.Message, "A pull request already exists for") {
				return nil, ErrPullRequestAlreadyExists
			}
		}
		return nil, err
	}

	ti := result.UpdatePullRequest.PullRequest.TimelineItems
	pr := &result.UpdatePullRequest.PullRequest.PullRequest
	pr.TimelineItems = ti.Nodes
	pr.Participants = result.UpdatePullRequest.PullRequest.Participants.Nodes

	items, err := c.loadRemainingTimelineItems(ctx, pr.ID, ti.PageInfo)
	if err != nil {
		return nil, err
	}
	pr.TimelineItems = append(pr.TimelineItems, items...)

	return pr, nil
}

// MarkPullRequestReadyForReview marks the PullRequest on Github as ready for review.
func (c *V4Client) MarkPullRequestReadyForReview(ctx context.Context, pr *PullRequest) error {
	version := c.determineGitHubVersion(ctx)
	prFragment, err := pullRequestFragments(version)
	if err != nil {
		return err
	}
	var q strings.Builder
	q.WriteString(prFragment)
	q.WriteString(`mutation	MarkPullRequestReadyForReview($input:MarkPullRequestReadyForReviewInput!) {
  markPullRequestReadyForReview(input:$input) {
    pullRequest {
      ... pr
    }
  }
}`)

	var result struct {
		MarkPullRequestReadyForReview struct {
			PullRequest struct {
				PullRequest
				Participants  struct{ Nodes []Actor }
				TimelineItems TimelineItemConnection
			} `json:"pullRequest"`
		} `json:"markPullRequestReadyForReview"`
	}

	input := map[string]interface{}{"input": struct {
		ID string `json:"pullRequestId"`
	}{ID: pr.ID}}
	err = c.requestGraphQL(ctx, q.String(), input, &result)
	if err != nil {
		return err
	}

	ti := result.MarkPullRequestReadyForReview.PullRequest.TimelineItems
	*pr = result.MarkPullRequestReadyForReview.PullRequest.PullRequest
	pr.TimelineItems = ti.Nodes
	pr.Participants = result.MarkPullRequestReadyForReview.PullRequest.Participants.Nodes

	items, err := c.loadRemainingTimelineItems(ctx, pr.ID, ti.PageInfo)
	if err != nil {
		return err
	}
	pr.TimelineItems = append(pr.TimelineItems, items...)

	return nil
}

// ClosePullRequest closes the PullRequest on Github.
func (c *V4Client) ClosePullRequest(ctx context.Context, pr *PullRequest) error {
	version := c.determineGitHubVersion(ctx)
	prFragment, err := pullRequestFragments(version)
	if err != nil {
		return err
	}
	var q strings.Builder
	q.WriteString(prFragment)
	q.WriteString(`mutation	ClosePullRequest($input:ClosePullRequestInput!) {
  closePullRequest(input:$input) {
    pullRequest {
      ... pr
    }
  }
}`)

	var result struct {
		ClosePullRequest struct {
			PullRequest struct {
				PullRequest
				Participants  struct{ Nodes []Actor }
				TimelineItems TimelineItemConnection
			} `json:"pullRequest"`
		} `json:"closePullRequest"`
	}

	input := map[string]interface{}{"input": struct {
		ID string `json:"pullRequestId"`
	}{ID: pr.ID}}
	err = c.requestGraphQL(ctx, q.String(), input, &result)
	if err != nil {
		return err
	}

	ti := result.ClosePullRequest.PullRequest.TimelineItems
	*pr = result.ClosePullRequest.PullRequest.PullRequest
	pr.TimelineItems = ti.Nodes
	pr.Participants = result.ClosePullRequest.PullRequest.Participants.Nodes

	items, err := c.loadRemainingTimelineItems(ctx, pr.ID, ti.PageInfo)
	if err != nil {
		return err
	}
	pr.TimelineItems = append(pr.TimelineItems, items...)

	return nil
}

// ReopenPullRequest reopens the PullRequest on Github.
func (c *V4Client) ReopenPullRequest(ctx context.Context, pr *PullRequest) error {
	version := c.determineGitHubVersion(ctx)
	prFragment, err := pullRequestFragments(version)
	if err != nil {
		return err
	}
	var q strings.Builder
	q.WriteString(prFragment)
	q.WriteString(`mutation	ReopenPullRequest($input:ReopenPullRequestInput!) {
  reopenPullRequest(input:$input) {
    pullRequest {
      ... pr
    }
  }
}`)

	var result struct {
		ReopenPullRequest struct {
			PullRequest struct {
				PullRequest
				Participants  struct{ Nodes []Actor }
				TimelineItems TimelineItemConnection
			} `json:"pullRequest"`
		} `json:"reopenPullRequest"`
	}

	input := map[string]interface{}{"input": struct {
		ID string `json:"pullRequestId"`
	}{ID: pr.ID}}
	err = c.requestGraphQL(ctx, q.String(), input, &result)
	if err != nil {
		return err
	}

	ti := result.ReopenPullRequest.PullRequest.TimelineItems
	*pr = result.ReopenPullRequest.PullRequest.PullRequest
	pr.TimelineItems = ti.Nodes
	pr.Participants = result.ReopenPullRequest.PullRequest.Participants.Nodes

	items, err := c.loadRemainingTimelineItems(ctx, pr.ID, ti.PageInfo)
	if err != nil {
		return err
	}
	pr.TimelineItems = append(pr.TimelineItems, items...)

	return nil
}

// LoadPullRequest loads a PullRequest from Github.
func (c *V4Client) LoadPullRequest(ctx context.Context, pr *PullRequest) error {
	owner, repo, err := SplitRepositoryNameWithOwner(pr.RepoWithOwner)
	if err != nil {
		return err
	}

	version := c.determineGitHubVersion(ctx)

	prFragment, err := pullRequestFragments(version)
	if err != nil {
		return err
	}

	q := prFragment + `
query($owner: String!, $name: String!, $number: Int!) {
	repository(owner: $owner, name: $name) {
		pullRequest(number: $number) { ...pr }
	}
}`

	var result struct {
		Repository struct {
			PullRequest struct {
				PullRequest
				Participants  struct{ Nodes []Actor }
				TimelineItems TimelineItemConnection
			}
		}
	}

	err = c.requestGraphQL(ctx, q, map[string]interface{}{"owner": owner, "name": repo, "number": pr.Number}, &result)
	if err != nil {
		if gqlErrs, ok := err.(graphqlErrors); ok {
			for _, err2 := range gqlErrs {
				if err2.Type == graphqlErrTypeNotFound && len(err2.Path) >= 1 {
					if repoPath, ok := err2.Path[0].(string); !ok || repoPath != "repository" {
						continue
					}
					if len(err2.Path) == 1 {
						return ErrRepoNotFound
					}
					if prPath, ok := err2.Path[1].(string); !ok || prPath != "pullRequest" {
						continue
					}
					return ErrPullRequestNotFound(pr.Number)
				}
			}
		}
		return err
	}

	ti := result.Repository.PullRequest.TimelineItems
	*pr = result.Repository.PullRequest.PullRequest
	pr.TimelineItems = ti.Nodes
	pr.Participants = result.Repository.PullRequest.Participants.Nodes

	items, err := c.loadRemainingTimelineItems(ctx, pr.ID, ti.PageInfo)
	if err != nil {
		return err
	}
	pr.TimelineItems = append(pr.TimelineItems, items...)

	return nil
}

// GetOpenPullRequestByRefs fetches the the pull request associated with the supplied
// refs. GitHub only allows one open PR by ref at a time.
// If nothing is found an error is returned.
func (c *V4Client) GetOpenPullRequestByRefs(ctx context.Context, owner, name, baseRef, headRef string) (*PullRequest, error) {
	version := c.determineGitHubVersion(ctx)
	prFragment, err := pullRequestFragments(version)
	if err != nil {
		return nil, err
	}
	var q strings.Builder
	q.WriteString(prFragment)
	q.WriteString("query {\n")
	q.WriteString(fmt.Sprintf("repository(owner: %q, name: %q) {\n",
		owner, name))
	q.WriteString(fmt.Sprintf("pullRequests(baseRefName: %q, headRefName: %q, first: 1, states: OPEN) { \n",
		abbreviateRef(baseRef), abbreviateRef(headRef),
	))
	q.WriteString("nodes{ ... pr }\n}\n}\n}")

	var results struct {
		Repository struct {
			PullRequests struct {
				Nodes []*struct {
					PullRequest
					Participants  struct{ Nodes []Actor }
					TimelineItems TimelineItemConnection
				}
			}
		}
	}

	err = c.requestGraphQL(ctx, q.String(), nil, &results)
	if err != nil {
		return nil, err
	}
	if len(results.Repository.PullRequests.Nodes) != 1 {
		return nil, fmt.Errorf("expected 1 pull request, got %d instead", len(results.Repository.PullRequests.Nodes))
	}

	node := results.Repository.PullRequests.Nodes[0]
	pr := node.PullRequest
	pr.Participants = node.Participants.Nodes
	pr.TimelineItems = node.TimelineItems.Nodes

	items, err := c.loadRemainingTimelineItems(ctx, pr.ID, node.TimelineItems.PageInfo)
	if err != nil {
		return nil, err
	}
	pr.TimelineItems = append(pr.TimelineItems, items...)

	return &pr, nil
}

func (c *V4Client) loadRemainingTimelineItems(ctx context.Context, prID string, pageInfo PageInfo) (items []TimelineItem, err error) {
	version := c.determineGitHubVersion(ctx)
	timelineItemTypes, err := timelineItemTypes(version)
	if err != nil {
		return nil, err
	}
	timelineItemsFragment, err := timelineItemsFragment(version)
	if err != nil {
		return nil, err
	}
	pi := pageInfo
	for pi.HasNextPage {
		var q strings.Builder
		q.WriteString(prCommonFragments)
		q.WriteString(timelineItemsFragment)
		q.WriteString(fmt.Sprintf(`query {
  node(id: %q) {
    ... on PullRequest {
      __typename
      timelineItems(first: 250, after: %q, itemTypes: [`+timelineItemTypes+`]) {
        pageInfo {
          hasNextPage
          endCursor
        }
        nodes {
          __typename
          ...timelineItems
        }
      }
    }
  }
}
`, prID, pi.EndCursor))

		var results struct {
			Node struct {
				TypeName      string `json:"__typename"`
				TimelineItems TimelineItemConnection
			}
		}

		err = c.requestGraphQL(ctx, q.String(), nil, &results)
		if err != nil {
			return
		}

		if results.Node.TypeName != "PullRequest" {
			return nil, fmt.Errorf("invalid node type received, want PullRequest, got %s", results.Node.TypeName)
		}

		items = append(items, results.Node.TimelineItems.Nodes...)
		if !results.Node.TimelineItems.PageInfo.HasNextPage {
			break
		}
		pi = results.Node.TimelineItems.PageInfo
	}
	return
}

// abbreviateRef removes the "refs/heads/" prefix from a given ref. If the ref
// doesn't have the prefix, it returns it unchanged.
//
// Copied from internal/vcs/git to avoid a cyclic import
func abbreviateRef(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

// timelineItemTypes contains all the types requested via GraphQL from the timelineItems connection on a pull request.
const timelineItemTypesFmtStr = `ASSIGNED_EVENT, CLOSED_EVENT, ISSUE_COMMENT, RENAMED_TITLE_EVENT, MERGED_EVENT, PULL_REQUEST_REVIEW, PULL_REQUEST_REVIEW_THREAD, REOPENED_EVENT, REVIEW_DISMISSED_EVENT, REVIEW_REQUEST_REMOVED_EVENT, REVIEW_REQUESTED_EVENT, UNASSIGNED_EVENT, LABELED_EVENT, UNLABELED_EVENT, PULL_REQUEST_COMMIT, READY_FOR_REVIEW_EVENT`

var ghe220Semver, _ = semver.NewConstraint("~2.20.0")
var ghe221PlusOrDotComSemver, _ = semver.NewConstraint(">= 2.21.0")

func timelineItemTypes(version *semver.Version) (string, error) {
	if ghe220Semver.Check(version) {
		return timelineItemTypesFmtStr, nil
	}
	if ghe221PlusOrDotComSemver.Check(version) {
		return timelineItemTypesFmtStr + `, CONVERT_TO_DRAFT_EVENT`, nil
	}
	return "", fmt.Errorf("unsupported version of GitHub: %s", version)
}

// This fragment was formatted using the "prettify" button in the GitHub API explorer:
// https://developer.github.com/v4/explorer/
const prCommonFragments = `
fragment actor on Actor {
  avatarUrl
  login
  url
}

fragment label on Label {
  name
  color
  description
  id
}
`

// This fragment was formatted using the "prettify" button in the GitHub API explorer:
// https://developer.github.com/v4/explorer/
const timelineItemsFragmentFmtstr = `
fragment commit on Commit {
  oid
  message
  messageHeadline
  committedDate
  pushedDate
  url
  committer {
    avatarUrl
    email
    name
    user {
      ...actor
    }
  }
}

fragment review on PullRequestReview {
  databaseId
  author {
    ...actor
  }
  authorAssociation
  body
  state
  url
  createdAt
  updatedAt
  commit {
    ...commit
  }
  includesCreatedEdit
}

fragment timelineItems on PullRequestTimelineItems {
  ... on AssignedEvent {
    actor {
      ...actor
    }
    assignee {
      ...actor
    }
    createdAt
  }
  ... on ClosedEvent {
    actor {
      ...actor
    }
    createdAt
    url
  }
  ... on IssueComment {
    databaseId
    author {
      ...actor
    }
    authorAssociation
    body
    createdAt
    editor {
      ...actor
    }
    url
    updatedAt
    includesCreatedEdit
    publishedAt
  }
  ... on RenamedTitleEvent {
    actor {
      ...actor
    }
    previousTitle
    currentTitle
    createdAt
  }
  ... on MergedEvent {
    actor {
      ...actor
    }
    mergeRefName
    url
    commit {
      ...commit
    }
    createdAt
  }
  ... on PullRequestReview {
    ...review
  }
  ... on PullRequestReviewThread {
    comments(last: 100) {
      nodes {
        databaseId
        author {
          ...actor
        }
        authorAssociation
        editor {
          ...actor
        }
        commit {
          ...commit
        }
        body
        state
        url
        createdAt
        updatedAt
        includesCreatedEdit
      }
    }
  }
  ... on ReopenedEvent {
    actor {
      ...actor
    }
    createdAt
  }
  ... on ReviewDismissedEvent {
    actor {
      ...actor
    }
    review {
      ...review
    }
    dismissalMessage
    createdAt
  }
  ... on ReviewRequestRemovedEvent {
    actor {
      ...actor
    }
    requestedReviewer {
      ...actor
    }
    requestedTeam: requestedReviewer {
      ... on Team {
        name
        url
        avatarUrl
      }
    }
    createdAt
  }
  ... on ReviewRequestedEvent {
    actor {
      ...actor
    }
    requestedReviewer {
      ...actor
    }
    requestedTeam: requestedReviewer {
      ... on Team {
        name
        url
        avatarUrl
      }
    }
    createdAt
  }
  ... on ReadyForReviewEvent {
    actor {
      ...actor
    }
    createdAt
  }
  ... on UnassignedEvent {
    actor {
      ...actor
    }
    assignee {
      ...actor
    }
    createdAt
  }
  ... on LabeledEvent {
    actor {
      ...actor
    }
    label {
      ...label
    }
    createdAt
  }
  ... on UnlabeledEvent {
    actor {
      ...actor
    }
    label {
      ...label
    }
    createdAt
  }
  ... on PullRequestCommit {
    commit {
      ...commit
    }
  }
  %s
}
`

const convertToDraftEventFmtstr = `
  ... on ConvertToDraftEvent {
    actor {
	  ...actor
	}
	createdAt
  }
`

func timelineItemsFragment(version *semver.Version) (string, error) {
	if ghe220Semver.Check(version) {
		// GHE 2.20 doesn't know about the ConvertToDraftEvent type.
		return fmt.Sprintf(timelineItemsFragmentFmtstr, ""), nil
	}
	if ghe221PlusOrDotComSemver.Check(version) {
		return fmt.Sprintf(timelineItemsFragmentFmtstr, convertToDraftEventFmtstr), nil
	}
	return "", fmt.Errorf("unsupported version of GitHub: %s", version)
}

// This fragment was formatted using the "prettify" button in the GitHub API explorer:
// https://developer.github.com/v4/explorer/
const pullRequestFragmentsFmtstr = prCommonFragments + `
fragment commitWithChecks on Commit {
  oid
  status {
    state
    contexts {
      id
      context
      state
      description
    }
  }
  checkSuites(last: 20) {
    nodes {
      id
      status
      conclusion
      checkRuns(last: 20) {
        nodes {
          id
          status
          conclusion
        }
      }
    }
  }
  committedDate
}

fragment prCommit on PullRequestCommit {
  commit {
    ...commitWithChecks
  }
}

fragment pr on PullRequest {
  id
  title
  body
  state
  url
  number
  createdAt
  updatedAt
  headRefOid
  baseRefOid
  headRefName
  baseRefName
  %s
  author {
    ...actor
  }
  participants(first: 100) {
    nodes {
      ...actor
    }
  }
  labels(first: 100) {
    nodes {
      ...label
    }
  }
  commits(last: 1) {
    nodes {
      ...prCommit
    }
  }
  timelineItems(first: 250, itemTypes: [%s]) {
    pageInfo {
      hasNextPage
      endCursor
    }
    nodes {
      __typename
      ...timelineItems
    }
  }
}
`

func pullRequestFragments(version *semver.Version) (string, error) {
	timelineItemTypes, err := timelineItemTypes(version)
	if err != nil {
		return "", err
	}
	timelineItemsFragment, err := timelineItemsFragment(version)
	if err != nil {
		return "", err
	}
	if ghe220Semver.Check(version) {
		// Don't ask for isDraft for ghe 2.20.
		return fmt.Sprintf(timelineItemsFragment+pullRequestFragmentsFmtstr, "", timelineItemTypes), nil
	}
	if ghe221PlusOrDotComSemver.Check(version) {
		return fmt.Sprintf(timelineItemsFragment+pullRequestFragmentsFmtstr, "isDraft", timelineItemTypes), nil
	}
	return "", fmt.Errorf("unsupported version of GitHub: %s", version)
}
