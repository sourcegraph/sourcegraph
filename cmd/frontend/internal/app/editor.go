package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/cloneurls"
	"github.com/sourcegraph/sourcegraph/internal/api"
)

func editorRev(ctx context.Context, repoName api.RepoName, rev string, beExplicit bool) (string, error) {
	if beExplicit {
		return "@" + rev, nil
	}
	if rev == "HEAD" {
		return "", nil // Detached head state
	}
	repo, err := backend.Repos.GetByName(ctx, repoName)
	if err != nil {
		// We weren't able to fetch the repo. This means it either doesn't
		// exist (unlikely) or that the user is not logged in (most likely). In
		// this case, the best user experience is to send them to the branch
		// they asked for. The front-end will inform them if the branch does
		// not exist.
		return "@" + rev, nil
	}
	// If we are on the default branch we want to return a clean URL without a
	// branch. If we fail its best to return the full URL and allow the
	// front-end to inform them of anything that is wrong.
	defaultBranchCommitID, err := backend.Repos.ResolveRev(ctx, repo, "")
	if err != nil {
		return "@" + rev, nil
	}
	branchCommitID, err := backend.Repos.ResolveRev(ctx, repo, rev)
	if err != nil {
		return "@" + rev, nil
	}
	if defaultBranchCommitID == branchCommitID {
		return "", nil // default branch, so make a clean URL without a branch.
	}
	return "@" + rev, nil
}

// editorRequest represents the parameters to a Sourcegraph "open file", "search", etc. editor request.
type editorRequest struct {
	// Fields that are required in all requests.
	editor  string // editor name, e.g. "Atom", "Sublime", etc.
	version string // editor extension version

	// Fields that are optional in all requests.
	utmProductName    string // Editor product name. Only present in JetBrains today (e.g. "IntelliJ", "GoLand")
	utmProductVersion string // Editor product version. Only present in JetBrains today.

	// openFileRequest is non-nil if this is an "open file on Sourcegraph" request.
	openFileRequest *editorOpenFileRequest

	// searchRequest is non-nil if this is a "search on Sourcegraph" request.
	searchRequest *editorSearchRequest
}

// editorSearchRequest represents parameters for "open file on Sourcegraph" editor requests.
type editorOpenFileRequest struct {
	remoteURL         string            // Git repository remote URL.
	hostnameToPattern map[string]string // Map of Git remote URL hostnames to patterns describing how they map to Sourcegraph repositories
	branch            string            // Git branch name.
	revision          string            // Git revision.
	file              string            // Unix filepath relative to repository root.

	// Zero-based cursor selection parameters. Required.
	startRow, endRow int
	startCol, endCol int
}

// editorSearchRequest represents parameters for "search on Sourcegraph" editor requests.
type editorSearchRequest struct {
	query string // The literal search query

	// Optional git repository remote URL. When present, the search will be performed just
	// in the repository (not globally).
	remoteURL         string
	hostnameToPattern map[string]string // Map of Git remote URL hostnames to patterns describing how they map to Sourcegraph repositories

	// Optional git repository branch name and revision. When one is present and remoteURL
	// is present, the search will be performed just at this branch/revision.
	branch   string
	revision string

	// Optional unix filepath relative to the repository root. When present, the search
	// will be performed with a file: search filter.
	file string
}

// addTracking adds the tracking ?utm_... parameters to the given query values.
func (r *editorRequest) addTracking(q url.Values) {
	q.Add("utm_source", r.editor+"-"+r.version)
	if r.utmProductName != "" {
		q.Add("utm_product_name", r.utmProductName)
	}
	if r.utmProductVersion != "" {
		q.Add("utm_product_version", r.utmProductVersion)
	}
}

