package updatecheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/siteid"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/usagestatsdeprecated"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/redispool"
	"github.com/sourcegraph/sourcegraph/internal/usagestats"
	"github.com/sourcegraph/sourcegraph/internal/version"
)

// metricsRecorder records operational metrics for methods.
var metricsRecorder = metrics.NewOperationMetrics(prometheus.DefaultRegisterer, "updatecheck_client", metrics.WithLabels("method"))

// Status of the check for software updates for Sourcegraph.
type Status struct {
	Date          time.Time // the time that the last check completed
	Err           error     // the error that occurred, if any. When present, indicates the instance is offline / unable to contact Sourcegraph.com
	UpdateVersion string    // the version string of the updated version, if any
}

// HasUpdate reports whether the status indicates an update is available.
func (s Status) HasUpdate() bool { return s.UpdateVersion != "" }

var (
	mu         sync.Mutex
	startedAt  *time.Time
	lastStatus *Status
)

// Last returns the status of the last-completed software update check.
func Last() *Status {
	mu.Lock()
	defer mu.Unlock()
	if lastStatus == nil {
		return nil
	}
	tmp := *lastStatus
	return &tmp
}

// IsPending returns whether an update check is in progress.
func IsPending() bool {
	mu.Lock()
	defer mu.Unlock()
	return startedAt != nil
}

// recordOperation returns a record fn that is called on any given return err. If an error is encountered
// it will register the err metric. The err is never altered.
func recordOperation(method string) func(*error) {
	start := time.Now()
	return func(err *error) {
		metricsRecorder.Observe(time.Since(start).Seconds(), 1, err, method)
	}
}

func getAndMarshalSiteActivityJSON(ctx context.Context, db dbutil.DB, criticalOnly bool) (_ json.RawMessage, err error) {
	defer recordOperation("getAndMarshalSiteActivityJSON")(&err)
	siteActivity, err := usagestats.GetSiteUsageStats(ctx, db, criticalOnly)

	if err != nil {
		return nil, err
	}

	return json.Marshal(siteActivity)
}

func hasSearchOccurred(ctx context.Context) (_ bool, err error) {
	defer recordOperation("hasSearchOccurred")(&err)
	return usagestats.HasSearchOccurred(ctx)
}

func hasFindRefsOccurred(ctx context.Context) (_ bool, err error) {
	defer recordOperation("hasSearchOccured")(&err)
	return usagestats.HasFindRefsOccurred(ctx)
}

func getTotalUsersCount(ctx context.Context) (_ int, err error) {
	defer recordOperation("getTotalUsersCount")(&err)
	return database.GlobalUsers.Count(ctx, &database.UsersListOptions{})
}

func getTotalReposCount(ctx context.Context) (_ int, err error) {
	defer recordOperation("getTotalReposCount")(&err)
	return database.GlobalRepos.Count(ctx, database.ReposListOptions{})
}

func getUsersActiveTodayCount(ctx context.Context) (_ int, err error) {
	defer recordOperation("getUsersActiveTodayCount")(&err)
	return usagestatsdeprecated.GetUsersActiveTodayCount(ctx)
}

func getInitialSiteAdminEmail(ctx context.Context) (_ string, err error) {
	defer recordOperation("getInitialSiteAdminEmail")(&err)
	return database.GlobalUserEmails.GetInitialSiteAdminEmail(ctx)
}

