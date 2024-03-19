// Package types defines types used by the frontend.
package types

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"

	"github.com/google/uuid"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	rtypes "github.com/sourcegraph/sourcegraph/internal/rbac/types"
)

// BatchChangeSource represents how a batch change can be created
// it can either be created locally or via an executor (SSBC)
type BatchChangeSource string

const (
	ExecutorBatchChangeSource BatchChangeSource = "executor"
	LocalBatchChangeSource    BatchChangeSource = "local"
)

// A SourceInfo represents a source a Repo belongs to (such as an external service).
type SourceInfo struct {
	ID       string
	CloneURL string
}

// ExternalServiceID returns the ID of the external service this
// SourceInfo refers to.
func (i SourceInfo) ExternalServiceID() int64 {
	_, id := extsvc.DecodeURN(i.ID)
	return id
}

// Repo represents a source code repository.
type Repo struct {
	// ID is the unique numeric ID for this repository.
	ID api.RepoID
	// Name is the name for this repository (e.g., "github.com/user/repo"). It
	// is the same as URI, unless the user configures a non-default
	// repositoryPathPattern.
	//
	// Previously, this was called RepoURI.
	Name api.RepoName
	// URI is the full name for this repository (e.g.,
	// "github.com/user/repo"). See the documentation for the Name field.
	URI string
	// Description is a brief description of the repository.
	Description string
	// Fork is whether this repository is a fork of another repository.
	Fork bool
	// Archived is whether the repository has been archived.
	Archived bool
	// Stars is the star count the repository has in the code host.
	Stars int `json:",omitempty"`
	// Private is whether the repository is private.
	Private bool
	// CreatedAt is when this repository was created on Sourcegraph.
	CreatedAt time.Time
	// UpdatedAt is when this repository's metadata was last updated on Sourcegraph.
	UpdatedAt time.Time
	// DeletedAt is when this repository was soft-deleted from Sourcegraph.
	DeletedAt time.Time
	// ExternalRepo identifies this repository by its ID on the external service where it resides (and the external
	// service itself).
	ExternalRepo api.ExternalRepoSpec
	// Sources identifies all the repo sources this Repo belongs to.
	// The key is a URN created by extsvc.URN
	Sources map[string]*SourceInfo
	// Metadata contains the raw source code host JSON metadata.
	Metadata any
	// Blocked contains the reason this repository was blocked and the timestamp of when it happened.
	Blocked *RepoBlock `json:",omitempty"`
	// KeyValuePairs is the set of key-value pairs associated with the repo
	KeyValuePairs map[string]*string `json:",omitempty"`
	// Topics synced from GitHub or GitLab
	Topics []string `json:",omitempty"`
}

func (r *Repo) IDName() RepoIDName {
	return RepoIDName{
		ID:   r.ID,
		Name: r.Name,
	}
}

type GitHubAppDomain string

func (s GitHubAppDomain) ToGraphQL() string { return strings.ToUpper(string(s)) }

const (
	ReposGitHubAppDomain   GitHubAppDomain = "repos"
	BatchesGitHubAppDomain GitHubAppDomain = "batches"
)

// RepoCommit is a record of a repo and a corresponding commit.
type RepoCommit struct {
	ID                   int64
	RepoID               api.RepoID
	CommitSHA            dbutil.CommitBytea
	PerforceChangelistID int64
	CreatedAt            time.Time
}

// SearchedRepo is a collection of metadata about repos that is used to decorate search results
type SearchedRepo struct {
	// ID is the unique numeric ID for this repository.
	ID api.RepoID
	// Name is the name for this repository (e.g., "github.com/user/repo"). It
	// is the same as URI, unless the user configures a non-default
	// repositoryPathPattern.
	Name api.RepoName
	// Description is a brief description of the repository.
	Description string
	// Fork is whether this repository is a fork of another repository.
	Fork bool
	// Archived is whether the repository has been archived.
	Archived bool
	// Private is whether the repository is private.
	Private bool
	// Stars is the star count the repository has in the code host.
	Stars int
	// LastFetched is the time of the last fetch of new commits from the code host.
	LastFetched *time.Time
	// A set of key-value pairs associated with the repo
	KeyValuePairs map[string]*string
	// Topics synced from GitHub or GitLab
	Topics []string
}

// RepoBlock contains data about a repo that has been blocked. Blocked repos aren't returned by store methods by default.
type RepoBlock struct {
	At     int64 // Unix timestamp
	Reason string
}

// CloneURLs returns all the clone URLs this repo is cloneable from.
func (r *Repo) CloneURLs() []string {
	urls := make([]string, 0, len(r.Sources))
	for _, src := range r.Sources {
		if src != nil && src.CloneURL != "" {
			urls = append(urls, src.CloneURL)
		}
	}
	return urls
}

// IsDeleted returns true if the repo is deleted.
func (r *Repo) IsDeleted() bool { return !r.DeletedAt.IsZero() }

// ExternalServiceIDs returns the IDs of the external services this
// repo belongs to.
func (r *Repo) ExternalServiceIDs() []int64 {
	ids := make([]int64, 0, len(r.Sources))
	for _, src := range r.Sources {
		ids = append(ids, src.ExternalServiceID())
	}
	return ids
}

func (r *Repo) ToExternalServiceRepository() *ExternalServiceRepository {
	return &ExternalServiceRepository{
		ID:         r.ID,
		Name:       r.Name,
		ExternalID: r.ExternalRepo.ID,
	}
}

// BlockedRepoError is returned by a Repo IsBlocked method.
type BlockedRepoError struct {
	Name   api.RepoName
	Reason string
}

func (e BlockedRepoError) Error() string {
	return fmt.Sprintf("repository %s has been blocked. reason: %s", e.Name, e.Reason)
}

// Blocked implements the blocker interface in the errcode package.
func (e BlockedRepoError) Blocked() bool { return true }

// IsBlocked returns a non nil error if the repo has been blocked.
func (r *Repo) IsBlocked() error {
	if r.Blocked != nil {
		return &BlockedRepoError{Name: r.Name, Reason: r.Blocked.Reason}
	}
	return nil
}

// RepoModifiedFields is a bitfield that tracks which fields were modified while
// syncing a repository.
type RepoModifiedFields uint64

const (
	RepoUnmodified   RepoModifiedFields = 0
	RepoModifiedName                    = 1 << iota
	RepoModifiedURI
	RepoModifiedDescription
	RepoModifiedExternalRepo
	RepoModifiedArchived
	RepoModifiedFork
	RepoModifiedPrivate
	RepoModifiedStars
	RepoModifiedMetadata
	RepoModifiedSources
)

func (m RepoModifiedFields) String() string {
	if m == RepoUnmodified {
		return "repo unmodified"
	}

	modifications := []string{}
	if m&RepoModifiedName == RepoModifiedName {
		modifications = append(modifications, "name")
	}
	if m&RepoModifiedURI == RepoModifiedURI {
		modifications = append(modifications, "uri")
	}
	if m&RepoModifiedDescription == RepoModifiedDescription {
		modifications = append(modifications, "description")
	}
	if m&RepoModifiedExternalRepo == RepoModifiedExternalRepo {
		modifications = append(modifications, "external repo")
	}
	if m&RepoModifiedArchived == RepoModifiedArchived {
		modifications = append(modifications, "archived")
	}
	if m&RepoModifiedFork == RepoModifiedFork {
		modifications = append(modifications, "fork")
	}
	if m&RepoModifiedPrivate == RepoModifiedPrivate {
		modifications = append(modifications, "private")
	}
	if m&RepoModifiedStars == RepoModifiedStars {
		modifications = append(modifications, "stars")
	}
	if m&RepoModifiedMetadata == RepoModifiedMetadata {
		modifications = append(modifications, "metadata")
	}
	if m&RepoModifiedSources == RepoModifiedSources {
		modifications = append(modifications, "sources")
	}

	return "repo modifications: " + strings.Join(modifications, ", ")
}

