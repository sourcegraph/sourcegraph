package internal

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mxk/go-flowrate/flowrate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/accesslog"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	envGitServiceMaxEgressBytesPerSecond = env.Get(
		"SRC_GIT_SERVICE_MAX_EGRESS_BYTES_PER_SECOND",
		"10000000000",
		"Git service egress rate limit in bytes per second (-1 = no limit, default = 10Gbps)")

	// gitServiceMaxEgressBytesPerSecond must be retrieved by getGitServiceMaxEgressBytesPerSecond,
	// which parses envGitServiceMaxEgressBytesPerSecond once and logs any error encountered
	// when parsing.
	gitServiceMaxEgressBytesPerSecond        int64
	getGitServiceMaxEgressBytesPerSecondOnce sync.Once
)

// NewHTTPHandler returns a HTTP handler that serves a git upload pack server,
// plus a few other endpoints.
func NewHTTPHandler(logger log.Logger, fs gitserverfs.FS, backendSource git.GitBackendSource) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", trace.WithRouteName("ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// This endpoint allows us to expose gitserver itself as a "git service"
	// (ETOOMANYGITS!) that allows other services to run commands like "git fetch"
	// directly against a gitserver replica and treat it as a git remote.
	//
	// Example use case for this is a repo migration from one replica to another during
	// scaling events and the new destination gitserver replica can directly clone from
	// the gitserver replica which hosts the repository currently.
	mux.HandleFunc("/git/", trace.WithRouteName("git", accesslog.HTTPMiddleware(
		logger.Scoped("gitserver.gitservice"),
		conf.DefaultClient(),
		func(rw http.ResponseWriter, r *http.Request) {
			logger := logger.Scoped("gitUploadPack")

			// Only support clones and fetches (git upload-pack). /info/refs sets the
			// service field.
			if svcQ := r.URL.Query().Get("service"); svcQ != "" && svcQ != "git-upload-pack" {
				http.Error(rw, "only support service git-upload-pack", http.StatusBadRequest)
				return
			}

			var repo, svc string
			for _, suffix := range []string{"/info/refs", "/git-upload-pack"} {
				if strings.HasSuffix(r.URL.Path, suffix) {
					svc = suffix
					repo = strings.TrimSuffix(r.URL.Path, suffix)
					repo = strings.TrimPrefix(repo, "/git")
					repo = strings.TrimPrefix(repo, "/")
					break
				}
			}
			if repo == "" {
				http.Error(rw, "no repo specified", http.StatusBadRequest)
				return
			}

			repoName := api.RepoName(repo)
			protocol := r.Header.Get("Git-Protocol")

			// Log which which actor is accessing the repo.
			accesslog.Record(r.Context(), repo,
				log.String("svc", svc),
				log.String("protocol", protocol),
			)

			dir := fs.RepoDir(repoName)
			cloned, err := fs.RepoCloned(repoName)
			if err != nil {
				http.Error(rw, "failed to check if repo is cloned: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if !cloned {
				http.Error(rw, fmt.Sprintf("repository %q not found", repoName), http.StatusNotFound)
				return
			}

			backend := backendSource(dir, repoName)

			body := r.Body
			defer body.Close()

			if r.Header.Get("Content-Encoding") == "gzip" {
				gzipReader, err := gzip.NewReader(body)
				if err != nil {
					http.Error(rw, "malformed payload: "+err.Error(), http.StatusBadRequest)
					return
				}
				defer gzipReader.Close()

				body = gzipReader
			}

			start := time.Now()
			metricServiceRunning.WithLabelValues(svc).Inc()

			finalizeMetrics := func(err error) {
				errLabel := strconv.FormatBool(err != nil)
				metricServiceRunning.WithLabelValues(svc).Dec()
				metricServiceDuration.WithLabelValues(svc, errLabel).Observe(time.Since(start).Seconds())

				fields := []log.Field{
					log.String("svc", svc),
					log.String("repo", repo),
					log.String("protocol", protocol),
					log.Duration("duration", time.Since(start)),
				}

				if err != nil {
					logger.Error("gitservice.ServeHTTP", append(fields, log.Error(err))...)
				} else if traceLogs {
					logger.Debug("gitservice.ServeHTTP", fields...)
				}
			}

			advertiseRefs := false
			switch svc {
			case "/info/refs":
				rw.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
				_, _ = rw.Write(packetWrite("# service=git-upload-pack\n"))
				_, _ = rw.Write([]byte("0000"))
				advertiseRefs = true
			case "/git-upload-pack":
				rw.Header().Set("Content-Type", "application/x-git-upload-pack-result")
			default:
				err = errors.Errorf("unexpected subpath (want /info/refs or /git-upload-pack): %q", svc)
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				finalizeMetrics(err)
				return
			}

			uploadPackReader, err := backend.UploadPack(r.Context(), body, protocol, advertiseRefs)
			if err != nil {
				err = errors.Wrap(err, "gitserver: failed to run git command")
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				finalizeMetrics(err)
				return
			}

			w := flowrateWriter(logger, rw)
			_, err = io.Copy(w, uploadPackReader)
			if err != nil {
				err = errors.Wrap(err, "failed to run git upload pack")
				logger.Error("git-service error", log.Error(err))
				_, _ = w.Write([]byte("\n" + err.Error() + "\n"))
			}

			_ = uploadPackReader.Close()
			finalizeMetrics(err)
		},
	)))

	return mux
}

