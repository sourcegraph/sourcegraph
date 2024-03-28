package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/dev/build-tracker-msp/build"
	"github.com/sourcegraph/sourcegraph/dev/build-tracker-msp/notify"
)

var (
	ErrInvalidToken  = errors.New("buildkite token is invalid")
	ErrInvalidHeader = errors.New("Header of request is invalid")
	ErrUnwantedEvent = errors.New("Unwanted event received")
)

var nowFunc = time.Now

// CleanUpInterval determines how often the old build cleaner should run
var CleanUpInterval = 5 * time.Minute

// BuildExpiryWindow defines the window for a build to be consider 'valid'. A build older than this window
// will be eligible for clean up.
var BuildExpiryWindow = 4 * time.Hour

// Server is the http server that listens for events from Buildkite. The server tracks builds and their associated jobs
// with the use of a BuildStore. Once a build is finished and has failed, the server sends a notification.
type Server struct {
	logger log.Logger
	// TODO: Move to redis or postgres
	store        *build.Store
	config       *Config
	notifyClient notify.NotificationClient
	http         *http.Server
}

// NewServer creatse a new server to listen for Buildkite webhook events.
func NewServer(addr string, logger log.Logger, c Config) *Server {
	logger = logger.Scoped("server")
	server := &Server{
		logger:       logger,
		store:        build.NewBuildStore(logger),
		config:       &c,
		notifyClient: notify.NewClient(logger, c.SlackToken, c.GithubToken, c.SlackChannel),
	}

	// Register routes the the server will be responding too
	r := mux.NewRouter()
	r.Path("/buildkite").HandlerFunc(server.handleEvent).Methods(http.MethodPost)
	r.Path("/healthz").HandlerFunc(server.handleHealthz).Methods(http.MethodGet)

	debug := r.PathPrefix("/-/debug").Subrouter()
	debug.Path("/{buildNumber}").HandlerFunc(server.handleGetBuild).Methods(http.MethodGet)

	server.http = &http.Server{
		Handler: r,
		Addr:    addr,
	}

	return server
}

