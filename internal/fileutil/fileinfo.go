package fileutil

import (
	"os"
	"time"
)

// FileInfo implements fs.FileInfo.
type FileInfo struct {
	Name_    string
	Mode_    os.FileMode
	Size_    int64
	ModTime_ time.Time
	Sys_     any
}

func (fi *FileInfo) Name() string       { return fi.Name_ }
func (fi *FileInfo) Size() int64        { return fi.Size_ }
func (fi *FileInfo) Mode() os.FileMode  { return fi.Mode_ }
func (fi *FileInfo) ModTime() time.Time { return fi.ModTime_ }
func (fi *FileInfo) IsDir() bool        { return fi.Mode().IsDir() }
func (fi *FileInfo) Sys() any           { return fi.Sys_ }
