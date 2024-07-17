package gitdomain

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/gobwas/glob"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sourcegraph/sourcegraph/internal/fileutil"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	v1 "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func NewOID(sha api.CommitID) (OID, error) {
	oidBytes, err := hex.DecodeString(string(sha))
	if err != nil {
		return OID{}, err
	}
	if len(oidBytes) != 20 {
		return OID{}, errors.Errorf("invalid OID length: %d", len(oidBytes))
	}
	var oid OID
	copy(oid[:], oidBytes)
	return oid, nil
}

// OID is a Git OID (40-char hex-encoded).
type OID [20]byte

func (oid OID) String() string { return hex.EncodeToString(oid[:]) }

// ObjectType is a valid Git object type (commit, tag, tree, and blob).
type ObjectType string

// Standard Git object types.
const (
	ObjectTypeCommit ObjectType = "commit"
	ObjectTypeTag    ObjectType = "tag"
	ObjectTypeTree   ObjectType = "tree"
	ObjectTypeBlob   ObjectType = "blob"
)

// ModeSubmodule is an os.FileMode mask indicating that the file is a Git submodule.
//
// To avoid being reported as a regular file mode by (os.FileMode).IsRegular, it sets other bits
// (os.ModeDevice) beyond the Git "160000" commit mode bits. The choice of os.ModeDevice is
// arbitrary.
const ModeSubmodule = 0o160000 | os.ModeDevice
const ModeSymlink = 0o20000

// Submodule holds information about a Git submodule and is
// returned in the FileInfo's Sys field by Stat/ReadDir calls.
type Submodule struct {
	// URL is the submodule repository clone URL.
	URL string

	// Path is the path of the submodule relative to the repository root.
	Path string

	// CommitID is the pinned commit ID of the submodule (in the
	// submodule repository's commit ID space).
	CommitID api.CommitID
}

// ObjectInfo holds information about a Git object and is returned in (fs.FileInfo).Sys for blobs
// and trees from Stat/ReadDir calls.
type ObjectInfo interface {
	OID() OID
}

// GitObject represents a GitObject
type GitObject struct {
	ID   OID
	Type ObjectType
}

func (o *GitObject) ToProto() *proto.GitObject {
	var id []byte
	if o.ID != (OID{}) {
		id = o.ID[:]
	}

	var t proto.GitObject_ObjectType
	switch o.Type {
	case ObjectTypeCommit:
		t = proto.GitObject_OBJECT_TYPE_COMMIT
	case ObjectTypeTag:
		t = proto.GitObject_OBJECT_TYPE_TAG
	case ObjectTypeTree:
		t = proto.GitObject_OBJECT_TYPE_TREE
	case ObjectTypeBlob:
		t = proto.GitObject_OBJECT_TYPE_BLOB

	default:
		t = proto.GitObject_OBJECT_TYPE_UNSPECIFIED
	}

	return &proto.GitObject{
		Id:   id,
		Type: t,
	}
}

func (o *GitObject) FromProto(p *proto.GitObject) {
	id := p.GetId()
	var oid OID
	if len(id) == 20 {
		copy(oid[:], id)
	}

	var t ObjectType

	switch p.GetType() {
	case proto.GitObject_OBJECT_TYPE_COMMIT:
		t = ObjectTypeCommit
	case proto.GitObject_OBJECT_TYPE_TAG:
		t = ObjectTypeTag
	case proto.GitObject_OBJECT_TYPE_TREE:
		t = ObjectTypeTree
	case proto.GitObject_OBJECT_TYPE_BLOB:
		t = ObjectTypeBlob

	}

	*o = GitObject{
		ID:   oid,
		Type: t,
	}
}

// IsAbsoluteRevision checks if the revision is a git OID SHA string.
//
// Note: This doesn't mean the SHA exists in a repository, nor does it mean it
// isn't a ref. Git allows 40-char hexadecimal strings to be references.
func IsAbsoluteRevision(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, r := range s {
		if !(('0' <= r && r <= '9') ||
			('a' <= r && r <= 'f') ||
			('A' <= r && r <= 'F')) {
			return false
		}
	}
	return true
}