func (s *Server) handleGetBuild(w http.ResponseWriter, req *http.Request) {
	if s.config.Production {
		user, pass, ok := req.BasicAuth()
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if user != "devx" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if pass != s.config.DebugPassword {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

	}
	vars := mux.Vars(req)

	buildNumParam, ok := vars["buildNumber"]
	if !ok {
		s.logger.Error("request received with no buildNumber path parameter", log.String("route", req.URL.Path))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	buildNum, err := strconv.Atoi(buildNumParam)
	if err != nil {
		s.logger.Error("invalid build number parameter received", log.String("buildNumParam", buildNumParam))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.logger.Info("retrieving build", log.Int("buildNumber", buildNum))
	build := s.store.GetByBuildNumber(buildNum)
	if build == nil {
		s.logger.Debug("no build found", log.Int("buildNumber", buildNum))
		w.WriteHeader(http.StatusNotFound)
		return
	}
	s.logger.Debug("encoding build", log.Int("buildNumber", buildNum))
	json.NewEncoder(w).Encode(build)
	w.WriteHeader(http.StatusOK)
}

// handleEvent handles an event received from the http listener. A event is valid when:
// - Has the correct headers from Buildkite
// - On of the following events
//   - job.finished
//   - build.finished
//
// - Has valid JSON
// Note that if we received an unwanted event ie. the event is not "job.finished" or "build.finished" we respond with a 200 OK regardless.
// Once all the conditions are met, the event is processed in a go routine with `processEvent`
func (s *Server) handleEvent(w http.ResponseWriter, req *http.Request) {
	h, ok := req.Header["X-Buildkite-Token"]
	if !ok || len(h) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	} else if h[0] != s.config.BuildkiteToken {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	h, ok = req.Header["X-Buildkite-Event"]
	if !ok || len(h) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	eventName := h[0]
	s.logger.Debug("received event", log.String("eventName", eventName))

	data, err := io.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		s.logger.Error("failed to read request body", log.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var event build.Event
	err = json.Unmarshal(data, &event)
	if err != nil {
		s.logger.Error("failed to unmarshall request body", log.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	go s.processEvent(&event)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleHealthz(w http.ResponseWriter, req *http.Request) {
	// do our super exhaustive check
	w.WriteHeader(http.StatusOK)
}

// notifyIfFailed sends a notification over slack if the provided build has failed. If the build is successful no notifcation is sent
func (s *Server) notifyIfFailed(b *build.Build) error {
	if !b.IsFinished() {
		s.logger.Info("build not finished yet, skipping notification", log.Int("buildNumber", b.GetNumber()))
	}
	// This determines the final build status
	info := determineBuildStatusNotification(s.logger, b)
	s.logger.Debug("build status notification",
		log.Int("buildNumber", info.BuildNumber),
		log.Int("Passed", len(info.Passed)),
		log.Int("Failed", len(info.Failed)),
		log.Int("Fixed", len(info.Fixed)),
	)

	if info.BuildStatus == string(build.BuildFailed) || info.BuildStatus == string(build.BuildFixed) {
		s.logger.Info("sending notification for build", log.Int("buildNumber", b.GetNumber()), log.String("status", string(info.BuildStatus)))
		// We lock the build while we send a notification so that we can ensure any late jobs do not interfere with what
		// we're about to send.
		b.Lock()
		defer b.Unlock()
		err := s.notifyClient.Send(info)
		return err
	}

	s.logger.Info("build has not failed", log.Int("buildNumber", b.GetNumber()), log.String("buildStatus", info.BuildStatus))
	return nil
}

func (s *Server) deleteOldBuilds(window time.Duration) {
	oldBuilds := make([]int, 0)
	now := nowFunc()
	for _, b := range s.store.FinishedBuilds() {
		finishedAt := *b.FinishedAt
		delta := now.Sub(finishedAt.Time)
		if delta >= window {
			s.logger.Debug("build past age window", log.Int("buildNumber", *b.Number), log.Time("FinishedAt", finishedAt.Time), log.Duration("window", window))
			oldBuilds = append(oldBuilds, *b.Number)
		}
	}
	s.logger.Info("deleting old builds", log.Int("oldBuildCount", len(oldBuilds)))
	s.store.DelByBuildNumber(oldBuilds...)
}

func (s *Server) startCleaner(every, window time.Duration) func() {
	ticker := time.NewTicker(every)
	done := make(chan interface{})

	go func() {
		for {
			select {
			case <-ticker.C:
				s.deleteOldBuilds(window)
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	return func() { done <- nil }
}

// processEvent processes a BuildEvent received from Buildkite. If the event is for a `build.finished` event we get the
// full build which includes all recorded jobs for the build and send a notification.
// processEvent delegates the decision to actually send a notifcation
func (s *Server) processEvent(event *build.Event) {
	s.logger.Info("processing event", log.String("eventName", event.Name), log.Int("buildNumber", event.GetBuildNumber()), log.String("jobName", event.GetJobName()))
	s.store.Add(event)
	b := s.store.GetByBuildNumber(event.GetBuildNumber())
	if event.IsBuildFinished() {
		if err := s.notifyIfFailed(b); err != nil {
			s.logger.Error("failed to send notification for build", log.Int("buildNumber", event.GetBuildNumber()), log.Error(err))
		}
	}
}

func determineBuildStatusNotification(logger log.Logger, b *build.Build) *notify.BuildNotification {
	info := notify.BuildNotification{
		BuildNumber:        b.GetNumber(),
		ConsecutiveFailure: b.ConsecutiveFailure,
		PipelineName:       b.Pipeline.GetName(),
		AuthorEmail:        b.GetAuthorEmail(),
		Message:            b.GetMessage(),
		Commit:             b.GetCommit(),
		BuildStatus:        "",
		BuildURL:           b.GetWebURL(),
		Fixed:              []notify.JobLine{},
		Failed:             []notify.JobLine{},
		Passed:             []notify.JobLine{},
		TotalSteps:         len(b.Steps),
	}

	// You may notice we do not check if the build is Failed and exit early, this is because of the following scenario
	// 1st build comes through it failed - we send a notification. 2nd build - a retry - comes through,
	// build passed. Now if we checked for build failed and didn't do any processing, we wouldn't be able
	// to process that the build has been fixed

	groups := build.GroupByStatus(b.Steps)
	for _, j := range groups[build.JobFixed] {
		info.Fixed = append(info.Fixed, j)
	}
	for _, j := range groups[build.JobFailed] {
		info.Failed = append(info.Failed, j)
	}
	for _, j := range groups[build.JobPassed] {
		info.Passed = append(info.Passed, j)
	}
	for _, j := range groups[build.JobUnknown] {
		logger.Debug("unknown job status", log.Int("buildNumber", b.GetNumber()), log.Object("job", log.String("name", j.Name), log.String("state", j.LastJob().GetState())))
	}

	if len(info.Failed) > 0 {
		info.BuildStatus = string(build.BuildFailed)
	} else if len(info.Fixed) > 0 {
		info.BuildStatus = string(build.BuildFixed)
	} else {
		info.BuildStatus = string(build.BuildPassed)
	}
	return &info
}

// Serve starts the http server and listens for buildkite build events to be sent on the route "/buildkite"
// func main() {
// 	sync := log.Init(log.Resource{
// 		Name:      "BuildTracker",
// 		Namespace: "CI",
// 	})
// 	defer sync.Sync()

// 	logger := log.Scoped("BuildTracker")

// 	serverConf, err := config.NewFromEnv()
// 	if err != nil {
// 		logger.Fatal("failed to get config from env", log.Error(err))
// 	}
// 	logger.Info("config loaded from environment", log.Object("config", log.String("SlackChannel", serverConf.SlackChannel), log.Bool("Production", serverConf.Production)))
// 	server := NewServer(logger, *serverConf)

// 	stopFn := server.startCleaner(CleanUpInterval, BuildExpiryWindow)
// 	defer stopFn()

// 	if server.config.Production {
// 		server.logger.Info("server is in production mode!")
// 	} else {
// 		server.logger.Info("server is in development mode!")
// 	}

// 	if err := server.http.ListenAndServe(); err != nil {
// 		logger.Fatal("server exited with error", log.Error(err))
// 	}
// }