// Update updates Repo r with the fields from the given newer Repo n, returning
// RepoUnmodified (0) if no fields were modified, and a non-zero value if one
// or more fields were modified.
func (r *Repo) Update(n *Repo) (modified RepoModifiedFields) {
	if !r.Name.Equal(n.Name) {
		r.Name = n.Name
		modified |= RepoModifiedName
	}

	if r.URI != n.URI {
		r.URI = n.URI
		modified |= RepoModifiedURI
	}

	if r.Description != n.Description {
		r.Description = n.Description
		modified |= RepoModifiedDescription
	}

	if n.ExternalRepo != (api.ExternalRepoSpec{}) &&
		!r.ExternalRepo.Equal(&n.ExternalRepo) {
		r.ExternalRepo = n.ExternalRepo
		modified |= RepoModifiedExternalRepo
	}

	if r.Archived != n.Archived {
		r.Archived = n.Archived
		modified |= RepoModifiedArchived
	}

	if r.Fork != n.Fork {
		r.Fork = n.Fork
		modified |= RepoModifiedFork
	}

	if r.Private != n.Private {
		r.Private = n.Private
		modified |= RepoModifiedPrivate
	}

	if r.Stars != n.Stars {
		r.Stars = n.Stars
		modified |= RepoModifiedStars
	}

	if !reflect.DeepEqual(r.Metadata, n.Metadata) {
		r.Metadata = n.Metadata
		modified |= RepoModifiedMetadata
	}

	for urn, info := range n.Sources {
		if old, ok := r.Sources[urn]; !ok || !reflect.DeepEqual(info, old) {
			r.Sources[urn] = info
			modified |= RepoModifiedSources
		}
	}

	return modified
}

// Clone returns a clone of the given repo.
func (r *Repo) Clone() *Repo {
	if r == nil {
		return nil
	}
	clone := *r
	if r.Sources != nil {
		clone.Sources = make(map[string]*SourceInfo, len(r.Sources))
		for k, v := range r.Sources {
			clone.Sources[k] = v
		}
	}
	return &clone
}

// Apply applies the given functional options to the Repo.
func (r *Repo) Apply(opts ...func(*Repo)) {
	if r == nil {
		return
	}

	for _, opt := range opts {
		opt(r)
	}
}

// With returns a clone of the given repo with the given functional options applied.
func (r *Repo) With(opts ...func(*Repo)) *Repo {
	clone := r.Clone()
	clone.Apply(opts...)
	return clone
}

// Less compares Repos by the important fields (fields with constraints in our
// DB). Additionally it will compare on Sources to give a deterministic order
// on repos returned from a sourcer.
//
// NewDiff relies on Less to deterministically decide on the order to merge
// repositories, as well as which repository to keep on conflicts.
//
// Context on using other fields such as timestamps to order/resolve
// conflicts: We only want to rely on values that have constraints in our
// database. Timestamps have the following downsides:
//
//   - We need to assume the upstream codehost has reasonable values for them
//   - Not all codehosts set them to relevant values (eg gitolite or other)
//   - They could change often for codehosts that do set them.
func (r *Repo) Less(s *Repo) bool {
	if r.ID != s.ID {
		return r.ID < s.ID
	}
	if r.Name != s.Name {
		return r.Name < s.Name
	}
	if cmp := r.ExternalRepo.Compare(s.ExternalRepo); cmp != 0 {
		return cmp == -1
	}

	return sortedSliceLess(sourcesKeys(r.Sources), sourcesKeys(s.Sources))
}

func (r *Repo) String() string {
	eid := fmt.Sprintf("{%s %s %s}", r.ExternalRepo.ServiceID, r.ExternalRepo.ServiceType, r.ExternalRepo.ID)
	if r.IsDeleted() {
		return fmt.Sprintf("Repo{ID: %d, Name: %q, EID: %s, IsDeleted: true}", r.ID, r.Name, eid)
	}
	return fmt.Sprintf("Repo{ID: %d, Name: %q, EID: %s}", r.ID, r.Name, eid)
}

func sourcesKeys(m map[string]*SourceInfo) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedSliceLess returns true if a < b
func sortedSliceLess(a, b []string) bool {
	for i, v := range a {
		if i == len(b) {
			return false
		}
		if v != b[i] {
			return v < b[i]
		}
	}
	return len(a) != len(b)
}

// Repos is an utility type with convenience methods for operating on lists of Repos.
type Repos []*Repo

func (rs Repos) Len() int           { return len(rs) }
func (rs Repos) Less(i, j int) bool { return rs[i].Less(rs[j]) }
func (rs Repos) Swap(i, j int)      { rs[i], rs[j] = rs[j], rs[i] }

// IDs returns the list of ids from all Repos.
func (rs Repos) IDs() []api.RepoID {
	ids := make([]api.RepoID, len(rs))
	for i := range rs {
		ids[i] = rs[i].ID
	}
	return ids
}

// Names returns the list of names from all Repos.
func (rs Repos) Names() []string {
	names := make([]string, len(rs))
	for i := range rs {
		names[i] = string(rs[i].Name)
	}
	return names
}

// NamesSummary caps the number of repos to 20 when composing a space-separated list string.
// Used in logging statements.
func (rs Repos) NamesSummary() string {
	if len(rs) > 20 {
		return strings.Join(rs[:20].Names(), " ") + "..."
	}
	return strings.Join(rs.Names(), " ")
}

// Kinds returns the unique set of kinds from all Repos.
func (rs Repos) Kinds() (kinds []string) {
	set := map[string]bool{}
	for _, r := range rs {
		kind := strings.ToUpper(r.ExternalRepo.ServiceType)
		if !set[kind] {
			kinds = append(kinds, kind)
			set[kind] = true
		}
	}
	return kinds
}

// ExternalRepos returns the list of set ExternalRepoSpecs from all Repos.
func (rs Repos) ExternalRepos() []api.ExternalRepoSpec {
	specs := make([]api.ExternalRepoSpec, 0, len(rs))
	for _, r := range rs {
		specs = append(specs, r.ExternalRepo)
	}
	return specs
}

// Sources returns a map of all the sources per repo id.
func (rs Repos) Sources() map[api.RepoID][]SourceInfo {
	sources := make(map[api.RepoID][]SourceInfo)
	for i := range rs {
		for _, info := range rs[i].Sources {
			sources[rs[i].ID] = append(sources[rs[i].ID], *info)
		}
	}

	return sources
}

// Concat adds the given Repos to the end of rs.
func (rs *Repos) Concat(others ...Repos) {
	for _, o := range others {
		*rs = append(*rs, o...)
	}
}

// Clone returns a clone of Repos.
func (rs Repos) Clone() Repos {
	o := make(Repos, 0, len(rs))
	for _, r := range rs {
		o = append(o, r.Clone())
	}
	return o
}

// Apply applies the given functional options to the Repo.
func (rs Repos) Apply(opts ...func(*Repo)) {
	for _, r := range rs {
		r.Apply(opts...)
	}
}

// With returns a clone of the given repos with the given functional options applied.
func (rs Repos) With(opts ...func(*Repo)) Repos {
	clone := rs.Clone()
	clone.Apply(opts...)
	return clone
}

// Filter returns all the Repos that match the given predicate.
func (rs Repos) Filter(pred func(*Repo) bool) (fs Repos) {
	for _, r := range rs {
		if pred(r) {
			fs = append(fs, r)
		}
	}
	return fs
}

// RepoIDName combines a repo name and ID into a single struct
type RepoIDName struct {
	ID   api.RepoID
	Name api.RepoName
}

// MinimalRepo represents a source code repository name, its ID and number of stars.
type MinimalRepo struct {
	ID    api.RepoID
	Name  api.RepoName
	Stars int
}

// MinimalRepos is an utility type with convenience methods for operating on lists of repo names
type MinimalRepos []MinimalRepo

func (rs MinimalRepos) Len() int           { return len(rs) }
func (rs MinimalRepos) Less(i, j int) bool { return rs[i].ID < rs[j].ID }
func (rs MinimalRepos) Swap(i, j int)      { rs[i], rs[j] = rs[j], rs[i] }

type CodeHostRepository struct {
	Name       string
	CodeHostID int64
	Private    bool
}

// RepoGitserverStatus includes basic repo data along with the current gitserver
// status for the repo, which may be unknown.
type RepoGitserverStatus struct {
	// ID is the unique numeric ID for this repository.
	ID api.RepoID
	// Name is the name for this repository (e.g., "github.com/user/repo").
	Name api.RepoName

	// GitserverRepo data if it exists
	*GitserverRepo
}

type CloneStatus string

const (
	CloneStatusUnknown   CloneStatus = ""
	CloneStatusNotCloned CloneStatus = "not_cloned"
	CloneStatusCloning   CloneStatus = "cloning"
	CloneStatusCloned    CloneStatus = "cloned"
)

func ParseCloneStatus(s string) CloneStatus {
	cs := CloneStatus(s)
	switch cs {
	case CloneStatusNotCloned, CloneStatusCloning, CloneStatusCloned:
		return cs
	default:
		return CloneStatusUnknown
	}
}

