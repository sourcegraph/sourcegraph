package releasecache

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"

	gh "github.com/google/go-github/v55/github"
	"github.com/gorilla/mux"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
)

// handler implements a http.Handler that wraps a VersionCache to provide two
// endpoints:
//
//   - GET /.*: this looks up the given branch and returns the latest
//     version, if any.
//   - POST /webhooks: this triggers an update of the version cache if given a
//     valid GitHub webhook.
//
// The routing relies on a previous handler having injected a gorilla.Mux
// variable called "rest" that includes the path to route.
type handler struct {
	logger log.Logger

	mu            sync.Mutex
	enabled       bool
	rc            ReleaseCache
	updater       *goroutine.PeriodicGoroutine
	webhookSecret string
}

func NewHandler(logger log.Logger) http.Handler {
	ctx := context.Background()
	logger = logger.Scoped("srcclicache")

	handler := &handler{
		logger: logger.Scoped("handler"),
	}

	// We'll build all the state up in a conf watcher, since the behaviour of
	// this handler is entirely dependent on the current site config.
	conf.Watch(func() {
		config, err := parseSiteConfig(conf.Get())
		if err != nil {
			logger.Error("error parsing release cache config", log.Error(err))
			return
		}

		handler.mu.Lock()
		defer handler.mu.Unlock()

		// If we already have an updater goroutine running, we need to stop it.
		if handler.updater != nil {
			handler.updater.Stop()
			handler.updater = nil
		}

		// If the cache should be disabled, then we can return here, since we've
		// already stopped any updater that was running.
		handler.enabled = config.enabled
		if !handler.enabled {
			return
		}

		// Otherwise, let's build a new release cache and start a fresh updater.
		rc, err := config.NewReleaseCache(logger)
		if err != nil {
			logger.Error("Failed to create GitHub client", log.Error(err))
			return
		}

		handler.updater = goroutine.NewPeriodicGoroutine(
			ctx,
			rc,
			goroutine.WithName("srccli.github-release-cache"),
			goroutine.WithDescription("caches src-cli versions polled periodically"),
			goroutine.WithInterval(config.interval),
		)
		go goroutine.MonitorBackgroundRoutines(ctx, handler.updater)

		handler.rc = rc
		handler.webhookSecret = config.webhookSecret
	})

	return handler
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If the version cache is disabled, then we can just return a 404 and be
	// done.
	if !h.enabled {
		http.NotFound(w, r)
		return
	}

	// Pull the remainder of the path to route out of the mux variables.
	rest, ok := mux.Vars(r)["rest"]
	if !ok {
		http.Error(w, "cannot access route", http.StatusBadRequest)
		return
	}

	// We'll just hardcode the routing logic here — there are only two
	// endpoints, so throwing a full mux.Router at this feels pointless.
	if r.Method == "POST" {
		if rest == "webhook" {
			h.handleWebhook(w, r)
		} else {
			http.Error(w, "cannot POST to endpoint", http.StatusMethodNotAllowed)
		}
	} else {
		h.handleBranch(w, rest)
	}
}

func (h *handler) handleBranch(w http.ResponseWriter, branch string) {
	version, err := h.rc.Current(branch)
	if err != nil {
		if errcode.IsNotFound(err) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			h.logger.Warn("error getting current branch", log.Error(err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	raw, err := json.Marshal(version)
	if err != nil {
		h.logger.Warn("error marshalling version to JSON", log.String("version", version))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(raw)
}

func (h *handler) handleWebhook(w http.ResponseWriter, r *http.Request) {
	h.doHandleWebhook(w, r, gh.ValidateSignature)
}

type signatureValidator func(signature string, payload []byte, secret []byte) error

func (h *handler) doHandleWebhook(w http.ResponseWriter, r *http.Request, signatureValidator signatureValidator) {
	defer r.Body.Close()
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Warn("error reading payload", log.Error(err))
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if err := signatureValidator(r.Header.Get("X-Hub-Signature"), payload, []byte(h.webhookSecret)); err != nil {
		h.logger.Warn("cannot validate webhook signature", log.Error(err))
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	// Rather than interrogating the payload, we'll just refresh the entire
	// cache.
	h.logger.Debug("received valid release webhook; refreshing release cache")
	if err := h.rc.UpdateNow(context.Background()); err != nil {
		h.logger.Error("error updating the release cache in response to a webhook", log.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
