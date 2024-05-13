package janitor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func repairRepo(ctx context.Context, logger log.Logger, backend git.GitBackend, gitDir common.GitDir) error {
	if err := git.SetGitAttributes(gitDir); err != nil {
		return err
	}

	if err := backend.Config().Unset(ctx, "gc.auto"); err != nil {
		return err
	}

	if err := removeStaleLocks(logger, gitDir); err != nil {
		return err
	}

	return nil
}

const gcLockFile = "gc.pid"

func removeStaleLocks(logger log.Logger, gitDir common.GitDir) error {
	// if removing a lock fails, we still want to try the other locks.
	var multi error

	// config.lock should be held for a very short amount of time.
	if _, err := removeFileOlderThan(logger, gitDir.Path("config.lock"), time.Minute); err != nil {
		multi = errors.Append(multi, err)
	}
	// packed-refs can be held for quite a while, so we are conservative
	// with the age.
	if _, err := removeFileOlderThan(logger, gitDir.Path("packed-refs.lock"), time.Hour); err != nil {
		multi = errors.Append(multi, err)
	}
	// when a multi-pack-index operation fails, we need to manually release the lock.
	if _, err := removeFileOlderThan(logger, gitDir.Path("objects", "pack", "multi-pack-index.lock"), time.Hour); err != nil {
		multi = errors.Append(multi, err)
	}
	// we use the same conservative age for locks inside of refs
	if err := gitserverfs.BestEffortWalk(gitDir.Path("refs"), func(path string, fi os.DirEntry) error {
		if fi.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".lock") {
			return nil
		}

		_, err := removeFileOlderThan(logger, path, time.Hour)
		return err
	}); err != nil {
		multi = errors.Append(multi, err)
	}
	// We have seen that, occasionally, commit-graph.locks prevent a git repack from
	// succeeding. Benchmarks on our dogfood cluster have shown that a commit-graph
	// call for a 5GB bare repository takes less than 1 min. The lock is only held
	// during a short period during this time. A 1-hour grace period is very
	// conservative.
	if _, err := removeFileOlderThan(logger, gitDir.Path("objects", "info", "commit-graph.lock"), time.Hour); err != nil {
		multi = errors.Append(multi, err)
	}

	// gc.pid is set by git gc and our sg maintenance script. 24 hours is twice the
	// time git gc uses internally.
	gcPIDMaxAge := 24 * time.Hour
	if foundStale, err := removeFileOlderThan(logger, gitDir.Path(gcLockFile), gcPIDMaxAge); err != nil {
		multi = errors.Append(multi, err)
	} else if foundStale {
		logger.Warn(
			"removeStaleLocks found a stale gc.pid lockfile and removed it. This should not happen and points to a problem with garbage collection. Monitor the repo for possible corruption and verify if this error reoccurs",
			log.String("path", string(gitDir)),
			log.Duration("age", gcPIDMaxAge))
	}

	// drop temporary pack files that can be left behind by a fetch.
	packdir := gitDir.Path("objects", "pack")
	err := gitserverfs.BestEffortWalk(packdir, func(path string, d os.DirEntry) error {
		if path != packdir && d.IsDir() {
			return filepath.SkipDir
		}

		file := filepath.Base(path)
		if !strings.HasPrefix(file, "tmp_pack_") {
			return nil
		}

		_, err := removeFileOlderThan(logger, path, 2*conf.GitLongCommandTimeout())
		return err
	})
	if err != nil {
		multi = errors.Append(multi, err)
	}

	return multi
}

// removeFileOlderThan removes path if its mtime is older than maxAge. If the
// file is missing, no error is returned. The first argument indicates whether a
// stale file was present.
func removeFileOlderThan(logger log.Logger, path string, maxAge time.Duration) (bool, error) {
	fi, err := os.Stat(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	age := time.Since(fi.ModTime())
	if age < maxAge {
		return false, nil
	}

	logger.Debug("removing stale lock file", log.String("path", path), log.Duration("age", age))
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return true, err
	}
	return true, nil
}
