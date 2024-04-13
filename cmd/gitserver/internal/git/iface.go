package git

import (
	"context"
	"io"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
)

// GitBackend is the interface through which operations on a git repository can
// be performed. It encapsulates the underlying git implementation and allows
// us to test out alternative backends.
// A GitBackend is expected to be scoped to a specific repository directory at
// initialization time, ie. it should not be shared across various repositories.
type GitBackend interface {
	// Config returns a backend for interacting with git configuration at .git/config.
	Config() GitConfigBackend
	// GetObject allows to read a git object from the git object database.
	GetObject(ctx context.Context, objectName string) (*gitdomain.GitObject, error)
	// MergeBase finds the merge base commit for the given base and head revspecs.
	// Returns an empty string and no error if no common merge-base was found.
	// If one of the two given revspecs does not exist, a RevisionNotFoundError
	// is returned.
	MergeBase(ctx context.Context, baseRevspec, headRevspec string) (api.CommitID, error)
	// Blame returns a reader for the blame info of the given path.
	// BlameHunkReader must always be closed.
	// If the file does not exist, a os.PathError is returned.
	// If the commit does not exist, a RevisionNotFoundError is returned.
	Blame(ctx context.Context, startCommit api.CommitID, path string, opt BlameOptions) (BlameHunkReader, error)
	// SymbolicRefHead resolves what the HEAD symbolic ref points to. This is also
	// commonly referred to as the default branch within Sourcegraph.
	// If short is true, the returned ref name will be shortened when possible
	// without ambiguity.
	SymbolicRefHead(ctx context.Context, short bool) (string, error)
	// RevParseHead resolves at what commit HEAD points to. If HEAD doesn't point
	// to anything, a RevisionNotFoundError is returned. This can occur, for example,
	// when the repository is empty (ie. has no commits).
	RevParseHead(ctx context.Context) (api.CommitID, error)
	// ReadFile returns a reader for the contents of the given file at the given commit.
	// If the file does not exist, a os.PathError is returned.
	// If the path points to a submodule, an empty reader is returned and no error.
	// If the commit does not exist, a RevisionNotFoundError is returned.
	ReadFile(ctx context.Context, commit api.CommitID, path string) (io.ReadCloser, error)
	// GetCommit retrieves the commit with the given ID from the git ODB.
	// If includeModifiedFiles is true, the returned GitCommitWithFiles will contain
	// the list of all files touched in this commit.
	// If the commit doesn't exist, a RevisionNotFoundError is returned.
	GetCommit(ctx context.Context, commit api.CommitID, includeModifiedFiles bool) (*GitCommitWithFiles, error)
	// ArchiveReader returns a reader for an archive in the given format.
	// Treeish is the tree or commit to archive, and paths is the list of
	// paths to include in the archive. If empty, all paths are included.
	//
	// If the commit does not exist, a RevisionNotFoundError is returned.
	ArchiveReader(ctx context.Context, format ArchiveFormat, treeish string, paths []string) (io.ReadCloser, error)
	// ResolveRevision resolves the given revspec to a commit ID.
	// I.e., HEAD, deadbeefdeadbeefdeadbeefdeadbeef, or refs/heads/main.
	// If passed a commit sha, will also verify that the commit exists.
	// If the revspec can not be resolved to a commit, a RevisionNotFoundError is returned.
	ResolveRevision(ctx context.Context, revspec string) (api.CommitID, error)
	// ListRefs returns a list of all the refs known to the repository, this includes
	// heads, tags, and other potential refs, but filters can be applied.
	//
	// The refs are ordered in the following order:
	// HEAD first, if part of the result set.
	// The rest will be ordered by creation date, in descending order, i.e., newest
	// first.
	// If two resources are created at the same timestamp, the records are ordered
	// alphabetically.
	ListRefs(ctx context.Context, opt ListRefsOpts) (RefIterator, error)
	// RevAtTime returns the OID of the nearest ancestor of `spec` that has a
	// commit time before the given time. To simplify the logic, it only
	// follows the first parent of merge commits to linearize the commit
	// history. The intent is to return the state of a branch at a given time.
	//
	// If revspec does not exist, a RevisionNotFoundError is returned.
	// If no commit exists in the history of revspec before time, an empty
	// commitID is returned.
	RevAtTime(ctx context.Context, revspec string, time time.Time) (api.CommitID, error)
	// RawDiff returns the raw git diff for the given range.
	// Diffs returned from this function will have the following settings applied:
	// - 3 lines of context
	// - No a/ b/ prefixes
	// - Rename detection
	// If either base or head don't exist, a RevisionNotFoundError is returned.
	RawDiff(ctx context.Context, base string, head string, typ GitDiffComparisonType, paths ...string) (io.ReadCloser, error)
	// ContributorCounts returns the number of commits per contributor in the
	// set of commits specified by the options.
	// Aggregations are done by email address.
	// If range does not exist, a RevisionNotFoundError is returned.
	ContributorCounts(ctx context.Context, opt ContributorCountsOpts) ([]*gitdomain.ContributorCount, error)
	// Fetch fetches the configured revspecs from the remote repository using the
	// given URL.
	// The returned iterator can be used to read all the ref updates in a structured
	// way.
	// The returned Reader can be used to read the progress of the fetch operation.
	Fetch(ctx context.Context, opt FetchOptions) (_ RefUpdateIterator, progress io.Reader, _ error)

	// Exec is a temporary helper to run arbitrary git commands from the exec endpoint.
	// No new usages of it should be introduced and once the migration is done we will
	// remove this method.
	Exec(ctx context.Context, args ...string) (io.ReadCloser, error)
}