// searchRedirect returns the redirect URL for the pre-validated search request.
func (r *editorRequest) searchRedirect(ctx context.Context) (string, error) {
	s := r.searchRequest

	// Handle searches scoped to a specific repository.
	var repoFilter string
	if s.remoteURL != "" {
		// Search in this repository.
		repoName, err := cloneurls.ReposourceCloneURLToRepoName(ctx, s.remoteURL)
		if err != nil {
			return "", err
		}
		if repoName == "" {
			// Any error here is a problem with the user's configured git remote
			// URL. We want them to actually read this error message.
			return "", fmt.Errorf("Git remote URL %q not supported", s.remoteURL)
		}
		// Note: we do not use ^ at the front of the repo filter because repoName may
		// produce imprecise results and a suffix match seems better than no match.
		repoFilter = "repo:" + regexp.QuoteMeta(string(repoName)) + "$"
	}

	// Handle searches scoped to a specific revision/branch.
	if repoFilter != "" && s.revision != "" {
		// Search in just this revision.
		repoFilter += "@" + s.revision
	} else if repoFilter != "" && s.branch != "" {
		// Search in just this branch.
		repoFilter += "@" + s.branch
	}

	// Handle searches scoped to a specific file.
	var fileFilter string
	if s.file != "" {
		fileFilter = "file:^" + regexp.QuoteMeta(s.file) + "$"
	}

	// Compose the final search query.
	parts := make([]string, 0, 3)
	for _, part := range []string{repoFilter, fileFilter, r.searchRequest.query} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	searchQuery := strings.Join(parts, " ")

	// Build the redirect URL.
	u := &url.URL{Path: "/search"}
	q := u.Query()
	q.Add("q", searchQuery)
	q.Add("patternType", "literal")
	r.addTracking(q)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// openFile returns the redirect URL for the pre-validated open-file request.
func (r *editorRequest) openFileRedirect(ctx context.Context) (string, error) {
	of := r.openFileRequest
	// Determine the repo name and branch.
	repoName, err := cloneurls.ReposourceCloneURLToRepoName(ctx, of.remoteURL)
	if err != nil {
		return "", err
	}
	if repoName == "" {
		// Any error here is a problem with the user's configured git remote
		// URL. We want them to actually read this error message.
		return "", fmt.Errorf("git remote URL %q not supported", of.remoteURL)
	}

	inputRev, beExplicit := of.revision, true
	if inputRev == "" {
		inputRev, beExplicit = of.branch, false
	}
	rev, err := editorRev(ctx, repoName, inputRev, beExplicit)
	if err != nil {
		return "", err
	}

	u := &url.URL{Path: path.Join("/", string(repoName)+rev, "/-/blob/", of.file)}
	q := u.Query()
	r.addTracking(q)
	u.RawQuery = q.Encode()
	if of.startRow == of.endRow && of.startCol == of.endCol {
		u.Fragment = fmt.Sprintf("L%d:%d", of.startRow+1, of.startCol+1)
	} else {
		u.Fragment = fmt.Sprintf("L%d:%d-%d:%d", of.startRow+1, of.startCol+1, of.endRow+1, of.endCol+1)
	}
	return u.String(), nil
}

// openFile returns the redirect URL for the pre-validated request.
func (r *editorRequest) redirectURL(ctx context.Context) (string, error) {
	if r.searchRequest != nil {
		return r.searchRedirect(ctx)
	} else if r.openFileRequest != nil {
		return r.openFileRedirect(ctx)
	}
	return "", errors.New("could not determine request type, missing ?search or ?remote_url")
}

// parseEditorRequest parses an editor request from the search query values.
func parseEditorRequest(q url.Values) (*editorRequest, error) {
	v := &editorRequest{
		editor:            q.Get("editor"),
		version:           q.Get("version"),
		utmProductName:    q.Get("utm_product_name"),
		utmProductVersion: q.Get("utm_product_name"),
	}
	if v.editor == "" {
		return nil, errors.New("expected URL parameter missing: editor=$EDITOR_NAME")
	}
	if v.version == "" {
		return nil, errors.New("expected URL parameter missing: version=$EDITOR_EXTENSION_VERSION")
	}

	if search := q.Get("search"); search != "" {
		// Search request parsing
		v.searchRequest = &editorSearchRequest{
			query:     q.Get("search"),
			remoteURL: q.Get("search_remote_url"),
			branch:    q.Get("search_branch"),
			revision:  q.Get("search_revision"),
			file:      q.Get("search_file"),
		}
		if hostnameToPatternStr := q.Get("search_hostname_patterns"); hostnameToPatternStr != "" {
			if err := json.Unmarshal([]byte(hostnameToPatternStr), &v.searchRequest.hostnameToPattern); err != nil {
				return nil, err
			}
		}
	} else if remoteURL := q.Get("remote_url"); remoteURL != "" {
		// Open-file request parsing
		startRow, _ := strconv.Atoi(q.Get("start_row"))
		endRow, _ := strconv.Atoi(q.Get("end_row"))
		startCol, _ := strconv.Atoi(q.Get("start_col"))
		endCol, _ := strconv.Atoi(q.Get("end_col"))
		v.openFileRequest = &editorOpenFileRequest{
			remoteURL: remoteURL,
			branch:    q.Get("branch"),
			revision:  q.Get("revision"),
			file:      q.Get("file"),
			startRow:  startRow,
			endRow:    endRow,
			startCol:  startCol,
			endCol:    endCol,
		}
		if hostnameToPatternStr := q.Get("hostname_patterns"); hostnameToPatternStr != "" {
			if err := json.Unmarshal([]byte(hostnameToPatternStr), &v.openFileRequest.hostnameToPattern); err != nil {
				return nil, err
			}
		}
	}
	return v, nil
}

func serveEditor(w http.ResponseWriter, r *http.Request) error {
	editorRequest, err := parseEditorRequest(r.URL.Query())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "%s", err.Error())
		return nil
	}
	redirectURL, err := editorRequest.redirectURL(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "%s", err.Error())
		return nil
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	return nil
}
