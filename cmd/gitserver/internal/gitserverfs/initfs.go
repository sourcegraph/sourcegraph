package gitserverfs

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func initGitserverFileSystem(logger log.Logger, reposDir string) error {
	// Ensure the ReposDir exists.
	if err := os.MkdirAll(reposDir, os.ModePerm); err != nil {
		return errors.Wrap(err, "creating SRC_REPOS_DIR")
	}
	// Ensure the Perforce Dir exists.
	p4Home, err := makeP4HomeDir(reposDir)
	if err != nil {
		return errors.Wrapf(err, "ensuring p4Home exists: %q", p4Home)
	}
	// Ensure the tmp dir exists, is cleaned up, and TMP_DIR is set properly.
	tmpDir, err := setupAndClearTmp(logger, reposDir)
	if err != nil {
		return errors.Wrap(err, "failed to setup temporary directory")
	}
	// Additionally, set TMP_DIR so other temporary files we may accidentally
	// create are on the faster RepoDir mount.
	if err := os.Setenv("TMP_DIR", tmpDir); err != nil {
		return errors.Wrap(err, "setting TMP_DIR")
	}

	// Delete the old reposStats file, which was used on gitserver prior to
	// 2023-08-14.
	if err := os.Remove(filepath.Join(reposDir, "repos-stats.json")); err != nil && !os.IsNotExist(err) {
		logger.Error("failed to remove old reposStats file", log.Error(err))
	}

	return nil
}

func makeP4HomeDir(reposDir string) (string, error) {
	p4Home := filepath.Join(reposDir, p4HomeName)
	// Ensure the directory exists
	if err := os.MkdirAll(p4Home, os.ModePerm); err != nil {
		return "", errors.Wrapf(err, "ensuring p4Home exists: %q", p4Home)
	}
	return p4Home, nil
}

// setupAndClearTmp sets up the tempdir for reposDir as well as clearing it
// out. It returns the temporary directory location.
func setupAndClearTmp(logger log.Logger, reposDir string) (string, error) {
	logger = logger.Scoped("setupAndClearTmp")

	// Additionally, we create directories with the prefix .tmp-old which are
	// asynchronously removed. We do not remove in place since it may be a
	// slow operation to block on. Our tmp dir will be ${s.ReposDir}/.tmp
	dir := filepath.Join(reposDir, tempDirName) // .tmp
	oldPrefix := tempDirName + "-old"
	if _, err := os.Stat(dir); err == nil {
		// Rename the current tmp file, so we can asynchronously remove it. Use
		// a consistent pattern so if we get interrupted, we can clean it
		// another time.
		oldTmp, err := os.MkdirTemp(reposDir, oldPrefix)
		if err != nil {
			return "", err
		}
		// oldTmp dir exists, so we need to use a child of oldTmp as the
		// rename target.
		if err := os.Rename(dir, filepath.Join(oldTmp, tempDirName)); err != nil {
			return "", err
		}
	}

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", err
	}

	// Asynchronously remove old temporary directories.
	// TODO: Why async?
	files, err := os.ReadDir(reposDir)
	if err != nil {
		logger.Error("failed to do tmp cleanup", log.Error(err))
	} else {
		for _, f := range files {
			// Remove older .tmp directories as well as our older tmp-
			// directories we would place into ReposDir. In September 2018 we
			// can remove support for removing tmp- directories.
			if !strings.HasPrefix(f.Name(), oldPrefix) && !strings.HasPrefix(f.Name(), "tmp-") {
				continue
			}
			go func(path string) {
				if err := os.RemoveAll(path); err != nil {
					logger.Error("failed to remove old temporary directory", log.String("path", path), log.Error(err))
				}
			}(filepath.Join(reposDir, f.Name()))
		}
	}

	return dir, nil
}
