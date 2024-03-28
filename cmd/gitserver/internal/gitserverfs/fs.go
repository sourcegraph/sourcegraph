package gitserverfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/mountinfo"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/diskusage"
	du "github.com/sourcegraph/sourcegraph/internal/diskusage"
	"github.com/sourcegraph/sourcegraph/internal/fileutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type FS interface {
	// Initialize creates all the necessary directory structures used by gitserverfs.
	Initialize() error
	DirSize(string) (int64, error)
	RepoDir(api.RepoName) common.GitDir
	ResolveRepoName(common.GitDir) api.RepoName
	TempDir(prefix string) (string, error)
	IgnorePath(string) bool
	P4HomeDir() (string, error)
	VisitRepos(func(api.RepoName, common.GitDir) (done bool, _ error)) error
	RepoCloned(api.RepoName) (bool, error)
	RemoveRepo(api.RepoName) error
	ForEachRepo(func(api.RepoName, common.GitDir) (done bool)) error
	DiskUsage() (diskusage.DiskUsage, error)
}

func New(observationCtx *observation.Context, reposDir string) FS {
	return &realGitserverFS{
		logger:              observationCtx.Logger.Scoped("gitserverfs"),
		observationCtx:      observationCtx,
		reposDir:            reposDir,
		clonedState:         make(map[api.RepoName]struct{}),
		lastCloneStateReset: time.Now(),
	}
}

type realGitserverFS struct {
	reposDir       string
	observationCtx *observation.Context
	logger         log.Logger
	// clonedState keeps track of the clone state of a repo.
	clonedState         map[api.RepoName]struct{}
	cloneStateMu        sync.Mutex
	lastCloneStateReset time.Time
}

func (r *realGitserverFS) Initialize() error {
	err := initGitserverFileSystem(r.logger, r.reposDir)
	if err != nil {
		return err
	}
	r.registerMetrics()
	return nil
}

func (r *realGitserverFS) DirSize(dir string) (int64, error) {
	if !filepath.IsAbs(dir) {
		return 0, errors.New("dir must be absolute")
	}
	return dirSize(dir)
}

func (r *realGitserverFS) RepoDir(name api.RepoName) common.GitDir {
	// We need to use api.UndeletedRepoName(repo) for the name, as this is a name
	// transformation done on the database side that gitserver cannot know about.
	dir := repoDirFromName(r.reposDir, api.UndeletedRepoName(name))
	// dir is expected to be cleaned, ie. it doesn't allow `..`.
	if !strings.HasPrefix(dir.Path(), r.reposDir) {
		panic("dir is outside of repos dir")
	}
	return dir
}

func (r *realGitserverFS) ResolveRepoName(dir common.GitDir) api.RepoName {
	return repoNameFromDir(r.reposDir, dir)
}

func (r *realGitserverFS) TempDir(prefix string) (string, error) {
	return tempDir(r.reposDir, prefix)
}

func (r *realGitserverFS) IgnorePath(path string) bool {
	return ignorePath(r.reposDir, path)
}

func (r *realGitserverFS) P4HomeDir() (string, error) {
	return makeP4HomeDir(r.reposDir)
}

func (r *realGitserverFS) VisitRepos(visit func(api.RepoName, common.GitDir) (done bool, _ error)) error {
	return nil
}

const cloneStateResetInterval = 5 * time.Minute

func (r *realGitserverFS) RepoCloned(name api.RepoName) (bool, error) {
	r.cloneStateMu.Lock()
	defer r.cloneStateMu.Unlock()

	// Every few minutes, we invalidate the entire cache, in case we fall into
	// some bad state, this'll fix the state every now and then.
	if time.Since(r.lastCloneStateReset) > cloneStateResetInterval {
		r.clonedState = make(map[api.RepoName]struct{})
	}

	_, cloned := r.clonedState[name]
	if cloned {
		return true, nil
	}

	cloned, err := repoCloned(r.RepoDir(name))
	if err != nil {
		return false, err
	}

	if cloned {
		r.clonedState[name] = struct{}{}
	}

	return cloned, nil
}

func (r *realGitserverFS) RemoveRepo(name api.RepoName) error {
	err := removeRepoDirectory(r.logger, r.reposDir, r.RepoDir(name))

	// Mark as not cloned anymore in cache. We even remove it from the cache
	// when the deletion failed partially, in the next call we'll recheck if
	// the repo is still there.
	r.cloneStateMu.Lock()
	delete(r.clonedState, name)
	r.cloneStateMu.Unlock()

	return err
}

// iterateGitDirs walks over the reposDir on disk and calls walkFn for each of the
// git directories found on disk.
func (r *realGitserverFS) ForEachRepo(visit func(api.RepoName, common.GitDir) bool) error {
	return BestEffortWalk(r.reposDir, func(dir string, fi fs.DirEntry) error {
		if ignorePath(r.reposDir, dir) {
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
		gitDir := common.GitDir(dir)

		if done := visit(r.ResolveRepoName(gitDir), gitDir); done {
			return filepath.SkipAll
		}

		return filepath.SkipDir
	})
}

func (r *realGitserverFS) DiskUsage() (diskusage.DiskUsage, error) {
	return du.New(r.reposDir)
}

var realGitserverFSMetricsRegisterer sync.Once

func (r *realGitserverFS) registerMetrics() {
	realGitserverFSMetricsRegisterer.Do(func() {
		// report the size of the repos dir
		opts := mountinfo.CollectorOpts{Namespace: "gitserver"}
		m := mountinfo.NewCollector(r.logger, opts, map[string]string{"reposDir": r.reposDir})
		r.observationCtx.Registerer.MustRegister(m)

		metrics.MustRegisterDiskMonitor(r.reposDir)

		// TODO: Start removal of these.
		// TODO(keegan) these are older names for the above disk metric. Keeping
		// them to prevent breaking dashboards. Can remove once no
		// alert/dashboards use them.
		c := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "src_gitserver_disk_space_available",
			Help: "Amount of free space disk space on the repos mount.",
		}, func() float64 {
			usage, err := du.New(r.reposDir)
			if err != nil {
				r.logger.Error("error getting disk usage info", log.Error(err))
				return 0
			}
			return float64(usage.Available())
		})
		prometheus.MustRegister(c)

		c = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "src_gitserver_disk_space_total",
			Help: "Amount of total disk space in the repos directory.",
		}, func() float64 {
			usage, err := du.New(r.reposDir)
			if err != nil {
				r.logger.Error("error getting disk usage info", log.Error(err))
				return 0
			}
			return float64(usage.Size())
		})
		prometheus.MustRegister(c)
	})
}