type GitDiffComparisonType int

const (
	// Corresponds to the BASE...HEAD syntax that returns any commits that are not
	// in both BASE and HEAD.
	GitDiffComparisonTypeIntersection GitDiffComparisonType = iota
	// Corresponds to the BASE..HEAD syntax that only returns any commits that are
	// in HEAD but not in BASE.
	GitDiffComparisonTypeOnlyInHead
)

// GitConfigBackend provides methods for interacting with git configuration.
type GitConfigBackend interface {
	// Get reads a given config value. If the value is not set, it returns an
	// empty string and no error.
	Get(ctx context.Context, key string) (string, error)
	// Set sets a config value for the given key.
	Set(ctx context.Context, key, value string) error
	// Unset removes a config value of the given key. If the key wasn't present,
	// no error is returned.
	Unset(ctx context.Context, key string) error
}

// BlameOptions are options for git blame.
type BlameOptions struct {
	IgnoreWhitespace bool
	Range            *BlameRange
}

type BlameRange struct {
	// 1-indexed start line
	StartLine int
	// 1-indexed end line
	EndLine int
}

// BlameHunkReader is a reader for git blame hunks.
type BlameHunkReader interface {
	// Consume the next hunk. io.EOF is returned at the end of the stream.
	Read() (*gitdomain.Hunk, error)
	Close() error
}

// GitCommitWithFiles wraps a gitdomain.Commit and adds a list of modified files.
// Modified files are only populated when requested.
// This data is required for sub repo permission filtering.
type GitCommitWithFiles struct {
	*gitdomain.Commit
	ModifiedFiles []string
}

// ArchiveFormat indicates the desired format of the archive as an enum.
type ArchiveFormat string

const (
	// ArchiveFormatZip indicates a zip archive is desired.
	ArchiveFormatZip ArchiveFormat = "zip"

	// ArchiveFormatTar indicates a tar archive is desired.
	ArchiveFormatTar ArchiveFormat = "tar"
)

// ListRefsOpts are additional options passed to ListRefs.
type ListRefsOpts struct {
	// If true, only heads are returned. Can be combined with HeadsOnly.
	HeadsOnly bool
	// If true, only tags are returned. Can be combined with TagsOnly.
	TagsOnly bool
	// If set, only return refs that point at the given commit shas. Multiple
	// values will be ORed together.
	PointsAtCommit []api.CommitID
	// If set, only return refs that contain the given commit shas.
	Contains []api.CommitID
}

// RefIterator iterates over refs.
type RefIterator interface {
	// Next returns the next ref.
	Next() (*gitdomain.Ref, error)
	// Close releases resources associated with the iterator.
	Close() error
}

// ContributorCountsOpts are options for the ContributorCounts method.
type ContributorCountsOpts struct {
	// If set, only count commits that are in the given range.
	// Range can contain:
	// - A commit hash or ref name, which includes that commit/ref and all of the parents
	// - A range of two commits/hashes, separated by either .. or ... notation: A..B, A...B
	Range string
	// If set, only count commits that are after the given time.
	After time.Time
	// If set, only count commits that are in the given path. Can be a pathspec
	// (e.g., "foo/bar/").
	Path string
}

// FetchOptions are options for the Fetch method. All options must be specified.
type FetchOptions struct {
	// RemoteURL is the URL of the remote to fetch from. It may contain credentials
	// in the User field.
	RemoteURL *vcs.URL
	// Refspecs specifies which refs to fetch and update.
	// Example format: {"+refs/heads/*:refs/heads/*", "+refs/tags/*:refs/tags/*"}.
	Refspecs []string
	// TLSConfig, if set, contains additional information about certificate settings
	// to pass to git.
	TLSConfig GitTLSConfig
}

type GitTLSConfig struct {
	// Whether to not verify the SSL certificate when fetching or pushing over
	// HTTPS.
	//
	// https://git-scm.com/docs/git-config#Documentation/git-config.txt-httpsslVerify
	SSLNoVerify bool

	// File containing the certificates to verify the peer with when fetching
	// or pushing over HTTPS.
	//
	// https://git-scm.com/docs/git-config#Documentation/git-config.txt-httpsslCAInfo
	SSLCAInfo string
}

// RefUpdateIterator is an interface that allows iterating over a set of RefUpdates.
type RefUpdateIterator interface {
	// Next returns the next RefUpdate.
	Next() (RefUpdate, error)
	// Close releases resources associated with the iterator.
	Close() error
}

type RefUpdate struct {
	Type           RefUpdateType
	OldSHA         api.CommitID
	NewSHA         api.CommitID
	LocalReference string
}

// RefUpdateType indicates what kind of update has been made.
type RefUpdateType byte

const (
	// for a successfully fetched fast-forward
	RefUpdateTypeFastForwardUpdate RefUpdateType = ' '
	// for a successful forced update
	RefUpdateTypeForcedUpdate RefUpdateType = '+'
	// for a successfully pruned ref
	RefUpdateTypePruned RefUpdateType = '-'
	// for a successful tag update
	RefUpdateTypeTagUpdate RefUpdateType = 't'
	// for a successfully fetched new ref
	RefUpdateTypeNewRef RefUpdateType = '*'
	// for a ref that was rejected or failed to update
	RefUpdateTypeFailed RefUpdateType = '!'
	// for a ref that was up to date and did not need fetching
	RefUpdateTypeUnchanged RefUpdateType = '='
)
