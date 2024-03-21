package search

import (
	"fmt"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// RepoStatus is a bit flag encoding the status of a search on a repository. A
// repository can be in many states, so any bit may be set.
type RepoStatus uint8

const (
	RepoStatusCloning  RepoStatus = 1 << iota // could not be searched because they were still being cloned
	RepoStatusMissing                         // could not be searched because they do not exist
	RepoStatusLimitHit                        // searched, but have results that were not returned due to exceeded limits
	RepoStatusTimedOut                        // repos that were not searched due to timeout
)

var repoStatusName = []struct {
	status RepoStatus
	name   string
}{
	{RepoStatusCloning, "cloning"},
	{RepoStatusMissing, "missing"},
	{RepoStatusLimitHit, "limithit"},
	{RepoStatusTimedOut, "timedout"},
}

func (s RepoStatus) String() string {
	var parts []string
	for _, p := range repoStatusName {
		if p.status&s != 0 {
			parts = append(parts, p.name)
		}
	}
	return "RepoStatus{" + strings.Join(parts, " ") + "}"
}

// RepoStatusMap is a mutable map from repository IDs to a union of
// RepoStatus.
type RepoStatusMap struct {
	m map[api.RepoID]RepoStatus

	// status is a union of all repo status.
	status RepoStatus
}

// Iterate will call f for each RepoID in m.
func (m *RepoStatusMap) Iterate(f func(api.RepoID, RepoStatus)) {
	if m == nil {
		return
	}

	for id, status := range m.m {
		f(id, status)
	}
}

// Filter calls f for each repo RepoID where mask is a subset of the repo
// status.
func (m *RepoStatusMap) Filter(mask RepoStatus, f func(api.RepoID)) {
	if m == nil {
		return
	}

	if m.status&mask == 0 {
		return
	}
	for id, status := range m.m {
		if status&mask != 0 {
			f(id)
		}
	}
}

// Get returns the RepoStatus for id.
func (m *RepoStatusMap) Get(id api.RepoID) RepoStatus {
	if m == nil {
		return 0
	}
	return m.m[id]
}

// Update unions status for id with the current status.
func (m *RepoStatusMap) Update(id api.RepoID, status RepoStatus) {
	if m.m == nil {
		m.m = make(map[api.RepoID]RepoStatus)
	}
	m.m[id] |= status
	m.status |= status
}

// Union is a fast path for calling m.Update on all entries in o.
func (m *RepoStatusMap) Union(o *RepoStatusMap) {
	m.status |= o.status
	if m.m == nil && len(o.m) > 0 {
		m.m = make(map[api.RepoID]RepoStatus, len(o.m))
	}
	for id, status := range o.m {
		m.m[id] |= status
	}
}

// Any returns true if there are any entries which contain a status in mask.
func (m *RepoStatusMap) Any(mask RepoStatus) bool {
	if m == nil {
		return false
	}
	return m.status&mask != 0
}

// All returns true if all entries contain status.
func (m *RepoStatusMap) All(status RepoStatus) bool {
	if !m.Any(status) {
		return false
	}
	for _, s := range m.m {
		if s&status == 0 {
			return false
		}
	}
	return true
}

// Len is the number of entries in the map.
func (m *RepoStatusMap) Len() int {
	if m == nil {
		return 0
	}
	return len(m.m)
}

func (m *RepoStatusMap) String() string {
	if m == nil {
		m = &RepoStatusMap{}
	}
	return fmt.Sprintf("RepoStatusMap{N=%d %s}", len(m.m), m.status)
}

// repoStatusSingleton is a convenience function to contain a RepoStatusMap
// with one entry.
func repoStatusSingleton(id api.RepoID, status RepoStatus) RepoStatusMap {
	if status == 0 {
		return RepoStatusMap{}
	}
	return RepoStatusMap{
		m:      map[api.RepoID]RepoStatus{id: status},
		status: status,
	}
}

// HandleRepoSearchResult returns information about repository status, whether
// search limits are hit, and error promotion. If searchErr is a fatal error, it
// returns a non-nil error; otherwise, if searchErr == nil or a non-fatal error,
// it returns a nil error.
func HandleRepoSearchResult(repoID api.RepoID, revSpecs []string, limitHit, timedOut bool, searchErr error) (RepoStatusMap, error) {
	var fatalErr error
	var status RepoStatus
	if limitHit {
		status |= RepoStatusLimitHit
	}

	if gitdomain.IsRepoNotExist(searchErr) {
		if gitdomain.IsCloneInProgress(searchErr) {
			status |= RepoStatusCloning
		} else {
			status |= RepoStatusMissing
		}
	} else if errors.HasType(searchErr, &gitdomain.RevisionNotFoundError{}) {
		if len(revSpecs) == 0 || len(revSpecs) == 1 && revSpecs[0] == "" {
			// If we didn't specify an input revision, then the repo is empty and can be ignored.
		} else {
			fatalErr = searchErr
		}
	} else if errcode.IsNotFound(searchErr) {
		status |= RepoStatusMissing
	} else if errcode.IsTimeout(searchErr) || errcode.IsTemporary(searchErr) || timedOut {
		status |= RepoStatusTimedOut
	} else if searchErr != nil {
		fatalErr = searchErr
	}
	return repoStatusSingleton(repoID, status), fatalErr
}