// ParseCloneStatusFromGraphQL converts the raw value of the GraphQL enum
// CloneStatus into the corresponding CloneStatus defined here. If the GraphQL
// value can't be matched to a CloneStatus, CloneStatusUnknown is returned.
func ParseCloneStatusFromGraphQL(s string) CloneStatus {
	return ParseCloneStatus(strings.ToLower(s))
}

// GitserverRepo represents the data gitserver knows about a repo
type GitserverRepo struct {
	RepoID api.RepoID
	// Usually represented by a gitserver hostname
	ShardID         string
	CloneStatus     CloneStatus
	CloningProgress string
	// The last error that occurred or empty if the last action was successful
	LastError string
	// The last time fetch was called.
	LastFetched time.Time
	// The last time a fetch updated the repository.
	LastChanged time.Time
	// Size of the repository in bytes.
	RepoSizeBytes int64
	// Time when corruption of repo was detected
	CorruptedAt time.Time
	UpdatedAt   time.Time
	// A log of the different types of corruption that was detected on this repo. The order of the log entries are
	// stored from most recent to least recent and capped at 10 entries. See LogCorruption on Gitserverrepo store.
	CorruptionLogs []RepoCorruptionLog
}

// RepoCorruptionLog represents a corruption event that has been detected on a repo.
type RepoCorruptionLog struct {
	// When the corruption event was detected
	Timestamp time.Time `json:"time"`
	// Why the repo is considered to be corrupt. Can be git output stderr output or a short reason like "missing head"
	Reason string `json:"reason"`
}

// ExternalService is a connection to an external service.
type ExternalService struct {
	ID             int64
	Kind           string
	DisplayName    string
	Config         *extsvc.EncryptableConfig
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      time.Time
	LastSyncAt     time.Time
	NextSyncAt     time.Time
	Unrestricted   bool       // Whether access to repositories belong to this external service is unrestricted.
	CloudDefault   bool       // Whether this external service is our default public service on Cloud
	HasWebhooks    *bool      // Whether this external service has webhooks configured; calculated from Config
	TokenExpiresAt *time.Time // Whether the token in this external services expires, nil indicates never expires.
	CodeHostID     *int32
	CreatorID      *int32
	LastUpdaterID  *int32
}

type ExternalServiceRepo struct {
	ExternalServiceID int64      `json:"externalServiceID"`
	RepoID            api.RepoID `json:"repoID"`
	CloneURL          string     `json:"cloneURL"`
	CreatedAt         time.Time  `json:"createdAt"`
}

// ExternalServiceSyncJob represents an sync job for an external service
type ExternalServiceSyncJob struct {
	ID                int64 // TODO: Why is this an int64, it's a 32 bit int in the database
	State             string
	FailureMessage    string
	QueuedAt          time.Time
	StartedAt         time.Time
	FinishedAt        time.Time
	ProcessAfter      time.Time
	NumResets         int // TODO: This is a 32 bit int in the database
	ExternalServiceID int64
	NumFailures       int
	Cancel            bool

	// Counters that show progress of a running job
	ReposSynced     int32
	RepoSyncErrors  int32
	ReposAdded      int32
	ReposDeleted    int32
	ReposModified   int32
	ReposUnmodified int32
}

// ExternalServiceNamespace represents a namespace on an external service that can have ownership over repositories
type ExternalServiceNamespace struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	ExternalID string `json:"external_id"`
}

// ExternalServiceRepository represents a repository on an external service that may not necessarily be sync'd with sourcegraph
type ExternalServiceRepository struct {
	ID         api.RepoID   `json:"id"`
	Name       api.RepoName `json:"name"`
	ExternalID string       `json:"external_id"`
}

// URN returns a unique resource identifier of this external service,
// used as the key in a repo's Sources map as well as the SourceInfo ID.
func (e *ExternalService) URN() string {
	return extsvc.URN(e.Kind, e.ID)
}

// IsDeleted returns true if the external service is deleted.
func (e *ExternalService) IsDeleted() bool { return !e.DeletedAt.IsZero() }

// Update updates ExternalService e with the fields from the given newer ExternalService n,
// returning true if modified.
func (e *ExternalService) Update(ctx context.Context, n *ExternalService) (modified bool, _ error) {
	if e.ID != n.ID {
		return false, nil
	}

	if !strings.EqualFold(e.Kind, n.Kind) {
		e.Kind, modified = strings.ToUpper(n.Kind), true
	}

	if e.DisplayName != n.DisplayName {
		e.DisplayName, modified = n.DisplayName, true
	}

	eConfig, err := e.Config.Decrypt(ctx)
	if err != nil {
		return false, err
	}

	nConfig, err := n.Config.Decrypt(ctx)
	if err != nil {
		return false, err
	}
	if eConfig != nConfig {
		e.Config.Set(nConfig)
		modified = true
	}

	if !e.UpdatedAt.Equal(n.UpdatedAt) {
		e.UpdatedAt, modified = n.UpdatedAt, true
	}

	if !e.DeletedAt.Equal(n.DeletedAt) {
		e.DeletedAt, modified = n.DeletedAt, true
	}

	return modified, nil
}

// Configuration returns the external service config.
func (e *ExternalService) Configuration(ctx context.Context) (cfg any, _ error) {
	return extsvc.ParseEncryptableConfig(ctx, e.Kind, e.Config)
}

// Clone returns a clone of the given external service.
func (e *ExternalService) Clone() *ExternalService {
	clone := *e
	return &clone
}

// Apply applies the given functional options to the ExternalService.
func (e *ExternalService) Apply(opts ...func(*ExternalService)) {
	if e == nil {
		return
	}

	for _, opt := range opts {
		opt(e)
	}
}

// With returns a clone of the given repo with the given functional options applied.
func (e *ExternalService) With(opts ...func(*ExternalService)) *ExternalService {
	clone := e.Clone()
	clone.Apply(opts...)
	return clone
}

// SupportsRepoExclusion returns true when given external service supports repo
// exclusion.
func (e *ExternalService) SupportsRepoExclusion() bool {
	return extsvc.SupportsRepoExclusion(e.Kind)
}

// ExternalServices is a utility type with convenience methods for operating on
// lists of ExternalServices.
type ExternalServices []*ExternalService

// IDs returns the list of ids from all ExternalServices.
func (es ExternalServices) IDs() []int64 {
	ids := make([]int64, len(es))
	for i := range es {
		ids[i] = es[i].ID
	}
	return ids
}

// DisplayNames returns the list of display names from all ExternalServices.
func (es ExternalServices) DisplayNames() []string {
	names := make([]string, len(es))
	for i := range es {
		names[i] = es[i].DisplayName
	}
	return names
}

// Kinds returns the unique set of Kinds in the given external services list.
func (es ExternalServices) Kinds() (kinds []string) {
	set := make(map[string]bool, len(es))
	for _, e := range es {
		if !set[e.Kind] {
			kinds = append(kinds, e.Kind)
			set[e.Kind] = true
		}
	}
	return kinds
}

// URNs returns the list of URNs from all ExternalServices.
func (es ExternalServices) URNs() []string {
	urns := make([]string, len(es))
	for i := range es {
		urns[i] = es[i].URN()
	}
	return urns
}

func (es ExternalServices) Len() int {
	return len(es)
}

func (es ExternalServices) Swap(i, j int) {
	es[i], es[j] = es[j], es[i]
}

func (es ExternalServices) Less(i, j int) bool {
	return es[i].ID < es[j].ID
}

// Clone returns a clone of the given external services.
func (es ExternalServices) Clone() ExternalServices {
	o := make(ExternalServices, 0, len(es))
	for _, r := range es {
		o = append(o, r.Clone())
	}
	return o
}

// Apply applies the given functional options to the ExternalService.
func (es ExternalServices) Apply(opts ...func(*ExternalService)) {
	for _, r := range es {
		r.Apply(opts...)
	}
}

// With returns a clone of the given external services with the given functional options applied.
func (es ExternalServices) With(opts ...func(*ExternalService)) ExternalServices {
	clone := es.Clone()
	clone.Apply(opts...)
	return clone
}

type GlobalState struct {
	SiteID      string
	Initialized bool // whether the initial site admin account has been created
}

// User represents a registered user.
type User struct {
	ID                    int32
	Username              string
	DisplayName           string
	AvatarURL             string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	SiteAdmin             bool
	BuiltinAuth           bool
	InvalidatedSessionsAt time.Time
	TosAccepted           bool
	CompletedPostSignup   bool
	SCIMControlled        bool
}

