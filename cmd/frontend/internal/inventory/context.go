package inventory

import (
	"archive/tar"
	"context"
	"io"
	"io/fs"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
)

// Context defines the environment in which the inventory is computed.
type Context struct {
	Repo api.RepoName

	CommitID api.CommitID

	// ReadTree is called to list the immediate children of a tree at path. The returned fs.FileInfo
	// values' Name method must return the full path (that can be passed to another ReadTree or
	// ReadFile call), not just the basename.
	ReadTree func(ctx context.Context, path string) ([]fs.FileInfo, error)

	// NewFileReader is called to get an io.ReadCloser from the file at path.
	NewFileReader func(ctx context.Context, path string) (io.ReadCloser, error)

	CacheKey func(e fs.FileInfo) string

	// CacheGet, if set, returns the cached inventory and true for the given tree, or false for a cache miss.
	CacheGet func(context.Context, string) (Inventory, bool)

	// CacheSet, if set, stores the inventory in the cache for the given tree.
	CacheSet func(context.Context, string, Inventory)

	NewTarReader func(io.ReadCloser) *tar.Reader

	ShouldSkipEnhancedLanguageDetection bool

	GitServerClient gitserver.Client
}
