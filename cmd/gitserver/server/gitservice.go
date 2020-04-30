package server

import (
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var uploadPackArgs = []string{
	// Partial clones/fetches
	"-c", "uploadpack.allowFilter=true",

	// Can fetch any object. Used in case of race between a resolve ref and a
	// fetch of a commit. Safe to do, since this is only used internally.
	"-c", "uploadpack.allowAnySHA1InWant=true",

	"upload-pack",

	"--stateless-rpc", "--strict",
}

// gitServiceHandler is a smart Git HTTP transfer protocol as documented at
// https://www.git-scm.com/docs/http-protocol.
//
// This allows users to clone any git repo. We only support the smart
// protocol. We aim to support modern git features such as protocol v2 to
// minimize traffic.
type gitServiceHandler struct {
	// Dir is a funcion which takes a repository name and returns an absolute
	// path to the GIT_DIR for it.
	Dir func(string) string
}

func (s *gitServiceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only support clones and fetches (git upload-pack). /info/refs sets the
	// service field.
	if svcQ := r.URL.Query().Get("service"); svcQ != "" && svcQ != "git-upload-pack" {
		http.Error(w, "only support service git-upload-pack", http.StatusBadRequest)
		return
	}

	var repo, svc string
	for _, suffix := range []string{"/info/refs", "/git-upload-pack"} {
		if strings.HasSuffix(r.URL.Path, suffix) {
			svc = suffix
			repo = strings.TrimSuffix(r.URL.Path, suffix)
			repo = strings.TrimPrefix(repo, "/")
			break
		}
	}

	dir := s.Dir(repo)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "failed to stat repo: "+err.Error(), http.StatusInternalServerError)
		return
	}

	start := time.Now()
	metricServiceRunning.WithLabelValues(svc).Inc()
	defer func() {
		metricServiceRunning.WithLabelValues(svc).Dec()
		metricServiceDuration.WithLabelValues(svc).Observe(time.Since(start).Seconds())
		if traceLogs {
			log15.Debug("TRACE gitserver git service", "svc", svc, "repo", repo, "protocol", r.Header.Get("Git-Protocol"), "duration", time.Since(start))
		}
	}()

	args := append([]string{}, uploadPackArgs...)
	switch svc {
	case "/info/refs":
		w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
		w.Write(packetWrite("# service=git-upload-pack\n"))
		w.Write([]byte("0000"))
		args = append(args, "--advertise-refs")
	case "/git-upload-pack":
		w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
	default:
		http.Error(w, "unexpected subpath (want /info/refs or /git-upload-pack) ", http.StatusInternalServerError)
		return
	}
	args = append(args, dir)

	body := r.Body
	defer body.Close()

	env := os.Environ()
	if protocol := r.Header.Get("Git-Protocol"); protocol != "" {
		env = append(env, "GIT_PROTOCOL="+protocol)
	}

	cmd := exec.CommandContext(r.Context(), "git", args...)
	cmd.Env = env
	cmd.Stdout = w
	cmd.Stdin = body
	if err := cmd.Run(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func packetWrite(str string) []byte {
	s := strconv.FormatInt(int64(len(str)+4), 16)
	if len(s)%4 != 0 {
		s = strings.Repeat("0", 4-len(s)%4) + s
	}
	return []byte(s + str)
}

var (
	metricServiceDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "src_gitserver_gitservice_duration_seconds",
		Help:    "A histogram of latencies for the git service (upload-pack for internal clones) endpoint.",
		Buckets: prometheus.ExponentialBuckets(.1, 5, 5), // 100ms -> 62s
	}, []string{"type"})

	metricServiceRunning = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "src_gitserver_gitservice_running",
		Help: "A histogram of latencies for the git service (upload-pack for internal clones) endpoint.",
	}, []string{"type"})
)
