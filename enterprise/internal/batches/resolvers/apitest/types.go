package apitest

import (
	"github.com/sourcegraph/go-diff/diff"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/batches"
)

type GitTarget struct {
	OID            string
	AbbreviatedOID string
	TargetType     string `json:"type"`
}

type DiffRange struct{ StartLine, Lines int }

type DiffStat struct{ Added, Deleted, Changed int32 }

func (ds DiffStat) ToDiffStat() *diff.Stat {
	return &diff.Stat{Added: ds.Added, Deleted: ds.Deleted, Changed: ds.Changed}
}

type FileDiffHunk struct {
	Body, Section      string
	OldNoNewlineAt     bool
	OldRange, NewRange DiffRange
}

type File struct {
	Name string
	// Ignoring other fields of File2, since that would require gitserver
}

type FileDiff struct {
	OldPath, NewPath string
	Hunks            []FileDiffHunk
	Stat             DiffStat
	OldFile          File
}

type FileDiffs struct {
	RawDiff    string
	DiffStat   DiffStat
	PageInfo   PageInfo
	Nodes      []FileDiff
	TotalCount int
}

type User struct {
	ID         string
	DatabaseID int32
	SiteAdmin  bool

	BatchChanges          BatchChangeConnection
	BatchChangesCodeHosts BatchChangesCodeHostsConnection
}

type Org struct {
	ID   string
	Name string

	BatchChanges BatchChangeConnection
}

type UserOrg struct {
	ID         string
	DatabaseID int32
	SiteAdmin  bool
	Name       string
}

type BatchChange struct {
	ID                      string
	Name                    string
	Description             string
	SpecCreator             *User
	InitialApplier          *User
	LastApplier             *User
	LastAppliedAt           string
	ViewerCanAdminister     bool
	Namespace               UserOrg
	CreatedAt               string
	UpdatedAt               string
	ClosedAt                string
	URL                     string
	ChangesetsStats         ChangesetsStats
	Changesets              ChangesetConnection
	ChangesetCountsOverTime []ChangesetCounts
	DiffStat                DiffStat
}

type BatchChangeConnection struct {
	Nodes      []BatchChange
	TotalCount int
	PageInfo   PageInfo
}

type ChangesetEvent struct {
	ID        string
	Changeset struct{ ID string }
	CreatedAt string
}

type ChangesetEventConnection struct {
	TotalCount int
	PageInfo   PageInfo
	Nodes      []ChangesetEvent
}

type Repository struct {
	ID   string
	Name string
}

type ExternalURL struct {
	URL         string
	ServiceKind string
	ServiceType string
}

type Changeset struct {
	Typename     string `json:"__typename"`
	ID           string
	Repository   Repository
	BatchChanges BatchChangeConnection
	CreatedAt    string
	UpdatedAt    string
	NextSyncAt   string
	Title        string
	Body         string
	Error        string
	State        string
	ExternalID   string
	ExternalURL  ExternalURL
	ReviewState  string
	CheckState   string
	Events       ChangesetEventConnection

	Diff Comparison

	Labels []Label

	CurrentSpec ChangesetSpec
}

type Comparison struct {
	Typename  string `json:"__typename"`
	FileDiffs FileDiffs
}

type Label struct {
	Text        string
	Color       string
	Description *string
}

type ChangesetConnection struct {
	Nodes      []Changeset
	TotalCount int
	PageInfo   PageInfo
}

type ChangesetsStats struct {
	Unpublished int
	Draft       int
	Open        int
	Merged      int
	Closed      int
	Deleted     int
	Total       int
}

type ChangesetCounts struct {
	Date                 string
	Total                int32
	Merged               int32
	Closed               int32
	Open                 int32
	Draft                int32
	OpenApproved         int32
	OpenChangesRequested int32
	OpenPending          int32
}

type BatchSpec struct {
	Typename string `json:"__typename"`
	ID       string

	OriginalInput string
	ParsedInput   graphqlbackend.JSONValue

	ApplyURL string

	Namespace UserOrg
	Creator   *User

	ChangesetSpecs ChangesetSpecConnection
	ApplyPreview   ChangesetApplyPreviewConnection

	ViewerCanAdminister bool

	DiffStat DiffStat

	// Alias for the above.
	AllCodeHosts BatchChangesCodeHostsConnection
	// Alias for the above.
	OnlyWithoutCredential BatchChangesCodeHostsConnection

	CreatedAt graphqlbackend.DateTime
	ExpiresAt *graphqlbackend.DateTime

	// NEW
	SupersedingBatchSpec *BatchSpec
	AppliesToBatchChange BatchChange
}

// ChangesetSpecDelta is the delta between two ChangesetSpecs describing the same Changeset.
type ChangesetSpecDelta struct {
	TitleChanged         bool
	BodyChanged          bool
	Undraft              bool
	BaseRefChanged       bool
	DiffChanged          bool
	CommitMessageChanged bool
	AuthorNameChanged    bool
	AuthorEmailChanged   bool
}

type ChangesetSpec struct {
	Typename string `json:"__typename"`
	ID       string

	Description ChangesetSpecDescription

	ExpiresAt *graphqlbackend.DateTime
}

type ChangesetSpecConnection struct {
	Nodes      []ChangesetSpec
	TotalCount int
	PageInfo   PageInfo
}

type ChangesetApplyPreviewConnection struct {
	Nodes      []ChangesetApplyPreview
	TotalCount int
	PageInfo   PageInfo
	Stats      ChangesetApplyPreviewConnectionStats
}

type ChangesetApplyPreviewConnectionStats struct {
	Push         int32
	Update       int32
	Undraft      int32
	Publish      int32
	PublishDraft int32
	Sync         int32
	Import       int32
	Close        int32
	Reopen       int32
	Sleep        int32
	Detach       int32
}

type ChangesetApplyPreview struct {
	Typename string `json:"__typename"`

	Operations []batches.ReconcilerOperation
	Delta      ChangesetSpecDelta
	Targets    ChangesetApplyPreviewTargets
}

type ChangesetApplyPreviewTargets struct {
	Typename string `json:"__typename"`

	ChangesetSpec ChangesetSpec
	Changeset     Changeset
}

type ChangesetSpecDescription struct {
	Typename string `json:"__typename"`

	BaseRepository Repository
	ExternalID     string
	BaseRef        string

	HeadRepository Repository
	HeadRef        string

	Title string
	Body  string

	Commits []GitCommitDescription

	Published batches.PublishedValue

	Diff struct {
		FileDiffs FileDiffs
	}
	DiffStat DiffStat
}

type GitCommitDescription struct {
	Author  Person
	Message string
	Subject string
	Body    string
	Diff    string
}

type PageInfo struct {
	HasNextPage bool
	EndCursor   *string
}

type Person struct {
	Name  string
	Email string
	User  *User
}

type BatchChangesCredential struct {
	ID                  string
	ExternalServiceKind string
	ExternalServiceURL  string
	CreatedAt           string
}

type EmptyResponse struct {
	AlwaysNil string
}

type BatchChangesCodeHostsConnection struct {
	PageInfo   PageInfo
	Nodes      []BatchChangesCodeHost
	TotalCount int
}

type BatchChangesCodeHost struct {
	ExternalServiceKind string
	ExternalServiceURL  string
	Credential          BatchChangesCredential
}
