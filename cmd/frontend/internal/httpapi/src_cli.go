package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/Masterminds/semver"
	"github.com/derision-test/glock"
	"github.com/gorilla/mux"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	srccli "github.com/sourcegraph/sourcegraph/internal/src-cli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	// How long to cache the recommended src-cli version before checking with
	// sourcegraph.com again.
	srcCliCacheLifetime = 10 * time.Minute
	srcCliDownloadsURL  = "https://github.com/sourcegraph/src-cli/releases/download"
	srcCliVersionCache  = "https://sourcegraph.com/.api/src-cli/versions"
)

var allowedFilenames = []string{
	"src_darwin_amd64",
	"src_darwin_arm64",
	"src_linux_amd64",
	"src_linux_arm64",
	"src_windows_amd64.exe",
	"src_windows_arm64.exe",
}

// srcCliVersionHandler is a HTTP handler that can return the current src-cli
// version appropriate for this Sourcegraph instance, along with redirect links
// to download that version from GitHub.
//
// Internally, this is lazily cached, with requests being made to
// sourcegraph.com no more than every srcCliCacheLifetime.
type srcCliVersionHandler struct {
	logger   log.Logger
	maxStale time.Duration

	mu         sync.RWMutex
	lastUpdate time.Time
	version    string

	// Fields used in testing.
	doer  httpcli.Doer
	clock glock.Clock
}

func newSrcCliVersionHandler(logger log.Logger) http.Handler {
	return &srcCliVersionHandler{
		clock:    glock.NewRealClock(),
		doer:     httpcli.ExternalDoer,
		logger:   logger.Scoped("srcCliVersionHandler"),
		maxStale: srcCliCacheLifetime,
	}
}

func (h *srcCliVersionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rest, found := mux.Vars(r)["rest"]
	if !found {
		http.NotFound(w, r)
		return
	}

	if rest == "version" {
		h.handleVersion(w)
	} else if isExpectedRelease(rest) {
		h.handleDownload(w, r, rest)
	} else {
		http.NotFound(w, r)
	}
}

func (h *srcCliVersionHandler) Version() string {
	// There's technically a race condition here: cachedVersion will acquire and
	// release a read lock on the handler mutex, and then (if called)
	// updateCachedVersion will acquire and release a write lock on the same
	// mutex. If Go supported upgradable locks, then we could do this in one
	// lock, but it doesn't and therefore we can't.
	//
	// Practically, what this means is that there may be more than one request
	// waiting to update the cached version at a time, which may result in extra
	// hits on the sourcegraph.com endpoint that provides src-cli version
	// metadata if multiple requests come in while the cached version is stale.
	// This should be fine: that endpoint has its own caching, and the write
	// lock isn't held while we wait for sourcegraph.com to respond; it's only
	// held long enough to actually update the handler's fields, which should be
	// extremely fast.
	version := h.cachedVersion()
	if version != "" {
		return version
	}

	version, err := h.updateCachedVersion()
	if err != nil {
		// We can't do much here: we'll log the error (at a low level so
		// airgapped instances don't fill up their logs with warnings), and then
		// return the minimum version hardcoded in the src-cli package.
		h.logger.Debug("cannot access sourcegraph.com version cache", log.Error(err))
		return srccli.MinimumVersion
	}

	return version
}

func (h *srcCliVersionHandler) cachedVersion() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.clock.Now().Sub(h.lastUpdate) > h.maxStale {
		return ""
	}
	return h.version
}

func (h *srcCliVersionHandler) updateCachedVersion() (string, error) {
	minimumVersion, err := semver.NewVersion(srccli.MinimumVersion)
	if err != nil {
		return "", errors.New("parsing minimum version")
	}

	urlStr := fmt.Sprintf("%s/%d.%d", srcCliVersionCache, minimumVersion.Major(), minimumVersion.Minor())
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return "", errors.Wrap(err, "building request")
	}

	resp, err := h.doer.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "getting version from Sourcegraph")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", errors.Newf("unexpected status code: %d", resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	var version string
	if err := dec.Decode(&version); err != nil {
		return "", errors.Wrap(err, "reading version from response payload")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastUpdate = h.clock.Now()
	h.version = version

	return version, nil
}

func (h *srcCliVersionHandler) handleDownload(w http.ResponseWriter, r *http.Request, filename string) {
	u, err := url.Parse(srcCliDownloadsURL)
	if err != nil {
		h.logger.Error("Illegal base src-cli download URL", log.String("url", srcCliDownloadsURL), log.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	u.Path = path.Join(u.Path, h.Version(), filename)
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (h *srcCliVersionHandler) handleVersion(w http.ResponseWriter) {
	writeJSON(w, struct {
		Version string `json:"version"`
	}{Version: h.Version()})
}

func isExpectedRelease(filename string) bool {
	for _, v := range allowedFilenames {
		if filename == v {
			return true
		}
	}
	return false
}
