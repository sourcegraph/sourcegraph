package internal

import (
	"sync"

	"github.com/sourcegraph/sourcegraph/internal/api"
)

// RepositoryLock is returned by RepositoryLocker.TryAcquire. It allows
// updating the status of a repo lock, as well as releasing the lock.
type RepositoryLock interface {
	// SetStatus updates the status for the lock. If the lock has been released,
	// this is a noop.
	SetStatus(status string)
	// Release releases the lock.
	Release()
}

// RepositoryLocker provides locks for doing operations to a repository.
// When a repository is locked, only the owner of the lock is allowed to perform
// writing operations against it.
//
// The main use of RepositoryLocker is to prevent concurrent clones. However,
// it is also used during maintenance tasks such as recloning/migrating/etc.
type RepositoryLocker interface {
	// TryAcquire acquires the lock for repo. If it is already held, ok is false
	// and lock is nil. Otherwise a non-nil lock is returned and true. When
	// finished with the lock you must call lock.Release.
	TryAcquire(repo api.RepoName, initialStatus string) (lock RepositoryLock, ok bool)
	// Status returns the status of the locked repo. If repo is not locked, then
	// locked is false.
	Status(repo api.RepoName) (status string, locked bool)
	// AllStatuses returns the status of all locked repositories.
	AllStatuses() map[api.RepoName]string
}

func NewRepositoryLocker() RepositoryLocker {
	return &repositoryLocker{
		status: make(map[api.RepoName]string),
	}
}

type repositoryLocker struct {
	// mu protects status
	mu sync.RWMutex
	// status tracks repos that are locked. The value is the status. If
	// a repo is in status, the repo is locked.
	status map[api.RepoName]string
}

func (rl *repositoryLocker) TryAcquire(repo api.RepoName, initialStatus string) (lock RepositoryLock, ok bool) {
	rl.mu.Lock()
	_, failed := rl.status[repo]
	if !failed {
		if rl.status == nil {
			rl.status = make(map[api.RepoName]string)
		}
		rl.status[repo] = initialStatus
	}
	rl.mu.Unlock()

	if failed {
		return nil, false
	}

	return &repositoryLock{
		unlock: func() {
			rl.mu.Lock()
			delete(rl.status, repo)
			rl.mu.Unlock()
		},
		setStatus: func(status string) {
			rl.mu.Lock()
			rl.status[repo] = status
			rl.mu.Unlock()
		},
		repo: repo,
	}, true
}

func (rl *repositoryLocker) Status(repo api.RepoName) (status string, locked bool) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	status, locked = rl.status[repo]
	return
}

func (rl *repositoryLocker) AllStatuses() map[api.RepoName]string {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	statuses := make(map[api.RepoName]string, len(rl.status))
	for repo, status := range rl.status {
		statuses[repo] = status
	}

	return statuses
}

type repositoryLock struct {
	unlock    func()
	setStatus func(status string)
	repo      api.RepoName

	mu   sync.Mutex
	done bool
}

func (l *repositoryLock) SetStatus(status string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Ensure this is still locked before updating the status
	if !l.done {
		l.setStatus(status)
	}
}

func (l *repositoryLock) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Prevent double release
	if !l.done {
		l.unlock()
		l.done = true
	}
}
