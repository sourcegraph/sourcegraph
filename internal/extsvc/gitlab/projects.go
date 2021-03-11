package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/peterhellberg/link"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Visibility string

const (
	Public   Visibility = "public"
	Private  Visibility = "private"
	Internal Visibility = "internal"
)

// Project is a GitLab project (equivalent to a GitHub repository).
type Project struct {
	ProjectCommon
	Visibility        Visibility     `json:"visibility"`                    // "private", "internal", or "public"
	ForkedFromProject *ProjectCommon `json:"forked_from_project,omitempty"` // If non-nil, the project from which this project was forked
	Archived          bool           `json:"archived"`
}

type ProjectCommon struct {
	ID                int    `json:"id"`                  // ID of project
	PathWithNamespace string `json:"path_with_namespace"` // full path name of project ("namespace1/namespace2/name")
	Description       string `json:"description"`         // description of project
	WebURL            string `json:"web_url"`             // the web URL of this project ("https://gitlab.com/foo/bar")i
	HTTPURLToRepo     string `json:"http_url_to_repo"`    // HTTP clone URL
	SSHURLToRepo      string `json:"ssh_url_to_repo"`     // SSH clone URL ("git@example.com:foo/bar.git")
}

// RequiresAuthentication reports whether this project requires authentication to view (i.e., its visibility is
// "private" or "internal").
func (p Project) RequiresAuthentication() bool {
	return p.Visibility == "private" || p.Visibility == "internal"
}

func idCacheKey(id int) string                                  { return "1:" + strconv.Itoa(id) }
func pathWithNamespaceCacheKey(pathWithNamespace string) string { return "1:" + pathWithNamespace }

// MockGetProject_Return is called by tests to mock (*Client).GetProject.
func MockGetProject_Return(returns *Project) {
	MockGetProject = func(*Client, context.Context, GetProjectOp) (*Project, error) {
		return returns, nil
	}
}

type GetProjectOp struct {
	ID                int
	PathWithNamespace string
	CommonOp
}

// GetProject gets a project from GitLab by either ID or path with namespace.
func (c *Client) GetProject(ctx context.Context, op GetProjectOp) (*Project, error) {
	if op.ID != 0 && op.PathWithNamespace != "" {
		panic("invalid args (specify exactly one of id and pathWithNamespace)")
	}

	if MockGetProject != nil {
		return MockGetProject(c, ctx, op)
	}

	var key string
	if op.ID != 0 {
		key = idCacheKey(op.ID)
	} else {
		key = pathWithNamespaceCacheKey(op.PathWithNamespace)
	}
	return c.cachedGetProject(ctx, key, op.NoCache, func(ctx context.Context) (proj *Project, keys []string, err error) {
		keys = append(keys, key)
		proj, err = c.getProjectFromAPI(ctx, op.ID, op.PathWithNamespace)
		if proj != nil {
			// Add the cache key for the other kind of specifier (ID vs. path with namespace) so it's addressable by
			// both in the cache.
			if op.ID != 0 {
				keys = append(keys, pathWithNamespaceCacheKey(proj.PathWithNamespace))
			} else {
				keys = append(keys, idCacheKey(proj.ID))
			}
		}
		return proj, keys, err
	})
}

// cachedGetProject caches the getProjectFromAPI call.
func (c *Client) cachedGetProject(ctx context.Context, key string, forceFetch bool, getProjectFromAPI func(context.Context) (proj *Project, keys []string, err error)) (*Project, error) {
	if !forceFetch {
		if cached := c.getProjectFromCache(ctx, key); cached != nil {
			projectsGitLabCacheCounter.WithLabelValues("hit").Inc()
			if cached.NotFound {
				return nil, ErrProjectNotFound
			}
			return &cached.Project, nil
		}
	}

	proj, keys, err := getProjectFromAPI(ctx)
	if IsNotFound(err) {
		// Before we do anything, ensure we cache NotFound responses.
		// Do this if client is unauthed or authed, it's okay since we're only caching not found responses here.
		c.addProjectToCache(keys, &cachedProj{NotFound: true})
		projectsGitLabCacheCounter.WithLabelValues("notfound").Inc()
	}
	if err != nil {
		projectsGitLabCacheCounter.WithLabelValues("error").Inc()
		return nil, err
	}

	c.addProjectToCache(keys, &cachedProj{Project: *proj})
	projectsGitLabCacheCounter.WithLabelValues("miss").Inc()

	return proj, nil
}

var projectsGitLabCacheCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "src_projs_gitlab_cache_hit",
	Help: "Counts cache hits and misses for GitLab project metadata.",
}, []string{"type"})

type cachedProj struct {
	Project

	// NotFound indicates that the GitLab API reported that the project was not found.
	NotFound bool
}

// getProjectFromCache attempts to get a response from the redis cache.
// It returns nil error for cache-hit condition and non-nil error for cache-miss.
func (c *Client) getProjectFromCache(ctx context.Context, key string) *cachedProj {
	b, ok := c.projCache.Get(strings.ToLower(key))
	if !ok {
		return nil
	}

	var cached cachedProj
	if err := json.Unmarshal(b, &cached); err != nil {
		return nil
	}

	return &cached
}

// addProjectToCache will cache the value for proj. The caller can provide multiple cache keys for the multiple
// ways that this project can be retrieved (e.g., both ID and path with namespace).
func (c *Client) addProjectToCache(keys []string, proj *cachedProj) {
	b, err := json.Marshal(proj)
	if err != nil {
		return
	}
	for _, key := range keys {
		c.projCache.Set(strings.ToLower(key), b)
	}
}

// getProjectFromAPI attempts to fetch a project from the GitLab API without use of the redis cache.
func (c *Client) getProjectFromAPI(ctx context.Context, id int, pathWithNamespace string) (proj *Project, err error) {
	var urlParam string
	if id != 0 {
		urlParam = strconv.Itoa(id)
	} else {
		urlParam = url.PathEscape(pathWithNamespace) // https://docs.gitlab.com/ce/api/README.html#namespaced-path-encoding
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("projects/%s", urlParam), nil)
	if err != nil {
		return nil, err
	}
	_, _, err = c.do(ctx, req, &proj)
	return proj, err
}

// ListProjects lists GitLab projects.
func (c *Client) ListProjects(ctx context.Context, urlStr string) (projs []*Project, nextPageURL *string, err error) {
	if MockListProjects != nil {
		return MockListProjects(c, ctx, urlStr)
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, nil, err
	}
	respHeader, _, err := c.do(ctx, req, &projs)
	if err != nil {
		return nil, nil, err
	}

	// Get URL to next page. See https://docs.gitlab.com/ee/api/README.html#pagination-link-header.
	if l := link.Parse(respHeader.Get("Link"))["next"]; l != nil {
		nextPageURL = &l.URI
	}

	// Add to cache.
	for _, proj := range projs {
		keys := []string{pathWithNamespaceCacheKey(proj.PathWithNamespace), idCacheKey(proj.ID)} // cache under multiple
		c.addProjectToCache(keys, &cachedProj{Project: *proj})
	}

	return projs, nextPageURL, nil
}