func packetWrite(str string) []byte {
	s := strconv.FormatInt(int64(len(str)+4), 16)
	if len(s)%4 != 0 {
		s = strings.Repeat("0", 4-len(s)%4) + s
	}
	return []byte(s + str)
}

// getGitServiceMaxEgressBytesPerSecond parses envGitServiceMaxEgressBytesPerSecond once
// and returns the same value on subsequent calls.
func getGitServiceMaxEgressBytesPerSecond(logger log.Logger) int64 {
	getGitServiceMaxEgressBytesPerSecondOnce.Do(func() {
		var err error
		gitServiceMaxEgressBytesPerSecond, err = strconv.ParseInt(envGitServiceMaxEgressBytesPerSecond, 10, 64)
		if err != nil {
			gitServiceMaxEgressBytesPerSecond = 10 * 1000 * 1000 * 1000 // 1G0bps
			logger.Error("failed parsing SRC_GIT_SERVICE_MAX_EGRESS_BYTES_PER_SECOND, defaulting to 1Gbps",
				log.Int64("bps", gitServiceMaxEgressBytesPerSecond),
				log.Error(err))
		}
	})

	return gitServiceMaxEgressBytesPerSecond
}

// flowrateWriter limits the write rate of w to 10 Gbps.
//
// We are cloning repositories from within the same network from another
// Sourcegraph service (zoekt-indexserver). This can end up being so fast that
// we harm our own network connectivity. In the case of zoekt-indexserver and
// gitserver running on the same host machine, we can even reach up to ~100
// Gbps and effectively DoS the Docker network, temporarily disrupting other
// containers running on the host.
//
// Google Compute Engine has a network bandwidth of about 1.64 Gbps
// between nodes, and AWS varies widely depending on instance type.
// We play it safe and default to 1 Gbps here (~119 MiB/s), which
// means we can fetch a 1 GiB archive in ~8.5 seconds.
func flowrateWriter(logger log.Logger, w io.Writer) io.Writer {
	if limit := getGitServiceMaxEgressBytesPerSecond(logger); limit > 0 {
		return flowrate.NewWriter(w, limit)
	}
	return w
}

var (
	metricServiceDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "src_gitserver_gitservice_duration_seconds",
		Help:    "A histogram of latencies for the git service (upload-pack for internal clones) endpoint.",
		Buckets: prometheus.ExponentialBuckets(.1, 4, 9),
		// [0.1 0.4 1.6 6.4 25.6 102.4 409.6 1638.4 6553.6]
	}, []string{"type", "error"})

	metricServiceRunning = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "src_gitserver_gitservice_running",
		Help: "A histogram of latencies for the git service (upload-pack for internal clones) endpoint.",
	}, []string{"type"})
)