// Name returns a name for the user. If the user has a display name,
// that is returned, otherwise their username is returned.
func (u *User) Name() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}

	return u.Username
}

// UserForSCIM extends user with email addresses and SCIM external ID.
type UserForSCIM struct {
	User
	Emails          []string
	SCIMExternalID  string
	SCIMAccountData string
	Active          bool
}

type SystemRole string

const (
	// UserSystemRole represents the role associated with all users on a Sourcegraph instance.
	UserSystemRole SystemRole = "USER"

	// SiteAdministratorSystemRole represents the role associated with Site Administrators
	// on a sourcegraph instance.
	SiteAdministratorSystemRole SystemRole = "SITE_ADMINISTRATOR"
)

type Role struct {
	ID        int32
	Name      string
	System    bool
	CreatedAt time.Time
}

func (r Role) IsSiteAdmin() bool {
	return r.Name == string(SiteAdministratorSystemRole)
}

func (r Role) IsUser() bool {
	return r.Name == string(UserSystemRole)
}

type Permission struct {
	ID        int32
	Namespace rtypes.PermissionNamespace
	Action    rtypes.NamespaceAction
	CreatedAt time.Time
}

// DisplayName returns an human-readable string for permissions.
func (p *Permission) DisplayName() string {
	// Based on the zanzibar representation for data relations:
	// <namespace>:<object_id>#<relation>@<user_id | user_group>
	return fmt.Sprintf("%s#%s", p.Namespace, p.Action)
}

type RolePermission struct {
	RoleID       int32
	PermissionID int32
	CreatedAt    time.Time
}

type UserRole struct {
	RoleID    int32
	UserID    int32
	CreatedAt time.Time
}

type NamespacePermission struct {
	ID         int64
	Namespace  rtypes.PermissionNamespace
	ResourceID int64
	UserID     int32
	CreatedAt  time.Time
}

func (n *NamespacePermission) DisplayName() string {
	// Based on the zanzibar representation for data relations:
	// <namespace>:<object_id>#@<user_id | user_group>
	return fmt.Sprintf("%s:%d@%d", n.Namespace, n.ResourceID, n.UserID)
}

type OrgMemberAutocompleteSearchItem struct {
	ID          int32
	Username    string
	DisplayName string
	AvatarURL   string
	InOrg       int32
}

