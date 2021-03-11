package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/types"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/inconshreveable/log15"
)

const (
	// repoTTL is how often we should reclone a repository
	repoTTL = time.Hour * 24 * 45
	// repoTTLGC is how often we should reclone a repository once it is
	// reporting git gc issues.
	repoTTLGC = time.Hour * 24 * 2
)

// EnableGCAuto is a temporary flag that allows us to control whether or not
// `git gc --auto` is invoked during janitorial activities. This flag will
// likely evolve into some form of site config value in the future.
var enableGCAuto, _ = strconv.ParseBool(env.Get("SRC_ENABLE_GC_AUTO", "true", "Use git-gc during janitorial cleanup phases"))

var (
	reposRemoved = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_gitserver_repos_removed",
		Help: "number of repos removed during cleanup",
	})
	reposRecloned = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_gitserver_repos_recloned",
		Help: "number of repos removed and recloned due to age",
	})
	reposRemovedDiskPressure = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_gitserver_repos_removed_disk_pressure",
		Help: "number of repos removed due to not enough disk space",
	})
	janitorRunning = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "src_gitserver_janitor_running",
		Help: "set to 1 when the gitserver janitor background job is running",
	})
	jobTimer = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "src_gitserver_janitor_job_duration_seconds",
		Help: "Duration of the individual jobs within the gitserver janitor background job",
	}, []string{"job_name"})
)

const reposStatsName = "repos-stats.json"

