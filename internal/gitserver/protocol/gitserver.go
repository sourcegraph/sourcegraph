package protocol

import (
	"encoding/json"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type SearchRequest struct {
	Repo                 api.RepoName
	Revisions            []string
	Query                Node
	IncludeDiff          bool
	Limit                int
	IncludeModifiedFiles bool
}

func (r *SearchRequest) ToProto() *proto.SearchRequest {
	revs := make([]*proto.RevisionSpecifier, 0, len(r.Revisions))
	for _, rev := range r.Revisions {
		revs = append(revs, &proto.RevisionSpecifier{RevSpec: rev})
	}
	return &proto.SearchRequest{
		Repo:                 string(r.Repo),
		Revisions:            revs,
		Query:                r.Query.ToProto(),
		IncludeDiff:          r.IncludeDiff,
		Limit:                int64(r.Limit),
		IncludeModifiedFiles: r.IncludeModifiedFiles,
	}
}

func SearchRequestFromProto(p *proto.SearchRequest) (*SearchRequest, error) {
	query, err := NodeFromProto(p.GetQuery())
	if err != nil {
		return nil, err
	}

	revisions := make([]string, 0, len(p.GetRevisions()))
	for _, rev := range p.GetRevisions() {
		revisions = append(revisions, rev.GetRevSpec())
	}

	return &SearchRequest{
		Repo:                 api.RepoName(p.GetRepo()),
		Revisions:            revisions,
		Query:                query,
		IncludeDiff:          p.GetIncludeDiff(),
		Limit:                int(p.GetLimit()),
		IncludeModifiedFiles: p.GetIncludeModifiedFiles(),
	}, nil
}

type SearchEventMatches []CommitMatch

type SearchEventDone struct {
	LimitHit bool
	Error    string
}

func (s SearchEventDone) Err() error {
	if s.Error != "" {
		var e gitdomain.RepoNotExistError
		if err := json.Unmarshal([]byte(s.Error), &e); err == nil {
			return &e
		}
		return errors.New(s.Error)
	}
	return nil
}

func NewSearchEventDone(limitHit bool, err error) SearchEventDone {
	event := SearchEventDone{
		LimitHit: limitHit,
	}
	var notExistError *gitdomain.RepoNotExistError
	if errors.As(err, &notExistError) {
		b, _ := json.Marshal(notExistError)
		event.Error = string(b)
	} else if err != nil {
		event.Error = err.Error()
	}
	return event
}

type CommitMatch struct {
	Oid        api.CommitID
	Author     Signature      `json:",omitempty"`
	Committer  Signature      `json:",omitempty"`
	Parents    []api.CommitID `json:",omitempty"`
	Refs       []string       `json:",omitempty"`
	SourceRefs []string       `json:",omitempty"`

	Message       result.MatchedString `json:",omitempty"`
	Diff          result.MatchedString `json:",omitempty"`
	ModifiedFiles []string             `json:",omitempty"`
}

func (cm *CommitMatch) ToProto() *proto.CommitMatch {
	parents := make([]string, 0, len(cm.Parents))
	for _, parent := range cm.Parents {
		parents = append(parents, string(parent))
	}
	return &proto.CommitMatch{
		Oid:           string(cm.Oid),
		Author:        cm.Author.ToProto(),
		Committer:     cm.Committer.ToProto(),
		Parents:       parents,
		Refs:          cm.Refs,
		SourceRefs:    cm.SourceRefs,
		Message:       matchedStringToProto(cm.Message),
		Diff:          matchedStringToProto(cm.Diff),
		ModifiedFiles: cm.ModifiedFiles,
	}
}

func CommitMatchFromProto(p *proto.CommitMatch) CommitMatch {
	parents := make([]api.CommitID, 0, len(p.GetParents()))
	for _, parent := range p.GetParents() {
		parents = append(parents, api.CommitID(parent))
	}
	return CommitMatch{
		Oid:           api.CommitID(p.GetOid()),
		Author:        SignatureFromProto(p.GetAuthor()),
		Committer:     SignatureFromProto(p.GetCommitter()),
		Parents:       parents,
		Refs:          p.GetRefs(),
		SourceRefs:    p.GetSourceRefs(),
		Message:       matchedStringFromProto(p.GetMessage()),
		Diff:          matchedStringFromProto(p.GetDiff()),
		ModifiedFiles: p.GetModifiedFiles(),
	}
}

func matchedStringFromProto(p *proto.CommitMatch_MatchedString) result.MatchedString {
	ranges := make([]result.Range, 0, len(p.GetRanges()))
	for _, rr := range p.GetRanges() {
		ranges = append(ranges, rangeFromProto(rr))
	}
	return result.MatchedString{
		Content:       p.GetContent(),
		MatchedRanges: ranges,
	}
}

func matchedStringToProto(ms result.MatchedString) *proto.CommitMatch_MatchedString {
	rrs := make([]*proto.CommitMatch_Range, 0, len(ms.MatchedRanges))
	for _, rr := range ms.MatchedRanges {
		rrs = append(rrs, rangeToProto(rr))
	}
	return &proto.CommitMatch_MatchedString{
		Content: ms.Content,
		Ranges:  rrs,
	}
}

func rangeToProto(r result.Range) *proto.CommitMatch_Range {
	return &proto.CommitMatch_Range{
		Start: locationToProto(r.Start),
		End:   locationToProto(r.End),
	}
}

func rangeFromProto(p *proto.CommitMatch_Range) result.Range {
	return result.Range{
		Start: locationFromProto(p.GetStart()),
		End:   locationFromProto(p.GetEnd()),
	}
}

func locationToProto(l result.Location) *proto.CommitMatch_Location {
	return &proto.CommitMatch_Location{
		Offset: uint32(l.Offset),
		Line:   uint32(l.Line),
		Column: uint32(l.Column),
	}
}

func locationFromProto(p *proto.CommitMatch_Location) result.Location {
	return result.Location{
		Offset: int(p.GetOffset()),
		Line:   int(p.GetLine()),
		Column: int(p.GetColumn()),
	}
}

type Signature struct {
	Name  string `json:",omitempty"`
	Email string `json:",omitempty"`
	Date  time.Time
}

func (s *Signature) ToProto() *proto.CommitMatch_Signature {
	return &proto.CommitMatch_Signature{
		Name:  s.Name,
		Email: s.Email,
		Date:  timestamppb.New(s.Date),
	}
}

func SignatureFromProto(p *proto.CommitMatch_Signature) Signature {
	return Signature{
		Name:  p.GetName(),
		Email: p.GetEmail(),
		Date:  p.GetDate().AsTime(),
	}
}

// ExecRequest is a request to execute a command inside a git repository.
//
// Note that this request is deserialized by both gitserver and the frontend's
// internal proxy route and any major change to this structure will need to
// be reconciled in both places.
type ExecRequest struct {
	Repo      api.RepoName `json:"repo"`
	Args      []string     `json:"args"`
	NoTimeout bool         `json:"noTimeout"`
}

// RepoUpdateRequest is a request to update the contents of a given repo, or clone it if it doesn't exist.
type RepoUpdateRequest struct {
	// Repo identifies URL for repo.
	Repo api.RepoName `json:"repo"`
}

func (r *RepoUpdateRequest) ToProto() *proto.RepoUpdateRequest {
	return &proto.RepoUpdateRequest{
		Repo: string(r.Repo),
	}
}

func (r *RepoUpdateRequest) FromProto(p *proto.RepoUpdateRequest) {
	*r = RepoUpdateRequest{
		Repo: api.RepoName(p.GetRepo()),
	}
}

// RepoUpdateResponse returns meta information of the repo enqueued for update.
type RepoUpdateResponse struct {
	LastFetched *time.Time `json:",omitempty"`
	LastChanged *time.Time `json:",omitempty"`

	// Error is an error reported by the update operation, and not a network protocol error.
	Error string `json:",omitempty"`
}

func (r *RepoUpdateResponse) ToProto() *proto.RepoUpdateResponse {
	var lastFetched, lastChanged *timestamppb.Timestamp
	if r.LastFetched != nil {
		lastFetched = timestamppb.New(*r.LastFetched)
	}

	if r.LastChanged != nil {
		lastChanged = timestamppb.New(*r.LastChanged)
	}

	return &proto.RepoUpdateResponse{
		LastFetched: timestamppb.New(lastFetched.AsTime()),
		LastChanged: timestamppb.New(lastChanged.AsTime()),
		Error:       r.Error,
	}
}

func (r *RepoUpdateResponse) FromProto(p *proto.RepoUpdateResponse) {
	var lastFetched, lastChanged time.Time
	if p.GetLastFetched() != nil {
		lf := p.GetLastFetched().AsTime()
		lastFetched = lf
	} else {
		lastFetched = time.Time{}
	}

	if p.GetLastChanged() != nil {
		lc := p.GetLastChanged().AsTime()
		lastChanged = lc
	} else {
		lastChanged = time.Time{}
	}

	*r = RepoUpdateResponse{
		LastFetched: &lastFetched,
		LastChanged: &lastChanged,
		Error:       p.GetError(),
	}
}

// RepoCloneRequest is a request to clone a repository asynchronously.
type RepoCloneRequest struct {
	Repo api.RepoName `json:"repo"`
}

// RepoCloneResponse returns an error if the repo clone request failed.
type RepoCloneResponse struct {
	Error string `json:",omitempty"`
}

func (r *RepoCloneResponse) ToProto() *proto.RepoCloneResponse {
	return &proto.RepoCloneResponse{
		Error: r.Error,
	}
}

func (r *RepoCloneResponse) FromProto(p *proto.RepoCloneResponse) {
	*r = RepoCloneResponse{
		Error: p.GetError(),
	}
}

type NotFoundPayload struct {
	CloneInProgress bool `json:"cloneInProgress"` // If true, exec returned with noop because clone is in progress.

	// CloneProgress is a progress message from the running clone command.
	CloneProgress string `json:"cloneProgress,omitempty"`
}

// IsRepoCloneableRequest is a request to determine if a repo is cloneable.
type IsRepoCloneableRequest struct {
	// Repo is the repository to check.
	Repo api.RepoName `json:"Repo"`
}

// IsRepoCloneableResponse is the response type for the IsRepoCloneableRequest.
type IsRepoCloneableResponse struct {
	Cloneable bool   // whether the repo is cloneable
	Cloned    bool   // true if the repo was ever cloned in the past
	Reason    string // if not cloneable, the reason why not
}

func (i *IsRepoCloneableResponse) ToProto() *proto.IsRepoCloneableResponse {
	return &proto.IsRepoCloneableResponse{
		Cloneable: i.Cloneable,
		Cloned:    i.Cloned,
		Reason:    i.Reason,
	}
}

func (i *IsRepoCloneableResponse) FromProto(p *proto.IsRepoCloneableResponse) {
	*i = IsRepoCloneableResponse{
		Cloneable: p.GetCloneable(),
		Cloned:    p.GetCloned(),
		Reason:    p.GetReason(),
	}
}

// RepoDeleteRequest is a request to delete a repository clone on gitserver
type RepoDeleteRequest struct {
	// Repo is the repository to delete.
	Repo api.RepoName
}

// RepoCloneProgress is information about the clone progress of a repo
type RepoCloneProgress struct {
	CloneInProgress bool   // whether the repository is currently being cloned
	CloneProgress   string // a progress message from the running clone command.
	Cloned          bool   // whether the repository has been cloned successfully
}

func (r *RepoCloneProgress) ToProto() *proto.RepoCloneProgressResponse {
	return &proto.RepoCloneProgressResponse{
		CloneInProgress: r.CloneInProgress,
		CloneProgress:   r.CloneProgress,
		Cloned:          r.Cloned,
	}
}

func (r *RepoCloneProgress) FromProto(p *proto.RepoCloneProgressResponse) {
	*r = RepoCloneProgress{
		CloneInProgress: p.GetCloneInProgress(),
		CloneProgress:   p.GetCloneProgress(),
		Cloned:          p.GetCloned(),
	}
}

// CreateCommitFromPatchRequest is the request information needed for creating
// the simulated staging area git object for a repo.
type CreateCommitFromPatchRequest struct {
	// Repo is the repository to get information about.
	Repo api.RepoName
	// BaseCommit is the revision that the staging area object is based on
	BaseCommit api.CommitID
	// Patch is the diff contents to be used to create the staging area revision
	Patch []byte
	// TargetRef is the ref that will be created for this patch
	TargetRef string
	// If set to true and the TargetRef already exists, an unique number will be appended to the end (ie TargetRef-{#}). The generated ref will be returned.
	UniqueRef bool
	// CommitInfo is the information that will be used when creating the commit from a patch
	CommitInfo PatchCommitInfo
	// Push specifies whether the target ref will be pushed to the code host: if
	// nil, no push will be attempted, if non-nil, a push will be attempted.
	Push *PushConfig
	// GitApplyArgs are the arguments that will be passed to `git apply` along
	// with `--cached`.
	GitApplyArgs []string
	// If specified, the changes will be pushed to this ref as opposed to TargetRef.
	PushRef *string
}

func (c *CreateCommitFromPatchRequest) ToMetadataProto() *proto.CreateCommitFromPatchBinaryRequest_Metadata {
	cc := &proto.CreateCommitFromPatchBinaryRequest_Metadata{
		Repo:         string(c.Repo),
		BaseCommit:   string(c.BaseCommit),
		TargetRef:    c.TargetRef,
		UniqueRef:    c.UniqueRef,
		CommitInfo:   c.CommitInfo.ToProto(),
		GitApplyArgs: c.GitApplyArgs,
		PushRef:      c.PushRef,
	}

	if c.Push != nil {
		cc.Push = c.Push.ToProto()
	}

	return cc
}

func (c *CreateCommitFromPatchRequest) FromProto(p *proto.CreateCommitFromPatchBinaryRequest_Metadata) {
	gp := p.GetPush()
	var pushConfig *PushConfig
	if gp != nil {
		pushConfig = &PushConfig{}
		pushConfig.FromProto(gp)
	}

	*c = CreateCommitFromPatchRequest{
		Repo:         api.RepoName(p.GetRepo()),
		BaseCommit:   api.CommitID(p.GetBaseCommit()),
		TargetRef:    p.GetTargetRef(),
		UniqueRef:    p.GetUniqueRef(),
		CommitInfo:   PatchCommitInfoFromProto(p.GetCommitInfo()),
		Push:         pushConfig,
		GitApplyArgs: p.GetGitApplyArgs(),
	}

	if p != nil {
		c.PushRef = p.PushRef
	}
}

// PatchCommitInfo will be used for commit information when creating a commit from a patch
type PatchCommitInfo struct {
	Messages       []string
	AuthorName     string
	AuthorEmail    string
	CommitterName  string
	CommitterEmail string
	Date           time.Time
}

func (p *PatchCommitInfo) ToProto() *proto.PatchCommitInfo {
	return &proto.PatchCommitInfo{
		Messages:       p.Messages,
		AuthorName:     p.AuthorName,
		AuthorEmail:    p.AuthorEmail,
		CommitterName:  p.CommitterName,
		CommitterEmail: p.CommitterEmail,
		Date:           timestamppb.New(p.Date),
	}
}

func PatchCommitInfoFromProto(p *proto.PatchCommitInfo) PatchCommitInfo {
	return PatchCommitInfo{
		Messages:       p.GetMessages(),
		AuthorName:     p.GetAuthorName(),
		AuthorEmail:    p.GetAuthorEmail(),
		CommitterName:  p.GetCommitterName(),
		CommitterEmail: p.GetCommitterEmail(),
		Date:           p.GetDate().AsTime(),
	}
}

// PushConfig provides the configuration required to push one or more commits to
// a code host.
type PushConfig struct {
	// RemoteURL is the git remote URL to which to push the commits.
	// The URL needs to include HTTP basic auth credentials if no
	// unauthenticated requests are allowed by the remote host.
	RemoteURL string

	// PrivateKey is used when the remote URL uses scheme `ssh`. If set,
	// this value is used as the content of the private key. Needs to be
	// set in conjunction with a passphrase.
	PrivateKey string

	// Passphrase is the passphrase to decrypt the private key. It is required
	// when passing PrivateKey.
	Passphrase string
}

func (p *PushConfig) ToProto() *proto.PushConfig {
	if p == nil {
		return nil
	}

	return &proto.PushConfig{
		RemoteUrl:  p.RemoteURL,
		PrivateKey: p.PrivateKey,
		Passphrase: p.Passphrase,
	}
}

func (pc *PushConfig) FromProto(p *proto.PushConfig) {
	*pc = PushConfig{
		RemoteURL:  p.GetRemoteUrl(),
		PrivateKey: p.GetPrivateKey(),
		Passphrase: p.GetPassphrase(),
	}
}

type GerritConfig struct {
	ChangeID     string
	PushMagicRef string
}

// CreateCommitFromPatchResponse is the response type returned after creating
// a commit from a patch
type CreateCommitFromPatchResponse struct {
	// Rev is the tag that the staging object can be found at
	Rev string

	// Error is populated only on error
	Error *CreateCommitFromPatchError

	// ChangelistId is the numeric ID of the changelist that is shelved for the patch.
	// only supplied for Perforce code hosts.
	// it's a string because it's optional, but usng a scalar pointer is not allowed in protobuf
	// so blank string means not provided
	ChangelistId string
}

func (r *CreateCommitFromPatchResponse) ToProto() (*proto.CreateCommitFromPatchBinaryResponse, *proto.CreateCommitFromPatchError) {
	res := &proto.CreateCommitFromPatchBinaryResponse{
		Rev:          r.Rev,
		ChangelistId: r.ChangelistId,
	}

	if r.Error != nil {
		return res, r.Error.ToProto()
	}

	return res, nil
}

func (r *CreateCommitFromPatchResponse) FromProto(res *proto.CreateCommitFromPatchBinaryResponse, err *proto.CreateCommitFromPatchError) {
	if err == nil {
		r.Error = nil
	} else {
		r.Error = &CreateCommitFromPatchError{}
		r.Error.FromProto(err)
	}
	r.Rev = res.GetRev()
	r.ChangelistId = res.ChangelistId
}

// SetError adds the supplied error related details to e.
func (e *CreateCommitFromPatchResponse) SetError(repo, command, out string, err error) {
	if e.Error == nil {
		e.Error = &CreateCommitFromPatchError{}
	}
	e.Error.RepositoryName = repo
	e.Error.Command = command
	e.Error.CombinedOutput = out
	e.Error.InternalError = err.Error()
}

// CreateCommitFromPatchError is populated on errors running
// CreateCommitFromPatch
type CreateCommitFromPatchError struct {
	// RepositoryName is the name of the repository
	RepositoryName string

	// InternalError is the internal error
	InternalError string

	// Command is the last git command that was attempted
	Command string
	// CombinedOutput is the combined stderr and stdout from running the command
	CombinedOutput string
}

func (e *CreateCommitFromPatchError) ToProto() *proto.CreateCommitFromPatchError {
	return &proto.CreateCommitFromPatchError{
		RepositoryName: e.RepositoryName,
		InternalError:  e.InternalError,
		Command:        e.Command,
		CombinedOutput: e.CombinedOutput,
	}
}

func (e *CreateCommitFromPatchError) FromProto(p *proto.CreateCommitFromPatchError) {
	*e = CreateCommitFromPatchError{
		RepositoryName: p.GetRepositoryName(),
		InternalError:  p.GetInternalError(),
		Command:        p.GetCommand(),
		CombinedOutput: p.GetCombinedOutput(),
	}
}

// Error returns a detailed error conforming to the error interface
func (e *CreateCommitFromPatchError) Error() string {
	return e.InternalError
}

type GetObjectRequest struct {
	Repo       api.RepoName
	ObjectName string
}

func (r *GetObjectRequest) ToProto() *proto.GetObjectRequest {
	return &proto.GetObjectRequest{
		Repo:       string(r.Repo),
		ObjectName: r.ObjectName,
	}
}

func (r *GetObjectRequest) FromProto(p *proto.GetObjectRequest) {
	*r = GetObjectRequest{
		Repo:       api.RepoName(p.GetRepo()),
		ObjectName: p.GetObjectName(),
	}
}

type GetObjectResponse struct {
	Object gitdomain.GitObject
}

func (r *GetObjectResponse) ToProto() *proto.GetObjectResponse {
	return &proto.GetObjectResponse{
		Object: r.Object.ToProto(),
	}
}

func (r *GetObjectResponse) FromProto(p *proto.GetObjectResponse) {
	obj := p.GetObject()

	var gitObj gitdomain.GitObject
	gitObj.FromProto(obj)
	*r = GetObjectResponse{
		Object: gitObj,
	}

}

// IsPerforcePathCloneableRequest is the request to check if a Perforce path is cloneable.
type IsPerforcePathCloneableRequest struct {
	P4Port    string `json:"p4port"`
	P4User    string `json:"p4user"`
	P4Passwd  string `json:"p4passwd"`
	DepotPath string `json:"depotPath"`
}

// IsPerforcePathCloneableResponse is the response from checking if a Perforce path is cloneable.
type IsPerforcePathCloneableResponse struct{}

// CheckPerforceCredentialsRequest is the request to check if given Perforce credentials are valid.
type CheckPerforceCredentialsRequest struct {
	P4Port   string `json:"p4port"`
	P4User   string `json:"p4user"`
	P4Passwd string `json:"p4passwd"`
}

// IsPerforcePathCloneableResponse is the response from checking if given Perforce credentials are valid.
type CheckPerforceCredentialsResponse struct{}

// PerforceConnectionDetails holds all the details required to talk to a Perforce server.
type PerforceConnectionDetails struct {
	P4Port   string
	P4User   string
	P4Passwd string
}

func (c PerforceConnectionDetails) ToProto() *proto.PerforceConnectionDetails {
	return &proto.PerforceConnectionDetails{
		P4Port:   c.P4Port,
		P4User:   c.P4User,
		P4Passwd: c.P4Passwd,
	}
}

// SystemInfo holds info on a Gitserver instance.
type SystemInfo struct {
	Address     string
	FreeSpace   uint64
	TotalSpace  uint64
	PercentUsed float32
}

type PerforceUsersRequest struct {
	P4Port   string `json:"p4port"`
	P4User   string `json:"p4user"`
	P4Passwd string `json:"p4passwd"`
}

type PerforceUsersResponse struct {
	Users []PerforceUser `json:"users"`
}

type PerforceUser struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

type PerforceProtectsForUserRequest struct {
	P4Port   string `json:"p4port"`
	P4User   string `json:"p4user"`
	P4Passwd string `json:"p4passwd"`
	Username string `json:"username"`
}

type PerforceProtectsForUserResponse struct {
	Protects []PerforceProtect `json:"protects"`
}

type PerforceProtect struct {
	Level       string `json:"level"`
	EntityType  string `json:"entityType"`
	EntityName  string `json:"entityName"`
	Match       string `json:"match"`
	IsExclusion bool   `json:"isExclusion"`
	Host        string `json:"host"`
}

type PerforceProtectsForDepotRequest struct {
	P4Port   string `json:"p4port"`
	P4User   string `json:"p4user"`
	P4Passwd string `json:"p4passwd"`
	Depot    string `json:"depot"`
}

type PerforceProtectsForDepotResponse struct {
	Protects []PerforceProtect `json:"protects"`
}

type PerforceGroupMembersRequest struct {
	P4Port   string `json:"p4port"`
	P4User   string `json:"p4user"`
	P4Passwd string `json:"p4passwd"`
	Group    string `json:"group"`
}

type PerforceGroupMembersResponse struct {
	Usernames []string `json:"usernames"`
}

type IsPerforceSuperUserRequest struct {
	P4Port   string `json:"p4port"`
	P4User   string `json:"p4user"`
	P4Passwd string `json:"p4passwd"`
}

type IsPerforceSuperUserResponse struct {
}

type PerforceGetChangelistRequest struct {
	P4Port       string `json:"p4port"`
	P4User       string `json:"p4user"`
	P4Passwd     string `json:"p4passwd"`
	ChangelistID string `json:"changelistID"`
}

type PerforceGetChangelistResponse struct {
	Changelist PerforceChangelist `json:"changelist"`
}

type PerforceChangelist struct {
	ID           string    `json:"id"`
	CreationDate time.Time `json:"creationDate"`
	State        string    `json:"state"`
	Author       string    `json:"author"`
	Title        string    `json:"title"`
	Message      string    `json:"message"`
}