type Org struct {
	ID          int32
	Name        string
	DisplayName *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type OrgMembership struct {
	ID        int32
	OrgID     int32
	UserID    int32
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PhabricatorRepo struct {
	ID       int32
	Name     api.RepoName
	URL      string
	Callsign string
}

type UserUsageStatistics struct {
	UserID                      int32
	PageViews                   int32
	SearchQueries               int32
	CodeIntelligenceActions     int32
	FindReferencesActions       int32
	LastActiveTime              *time.Time
	LastCodeHostIntegrationTime *time.Time
}

// UserUsageCounts captures the usage numbers of a user in a single day.
type UserUsageCounts struct {
	Date           time.Time
	UserID         uint32
	SearchCount    int32
	CodeIntelCount int32
}

// UserDates captures the created and deleted dates of a single user.
type UserDates struct {
	UserID    int32
	CreatedAt time.Time
	DeletedAt time.Time
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type CodyUsageStatistics struct {
	Daily   *CodyUsagePeriodLimited
	Weekly  *CodyUsagePeriodLimited
	Monthly *CodyUsagePeriod
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type CodyUsagePeriod struct {
	StartTime                  time.Time
	TotalCodyUsers             *CodyCountStatistics `json:"TotalCodyUsers,omitempty"`
	TotalProductUsers          *CodyCountStatistics `json:"TotalProductUsers,omitempty"`
	TotalVSCodeProductUsers    *CodyCountStatistics `json:"TotalVSCodeProductUsers,omitempty"`
	TotalJetBrainsProductUsers *CodyCountStatistics `json:"TotalJetBrainsProductUsers,omitempty"`
	TotalNeovimProductUsers    *CodyCountStatistics `json:"TotalNeovimProductUsers,omitempty"`
	TotalEmacsProductUsers     *CodyCountStatistics `json:"TotalEmacsProductUsers,omitempty"`
	TotalWebProductUsers       *CodyCountStatistics `json:"TotalWebProductUsers,omitempty"`
}

type CodyUsagePeriodLimited struct {
	StartTime         time.Time
	TotalCodyUsers    *CodyCountStatistics `json:"TotalCodyUsers,omitempty"`
	TotalProductUsers *CodyCountStatistics `json:"TotalProductUsers,omitempty"`
}

type CodyCountStatistics struct {
	UserCount   *int32 `json:"UserCount,omitempty"`
	EventsCount *int32 `json:"EventsCount,omitempty"`
}

// CodyAggregatedUsage represents the total Cody-related event count and
// unique users for the current day, week, and month, as well as the
// count of total unique users by client for the current month.
type CodyAggregatedUsage struct {
	Month                      time.Time
	Week                       time.Time
	Day                        time.Time
	TotalMonth                 int32
	TotalWeek                  int32
	TotalDay                   int32
	UniquesMonth               int32
	UniquesWeek                int32
	UniquesDay                 int32
	ProductUsersMonth          int32
	ProductUsersWeek           int32
	ProductUsersDay            int32
	VSCodeProductUsersMonth    int32
	JetBrainsProductUsersMonth int32
	NeovimProductUsersMonth    int32
	EmacsProductUsersMonth     int32
	WebProductUsersMonth       int32
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type CodyProviders struct {
	Completions *CodyCompletionProvider
	Embeddings  *CodyEmbeddingsProvider
}

type CodyCompletionProvider struct {
	ChatModel       string
	CompletionModel string
	FastChatModel   string
	Provider        conftypes.CompletionsProviderName
}

type CodyEmbeddingsProvider struct {
	Model    string
	Provider conftypes.EmbeddingsProviderName
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler.
// RepoMetadataAggregatedStats represents the total number of repo metadata,
// number of repositories with any metadata, total and unique number of
// events for repo metadata usage related events over the current day, week, month.
type RepoMetadataAggregatedStats struct {
	Summary *RepoMetadataAggregatedSummary
	Daily   *RepoMetadataAggregatedEvents
	Weekly  *RepoMetadataAggregatedEvents
	Monthly *RepoMetadataAggregatedEvents
}

type RepoMetadataAggregatedSummary struct {
	IsEnabled              bool
	RepoMetadataCount      *int32
	ReposWithMetadataCount *int32
}

type RepoMetadataAggregatedEvents struct {
	StartTime          time.Time
	CreateRepoMetadata *EventStats
	UpdateRepoMetadata *EventStats
	DeleteRepoMetadata *EventStats
	SearchFilterUsage  *EventStats
}

type EventStats struct {
	UsersCount  *int32
	EventsCount *int32
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type SiteUsageStatistics struct {
	DAUs  []*SiteActivityPeriod
	WAUs  []*SiteActivityPeriod
	MAUs  []*SiteActivityPeriod
	RMAUs []*SiteActivityPeriod
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type SiteActivityPeriod struct {
	StartTime            time.Time
	UserCount            int32
	RegisteredUserCount  int32
	AnonymousUserCount   int32
	IntegrationUserCount int32
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type BatchChangesUsageStatistics struct {
	// ViewBatchChangeApplyPageCount is the number of page views on the apply page
	// ("preview" page).
	ViewBatchChangeApplyPageCount int32
	// ViewBatchChangeDetailsPageAfterCreateCount is the number of page views on
	// the batch changes details page *after creating* the batch change on the apply
	// page by clicking "Apply".
	ViewBatchChangeDetailsPageAfterCreateCount int32
	// ViewBatchChangeDetailsPageAfterUpdateCount is the number of page views on
	// the batch changes details page *after updating* a batch change on the apply page
	// by clicking "Apply".
	ViewBatchChangeDetailsPageAfterUpdateCount int32

	// BatchChangesCount is the number of batch changes on the instance. This can go
	// down when users delete a batch change.
	BatchChangesCount int32
	// BatchChangesClosedCount is the number of *closed* batch changes on the
	// instance. This can go down when users delete a batch change.
	BatchChangesClosedCount int32

	// BatchSpecsCreatedCount is the number of batch change specs that have been
	// created by running `src batch [preview|apply]`. This number never
	// goes down since it's based on event logs, even if the batch specs
	// were not used and cleaned up.
	BatchSpecsCreatedCount int32
	// ChangesetSpecsCreatedCount is the number of changeset specs that have
	// been created by running `src batch [preview|apply]`. This number
	// never goes down since it's based on event logs, even if the changeset
	// specs were not used and cleaned up.
	ChangesetSpecsCreatedCount int32

	// PublishedChangesetsUnpublishedCount is the number of changesets in the
	// database that have not been published but belong to a batch change.
	// This number *could* go down, since it's not
	// based on event logs, but so far (Mar 2021) we never cleaned up
	// changesets in the database.
	PublishedChangesetsUnpublishedCount int32

	// PublishedChangesetsCount is the number of changesets published on code hosts
	// by batch changes. This number *could* go down, since it's not based on
	// event logs, but so far (Mar 2021) we never cleaned up changesets in the
	// database.
	PublishedChangesetsCount int32
	// PublishedChangesetsDiffStatAddedSum is the total sum of lines added by
	// changesets published on the code host by batch changes.
	PublishedChangesetsDiffStatAddedSum int32
	// PublishedChangesetsDiffStatDeletedSum is the total sum of lines deleted by
	// changesets published on the code host by batch changes.
	PublishedChangesetsDiffStatDeletedSum int32

	// PublishedChangesetsMergedCount is the number of changesets published on
	// code hosts by batch changes that have also been *merged*.
	// This number *could* go down, since it's not based on event logs, but
	// so far (Mar 2021) we never cleaned up changesets in the database.
	PublishedChangesetsMergedCount int32
	// PublishedChangesetsMergedDiffStatAddedSum is the total sum of lines added by
	// changesets published on the code host by batch changes and merged.
	PublishedChangesetsMergedDiffStatAddedSum int32
	// PublishedChangesetsMergedDiffStatDeletedSum is the total sum of lines deleted by
	// changesets published on the code host by batch changes and merged.
	PublishedChangesetsMergedDiffStatDeletedSum int32

	// ImportedChangesetsCount is the total number of changesets that have been
	// imported by a batch change to be tracked.
	// This number *could* go down, since it's not based on event logs, but
	// so far (Mar 2021) we never cleaned up changesets in the database.
	ImportedChangesetsCount int32
	// ManualChangesetsCount is the total number of *merged* changesets that
	// have been imported by a batch change to be tracked.
	// This number *could* go down, since it's not based on event logs, but
	// so far (Mar 2021) we never cleaned up changesets in the database.
	ImportedChangesetsMergedCount int32

	// CurrentMonthContributorsCount is the count of unique users that have logged a
	// "contributing" batch changes event, such as "BatchChangeCreated".
	//
	// See `contributorsEvents` in `GetBatchChangesUsageStatistics` for a full list
	// of events.
	CurrentMonthContributorsCount int64

	// CurrentMonthUsersCount is the count of unique users that have logged a
	// "using" batch changes event, such as "ViewBatchChangesListPage" and also "BatchChangeCreated".
	//
	// See `contributorsEvents` in `GetBatchChangesUsageStatistics` for a full
	// list of events.
	CurrentMonthUsersCount int64

	BatchChangesCohorts []*BatchChangesCohort

	// ActiveExecutorsCount is the count of executors that have had a heartbeat in the last
	// 15 seconds.
	ActiveExecutorsCount int32

	// BulkOperationsCount is the count of bulk operations used to manage changesets
	BulkOperationsCount []*BulkOperationsCount

	// ChangesetDistribution is the distribution of batch changes per source and the amount of
	// changesets created via the different sources
	ChangesetDistribution []*ChangesetDistribution

	// BatchChangeStatsBySource is the distribution of batch change x changesets statistics
	// across multiple sources
	BatchChangeStatsBySource []*BatchChangeStatsBySource

	// MonthlyBatchChangesExecutorUsage is the number of users who ran a job on an
	// executor in a given month
	MonthlyBatchChangesExecutorUsage []*MonthlyBatchChangesExecutorUsage

	WeeklyBulkOperationStats []*WeeklyBulkOperationStats
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type BulkOperationsCount struct {
	Name  string
	Count int32
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type WeeklyBulkOperationStats struct {
	// Week is the week of this cohort and is used to group batch changes by
	// their creation date.
	Week string

	// Count is the number of bulk operations carried out in a particular week.
	Count int32

	BulkOperation string
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type MonthlyBatchChangesExecutorUsage struct {
	// Month of the year corresponding to this executor usage data.
	Month string

	// The number of unique users who ran a job on an executor this month.
	Count int32

	// The cumulative number of minutes of executor usage for batch changes this month.
	Minutes int64
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type BatchChangeStatsBySource struct {
	// the source of the changesets belonging to the batch changes
	// indicating whether the changeset was created via an executor or locally.
	Source BatchChangeSource

	// the amount of changesets published using this batch change source.
	PublishedChangesetsCount int32

	// the amount of batch changes created from this source.
	BatchChangesCount int32
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type ChangesetDistribution struct {
	// the source of the changesets belonging to the batch changes
	// indicating whether the changeset was created via an executor or locally
	Source BatchChangeSource

	// range of changeset distribution per batch_change
	Range string

	// number of batch changes with the range of changesets defined
	BatchChangesCount int32
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type BatchChangesCohort struct {
	// Week is the week of this cohort and is used to group batch changes by
	// their creation date.
	Week string

	// BatchChangesClosed is the number of batch changes that were created in Week and
	// are currently closed.
	BatchChangesClosed int64

	// BatchChangesOpen is the number of batch changes that were created in Week and
	// are currently open.
	BatchChangesOpen int64

	// The following are the counts of the changesets that are currently
	// attached to the batch changes in this cohort.

	ChangesetsImported        int64
	ChangesetsUnpublished     int64
	ChangesetsPublished       int64
	ChangesetsPublishedOpen   int64
	ChangesetsPublishedDraft  int64
	ChangesetsPublishedMerged int64
	ChangesetsPublishedClosed int64
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type SearchUsageStatistics struct {
	Daily   []*SearchUsagePeriod
	Weekly  []*SearchUsagePeriod
	Monthly []*SearchUsagePeriod
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type SearchUsagePeriod struct {
	StartTime  time.Time
	TotalUsers int32

	// Counts and latency statistics for different kinds of searches.
	Literal    *SearchEventStatistics
	Regexp     *SearchEventStatistics
	Commit     *SearchEventStatistics
	Diff       *SearchEventStatistics
	File       *SearchEventStatistics
	Structural *SearchEventStatistics
	Symbol     *SearchEventStatistics

	// Counts of search query attributes. Ref: RFC 384.
	OperatorOr              *SearchCountStatistics
	OperatorAnd             *SearchCountStatistics
	OperatorNot             *SearchCountStatistics
	SelectRepo              *SearchCountStatistics
	SelectFile              *SearchCountStatistics
	SelectContent           *SearchCountStatistics
	SelectSymbol            *SearchCountStatistics
	SelectCommitDiffAdded   *SearchCountStatistics
	SelectCommitDiffRemoved *SearchCountStatistics
	RepoContains            *SearchCountStatistics
	RepoContainsFile        *SearchCountStatistics
	RepoContainsContent     *SearchCountStatistics
	RepoContainsCommitAfter *SearchCountStatistics
	RepoDependencies        *SearchCountStatistics
	CountAll                *SearchCountStatistics
	NonGlobalContext        *SearchCountStatistics
	OnlyPatterns            *SearchCountStatistics
	OnlyPatternsThreeOrMore *SearchCountStatistics

	// DEPRECATED. Counts statistics for fields.
	After              *SearchCountStatistics
	Archived           *SearchCountStatistics
	Author             *SearchCountStatistics
	Before             *SearchCountStatistics
	Case               *SearchCountStatistics
	Committer          *SearchCountStatistics
	Content            *SearchCountStatistics
	Count              *SearchCountStatistics
	Fork               *SearchCountStatistics
	Index              *SearchCountStatistics
	Lang               *SearchCountStatistics
	Message            *SearchCountStatistics
	PatternType        *SearchCountStatistics
	Repo               *SearchEventStatistics
	Repohascommitafter *SearchCountStatistics
	Repohasfile        *SearchCountStatistics
	Repogroup          *SearchCountStatistics
	Timeout            *SearchCountStatistics
	Type               *SearchCountStatistics

	// DEPRECATED. Search modes statistics refers to removed functionality.
	SearchModes *SearchModeUsageStatistics
}

type SearchModeUsageStatistics struct {
	Interactive *SearchCountStatistics
	PlainText   *SearchCountStatistics
}

type SearchCountStatistics struct {
	UserCount   *int32
	EventsCount *int32
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type SearchEventStatistics struct {
	UserCount      *int32
	EventsCount    *int32
	EventLatencies *SearchEventLatencies
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type SearchEventLatencies struct {
	P50 float64
	P90 float64
	P99 float64
}

// SiteUsageSummary is an alternate view of SiteUsageStatistics which is
// calculated in the database layer.
type SiteUsageSummary struct {
	RollingMonth                   time.Time
	Month                          time.Time
	Week                           time.Time
	Day                            time.Time
	UniquesRollingMonth            int32
	UniquesMonth                   int32
	UniquesWeek                    int32
	UniquesDay                     int32
	RegisteredUniquesRollingMonth  int32
	RegisteredUniquesMonth         int32
	RegisteredUniquesWeek          int32
	RegisteredUniquesDay           int32
	IntegrationUniquesRollingMonth int32
	IntegrationUniquesMonth        int32
	IntegrationUniquesWeek         int32
	IntegrationUniquesDay          int32
}

// SearchAggregatedEvent represents the total events, unique users, and
// latencies over the current month, week, and day for a single search event.
type SearchAggregatedEvent struct {
	Name           string
	Month          time.Time
	Week           time.Time
	Day            time.Time
	TotalMonth     int32
	TotalWeek      int32
	TotalDay       int32
	UniquesMonth   int32
	UniquesWeek    int32
	UniquesDay     int32
	LatenciesMonth []float64
	LatenciesWeek  []float64
	LatenciesDay   []float64
}

type SurveyResponse struct {
	ID           int32
	UserID       *int32
	Email        *string
	Score        int32
	Reason       *string
	Better       *string
	OtherUseCase *string
	CreatedAt    time.Time
}

type Event struct {
	ID              int32
	Name            string
	URL             string
	UserID          int32
	AnonymousUserID string
	Argument        string
	Source          string
	Version         string
	Timestamp       time.Time
}

// GrowthStatistics represents the total users that were created,
// deleted, resurrected, churned and retained over the current month.
type GrowthStatistics struct {
	DeletedUsers           int32
	CreatedUsers           int32
	ResurrectedUsers       int32
	ChurnedUsers           int32
	RetainedUsers          int32
	PendingAccessRequests  int32
	ApprovedAccessRequests int32
	RejectedAccessRequests int32
}

// IDEExtensionsUsage represents the daily, weekly and monthly numbers
// of search performed and user state events from all IDE extensions,
// and all inbound traffic from the extension to Sourcegraph instance
type IDEExtensionsUsage struct {
	IDEs []*IDEExtensionsUsageStatistics
}

// Usage statistics from each IDE extension
type IDEExtensionsUsageStatistics struct {
	IdeKind string
	Month   IDEExtensionsUsageRegularPeriod
	Week    IDEExtensionsUsageRegularPeriod
	Day     IDEExtensionsUsageDailyPeriod
}

// Monthly and Weekly usage from each IDE extension
type IDEExtensionsUsageRegularPeriod struct {
	StartTime         time.Time
	SearchesPerformed IDEExtensionsUsageSearchesPerformed
}

// Daily usage from each IDE extension
type IDEExtensionsUsageDailyPeriod struct {
	StartTime         time.Time
	SearchesPerformed IDEExtensionsUsageSearchesPerformed
	UserState         IDEExtensionsUsageUserState
	RedirectsCount    int32
}

// Count of unique users who performed searches & total searches performed
type IDEExtensionsUsageSearchesPerformed struct {
	UniquesCount int32
	TotalCount   int32
}

// Count of unique users who installed & uninstalled each extension
type IDEExtensionsUsageUserState struct {
	Installs   int32
	Uninstalls int32
}

// MigratedExtensionsUsageStatistics repreents the numbers of interactions with
// the migrated extensions (git blame, open in editor, search exports, and go
// imports search).
type MigratedExtensionsUsageStatistics struct {
	GitBlameEnabled                 *int32
	GitBlameEnabledUniqueUsers      *int32
	GitBlameDisabled                *int32
	GitBlameDisabledUniqueUsers     *int32
	GitBlamePopupViewed             *int32
	GitBlamePopupViewedUniqueUsers  *int32
	GitBlamePopupClicked            *int32
	GitBlamePopupClickedUniqueUsers *int32

	SearchExportPerformed            *int32
	SearchExportPerformedUniqueUsers *int32
	SearchExportFailed               *int32
	SearchExportFailedUniqueUsers    *int32

	OpenInEditor []*MigratedExtensionsOpenInEditorUsageStatistics
}

type MigratedExtensionsOpenInEditorUsageStatistics struct {
	IdeKind            string
	Clicked            *int32
	ClickedUniqueUsers *int32
}

// CodeHostIntegrationUsage represents the daily, weekly and monthly
// number of unique users and events for code host integration usage
// and inbound traffic from code host integration to Sourcegraph instance
type CodeHostIntegrationUsage struct {
	Month CodeHostIntegrationUsagePeriod
	Week  CodeHostIntegrationUsagePeriod
	Day   CodeHostIntegrationUsagePeriod
}

type CodeHostIntegrationUsagePeriod struct {
	StartTime         time.Time
	BrowserExtension  CodeHostIntegrationUsageType
	NativeIntegration CodeHostIntegrationUsageType
}

type CodeHostIntegrationUsageType struct {
	UniquesCount        int32
	TotalCount          int32
	InboundTrafficToWeb CodeHostIntegrationUsageInboundTrafficToWeb
}

type CodeHostIntegrationUsageInboundTrafficToWeb struct {
	UniquesCount int32
	TotalCount   int32
}

// SavedSearches represents the total number of saved searches, users
// using saved searches, and usage of saved searches.
type SavedSearches struct {
	TotalSavedSearches   int32
	UniqueUsers          int32
	NotificationsSent    int32
	NotificationsClicked int32
	UniqueUserPageViews  int32
	OrgSavedSearches     int32
}

// Panel homepage represents interaction data on the
// enterprise homepage panels.
type HomepagePanels struct {
	RecentFilesClickedPercentage           *float64
	RecentSearchClickedPercentage          *float64
	RecentRepositoriesClickedPercentage    *float64
	SavedSearchesClickedPercentage         *float64
	NewSavedSearchesClickedPercentage      *float64
	TotalPanelViews                        *float64
	UsersFilesClickedPercentage            *float64
	UsersSearchClickedPercentage           *float64
	UsersRepositoriesClickedPercentage     *float64
	UsersSavedSearchesClickedPercentage    *float64
	UsersNewSavedSearchesClickedPercentage *float64
	PercentUsersShown                      *float64
}

type WeeklyRetentionStats struct {
	WeekStart  time.Time
	CohortSize *int32
	Week0      *float64
	Week1      *float64
	Week2      *float64
	Week3      *float64
	Week4      *float64
	Week5      *float64
	Week6      *float64
	Week7      *float64
	Week8      *float64
	Week9      *float64
	Week10     *float64
	Week11     *float64
}

type RetentionStats struct {
	Weekly []*WeeklyRetentionStats
}

type SearchOnboarding struct {
	TotalOnboardingTourViews   *int32
	ViewedLangStep             *int32
	ViewedFilterRepoStep       *int32
	ViewedAddQueryTermStep     *int32
	ViewedSubmitSearchStep     *int32
	ViewedSearchReferenceStep  *int32
	CloseOnboardingTourClicked *int32
}

// Weekly usage statistics for the extensions platform
type ExtensionsUsageStatistics struct {
	WeekStart                  time.Time
	UsageStatisticsByExtension []*ExtensionUsageStatistics
	// Average number of non-default extensions used by users
	// that have used at least one non-default extension
	AverageNonDefaultExtensions *float64
	// The count of users that have activated a non-default extension this week
	NonDefaultExtensionUsers *int32
}

// Weekly statistics for an individual extension
type ExtensionUsageStatistics struct {
	// The count of users that have activated this extension
	UserCount *int32
	// The average number of activations for users that have
	// used this extension at least once
	AverageActivations *float64
	ExtensionID        *string
}

type SearchJobsUsageStatistics struct {
	WeeklySearchJobsPageViews            *int32
	WeeklySearchJobsCreateClick          *int32
	WeeklySearchJobsDownloadClicks       *int32
	WeeklySearchJobsViewLogsClicks       *int32
	WeeklySearchJobsUniquePageViews      *int32
	WeeklySearchJobsUniqueDownloadClicks *int32
	WeeklySearchJobsUniqueViewLogsClicks *int32
	WeeklySearchJobsSearchFormShown      []SearchJobsSearchFormShownPing
}

type SearchJobsSearchFormShownPing struct {
	ValidState string
	TotalCount int
}

type CodeInsightsUsageStatistics struct {
	WeeklyUsageStatisticsByInsight                 []*InsightUsageStatistics
	WeeklyInsightsPageViews                        *int32
	WeeklyStandaloneInsightPageViews               *int32
	WeeklyStandaloneDashboardClicks                *int32
	WeeklyStandaloneEditClicks                     *int32
	WeeklyInsightsGetStartedPageViews              *int32
	WeeklyInsightsUniquePageViews                  *int32
	WeeklyInsightsGetStartedUniquePageViews        *int32
	WeeklyStandaloneInsightUniquePageViews         *int32
	WeeklyStandaloneInsightUniqueDashboardClicks   *int32
	WeeklyStandaloneInsightUniqueEditClicks        *int32
	WeeklyInsightConfigureClick                    *int32
	WeeklyInsightAddMoreClick                      *int32
	WeekStart                                      time.Time
	WeeklyInsightCreators                          *int32
	WeeklyFirstTimeInsightCreators                 *int32
	WeeklyAggregatedUsage                          []AggregatedPingStats
	WeeklyGetStartedTabClickByTab                  []InsightGetStartedTabClickPing
	WeeklyGetStartedTabMoreClickByTab              []InsightGetStartedTabClickPing
	InsightTimeIntervals                           []InsightTimeIntervalPing
	InsightOrgVisible                              []OrgVisibleInsightPing
	InsightTotalCounts                             InsightTotalCounts
	TotalOrgsWithDashboard                         *int32
	TotalDashboardCount                            *int32
	InsightsPerDashboard                           InsightsPerDashboardPing
	WeeklyGroupResultsOpenSection                  *int32
	WeeklyGroupResultsCollapseSection              *int32
	WeeklyGroupResultsInfoIconHover                *int32
	WeeklyGroupResultsExpandedViewOpen             []GroupResultExpandedViewPing
	WeeklyGroupResultsExpandedViewCollapse         []GroupResultExpandedViewPing
	WeeklyGroupResultsChartBarHover                []GroupResultPing
	WeeklyGroupResultsChartBarClick                []GroupResultPing
	WeeklyGroupResultsAggregationModeClicked       []GroupResultPing
	WeeklyGroupResultsAggregationModeDisabledHover []GroupResultPing
	WeeklyGroupResultsSearches                     []GroupResultSearchPing
	WeeklySeriesBackfillTime                       []InsightsBackfillTimePing
	WeeklyDataExportClicks                         *int32
}

type GroupResultPing struct {
	AggregationMode *string
	UIMode          *string
	Count           *int32
	BarIndex        *int32
}

type GroupResultExpandedViewPing struct {
	AggregationMode *string
	Count           *int32
}

type GroupResultSearchPing struct {
	Name            PingName
	AggregationMode *string
	Count           *int32
}

type CodeInsightsCriticalTelemetry struct {
	TotalInsights int32
}

// Usage statistics for a type of code insight
type InsightUsageStatistics struct {
	InsightType      *string
	Additions        *int32
	Edits            *int32
	Removals         *int32
	Hovers           *int32
	UICustomizations *int32
	DataPointClicks  *int32
	FiltersChange    *int32
}

type PingName string

// AggregatedPingStats is a generic representation of an aggregated ping statistic
type AggregatedPingStats struct {
	Name        PingName
	TotalCount  int
	UniqueCount int
}

type InsightTimeIntervalPing struct {
	IntervalDays int
	TotalCount   int
}

type OrgVisibleInsightPing struct {
	Type       string
	TotalCount int
}

type InsightViewsCountPing struct {
	ViewType   string
	TotalCount int
}

type InsightSeriesCountPing struct {
	GenerationType string
	TotalCount     int
}

type InsightViewSeriesCountPing struct {
	GenerationType string
	ViewType       string
	TotalCount     int
}

type InsightGetStartedTabClickPing struct {
	TabName    string
	TotalCount int
}

type InsightTotalCounts struct {
	ViewCounts       []InsightViewsCountPing
	SeriesCounts     []InsightSeriesCountPing
	ViewSeriesCounts []InsightViewSeriesCountPing
}

type InsightsPerDashboardPing struct {
	Avg    float32
	Max    int
	Min    int
	StdDev float32
	Median float32
}

type InsightsBackfillTimePing struct {
	AllRepos   bool
	P99Seconds int
	P90Seconds int
	P50Seconds int
	Count      int
}

type CodeMonitoringUsageStatistics struct {
	CodeMonitoringPageViews                       *int32
	CreateCodeMonitorPageViews                    *int32
	CreateCodeMonitorPageViewsWithTriggerQuery    *int32
	CreateCodeMonitorPageViewsWithoutTriggerQuery *int32
	ManageCodeMonitorPageViews                    *int32
	CodeMonitorEmailLinkClicked                   *int32
	ExampleMonitorClicked                         *int32
	GettingStartedPageViewed                      *int32
	CreateFormSubmitted                           *int32
	ManageFormSubmitted                           *int32
	ManageDeleteSubmitted                         *int32
	LogsPageViewed                                *int32
	EmailActionsTriggered                         *int32
	EmailActionsErrored                           *int32
	EmailActionsTriggeredUniqueUsers              *int32
	EmailActionsEnabled                           *int32
	EmailActionsEnabledUniqueUsers                *int32
	SlackActionsTriggered                         *int32
	SlackActionsErrored                           *int32
	SlackActionsTriggeredUniqueUsers              *int32
	SlackActionsEnabled                           *int32
	SlackActionsEnabledUniqueUsers                *int32
	WebhookActionsTriggered                       *int32
	WebhookActionsErrored                         *int32
	WebhookActionsTriggeredUniqueUsers            *int32
	WebhookActionsEnabled                         *int32
	WebhookActionsEnabledUniqueUsers              *int32
	MonitorsEnabled                               *int32
	MonitorsEnabledUniqueUsers                    *int32
	// (TODO @jasonhawkharris ) Currently, MonitorsEnabledLastRunErrored is unpopulated
	// It will require adjusting the query to select a row inside of a group
	MonitorsEnabledLastRunErrored *int32
	ReposMonitored                *int32
	TriggerRuns                   *int32
	TriggerRunsErrored            *int32
	P50TriggerRunTimeSeconds      *float32
	P90TriggerRunTimeSeconds      *float32
}

type NotebooksUsageStatistics struct {
	NotebookPageViews                *int32
	EmbeddedNotebookPageViews        *int32
	NotebooksListPageViews           *int32
	NotebooksCreatedCount            *int32
	NotebookAddedStarsCount          *int32
	NotebookAddedMarkdownBlocksCount *int32
	NotebookAddedQueryBlocksCount    *int32
	NotebookAddedFileBlocksCount     *int32
	NotebookAddedSymbolBlocksCount   *int32
}

type OwnershipUsageStatistics struct {
	// Statistics about ownership data in repositories
	ReposCount *OwnershipUsageReposCounts `json:"repos_count,omitempty"`

	// Activity of selecting owners as search results using
	// `select:file.owners`.
	SelectFileOwnersSearch *OwnershipUsageStatisticsActiveUsers `json:"select_file_owners_search,omitempty"`

	// Activity of using a `file:has.owner` predicate in search.
	FileHasOwnerSearch *OwnershipUsageStatisticsActiveUsers `json:"file_has_owner_search,omitempty"`

	// Opening ownership panel.
	OwnershipPanelOpened *OwnershipUsageStatisticsActiveUsers `json:"ownership_panel_opened,omitempty"`

	// AssignedOwnersCount is the total number of assigned owners. For instance
	// if an owner is assigned to a single file - that counts as one,
	// for the whole repo - also counts as one.
	AssignedOwnersCount *int32 `json:"assigned_owners_count"`
}

type OwnershipUsageReposCounts struct {
	// Total number of repositories. Can be used in computing adoption
	// ratio as denominator to number of repos with ownership.
	Total *int32 `json:"total,omitempty"`

	// Number of repos in an instance that have ownership
	// data (of any source, either CODEOWNERS file or API).
	WithOwnership *int32 `json:"with_ownership,omitempty"`

	// Number of repos in an instance that have ownership
	// data ingested through the API.
	WithIngestedOwnership *int32 `json:"with_ingested_ownership,omitempty"`
}

type OwnershipUsageStatisticsActiveUsers struct {
	// Daily-Active Users
	DAU *int32 `json:"dau,omitempty"`

	// Weekly-Active Users
	WAU *int32 `json:"wau,omitempty"`

	// Monthly-Active Users
	MAU *int32 `json:"mau,omitempty"`
}

// Secret represents the secrets table
type Secret struct {
	ID int32

	// The table containing an object whose token is being encrypted.
	SourceType sql.NullString

	// The ID of the object in the SourceType table.
	SourceID sql.NullInt32

	// KeyName represents a unique key for the case where we're storing key-value pairs.
	KeyName sql.NullString

	// Value contains the encrypted string
	Value string
}

type SearchContext struct {
	ID int64
	// Name contains the non-prefixed part of the search context spec.
	// The name is a substring of the spec and it should NOT be used as the spec itself.
	// The spec contains additional information (such as the @ prefix and the context namespace)
	// that helps differentiate between different search contexts.
	// Example mappings from context spec to context name:
	// global -> global, @user -> user, @org -> org,
	// @user/ctx1 -> ctx1, @org/ctx2 -> ctx2.
	Name        string
	Description string
	// Public property controls the visibility of the search context. Public search context is available to
	// any user on the instance. If a public search context contains private repositories, those are filtered out
	// for unauthorized users. Private search contexts are only available to their owners. Private user search context
	// is available only to the user, private org search context is available only to the members of the org, and private
	// instance-level search contexts is available only to site-admins.
	Public          bool
	NamespaceUserID int32 // if non-zero, the owner is this user. NamespaceUserID/NamespaceOrgID are mutually exclusive.
	NamespaceOrgID  int32 // if non-zero, the owner is this organization. NamespaceUserID/NamespaceOrgID are mutually exclusive.
	UpdatedAt       time.Time

	// We cache namespace names to avoid separate database lookups when constructing the search context spec

	// NamespaceUserName is the name of the user if NamespaceUserID is present.
	NamespaceUserName string
	// NamespaceOrgName is the name of the org if NamespaceOrgID is present.
	NamespaceOrgName string

	// Query is the Sourcegraph query that defines this search context
	// e.g. repo:^github\.com/org rev:bar archive:no f:sub/dir
	Query string

	// Whether the search context is auto-defined by Sourcegraph. Auto-defined search contexts are not editable by users.
	AutoDefined bool

	// Whether the search context is the default for the user. If the user hasn't explicitly set a default or is not authenticated, the global search context is used.
	Default bool

	// Whether the user has starred the context. If the user is not authenticated, this field is always false.
	Starred bool
}

// SearchContextRepositoryRevisions is a simple wrapper for a repository and its revisions
// contained in a search context. It is made compatible with search.RepositoryRevisions, so it can be easily
// converted when needed. We could use search.RepositoryRevisions directly instead, but it
// introduces an import cycle with `internal/vcs/git` package when used in `internal/database/search_contexts.go`.
type SearchContextRepositoryRevisions struct {
	Repo      MinimalRepo
	Revisions []string
}

type EncryptableSecret = encryption.Encryptable

// NewUnencryptedSecret creates an EncryptableSecret that *may* be encrypted in
// the future, but the current value has not yet been encrypted.
func NewUnencryptedSecret(value string) *EncryptableSecret {
	return encryption.NewUnencrypted(value)
}

// NewEncryptedSecret creates an EncryptableSecret that has come from an
// encrypted source. In this case you need to provide the keyID and key in order
// to be able to decrypt it.
func NewEncryptedSecret(cipher, keyID string, key encryption.Key) *EncryptableSecret {
	return encryption.NewEncrypted(cipher, keyID, key)
}

// Webhook defines the information we need to handle incoming webhooks from a
// code host.
type Webhook struct {
	// The primary key, used for sorting and pagination.
	ID int32
	// UUID is the ID we display externally and will appear in the webhook URL.
	UUID uuid.UUID
	// Name is a descriptive webhook name which is shown on the UI for convenience.
	Name         string
	CodeHostKind string
	CodeHostURN  extsvc.CodeHostBaseURL
	// Secret can be in one of three states:
	//
	// 1. nil, no secret provided.
	// 2. Provided but not encrypted.
	// 3. Provided and encrypted.
	//
	// For 2 and 3 you interact with it in the same way and just assume that it IS
	// encrypted. All the methods on EncryptableSecret will just pass around the raw
	// value and encryption / decryption methods are noops.
	Secret          *EncryptableSecret
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CreatedByUserID int32
	UpdatedByUserID int32
}

// OutboundRequestLogItem represents a single outbound request made by Sourcegraph.
type OutboundRequestLogItem struct {
	ID                 string              `json:"id"`
	StartedAt          time.Time           `json:"startedAt"`
	Method             string              `json:"method"` // The request method (GET, POST, etc.)
	URL                string              `json:"url"`
	RequestHeaders     map[string][]string `json:"requestHeaders"`
	RequestBody        string              `json:"requestBody"`
	StatusCode         int32               `json:"statusCode"` // The response status code
	ResponseHeaders    map[string][]string `json:"responseHeaders"`
	Duration           float64             `json:"duration"`
	ErrorMessage       string              `json:"errorMessage"`
	CreationStackFrame string              `json:"creationStackFrame"`
	CallStackFrame     string              `json:"callStackFrame"` // Should be "CallStack" once this is final
}

type SlowRequest struct {
	Index     string         `json:"index"`
	Start     time.Time      `json:"start"`
	Duration  time.Duration  `json:"duration"`
	UserID    int32          `json:"userId"`
	Name      string         `json:"name"`
	Source    string         `json:"source"`
	Variables map[string]any `json:"variables"`
	Errors    []string       `json:"errors"`
	Query     string         `json:"query"`
	Filepath  string         `json:"filepath"`
}

type Team struct {
	ID           int32
	Name         string
	DisplayName  string
	ReadOnly     bool
	ParentTeamID int32
	CreatorID    int32
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type TeamMember struct {
	UserID    int32
	TeamID    int32
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AccessRequestStatus string

type AccessRequest struct {
	ID               int32
	Name             string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Email            string
	AdditionalInfo   string
	Status           AccessRequestStatus
	DecisionByUserID *int32
}

const (
	AccessRequestStatusPending  AccessRequestStatus = "PENDING"
	AccessRequestStatusApproved AccessRequestStatus = "APPROVED"
	AccessRequestStatusRejected AccessRequestStatus = "REJECTED"
	AccessRequestStatusCanceled AccessRequestStatus = "CANCELED"
)

type PerforceChangelist struct {
	CommitSHA    api.CommitID
	ChangelistID int64
}

// CodeHost represents a signle code source, usually defined by url e.g. github.com, gitlab.com, bitbucket.sgdev.org.
type CodeHost struct {
	ID                          int32
	Kind                        string
	URL                         string
	APIRateLimitQuota           *int32
	APIRateLimitIntervalSeconds *int32
	GitRateLimitQuota           *int32
	GitRateLimitIntervalSeconds *int32
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

// RepoSyncDiff is the difference found by a sync between what is in the store and
// what is returned from sources.
type RepoSyncDiff struct {
	Added      Repos
	Deleted    Repos
	Modified   ReposModified
	Unmodified Repos
}

// Sort sorts all Diff elements by Repo.IDs.
func (d *RepoSyncDiff) Sort() {
	for _, ds := range []Repos{
		d.Added,
		d.Deleted,
		d.Modified.Repos(),
		d.Unmodified,
	} {
		sort.Sort(ds)
	}
}

// Repos returns all repos in the Diff.
func (d *RepoSyncDiff) Repos() Repos {
	all := make(Repos, 0, len(d.Added)+
		len(d.Deleted)+
		len(d.Modified)+
		len(d.Unmodified))

	for _, rs := range []Repos{
		d.Added,
		d.Deleted,
		d.Modified.Repos(),
		d.Unmodified,
	} {
		all = append(all, rs...)
	}

	return all
}

func (d *RepoSyncDiff) Len() int {
	return len(d.Deleted) + len(d.Modified) + len(d.Added) + len(d.Unmodified)
}

// RepoModified tracks the modifications applied to a single repository after a
// sync.
type RepoModified struct {
	Repo     *Repo
	Modified RepoModifiedFields
}

type ReposModified []RepoModified

// Repos returns all modified repositories.
func (rm ReposModified) Repos() Repos {
	repos := make(Repos, len(rm))
	for i := range rm {
		repos[i] = rm[i].Repo
	}

	return repos
}

// ReposModified returns only the repositories that had a specific field
// modified in the sync.
func (rm ReposModified) ReposModified(modified RepoModifiedFields) Repos {
	repos := Repos{}
	for _, pair := range rm {
		if pair.Modified&modified == modified {
			repos = append(repos, pair.Repo)
		}
	}

	return repos
}
