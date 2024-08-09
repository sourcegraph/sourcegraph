package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/buildkite/go-buildkite/v3/buildkite"
	"github.com/gorilla/mux"
	"golang.org/x/exp/maps"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/dev/build-tracker/build"
	"github.com/sourcegraph/sourcegraph/dev/build-tracker/config"
	"github.com/sourcegraph/sourcegraph/dev/build-tracker/notify"
	"github.com/sourcegraph/sourcegraph/internal/testutil"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/lib/background"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/runtime"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// CleanUpInterval determines how often the old build cleaner should run
var CleanUpInterval = 5 * time.Minute

// BuildExpiryWindow defines the window for a build to be consider 'valid'. A build older than this window
// will be eligible for clean up.
var BuildExpiryWindow = 4 * time.Hour

// Server is the http server that listens for events from Buildkite. The server tracks builds and their associated jobs
// with the use of a BuildStore. Once a build is finished and has failed, the server sends a notification.
type Server struct {
	logger       log.Logger
	store        *build.Store
	config       *config.Config
	notifyClient notify.NotificationClient
	bkClient     *buildkite.Client
	bqWriter     BigQueryWriter
	http         *http.Server
}

// NewServer creatse a new server to listen for Buildkite webhook events.
func NewServer(addr string, logger log.Logger, c config.Config, bqWriter BigQueryWriter) *Server {
	logger = logger.Scoped("server")

	if testutil.IsTest && c.BuildkiteToken == "" {
		// buildkite.NewTokenConfig will complain even though we don't care
		c.BuildkiteToken = " "
	}

	bk, err := buildkite.NewTokenConfig(c.BuildkiteToken, false)
	if err != nil {
		panic(err)
	}

	server := &Server{
		logger:       logger,
		store:        build.NewBuildStore(logger),
		config:       &c,
		notifyClient: notify.NewClient(logger, c.SlackToken, c.SlackChannel),
		bqWriter:     bqWriter,
		bkClient:     buildkite.NewClient(bk.Client()),
	}

	// Register routes the server will be responding too
	r := mux.NewRouter()
	r.Path("/buildkite").HandlerFunc(server.handleEvent).Methods(http.MethodPost)
	r.Path("/-/healthz").HandlerFunc(server.handleHealthz).Methods(http.MethodGet)

	debug := r.PathPrefix("/-/debug").Subrouter()
	debug.Path("/{buildNumber}").HandlerFunc(server.handleGetBuild).Methods(http.MethodGet)

	server.http = &http.Server{
		Handler: r,
		Addr:    addr,
	}

	return server
}

func (s *Server) Name() string { return "build-tracker" }

func (s *Server) Start() {
	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error("error stopping server", log.Error(err))
	}
}