func getAndMarshalCampaignsUsageJSON(ctx context.Context) (_ json.RawMessage, err error) {
	defer recordOperation("getAndMarshalCampaignsUsageJSON")(&err)

	campaignsUsage, err := usagestats.GetBatchChangesUsageStatistics(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(campaignsUsage)
}

func getAndMarshalGrowthStatisticsJSON(ctx context.Context) (_ json.RawMessage, err error) {
	defer recordOperation("getAndMarshalGrowthStatisticsJSON")(&err)

	growthStatistics, err := usagestats.GetGrowthStatistics(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(growthStatistics)
}

func getAndMarshalSavedSearchesJSON(ctx context.Context) (_ json.RawMessage, err error) {
	defer recordOperation("getAndMarshalSavedSearchesJSON")(&err)

	savedSearches, err := usagestats.GetSavedSearches(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(savedSearches)
}

func getAndMarshalHomepagePanelsJSON(ctx context.Context) (_ json.RawMessage, err error) {
	defer recordOperation("getAndMarshalHomepagePanelsJSON")(&err)

	homepagePanels, err := usagestats.GetHomepagePanels(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(homepagePanels)
}

func getAndMarshalRepositoriesJSON(ctx context.Context) (_ json.RawMessage, err error) {
	defer recordOperation("getAndMarshalRepositoriesJSON")(&err)

	repos, err := usagestats.GetRepositories(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(repos)
}

func getAndMarshalRetentionStatisticsJSON(ctx context.Context) (_ json.RawMessage, err error) {
	defer recordOperation("getAndMarshalRetentionStatisticsJSON")(&err)

	retentionStatistics, err := usagestats.GetRetentionStatistics(ctx)
	if err != nil {
		return nil, err
	}

	return json.Marshal(retentionStatistics)
}

func getAndMarshalSearchOnboardingJSON(ctx context.Context) (_ json.RawMessage, err error) {
	defer recordOperation("getAndMarshalSearchOnboardingJSON")(&err)

	searchOnboarding, err := usagestats.GetSearchOnboarding(ctx)
	if err != nil {
		return nil, err
	}

	return json.Marshal(searchOnboarding)
}

func getAndMarshalAggregatedCodeIntelUsageJSON(ctx context.Context, db dbutil.DB) (_ json.RawMessage, err error) {
	defer recordOperation("getAndMarshalAggregatedCodeIntelUsageJSON")(&err)

	codeIntelUsage, err := usagestats.GetAggregatedCodeIntelStats(ctx, db)
	if err != nil {
		return nil, err
	}

	return json.Marshal(codeIntelUsage)
}

func getAndMarshalAggregatedSearchUsageJSON(ctx context.Context, db dbutil.DB) (_ json.RawMessage, err error) {
	defer recordOperation("getAndMarshalAggregatedSearchUsageJSON")(&err)

	searchUsage, err := usagestats.GetAggregatedSearchStats(ctx, db)
	if err != nil {
		return nil, err
	}

	return json.Marshal(searchUsage)
}

func getAndMarshalExtensionsUsageStatisticsJSON(ctx context.Context) (_ json.RawMessage, err error) {
	defer recordOperation("getAndMarshalExtensionsUsageStatisticsJSON")

	extensionsUsage, err := usagestats.GetExtensionsUsageStatistics(ctx)
	if err != nil {
		return nil, err
	}

	return json.Marshal(extensionsUsage)
}

func getAndMarshalCodeInsightsUsageJSON(ctx context.Context) (_ json.RawMessage, err error) {
	defer recordOperation("getAndMarshalCodeInsightsUsageJSON")

	codeInsightsUsage, err := usagestats.GetCodeInsightsUsageStatistics(ctx)
	if err != nil {
		return nil, err
	}

	return json.Marshal(codeInsightsUsage)
}

func getDependencyVersions(ctx context.Context, logFunc func(string, ...interface{})) (json.RawMessage, error) {
	var (
		err error
		dv  dependencyVersions
	)
	// get redis cache server version
	dv.RedisCacheVersion, err = getRedisVersion(redispool.Cache.Dial)
	if err != nil {
		logFunc("updatecheck.getDependencyVersions: unable to get Redis cache version", "error", err)
	}

	// get redis store server version
	dv.RedisStoreVersion, err = getRedisVersion(redispool.Store.Dial)
	if err != nil {
		logFunc("updatecheck.getDependencyVersions: unable to get Redis store version", "error", err)
	}

	// get postgres version
	err = dbconn.Global.QueryRowContext(ctx, "SHOW server_version").Scan(&dv.PostgresVersion)
	if err != nil {
		logFunc("updatecheck.getDependencyVersions: unable to get Postgres version", "error", err)
	}
	return json.Marshal(dv)
}

func getRedisVersion(dialFunc func() (redis.Conn, error)) (string, error) {
	conn, err := dialFunc()
	if err != nil {
		return "", err
	}
	buf, err := redis.Bytes(conn.Do("INFO"))
	if err != nil {
		return "", err
	}

	m, err := parseRedisInfo(buf)
	return m["redis_version"], err

}
func parseRedisInfo(buf []byte) (map[string]string, error) {
	var (
		lines = bytes.Split(buf, []byte("\n"))
		m     = make(map[string]string, len(lines))
	)

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, []byte("#")) || len(line) == 0 {
			continue
		}

		parts := bytes.Split(line, []byte(":"))
		if len(parts) != 2 {
			return nil, fmt.Errorf("expected a key:value line, got %q", string(line))
		}
		m[string(parts[0])] = string(parts[1])
	}

	return m, nil
}

func updateBody(ctx context.Context, db dbutil.DB) (io.Reader, error) {
	logFunc := log15.Debug
	if envvar.SourcegraphDotComMode() {
		logFunc = log15.Warn
	}

	r := &pingRequest{
		ClientSiteID:        siteid.Get(),
		DeployType:          conf.DeployType(),
		ClientVersionString: version.Version(),
		LicenseKey:          conf.Get().LicenseKey,
		CodeIntelUsage:      []byte("{}"),
		NewCodeIntelUsage:   []byte("{}"),
		SearchUsage:         []byte("{}"),
		CampaignsUsage:      []byte("{}"),
		GrowthStatistics:    []byte("{}"),
		SavedSearches:       []byte("{}"),
		HomepagePanels:      []byte("{}"),
		Repositories:        []byte("{}"),
		RetentionStatistics: []byte("{}"),
		SearchOnboarding:    []byte("{}"),
		ExtensionsUsage:     []byte("{}"),
		CodeInsightsUsage:   []byte("{}"),
	}

	totalUsers, err := getTotalUsersCount(ctx)
	if err != nil {
		logFunc("telemetry: database.Users.Count failed", "error", err)
	}
	r.TotalUsers = int32(totalUsers)
	r.InitialAdminEmail, err = getInitialSiteAdminEmail(ctx)
	if err != nil {
		logFunc("telemetry: database.UserEmails.GetInitialSiteAdminEmail failed", "error", err)
	}

	r.DependencyVersions, err = getDependencyVersions(ctx, logFunc)
	if err != nil {
		logFunc("telemetry: getDependencyVersions failed", "error", err)
	}

	if !conf.Get().DisableNonCriticalTelemetry {
		// TODO(Dan): migrate this to the new usagestats package.
		//
		// For the time being, instances will report daily active users through the legacy package via this argument,
		// as well as using the new package through the `act` argument below. This will allow comparison during the
		// transition.
		count, err := getUsersActiveTodayCount(ctx)
		if err != nil {
			logFunc("telemetry: updatecheck.getUsersActiveToday failed", "error", err)
		}
		r.UniqueUsers = int32(count)
		totalRepos, err := getTotalReposCount(ctx)
		if err != nil {
			logFunc("telemetry: updatecheck.getTotalReposCount failed", "error", err)
		}
		r.HasRepos = totalRepos > 0

		r.EverSearched, err = hasSearchOccurred(ctx)
		if err != nil {
			logFunc("telemetry: updatecheck.hasSearchOccurred failed", "error", err)
		}
		r.EverFindRefs, err = hasFindRefsOccurred(ctx)
		if err != nil {
			logFunc("telemetry: updatecheck.hasFindRefsOccurred failed", "error", err)
		}
		r.CampaignsUsage, err = getAndMarshalCampaignsUsageJSON(ctx)
		if err != nil {
			logFunc("telemetry: updatecheck.getAndMarshalCampaignsUsageJSON failed", "error", err)
		}
		r.GrowthStatistics, err = getAndMarshalGrowthStatisticsJSON(ctx)
		if err != nil {
			logFunc("telemetry: updatecheck.getAndMarshalGrowthStatisticsJSON failed", "error", err)
		}

		r.SavedSearches, err = getAndMarshalSavedSearchesJSON(ctx)
		if err != nil {
			logFunc("telemetry: updatecheck.getAndMarshalSavedSearchesJSON failed", "error", err)
		}

		r.HomepagePanels, err = getAndMarshalHomepagePanelsJSON(ctx)
		if err != nil {
			logFunc("telemetry: updatecheck.getAndMarshalHomepagePanelsJSON failed", "error", err)
		}

		r.SearchOnboarding, err = getAndMarshalSearchOnboardingJSON(ctx)
		if err != nil {
			logFunc("telemetry: updatecheck.getAndMarshalSearchOnboardingJSON failed", "error", err)
		}

		r.Repositories, err = getAndMarshalRepositoriesJSON(ctx)
		if err != nil {
			logFunc("telemetry: updatecheck.getAndMarshalRepositoriesJSON failed", "error", err)
		}

		r.RetentionStatistics, err = getAndMarshalRetentionStatisticsJSON(ctx)
		if err != nil {
			logFunc("telemetry: updatecheck.getAndMarshalRetentionStatisticsJSON failed", "error", err)
		}

		r.ExtensionsUsage, err = getAndMarshalExtensionsUsageStatisticsJSON(ctx)
		if err != nil {
			logFunc("telemetry: updatecheck.getAndMarshalExtensionsUsageStatisticsJSON failed", "error", err)
		}

		r.CodeInsightsUsage, err = getAndMarshalCodeInsightsUsageJSON(ctx)
		if err != nil {
			logFunc("telemetry: updatecheck.getAndMarshalCodeInsightsUsageJSON failed", "error", err)
		}

		r.ExternalServices, err = externalServiceKinds(ctx)
		if err != nil {
			logFunc("telemetry: externalServicesKinds failed", "error", err)
		}

		r.HasExtURL = conf.UsingExternalURL()
		r.BuiltinSignupAllowed = conf.IsBuiltinSignupAllowed()
		r.AuthProviders = authProviderTypes()

		// The following methods are the most expensive to calculate, so we do them in
		// parallel.

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Activity, err = getAndMarshalSiteActivityJSON(ctx, db, false)
			if err != nil {
				logFunc("telemetry: updatecheck.getAndMarshalSiteActivityJSON failed", "error", err)
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			r.NewCodeIntelUsage, err = getAndMarshalAggregatedCodeIntelUsageJSON(ctx, db)
			if err != nil {
				logFunc("telemetry: updatecheck.getAndMarshalAggregatedCodeIntelUsageJSON failed", "error", err)
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			r.SearchUsage, err = getAndMarshalAggregatedSearchUsageJSON(ctx, db)
			if err != nil {
				logFunc("telemetry: updatecheck.getAndMarshalAggregatedSearchUsageJSON failed", "error", err)
			}
		}()

		wg.Wait()
	} else {
		r.Repositories, err = getAndMarshalRepositoriesJSON(ctx)
		if err != nil {
			logFunc("telemetry: updatecheck.getAndMarshalRepositoriesJSON failed", "error", err)
		}

		r.Activity, err = getAndMarshalSiteActivityJSON(ctx, db, true)
		if err != nil {
			logFunc("telemetry: updatecheck.getAndMarshalSiteActivityJSON failed", "error", err)
		}
	}

	contents, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	err = database.EventLogs(db).Insert(ctx, &database.Event{
		UserID:          0,
		Name:            "ping",
		URL:             "",
		AnonymousUserID: "backend",
		Source:          "BACKEND",
		Argument:        contents,
		Timestamp:       time.Now().UTC(),
	})

	return bytes.NewReader(contents), err
}

func authProviderTypes() []string {
	ps := conf.Get().AuthProviders
	types := make([]string, len(ps))
	for i, p := range ps {
		types[i] = conf.AuthProviderType(p)
	}
	return types
}

func externalServiceKinds(ctx context.Context) (kinds []string, err error) {
	defer recordOperation("externalServiceKinds")(&err)
	kinds, err = database.GlobalExternalServices.DistinctKinds(ctx)
	return kinds, err
}

// check performs an update check and updates the global state.
func check(db dbutil.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	doCheck := func() (updateVersion string, err error) {
		body, err := updateBody(ctx, db)
		if err != nil {
			return "", err
		}

		req, err := http.NewRequest("POST", "https://sourcegraph.com/.api/updates", body)
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(ctx)

		resp, err := httpcli.ExternalDoer().Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			var description string
			if body, err := ioutil.ReadAll(io.LimitReader(resp.Body, 30)); err != nil {
				description = err.Error()
			} else if len(body) == 0 {
				description = "(no response body)"
			} else {
				description = strconv.Quote(string(bytes.TrimSpace(body)))
			}
			return "", fmt.Errorf("update endpoint returned HTTP error %d: %s", resp.StatusCode, description)
		}

		if resp.StatusCode == http.StatusNoContent {
			return "", nil // no update available
		}

		var latestBuild build
		if err := json.NewDecoder(resp.Body).Decode(&latestBuild); err != nil {
			return "", err
		}
		return latestBuild.Version.String(), nil
	}

	mu.Lock()
	thisCheckStartedAt := time.Now()
	startedAt = &thisCheckStartedAt
	mu.Unlock()

	updateVersion, err := doCheck()

	if err != nil {
		log15.Error("telemetry: updatecheck failed", "error", err)
	}

	mu.Lock()
	if startedAt != nil && !startedAt.After(thisCheckStartedAt) {
		startedAt = nil
	}
	lastStatus = &Status{
		Date:          time.Now(),
		Err:           err,
		UpdateVersion: updateVersion,
	}
	mu.Unlock()
}

var started bool

// Start starts checking for software updates periodically.
func Start(db dbutil.DB) {
	if started {
		panic("already started")
	}
	started = true

	if channel := conf.UpdateChannel(); channel != "release" {
		return // no update check
	}

	const delay = 30 * time.Minute
	for {
		check(db)

		// Randomize sleep to prevent thundering herds.
		randomDelay := time.Duration(rand.Intn(600)) * time.Second
		time.Sleep(delay + randomDelay)
	}
}
