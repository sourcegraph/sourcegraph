package handlerutil

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/routevar"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// GetRepo gets the repo (from the reposSvc) specified in the URL's
// Repo route param. Callers should ideally check for a return error of type
// URLMovedError and handle this scenario by warning or redirecting the user.
func GetRepo(ctx context.Context, vars map[string]string) (*types.Repo, error) {
	origRepo := routevar.ToRepo(vars)

	repo, err := backend.Repos.GetByName(ctx, origRepo)
	if err != nil {
		return nil, err
	}

	if origRepo != repo.Name {
		return nil, &URLMovedError{repo.Name}
	}

	return repo, nil
}

// getRepoRev resolves the repository and commit specified in the route vars.
func getRepoRev(ctx context.Context, vars map[string]string, repoID api.RepoID) (api.RepoID, api.CommitID, error) {
	repoRev := routevar.ToRepoRev(vars)
	repo, err := backend.Repos.Get(ctx, repoID)
	if err != nil {
		return repoID, "", err
	}
	commitID, err := backend.Repos.ResolveRev(ctx, repo, repoRev.Rev)
	if err != nil {
		return repoID, "", err
	}

	return repoID, commitID, nil
}

// GetRepoAndRev returns the repo object and the commit ID for a repository. It may
// also return custom error URLMovedError to allow special handling of this case,
// such as for example redirecting the user.
func GetRepoAndRev(ctx context.Context, vars map[string]string) (*types.Repo, api.CommitID, error) {
	repo, err := GetRepo(ctx, vars)
	if err != nil {
		return repo, "", err
	}

	_, commitID, err := getRepoRev(ctx, vars, repo.ID)
	return repo, commitID, err
}

// RedirectToNewRepoName writes an HTTP redirect response with a
// Location that matches the request's location except with the
// Repo route var updated to refer to newRepoName (instead of the
// originally requested repo name).
func RedirectToNewRepoName(w http.ResponseWriter, r *http.Request, newRepoName api.RepoName) error {
	origVars := mux.Vars(r)
	origVars["Repo"] = string(newRepoName)

	var pairs []string
	for k, v := range origVars {
		pairs = append(pairs, k, v)
	}
	destURL, err := mux.CurrentRoute(r).URLPath(pairs...)
	if err != nil {
		return err
	}

	http.Redirect(w, r, destURL.String(), http.StatusMovedPermanently)
	return nil
}