// cleanupRepos walks the repos directory and performs maintenance tasks:
//
// 1. Remove corrupt repos.
// 2. Remove stale lock files.
// 3. Remove repos based on disk pressure.
// 4. Reclone repos after a while. (simulate git gc)
// 5. Remove repos on disk that don't belong in this shard
func (s *Server) cleanupRepos(addrs []string) {
	janitorRunning.Set(1)
	defer janitorRunning.Set(0)

	bCtx, bCancel := s.serverContext()
	defer bCancel()

	stats := protocol.ReposStats{
		UpdatedAt: time.Now(),
	}

	computeStats := func(dir GitDir) (done bool, err error) {
		stats.GitDirBytes += dirSize(dir.Path("."))
		return false, nil
	}

	maybeRemoveCorrupt := func(dir GitDir) (done bool, err error) {
		// We treat repositories missing HEAD to be corrupt. Both our cloning
		// and fetching ensure there is a HEAD file.
		_, err = os.Stat(dir.Path("HEAD"))
		if !os.IsNotExist(err) {
			return false, err
		}

		log15.Info("removing corrupt repo", "repo", dir)
		if err := s.removeRepoDirectory(dir); err != nil {
			return true, err
		}
		reposRemoved.Inc()
		return true, nil
	}

	ensureGitAttributes := func(dir GitDir) (done bool, err error) {
		return false, setGitAttributes(dir)
	}

	scrubRemoteURL := func(dir GitDir) (done bool, err error) {
		cmd := exec.Command("git", "remote", "remove", "origin")
		dir.Set(cmd)
		// ignore error since we fail if the remote has already been scrubbed.
		_ = cmd.Run()
		return false, nil
	}

	maybeReclone := func(dir GitDir) (done bool, err error) {
		repoType, err := getRepositoryType(dir)
		if err != nil {
			return false, err
		}

		recloneTime, err := getRecloneTime(dir)
		if err != nil {
			return false, err
		}

		// Add a jitter to spread out recloning of repos cloned at the same
		// time.
		var reason string
		const maybeCorrupt = "maybeCorrupt"
		if maybeCorrupt, _ := gitConfigGet(dir, "sourcegraph.maybeCorruptRepo"); maybeCorrupt != "" {
			reason = maybeCorrupt
			// unset flag to stop constantly recloning if it fails.
			_ = gitConfigUnset(dir, "sourcegraph.maybeCorruptRepo")
		}
		if time.Since(recloneTime) > repoTTL+jitterDuration(string(dir), repoTTL/4) {
			reason = "old"
		}
		if time.Since(recloneTime) > repoTTLGC+jitterDuration(string(dir), repoTTLGC/4) {
			if gclog, err := ioutil.ReadFile(dir.Path("gc.log")); err == nil && len(gclog) > 0 {
				reason = fmt.Sprintf("git gc %s", string(bytes.TrimSpace(gclog)))
			}
		}

		// We believe converting a Perforce depot to a Git repository is generally a
		// very expensive operation, therefore we do not try to reclone/redo the
		// conversion only because it is old or slow to do "git gc".
		if repoType == "perforce" && reason != maybeCorrupt {
			reason = ""
		}

		if reason == "" {
			return false, nil
		}

		ctx, cancel := context.WithTimeout(bCtx, longGitCommandTimeout)
		defer cancel()

		// name is the relative path to ReposDir, but without the .git suffix.
		repo := s.name(dir)
		log15.Info("recloning expired repo", "repo", repo, "cloned", recloneTime, "reason", reason)

		// update the reclone time so that we don't constantly reclone if
		// cloning fails. For example if a repo fails to clone due to being
		// large, we will constantly be doing a clone which uses up lots of
		// resources.
		if err := setRecloneTime(dir, recloneTime.Add(time.Since(recloneTime)/2)); err != nil {
			log15.Warn("setting backed off reclone time failed", "repo", repo, "cloned", recloneTime, "reason", reason, "error", err)
		}

		if _, err := s.cloneRepo(ctx, repo, &cloneOptions{Block: true, Overwrite: true}); err != nil {
			return true, err
		}
		reposRecloned.Inc()
		return true, nil
	}

	removeStaleLocks := func(dir GitDir) (done bool, err error) {
		gitDir := string(dir)

		// if removing a lock fails, we still want to try the other locks.
		var multi error

		// config.lock should be held for a very short amount of time.
		if err := removeFileOlderThan(filepath.Join(gitDir, "config.lock"), time.Minute); err != nil {
			multi = multierror.Append(multi, err)
		}
		// packed-refs can be held for quite a while, so we are conservative
		// with the age.
		if err := removeFileOlderThan(filepath.Join(gitDir, "packed-refs.lock"), time.Hour); err != nil {
			multi = multierror.Append(multi, err)
		}
		// we use the same conservative age for locks inside of refs
		if err := bestEffortWalk(filepath.Join(gitDir, "refs"), func(path string, fi os.FileInfo) error {
			if fi.IsDir() {
				return nil
			}

			if !strings.HasSuffix(path, ".lock") {
				return nil
			}

			return removeFileOlderThan(path, time.Hour)
		}); err != nil {
			multi = multierror.Append(multi, err)
		}

		return false, multi
	}

	performGC := func(dir GitDir) (done bool, err error) {
		if !enableGCAuto {
			return false, nil
		}
		return false, gitGC(dir)
	}

	removeWrongShard := func(dir GitDir) (done bool, err error) {
		if len(addrs) == 0 {
			return false, nil
		}
		addr := gitserver.AddrForRepo(s.name(dir), addrs)
		if s.hostnameMatch(addr) {
			return false, nil
		}
		log15.Info("removing repo for wrong shard", "repo", dir)
		if err := s.removeRepoDirectory(dir); err != nil {
			return true, err
		}
		reposRemoved.Inc()
		return false, nil
	}

	type cleanupFn struct {
		Name string
		Do   func(GitDir) (bool, error)
	}
	cleanups := []cleanupFn{
		{"compute statistics", computeStats},
		// Do some sanity checks on the repository.
		{"maybe remove corrupt", maybeRemoveCorrupt},
		// If git is interrupted it can leave lock files lying around. It does not clean
		// these up, and instead fails commands.
		{"remove stale locks", removeStaleLocks},
		// We always want to have the same git attributes file at info/attributes.
		{"ensure git attributes", ensureGitAttributes},
		// 2021-03-01 (tomas,keegan) we used to store an authenticated remote URL on
		// disk. We no longer need it so we can scrub it.
		{"scrub remote URL", scrubRemoteURL},
		// Old git clones accumulate loose git objects that waste space and slow down git
		// operations. Periodically do a fresh clone to avoid these problems. git gc is
		// slow and resource intensive. It is cheaper and faster to just reclone the
		// repository.
		{"maybe reclone", maybeReclone},
		// Runs a number of housekeeping tasks within the current repository, such as
		// compressing file revisions (to reduce disk space and increase performance),
		// removing unreachable objects which may have been created from prior
		// invocations of git add, packing refs, pruning reflog, rerere metadata or stale
		// working trees. May also update ancillary indexes such as the commit-graph.
		{"garbage collect", performGC},
		// Repos are sharded across gitserver instances based on their name. Remove repos
		// that no longer belong on this shard.
		{"remove wrong shard", removeWrongShard},
	}

	err := bestEffortWalk(s.ReposDir, func(dir string, fi os.FileInfo) error {
		if s.ignorePath(dir) {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Look for $GIT_DIR
		if !fi.IsDir() || fi.Name() != ".git" {
			return nil
		}

		// We are sure this is a GIT_DIR after the above check
		gitDir := GitDir(dir)

		for _, cfn := range cleanups {
			start := time.Now()
			done, err := cfn.Do(gitDir)
			if err != nil {
				log15.Error("error running cleanup command", "name", cfn.Name, "repo", gitDir, "error", err)
			}
			jobTimer.WithLabelValues(cfn.Name).Observe(time.Since(start).Seconds())
			if done {
				break
			}
		}
		return filepath.SkipDir
	})
	if err != nil {
		log15.Error("cleanup: error iterating over repositories", "error", err)
	}

	if b, err := json.Marshal(stats); err != nil {
		log15.Error("cleanup: failed to marshal periodic stats", "error", err)
	} else if err = ioutil.WriteFile(filepath.Join(s.ReposDir, reposStatsName), b, 0666); err != nil {
		log15.Error("cleanup: failed to write periodic stats", "error", err)
	}

	if s.DiskSizer == nil {
		s.DiskSizer = &StatDiskSizer{}
	}
	b, err := s.howManyBytesToFree()
	if err != nil {
		log15.Error("cleanup: ensuring free disk space", "error", err)
	}
	if err := s.freeUpSpace(b); err != nil {
		log15.Error("cleanup: error freeing up space", "error", err)
	}
}

// DiskSizer gets information about disk size and free space.
type DiskSizer interface {
	BytesFreeOnDisk(mountPoint string) (uint64, error)
	DiskSizeBytes(mountPoint string) (uint64, error)
}

// howManyBytesToFree returns the number of bytes that should be freed to make sure
// there is sufficient disk space free to satisfy s.DesiredPercentFree.
func (s *Server) howManyBytesToFree() (int64, error) {
	actualFreeBytes, err := s.DiskSizer.BytesFreeOnDisk(s.ReposDir)
	if err != nil {
		return 0, errors.Wrap(err, "finding the amount of space free on disk")
	}

	// Free up space if necessary.
	diskSizeBytes, err := s.DiskSizer.DiskSizeBytes(s.ReposDir)
	if err != nil {
		return 0, errors.Wrap(err, "getting disk size")
	}
	desiredFreeBytes := uint64(float64(s.DesiredPercentFree) / 100.0 * float64(diskSizeBytes))
	howManyBytesToFree := int64(desiredFreeBytes - actualFreeBytes)
	if howManyBytesToFree < 0 {
		howManyBytesToFree = 0
	}
	const G = float64(1024 * 1024 * 1024)
	log15.Debug("cleanup",
		"desired percent free", s.DesiredPercentFree,
		"actual percent free", float64(actualFreeBytes)/float64(diskSizeBytes)*100.0,
		"amount to free in GiB", float64(howManyBytesToFree)/G)
	return howManyBytesToFree, nil
}

type StatDiskSizer struct{}

func (s *StatDiskSizer) BytesFreeOnDisk(mountPoint string) (uint64, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(mountPoint, &fs); err != nil {
		return 0, errors.Wrap(err, "statting")
	}
	free := fs.Bavail * uint64(fs.Bsize)
	return free, nil
}

func (s *StatDiskSizer) DiskSizeBytes(mountPoint string) (uint64, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(mountPoint, &fs); err != nil {
		return 0, errors.Wrap(err, "statting")
	}
	free := fs.Blocks * uint64(fs.Bsize)
	return free, nil
}

// freeUpSpace removes git directories under ReposDir, in order from least
// recently to most recently used, until it has freed howManyBytesToFree.
func (s *Server) freeUpSpace(howManyBytesToFree int64) error {
	if howManyBytesToFree <= 0 {
		return nil
	}

	// Get the git directories and their mod times.
	gitDirs, err := s.findGitDirs()
	if err != nil {
		return errors.Wrap(err, "finding git dirs")
	}
	dirModTimes := make(map[GitDir]time.Time, len(gitDirs))
	for _, d := range gitDirs {
		mt, err := gitDirModTime(d)
		if err != nil {
			return errors.Wrap(err, "computing mod time of git dir")
		}
		dirModTimes[d] = mt
	}

	// Sort the repos from least to most recently used.
	sort.Slice(gitDirs, func(i, j int) bool {
		return dirModTimes[gitDirs[i]].Before(dirModTimes[gitDirs[j]])
	})

	// Remove repos until howManyBytesToFree is met or exceeded.
	var spaceFreed int64
	diskSizeBytes, err := s.DiskSizer.DiskSizeBytes(s.ReposDir)
	if err != nil {
		return errors.Wrap(err, "getting disk size")
	}
	for _, d := range gitDirs {
		if spaceFreed >= howManyBytesToFree {
			return nil
		}
		delta := dirSize(d.Path("."))
		if err := s.removeRepoDirectory(d); err != nil {
			return errors.Wrap(err, "removing repo directory")
		}
		spaceFreed += delta
		reposRemovedDiskPressure.Inc()

		// Report the new disk usage situation after removing this repo.
		actualFreeBytes, err := s.DiskSizer.BytesFreeOnDisk(s.ReposDir)
		if err != nil {
			return errors.Wrap(err, "finding the amount of space free on disk")
		}
		G := float64(1024 * 1024 * 1024)
		log15.Warn("cleanup: removed least recently used repo",
			"repo", d,
			"how old", time.Since(dirModTimes[d]),
			"free space in GiB", float64(actualFreeBytes)/G,
			"actual percent of disk space free", float64(actualFreeBytes)/float64(diskSizeBytes)*100.0,
			"desired percent of disk space free", float64(s.DesiredPercentFree),
			"space freed in GiB", float64(spaceFreed)/G,
			"how much space to free in GiB", float64(howManyBytesToFree)/G)
	}

	// Check.
	if spaceFreed < howManyBytesToFree {
		return fmt.Errorf("only freed %d bytes, wanted to free %d", spaceFreed, howManyBytesToFree)
	}
	return nil
}

func gitDirModTime(d GitDir) (time.Time, error) {
	head, err := os.Stat(d.Path("HEAD"))
	if err != nil {
		return time.Time{}, errors.Wrap(err, "getting repository modification time")
	}
	return head.ModTime(), nil
}

func (s *Server) findGitDirs() ([]GitDir, error) {
	var dirs []GitDir
	err := bestEffortWalk(s.ReposDir, func(path string, fi os.FileInfo) error {
		if s.ignorePath(path) {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !fi.IsDir() || fi.Name() != ".git" {
			return nil
		}
		dirs = append(dirs, GitDir(path))
		return filepath.SkipDir
	})
	if err != nil {
		return nil, errors.Wrap(err, "findGitDirs")
	}
	return dirs, nil
}

// dirSize returns the total size in bytes of all the files under d.
func dirSize(d string) int64 {
	var size int64
	// We don't return an error, so we know that err is always nil and can be
	// ignored.
	_ = bestEffortWalk(d, func(path string, fi os.FileInfo) error {
		if fi.IsDir() {
			return nil
		}
		size += fi.Size()
		return nil
	})
	return size
}

func (s *Server) setCloneStatus(ctx context.Context, name api.RepoName, status types.CloneStatus) (err error) {
	if s.DB == nil {
		return nil
	}
	tx, err := database.Repos(s.DB).Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	repo, err := tx.GetByName(ctx, name)
	if err != nil {
		return err
	}
	return database.NewGitserverReposWith(tx).SetCloneStatus(ctx, repo.ID, status, s.Hostname)
}

// setCloneStatusNonFatal is the same as setCloneStatus but only logs errors
func (s *Server) setCloneStatusNonFatal(ctx context.Context, name api.RepoName, status types.CloneStatus) {
	if err := s.setCloneStatus(ctx, name, status); err != nil {
		log15.Warn("Setting clone status in DB", "error", err)
	}
}

// removeRepoDirectory atomically removes a directory from s.ReposDir.
//
// It first moves the directory to a temporary location to avoid leaving
// partial state in the event of server restart or concurrent modifications to
// the directory.
//
// Additionally it removes parent empty directories up until s.ReposDir.
func (s *Server) removeRepoDirectory(gitDir GitDir) error {
	ctx := context.Background()
	dir := string(gitDir)

	// Rename out of the location so we can atomically stop using the repo.
	tmp, err := s.tempDir("delete-repo")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := renameAndSync(dir, filepath.Join(tmp, "repo")); err != nil {
		return err
	}

	// Everything after this point is just cleanup, so any error that occurs
	// should not be returned, just logged.

	// Set as not_cloned in the database
	s.setCloneStatusNonFatal(ctx, s.name(gitDir), types.CloneStatusNotCloned)

	// Cleanup empty parent directories. We just attempt to remove and if we
	// have a failure we assume it's due to the directory having other
	// children. If we checked first we could race with someone else adding a
	// new clone.
	rootInfo, err := os.Stat(s.ReposDir)
	if err != nil {
		log15.Warn("Failed to stat ReposDir", "error", err)
		return nil
	}
	current := dir
	for {
		parent := filepath.Dir(current)
		if parent == current {
			// This shouldn't happen, but protecting against escaping
			// ReposDir.
			break
		}
		current = parent
		info, err := os.Stat(current)
		if os.IsNotExist(err) {
			// Someone else beat us to it.
			break
		}
		if err != nil {
			log15.Warn("failed to stat parent directory", "dir", current, "error", err)
			return nil
		}
		if os.SameFile(rootInfo, info) {
			// Stop, we are at the parent.
			break
		}

		if err := os.Remove(current); err != nil {
			// Stop, we assume remove failed due to current not being empty.
			break
		}
	}

	// Delete the atomically renamed dir. We do this last since if it fails we
	// will rely on a janitor job to clean up for us.
	if err := os.RemoveAll(filepath.Join(tmp, "repo")); err != nil {
		log15.Warn("failed to cleanup after removing dir", "dir", dir, "error", err)
	}

	return nil
}

// cleanTmpFiles tries to remove tmp_pack_* files from .git/objects/pack.
// These files can be created by an interrupted fetch operation,
// and would be purged by `git gc --prune=now`, but `git gc` is
// very slow. Removing these files while they're in use will cause
// an operation to fail, but not damage the repository.
func (s *Server) cleanTmpFiles(dir GitDir) {
	now := time.Now()
	packdir := dir.Path("objects", "pack")
	err := bestEffortWalk(packdir, func(path string, info os.FileInfo) error {
		if path != packdir && info.IsDir() {
			return filepath.SkipDir
		}
		file := filepath.Base(path)
		if strings.HasPrefix(file, "tmp_pack_") {
			if now.Sub(info.ModTime()) > longGitCommandTimeout {
				err := os.Remove(path)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		log15.Error("error removing tmp_pack_* files", "error", err)
	}
}

// SetupAndClearTmp sets up the the tempdir for ReposDir as well as clearing it
// out. It returns the temporary directory location.
func (s *Server) SetupAndClearTmp() (string, error) {
	// Additionally we create directories with the prefix .tmp-old which are
	// asynchronously removed. We do not remove in place since it may be a
	// slow operation to block on. Our tmp dir will be ${s.ReposDir}/.tmp
	dir := filepath.Join(s.ReposDir, tempDirName) // .tmp
	oldPrefix := tempDirName + "-old"
	if _, err := os.Stat(dir); err == nil {
		// Rename the current tmp file so we can asynchronously remove it. Use
		// a consistent pattern so if we get interrupted, we can clean it
		// another time.
		oldTmp, err := ioutil.TempDir(s.ReposDir, oldPrefix)
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

	// Asynchronously remove old temporary directories
	files, err := ioutil.ReadDir(s.ReposDir)
	if err != nil {
		log15.Error("failed to do tmp cleanup", "error", err)
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
					log15.Error("cleanup: failed to remove old temporary directory", "path", path, "error", err)
				}
			}(filepath.Join(s.ReposDir, f.Name()))
		}
	}

	return dir, nil
}

// setRepositoryType sets the type of the repository.
func setRepositoryType(dir GitDir, typ string) error {
	return gitConfigSet(dir, "sourcegraph.type", typ)
}

// getRepositoryType returns the type of the repository.
func getRepositoryType(dir GitDir) (string, error) {
	val, err := gitConfigGet(dir, "sourcegraph.type")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(val), nil
}

// setRecloneTime sets the time a repository is cloned.
func setRecloneTime(dir GitDir, now time.Time) error {
	err := gitConfigSet(dir, "sourcegraph.recloneTimestamp", strconv.FormatInt(now.Unix(), 10))
	if err != nil {
		ensureHEAD(dir)
		return errors.Wrap(err, "failed to update recloneTimestamp")
	}
	return nil
}

// getRecloneTime returns an approximate time a repository is cloned. If the
// value is not stored in the repository, the reclone time for the repository
// is set to now.
func getRecloneTime(dir GitDir) (time.Time, error) {
	// We store the time we recloned the repository. If the value is missing,
	// we store the current time. This decouples this timestamp from the
	// different ways a clone can appear in gitserver.
	update := func() (time.Time, error) {
		now := time.Now()
		return now, setRecloneTime(dir, now)
	}

	value, err := gitConfigGet(dir, "sourcegraph.recloneTimestamp")
	if err != nil {
		return time.Unix(0, 0), errors.Wrap(err, "failed to determine clone timestamp")
	}
	if value == "" {
		return update()
	}

	sec, err := strconv.ParseInt(strings.TrimSpace(value), 10, 0)
	if err != nil {
		// If the value is bad update it to the current time
		now, err2 := update()
		if err2 != nil {
			err = err2
		}
		return now, err
	}

	return time.Unix(sec, 0), nil
}

// maybeCorruptStderrRe matches stderr lines from git which indicate there
// might be repository corruption.
//
// See https://github.com/sourcegraph/sourcegraph/issues/6676 for more
// context.
var maybeCorruptStderrRe = lazyregexp.NewPOSIX(`^error: (Could not read|packfile) `)

func checkMaybeCorruptRepo(repo api.RepoName, dir GitDir, stderr string) {
	if !maybeCorruptStderrRe.MatchString(stderr) {
		return
	}

	log15.Warn("marking repo for recloning due to stderr output indicating repo corruption", "repo", repo, "stderr", stderr)

	// We set a flag in the config for the cleanup janitor job to fix. The
	// janitor runs every minute.
	err := gitConfigSet(dir, "sourcegraph.maybeCorruptRepo", strconv.FormatInt(time.Now().Unix(), 10))
	if err != nil {
		log15.Error("failed to set maybeCorruptRepo config", repo, "repo", "error", err)
	}
}

// gitGC will invoke `git-gc` to clean up any garbage in the repo. It will
// operate synchronously and be aggressive with its internal heurisitcs when
// deciding to act (meaning it will act now at lower thresholds).
func gitGC(dir GitDir) error {
	cmd := exec.Command("git", "-c", "gc.auto=1", "-c", "gc.autoDetach=false", "gc", "--auto")
	dir.Set(cmd)
	err := cmd.Run()
	if err != nil {
		return errors.Wrapf(wrapCmdError(cmd, err), "failed to git-gc")
	}
	return nil
}

func gitConfigGet(dir GitDir, key string) (string, error) {
	cmd := exec.Command("git", "config", "--get", key)
	dir.Set(cmd)
	out, err := cmd.Output()
	if err != nil {
		// Exit code 1 means the key is not set.
		if ee, ok := err.(*exec.ExitError); ok && ee.Sys().(syscall.WaitStatus).ExitStatus() == 1 {
			return "", nil
		}
		return "", errors.Wrapf(wrapCmdError(cmd, err), "failed to get git config %s", key)
	}
	return string(out), nil
}

func gitConfigSet(dir GitDir, key, value string) error {
	cmd := exec.Command("git", "config", key, value)
	dir.Set(cmd)
	err := cmd.Run()
	if err != nil {
		return errors.Wrapf(wrapCmdError(cmd, err), "failed to set git config %s", key)
	}
	return nil
}

func gitConfigUnset(dir GitDir, key string) error {
	cmd := exec.Command("git", "config", "--unset-all", key)
	dir.Set(cmd)
	err := cmd.Run()
	if err != nil {
		// Exit code 5 means the key is not set.
		if ee, ok := err.(*exec.ExitError); ok && ee.Sys().(syscall.WaitStatus).ExitStatus() == 5 {
			return nil
		}
		return errors.Wrapf(wrapCmdError(cmd, err), "failed to unset git config %s", key)
	}
	return nil
}

// jitterDuration returns a duration between [0, d) based on key. This is like
// a random duration, but instead of a random source it is computed via a hash
// on key.
func jitterDuration(key string, d time.Duration) time.Duration {
	h := fnv.New64()
	_, _ = io.WriteString(h, key)
	r := time.Duration(h.Sum64())
	if r < 0 {
		// +1 because we have one more negative value than positive. ie
		// math.MinInt64 == -math.MinInt64.
		r = -(r + 1)
	}
	return r % d
}

// wrapCmdError will wrap errors for cmd to include the arguments. If the error
// is an exec.ExitError and cmd was invoked with Output(), it will also include
// the captured stderr.
func wrapCmdError(cmd *exec.Cmd, err error) error {
	if err == nil {
		return nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return errors.Wrapf(err, "%s %s failed with stderr: %s", cmd.Path, strings.Join(cmd.Args, " "), string(ee.Stderr))
	}
	return errors.Wrapf(err, "%s %s failed", cmd.Path, strings.Join(cmd.Args, " "))
}

// removeFileOlderThan removes path if its mtime is older than maxAge. If the
// file is missing, no error is returned.
func removeFileOlderThan(path string, maxAge time.Duration) error {
	fi, err := os.Stat(filepath.Join(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	age := time.Since(fi.ModTime())
	if age < maxAge {
		return nil
	}

	log15.Debug("removing stale lock file", "path", path, "age", age)
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