func EnsureAbsoluteCommit(commitID api.CommitID) error {
	// We don't want to even be running commands on non-absolute
	// commit IDs if we can avoid it, because we can't cache the
	// expensive part of those computations.
	if !IsAbsoluteRevision(string(commitID)) {
		return errors.Errorf("non-absolute commit ID: %q", commitID)
	}
	return nil
}

// Commit represents a git commit
type Commit struct {
	ID        api.CommitID `json:"ID,omitempty"`
	Author    Signature    `json:"Author"`
	Committer *Signature   `json:"Committer,omitempty"`
	Message   Message      `json:"Message,omitempty"`
	// Parents are the commit IDs of this commit's parent commits.
	Parents []api.CommitID `json:"Parents,omitempty"`
}

func (c *Commit) ToProto() *proto.GitCommit {
	parents := make([]string, len(c.Parents))
	for i, p := range c.Parents {
		parents[i] = string(p)
	}

	return &proto.GitCommit{
		Oid:     string(c.ID),
		Message: []byte(c.Message),
		Parents: parents,
		Author: &proto.GitSignature{
			Name:  []byte(c.Author.Name),
			Email: []byte(c.Author.Email),
			Date:  timestamppb.New(c.Author.Date),
		},
		Committer: &proto.GitSignature{
			Name:  []byte(c.Committer.Name),
			Email: []byte(c.Committer.Email),
			Date:  timestamppb.New(c.Committer.Date),
		},
	}
}

func CommitFromProto(p *proto.GitCommit) *Commit {
	parents := make([]api.CommitID, len(p.GetParents()))
	for i, p := range p.GetParents() {
		parents[i] = api.CommitID(p)
	}

	return &Commit{
		ID:      api.CommitID(p.GetOid()),
		Message: Message(p.GetMessage()),
		Author: Signature{
			// TODO@ggilmore: It's entirely possible that the "name" could include non-utf8 characters, as there is no such enforcement in the git cli. We should consider using []byte here.
			Name: string(p.GetAuthor().GetName()),
			// TODO@ggilmore: It's entirely possible that the "email" could include non-utf8 characters, as there is no such enforcement in the git cli. We should consider using []byte here.
			Email: string(p.GetAuthor().GetEmail()),
			Date:  p.GetAuthor().GetDate().AsTime(),
		},
		Committer: &Signature{
			// TODO@ggilmore: It's entirely possible that the "name" could include non-utf8 characters, as there is no such enforcement in the git cli. We should consider using []byte here.
			Name: string(p.GetCommitter().GetName()),
			// TODO@ggilmore: It's entirely possible that the "email" could include non-utf8 characters, as there is no such enforcement in the git cli. We should consider using []byte here.
			Email: string(p.GetCommitter().GetEmail()),
			Date:  p.GetCommitter().GetDate().AsTime(),
		},
		Parents: parents,
	}
}

// Message represents a git commit message
type Message string

// Subject returns the first line of the commit message
func (m Message) Subject() string {
	message := string(m)
	i := strings.Index(message, "\n")
	if i == -1 {
		return strings.TrimSpace(message)
	}
	return strings.TrimSpace(message[:i])
}

// Body returns the contents of the Git commit message after the subject.
func (m Message) Body() string {
	message := string(m)
	i := strings.Index(message, "\n")
	if i == -1 {
		return ""
	}
	return strings.TrimSpace(message[i:])
}

// PreviousCommit represents the previous commit a file was changed in.
type PreviousCommit struct {
	CommitID api.CommitID `json:"commitID"`
	Filename string       `json:"filename"`
}

// A Hunk is a contiguous portion of a file associated with a commit.
type Hunk struct {
	StartLine      uint32 // 1-indexed start line number
	EndLine        uint32 // 1-indexed end line number
	StartByte      uint32 // 0-indexed start byte position (inclusive)
	EndByte        uint32 // 0-indexed end byte position (exclusive)
	CommitID       api.CommitID
	PreviousCommit *PreviousCommit
	Author         Signature
	Message        string
	Filename       string
}

