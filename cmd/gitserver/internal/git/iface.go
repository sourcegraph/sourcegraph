package git

import (
	"context"
	"io"
	"io/fs"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
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
	// If any path does not exist, a os.PathError is returned.
	ArchiveReader(ctx context.Context, format ArchiveFormat, treeish string, paths []string) (io.ReadCloser, error)
	// ResolveRevision resolves the given revspec to a commit ID.
	// I.e., HEAD, deadbeefdeadbeefdeadbeefdeadbeef, or refs/heads/main.
	// If passed a commit sha, will also verify that the commit exists.
	// If the revspec can not be resolved to a commit, a RevisionNotFoundError is returned.
	ResolveRevision(ctx context.Context, revspec string) (api.CommitID, error)
	// Stat returns the file info for the given path at the given commit.
	// If the file does not exist, a os.PathError is returned.
	// If the commit does not exist, a RevisionNotFoundError is returned.
	// Stat supports submodules, symlinks, directories and files.
	Stat(ctx context.Context, commit api.CommitID, path string) (fs.FileInfo, error)
	// ReadDir returns the list of files and directories in the given path at the given commit.
	// Path can be used to read subdirectories.
	// If the path does not exist, a os.PathError is returned.
	// If the commit does not exist, a RevisionNotFoundError is returned.
	// ReadDir supports submodules, symlinks, directories and files.
	// If recursive is true, ReadDir will return the contents of all subdirectories.
	// The caller must call Close on the returned ReadDirIterator when done.
	ReadDir(ctx context.Context, commit api.CommitID, path string, recursive bool) (ReadDirIterator, error)

	// Exec is a temporary helper to run arbitrary git commands from the exec endpoint.
	// No new usages of it should be introduced and once the migration is done we will
	// remove this method.
	Exec(ctx context.Context, args ...string) (io.ReadCloser, error)
}

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

// ReadDirIterator is an iterator for the contents of a directory.
type ReadDirIterator interface {
	// Next returns the next file in the directory. io.EOF is returned at the end
	// of the stream.
	Next() (fs.FileInfo, error)
	// Close closes the iterator.
	Close() error
}
