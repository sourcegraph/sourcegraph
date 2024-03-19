package mapfs

import (
	"io"
	"io/fs"
	"time"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type mapFSDirectory struct {
	name    string
	entries []string
	offset  int
}

func (d *mapFSDirectory) Stat() (fs.FileInfo, error) {
	return &mapFSDirectoryEntry{name: d.name}, nil
}

func (d *mapFSDirectory) ReadDir(count int) ([]fs.DirEntry, error) {
	n := len(d.entries) - d.offset
	if n == 0 {
		if count <= 0 {
			return nil, nil
		}
		return nil, io.EOF
	}
	if count > 0 && n > count {
		n = count
	}

	list := make([]fs.DirEntry, 0, n)
	for range n {
		name := d.entries[d.offset]
		list = append(list, &mapFSDirectoryEntry{name: name})
		d.offset++
	}

	return list, nil
}

func (d *mapFSDirectory) Read(_ []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: errors.New("is a directory")}
}

func (d *mapFSDirectory) Close() error {
	return nil
}

type mapFSDirectoryEntry struct {
	name string
}

func (e *mapFSDirectoryEntry) Name() string               { return e.name }
func (e *mapFSDirectoryEntry) Size() int64                { return 0 }
func (e *mapFSDirectoryEntry) Mode() fs.FileMode          { return fs.ModeDir }
func (e *mapFSDirectoryEntry) ModTime() time.Time         { return time.Time{} }
func (e *mapFSDirectoryEntry) IsDir() bool                { return e.Mode().IsDir() }
func (e *mapFSDirectoryEntry) Sys() any                   { return nil }
func (e *mapFSDirectoryEntry) Type() fs.FileMode          { return fs.ModeDir }
func (e *mapFSDirectoryEntry) Info() (fs.FileInfo, error) { return e, nil }
