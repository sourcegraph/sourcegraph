package codenav

import (
	"sync"

	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/core"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	sgTypes "github.com/sourcegraph/sourcegraph/internal/types"
)

type RequestState struct {
	// Local Caches
	dataLoader        *UploadsDataLoader
	GitTreeTranslator GitTreeTranslator
	commitCache       CommitCache
	// maximumIndexesPerMonikerSearch configures the maximum number of reference upload identifiers
	// that can be passed to a single moniker search query. Previously this limit was meant to keep
	// the number of SQLite files we'd have to open within a single call relatively low. Since we've
	// migrated to Postgres this limit is not a concern. Now we only want to limit these values
	// based on the number of elements we can pass to an IN () clause in the codeintel-db, as well
	// as the size required to encode them in a user-facing pagination cursor.
	maximumIndexesPerMonikerSearch int

	authChecker authz.SubRepoPermissionChecker

	RepositoryID api.RepoID
	Commit       api.CommitID
	Path         core.RepoRelPath
}

func (r *RequestState) Attrs() []attribute.KeyValue {
	out := []attribute.KeyValue{
		attribute.Int("repositoryID", int(r.RepositoryID)),
		attribute.String("commit", string(r.Commit)),
		attribute.String("path", r.Path.RawValue()),
	}
	if r.dataLoader != nil {
		uploads := r.dataLoader.uploads
		out = append(out, attribute.Int("numUploads", len(uploads)), attribute.String("uploads", uploadIDsToString(uploads)))
	}
	return out
}

func NewRequestState(
	uploads []shared.CompletedUpload,
	repoStore database.RepoStore,
	authChecker authz.SubRepoPermissionChecker,
	gitserverClient gitserver.Client,
	repo *sgTypes.Repo,
	commit api.CommitID,
	path core.RepoRelPath,
	maxIndexes int,
	hunkCache HunkCache,
) RequestState {
	r := &RequestState{
		// repoStore:    repoStore,
		RepositoryID: repo.ID,
		Commit:       commit,
		Path:         path,
	}
	r.SetUploadsDataLoader(uploads)
	r.SetAuthChecker(authChecker)
	r.SetLocalGitTreeTranslator(gitserverClient, repo, commit, hunkCache)
	r.SetLocalCommitCache(repoStore, gitserverClient)
	r.SetMaximumIndexesPerMonikerSearch(maxIndexes)

	return *r
}

func (r RequestState) GetCacheUploads() []shared.CompletedUpload {
	return r.dataLoader.uploads
}

func (r RequestState) GetCacheUploadsAtIndex(index int) shared.CompletedUpload {
	if index < 0 || index >= len(r.dataLoader.uploads) {
		return shared.CompletedUpload{}
	}

	return r.dataLoader.uploads[index]
}

func (r *RequestState) SetAuthChecker(authChecker authz.SubRepoPermissionChecker) {
	r.authChecker = authChecker
}

func (r *RequestState) SetUploadsDataLoader(uploads []shared.CompletedUpload) {
	r.dataLoader = NewUploadsDataLoader()
	for _, upload := range uploads {
		r.dataLoader.AddUpload(upload)
	}
}

func (r *RequestState) SetLocalGitTreeTranslator(client gitserver.Client, repo *sgTypes.Repo, commit api.CommitID, hunkCache HunkCache) {
	args := &translationBase{
		repo:   repo,
		commit: commit,
	}

	r.GitTreeTranslator = NewGitTreeTranslator(client, args, hunkCache)
}

func (r *RequestState) SetLocalCommitCache(repoStore database.RepoStore, client gitserver.Client) {
	r.commitCache = NewCommitCache(repoStore, client)
}

func (r *RequestState) SetMaximumIndexesPerMonikerSearch(maxNumber int) {
	r.maximumIndexesPerMonikerSearch = maxNumber
}

type UploadsDataLoader struct {
	uploads     []shared.CompletedUpload
	uploadsByID map[int]shared.CompletedUpload
	cacheMutex  sync.RWMutex
}

func NewUploadsDataLoader() *UploadsDataLoader {
	return &UploadsDataLoader{
		uploadsByID: make(map[int]shared.CompletedUpload),
	}
}

func (l *UploadsDataLoader) GetUploadFromCacheMap(id int) (shared.CompletedUpload, bool) {
	l.cacheMutex.RLock()
	defer l.cacheMutex.RUnlock()

	upload, ok := l.uploadsByID[id]
	return upload, ok
}

func (l *UploadsDataLoader) SetUploadInCacheMap(uploads []shared.CompletedUpload) {
	l.cacheMutex.Lock()
	defer l.cacheMutex.Unlock()

	// Sus, compare with AddUpload, where we're also appending the new uploads to l.uploads
	// There seem to be invariants broken here, or not written down elsewhere
	for i := range uploads {
		l.uploadsByID[uploads[i].ID] = uploads[i]
	}
}

func (l *UploadsDataLoader) AddUpload(dump shared.CompletedUpload) {
	l.cacheMutex.Lock()
	defer l.cacheMutex.Unlock()

	l.uploads = append(l.uploads, dump)
	l.uploadsByID[dump.ID] = dump
}
