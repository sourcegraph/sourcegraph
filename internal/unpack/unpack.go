/*
Copyright 2018 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package unpack

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Opts struct {
	// SkipInvalid makes unpacking skip any invalid files rather than aborting
	// the whole unpack.
	SkipInvalid bool

	// Filter filters out files that match the given predicate.
	Filter func(file fs.FileInfo) bool
}

// Tgz unpacks the contents of the given gzip compressed tarball under dir.
func Tgz(r io.Reader, dir string, opt Opts) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	return Tar(gzr, dir, opt)
}

// Tar unpacks the contents of the specified tarball under dir.
func Tar(r io.Reader, dir string, opt Opts) error {
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if header.Size < 0 {
			continue
		}

		if opt.Filter != nil && !opt.Filter(header.FileInfo()) {
			continue
		}

		err = sanitizeTarPath(header, dir)
		if err != nil {
			if opt.SkipInvalid {
				continue
			}
			return err
		}

		err = extractFile(tr, header, dir)
		if err != nil {
			return err
		}
	}
}

// extractTarFile extracts a single file or directory from tarball into dir.
func extractFile(tr *tar.Reader, h *tar.Header, dir string) error {
	switch h.Typeflag {
	case tar.TypeDir:
		return withDir(filepath.Join(dir, h.Name), nil)
	case tar.TypeBlock, tar.TypeChar, tar.TypeReg, tar.TypeRegA, tar.TypeFifo:
		return writeFile(filepath.Join(dir, h.Name), tr, h.Size, h.FileInfo().Mode())
	case tar.TypeLink:
		return writeHardLink(filepath.Join(dir, h.Name), filepath.Join(dir, h.Linkname))
	case tar.TypeSymlink:
		return writeSymbolicLink(filepath.Join(dir, h.Name), h.Linkname)
	}
	return nil
}

// sanitizeTarPath checks that the tar header paths resolve to a subdirectory
// path, and don't contain file paths or links that could escape the tar file
// like ../../etc/password.
func sanitizeTarPath(h *tar.Header, dir string) error {
	// Sanitize all tar paths resolve to within the destination directory.
	destPath := filepath.Join(dir, h.Name)
	if !strings.HasPrefix(destPath, filepath.Clean(dir)+string(os.PathSeparator)) {
		return errors.Errorf("%s: illegal file path", h.Name)
	}

	// Ensure link destinations resolve to within the destination directory.
	if h.Linkname != "" {
		if filepath.IsAbs(h.Linkname) {
			if !strings.HasPrefix(filepath.Clean(h.Linkname), filepath.Clean(dir)+string(os.PathSeparator)) {
				return errors.Errorf("%s: illegal link path", h.Linkname)
			}
		} else {
			// Relative paths are relative to filename after extraction to directory.
			linkPath := filepath.Join(dir, filepath.Dir(h.Name), h.Linkname)
			if !strings.HasPrefix(linkPath, filepath.Clean(dir)+string(os.PathSeparator)) {
				return errors.Errorf("%s: illegal link path", h.Linkname)
			}
		}
	}
	return nil
}

func writeFile(path string, r io.Reader, n int64, mode os.FileMode) error {
	return withDir(path, func() error {
		// Create file only if it does not exist to prevent overwriting existing
		// files (like session recordings).
		out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			return err
		}

		if _, err = io.CopyN(out, r, n); err != nil {
			return err
		}

		return out.Close()
	})
}

func writeSymbolicLink(path string, target string) error {
	return withDir(path, func() error { return os.Symlink(target, path) })
}

func writeHardLink(path string, target string) error {
	return withDir(path, func() error { return os.Link(target, path) })
}

func withDir(path string, fn func() error) error {
	err := os.MkdirAll(filepath.Dir(path), 0770)
	if err != nil {
		return err
	}

	if fn == nil {
		return nil
	}

	return fn()
}