// repoCloned checks if dir or `${dir}/.git` is a valid GIT_DIR.
func repoCloned(dir common.GitDir) (bool, error) {
	_, err := os.Stat(dir.Path("HEAD"))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// tempDirName is the name used for the temporary directory under ReposDir.
const tempDirName = ".tmp"

// p4HomeName is the name used for the directory that git p4 will use as $HOME
// and where it will store cache data.
const p4HomeName = ".p4home"

func repoDirFromName(reposDir string, name api.RepoName) common.GitDir {
	p := string(protocol.NormalizeRepo(name))
	return common.GitDir(filepath.Join(reposDir, filepath.FromSlash(p), ".git"))
}

func repoNameFromDir(reposDir string, dir common.GitDir) api.RepoName {
	// dir == ${s.ReposDir}/${name}/.git
	parent := filepath.Dir(string(dir))                   // remove suffix "/.git"
	name := strings.TrimPrefix(parent, reposDir)          // remove prefix "${s.ReposDir}"
	name = strings.Trim(name, string(filepath.Separator)) // remove /
	name = filepath.ToSlash(name)                         // filepath -> path
	return protocol.NormalizeRepo(api.RepoName(name))
}

// tempDir is a wrapper around os.MkdirTemp, but using the given reposDir
// temporary directory filepath.Join(s.ReposDir, tempDirName).
//
// This directory is cleaned up by gitserver and will be ignored by repository
// listing operations.
func tempDir(reposDir, prefix string) (name string, err error) {
	// TODO: At runtime, this directory always exists. We only need to ensure
	// the directory exists here because tests use this function without creating
	// the directory first. Ideally, we can remove this later.
	tmp := filepath.Join(reposDir, tempDirName)
	if err := os.MkdirAll(tmp, os.ModePerm); err != nil {
		return "", err
	}
	return os.MkdirTemp(tmp, prefix)
}

func ignorePath(reposDir string, path string) bool {
	// We ignore any path which starts with .tmp or .p4home in ReposDir
	if filepath.Dir(path) != reposDir {
		return false
	}
	base := filepath.Base(path)
	return strings.HasPrefix(base, tempDirName) || strings.HasPrefix(base, p4HomeName)
}

// removeRepoDirectory atomically removes a directory from reposDir.
//
// It first moves the directory to a temporary location to avoid leaving
// partial state in the event of server restart or concurrent modifications to
// the directory.
//
// Additionally, it removes parent empty directories up until reposDir.
func removeRepoDirectory(logger log.Logger, reposDir string, gitDir common.GitDir) error {
	dir := string(gitDir)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// If directory doesn't exist we can avoid all the work below and treat it as if
		// it was removed.
		return nil
	}

	// Rename out of the location, so we can atomically stop using the repo.
	tmp, err := tempDir(reposDir, "delete-repo")
	if err != nil {
		return err
	}
	defer func() {
		// Delete the atomically renamed dir.
		if err := os.RemoveAll(filepath.Join(tmp)); err != nil {
			logger.Warn("failed to cleanup after removing dir", log.String("dir", dir), log.Error(err))
		}
	}()
	if err := fileutil.RenameAndSync(dir, filepath.Join(tmp, "repo")); err != nil {
		return err
	}

	// Everything after this point is just cleanup, so any error that occurs
	// should not be returned, just logged.

	// Cleanup empty parent directories. We just attempt to remove and if we
	// have a failure we assume it's due to the directory having other
	// children. If we checked first we could race with someone else adding a
	// new clone.
	rootInfo, err := os.Stat(reposDir)
	if err != nil {
		logger.Warn("Failed to stat ReposDir", log.Error(err))
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
			logger.Warn("failed to stat parent directory", log.String("dir", current), log.Error(err))
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

	return nil
}

// BestEffortWalk is a filepath.WalkDir which ignores errors that can be passed
// to walkFn. This is a common pattern used in gitserver for best effort work.
//
// Note: We still respect errors returned by walkFn.
//
// filepath.Walk can return errors if we run into permission errors or a file
// disappears between readdir and the stat of the file. In either case this
// error can be ignored for best effort code.
func BestEffortWalk(root string, walkFn func(path string, entry fs.DirEntry) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		return walkFn(path, d)
	})
}

// dirSize returns the total size in bytes of all the files under d.
func dirSize(d string) (int64, error) {
	var size int64
	return size, BestEffortWalk(d, func(path string, d fs.DirEntry) error {
		if d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			// We ignore errors for individual files.
			return nil
		}
		size += fi.Size()
		return nil
	})
}