func HunkFromBlameProto(h *proto.BlameHunk) *Hunk {
	if h == nil {
		return nil
	}

	var previousCommit *PreviousCommit
	protoPreviousCommit := h.GetPreviousCommit()
	if protoPreviousCommit != nil {
		previousCommit = &PreviousCommit{
			CommitID: api.CommitID(protoPreviousCommit.GetCommit()),
			Filename: protoPreviousCommit.GetFilename(),
		}
	}

	return &Hunk{
		StartLine:      h.GetStartLine(),
		EndLine:        h.GetEndLine(),
		StartByte:      h.GetStartByte(),
		EndByte:        h.GetEndByte(),
		CommitID:       api.CommitID(h.GetCommit()),
		PreviousCommit: previousCommit,
		Message:        h.GetMessage(),
		Filename:       h.GetFilename(),
		Author: Signature{
			Name:  string(h.GetAuthor().GetName()), // Note: This is not necessarily a valid UTF-8 string, as git does not enforce this.
			Email: h.GetAuthor().GetEmail(),
			Date:  h.GetAuthor().GetDate().AsTime(),
		},
	}
}

func (h *Hunk) ToProto() *proto.BlameHunk {
	if h == nil {
		return nil
	}

	var protoPreviousCommit *proto.PreviousCommit
	if h.PreviousCommit != nil {
		protoPreviousCommit = &proto.PreviousCommit{
			Commit:   string(h.PreviousCommit.CommitID),
			Filename: h.PreviousCommit.Filename,
		}
	}
	return &proto.BlameHunk{
		StartLine:      uint32(h.StartLine),
		EndLine:        uint32(h.EndLine),
		StartByte:      uint32(h.StartByte),
		EndByte:        uint32(h.EndByte),
		Commit:         string(h.CommitID),
		PreviousCommit: protoPreviousCommit,
		Message:        h.Message,
		Filename:       h.Filename,
		Author: &proto.BlameAuthor{
			Name:  []byte(h.Author.Name), // We can't guarantee this is valid UTF-8. So, we have to use []byte.
			Email: h.Author.Email,
			Date:  timestamppb.New(h.Author.Date),
		},
	}
}

// Signature represents a commit signature
type Signature struct {
	// Name is the name of the author or committer.
	//
	// Note: This is not necessarily a valid UTF-8 string, as git does not enforce
	// this. It's up to the caller to check validity or sanitize the string as needed.
	Name string `json:"Name,omitempty"`
	// Email is the email of the author or committer.
	//
	// Note: This is not necessarily a valid UTF-8 string, as git does not enforce
	// this. It's up to the caller to check validity or sanitize the string as needed.
	Email string    `json:"Email,omitempty"`
	Date  time.Time `json:"Date"`
}

type RefType int

const (
	RefTypeUnknown RefType = iota
	RefTypeBranch
	RefTypeTag
)

func RefTypeFromProto(t proto.GitRef_RefType) RefType {
	switch t {
	case proto.GitRef_REF_TYPE_BRANCH:
		return RefTypeBranch
	case proto.GitRef_REF_TYPE_TAG:
		return RefTypeTag
	default:
		return RefTypeUnknown
	}
}

func (t RefType) ToProto() proto.GitRef_RefType {
	switch t {
	case RefTypeBranch:
		return proto.GitRef_REF_TYPE_BRANCH
	case RefTypeTag:
		return proto.GitRef_REF_TYPE_TAG
	default:
		return proto.GitRef_REF_TYPE_UNSPECIFIED
	}
}

// A ContributorCount is a contributor to a repository.
type ContributorCount struct {
	Name  string
	Email string
	Count int32
}

func (p *ContributorCount) String() string {
	return fmt.Sprintf("%d %s <%s>", p.Count, p.Name, p.Email)
}

func ContributorCountFromProto(p *proto.ContributorCount) *ContributorCount {
	if p == nil {
		return nil
	}
	c := &ContributorCount{
		Count: p.GetCount(),
	}
	if p.GetAuthor() != nil {
		c.Name = string(p.GetAuthor().GetName())
		c.Email = string(p.GetAuthor().GetEmail())
	}
	return c
}

func (c *ContributorCount) ToProto() *proto.ContributorCount {
	if c == nil {
		return nil
	}
	return &proto.ContributorCount{
		Count: c.Count,
		Author: &proto.GitSignature{
			Name:  []byte(c.Name),
			Email: []byte(c.Email),
		},
	}
}

