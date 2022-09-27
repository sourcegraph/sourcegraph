package http

import (
	"context"
	"net/http"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	uploads "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/uploadhandler"
	"github.com/sourcegraph/sourcegraph/internal/uploadstore"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Handler struct {
	svc        *uploads.Service
	operations *operations
}

func newHandler(svc *uploads.Service, operations *operations) http.Handler {
	return newUploadHandler(
		nil, // TODO
		nil, // TODO
		nil, // TODO
		operations,
	)
}

var revhashPattern = lazyregexp.New(`^[a-z0-9]{40}$`)

func newUploadHandler(
	db database.DB,
	dbStore uploadhandler.DBStore[UploadMetadata],
	uploadStore uploadstore.Store,
	operations *operations,
) http.Handler {
	logger := log.Scoped("UploadHandler", "")

	metadataFromRequest := func(ctx context.Context, r *http.Request) (UploadMetadata, int, error) {
		commit := getQuery(r, "commit")
		if !revhashPattern.Match([]byte(commit)) {
			return UploadMetadata{}, http.StatusBadRequest, errors.Errorf("commit must be a 40-character revhash")
		}

		// Ensure that the repository and commit given in the request are resolvable.
		repositoryName := getQuery(r, "repository")
		repositoryID, statusCode, err := ensureRepoAndCommitExist(ctx, logger, db, repositoryName, commit)
		if err != nil {
			return UploadMetadata{}, statusCode, err
		}

		// Populate state from request
		return UploadMetadata{
			RepositoryID:      repositoryID,
			Commit:            commit,
			Root:              sanitizeRoot(getQuery(r, "root")),
			Indexer:           getQuery(r, "indexerName"),
			IndexerVersion:    getQuery(r, "indexerVersion"),
			AssociatedIndexID: getQueryInt(r, "associatedIndexId"),
		}, 0, nil
	}

	handler := uploadhandler.NewUploadHandler(
		logger,
		dbStore,
		uploadStore,
		operations.Operations,
		metadataFromRequest,
	)

	return handler
}

func ensureRepoAndCommitExist(ctx context.Context, logger log.Logger, db database.DB, repoName, commit string) (int, int, error) {
	// 🚨 SECURITY: Bypass authz here; we've already determined that the current request is
	// authorized to view the target repository; they are either a site admin or the code
	// host has explicit listed them with some level of access (depending on the code host).
	ctx = actor.WithInternalActor(ctx)

	//
	// 1. Resolve repository

	repo, err := backend.NewRepos(logger, db).GetByName(ctx, api.RepoName(repoName))
	if err != nil {
		if errcode.IsNotFound(err) {
			return 0, http.StatusNotFound, errors.Errorf("unknown repository %q", repoName)
		}

		return 0, http.StatusInternalServerError, err
	}

	//
	// 2. Resolve commit

	if _, err := backend.NewRepos(logger, db).ResolveRev(ctx, repo, commit); err != nil {
		var reason string
		if errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
			reason = "commit not found"
		} else if gitdomain.IsCloneInProgress(err) {
			reason = "repository still cloning"
		} else {
			return 0, http.StatusInternalServerError, err
		}

		logger.Warn("Accepting LSIF upload with unresolvable commit", log.String("reason", reason))
	}

	return int(repo.ID), 0, nil
}
