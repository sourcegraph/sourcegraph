package vfsutil

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/sourcegraph/ctxvfs"
	"golang.org/x/tools/godoc/vfs"
	"golang.org/x/tools/godoc/vfs/zipfs"
)

// archiveReader is like zip.ReadCloser, but it allows us to use a custom
// closer.
type archiveReader struct {
	*zip.Reader
	io.Closer
	Evicter

	// StripTopLevelDir specifies whether or not to strip the top level
	// directory in the zip archive (e.g. GitHub archives always have 1 top
	// level directory "{repobasename}-{sha}/").
	StripTopLevelDir bool

	// prefix is the name of the directory that was stripped from the archive
	// (or "" if nothing was stripped).
	prefix string
}

// ArchiveFS is a ctxvfs.FileSystem backed by an Archiver.
type ArchiveFS struct {
	fetch func(context.Context) (*archiveReader, error)

	// EvictOnClose when true will evict the underlying archive from the
	// archive cache when closed.
	EvictOnClose bool

	once sync.Once
	err  error // the error encountered during the fetch call (if any)
	ar   *archiveReader
	fs   vfs.FileSystem // the zipfs virtual file system

	// We have a mutex for closed to prevent Close and fetch racing.
	closedMu sync.Mutex
	closed   bool
}

// fetchOrWait initiates the fetch if it has not yet
// started. Otherwise it waits for it to finish.
func (fs *ArchiveFS) fetchOrWait(ctx context.Context) error {
	fs.once.Do(func() {
		// If we have already closed, do not open new resources. If we
		// haven't closed, prevent closing while fetching by holding
		// the lock.
		fs.closedMu.Lock()
		defer fs.closedMu.Unlock()
		if fs.closed {
			fs.err = errors.New("closed")
			return
		}

		fs.ar, fs.err = fs.fetch(ctx)
		if fs.err == nil {
			fs.fs = zipfs.New(&zip.ReadCloser{Reader: *fs.ar.Reader}, "")
			if fs.ar.StripTopLevelDir {
				entries, err := fs.fs.ReadDir("/")
				if err == nil && len(entries) == 1 && entries[0].IsDir() {
					fs.ar.prefix = entries[0].Name()
				}
			}

			if fs.ar.prefix != "" {
				ns := vfs.NameSpace{}
				ns.Bind("/", fs.fs, "/"+fs.ar.prefix, vfs.BindReplace)
				fs.fs = ns
			}
		}
	})
	return fs.err
}

func (fs *ArchiveFS) Open(ctx context.Context, name string) (ctxvfs.ReadSeekCloser, error) {
	if err := fs.fetchOrWait(ctx); err != nil {
		return nil, err
	}
	return fs.fs.Open(name)
}

func (fs *ArchiveFS) Lstat(ctx context.Context, path string) (os.FileInfo, error) {
	if err := fs.fetchOrWait(ctx); err != nil {
		return nil, err
	}
	return fs.fs.Lstat(path)
}

func (fs *ArchiveFS) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	if err := fs.fetchOrWait(ctx); err != nil {
		return nil, err
	}
	return fs.fs.Stat(path)
}

func (fs *ArchiveFS) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	if err := fs.fetchOrWait(ctx); err != nil {
		return nil, err
	}
	return fs.fs.ReadDir(path)
}

func (fs *ArchiveFS) ListAllFiles(ctx context.Context) ([]string, error) {
	if err := fs.fetchOrWait(ctx); err != nil {
		return nil, err
	}

	filenames := make([]string, 0, len(fs.ar.File))
	for _, f := range fs.ar.File {
		if f.Mode().IsRegular() {
			filenames = append(filenames, strings.TrimPrefix(f.Name, fs.ar.prefix+"/"))
		}
	}
	return filenames, nil
}

func (fs *ArchiveFS) Close() error {
	fs.closedMu.Lock()
	defer fs.closedMu.Unlock()
	if fs.closed {
		return errors.New("already closed")
	}

	fs.closed = true
	if fs.ar != nil && fs.ar.Closer != nil {
		err := fs.ar.Close()
		if err != nil {
			return err
		}
		if fs.EvictOnClose && fs.ar.Evicter != nil {
			fs.ar.Evict()
		}
	}
	return nil
}

func (fs *ArchiveFS) String() string { return "ArchiveFS(" + fs.fs.String() + ")" }