// Ref describes a Git ref.
type Ref struct {
	// Name the full name of the ref (e.g., "refs/heads/mybranch").
	Name string
	// ShortName the abbreviated name of the ref, if it wouldn't be ambiguous (e.g., "mybranch").
	ShortName string
	// Type is the type of this reference.
	Type RefType
	// CommitID is the hash of the commit the reference is currently pointing at.
	// For a head reference, this is the commit the head is currently pointing at.
	// For a tag, this is the commit that the tag is attached to.
	CommitID api.CommitID
	// RefOID is the full object ID of the reference. For a head reference and
	// a lightweight tag, this value is the same as CommitID. For annotated tags,
	// it is the object ID of the tag.
	RefOID api.CommitID
	// CreatedDate is the date the ref was created or modified last.
	CreatedDate time.Time
	// IsHead indicates whether this is the head reference.
	IsHead bool
}

func RefFromProto(r *proto.GitRef) Ref {
	return Ref{
		Name:        string(r.GetRefName()),
		ShortName:   string(r.GetShortRefName()),
		Type:        RefTypeFromProto(r.GetRefType()),
		CommitID:    api.CommitID(r.GetTargetCommit()),
		RefOID:      api.CommitID(r.GetRefOid()),
		CreatedDate: r.GetCreatedAt().AsTime(),
		IsHead:      r.GetIsHead(),
	}
}

func (r *Ref) ToProto() *proto.GitRef {
	return &proto.GitRef{
		RefName:      []byte(r.Name),
		ShortRefName: []byte(r.ShortName),
		TargetCommit: string(r.CommitID),
		RefOid:       string(r.RefOID),
		CreatedAt:    timestamppb.New(r.CreatedDate),
		RefType:      r.Type.ToProto(),
		IsHead:       r.IsHead,
	}
}

// BehindAhead is a set of behind/ahead counts.
type BehindAhead struct {
	Behind uint32 `json:"Behind,omitempty"`
	Ahead  uint32 `json:"Ahead,omitempty"`
}

func BehindAheadFromProto(p *proto.BehindAheadResponse) *BehindAhead {
	if p == nil {
		return nil
	}

	return &BehindAhead{
		Behind: p.GetBehind(),
		Ahead:  p.GetAhead(),
	}
}

func (b *BehindAhead) ToProto() *proto.BehindAheadResponse {
	if b == nil {
		return nil
	}
	return &proto.BehindAheadResponse{
		Behind: b.Behind,
		Ahead:  b.Ahead,
	}
}

// EnsureRefPrefix checks whether the ref is a full ref and contains the
// "refs/heads" prefix (i.e. "refs/heads/master") or just an abbreviated ref
// (i.e. "master") and adds the "refs/heads/" prefix if the latter is the case.
func EnsureRefPrefix(ref string) string {
	return "refs/heads/" + strings.TrimPrefix(ref, "refs/heads/")
}