func (s *Server) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := s.http.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "shutdown server")
	}

	s.logger.Info("server stopped")
	return nil
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
	b := s.store.GetByBuildNumber(buildNum)
	if b == nil {
		// fallback to buildkite api on cache miss
		s.logger.Debug("cache miss, retrieving build from buildkite", log.Int("buildNumber", buildNum))
		bb, _, err := s.bkClient.Builds.Get("sourcegraph", "sourcegraph", fmt.Sprintf("%d", buildNum), nil)
		if err != nil {
			s.logger.Error("failed to retrieve build from buildkite", log.Error(err), log.Int("buildNumber", buildNum))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		b = build.FromBuildkiteBuild(*bb)
	}
	s.logger.Debug("encoding build", log.Int("buildNumber", buildNum))
	json.NewEncoder(w).Encode(b)
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
	} else if h[0] != s.config.BuildkiteWebhookToken {
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

	if testutil.IsTest {
		s.processEvent(&event)
	} else {
		go s.processEvent(&event)
	}
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

func (s *Server) triggerMetricsPipeline(b *build.Build) error {
	if *b.State == "cancelled" {
		return nil
	}

	repo := strings.TrimSuffix(*b.Pipeline.Repository, ".git")
	var prBase string
	var prNumber int
	if b.PullRequest != nil {
		prNumber, _ = strconv.Atoi(*b.PullRequest.ID)
		prBase = *b.PullRequest.Base
	}

	devxServiceCommit, err := getDevxServiceLatestCommit(s.config.GithubToken)
	if err != nil {
		return err
	}

	args := &buildkite.CreateBuild{
		Commit:  devxServiceCommit,
		Branch:  "main",
		Message: b.GetMessage(),
		Author:  b.GetCommitAuthor(),
		// TODO: do we need to clone b.Env?
		Env: map[string]string{
			// can't use BUILDKITE_ prefixed keys as they're reserved
			"DEVX_TRIGGERED_FROM_BUILD_ID":      pointers.DerefZero(b.ID),
			"DEVX_TRIGGERED_FROM_BUILD_NUMBER":  strconv.Itoa(pointers.DerefZero(b.Number)),
			"DEVX_TRIGGERED_FROM_PIPELINE_SLUG": pointers.DerefZero(b.Pipeline.Slug),
			"DEVX_TRIGGERED_FROM_COMMIT":        b.GetCommit(),
			"DEVX_TRIGGERED_FROM_COMMIT_URL":    fmt.Sprintf("%s/commit/%s", repo, b.GetCommit()),
			"DEVX_TRIGGERED_FROM_BRANCH":        b.GetBranch(),
			"DEVX_TRIGGERED_FROM_BRANCH_URL":    fmt.Sprintf("%s/tree/%s", repo, b.GetBranch()),
		},
		MetaData: map[string]string{},
	}

	if prNumber != 0 {
		maps.Copy(args.Env, map[string]string{
			"DEVX_TRIGGERED_FROM_PR_NUMBER":   strconv.Itoa(prNumber),
			"DEVX_TRIGGERED_FROM_PR_URL":      fmt.Sprintf("%s/pull/%d", repo, prNumber),
			"DEVX_TRIGGERED_FROM_BASE_BRANCH": prBase,
		})
	}

	triggered, response, err := s.bkClient.Builds.Create("sourcegraph", "devx-build-metrics", args)
	if err != nil {
		return errors.Wrap(err, "error triggering job")
	}

	if response.StatusCode >= 0 && response.StatusCode <= 300 {
		s.logger.Info("triggered job successfully", log.String("buildUrl", *triggered.WebURL), log.String("buildID", *triggered.ID), log.String("sourceBuildID", *b.ID))
	} else {
		s.logger.Warn("unexpected response for triggered job creation", log.Int("statusCode", response.StatusCode))
	}

	return nil
}

// processEvent processes a BuildEvent received from Buildkite. If the event is for a `build.finished` event we get the
// full build which includes all recorded jobs for the build and send a notification.
// processEvent delegates the decision to actually send a notifcation
func (s *Server) processEvent(event *build.Event) {
	if event.Build.Number != nil {
		s.logger.Info("processing event", log.String("eventName", event.Name), log.Int("buildNumber", event.GetBuildNumber()), log.String("jobName", event.GetJobName()))
		s.store.Add(event)
		b := s.store.GetByBuildNumber(event.GetBuildNumber())
		if event.IsBuildFinished() {
			if *event.Build.Branch == "main" {
				if err := s.notifyIfFailed(b); err != nil {
					s.logger.Error("failed to send notification for build", log.Int("buildNumber", event.GetBuildNumber()), log.Error(err))
				}
			}

			if err := s.triggerMetricsPipeline(b); err != nil {
				s.logger.Error("failed to trigger metrics pipeline for build", log.Int("buildNumber", event.GetBuildNumber()), log.Error(err))
			}
		}
	} else if event.Agent.ID != nil {
		if err := s.bqWriter.Write(context.Background(), &BuildkiteAgentEvent{
			event: event.Name,
			Agent: event.Agent,
			// if using HMAC signature from Buildkite, we could get a timestamp from there.
			// But we don't currently use that method, so we just use the current time.
			timestamp: time.Now(),
		}); err != nil {
			s.logger.Error("failed to write agent event to BigQuery", log.String("event", event.Name), log.String("agentID", *event.Agent.ID), log.Error(err))
		}
	}
}

func determineBuildStatusNotification(logger log.Logger, b *build.Build) *notify.BuildNotification {
	author := b.GetCommitAuthor()
	isRelease := b.IsReleaseBuild()
	// With a release build the person who made the last commit isn't the creator of the build
	if isRelease {
		author = b.GetBuildAuthor()
	}
	info := notify.BuildNotification{
		BuildNumber:        b.GetNumber(),
		ConsecutiveFailure: b.ConsecutiveFailure,
		AuthorName:         author.Name,
		Message:            b.GetMessage(),
		Commit:             b.GetCommit(),
		BuildStatus:        "",
		BuildURL:           b.GetWebURL(),
		Fixed:              []notify.JobLine{},
		Failed:             []notify.JobLine{},
		Passed:             []notify.JobLine{},
		TotalSteps:         len(b.Steps),
		IsRelease:          isRelease,
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

func getDevxServiceLatestCommit(token string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/sourcegraph/devx-service/branches/main", nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Accept", "application/vnd.github+json")
	// I do as the docs command https://docs.github.com/en/rest/branches/branches#get-a-branch
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.Newf("unexpected response from GitHub: %d", resp.StatusCode)
	}

	var respData struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", err
	}

	return respData.Commit.SHA, nil
}

func main() {
	runtime.Start[config.Config](Service{})
}

type Service struct{}

// Initialize implements runtime.Service.
func (s Service) Initialize(ctx context.Context, logger log.Logger, contract runtime.ServiceContract, config config.Config) (background.Routine, error) {
	logger.Info("config loaded from environment", log.Object("config", log.String("SlackChannel", config.SlackChannel), log.Bool("Production", config.Production)))

	bqWriter, err := contract.BigQuery.GetTableWriter(ctx, "agent_status")
	if err != nil {
		return nil, err
	}

	server := NewServer(fmt.Sprintf(":%d", contract.Port), logger, config, bqWriter)

	return background.CombinedRoutine{
		server,
		deleteOldBuilds(logger, server.store, CleanUpInterval, BuildExpiryWindow),
	}, nil
}

func (s Service) Name() string    { return "build-tracker" }
func (s Service) Version() string { return version.Version() }
