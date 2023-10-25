package git

import (
	"archive/tar"
	"context"
	"io"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
)

type ArchiveFormat string

const (
	Tar ArchiveFormat = "tar"
	Zip ArchiveFormat = "zip"
)

func Archive(ctx context.Context, dir common.GitDir, w io.Writer, format ArchiveFormat, commitish string) error {
	r, err := git.PlainOpen(dir.Path())
	if err != nil {
		return err
	}

	c, err := r.CommitObject(plumbing.NewHash(commitish))
	if err != nil {
		return err
	}

	b, err := git.Blame(c, "a/b.txt")

	it, err := c.Files()
	if err != nil {
		return err
	}
	defer it.Close()

	tw := tar.NewWriter(w)

	for {
		f, err := it.Next()
		if err != nil {
			// Iteration done, no more files!
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
		}

		// For now, only support regular files and executable files.
		switch f.Mode {
		case filemode.Executable, filemode.Regular:
		default:
			continue
		}

		mode := int64(0o644)
		if f.Mode == filemode.Executable {
			mode = 0o755
		}

		// Write tar file header.
		if err := tw.WriteHeader(&tar.Header{
			Name:     f.Name,
			Typeflag: tar.TypeReg,
			Size:     f.Size,
			Mode:     mode,
		}); err != nil {
			return err
		}

		// Copy blob contents into tar writer.
		r, err := f.Blob.Reader()
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, r)
		if err != nil {
			return err
		}
		if err := r.Close(); err != nil {
			return err
		}
	}

	return tw.Close()
}