// AbbreviateRef removes the "refs/heads/" prefix from a given ref. If the ref
// doesn't have the prefix, it returns it unchanged.
func AbbreviateRef(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

var invalidBranch = lazyregexp.New(`\.\.|/\.|\.lock$|[\000-\037\177 ~^:?*[]+|^/|/$|//|\.$|@{|^@$|\\`)

// ValidateBranchName returns false if the given string is not a valid branch name.
// It follows the rules here: https://git-scm.com/docs/git-check-ref-format
// NOTE: It does not require a slash as mentioned in point 2.
func ValidateBranchName(branch string) bool {
	return !(invalidBranch.MatchString(branch) || strings.EqualFold(branch, "head"))
}

// RefGlob describes a glob pattern that either includes or excludes refs. Exactly 1 of the fields
// must be set.
type RefGlob struct {
	// Include is a glob pattern for including refs interpreted as in `git log --glob`. See the
	// git-log(1) manual page for details.
	Include string

	// Exclude is a glob pattern for excluding refs interpreted as in `git log --exclude`. See the
	// git-log(1) manual page for details.
	Exclude string
}

// RefGlobs is a compiled matcher based on RefGlob patterns. Use CompileRefGlobs to create it.
type RefGlobs []compiledRefGlobPattern

type compiledRefGlobPattern struct {
	pattern glob.Glob
	include bool // true for include, false for exclude
}

// CompileRefGlobs compiles the ordered ref glob patterns (interpreted as in `git log --glob
// ... --exclude ...`; see the git-log(1) manual page) into a matcher. If the input patterns are
// invalid, an error is returned.
func CompileRefGlobs(globs []RefGlob) (RefGlobs, error) {
	c := make(RefGlobs, len(globs))
	for i, g := range globs {
		// Validate exclude globs according to `git log --exclude`'s specs: "The patterns
		// given...must begin with refs/... If a trailing /* is intended, it must be given
		// explicitly."
		if g.Exclude != "" {
			if !strings.HasPrefix(g.Exclude, "refs/") {
				return nil, errors.Errorf(`git ref exclude glob must begin with "refs/" (got %q)`, g.Exclude)
			}
		}

		// Add implicits (according to `git log --glob`'s specs).
		if g.Include != "" {
			// `git log --glob`: "Leading refs/ is automatically prepended if missing.".
			if !strings.HasPrefix(g.Include, "refs/") {
				g.Include = "refs/" + g.Include
			}

			// `git log --glob`: "If pattern lacks ?, *, or [, /* at the end is implied." Also
			// support an important undocumented case: support exact matches. For example, the
			// pattern refs/heads/a should match the ref refs/heads/a (i.e., just appending /* to
			// the pattern would yield refs/heads/a/*, which would *not* match refs/heads/a, so we
			// need to make the /* optional).
			if !strings.ContainsAny(g.Include, "?*[") {
				var suffix string
				if strings.HasSuffix(g.Include, "/") {
					suffix = "*"
				} else {
					suffix = "/*"
				}
				g.Include += "{," + suffix + "}"
			}
		}

		var pattern string
		if g.Include != "" {
			pattern = g.Include
			c[i].include = true
		} else {
			pattern = g.Exclude
		}
		var err error
		c[i].pattern, err = glob.Compile(pattern)
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

// Match reports whether the named ref matches the ref globs.
func (gs RefGlobs) Match(ref string) bool {
	match := false
	for _, g := range gs {
		if g.include == match {
			// If the glob does not change the outcome, skip it. (For example, if the ref is already
			// matched, and the next glob is another include glob.)
			continue
		}
		if g.pattern.Match(ref) {
			match = g.include
		}
	}
	return match
}

// Pathspec is a git term for a pattern that matches paths using glob-like syntax.
// https://git-scm.com/docs/gitglossary#Documentation/gitglossary.txt-aiddefpathspecapathspec
type Pathspec string

func FSFileInfoToProto(fi fs.FileInfo) *v1.FileInfo {
	p := &proto.FileInfo{
		Name: []byte(fi.Name()),
		Size: fi.Size(),
		Mode: uint32(fi.Mode()),
	}
	sys := fi.Sys()
	switch s := sys.(type) {
	case Submodule:
		p.Submodule = &proto.GitSubmodule{
			Url:       s.URL,
			CommitSha: string(s.CommitID),
			Path:      []byte(s.Path),
		}
	case ObjectInfo:
		p.BlobOid = s.OID().String()
	}
	return p
}

func ProtoFileInfoToFS(fi *v1.FileInfo) fs.FileInfo {
	var sys any
	if sm := fi.GetSubmodule(); sm != nil {
		sys = Submodule{
			URL:      sm.GetUrl(),
			Path:     string(sm.GetPath()),
			CommitID: api.CommitID(sm.GetCommitSha()),
		}
	} else {
		oid, _ := decodeOID(fi.GetBlobOid())
		sys = objectInfo(oid)
	}
	return &fileutil.FileInfo{
		Name_:    string(fi.GetName()),
		Mode_:    fs.FileMode(fi.GetMode()),
		Size_:    fi.GetSize(),
		ModTime_: time.Time{}, // Not supported.
		Sys_:     sys,
	}
}

func decodeOID(sha string) (OID, error) {
	oidBytes, err := hex.DecodeString(sha)
	if err != nil {
		return OID{}, err
	}
	var oid OID
	copy(oid[:], oidBytes)
	return oid, nil
}

type objectInfo OID

func (oid objectInfo) OID() OID { return OID(oid) }
